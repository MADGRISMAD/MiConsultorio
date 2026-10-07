package api_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

func day(offset int) string { return time.Now().AddDate(0, 0, offset).Format("2006-01-02") }

func (e *env) scalar(sql string, args ...any) float64 {
	e.t.Helper()
	var v float64
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		e.t.Fatal(err)
	}
	return v
}

// stockConsistent checks that the item total equals the sum of its lots.
func (e *env) stockConsistent(item string) {
	e.t.Helper()
	total := e.scalar(`SELECT stock::float8 FROM catalog_items WHERE id = $1`, item)
	lots := e.scalar(`SELECT coalesce(sum(qty), 0)::float8 FROM stock_lots WHERE item_id = $1`, item)
	if total != lots {
		e.t.Fatalf("stock %v != sum of lots %v", total, lots)
	}
}

func payCash(amount int) []map[string]any {
	return []map[string]any{{"method": "cash", "amount_cents": amount}}
}

func openCash(c *client) {
	c.expect(201, "POST", "/api/pos/cash/open", map[string]any{"opening_cents": 0})
}

func lotsOf(c *client, item string) map[string]float64 {
	c.e.t.Helper()
	out := map[string]float64{}
	for _, l := range c.expect(200, "GET", "/api/pos/items/"+item+"/lots", nil)["lots"].([]any) {
		m := l.(map[string]any)
		out[m["lot_code"].(string)] = num(m, "qty")
	}
	return out
}

func TestPOS2LotsFEFOAndExpiry(t *testing.T) {
	e := setup(t)
	admin, cash := e.login("admin_a"), e.login("cash_a")
	item := newItem(admin, map[string]any{"kind": "product", "name": "Lidocaína", "price_cents": 1000, "track_stock": true, "min_stock": 2})
	admin.expect(400, "POST", "/api/pos/items/"+item+"/stock", map[string]any{"delta": 5, "reason": "purchase", "lot_code": "X", "expires_on": day(-3)})
	admin.expect(400, "POST", "/api/pos/items/"+item+"/stock", map[string]any{"delta": 5, "reason": "purchase", "lot_code": "X", "expires_on": "mañana"})
	admin.expect(200, "POST", "/api/pos/items/"+item+"/stock", map[string]any{"delta": 5, "reason": "purchase", "lot_code": "LATE", "expires_on": day(90)})
	admin.expect(200, "POST", "/api/pos/items/"+item+"/stock", map[string]any{"delta": 5, "reason": "purchase", "lot_code": "SOON", "expires_on": day(20)})
	admin.expect(200, "POST", "/api/pos/items/"+item+"/stock", map[string]any{"delta": 2, "reason": "purchase", "lot_code": "SOON", "expires_on": day(20)}) // same lot
	admin.expect(200, "POST", "/api/pos/items/"+item+"/stock", map[string]any{"delta": 4, "reason": "purchase"})                                            // no lot
	cash.expect(403, "GET", "/api/pos/items/"+item+"/lots", nil)
	if l := lotsOf(admin, item); len(l) != 3 || l["SOON"] != 7 || l["LATE"] != 5 || l["SIN LOTE"] != 4 {
		t.Fatalf("lots: %v", l)
	}
	e.stockConsistent(item)

	// the item list shows the next expiry
	got := admin.expect(200, "GET", "/api/pos/items?q=lido", nil)["items"].([]any)[0].(map[string]any)
	if got["next_expiry"] != day(20) {
		t.Fatalf("next_expiry: %v", got["next_expiry"])
	}

	openCash(cash)
	sell := func(qty int, extra map[string]any) (int, map[string]any) {
		body := map[string]any{"lines": []map[string]any{{"item_id": item, "qty": qty}}, "payments": payCash(1000 * qty)}
		for k, v := range extra {
			body[k] = v
		}
		return cash.do("POST", "/api/pos/sales", body)
	}
	// 9 units: SOON (7) first, then LATE (2); SIN LOTE (no expiry) last
	code, out := sell(9, nil)
	if code != 201 {
		t.Fatalf("sale: %d %v", code, out)
	}
	saleID := sub(out, "sale")["id"].(string)
	if l := lotsOf(admin, item); len(l) != 2 || l["LATE"] != 3 || l["SIN LOTE"] != 4 {
		t.Fatalf("lots after FEFO sale: %v", l)
	}
	e.stockConsistent(item)
	if n := e.scalar(`SELECT count(*) FROM stock_movements WHERE sale_id = $1 AND lot_id IS NOT NULL AND reason = 'sale'`, saleID); n != 2 {
		t.Fatalf("movements must record the lots: %v", n)
	}

	// voiding returns the units to the lots they came from
	admin.expect(200, "POST", "/api/pos/sales/"+saleID+"/void", map[string]any{"reason": "prueba"})
	if l := lotsOf(admin, item); l["SOON"] != 7 || l["LATE"] != 5 || l["SIN LOTE"] != 4 {
		t.Fatalf("lots after void: %v", l)
	}
	e.stockConsistent(item)

	// an expired lot cannot be sold
	e.exec(`UPDATE stock_lots SET expires_on = $1::date WHERE item_id = $2 AND lot_code = 'SOON'`, day(-2), item)
	e.exec(`UPDATE stock_lots SET qty = 0 WHERE item_id = $1 AND lot_code <> 'SOON'`, item)
	e.exec(`UPDATE catalog_items SET stock = 7 WHERE id = $1`, item)
	code, out = sell(1, nil)
	if code != 409 || out["code"] != "LOT_EXPIRED" {
		t.Fatalf("expired sale: %d %v", code, out)
	}
	// only an administrator, with a reason
	if code, _ := sell(1, map[string]any{"allow_expired": true, "expired_reason": "Prueba"}); code != 403 {
		t.Fatalf("cashier override must be refused, got %d", code)
	}
	body := map[string]any{"lines": []map[string]any{{"item_id": item, "qty": 1}}, "payments": payCash(1000), "allow_expired": true}
	admin.expect(400, "POST", "/api/pos/sales", body) // reason required
	body["expired_reason"] = "Uso interno autorizado"
	admin.expect(201, "POST", "/api/pos/sales", body)
	if n := e.scalar(`SELECT count(*) FROM activity_log WHERE type = 'sale_expired_override'`); n != 1 {
		t.Fatalf("override must be audited, got %v", n)
	}
	e.stockConsistent(item)

	// alerts
	e.exec(`UPDATE stock_lots SET qty = 6, expires_on = $1::date WHERE item_id = $2 AND lot_code = 'LATE'`, day(15), item)
	e.exec(`UPDATE catalog_items SET stock = 12 WHERE id = $1`, item)
	al := cash.expect(200, "GET", "/api/pos/alerts?days=60", nil)
	if len(al["expired"].([]any)) != 1 || len(al["expiring"].([]any)) != 1 {
		t.Fatalf("alerts: %v", al)
	}
	cash.expect(400, "GET", "/api/pos/alerts?days=0", nil)
	e.exec(`UPDATE catalog_items SET stock = 1, min_stock = 3 WHERE id = $1`, item)
	if n := len(cash.expect(200, "GET", "/api/pos/alerts", nil)["low"].([]any)); n != 1 {
		t.Fatalf("low stock alerts: %d", n)
	}
	// clinic B sees none of it, and cannot read A's lots
	b := e.login("admin_b")
	if n := len(b.expect(200, "GET", "/api/pos/alerts", nil)["expired"].([]any)); n != 0 {
		t.Fatalf("clinic B sees %d expired lots of A", n)
	}
	if n := len(b.expect(200, "GET", "/api/pos/items/"+item+"/lots", nil)["lots"].([]any)); n != 0 {
		t.Fatalf("clinic B reads lots of A: %d", n)
	}
}

func TestPOS2PlanGate(t *testing.T) {
	e := setup(t)
	e.exec(`UPDATE clinics SET plan = 'basico' WHERE id = $1`, e.clinicA)
	a := e.login("admin_a")
	for _, p := range []string{"/api/pos/alerts", "/api/pos/receivables", "/api/pos/professionals", "/api/pos/commission-rules", "/api/pos/reports/commissions"} {
		if code, out := a.do("GET", p, nil); code != 403 || out["code"] != "PLAN_REQUIRED" {
			t.Errorf("%s on basico: %d %v", p, code, out)
		}
	}
	doc := e.login("doc_a")
	if code, out := doc.do("POST", "/api/encounters/00000000-0000-0000-0000-000000000000/consumables", map[string]any{"items": []map[string]any{{"item_id": "00000000-0000-0000-0000-000000000000", "qty": 1}}}); code != 403 || out["code"] != "PLAN_REQUIRED" {
		t.Errorf("consumables on basico: %d %v", code, out)
	}
}

func TestPOS2ConsumablesAndEncounters(t *testing.T) {
	e := setup(t)
	admin, cash, doc := e.login("admin_a"), e.login("cash_a"), e.login("doc_a")
	gauze := newItem(admin, map[string]any{"kind": "product", "name": "Gasas", "price_cents": 500, "track_stock": true, "stock": 10, "lot_code": "G1", "expires_on": day(100)})
	glove := newItem(admin, map[string]any{"kind": "product", "name": "Guantes", "price_cents": 300, "track_stock": true, "stock": 1})
	svc := newItem(admin, map[string]any{"kind": "service", "name": "Curación", "price_cents": 20000})
	other := newItem(admin, map[string]any{"kind": "service", "name": "Otro", "price_cents": 100})

	cash.expect(403, "PUT", "/api/pos/items/"+svc+"/consumables", map[string]any{"items": []map[string]any{{"product_id": gauze, "qty": 2}}})
	admin.expect(409, "PUT", "/api/pos/items/"+gauze+"/consumables", map[string]any{"items": []map[string]any{}}) // a product has none
	admin.expect(400, "PUT", "/api/pos/items/"+svc+"/consumables", map[string]any{"items": []map[string]any{{"product_id": other, "qty": 1}}})
	admin.expect(200, "PUT", "/api/pos/items/"+svc+"/consumables", map[string]any{"items": []map[string]any{{"product_id": gauze, "qty": 2}, {"product_id": glove, "qty": 2}}})
	if n := len(cash.expect(200, "GET", "/api/pos/items/"+svc+"/consumables", nil)["consumables"].([]any)); n != 2 {
		t.Fatalf("consumables: %d", n)
	}
	// B cannot use A's products as its own
	e.login("admin_b").expect(404, "PUT", "/api/pos/items/"+svc+"/consumables", map[string]any{"items": []map[string]any{}})

	openCash(cash)
	out := sub(cash.expect(201, "POST", "/api/pos/sales", map[string]any{
		"lines": []map[string]any{{"item_id": svc, "qty": 2}}, "payments": payCash(40000)}), "sale")
	if gs := e.scalar(`SELECT stock::float8 FROM catalog_items WHERE id = $1`, gauze); gs != 6 {
		t.Fatalf("gauze after 2 services: %v", gs)
	}
	// the glove supply ran short: the sale is not blocked but warns
	if w, _ := out["warnings"].([]any); len(w) != 1 {
		t.Fatalf("expected a shortage warning: %v", out["warnings"])
	}
	if e.scalar(`SELECT stock::float8 FROM catalog_items WHERE id = $1`, glove) != 0 {
		t.Fatal("glove stock should be used up")
	}
	if n := e.scalar(`SELECT count(*) FROM stock_movements WHERE sale_id = $1 AND reason = 'consumption' AND note LIKE 'Consumo de servicio Curación%'`, out["id"].(string)); n != 2 {
		t.Fatalf("consumption movements: %v", n)
	}
	e.stockConsistent(gauze)
	e.stockConsistent(glove)
	admin.expect(200, "POST", "/api/pos/sales/"+out["id"].(string)+"/void", map[string]any{"reason": "x"})
	if gs := e.scalar(`SELECT stock::float8 FROM catalog_items WHERE id = $1`, gauze); gs != 10 {
		t.Fatalf("void must return consumables: %v", gs)
	}
	e.stockConsistent(gauze)

	// supplies used in an encounter without selling them
	var patient, enc string
	if err := e.pool.QueryRow(context.Background(), `INSERT INTO patients (clinic_id, file_number, names) VALUES ($1, 1, 'Paciente') RETURNING id`, e.clinicA).Scan(&patient); err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(context.Background(), `INSERT INTO encounters (clinic_id, patient_id, kind, author_name) VALUES ($1, $2, 'procedimiento', 'x') RETURNING id`, e.clinicA, patient).Scan(&enc); err != nil {
		t.Fatal(err)
	}
	path := "/api/encounters/" + enc + "/consumables"
	items := func(q float64) map[string]any {
		return map[string]any{"items": []map[string]any{{"item_id": gauze, "qty": q}}}
	}
	cash.expect(403, "POST", path, items(1))
	doc.expect(201, "POST", path, items(3))
	doc.expect(409, "POST", path, items(100))
	doc.expect(400, "POST", path, map[string]any{"items": []map[string]any{{"item_id": svc, "qty": 1}}}) // not a product
	if e.scalar(`SELECT stock::float8 FROM catalog_items WHERE id = $1`, gauze) != 7 {
		t.Fatal("encounter consumption must leave stock at 7")
	}
	if n := len(doc.expect(200, "GET", path, nil)["consumed"].([]any)); n != 1 {
		t.Fatalf("consumed list: %d", n)
	}
	e.stockConsistent(gauze)
	e.login("doc_b").expect(404, "POST", path, items(1))
}

func TestPOS2CommissionPriority(t *testing.T) {
	e := setup(t)
	admin, cash := e.login("admin_a"), e.login("cash_a")
	docID := e.userID("doc_a")
	surg := newItem(admin, map[string]any{"kind": "service", "name": "Cirugía menor", "category": "Cirugía", "price_cents": 100000})
	surg2 := newItem(admin, map[string]any{"kind": "service", "name": "Cirugía mayor", "category": "Cirugía", "price_cents": 100000})
	consult := newItem(admin, map[string]any{"kind": "service", "name": "Consulta", "category": "Consultas", "price_cents": 100000})

	for _, r := range []map[string]any{
		{"percent": 5},                                     // everyone
		{"user_id": docID, "percent": 10},                  // this professional
		{"category": "Cirugía", "percent": 15},             // category
		{"item_id": surg, "percent": 20},                   // item
		{"item_id": surg, "user_id": docID, "percent": 25}, // item for this professional
	} {
		admin.expect(201, "POST", "/api/pos/commission-rules", r)
	}
	admin.expect(400, "POST", "/api/pos/commission-rules", map[string]any{"item_id": surg, "category": "x", "percent": 1})
	admin.expect(400, "POST", "/api/pos/commission-rules", map[string]any{"percent": 101})
	cash.expect(403, "POST", "/api/pos/commission-rules", map[string]any{"percent": 1})
	e.login("admin_b").expect(400, "POST", "/api/pos/commission-rules", map[string]any{"item_id": surg, "percent": 1}) // item of another clinic

	openCash(cash)
	sell := func(item, pro string) {
		body := map[string]any{"lines": []map[string]any{{"item_id": item, "qty": 1}}, "payments": payCash(100000)}
		if pro != "" {
			body["professional_id"] = pro
		}
		cash.expect(201, "POST", "/api/pos/sales", body)
	}
	sell(surg, docID)    // 25: item + professional
	sell(surg, "")       // 20: item
	sell(surg2, docID)   // 15: category beats professional
	sell(consult, docID) // 10: professional beats general
	sell(consult, "")    // 5: general
	want := []float64{25, 20, 15, 10, 5}
	rows := e.pool
	_ = rows
	res := admin.expect(200, "GET", "/api/pos/reports/commissions", nil)
	if num(res, "commission_cents") != (25+20+15+10+5)*1000 {
		t.Fatalf("commission total: %v", res)
	}
	lines := res["lines"].([]any)
	for i, l := range lines {
		if num(l.(map[string]any), "percent") != want[i] {
			t.Errorf("sale %d: %v%% want %v%%", i+1, num(l.(map[string]any), "percent"), want[i])
		}
	}
	// by professional
	if n := len(admin.expect(200, "GET", "/api/pos/reports/commissions?professional="+docID, nil)["lines"].([]any)); n != 3 {
		t.Fatalf("doc lines: %d", n)
	}
	if n := len(admin.expect(200, "GET", "/api/pos/reports/commissions?professional=none", nil)["lines"].([]any)); n != 2 {
		t.Fatalf("no-professional lines: %d", n)
	}
	// rules changed later do not touch what was sold
	e.exec(`UPDATE commission_rules SET percent = 99`)
	if num(admin.expect(200, "GET", "/api/pos/reports/commissions", nil), "commission_cents") != 75000 {
		t.Fatal("commission is a snapshot taken when selling")
	}
	// permissions and isolation
	cash.expect(200, "GET", "/api/pos/reports/commissions", nil)
	e.login("recep_a").expect(403, "GET", "/api/pos/reports/commissions", nil)
	if n := len(e.login("admin_b").expect(200, "GET", "/api/pos/reports/commissions", nil)["lines"].([]any)); n != 0 {
		t.Fatalf("clinic B sees %d commission lines", n)
	}
	// a professional of another clinic cannot be credited
	cash.expect(400, "POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": consult, "qty": 1}}, "payments": payCash(100000), "professional_id": e.userID("doc_b")})
	// CSV
	req, _ := http.NewRequest("GET", e.srv.URL+"/api/pos/reports/commissions.csv", nil)
	r, err := admin.c.Do(req)
	if err != nil || r.StatusCode != 200 || !strings.Contains(r.Header.Get("Content-Type"), "text/csv") {
		t.Fatalf("csv: %v %v", err, r)
	}
	r.Body.Close()
}

func TestPOS2CreditSales(t *testing.T) {
	e := setup(t)
	admin, cash, recep := e.login("admin_a"), e.login("cash_a"), e.login("recep_a")
	svc := newItem(admin, map[string]any{"kind": "service", "name": "Ortodoncia", "price_cents": 100000})
	openCash(cash)

	// on account needs somebody to charge
	cash.expect(400, "POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": svc, "qty": 1}}, "on_account": true, "payments": payCash(30000)})
	// without on_account the payments must still cover everything
	cash.expect(400, "POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": svc, "qty": 1}}, "customer_name": "Ana", "payments": payCash(30000)})
	s := sub(cash.expect(201, "POST", "/api/pos/sales", map[string]any{
		"lines": []map[string]any{{"item_id": svc, "qty": 1}}, "on_account": true, "customer_name": "Ana Pérez", "payments": payCash(30000)}), "sale")
	id := s["id"].(string)
	if s["status"] != "open" || num(s, "balance_cents") != 70000 || num(s, "paid_cents") != 30000 {
		t.Fatalf("open sale: %v", s)
	}
	cur := sub(cash.expect(200, "GET", "/api/pos/cash/current", nil), "session")
	if num(cur, "expected_cents") != 30000 {
		t.Fatalf("down payment must reach the register: %v", cur)
	}

	// cannot be invoiced while open
	inv := map[string]any{"sale_id": id, "rfc": "XAXX010101000", "legal_name": "Ana Pérez", "zip_code": "22000", "cfdi_use": "G03"}
	cash.expect(409, "POST", "/api/pos/invoices", inv)

	rec := cash.expect(200, "GET", "/api/pos/receivables", nil)
	cust := rec["customers"].([]any)
	if len(cust) != 1 || num(rec, "total_cents") != 70000 || num(cust[0].(map[string]any), "balance_cents") != 70000 {
		t.Fatalf("receivables: %v", rec)
	}
	// pos permission: reception can collect
	pay := func(c *client, want int, amount int) map[string]any {
		return c.expect(want, "POST", "/api/pos/sales/"+id+"/payments", map[string]any{"payments": payCash(amount)})
	}
	e.login("doc_a").expect(403, "POST", "/api/pos/sales/"+id+"/payments", map[string]any{"payments": payCash(100)})
	pay(cash, 400, 80000) // more than the balance
	r := sub(pay(recep, 201, 20000), "sale")
	if r["status"] != "open" || num(r, "balance_cents") != 50000 || len(r["payments"].([]any)) != 2 {
		t.Fatalf("after abono: %v", r)
	}
	if num(sub(cash.expect(200, "GET", "/api/pos/cash/current", nil), "session"), "expected_cents") != 50000 {
		t.Fatal("abono must reach the register")
	}
	// another clinic cannot touch it
	e.login("admin_b").expect(404, "POST", "/api/pos/sales/"+id+"/payments", map[string]any{"payments": payCash(100)})
	if n := len(e.login("admin_b").expect(200, "GET", "/api/pos/receivables", nil)["customers"].([]any)); n != 0 {
		t.Fatalf("clinic B sees %d receivables", n)
	}

	// void: only what was paid comes back
	v := admin.expect(200, "POST", "/api/pos/sales/"+id+"/void", map[string]any{"reason": "Dejó el tratamiento"})
	if num(v, "refund_cents") != 50000 {
		t.Fatalf("refund: %v", v)
	}
	pay(cash, 409, 100) // void sales take no abonos
	if num(sub(cash.expect(200, "GET", "/api/pos/cash/current", nil), "session"), "expected_cents") != 0 {
		t.Fatal("voided sale must leave the register")
	}
	if n := len(cash.expect(200, "GET", "/api/pos/receivables", nil)["customers"].([]any)); n != 0 {
		t.Fatalf("voided sale still in receivables: %d", n)
	}

	// settling turns the sale into paid and then it can be invoiced
	s2 := sub(cash.expect(201, "POST", "/api/pos/sales", map[string]any{
		"lines": []map[string]any{{"item_id": svc, "qty": 1}}, "on_account": true, "customer_name": "Luis", "payments": []map[string]any{}}), "sale")
	if s2["status"] != "open" || num(s2, "balance_cents") != 100000 {
		t.Fatalf("sale without down payment: %v", s2)
	}
	id = s2["id"].(string)
	done := sub(pay(cash, 201, 100000), "sale")
	if done["status"] != "paid" || num(done, "balance_cents") != 0 {
		t.Fatalf("settled: %v", done)
	}
	pay(cash, 409, 100)
	inv["sale_id"] = id
	cash.expect(201, "POST", "/api/pos/invoices", inv)
	rep := sub(admin.expect(200, "GET", "/api/pos/reports", nil), "report")
	if num(rep, "sales") != 1 || num(rep, "total_cents") != 100000 {
		t.Fatalf("report counts only settled sales: %v", rep)
	}
}

func TestPOS2AppointmentAndPlanLinking(t *testing.T) {
	e := setup(t)
	admin, cash := e.login("admin_a"), e.login("cash_a")
	svc := newItem(admin, map[string]any{"kind": "service", "name": "Limpieza", "price_cents": 50000})
	openCash(cash)
	var patient, appt string
	if err := e.pool.QueryRow(context.Background(), `INSERT INTO patients (clinic_id, file_number, names, last_names) VALUES ($1, 1, 'Marta', 'López') RETURNING id`, e.clinicA).Scan(&patient); err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(context.Background(), `INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, patient_id, professional_id)
		VALUES ($1, '', 'Marta', 'López', current_date, '10:00', '10:30', $2, $3) RETURNING id`, e.clinicA, patient, e.userID("doc_a")).Scan(&appt); err != nil {
		t.Fatal(err)
	}
	// plan tables belong to another module, but a link to unknown items never breaks the sale
	var plan, planItem string
	if err := e.pool.QueryRow(context.Background(), `INSERT INTO treatment_plans (clinic_id, patient_id, title) VALUES ($1, $2, 'Plan') RETURNING id`, e.clinicA, patient).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(context.Background(), `INSERT INTO treatment_plan_items (plan_id, clinic_id, description, qty, unit_price_cents) VALUES ($1, $2, 'Limpieza', 1, 50000) RETURNING id`, plan, e.clinicA).Scan(&planItem); err != nil {
		t.Fatal(err)
	}

	// an appointment of another clinic or a patient of another clinic is refused
	var foreign string
	if err := e.pool.QueryRow(context.Background(), `INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour) VALUES ($1, '', 'X', 'Y', current_date, '10:00', '10:30') RETURNING id`, e.clinicB).Scan(&foreign); err != nil {
		t.Fatal(err)
	}
	lines := []map[string]any{{"item_id": svc, "qty": 1}}
	cash.expect(400, "POST", "/api/pos/sales", map[string]any{"lines": lines, "payments": payCash(50000), "appointment_id": foreign})
	cash.expect(400, "POST", "/api/pos/sales", map[string]any{"lines": lines, "payments": payCash(50000), "patient_id": foreign})

	s := sub(cash.expect(201, "POST", "/api/pos/sales", map[string]any{
		"lines": lines, "payments": payCash(50000), "appointment_id": appt, "plan_item_ids": []string{planItem, "00000000-0000-0000-0000-000000000001"}}), "sale")
	if s["customer_name"] != "Marta López" || s["patient_id"] != patient || s["professional_id"] != e.userID("doc_a") {
		t.Fatalf("patient and professional must come from the appointment: %v", s)
	}
	var status, saleID string
	if err := e.pool.QueryRow(context.Background(), `SELECT status, sale_id::text FROM appointments WHERE id = $1`, appt).Scan(&status, &saleID); err != nil {
		t.Fatal(err)
	}
	if status != "completed" || saleID != s["id"] {
		t.Fatalf("appointment: %s %s", status, saleID)
	}
	var linked string
	if err := e.pool.QueryRow(context.Background(), `SELECT sale_id::text FROM treatment_plan_items WHERE id = $1`, planItem).Scan(&linked); err != nil || linked != s["id"] {
		t.Fatalf("plan item link: %v %v", linked, err)
	}
}

// ---------------------------------------------------------------------------
// CFDI stamping against a fake PAC
// ---------------------------------------------------------------------------

type fakePAC struct {
	mu        sync.Mutex
	stamps    []map[string]any
	auth      []string
	cancelled []string
	failNext  bool
}

func newFakePAC(t *testing.T) (*fakePAC, *httptest.Server) {
	f := &fakePAC{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/api-lite/3/cfdis":
			if f.failNext {
				f.failNext = false
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"Message":"La solicitud no es válida.","ModelState":{"Receiver.TaxZipCode":["El código postal no corresponde al RFC."]}}`))
				return
			}
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.stamps = append(f.stamps, body)
			_, _ = w.Write([]byte(`{"Id":"pac-id-` + string(rune('0'+len(f.stamps))) + `","Complement":{"TaxStamp":{"Uuid":"123e4567-e89b-12d3-a456-42661417400` + string(rune('0'+len(f.stamps))) + `"}}}`))
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/cfdi/xml/issuedLite/"):
			_ = json.NewEncoder(w).Encode(map[string]string{"Content": base64.StdEncoding.EncodeToString([]byte("<cfdi:Comprobante/>"))})
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/cfdi/pdf/issuedLite/"):
			_ = json.NewEncoder(w).Encode(map[string]string{"Content": base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 fake"))})
		case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/cfdi/"):
			f.cancelled = append(f.cancelled, r.URL.Path+"?"+r.URL.RawQuery)
			_, _ = w.Write([]byte(`{"Status":"canceled"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func TestPOS2CFDIStamping(t *testing.T) {
	pac, pacSrv := newFakePAC(t)
	dir := t.TempDir()
	e := setupWith(t, func(c *config.Config) {
		c.FacturamaUser, c.FacturamaPass, c.FacturamaBase = "usuario", "secreto-pac", pacSrv.URL
		c.UploadsDir = dir
	})
	admin, cash := e.login("admin_a"), e.login("cash_a")
	set := sub(admin.expect(200, "GET", "/api/pos/settings", nil), "settings")
	set["rfc"], set["legal_name"], set["tax_regime"], set["zip_code"] = "EKU9003173C9", "Clínica A SA de CV", "601 - General de Ley Personas Morales", "78220"
	admin.expect(200, "PUT", "/api/pos/settings", set)
	svc := newItem(admin, map[string]any{"kind": "service", "name": "Consulta", "price_cents": 58000, "tax_rate": 16})
	prod := newItem(admin, map[string]any{"kind": "product", "name": "Crema", "price_cents": 11600, "tax_rate": 16, "unit": "pza", "sat_product_code": "51101500", "sat_unit_code": "XBX"})
	admin.expect(400, "PUT", "/api/pos/items/"+prod, map[string]any{"kind": "product", "name": "Crema", "price_cents": 1, "sat_product_code": "123"})
	openCash(cash)
	sale := sub(cash.expect(201, "POST", "/api/pos/sales", map[string]any{
		"lines": []map[string]any{{"item_id": svc, "qty": 1}, {"item_id": prod, "qty": 1}}, "payments": payCash(69600)}), "sale")
	inv := map[string]any{"sale_id": sale["id"], "rfc": "URE180429TM6", "legal_name": "Universidad Robótica Española", "tax_regime": "601 - General de Ley Personas Morales",
		"zip_code": "65000", "cfdi_use": "G03", "email": "cliente@example.com"}
	cash.expect(201, "POST", "/api/pos/invoices", inv)
	list := cash.expect(200, "GET", "/api/pos/invoices", nil)
	if sub(list, "stamping")["enabled"] != true {
		t.Fatal("stamping should show as enabled")
	}
	iid := list["invoices"].([]any)[0].(map[string]any)["id"].(string)

	e.login("recep_a").expect(403, "POST", "/api/pos/invoices/"+iid+"/stamp", nil)
	e.login("admin_b").expect(404, "POST", "/api/pos/invoices/"+iid+"/stamp", nil)

	// a PAC refusal is shown and leaves the request pending
	pac.failNext = true
	code, out := cash.do("POST", "/api/pos/invoices/"+iid+"/stamp", nil)
	if code != 502 || !strings.Contains(out["message"].(string), "código postal") {
		t.Fatalf("PAC refusal: %d %v", code, out)
	}
	if st := e.scalar(`SELECT count(*) FROM invoice_requests WHERE id = $1 AND status = 'pending' AND cfdi_state = ''`, iid); st != 1 {
		t.Fatal("a refused stamp must leave the request pending")
	}

	res := cash.expect(200, "POST", "/api/pos/invoices/"+iid+"/stamp", nil)
	if res["fiscal_uuid"] != "123E4567-E89B-12D3-A456-426614174001" || res["emailed"] != true {
		t.Fatalf("stamp: %v", res)
	}
	cash.expect(409, "POST", "/api/pos/invoices/"+iid+"/stamp", nil) // not twice
	cash.expect(409, "PATCH", "/api/pos/invoices/"+iid, map[string]any{"status": "cancelled"})

	// what was sent to the PAC
	body := pac.stamps[0]
	if body["PaymentMethod"] != "PUE" || body["PaymentForm"] != "01" || body["ExpeditionPlace"] != "78220" || body["CfdiType"] != "I" {
		t.Fatalf("cfdi header: %v", body)
	}
	if sub(body, "Issuer")["FiscalRegime"] != "601" || sub(body, "Receiver")["TaxZipCode"] != "65000" || sub(body, "Receiver")["CfdiUse"] != "G03" {
		t.Fatalf("parties: %v", body)
	}
	items := body["Items"].([]any)
	byName := map[string]map[string]any{}
	for _, it := range items {
		m := it.(map[string]any)
		byName[m["Description"].(string)] = m
	}
	if m := byName["Consulta"]; m["ProductCode"] != "85121800" || m["UnitCode"] != "E48" || num(m, "Subtotal") != 500 || num(m, "Total") != 580 {
		t.Fatalf("service line: %v", m)
	}
	if m := byName["Crema"]; m["ProductCode"] != "51101500" || m["UnitCode"] != "XBX" || num(m, "Subtotal") != 100 {
		t.Fatalf("product line: %v", m)
	}
	for _, a := range pac.auth {
		if a != "Basic dXN1YXJpbzpzZWNyZXRvLXBhYw==" {
			t.Fatalf("authorization header: %q", a)
		}
	}

	// stored files with private permissions, and served back
	xml := filepath.Join(dir, "cfdi", e.clinicA, "123e4567-e89b-12d3-a456-426614174001.xml")
	st, err := os.Stat(xml)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("xml file: %v %v", st, err)
	}
	for ext, want := range map[string]string{"xml": "<cfdi:Comprobante/>", "pdf": "%PDF-1.4 fake"} {
		req, _ := http.NewRequest("GET", e.srv.URL+"/api/pos/invoices/"+iid+"/"+ext, nil)
		r, err := cash.c.Do(req)
		if err != nil || r.StatusCode != 200 {
			t.Fatalf("download %s: %v %v", ext, err, r)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if string(b) != want {
			t.Fatalf("%s content: %q", ext, b)
		}
	}
	e.login("admin_b").expect(404, "GET", "/api/pos/invoices/"+iid+"/xml", nil)
	// a deleted file is fetched from the PAC again
	_ = os.Remove(xml)
	req, _ := http.NewRequest("GET", e.srv.URL+"/api/pos/invoices/"+iid+"/xml", nil)
	if r, err := cash.c.Do(req); err != nil || r.StatusCode != 200 {
		t.Fatalf("re-download: %v %v", err, r)
	}

	// mailed with attachments
	m := e.mail.wait(t, 1)
	if len(m.Attachments) != 2 || m.To[0] != "cliente@example.com" {
		t.Fatalf("mail: %+v", m)
	}

	// the sale cannot be voided until the CFDI is cancelled
	admin.expect(409, "POST", "/api/pos/sales/"+sale["id"].(string)+"/void", map[string]any{"reason": "x"})
	cash.expect(403, "POST", "/api/pos/invoices/"+iid+"/cancel-cfdi", map[string]any{"motive": "02"})
	admin.expect(400, "POST", "/api/pos/invoices/"+iid+"/cancel-cfdi", map[string]any{"motive": "09"})
	admin.expect(400, "POST", "/api/pos/invoices/"+iid+"/cancel-cfdi", map[string]any{"motive": "01"}) // needs the replacement
	admin.expect(200, "POST", "/api/pos/invoices/"+iid+"/cancel-cfdi", map[string]any{"motive": "02"})
	admin.expect(409, "POST", "/api/pos/invoices/"+iid+"/cancel-cfdi", map[string]any{"motive": "02"})
	if len(pac.cancelled) != 1 || !strings.Contains(pac.cancelled[0], "motive=02") {
		t.Fatalf("cancel call: %v", pac.cancelled)
	}
	admin.expect(200, "POST", "/api/pos/sales/"+sale["id"].(string)+"/void", map[string]any{"reason": "ahora sí"})
}

func TestPOS2CFDIWithoutCredentials(t *testing.T) {
	e := setup(t)
	cash := e.login("cash_a")
	admin := e.login("admin_a")
	svc := newItem(admin, map[string]any{"kind": "service", "name": "Consulta", "price_cents": 10000})
	openCash(cash)
	sale := sub(cash.expect(201, "POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": svc, "qty": 1}}, "payments": payCash(10000)}), "sale")
	cash.expect(201, "POST", "/api/pos/invoices", map[string]any{"sale_id": sale["id"], "rfc": "XAXX010101000", "legal_name": "Público", "zip_code": "22000", "cfdi_use": "S01"})
	list := cash.expect(200, "GET", "/api/pos/invoices", nil)
	if sub(list, "stamping")["enabled"] != false {
		t.Fatal("stamping must show as not configured")
	}
	iid := list["invoices"].([]any)[0].(map[string]any)["id"].(string)
	if code, out := cash.do("POST", "/api/pos/invoices/"+iid+"/stamp", nil); code != 409 || out["code"] != "NOT_CONFIGURED" {
		t.Fatalf("stamp without credentials: %d %v", code, out)
	}
	// the manual flow keeps working
	cash.expect(200, "PATCH", "/api/pos/invoices/"+iid, map[string]any{"status": "issued", "fiscal_uuid": "123e4567-e89b-12d3-a456-426614174000"})
}
