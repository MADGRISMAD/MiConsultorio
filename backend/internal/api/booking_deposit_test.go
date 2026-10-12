package api_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
)

// connectMP links clinic A to the fake Mercado Pago the way the clinic's admin does.
func connectMP(t *testing.T, b *bookingEnv, apiBase string) {
	t.Helper()
	b.cfg.MPAPIBase, b.cfg.MPClientID, b.cfg.MPClientSecret = apiBase, "client", "secret"
	b.cfg.MPAuthBase = "https://auth.mercadopago.com.mx"
	u, err := url.Parse(b.login("admin_a").expect(200, "GET", "/api/pos/point/connect", nil)["url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := noRedirect.Get(b.srv.URL + "/api/point/oauth/callback?code=good-code&state=" + url.QueryEscape(u.Query().Get("state")))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if !strings.Contains(res.Header.Get("Location"), "mp=ok") {
		t.Fatalf("connect: %s", res.Header.Get("Location"))
	}
}

func TestOnlineDeposit(t *testing.T) {
	b := newBookingEnv(t, false)
	fake, srv := newFakeMP(t)
	connectMP(t, b, srv.URL)
	admin, anon := b.login("admin_a"), b.anon()

	var svc string
	if err := b.pool.QueryRow(t.Context(), `SELECT id::text FROM catalog_items WHERE clinic_id = $1 AND name = 'Consulta general'`, b.clinicA).Scan(&svc); err != nil {
		t.Fatal(err)
	}
	book := func(start, email, phone string) (int, map[string]any) {
		return anon.do("POST", "/api/public/booking/"+b.slugA+"/appointments", map[string]any{
			"professional_id": b.pro, "service_id": svc, "date": b.date, "start": start, "names": "Ana", "last_names": "López",
			"email": email, "phone": phone, "reason": "Dolor", "accept_privacy": true, "accept_reminders": true,
		})
	}

	// no percentage: booked straight away, nothing to pay
	if code, out := book("09:00", "a@x.mx", "5511111111"); code != 201 || sub(out, "appointment")["status"] != "scheduled" {
		t.Fatalf("without anticipo: %d %v", code, out)
	}

	// validation of the percentage, per service and for the whole agenda
	admin.expect(400, "PUT", "/api/agenda/services/"+svc+"/booking", map[string]any{"modality": "presencial", "deposit_percent": 101})
	admin.expect(200, "PUT", "/api/agenda/services/"+svc+"/booking", map[string]any{"modality": "presencial", "deposit_percent": 30})
	info := sub(anon.expect(200, "GET", "/api/public/booking/"+b.slugA, nil), "booking")
	found := false
	for _, s := range info["services"].([]any) {
		if m := s.(map[string]any); m["id"] == svc {
			found = true
			if num(m, "deposit_cents") != 15000 {
				t.Fatalf("anticipo shown to the visitor: %v", m)
			}
		}
	}
	if !found {
		t.Fatal("service missing")
	}

	// booking holds the time and asks for the payment; nobody is told yet
	mails := len(b.mail.sent)
	code, out := book("10:00", "b@x.mx", "5522222222")
	if code != 201 {
		t.Fatalf("book with anticipo: %d %v", code, out)
	}
	ap := sub(out, "appointment")
	dep := ap["deposit"].(map[string]any)
	if ap["status"] != "pending_payment" || num(dep, "cents") != 15000 || !strings.HasPrefix(dep["pay_url"].(string), "https://mp.test/pay/") {
		t.Fatalf("pending booking: %v", ap)
	}
	token := ap["token"].(string)
	var apptID string
	var remindersQueued int
	if err := b.pool.QueryRow(t.Context(), `SELECT id::text FROM appointments WHERE confirm_token = $1`, token).Scan(&apptID); err != nil {
		t.Fatal(err)
	}
	_ = b.pool.QueryRow(t.Context(), `SELECT count(*) FROM appointment_reminders WHERE appointment_id = $1`, apptID).Scan(&remindersQueued)
	time.Sleep(100 * time.Millisecond)
	if remindersQueued != 0 || len(b.mail.sent) != mails {
		t.Fatalf("a pending payment must not remind or mail: reminders=%d mails=%d", remindersQueued, len(b.mail.sent)-mails)
	}
	if code, _ := book("10:00", "c@x.mx", "5533333333"); code != 409 {
		t.Fatalf("the held time is taken, got %d", code)
	}
	if v := sub(anon.expect(200, "GET", "/api/public/appointments/"+token, nil), "appointment"); v["status"] != "pending_payment" || sub(v, "deposit")["pay_url"] == "" {
		t.Fatalf("view while unpaid: %v", v)
	}

	// paid: coming back from Mercado Pago confirms it and does what a new booking does
	fake.addPayment("9001", "caresia-dep:"+apptID, 150.00, "approved")
	v := sub(anon.expect(200, "GET", "/api/public/appointments/"+token, nil), "appointment")
	if v["status"] != "scheduled" || sub(v, "deposit")["state"] != "paid" {
		t.Fatalf("view after paying: %v", v)
	}
	b.mail.wait(t, mails+1)
	_ = b.pool.QueryRow(t.Context(), `SELECT count(*) FROM appointment_reminders WHERE appointment_id = $1`, apptID).Scan(&remindersQueued)
	if remindersQueued == 0 {
		t.Fatal("paid bookings get their reminders")
	}

	// the cashier can apply the paid anticipo once
	dp := sub(b.login("cash_a").expect(200, "GET", "/api/pos/appointments/"+apptID+"/deposit", nil), "deposit")
	if dp == nil || num(dp, "cents") != 15000 {
		t.Fatalf("pos deposit: %v", dp)
	}

	// the patient cancels with time to spare: the anticipo goes back
	anon.expect(200, "POST", "/api/public/appointments/"+token+"/cancel", map[string]any{})
	if len(fake.refunded) != 1 || fake.refunded[0] != "payment:9001" {
		t.Fatalf("refund: %v", fake.refunded)
	}
	var state string
	_ = b.pool.QueryRow(t.Context(), `SELECT deposit_state FROM appointments WHERE id = $1`, apptID).Scan(&state)
	if state != "refunded" {
		t.Fatalf("deposit state: %s", state)
	}

	// unpaid ones are released after the hold, and a late payment is returned
	code, out = book("11:00", "d@x.mx", "5544444444")
	if code != 201 {
		t.Fatalf("second pending: %d %v", code, out)
	}
	var id2 string
	_ = b.pool.QueryRow(t.Context(), `SELECT id::text FROM appointments WHERE confirm_token = $1`, sub(out, "appointment")["token"]).Scan(&id2)
	b.exec(`UPDATE appointments SET deposit_expires_at = now() - interval '1 minute' WHERE id = $1`, id2)
	api.RunDepositsOnce(t.Context(), b.pool, b.cfg, b.mail)
	_ = b.pool.QueryRow(t.Context(), `SELECT status FROM appointments WHERE id = $1`, id2).Scan(&state)
	if state != "cancelled" {
		t.Fatalf("expired hold: %s", state)
	}
	if code, _ := book("11:00", "e@x.mx", "5555555555"); code != 201 {
		t.Fatalf("the released time can be booked again, got %d", code)
	}
	fake.addPayment("9002", "caresia-dep:"+id2, 150.00, "approved")
	anon.expect(200, "POST", "/api/webhooks/mercadopago?type=payment&dep="+id2, map[string]any{"type": "payment", "data": map[string]any{"id": "9002"}})
	if len(fake.refunded) != 2 || fake.refunded[1] != "payment:9002" {
		t.Fatalf("late payment must be refunded: %v", fake.refunded)
	}

	// staff: a no-show keeps the anticipo of a paid visit
	_, out = book("12:00", "f@x.mx", "5566666666")
	var id3 string
	_ = b.pool.QueryRow(t.Context(), `SELECT id::text FROM appointments WHERE confirm_token = $1`, sub(out, "appointment")["token"]).Scan(&id3)
	fake.addPayment("9003", "caresia-dep:"+id3, 150.00, "approved")
	anon.expect(200, "GET", "/api/public/appointments/"+sub(out, "appointment")["token"].(string), nil)
	b.exec(`UPDATE appointments SET date = current_date - 1 WHERE id = $1`, id3)
	admin.expect(200, "POST", "/api/appointments/"+id3+"/status", map[string]any{"status": "no_show"})
	_ = b.pool.QueryRow(t.Context(), `SELECT deposit_state FROM appointments WHERE id = $1`, id3).Scan(&state)
	if state != "kept" || len(fake.refunded) != 2 {
		t.Fatalf("no-show keeps the anticipo: %s %v", state, fake.refunded)
	}

	// without Mercado Pago connected nothing is asked for
	b.exec(`DELETE FROM mp_accounts WHERE clinic_id = $1`, b.clinicA)
	if code, out := book("14:00", "g@x.mx", "5577777777"); code != 201 || sub(out, "appointment")["status"] != "scheduled" {
		t.Fatalf("not connected: %d %v", code, out)
	}
}
