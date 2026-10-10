package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

func TestBillingSubscription(t *testing.T) {
	fake, srv := newFakeMP(t)
	e := setupWith(t, func(c *config.Config) {
		c.MPAccessToken, c.MPAPIBase = "platform-token", srv.URL
		c.PlanPriceMonth["crecimiento"] = 600
		c.PlanPriceMonth["basico"] = 300
	})
	trialEnd := time.Now().Add(5*24*time.Hour + time.Hour)
	e.exec(`UPDATE clinics SET plan='basico', billing_status='trialing', trial_ends_at=$2 WHERE id=$1`, e.clinicA, trialEnd)
	admin := e.login("admin_a")
	webhook := func(typ, id string) int {
		body, _ := json.Marshal(map[string]any{"type": typ, "data": map[string]any{"id": id}})
		res, err := http.Post(e.srv.URL+"/api/webhooks/mercadopago", "application/json", strings.NewReader(string(body)))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	clinic := func() (plan, preID string, end time.Time, cancelAtEnd bool) {
		if err := e.pool.QueryRow(t.Context(), `SELECT plan, mp_preapproval_id, current_period_end, cancel_at_period_end FROM clinics WHERE id=$1`, e.clinicA).Scan(&plan, &preID, &end, &cancelAtEnd); err != nil {
			t.Fatal(err)
		}
		return
	}
	payments := func() int {
		var n int
		if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM payments WHERE clinic_id=$1 AND provider='mercadopago'`, e.clinicA).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Alta: suscripción mensual que respeta los días de prueba que quedan
	co := admin.expect(201, "POST", "/api/billing/checkout", map[string]any{"plan": "crecimiento", "period": "month"})
	id := co["id"].(string)
	if co["init_point"] != "https://mp.test/sub/pre-1" || co["subscription"] != true {
		t.Fatalf("checkout: %v", co)
	}
	body := fake.preBodies[0]
	rec := body["auto_recurring"].(map[string]any)
	if body["external_reference"] != "caresia-sub:"+id || body["payer_email"] != "admin_a@clinic.mx" || rec["transaction_amount"] != 600.0 || rec["frequency"] != 1.0 ||
		body["notification_url"] != "http://api.test/api/webhooks/mercadopago" || body["back_url"] != "http://app.test/suscripcion?suscripcion="+id {
		t.Fatalf("cuerpo del preapproval: %v", body)
	}
	if ft := rec["free_trial"].(map[string]any); ft["frequency"] != 6.0 || ft["frequency_type"] != "days" {
		t.Fatalf("la prueba que queda debe respetarse: %v", ft)
	}
	if st := sub(admin.expect(200, "GET", "/api/billing/checkouts/"+id, nil), "checkout")["status"]; st != "pending" {
		t.Fatalf("sin autorizar sigue pendiente: %v", st)
	}

	// La autoriza en Mercado Pago; aunque no llegue el aviso, la página de regreso la sincroniza
	fake.setPre("pre-1", "authorized", trialEnd)
	if st := sub(admin.expect(200, "GET", "/api/billing/checkouts/"+id, nil), "checkout")["status"]; st != "paid" {
		t.Fatalf("la página de regreso debe activar la suscripción: %v", st)
	}
	plan, pre, end, _ := clinic()
	if plan != "crecimiento" || pre != "pre-1" || end.Sub(trialEnd).Abs() > time.Minute {
		t.Fatalf("clínica tras autorizar: %s %s %v", plan, pre, end)
	}
	ov := admin.expect(200, "GET", "/api/billing", nil)
	if s := sub(ov, "subscription"); s["active"] != true || s["period"] != "month" || s["status"] != "authorized" || sub(ov, "billing")["state"] != "active" {
		t.Fatalf("resumen: %v", ov["subscription"])
	}
	admin.expect(409, "POST", "/api/billing/checkout", map[string]any{"plan": "crecimiento", "period": "month"}) // ya suscrito

	// Primer cobro (al terminar la prueba): aviso del cobro; el plan vale hasta el siguiente
	next := trialEnd.AddDate(0, 1, 0)
	fake.setPre("pre-1", "authorized", next)
	fake.addCharge("7001", "pre-1", 600, "processed")
	if code := webhook("subscription_authorized_payment", "7001"); code != 200 {
		t.Fatalf("webhook cobro: %d", code)
	}
	webhook("subscription_authorized_payment", "7001") // repetido
	if n := payments(); n != 1 {
		t.Fatalf("un cobro repetido no se duplica: %d", n)
	}
	if _, _, end, _ := clinic(); end.Sub(next).Abs() > time.Minute {
		t.Fatalf("el periodo debe ir hasta el siguiente cobro: %v", end)
	}
	if len(e.mail.sent) == 0 {
		t.Fatal("se espera el recibo del cobro por correo")
	}
	fake.addCharge("7002", "pre-1", 600, "recycling") // cobro rechazado que MP reintenta: no cuenta
	webhook("subscription_authorized_payment", "7002")
	if n := payments(); n != 1 {
		t.Fatalf("un cobro no procesado no se anota: %d", n)
	}

	// Avisos que no son de Caresia (la misma cuenta la usa MiTiendita) se ignoran
	fake.mu.Lock()
	fake.preapprovals["pre-mt"] = map[string]any{"id": "pre-mt", "status": "cancelled", "external_reference": "64f0c0ffee:basic:month"}
	fake.mu.Unlock()
	if code := webhook("subscription_preapproval", "pre-mt"); code != 200 {
		t.Fatalf("aviso ajeno: %d", code)
	}
	if code := webhook("subscription_preapproval", "no-existe"); code != 200 {
		t.Fatalf("aviso desconocido: %d", code)
	}
	if _, pre, _, _ := clinic(); pre != "pre-1" {
		t.Fatalf("un aviso ajeno no debe tocar la clínica: %s", pre)
	}

	// Cambio de plan: la nueva reemplaza a la anterior, que se cancela en Mercado Pago
	co2 := admin.expect(201, "POST", "/api/billing/checkout", map[string]any{"plan": "basico", "period": "year"})
	if rec := fake.preBodies[1]["auto_recurring"].(map[string]any); rec["frequency"] != 12.0 || rec["transaction_amount"] != 3000.0 {
		t.Fatalf("anual: %v", rec)
	}
	fake.setPre("pre-2", "authorized", time.Now().AddDate(1, 0, 0))
	if code := webhook("subscription_preapproval", "pre-2"); code != 200 {
		t.Fatalf("webhook alta: %d", code)
	}
	if plan, pre, _, _ := clinic(); plan != "basico" || pre != "pre-2" {
		t.Fatalf("tras cambiar de plan: %s %s", plan, pre)
	}
	if fake.preapprovals["pre-1"]["status"] != "cancelled" {
		t.Fatal("la suscripción anterior debe cancelarse")
	}
	_ = co2

	// Permisos: solo el administrador de la propia clínica
	if code := status(e.login("doc_a"), "POST", "/api/billing/subscription/cancel"); code != 403 {
		t.Fatalf("doctor no cancela: %d", code)
	}
	e.login("admin_b").expect(404, "POST", "/api/billing/subscription/cancel", nil)

	// Cancelar: sigue con acceso hasta el fin del periodo pagado
	admin.expect(200, "POST", "/api/billing/subscription/cancel", nil)
	if fake.preapprovals["pre-2"]["status"] != "cancelled" {
		t.Fatal("debe cancelarse en Mercado Pago")
	}
	if _, _, _, cancelAtEnd := clinic(); !cancelAtEnd {
		t.Fatal("debe quedar marcada para terminar al fin del periodo")
	}
	if s := sub(sub(admin.expect(200, "GET", "/api/session", nil), "session"), "billing"); s["state"] != "active" {
		t.Fatalf("tras cancelar conserva el acceso: %v", s)
	}
	admin.expect(200, "GET", "/api/appointments", nil)
}
