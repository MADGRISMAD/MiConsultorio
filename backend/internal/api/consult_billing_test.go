package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

// cbPatient creates a patient as the doctor and returns its id.
func cbPatient(c *client) string {
	c.e.t.Helper()
	cbSeq++
	return sub(c.expect(201, "POST", "/api/patients", person(map[string]any{"curp": "", "names": "Paciente " + strings.Repeat("x", cbSeq)})), "patient")["id"].(string)
}

var cbSeq int

func cbCharge(c *client, want int, body map[string]any) map[string]any {
	c.e.t.Helper()
	return c.expect(want, "POST", "/api/consult-charges", body)
}

func (e *env) scalarText(sql string, args ...any) string {
	e.t.Helper()
	var v string
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		e.t.Fatal(err)
	}
	return v
}

func cbStock(e *env, item string) float64 {
	return e.scalar(`SELECT stock::float8 FROM catalog_items WHERE id = $1`, item)
}

func cbSale(c *client, want int, lines []map[string]any, charge string, total int) map[string]any {
	c.e.t.Helper()
	return c.expect(want, "POST", "/api/pos/sales", map[string]any{"lines": lines, "payments": payCash(total), "consult_charge_id": charge})
}

func TestConsultChargeNoDoubleStockDeduction(t *testing.T) {
	e := setup(t)
	admin, doc, cash := e.login("admin_a"), e.login("doc_a"), e.login("cash_a")
	svc := newItem(admin, map[string]any{"kind": "service", "name": "Curación", "price_cents": 50000})
	gasa := newItem(admin, map[string]any{"kind": "product", "name": "Gasa", "price_cents": 2000, "track_stock": true, "stock": 10, "min_stock": 2})
	pat := cbPatient(doc)
	openCash(cash)

	// the doctor marks 3 gauzes as used: they leave the stock now
	out := cbCharge(doc, 201, map[string]any{"patient_id": pat, "items": []map[string]any{
		{"catalog_item_id": svc, "qty": 1}, {"catalog_item_id": gasa, "qty": 3, "consumed": true}}})
	ch := sub(out, "charge")
	id := ch["id"].(string)
	if ch["status"] != "draft" || num(ch, "total_cents") != 56000 || cbStock(e, gasa) != 7 {
		t.Fatalf("charge %v stock %v", ch, cbStock(e, gasa))
	}
	e.stockConsistent(gasa)
	if n := e.scalar(`SELECT count(*) FROM stock_movements WHERE item_id = $1 AND reason = 'consumption' AND charge_item_id IS NOT NULL`, gasa); n != 1 {
		t.Fatalf("consumption movement must point at the line: %v", n)
	}

	// not visible to the register until it is sent
	if n := len(cash.expect(200, "GET", "/api/consult-charges?status=sent", nil)["charges"].([]any)); n != 0 {
		t.Fatalf("drafts must not reach the register: %d", n)
	}
	cash.expect(404, "GET", "/api/consult-charges/"+id, nil)
	doc.expect(200, "POST", "/api/consult-charges/"+id+"/send", nil)
	list := cash.expect(200, "GET", "/api/consult-charges?status=sent", nil)["charges"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["id"] != id {
		t.Fatalf("sent list: %v", list)
	}
	if got := sub(cash.expect(200, "GET", "/api/consult-charges/"+id, nil), "charge"); len(got["items"].([]any)) != 2 {
		t.Fatalf("charge detail: %v", got)
	}

	// charging it does not take the 3 gauzes again
	sale := sub(cbSale(cash, 201, []map[string]any{{"item_id": svc, "qty": 1}, {"item_id": gasa, "qty": 3}}, id, 56000), "sale")
	if cbStock(e, gasa) != 7 {
		t.Fatalf("gauze taken twice: %v", cbStock(e, gasa))
	}
	e.stockConsistent(gasa)
	var status, saleID string
	if err := e.pool.QueryRow(context.Background(), `SELECT status, sale_id::text FROM encounter_charges WHERE id = $1`, id).Scan(&status, &saleID); err != nil || status != "charged" || saleID != sale["id"] {
		t.Fatalf("charge after sale: %v %v %v", status, saleID, err)
	}
	if sale["patient_id"] != pat {
		t.Fatalf("sale must carry the patient: %v", sale["patient_id"])
	}
	// the charge is closed
	if code, body := cash.do("POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": svc, "qty": 1}}, "payments": payCash(50000), "consult_charge_id": id}); code != 409 || body["code"] != "CHARGE_CLOSED" {
		t.Fatalf("second charge: %d %v", code, body)
	}
	// voiding the sale gives back nothing the consultation spent
	admin.expect(200, "POST", "/api/pos/sales/"+sale["id"].(string)+"/void", map[string]any{"reason": "prueba"})
	if cbStock(e, gasa) != 7 {
		t.Fatalf("void must not restock supplies spent in the consultation: %v", cbStock(e, gasa))
	}

	// a line bigger than what was consumed only takes the difference
	id2 := sub(cbCharge(doc, 201, map[string]any{"patient_id": pat, "items": []map[string]any{{"catalog_item_id": gasa, "qty": 2, "consumed": true}}}), "charge")["id"].(string)
	if cbStock(e, gasa) != 5 {
		t.Fatalf("stock: %v", cbStock(e, gasa))
	}
	doc.expect(200, "POST", "/api/consult-charges/"+id2+"/send", nil)
	cbSale(cash, 201, []map[string]any{{"item_id": gasa, "qty": 5}}, id2, 10000)
	if cbStock(e, gasa) != 2 { // 5 - (5 billed - 2 already out)
		t.Fatalf("difference only: %v", cbStock(e, gasa))
	}
	e.stockConsistent(gasa)

	// a supply used but not billed never covers another line of the same item
	id3 := sub(cbCharge(doc, 201, map[string]any{"patient_id": pat, "items": []map[string]any{
		{"catalog_item_id": svc, "qty": 1}, {"catalog_item_id": gasa, "qty": 1, "consumed": true, "no_charge": true}}}), "charge")["id"].(string)
	if cbStock(e, gasa) != 1 {
		t.Fatalf("stock: %v", cbStock(e, gasa))
	}
	doc.expect(200, "POST", "/api/consult-charges/"+id3+"/send", nil)
	cbSale(cash, 201, []map[string]any{{"item_id": svc, "qty": 1}, {"item_id": gasa, "qty": 1}}, id3, 52000)
	if cbStock(e, gasa) != 0 {
		t.Fatalf("a no-charge supply must not be credited to a billed line: %v", cbStock(e, gasa))
	}
	e.stockConsistent(gasa)
}

func TestConsultChargeLegacyEncounterConsumables(t *testing.T) {
	e := setup(t)
	admin, doc, cash := e.login("admin_a"), e.login("doc_a"), e.login("cash_a")
	gasa := newItem(admin, map[string]any{"kind": "product", "name": "Gasa", "price_cents": 2000, "track_stock": true, "stock": 10})
	pat := cbPatient(doc)
	openCash(cash)
	enc := sub(doc.expect(201, "POST", "/api/patients/"+pat+"/encounters", map[string]any{"reason": "Curación"}), "encounter")["id"].(string)

	// consumed through the existing endpoint...
	doc.expect(201, "POST", "/api/encounters/"+enc+"/consumables", map[string]any{"items": []map[string]any{{"item_id": gasa, "qty": 2}}})
	if cbStock(e, gasa) != 8 {
		t.Fatalf("stock: %v", cbStock(e, gasa))
	}
	// ...and charged through the pre-account without marking them again
	id := sub(cbCharge(doc, 201, map[string]any{"patient_id": pat, "encounter_id": enc, "items": []map[string]any{{"catalog_item_id": gasa, "qty": 2}}}), "charge")["id"].(string)
	doc.expect(200, "POST", "/api/consult-charges/"+id+"/send", nil)
	cbSale(cash, 201, []map[string]any{{"item_id": gasa, "qty": 2}}, id, 4000)
	if cbStock(e, gasa) != 8 {
		t.Fatalf("already consumed through the encounter, stock must stay: %v", cbStock(e, gasa))
	}
	// the consumption was claimed: another pre-account of the same consultation takes stock normally
	id2 := sub(cbCharge(doc, 201, map[string]any{"patient_id": pat, "encounter_id": enc, "items": []map[string]any{{"catalog_item_id": gasa, "qty": 1}}}), "charge")["id"].(string)
	doc.expect(200, "POST", "/api/consult-charges/"+id2+"/send", nil)
	cbSale(cash, 201, []map[string]any{{"item_id": gasa, "qty": 1}}, id2, 2000)
	if cbStock(e, gasa) != 7 {
		t.Fatalf("claimed consumption must not cover a second sale: %v", cbStock(e, gasa))
	}
	e.stockConsistent(gasa)

	// a charge made before the note is linked later, and its consumptions follow the note
	id3 := sub(cbCharge(doc, 201, map[string]any{"patient_id": pat, "items": []map[string]any{{"catalog_item_id": gasa, "qty": 1, "consumed": true}}}), "charge")["id"].(string)
	enc2 := sub(doc.expect(201, "POST", "/api/patients/"+pat+"/encounters", map[string]any{"reason": "Otra"}), "encounter")["id"].(string)
	line := sub(doc.expect(200, "GET", "/api/consult-charges/"+id3, nil), "charge")["items"].([]any)[0].(map[string]any)["id"]
	doc.expect(200, "PUT", "/api/consult-charges/"+id3, map[string]any{"encounter_id": enc2, "items": []map[string]any{{"id": line, "catalog_item_id": gasa, "qty": 1, "consumed": true}}})
	if n := e.scalar(`SELECT count(*) FROM stock_movements WHERE encounter_id = $1 AND charge_item_id IS NOT NULL`, enc2); n != 1 {
		t.Fatalf("movement must follow the note: %v", n)
	}
	// a consumed line cannot be dropped
	if code, out := doc.do("PUT", "/api/consult-charges/"+id3, map[string]any{"items": []map[string]any{{"name": "Otra cosa", "qty": 1}}}); code != 409 || out["code"] != "CONSUMED_LOCKED" {
		t.Fatalf("consumed line dropped: %d %v", code, out)
	}
	doc.expect(409, "PUT", "/api/consult-charges/"+id3, map[string]any{"encounter_id": enc, "items": []map[string]any{{"id": line, "catalog_item_id": gasa, "qty": 1, "consumed": true}}}) // other encounter
}

func TestConsultChargePermissionsAndIsolation(t *testing.T) {
	e := setup(t)
	admin, doc, cash, recep := e.login("admin_a"), e.login("doc_a"), e.login("cash_a"), e.login("recep_a")
	adminB, docB := e.login("admin_b"), e.login("doc_b")
	e.addUser(e.clinicA, "doc2_a", "doctor")
	doc2 := e.login("doc2_a")
	svc := newItem(admin, map[string]any{"kind": "service", "name": "Curación", "price_cents": 50000})
	pat := cbPatient(doc)
	body := map[string]any{"patient_id": pat, "items": []map[string]any{{"catalog_item_id": svc, "qty": 1}}}

	// only clinical staff builds it
	cbCharge(cash, 403, body)
	cbCharge(recep, 403, body)
	id := sub(cbCharge(doc, 201, body), "charge")["id"].(string)
	doc.expect(200, "POST", "/api/consult-charges/"+id+"/send", nil)

	// the register (pos) sees sent ones, not other doctors' drafts, and cannot edit
	recep.expect(200, "GET", "/api/consult-charges/"+id, nil)
	cash.expect(403, "PUT", "/api/consult-charges/"+id, body)
	cash.expect(403, "POST", "/api/consult-charges/"+id+"/cancel", nil)
	cash.expect(403, "GET", "/api/consult-charges/catalog", nil)
	// another doctor cannot touch it; the administrator can
	doc2.expect(404, "GET", "/api/consult-charges/"+id, nil)
	doc2.expect(403, "PUT", "/api/consult-charges/"+id, body)
	doc2.expect(403, "POST", "/api/consult-charges/"+id+"/cancel", nil)
	admin.expect(200, "PUT", "/api/consult-charges/"+id, body)

	// another clinic sees nothing and cannot use the patient, the charge or the item
	adminB.expect(404, "GET", "/api/consult-charges/"+id, nil)
	adminB.expect(404, "POST", "/api/consult-charges/"+id+"/cancel", nil)
	if n := len(adminB.expect(200, "GET", "/api/consult-charges", nil)["charges"].([]any)); n != 0 {
		t.Fatalf("clinic B list: %d", n)
	}
	cbCharge(docB, 400, body) // patient and item of clinic A
	svcB := newItem(adminB, map[string]any{"kind": "service", "name": "B", "price_cents": 1000})
	cbCharge(docB, 400, map[string]any{"patient_id": pat, "items": []map[string]any{{"catalog_item_id": svcB, "qty": 1}}})
	openCash(e.login("cash_b"))
	if code, _ := e.login("cash_b").do("POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": svcB, "qty": 1}}, "payments": payCash(1000), "consult_charge_id": id}); code != 404 {
		t.Fatalf("a foreign pre-account must not be chargeable: %d", code)
	}
	// the charge survives untouched
	if n := e.scalar(`SELECT count(*) FROM encounter_charges WHERE id = $1 AND status = 'sent'`, id); n != 1 {
		t.Fatal("charge must still be sent")
	}

	// cancelling closes it for the register
	doc.expect(200, "POST", "/api/consult-charges/"+id+"/cancel", map[string]any{"reason": "El paciente no lo requirió"})
	openCash(cash)
	if code, out := cash.do("POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": svc, "qty": 1}}, "payments": payCash(50000), "consult_charge_id": id}); code != 409 || out["code"] != "CHARGE_CLOSED" {
		t.Fatalf("cancelled charge: %d %v", code, out)
	}
	doc.expect(409, "PUT", "/api/consult-charges/"+id, body)
	cash.expect(400, "GET", "/api/consult-charges?status=nada", nil)
}

func TestConsultChargeBasicPlanHasNoPrices(t *testing.T) {
	e := setup(t)
	e.exec(`UPDATE clinics SET plan = 'basico' WHERE id = $1`, e.clinicB)
	docB := e.login("doc_b")
	pat := cbPatient(docB)

	// a plain list of concepts works, with no price
	ch := sub(cbCharge(docB, 201, map[string]any{"patient_id": pat, "items": []map[string]any{{"name": "Sutura", "qty": 1, "unit_price_cents": 99900}}}), "charge")
	if num(ch, "total_cents") != 0 || ch["status"] != "draft" {
		t.Fatalf("basic plan charge: %v", ch)
	}
	id := ch["id"].(string)
	// but no catalog, no stock, no register
	if code, out := docB.do("POST", "/api/consult-charges", map[string]any{"patient_id": pat, "items": []map[string]any{{"catalog_item_id": "00000000-0000-4000-8000-000000000001", "qty": 1}}}); code != 403 || out["code"] != "PLAN_REQUIRED" {
		t.Fatalf("catalog: %d %v", code, out)
	}
	if code, out := docB.do("POST", "/api/consult-charges", map[string]any{"patient_id": pat, "items": []map[string]any{{"name": "Gasa", "qty": 1, "consumed": true}}}); code != 403 || out["code"] != "PLAN_REQUIRED" {
		t.Fatalf("consumed: %d %v", code, out)
	}
	if code, out := docB.do("POST", "/api/consult-charges/"+id+"/send", nil); code != 403 || out["code"] != "PLAN_REQUIRED" {
		t.Fatalf("send: %d %v", code, out)
	}
	if code, out := docB.do("GET", "/api/consult-charges/catalog", nil); code != 403 || out["code"] != "PLAN_REQUIRED" {
		t.Fatalf("catalog list: %d %v", code, out)
	}
	if n := len(docB.expect(200, "GET", "/api/consult-charges?patient_id="+pat, nil)["charges"].([]any)); n != 1 {
		t.Fatalf("own list: %d", n)
	}
}

func TestConsultChargeSaleRollsBackTogether(t *testing.T) {
	e := setup(t)
	admin, doc, cash := e.login("admin_a"), e.login("doc_a"), e.login("cash_a")
	gasa := newItem(admin, map[string]any{"kind": "product", "name": "Gasa", "price_cents": 2000, "track_stock": true, "stock": 4})
	pat := cbPatient(doc)
	openCash(cash)
	id := sub(cbCharge(doc, 201, map[string]any{"patient_id": pat, "items": []map[string]any{{"catalog_item_id": gasa, "qty": 3, "consumed": true}}}), "charge")["id"].(string)
	doc.expect(200, "POST", "/api/consult-charges/"+id+"/send", nil)

	// the payments do not add up: nothing changes
	cbSale(cash, 400, []map[string]any{{"item_id": gasa, "qty": 3}}, id, 100)
	// more than the pool plus the stock: refused with no side effects
	if code, out := cash.do("POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": gasa, "qty": 9}}, "payments": payCash(18000), "consult_charge_id": id}); code != 409 || out["code"] != "NO_STOCK" {
		t.Fatalf("no stock: %d %v", code, out)
	}
	if n := e.scalar(`SELECT count(*) FROM encounter_charges WHERE id = $1 AND status = 'sent' AND sale_id IS NULL`, id); n != 1 {
		t.Fatal("a failed sale must leave the pre-account sent")
	}
	if n := e.scalar(`SELECT count(*) FROM stock_movements WHERE billed_sale_id IS NOT NULL`); n != 0 {
		t.Fatal("a failed sale must not mark consumptions as billed")
	}
	if cbStock(e, gasa) != 1 {
		t.Fatalf("stock: %v", cbStock(e, gasa))
	}
	// the patient on the sale must be the pre-account's
	other := cbPatient(doc)
	if code, _ := cash.do("POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": gasa, "qty": 3}}, "payments": payCash(6000), "consult_charge_id": id, "patient_id": other}); code != 400 {
		t.Fatalf("patient mismatch: %d", code)
	}
	// and then it works, with the stock untouched
	cbSale(cash, 201, []map[string]any{{"item_id": gasa, "qty": 3}}, id, 6000)
	if cbStock(e, gasa) != 1 || e.scalar(`SELECT count(*) FROM stock_movements WHERE billed_sale_id IS NOT NULL`) != 1 {
		t.Fatalf("stock %v", cbStock(e, gasa))
	}
}

func TestConsultChargeCatalogShowsStockAndExpiry(t *testing.T) {
	e := setup(t)
	admin, doc := e.login("admin_a"), e.login("doc_a")
	ok := newItem(admin, map[string]any{"kind": "product", "name": "Jeringa", "price_cents": 500, "track_stock": true, "stock": 5, "min_stock": 1, "lot_code": "L1", "expires_on": day(90)})
	gone := newItem(admin, map[string]any{"kind": "product", "name": "Lidocaína", "price_cents": 900, "track_stock": true, "stock": 3, "lot_code": "L2", "expires_on": day(90)})
	e.exec(`UPDATE stock_lots SET expires_on = $1::date WHERE item_id = $2`, day(-3), gone)
	byName := map[string]map[string]any{}
	for _, it := range doc.expect(200, "GET", "/api/consult-charges/catalog?kind=product", nil)["items"].([]any) {
		m := it.(map[string]any)
		byName[m["name"].(string)] = m
	}
	if m := byName["Jeringa"]; num(m, "usable_stock") != 5 || m["id"] != ok || m["stock_warning"] != nil {
		t.Fatalf("jeringa: %v", m)
	}
	if m := byName["Lidocaína"]; num(m, "usable_stock") != 0 || !strings.Contains(m["stock_warning"].(string), "caducadas") {
		t.Fatalf("lidocaína: %v", m)
	}
	// expired supplies cannot be marked as used
	pat := cbPatient(doc)
	if code, out := doc.do("POST", "/api/consult-charges", map[string]any{"patient_id": pat, "items": []map[string]any{{"catalog_item_id": gone, "qty": 1, "consumed": true}}}); code != 409 || out["code"] != "LOT_EXPIRED" {
		t.Fatalf("expired: %d %v", code, out)
	}
	if code, out := doc.do("POST", "/api/consult-charges", map[string]any{"patient_id": pat, "items": []map[string]any{{"catalog_item_id": ok, "qty": 9, "consumed": true}}}); code != 409 || out["code"] != "NO_STOCK" {
		t.Fatalf("no stock: %d %v", code, out)
	}
	if n := e.scalar(`SELECT count(*) FROM encounter_charges`); n != 0 {
		t.Fatalf("a refused charge must leave nothing behind: %v", n)
	}
}

// ---------------------------------------------------------------------------
// CFDI payment complement against the fake PAC
// ---------------------------------------------------------------------------

func TestCFDIPaymentComplement(t *testing.T) {
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
	svc := newItem(admin, map[string]any{"kind": "service", "name": "Tratamiento", "price_cents": 116000, "tax_rate": 16})
	openCash(cash)
	sale := sub(cash.expect(201, "POST", "/api/pos/sales", map[string]any{
		"lines": []map[string]any{{"item_id": svc, "qty": 1}}, "on_account": true, "customer_name": "Ana Pérez", "payments": payCash(40000)}), "sale")
	sid := sale["id"].(string)
	addPay := func(method string, amount int) {
		cash.expect(201, "POST", "/api/pos/sales/"+sid+"/payments", map[string]any{"payments": []map[string]any{{"method": method, "amount_cents": amount}}})
	}
	addPay("transfer", 36000)
	addPay("cash", 40000)
	inv := map[string]any{"sale_id": sid, "rfc": "URE180429TM6", "legal_name": "Universidad Robótica Española", "tax_regime": "601 - General de Ley Personas Morales",
		"zip_code": "65000", "cfdi_use": "G03", "email": "cliente@example.com"}
	cash.expect(201, "POST", "/api/pos/invoices", inv)
	iid := cash.expect(200, "GET", "/api/pos/invoices", nil)["invoices"].([]any)[0].(map[string]any)["id"].(string)
	if got := cash.expect(200, "GET", "/api/pos/invoices", nil)["invoices"].([]any)[0].(map[string]any); got["on_credit"] != true {
		t.Fatalf("the list must say the sale is on credit: %v", got)
	}

	// nothing to complement until the PPD invoice is stamped
	pays := func() []any {
		return cash.expect(200, "GET", "/api/pos/invoices/"+iid+"/payment-complements", nil)["payments"].([]any)
	}
	first := pays()[0].(map[string]any)["payment_id"].(string)
	cash.expect(409, "POST", "/api/pos/invoices/"+iid+"/payment-complement", map[string]any{"payment_id": first})
	cash.expect(200, "POST", "/api/pos/invoices/"+iid+"/stamp", nil)
	if pac.stamps[0]["PaymentMethod"] != "PPD" || pac.stamps[0]["PaymentForm"] != "99" {
		t.Fatalf("PPD invoice: %v", pac.stamps[0])
	}
	e.mail.wait(t, 1)

	list := pays()
	if len(list) != 3 {
		t.Fatalf("payments: %v", list)
	}
	want := [][3]float64{{116000, 40000, 76000}, {76000, 36000, 40000}, {40000, 40000, 0}}
	ids := make([]string, 3)
	for i, x := range list {
		m := x.(map[string]any)
		ids[i] = m["payment_id"].(string)
		if num(m, "installment") != float64(i+1) || num(m, "previous_cents") != want[i][0] || num(m, "amount_cents") != want[i][1] || num(m, "balance_cents") != want[i][2] || m["can_issue"] != true {
			t.Fatalf("installment %d: %v", i+1, m)
		}
	}

	// permissions and isolation
	e.login("recep_a").expect(403, "POST", "/api/pos/invoices/"+iid+"/payment-complement", map[string]any{"payment_id": ids[1]})
	e.login("admin_b").expect(404, "POST", "/api/pos/invoices/"+iid+"/payment-complement", map[string]any{"payment_id": ids[1]})
	e.login("admin_b").expect(404, "GET", "/api/pos/invoices/"+iid+"/payment-complements", nil)
	cash.expect(400, "POST", "/api/pos/invoices/"+iid+"/payment-complement", map[string]any{"payment_id": "x"})
	cash.expect(404, "POST", "/api/pos/invoices/"+iid+"/payment-complement", map[string]any{"payment_id": "00000000-0000-4000-8000-000000000001"})

	// a PAC refusal leaves no trace and can be retried
	pac.failNext = true
	if code, out := cash.do("POST", "/api/pos/invoices/"+iid+"/payment-complement", map[string]any{"payment_id": ids[1]}); code != 502 || !strings.Contains(out["message"].(string), "complemento") {
		t.Fatalf("refusal: %d %v", code, out)
	}
	if n := e.scalar(`SELECT count(*) FROM invoice_payment_complements`); n != 0 {
		t.Fatal("a refused stamp must not leave a claim")
	}

	res := cash.expect(200, "POST", "/api/pos/invoices/"+iid+"/payment-complement", map[string]any{"payment_id": ids[1]})
	if num(res, "installment") != 2 || res["emailed"] != true || !strings.HasPrefix(res["fiscal_uuid"].(string), "123E4567") {
		t.Fatalf("complement: %v", res)
	}
	cash.expect(409, "POST", "/api/pos/invoices/"+iid+"/payment-complement", map[string]any{"payment_id": ids[1]}) // not twice

	// what the PAC received
	body := pac.stamps[len(pac.stamps)-1]
	if body["CfdiType"] != "P" || sub(body, "Receiver")["CfdiUse"] != "CP01" || body["ExpeditionPlace"] != "78220" {
		t.Fatalf("header: %v", body)
	}
	pay := sub(body, "Complement")["Payments"].([]any)[0].(map[string]any)
	rel := pay["RelatedDocuments"].([]any)[0].(map[string]any)
	if pay["PaymentForm"] != "03" || num(pay, "Amount") != 360 {
		t.Fatalf("payment: %v", pay)
	}
	if rel["Uuid"] != "123E4567-E89B-12D3-A456-426614174001" || rel["PaymentMethod"] != "PPD" || num(rel, "PartialityNumber") != 2 ||
		num(rel, "PreviousBalanceAmount") != 760 || num(rel, "AmountPaid") != 360 || num(rel, "ImpSaldoInsoluto") != 400 || rel["TaxObject"] != "02" {
		t.Fatalf("related document: %v", rel)
	}
	tax := rel["Taxes"].([]any)[0].(map[string]any)
	if num(tax, "Rate") != 0.16 || num(tax, "Base") != 310.34 || num(tax, "Total") != 49.66 {
		t.Fatalf("tax of the part paid: %v", tax)
	}

	// stored and mailed with its files
	if _, err := os.Stat(filepath.Join(dir, "cfdi", e.clinicA, strings.ToLower(res["fiscal_uuid"].(string))+".xml")); err != nil {
		t.Fatalf("xml: %v", err)
	}
	m := e.mail.wait(t, 2)
	if len(m.Attachments) != 2 || !strings.Contains(m.Subject, "Complemento de pago") {
		t.Fatalf("mail: %+v", m)
	}
	cid := ""
	for _, x := range pays() {
		if mm := x.(map[string]any); mm["payment_id"] == ids[1] {
			cid = mm["complement_id"].(string)
			if mm["state"] != "stamped" || mm["can_issue"] != false || mm["complement_uuid"] != res["fiscal_uuid"] {
				t.Fatalf("listed complement: %v", mm)
			}
		}
	}
	for ext, want := range map[string]string{"xml": "<cfdi:Comprobante/>", "pdf": "%PDF-1.4 fake"} {
		resp, b := cash.raw("GET", "/api/pos/invoices/"+iid+"/payment-complements/"+cid+"/"+ext)
		if resp.StatusCode != 200 || string(b) != want {
			t.Fatalf("download %s: %d %q", ext, resp.StatusCode, b)
		}
	}
	if resp, _ := e.login("admin_b").raw("GET", "/api/pos/invoices/"+iid+"/payment-complements/"+cid+"/xml"); resp.StatusCode != 404 {
		t.Fatalf("foreign download: %d", resp.StatusCode)
	}
}

func TestCFDIPaymentComplementOnlyForSalesOnAccount(t *testing.T) {
	_, pacSrv := newFakePAC(t)
	e := setupWith(t, func(c *config.Config) {
		c.FacturamaUser, c.FacturamaPass, c.FacturamaBase = "usuario", "secreto-pac", pacSrv.URL
		c.UploadsDir = t.TempDir()
	})
	admin, cash := e.login("admin_a"), e.login("cash_a")
	set := sub(admin.expect(200, "GET", "/api/pos/settings", nil), "settings")
	set["rfc"], set["legal_name"], set["tax_regime"], set["zip_code"] = "EKU9003173C9", "Clínica A SA de CV", "601 - General de Ley Personas Morales", "78220"
	admin.expect(200, "PUT", "/api/pos/settings", set)
	svc := newItem(admin, map[string]any{"kind": "service", "name": "Consulta", "price_cents": 58000, "tax_rate": 16})
	openCash(cash)
	sale := sub(cash.expect(201, "POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": svc, "qty": 1}}, "payments": payCash(58000)}), "sale")
	cash.expect(201, "POST", "/api/pos/invoices", map[string]any{"sale_id": sale["id"], "rfc": "URE180429TM6", "legal_name": "Universidad", "tax_regime": "601 - General de Ley Personas Morales", "zip_code": "65000", "cfdi_use": "G03"})
	iid := cash.expect(200, "GET", "/api/pos/invoices", nil)["invoices"].([]any)[0].(map[string]any)["id"].(string)
	cash.expect(200, "POST", "/api/pos/invoices/"+iid+"/stamp", nil)
	if got := cash.expect(200, "GET", "/api/pos/invoices/"+iid+"/payment-complements", nil); got["on_credit"] != false || len(got["payments"].([]any)) != 0 {
		t.Fatalf("a PUE sale has no complements: %v", got)
	}
	pid := e.scalarText(`SELECT id::text FROM sale_payments WHERE sale_id = $1`, sale["id"])
	cash.expect(409, "POST", "/api/pos/invoices/"+iid+"/payment-complement", map[string]any{"payment_id": pid})
}

func TestCFDIPaymentComplementWithoutCredentials(t *testing.T) {
	e := setup(t)
	cash := e.login("cash_a")
	cash.expect(404, "GET", "/api/pos/invoices/00000000-0000-4000-8000-000000000001/payment-complements", nil)
	if code, out := cash.do("POST", "/api/pos/invoices/00000000-0000-4000-8000-000000000001/payment-complement", map[string]any{"payment_id": "00000000-0000-4000-8000-000000000002"}); code != 409 || out["code"] != "NOT_CONFIGURED" {
		t.Fatalf("without credentials: %d %v", code, out)
	}
}
