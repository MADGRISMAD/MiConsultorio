package api_test

import (
	"strings"
	"testing"
)

func TestPOSReturnsCashStockAndLimits(t *testing.T) {
	e := setup(t)
	admin, cash := e.login("admin_a"), e.login("cash_a")
	consult := newItem(admin, map[string]any{"kind": "service", "name": "Consulta", "price_cents": 50000})
	gauze := newItem(admin, map[string]any{"kind": "product", "name": "Gasas", "price_cents": 11600, "cost_cents": 4000, "tax_rate": 16,
		"track_stock": true, "stock": 10})
	cash.expect(201, "POST", "/api/pos/cash/open", map[string]any{"opening_cents": 100000})
	stock := func() float64 {
		return num(admin.expect(200, "GET", "/api/pos/items?q=gasas", nil)["items"].([]any)[0].(map[string]any), "stock")
	}
	sale := sub(cash.expect(201, "POST", "/api/pos/sales", map[string]any{
		"lines":    []map[string]any{{"item_id": consult, "qty": 1}, {"item_id": gauze, "qty": 2}},
		"payments": []map[string]any{{"method": "cash", "amount_cents": 73200, "received_cents": 80000}}}), "sale")
	saleID := sale["id"].(string)
	var gLine, cLine string
	for _, l := range sale["lines"].([]any) {
		m := l.(map[string]any)
		if m["name"] == "Gasas" {
			gLine = m["id"].(string)
		} else {
			cLine = m["id"].(string)
		}
	}
	if stock() != 8 {
		t.Fatalf("stock after the sale: %v", stock())
	}
	url := "/api/pos/sales/" + saleID + "/returns"

	info := cash.expect(200, "GET", url, nil)
	if info["can_return"] != true || len(info["lines"].([]any)) != 2 {
		t.Fatalf("returnable info: %v", info)
	}

	// validations
	cash.expect(400, "POST", url, map[string]any{"reason": "", "lines": []map[string]any{{"sale_item_id": gLine, "qty": 1}}})
	cash.expect(400, "POST", url, map[string]any{"reason": "x", "lines": []map[string]any{}})
	cash.expect(400, "POST", url, map[string]any{"reason": "x", "lines": []map[string]any{{"sale_item_id": gLine, "qty": 0}}})
	cash.expect(400, "POST", url, map[string]any{"reason": "x", "lines": []map[string]any{{"sale_item_id": "00000000-0000-0000-0000-000000000000", "qty": 1}}})
	if code, o := cash.do("POST", url, map[string]any{"reason": "x", "lines": []map[string]any{{"sale_item_id": gLine, "qty": 3}}}); code != 409 || o["code"] != "OVER_RETURN" {
		t.Fatalf("returning more than was sold: %d %v", code, o)
	}
	if code, o := cash.do("POST", url, map[string]any{"reason": "x", "lines": []map[string]any{{"sale_item_id": gLine, "qty": 1}},
		"refunds": []map[string]any{{"method": "cash", "amount_cents": 5}}}); code != 400 || o["code"] != "REFUND_MISMATCH" {
		t.Fatalf("refund must equal the return: %d %v", code, o)
	}
	if code, o := cash.do("POST", url, map[string]any{"reason": "x", "lines": []map[string]any{{"sale_item_id": gLine, "qty": 1}},
		"refunds": []map[string]any{{"method": "card", "amount_cents": 11600}}}); code != 409 || o["code"] != "REFUND_EXCEEDS" {
		t.Fatalf("refunding through a method that was not used: %d %v", code, o)
	}

	// one gauze back, into the inventory, cash out of the register
	r1 := sub(cash.expect(201, "POST", url, map[string]any{"reason": "Vino dañada", "lines": []map[string]any{{"sale_item_id": gLine, "qty": 1}}}), "return")
	if num(r1, "total_cents") != 11600 || num(r1, "folio") != 1 || stock() != 9 {
		t.Fatalf("first return: %v stock=%v", r1, stock())
	}
	cur := sub(cash.expect(200, "GET", "/api/pos/cash/current", nil), "session")
	if num(cur, "expected_cents") != 100000+73200-11600 {
		t.Fatalf("cash returned must leave the register: %v", cur)
	}
	// not restocked (damaged): money back, units stay out
	r2 := sub(cash.expect(201, "POST", url, map[string]any{"reason": "Rota", "lines": []map[string]any{{"sale_item_id": gLine, "qty": 1, "restock": false}}}), "return")
	if num(r2, "folio") != 2 || stock() != 9 {
		t.Fatalf("a damaged unit must not return to stock: %v stock=%v", r2, stock())
	}
	if code, o := cash.do("POST", url, map[string]any{"reason": "otra", "lines": []map[string]any{{"sale_item_id": gLine, "qty": 1}}}); code != 409 || o["code"] != "OVER_RETURN" {
		t.Fatalf("both gauzes are already back: %d %v", code, o)
	}
	// the service is refunded but never restocked; with it the whole sale is back, to the cent
	r3 := sub(cash.expect(201, "POST", url, map[string]any{"reason": "No quiso la consulta", "lines": []map[string]any{{"sale_item_id": cLine, "qty": 1}}}), "return")
	if num(r3, "total_cents") != 50000 {
		t.Fatalf("service return: %v", r3)
	}
	info = cash.expect(200, "GET", url, nil)
	if num(info, "returned_cents") != 73200 || len(info["returns"].([]any)) != 3 {
		t.Fatalf("everything is back: %v", info)
	}
	if code, o := admin.do("POST", "/api/pos/sales/"+saleID+"/void", map[string]any{"reason": "x"}); code != 409 || o["code"] != "HAS_RETURNS" {
		t.Fatalf("a sale with returns cannot be voided whole: %d %v", code, o)
	}

	// reports: sales stay, returns come off
	rep := sub(admin.expect(200, "GET", "/api/pos/reports", nil), "report")
	if num(rep, "returns_cents") != 73200 || num(rep, "net_cents") != 0 || num(rep, "returns_count") != 3 {
		t.Fatalf("report must show the returns: %v", rep)
	}

	// closing the register: cash cannot go back while it is closed
	cash.expect(200, "POST", "/api/pos/cash/close", map[string]any{"counted_cents": 100000})
	cash.expect(201, "POST", "/api/pos/cash/open", map[string]any{"opening_cents": 0})
	s2 := sub(cash.expect(201, "POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": consult, "qty": 1}},
		"payments": []map[string]any{{"method": "cash", "amount_cents": 50000}}}), "sale")
	cash.expect(200, "POST", "/api/pos/cash/close", map[string]any{"counted_cents": 50000})
	l2 := s2["lines"].([]any)[0].(map[string]any)["id"].(string)
	if code, o := cash.do("POST", "/api/pos/sales/"+s2["id"].(string)+"/returns", map[string]any{"reason": "x", "lines": []map[string]any{{"sale_item_id": l2, "qty": 1}}}); code != 409 || o["code"] != "CASH_CLOSED" {
		t.Fatalf("cash back with the register closed: %d %v", code, o)
	}
}

func TestPOSReturnsSpreadTheTicketDiscountAndCommission(t *testing.T) {
	e := setup(t)
	admin, cash := e.login("admin_a"), e.login("cash_a")
	cash.expect(201, "POST", "/api/pos/cash/open", map[string]any{"opening_cents": 0})
	sale := sub(cash.expect(201, "POST", "/api/pos/sales", map[string]any{
		"lines":          []map[string]any{{"name": "Certificado", "qty": 1, "unit_price_cents": 20000}, {"name": "Consulta", "qty": 1, "unit_price_cents": 30000}},
		"discount_cents": 3333,
		"payments":       []map[string]any{{"method": "card", "amount_cents": 46667}}}), "sale")
	if num(sale, "total_cents") != 46667 {
		t.Fatalf("sale: %v", sale)
	}
	var lines []map[string]any
	for _, l := range sale["lines"].([]any) {
		lines = append(lines, map[string]any{"sale_item_id": l.(map[string]any)["id"], "qty": 1})
	}
	url := "/api/pos/sales/" + sale["id"].(string) + "/returns"
	// both lines at once: the customer gets back exactly what they paid, card to card
	r := sub(cash.expect(201, "POST", url, map[string]any{"reason": "Cancelan todo", "lines": lines}), "return")
	if num(r, "total_cents") != 46667 {
		t.Fatalf("the ticket discount must be spread so the lines add up: %v", r)
	}
	if m := r["refunds"].([]any)[0].(map[string]any)["method"]; m != "card" {
		t.Fatalf("refund goes back the way it came: %v", m)
	}
	if !strings.Contains(sale["id"].(string), "-") {
		t.Fatal("unexpected id")
	}
	// returns are visible to the clinic only (another clinic cannot see the sale)
	other := e.login("admin_b")
	if code := status(other, "GET", url); code != 404 && code != 403 {
		t.Fatalf("another clinic must not read the returns of this sale: %d", code)
	}
	_ = admin
}
