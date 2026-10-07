package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

func num(m map[string]any, key string) float64 { return m[key].(float64) }

func newItem(c *client, body map[string]any) string {
	c.e.t.Helper()
	out := c.expect(201, "POST", "/api/pos/items", body)
	return sub(out, "item")["id"].(string)
}

func TestPOSPlanAndRoleGate(t *testing.T) {
	e := setup(t)
	// Básico: no cobros
	e.exec(`UPDATE clinics SET plan = 'basico' WHERE id = $1`, e.clinicB)
	b := e.login("admin_b")
	if code, out := b.do("GET", "/api/pos/items", nil); code != 403 || out["code"] != "PLAN_REQUIRED" {
		t.Fatalf("basico must not reach cobros: %d %v", code, out)
	}
	// Crecimiento
	if sub(sub(b.expect(200, "GET", "/api/session", nil), "session"), "billing")["cobros"] != false {
		t.Fatal("billing.cobros must be false on basico")
	}
	for _, c := range []struct {
		user   string
		method string
		path   string
		want   int
	}{
		{"admin_a", "GET", "/api/pos/items", 200},
		{"cash_a", "GET", "/api/pos/items", 200},
		{"recep_a", "GET", "/api/pos/items", 200},
		{"doc_a", "GET", "/api/pos/items", 403},
		{"cash_a", "GET", "/api/pos/reports", 200},
		{"recep_a", "GET", "/api/pos/reports", 403},
		{"cash_a", "POST", "/api/pos/items", 403},
		{"recep_a", "GET", "/api/pos/settings", 200},
		{"cash_a", "PUT", "/api/pos/settings", 403},
	} {
		if got := status(e.login(c.user), c.method, c.path); got != c.want {
			t.Errorf("%s %s %s: got %d want %d", c.user, c.method, c.path, got, c.want)
		}
	}
}

func TestPOSSalesAndCash(t *testing.T) {
	e := setup(t)
	admin, cash := e.login("admin_a"), e.login("cash_a")

	consult := newItem(admin, map[string]any{"kind": "service", "name": "Consulta general", "price_cents": 50000})
	gauze := newItem(admin, map[string]any{"kind": "product", "name": "Gasas", "sku": "GAS-1", "barcode": "750100", "price_cents": 11600, "cost_cents": 4000,
		"tax_rate": 16, "track_stock": true, "stock": 10, "min_stock": 3})
	if code, _ := admin.do("POST", "/api/pos/items", map[string]any{"kind": "product", "name": "Otra", "sku": "gas-1", "price_cents": 100}); code != 409 {
		t.Fatalf("duplicate SKU (case-insensitive) must conflict, got %d", code)
	}

	sale := map[string]any{
		"lines":    []map[string]any{{"item_id": consult, "qty": 1}, {"item_id": gauze, "qty": 2}},
		"payments": []map[string]any{{"method": "cash", "amount_cents": 73200, "received_cents": 80000}},
	}
	// register closed
	if code, out := cash.do("POST", "/api/pos/sales", sale); code != 409 || out["code"] != "CASH_CLOSED" {
		t.Fatalf("selling with the register closed: %d %v", code, out)
	}
	cash.expect(201, "POST", "/api/pos/cash/open", map[string]any{"opening_cents": 100000})
	if code, _ := cash.do("POST", "/api/pos/cash/open", map[string]any{"opening_cents": 1}); code != 409 {
		t.Fatalf("a second open register must conflict, got %d", code)
	}

	// payments must add up
	bad := map[string]any{"lines": sale["lines"], "payments": []map[string]any{{"method": "cash", "amount_cents": 100}}}
	cash.expect(400, "POST", "/api/pos/sales", bad)
	// method not enabled
	cash.expect(400, "POST", "/api/pos/sales", map[string]any{"lines": sale["lines"], "payments": []map[string]any{{"method": "mp_point", "amount_cents": 73200}}})

	out := sub(cash.expect(201, "POST", "/api/pos/sales", sale), "sale")
	if num(out, "folio") != 1 || num(out, "total_cents") != 73200 || num(out, "tax_cents") != 3200 {
		t.Fatalf("sale totals: %v", out)
	}
	if p := out["payments"].([]any)[0].(map[string]any); num(p, "change_cents") != 6800 {
		t.Fatalf("change: %v", p)
	}
	items := admin.expect(200, "GET", "/api/pos/items?q=gasas", nil)["items"].([]any)
	if num(items[0].(map[string]any), "stock") != 8 {
		t.Fatalf("stock after sale: %v", items[0])
	}
	// barcode search
	if n := len(admin.expect(200, "GET", "/api/pos/items?q=750100", nil)["items"].([]any)); n != 1 {
		t.Fatalf("barcode search found %d", n)
	}
	// not enough stock
	code, o := cash.do("POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": gauze, "qty": 9}}, "payments": []map[string]any{{"method": "cash", "amount_cents": 104400}}})
	if code != 409 || o["code"] != "NO_STOCK" {
		t.Fatalf("overselling: %d %v", code, o)
	}

	// free line + ticket discount + split payment
	out2 := sub(cash.expect(201, "POST", "/api/pos/sales", map[string]any{
		"lines":          []map[string]any{{"name": "Certificado médico", "qty": 1, "unit_price_cents": 20000}},
		"discount_cents": 2000, "customer_name": "Ana Pérez",
		"payments": []map[string]any{{"method": "card", "amount_cents": 8000, "reference": "AUTH123"}, {"method": "transfer", "amount_cents": 10000}},
	}), "sale")
	if num(out2, "total_cents") != 18000 || num(out2, "folio") != 2 {
		t.Fatalf("second sale: %v", out2)
	}

	// movement out + register state
	cash.expect(409, "POST", "/api/pos/cash/movements", map[string]any{"kind": "out", "amount_cents": 99999999, "concept": "Demasiado"})
	cash.expect(201, "POST", "/api/pos/cash/movements", map[string]any{"kind": "out", "amount_cents": 5000, "concept": "Garrafón de agua"})
	cur := sub(cash.expect(200, "GET", "/api/pos/cash/current", nil), "session")
	// 1000.00 opening + 732.00 cash sale - 50.00 out
	if num(cur, "expected_cents") != 100000+73200-5000 || num(cur, "sales") != 2 {
		t.Fatalf("expected cash: %v", cur)
	}

	// void the first sale: stock returns, cash expectation drops
	cash.expect(403, "POST", "/api/pos/sales/"+out["id"].(string)+"/void", map[string]any{"reason": "Error"})
	admin.expect(400, "POST", "/api/pos/sales/"+out["id"].(string)+"/void", map[string]any{"reason": ""})
	admin.expect(200, "POST", "/api/pos/sales/"+out["id"].(string)+"/void", map[string]any{"reason": "Paciente se arrepintió"})
	admin.expect(409, "POST", "/api/pos/sales/"+out["id"].(string)+"/void", map[string]any{"reason": "otra vez"})
	if n := num(admin.expect(200, "GET", "/api/pos/items?q=gasas", nil)["items"].([]any)[0].(map[string]any), "stock"); n != 10 {
		t.Fatalf("stock must come back after a void, got %v", n)
	}
	cur = sub(cash.expect(200, "GET", "/api/pos/cash/current", nil), "session")
	if num(cur, "expected_cents") != 100000-5000 || num(cur, "sales") != 1 {
		t.Fatalf("voided sale must leave the register: %v", cur)
	}

	// close with a shortage
	closed := sub(cash.expect(200, "POST", "/api/pos/cash/close", map[string]any{"counted_cents": 94000, "note": "Faltan 10"}), "session")
	if num(closed, "diff_cents") != -1000 || closed["closed_at"] == nil {
		t.Fatalf("closing summary: %v", closed)
	}
	if cash.expect(200, "GET", "/api/pos/cash/current", nil)["session"] != nil {
		t.Fatal("no register should be open after closing")
	}
	cash.expect(409, "POST", "/api/pos/cash/close", map[string]any{"counted_cents": 1})
	if n := len(admin.expect(200, "GET", "/api/pos/cash/sessions", nil)["sessions"].([]any)); n != 1 {
		t.Fatalf("closed sessions: %d", n)
	}

	// report
	rep := sub(admin.expect(200, "GET", "/api/pos/reports", nil), "report")
	if num(rep, "sales") != 1 || num(rep, "void_sales") != 1 || num(rep, "total_cents") != 18000 {
		t.Fatalf("report: %v", rep)
	}
	if methods := rep["by_method"].([]any); len(methods) != 2 {
		t.Fatalf("by_method: %v", methods)
	}
	// csv
	req, _ := http.NewRequest("GET", e.srv.URL+"/api/pos/reports/sales.csv", nil)
	// reuse the admin cookie jar through a request made by its client
	res, err := admin.c.Do(withBase(req, e.srv.URL))
	if err != nil || res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Type"), "text/csv") {
		t.Fatalf("csv: %v %v", err, res)
	}
	res.Body.Close()

	// invoices
	sid := out2["id"].(string)
	inv := map[string]any{"sale_id": sid, "rfc": "XAXX010101000", "legal_name": "Ana Pérez", "zip_code": "22000", "cfdi_use": "G03", "email": "ana@x.mx"}
	cash.expect(400, "POST", "/api/pos/invoices", map[string]any{"sale_id": sid, "rfc": "mal", "legal_name": "x", "zip_code": "22000"})
	cash.expect(201, "POST", "/api/pos/invoices", inv)
	cash.expect(409, "POST", "/api/pos/invoices", inv)
	list := cash.expect(200, "GET", "/api/pos/invoices?status=pending", nil)["invoices"].([]any)
	if len(list) != 1 {
		t.Fatalf("invoices: %v", list)
	}
	iid := list[0].(map[string]any)["id"].(string)
	cash.expect(400, "PATCH", "/api/pos/invoices/"+iid, map[string]any{"status": "issued", "fiscal_uuid": "nope"})
	cash.expect(200, "PATCH", "/api/pos/invoices/"+iid, map[string]any{"status": "issued", "fiscal_uuid": "123e4567-e89b-12d3-a456-426614174000"})
	admin.expect(409, "POST", "/api/pos/sales/"+sid+"/void", map[string]any{"reason": "con factura"})

	// stock adjustments
	admin.expect(200, "POST", "/api/pos/items/"+gauze+"/stock", map[string]any{"delta": 5, "reason": "purchase"})
	admin.expect(409, "POST", "/api/pos/items/"+gauze+"/stock", map[string]any{"delta": -100, "reason": "loss"})
	admin.expect(200, "POST", "/api/pos/items/"+gauze+"/stock", map[string]any{"set_to": 12, "reason": "adjustment"})
	if n := len(admin.expect(200, "GET", "/api/pos/items/"+gauze+"/movements", nil)["movements"].([]any)); n < 5 {
		t.Fatalf("stock history has %d entries", n)
	}
	admin.expect(409, "POST", "/api/pos/items/"+consult+"/stock", map[string]any{"delta": 1, "reason": "purchase"})
	// an item that was sold is archived, not deleted
	if admin.expect(200, "DELETE", "/api/pos/items/"+consult, nil)["archived"] != true {
		t.Fatal("sold item must be archived")
	}
	fresh := newItem(admin, map[string]any{"kind": "service", "name": "Nuevo", "price_cents": 100})
	if admin.expect(200, "DELETE", "/api/pos/items/"+fresh, nil)["archived"] != false {
		t.Fatal("unsold item must be deleted")
	}

	// settings
	set := sub(admin.expect(200, "GET", "/api/pos/settings", nil), "settings")
	set["rfc"], set["methods"] = "XAXX010101000", []any{"cash", "card"}
	admin.expect(200, "PUT", "/api/pos/settings", set)
	set["rfc"] = "malo"
	admin.expect(400, "PUT", "/api/pos/settings", set)
	cash.expect(400, "POST", "/api/pos/cash/open", map[string]any{"opening_cents": -1})
}

func withBase(req *http.Request, base string) *http.Request {
	u, _ := url.Parse(base)
	req.URL.Scheme, req.URL.Host = u.Scheme, u.Host
	return req
}

func TestPOSIsolationBetweenClinics(t *testing.T) {
	e := setup(t)
	a, b := e.login("admin_a"), e.login("admin_b")
	id := newItem(a, map[string]any{"kind": "service", "name": "Solo de A", "price_cents": 100})
	if n := len(b.expect(200, "GET", "/api/pos/items", nil)["items"].([]any)); n != 0 {
		t.Fatalf("clinic B sees %d items of A", n)
	}
	b.expect(404, "PUT", "/api/pos/items/"+id, map[string]any{"kind": "service", "name": "Hack", "price_cents": 1})
	b.expect(404, "DELETE", "/api/pos/items/"+id, nil)
	b.expect(404, "POST", "/api/pos/items/"+id+"/stock", map[string]any{"delta": 1, "reason": "purchase"})
	a.expect(201, "POST", "/api/pos/cash/open", map[string]any{"opening_cents": 0})
	sid := sub(a.expect(201, "POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": id, "qty": 1}}, "payments": []map[string]any{{"method": "cash", "amount_cents": 100}}}), "sale")["id"].(string)
	b.expect(404, "GET", "/api/pos/sales/"+sid, nil)
	b.expect(404, "POST", "/api/pos/sales/"+sid+"/void", map[string]any{"reason": "x"})
	// B cannot sell A's item
	b.expect(201, "POST", "/api/pos/cash/open", map[string]any{"opening_cents": 0})
	b.expect(409, "POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": id, "qty": 1}}, "payments": []map[string]any{{"method": "cash", "amount_cents": 100}}})
}

// ---------------------------------------------------------------------------
// Mercado Pago
// ---------------------------------------------------------------------------

type fakeMP struct {
	mu               sync.Mutex
	payments         map[string]map[string]any // id -> payment
	intents          map[string]map[string]any
	prefs            int
	canceled         []string
	refunded         []string
	mode             string
	setups           int
	busy, refundFail bool
}

func newFakeMP(t *testing.T) (*fakeMP, *httptest.Server) {
	f := &fakeMP{payments: map[string]map[string]any{}, intents: map[string]map[string]any{}}
	mux := http.NewServeMux()
	reply := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("/checkout/preferences", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.prefs++
		n := f.prefs
		f.mu.Unlock()
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		reply(w, map[string]any{"id": fmt.Sprintf("pref-%d", n), "init_point": fmt.Sprintf("https://mp.test/pay/%d", n), "sandbox_init_point": "https://sandbox.mp.test/pay"})
	})
	mux.HandleFunc("/v1/payments/search", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		ref := r.URL.Query().Get("external_reference")
		res := []any{}
		for id, p := range f.payments {
			if p["external_reference"] == ref {
				res = append(res, map[string]any{"id": json.Number(id), "status": p["status"]})
			}
		}
		reply(w, map[string]any{"results": res})
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["grant_type"] == "authorization_code" && body["code"] != "good-code" {
			w.WriteHeader(400)
			reply(w, map[string]any{"message": "invalid code"})
			return
		}
		reply(w, map[string]any{"access_token": "clinic-token", "refresh_token": "clinic-refresh", "expires_in": 15552000, "user_id": 424242})
	})
	mux.HandleFunc("/terminals/v1/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer clinic-token" {
			w.WriteHeader(401)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		mode := f.mode
		if mode == "" {
			mode = "PDV"
		}
		reply(w, map[string]any{"data": map[string]any{"terminals": []any{map[string]any{"id": "PAX_A910__SN1", "operating_mode": mode, "external_pos_id": "CAJA1"}}}})
	})
	mux.HandleFunc("/terminals/v1/setup", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.setups++
		f.mode = "PDV"
		reply(w, map[string]any{})
	})
	mux.HandleFunc("/v1/orders", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.busy {
			w.WriteHeader(409)
			reply(w, map[string]any{"message": "The terminal already has an order"})
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := fmt.Sprintf("ord-%d", len(f.intents)+1)
		pays := body["transactions"].(map[string]any)["payments"].([]any)
		f.intents[id] = map[string]any{"id": id, "status": "created", "amount": pays[0].(map[string]any)["amount"], "terminal": body["config"].(map[string]any)["point"].(map[string]any)["terminal_id"]}
		w.WriteHeader(201)
		reply(w, map[string]any{"id": id, "status": "created"})
	})
	mux.HandleFunc("/v1/orders/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		rest := strings.TrimPrefix(r.URL.Path, "/v1/orders/")
		id, action, _ := strings.Cut(rest, "/")
		in, ok := f.intents[id]
		if !ok {
			w.WriteHeader(404)
			reply(w, map[string]any{"message": "order not found"})
			return
		}
		switch action {
		case "cancel":
			f.canceled = append(f.canceled, id)
			in["status"] = "canceled"
			reply(w, in)
		case "refund":
			if f.refundFail {
				w.WriteHeader(400)
				reply(w, map[string]any{"message": "refund not allowed"})
				return
			}
			f.refunded = append(f.refunded, "order:"+id)
			reply(w, map[string]any{})
		default:
			reply(w, in)
		}
	})
	mux.HandleFunc("/v1/payments/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		rest := strings.TrimPrefix(r.URL.Path, "/v1/payments/")
		if id, ok := strings.CutSuffix(rest, "/refunds"); ok && r.Method == "POST" {
			f.refunded = append(f.refunded, "payment:"+id)
			reply(w, map[string]any{})
			return
		}
		if rest == "search" {
			return
		}
		p, ok := f.payments[rest]
		if !ok {
			w.WriteHeader(404)
			reply(w, map[string]any{"message": "not found"})
			return
		}
		reply(w, p)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeMP) addPayment(id, ref string, amount float64, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.payments[id] = map[string]any{"id": json.Number(id), "status": status, "external_reference": ref, "transaction_amount": amount}
}

func TestBillingCheckoutAndWebhook(t *testing.T) {
	// not configured
	e0 := setup(t)
	if code, out := e0.login("admin_a").do("POST", "/api/billing/checkout", map[string]any{"plan": "crecimiento"}); code != 503 || out["code"] != "NOT_CONFIGURED" {
		t.Fatalf("unconfigured checkout: %d %v", code, out)
	}

	fake, srv := newFakeMP(t)
	e := setupWith(t, func(c *config.Config) {
		c.MPAccessToken, c.MPAPIBase = "platform-token", srv.URL
		c.PlanPriceMonth["crecimiento"] = 600
	})
	e.exec(`UPDATE clinics SET plan = 'basico', billing_status = 'trialing', trial_ends_at = now() - interval '1 day' WHERE id = $1`, e.clinicA)
	admin := e.login("admin_a")

	// the plan screen works even though the trial is over
	ov := admin.expect(200, "GET", "/api/billing", nil)
	offers := ov["offers"].([]any)
	if len(offers) != 3 || num(offers[1].(map[string]any), "month_cents") != 60000 || num(offers[1].(map[string]any), "year_cents") != 600000 {
		t.Fatalf("offers: %v", offers)
	}
	admin.expect(409, "POST", "/api/billing/checkout", map[string]any{"plan": "pro"}) // custom price, not sold online
	admin.expect(400, "POST", "/api/billing/checkout", map[string]any{"plan": "crecimiento", "period": "decade"})
	if code := status(e.login("doc_a"), "GET", "/api/billing"); code != 403 {
		t.Fatalf("only admins manage billing, got %d", code)
	}

	co := admin.expect(201, "POST", "/api/billing/checkout", map[string]any{"plan": "crecimiento", "period": "year"})
	id := co["id"].(string)
	if !strings.HasPrefix(co["init_point"].(string), "https://mp.test/pay/") {
		t.Fatalf("checkout: %v", co)
	}
	// still locked until paid
	if code := status(admin, "GET", "/api/appointments"); code != 403 {
		t.Fatalf("unpaid clinic must stay locked, got %d", code)
	}

	// webhook with a payment that does not belong to us / underpaid / pending → nothing happens
	post := func(paymentID string) int {
		body, _ := json.Marshal(map[string]any{"type": "payment", "data": map[string]any{"id": paymentID}})
		res, err := http.Post(e.srv.URL+"/api/webhooks/mercadopago", "application/json", strings.NewReader(string(body)))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	fake.addPayment("9001", "other:xyz", 6000, "approved")
	fake.addPayment("9002", "caresia:"+id, 10, "approved") // underpaid
	fake.addPayment("9003", "caresia:"+id, 6000, "pending")
	for _, pid := range []string{"9001", "9002", "9003", "404404"} {
		if code := post(pid); code != 200 {
			t.Fatalf("webhook %s: %d", pid, code)
		}
	}
	if st := sub(admin.expect(200, "GET", "/api/billing/checkouts/"+id, nil), "checkout")["status"]; st != "pending" {
		t.Fatalf("checkout must stay pending, got %v", st)
	}

	// approved payment
	fake.addPayment("9004", "caresia:"+id, 6000, "approved")
	if code := post("9004"); code != 200 {
		t.Fatalf("webhook: %d", code)
	}
	post("9004") // replay
	st := sub(admin.expect(200, "GET", "/api/billing/checkouts/"+id, nil), "checkout")
	if st["status"] != "paid" {
		t.Fatalf("checkout status: %v", st)
	}
	s := sub(sub(admin.expect(200, "GET", "/api/session", nil), "session"), "billing")
	if s["plan"] != "crecimiento" || s["state"] != "active" || s["cobros"] != true {
		t.Fatalf("billing after payment: %v", s)
	}
	admin.expect(200, "GET", "/api/appointments", nil)
	var n int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM payments WHERE clinic_id=$1 AND provider='mercadopago'`, e.clinicA).Scan(&n); err != nil || n != 1 {
		t.Fatalf("a replayed webhook must not duplicate the payment: %d %v", n, err)
	}

	// the return page can settle by itself if the webhook was lost
	co2 := admin.expect(201, "POST", "/api/billing/checkout", map[string]any{"plan": "crecimiento", "period": "month"})
	fake.addPayment("9005", "caresia:"+co2["id"].(string), 600, "approved")
	if st := sub(admin.expect(200, "GET", "/api/billing/checkouts/"+co2["id"].(string), nil), "checkout")["status"]; st != "paid" {
		t.Fatalf("status poll must settle a paid checkout, got %v", st)
	}
	// another clinic cannot read it
	e.login("admin_b").expect(404, "GET", "/api/billing/checkouts/"+id, nil)
}

func TestPointTerminalAndLinks(t *testing.T) {
	fake, srv := newFakeMP(t)
	e := setupWith(t, func(c *config.Config) {
		c.MPAPIBase, c.MPClientID, c.MPClientSecret = srv.URL, "client", "secret"
		c.MPAuthBase = "https://auth.mercadopago.com.mx"
	})
	admin, cash := e.login("admin_a"), e.login("cash_a")

	// not connected yet
	if st := cash.expect(200, "GET", "/api/pos/point/status", nil); st["code"] != "not_connected" || st["ok"] != false {
		t.Fatalf("status before connecting: %v", st)
	}
	if code, out := cash.do("POST", "/api/pos/point/charges", map[string]any{"amount_cents": 50000}); code != 409 || out["code"] != "not_connected" {
		t.Fatalf("charging before connecting: %d %v", code, out)
	}
	cash.expect(403, "GET", "/api/pos/point/connect", nil)
	u, err := url.Parse(admin.expect(200, "GET", "/api/pos/point/connect", nil)["url"].(string))
	if err != nil || u.Host != "auth.mercadopago.com.mx" || u.Query().Get("client_id") != "client" || u.Query().Get("redirect_uri") != "http://api.test/api/point/oauth/callback" {
		t.Fatalf("connect url: %v %v", u, err)
	}
	state := u.Query().Get("state")

	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	callback := func(q string) string {
		res, err := noRedirect.Get(e.srv.URL + "/api/point/oauth/callback?" + q)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.Header.Get("Location")
	}
	if loc := callback("code=good-code&state=" + url.QueryEscape("forged.state")); !strings.Contains(loc, "mp=error") || !strings.Contains(loc, "reason=state") {
		t.Fatalf("forged state must fail with a reason: %s", loc)
	}
	if loc := callback("code=bad-code&state=" + url.QueryEscape(state)); !strings.Contains(loc, "reason=oauth_failed") || !strings.Contains(loc, "detail=invalid+code") {
		t.Fatalf("a rejected code must say why: %s", loc)
	}
	if loc := callback("error=access_denied&state=" + url.QueryEscape(state)); !strings.Contains(loc, "reason=cancelled") {
		t.Fatalf("cancelling on Mercado Pago: %s", loc)
	}
	if loc := callback("code=good-code&state=" + url.QueryEscape(state)); loc != "http://app.test/ajustes?mp=ok&s=terminal" {
		t.Fatalf("callback: %s", loc)
	}
	// tokens are encrypted at rest
	var raw []byte
	if err := e.pool.QueryRow(t.Context(), `SELECT access_token FROM mp_accounts WHERE clinic_id=$1`, e.clinicA).Scan(&raw); err != nil || strings.Contains(string(raw), "clinic-token") {
		t.Fatalf("token must be stored encrypted: %v", err)
	}
	if sub(admin.expect(200, "GET", "/api/pos/settings", nil), "providers")["point_connected"] != true {
		t.Fatal("providers must report the connection")
	}

	// connected but no terminal chosen
	if st := cash.expect(200, "GET", "/api/pos/point/status", nil); st["code"] != "no_terminal" || st["connected"] != true {
		t.Fatalf("status without terminal: %v", st)
	}
	cash.expect(403, "GET", "/api/pos/point/terminals", nil)
	terms := admin.expect(200, "GET", "/api/pos/point/terminals", nil)["terminals"].([]any)
	if len(terms) != 1 || terms[0].(map[string]any)["label"] != "CAJA1" || terms[0].(map[string]any)["registered"] != false {
		t.Fatalf("terminals: %v", terms)
	}
	admin.expect(404, "POST", "/api/pos/point/terminal", map[string]any{"terminal_id": "OTRA"})
	admin.expect(200, "POST", "/api/pos/point/terminal", map[string]any{"terminal_id": "PAX_A910__SN1"})
	if fake.setups != 1 {
		t.Fatalf("registering must put the terminal in PDV mode, setups=%d", fake.setups)
	}
	if st := cash.expect(200, "GET", "/api/pos/point/status", nil); st["ok"] != true || st["terminal_label"] != "CAJA1" {
		t.Fatalf("ready status: %v", st)
	}
	// a terminal that drifted out of PDV mode is fixed before charging
	fake.mu.Lock()
	fake.mode = "STANDALONE"
	fake.mu.Unlock()
	cash.expect(200, "GET", "/api/pos/point/status", nil)
	if fake.setups != 2 {
		t.Fatalf("a terminal outside PDV must be switched back, setups=%d", fake.setups)
	}

	// enable the methods, sell with the terminal
	set := sub(admin.expect(200, "GET", "/api/pos/settings", nil), "settings")
	set["methods"] = []any{"cash", "card", "transfer", "mp_point", "mp_link"}
	admin.expect(200, "PUT", "/api/pos/settings", set)
	svc := newItem(admin, map[string]any{"kind": "service", "name": "Consulta", "price_cents": 50000})
	cash.expect(201, "POST", "/api/pos/cash/open", map[string]any{"opening_cents": 0})
	sale := func(intent string, amount int) (int, map[string]any) {
		return cash.do("POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": svc, "qty": 1}},
			"payments": []map[string]any{{"method": "mp_point", "amount_cents": amount, "intent_id": intent}}})
	}

	cash.expect(400, "POST", "/api/pos/point/charges", map[string]any{"amount_cents": 5})
	fake.mu.Lock()
	fake.busy = true
	fake.mu.Unlock()
	if code, out := cash.do("POST", "/api/pos/point/charges", map[string]any{"amount_cents": 50000}); code != 409 || out["code"] != "terminal_busy" {
		t.Fatalf("busy terminal: %d %v", code, out)
	}
	fake.mu.Lock()
	fake.busy = false
	fake.mu.Unlock()
	in := cash.expect(201, "POST", "/api/pos/point/charges", map[string]any{"amount_cents": 50000})
	iid := in["id"].(string)
	if code, _ := sale(iid, 50000); code != 409 {
		t.Fatalf("an unapproved terminal payment must not close a sale, got %d", code)
	}
	if st := sub(cash.expect(200, "GET", "/api/pos/charges/"+iid, nil), "charge")["status"]; st != "open" {
		t.Fatalf("status: %v", st)
	}
	// the terminal reports a different amount than asked: flagged, never approved
	fake.mu.Lock()
	fake.intents[iid]["status"] = "processed"
	fake.intents[iid]["transactions"] = map[string]any{"payments": []any{map[string]any{"id": "777", "amount": "500.00", "paid_amount": "400.00"}}}
	fake.mu.Unlock()
	if st := sub(cash.expect(200, "GET", "/api/pos/charges/"+iid, nil), "charge")["status"]; st != "error" {
		t.Fatalf("a mismatching amount must not be approved, got %v", st)
	}
	in = cash.expect(201, "POST", "/api/pos/point/charges", map[string]any{"amount_cents": 50000})
	iid = in["id"].(string)
	fake.mu.Lock()
	fake.intents[iid]["status"] = "processed"
	fake.intents[iid]["transactions"] = map[string]any{"payments": []any{map[string]any{"id": "778", "amount": "500.00", "paid_amount": "500.00"}}}
	fake.mu.Unlock()
	if st := sub(cash.expect(200, "GET", "/api/pos/charges/"+iid, nil), "charge")["status"]; st != "approved" {
		t.Fatalf("a processed order for the right amount must be approved, got %v", st)
	}
	if code, out := sale("nope", 50000); code != 400 {
		t.Fatalf("unknown intent: %d %v", code, out)
	}
	if code, out := sale(iid, 40000); code != 400 {
		t.Fatalf("a different amount must be rejected: %d %v", code, out)
	}
	code, out := sale(iid, 50000)
	if code != 201 || sub(out, "sale")["payments"].([]any)[0].(map[string]any)["reference"] != "778" {
		t.Fatalf("sale with the terminal: %d %v", code, out)
	}
	if code, _ := sale(iid, 50000); code != 409 {
		t.Fatalf("a terminal payment must be usable once, got %d", code)
	}

	// voiding the sale refunds the card; if Mercado Pago refuses, the sale stays
	sid := sub(out, "sale")["id"].(string)
	fake.mu.Lock()
	fake.refundFail = true
	fake.mu.Unlock()
	if code, o := admin.do("POST", "/api/pos/sales/"+sid+"/void", map[string]any{"reason": "Error"}); code != 502 || o["code"] != "refund_failed" {
		t.Fatalf("a refused refund must stop the void: %d %v", code, o)
	}
	if st := sub(admin.expect(200, "GET", "/api/pos/sales/"+sid, nil), "sale")["status"]; st != "paid" {
		t.Fatalf("the sale must stay paid after a failed refund, got %v", st)
	}
	fake.mu.Lock()
	fake.refundFail = false
	fake.mu.Unlock()
	admin.expect(200, "POST", "/api/pos/sales/"+sid+"/void", map[string]any{"reason": "Error"})
	if len(fake.refunded) != 1 || fake.refunded[0] != "order:"+iid {
		t.Fatalf("the order must be refunded exactly once: %v", fake.refunded)
	}

	// cancel an open charge
	in2 := cash.expect(201, "POST", "/api/pos/point/charges", map[string]any{"amount_cents": 1000})
	if st := sub(cash.expect(200, "DELETE", "/api/pos/charges/"+in2["id"].(string), nil), "charge")["status"]; st != "canceled" {
		t.Fatalf("cancel: %v", st)
	}
	if len(fake.canceled) != 1 {
		t.Fatalf("the terminal must be told to cancel: %v", fake.canceled)
	}
	cash.expect(409, "DELETE", "/api/pos/charges/"+in2["id"].(string), nil)

	// the order webhook refreshes a charge without anyone polling
	in3 := cash.expect(201, "POST", "/api/pos/point/charges", map[string]any{"amount_cents": 2000})
	fake.mu.Lock()
	fake.intents[in3["id"].(string)]["status"] = "processed"
	fake.intents[in3["id"].(string)]["transactions"] = map[string]any{"payments": []any{map[string]any{"id": "900", "amount": "20.00", "paid_amount": "20.00"}}}
	fake.mu.Unlock()
	body, _ := json.Marshal(map[string]any{"type": "order", "data": map[string]any{"id": in3["id"]}})
	res, err := http.Post(e.srv.URL+"/api/point/webhook", "application/json", strings.NewReader(string(body)))
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("order webhook: %v %v", err, res)
	}
	res.Body.Close()
	var st3 string
	_ = e.pool.QueryRow(t.Context(), `SELECT status FROM mp_charges WHERE id=$1`, in3["id"]).Scan(&st3)
	if st3 != "approved" {
		t.Fatalf("the webhook must settle the charge, got %q", st3)
	}

	// payment link
	link := cash.expect(201, "POST", "/api/pos/mp/links", map[string]any{"amount_cents": 50000, "title": "Consulta"})
	if !strings.HasPrefix(link["url"].(string), "https://mp.test/pay/") {
		t.Fatalf("link: %v", link)
	}
	lid := link["id"].(string)
	var ref string
	_ = e.pool.QueryRow(t.Context(), `SELECT external_ref FROM mp_charges WHERE id=$1`, lid).Scan(&ref)
	fake.addPayment("888", ref, 500, "approved")
	if st := sub(cash.expect(200, "GET", "/api/pos/charges/"+lid, nil), "charge")["status"]; st != "approved" {
		t.Fatalf("link status: %v", st)
	}
	cash.expect(201, "POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": svc, "qty": 1}},
		"payments": []map[string]any{{"method": "mp_link", "amount_cents": 50000, "intent_id": lid}}})

	// another clinic cannot touch these charges; disconnect works
	e.login("admin_b").expect(404, "GET", "/api/pos/charges/"+lid, nil)
	admin.expect(200, "POST", "/api/pos/point/disconnect", nil)
	if st := cash.expect(200, "GET", "/api/pos/point/status", nil); st["code"] != "not_connected" {
		t.Fatalf("after disconnecting: %v", st)
	}
}

func TestMagicInventoryAndPrices(t *testing.T) {
	var calls int
	gem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("x-goog-api-key") != "gem-key" {
			w.WriteHeader(403)
			return
		}
		var body struct {
			Contents []struct {
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		prompt := body.Contents[0].Parts[0]["text"].(string)
		text := `{"items":[{"name":"Paracetamol 500 mg","kind":"product","category":"Medicamentos","price":45.5,"cost":30,"stock":12,"unit":"caja"},{"name":"Consulta","kind":"service","price":500,"stock":99},{"name":"  ","kind":"product"}]}`
		if strings.Contains(prompt, "asesor de precios") {
			text = `{"prices":[{"i":0,"price":65,"note":"Competitivo"},{"i":1,"price":5,"note":"muy bajo, se ignora"}]}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": text}}}}}})
	}))
	defer gem.Close()

	e := setupWith(t, func(c *config.Config) {
		c.GeminiAPIKey, c.GeminiModel, c.GeminiAPIBase = "gem-key", "test-model", gem.URL
	})
	admin := e.login("admin_a")
	cash := e.login("cash_a")
	cash.expect(403, "POST", "/api/pos/magic/inventory", map[string]any{"text": "x"})
	admin.expect(400, "POST", "/api/pos/magic/inventory", map[string]any{})

	out := admin.expect(200, "POST", "/api/pos/magic/inventory", map[string]any{"text": "Paracetamol 500mg caja $45.50, 12 piezas; Consulta $500"})
	items := out["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("blank names must be dropped: %v", items)
	}
	p := items[0].(map[string]any)
	if num(p, "price_cents") != 4550 || num(p, "cost_cents") != 3000 || num(p, "stock") != 12 {
		t.Fatalf("parsed item: %v", p)
	}
	if num(items[1].(map[string]any), "stock") != 0 {
		t.Fatal("services never carry stock")
	}
	// nothing was saved by the preview
	if n := len(admin.expect(200, "GET", "/api/pos/items", nil)["items"].([]any)); n != 0 {
		t.Fatalf("preview must not save: %d", n)
	}
	// confirm: import
	imp := admin.expect(201, "POST", "/api/pos/items/import", map[string]any{"items": []map[string]any{
		{"kind": "product", "name": "Paracetamol 500 mg", "sku": "PARA", "price_cents": 4550, "cost_cents": 3000, "track_stock": true, "stock": 12},
		{"kind": "service", "name": "Consulta", "price_cents": 50000}}})
	if num(imp, "created") != 2 {
		t.Fatalf("import: %v", imp)
	}
	again := admin.expect(201, "POST", "/api/pos/items/import", map[string]any{"items": []map[string]any{{"kind": "product", "name": "Paracetamol 500 mg", "sku": "PARA", "price_cents": 1}}})
	if num(again, "created") != 0 || len(again["skipped"].([]any)) != 1 {
		t.Fatalf("duplicates must be skipped: %v", again)
	}

	// prices: AI raises the first, the too-low second suggestion is ignored in favour of the base
	pr := admin.expect(200, "POST", "/api/pos/magic/price", map[string]any{"margin_pct": 30, "items": []map[string]any{{"id": "a", "name": "Paracetamol", "cost_cents": 3000}, {"id": "b", "name": "Gasas", "cost_cents": 1000}}})
	sug := pr["suggestions"].([]any)
	if pr["ai"] != true || num(sug[0].(map[string]any), "price_cents") != 6500 {
		t.Fatalf("price suggestions: %v", pr)
	}
	if num(sug[1].(map[string]any), "price_cents") != 1500 { // 10/(1-0.3)=14.29 → $15
		t.Fatalf("base price must hold: %v", sug[1])
	}
	st := admin.expect(200, "GET", "/api/pos/magic", nil)
	if num(st, "used") != 2 || num(st, "limit") != 150 {
		t.Fatalf("usage: %v", st)
	}
	// allowance exhausted
	e.exec(`UPDATE magic_usage SET used = 150`)
	if code, o := admin.do("POST", "/api/pos/magic/inventory", map[string]any{"text": "x"}); code != 402 || o["code"] != "MAGIC_LIMIT" {
		t.Fatalf("limit: %d %v", code, o)
	}
	// without the key the price helper still works from the margin alone
	e2 := setup(t)
	pr2 := e2.login("admin_a").expect(200, "POST", "/api/pos/magic/price", map[string]any{"margin_pct": 50, "items": []map[string]any{{"id": "a", "name": "X", "cost_cents": 10000}}})
	if pr2["ai"] != false || num(pr2["suggestions"].([]any)[0].(map[string]any), "price_cents") != 20000 {
		t.Fatalf("offline prices: %v", pr2)
	}
	e2.login("admin_a").expect(503, "POST", "/api/pos/magic/inventory", map[string]any{"text": "x"})
}
