package api_test

import (
	"context"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/fieldcrypt"
)

func labOrderURL(id string) string { return "/api/lab/orders/" + id }

func labResultByAnalyte(t *testing.T, order map[string]any, analyte string) map[string]any {
	t.Helper()
	for _, x := range order["results"].([]any) {
		m := x.(map[string]any)
		if m["analyte"] == analyte && m["superseded_by"] == nil {
			return m
		}
	}
	t.Fatalf("no current result for %s in %v", analyte, order["results"])
	return nil
}

func TestLabOrdersFlagsAndPermissions(t *testing.T) {
	e := setup(t)
	doc, admin, recep, cash, other := e.login("doc_a"), e.login("admin_a"), e.login("recep_a"), e.login("cash_a"), e.login("doc_b")
	pid := newPerson(t, doc, "mejj700312hdfdrr04") // Hombre
	base := "/api/patients/" + pid + "/lab/orders"

	cat := doc.expect(200, "GET", "/api/lab/catalog", nil)
	if cat["notice"] == "" || len(cat["panels"].([]any)) < 8 {
		t.Fatalf("catalog: %v", cat)
	}
	recep.expect(403, "GET", "/api/lab/catalog", nil)
	e.anon().expect(401, "GET", "/api/lab/catalog", nil)

	doc.expect(400, "POST", base, map[string]any{"title": ""})
	doc.expect(400, "POST", base, map[string]any{"title": "BH", "results": []any{map[string]any{"analyte": "Hemoglobina"}}})
	doc.expect(400, "POST", base, map[string]any{"title": "BH", "results": []any{map[string]any{"analyte": "X", "value_num": 1, "ref_low": 5, "ref_high": 2}}})
	doc.expect(400, "POST", base, map[string]any{"title": "BH", "attachment_id": "00000000-0000-4000-8000-000000000000"})

	out := sub(doc.expect(201, "POST", base, map[string]any{
		"title": "Biometría hemática y química", "lab_name": "Lab Central", "notes": "Ayuno de 8 h",
		"results": []any{
			map[string]any{"panel": "bh", "analyte": "Hemoglobina", "value_num": 18.5},                                    // catalog range of men: high
			map[string]any{"panel": "bh", "analyte": "Leucocitos", "value_num": 2.0, "flag": "normal"},                    // client flag ignored: low
			map[string]any{"panel": "bh", "analyte": "Plaquetas", "value_num": 250},                                       // normal
			map[string]any{"panel": "qs6", "analyte": "Glucosa en ayuno", "value_num": 85, "ref_low": 60, "ref_high": 80}, // own range prevails: high
			map[string]any{"panel": "qs6", "analyte": "Colesterol total", "value_num": 400, "flag": "critico"},            // escalated
			map[string]any{"panel": "otro", "analyte": "Marcador propio", "value_num": 7},                                 // no range anywhere
			map[string]any{"panel": "ego", "analyte": "Color", "value_text": "Amarillo"},
			map[string]any{"panel": "ego", "analyte": "Sangre", "value_text": "Positivo", "flag": "anormal"},
		},
	}), "order")
	if out["status"] != "parcial" {
		t.Fatalf("status: %v", out["status"])
	}
	check := func(analyte, flag, source string) {
		t.Helper()
		x := labResultByAnalyte(t, out, analyte)
		if x["flag"] != flag || x["ref_source"] != source {
			t.Fatalf("%s: flag %v source %v, want %s/%s", analyte, x["flag"], x["ref_source"], flag, source)
		}
	}
	check("Hemoglobina", "alto", "catalogo")
	check("Leucocitos", "bajo", "catalogo")
	check("Plaquetas", "normal", "catalogo")
	check("Glucosa en ayuno", "alto", "laboratorio")
	check("Colesterol total", "critico", "catalogo")
	check("Marcador propio", "na", "")
	check("Color", "na", "")
	check("Sangre", "anormal", "")
	if u := labResultByAnalyte(t, out, "Plaquetas")["unit"]; u != "x10^3/µL" {
		t.Fatalf("unit from catalog: %v", u)
	}

	// Notes are sealed at rest and readable through the API.
	var raw string
	if err := e.pool.QueryRow(context.Background(), `SELECT notes FROM lab_orders WHERE id = $1`, out["id"]).Scan(&raw); err != nil || !fieldcrypt.IsSealed(raw) {
		t.Fatalf("notes at rest: %q %v", raw, err)
	}
	if out["notes"] != "Ayuno de 8 h" {
		t.Fatalf("notes: %v", out["notes"])
	}

	id := out["id"].(string)
	list := doc.expect(200, "GET", base, nil)["orders"].([]any)
	if len(list) != 1 || len(list[0].(map[string]any)["results"].([]any)) != 8 {
		t.Fatalf("list: %v", list)
	}
	var n int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM record_access WHERE patient_id = $1 AND action = 'view'`, pid).Scan(&n)
	if n == 0 {
		t.Fatal("reading lab results was not recorded in record_access")
	}
	doc.expect(200, "GET", labOrderURL(id), nil)
	admin.expect(200, "GET", base, nil)

	// permissions and isolation
	for _, c := range []*client{recep, cash} {
		c.expect(403, "GET", base, nil)
		c.expect(403, "POST", base, map[string]any{"title": "x"})
		c.expect(403, "GET", labOrderURL(id), nil)
		c.expect(403, "POST", labOrderURL(id)+"/results", map[string]any{"results": []any{map[string]any{"analyte": "a", "value_num": 1}}})
		c.expect(403, "GET", "/api/patients/"+pid+"/lab-trends", nil)
	}
	other.expect(404, "GET", base, nil)
	other.expect(404, "POST", base, map[string]any{"title": "x"})
	other.expect(404, "GET", labOrderURL(id), nil)
	other.expect(404, "PUT", labOrderURL(id), map[string]any{"title": "x"})
	other.expect(404, "POST", labOrderURL(id)+"/status", map[string]any{"status": "cancelado", "reason": "x"})
	other.expect(404, "POST", labOrderURL(id)+"/results", map[string]any{"results": []any{map[string]any{"analyte": "a", "value_num": 1}}})
	other.expect(404, "GET", "/api/patients/"+pid+"/lab-trends", nil)
	e.anon().expect(401, "GET", base, nil)
}

func TestLabSupersedesAndAppendOnly(t *testing.T) {
	e := setup(t)
	doc, other := e.login("doc_a"), e.login("doc_b")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	base := "/api/patients/" + pid + "/lab/orders"
	trends := "/api/patients/" + pid + "/lab-trends?analyte="

	order := sub(doc.expect(201, "POST", base, map[string]any{"title": "Química", "results": []any{
		map[string]any{"panel": "qs6", "analyte": "Glucosa en ayuno", "value_num": 150, "resulted_at": "2026-01-10T08:00:00Z"},
		map[string]any{"panel": "qs6", "analyte": "Urea", "value_num": 30},
	}}), "order")
	oid := order["id"].(string)
	wrong := labResultByAnalyte(t, order, "Glucosa en ayuno")["id"].(string)
	urea := labResultByAnalyte(t, order, "Urea")["id"].(string)
	res := labOrderURL(oid) + "/results"

	// A correction needs the reason, the same analyte and an existing result of the same order.
	doc.expect(400, "POST", res, map[string]any{"results": []any{map[string]any{"analyte": "Glucosa en ayuno", "value_num": 105, "supersedes_id": wrong}}})
	doc.expect(400, "POST", res, map[string]any{"results": []any{map[string]any{"analyte": "Glucosa en ayuno", "value_num": 105, "supersedes_id": urea, "notes": "x"}}})
	doc.expect(400, "POST", res, map[string]any{"results": []any{map[string]any{"analyte": "Glucosa en ayuno", "value_num": 105, "supersedes_id": "00000000-0000-4000-8000-000000000000", "notes": "x"}}})
	other.expect(404, "POST", res, map[string]any{"results": []any{map[string]any{"analyte": "Glucosa en ayuno", "value_num": 105, "supersedes_id": wrong, "notes": "x"}}})

	fixed := sub(doc.expect(201, "POST", res, map[string]any{"results": []any{
		map[string]any{"panel": "qs6", "analyte": "Glucosa en ayuno", "value_num": 105, "supersedes_id": wrong, "notes": "Error de captura: era 105", "resulted_at": "2026-01-10T08:00:00Z"},
	}}), "order")
	var oldRow, newRow map[string]any
	for _, x := range fixed["results"].([]any) {
		m := x.(map[string]any)
		if m["id"] == wrong {
			oldRow = m
		}
		if m["supersedes_id"] == wrong {
			newRow = m
		}
	}
	if oldRow == nil || newRow == nil || oldRow["superseded_by"] != newRow["id"] || oldRow["value_num"].(float64) != 150 || newRow["flag"] != "alto" {
		t.Fatalf("history: old %v new %v", oldRow, newRow)
	}
	if len(fixed["results"].([]any)) != 3 {
		t.Fatalf("the old value must stay in the history: %v", fixed["results"])
	}
	// The same result cannot be corrected twice; the correction is what gets corrected next.
	doc.expect(409, "POST", res, map[string]any{"results": []any{map[string]any{"analyte": "Glucosa en ayuno", "value_num": 99, "supersedes_id": wrong, "notes": "otra vez"}}})
	doc.expect(201, "POST", res, map[string]any{"results": []any{map[string]any{"analyte": "Glucosa en ayuno", "value_num": 99, "supersedes_id": newRow["id"], "notes": "Segunda corrección"}}})

	// Trends use only the current version.
	tr := doc.expect(200, "GET", trends+"glucosa%20EN%20ayuno", nil)
	pts := tr["points"].([]any)
	if len(pts) != 1 || pts[0].(map[string]any)["value"].(float64) != 99 {
		t.Fatalf("trend: %v", pts)
	}
	if len(tr["analytes"].([]any)) != 2 {
		t.Fatalf("analytes: %v", tr["analytes"])
	}
	if len(doc.expect(200, "GET", trends+"Inexistente", nil)["points"].([]any)) != 0 {
		t.Fatal("unknown analyte has points")
	}

	// The database refuses edits and deletes of the values.
	ctx := context.Background()
	if _, err := e.pool.Exec(ctx, `UPDATE lab_results SET value_num = 1 WHERE id = $1`, wrong); err == nil {
		t.Fatal("a lab result was edited")
	}
	if _, err := e.pool.Exec(ctx, `UPDATE lab_results SET flag = 'normal' WHERE id = $1`, wrong); err == nil {
		t.Fatal("a lab flag was edited")
	}
	// There is no route to delete results or orders.
	for _, p := range []string{labOrderURL(oid), res} {
		if status, _ := doc.do("DELETE", p, nil); status < 400 {
			t.Fatalf("DELETE %s answered %d", p, status)
		}
	}
	// Notes may be re-sealed (key rotation) but nothing else.
	if _, err := e.pool.Exec(ctx, `UPDATE lab_results SET notes = notes WHERE id = $1`, wrong); err != nil {
		t.Fatalf("notes-only update: %v", err)
	}
}

func TestLabOrderStatusAndAttachment(t *testing.T) {
	e := setup(t)
	doc, other := e.login("doc_a"), e.login("doc_b")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	pid2 := sub(doc.expect(201, "POST", "/api/patients/", person(map[string]any{"curp": "gomm800101mdfrrr09", "names": "Ana"})), "patient")["id"].(string)
	otherPid := newPerson(t, other, "mejj700312hdfdrr04")
	base := "/api/patients/" + pid + "/lab/orders"
	ctx := context.Background()

	att := func(clinic, patient string) string {
		var id string
		err := e.pool.QueryRow(ctx, `INSERT INTO attachments (clinic_id, patient_id, kind, title, mime, size_bytes, sha256, storage_key)
			VALUES ($1,$2,'lab','Resultado PDF','application/pdf',10,'x', gen_random_uuid()::text) RETURNING id::text`, clinic, patient).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	mine, forOther, foreign := att(e.clinicA, pid), att(e.clinicA, pid2), att(e.clinicB, otherPid)

	doc.expect(400, "POST", base, map[string]any{"title": "BH", "attachment_id": forOther}) // another patient's file
	doc.expect(400, "POST", base, map[string]any{"title": "BH", "attachment_id": foreign})  // another clinic's file
	o := sub(doc.expect(201, "POST", base, map[string]any{"title": "BH", "attachment_id": mine}), "order")
	if o["attachment_id"] != mine || o["status"] != "solicitado" {
		t.Fatalf("order: %v", o)
	}
	oid := o["id"].(string)
	doc.expect(400, "PUT", labOrderURL(oid), map[string]any{"title": "BH", "attachment_id": foreign})
	up := sub(doc.expect(200, "PUT", labOrderURL(oid), map[string]any{"title": "BH completa", "lab_name": "Otro lab", "notes": "x", "attachment_id": ""}), "order")
	if up["title"] != "BH completa" || up["attachment_id"] != nil {
		t.Fatalf("update: %v", up)
	}

	// Status rules.
	doc.expect(400, "POST", labOrderURL(oid)+"/status", map[string]any{"status": "magia"})
	doc.expect(409, "POST", labOrderURL(oid)+"/status", map[string]any{"status": "completo"}) // no results yet
	doc.expect(400, "POST", labOrderURL(oid)+"/status", map[string]any{"status": "cancelado"})
	doc.expect(400, "POST", labOrderURL(oid)+"/status", map[string]any{"status": "cancelado", "reason": "  "})
	doc.expect(201, "POST", labOrderURL(oid)+"/results", map[string]any{"results": []any{map[string]any{"panel": "bh", "analyte": "Plaquetas", "value_num": 250}}})
	if s := sub(doc.expect(200, "GET", labOrderURL(oid), nil), "order")["status"]; s != "parcial" {
		t.Fatalf("after first result: %v", s)
	}
	doc.expect(409, "POST", labOrderURL(oid)+"/status", map[string]any{"status": "parcial"})
	if s := sub(doc.expect(200, "POST", labOrderURL(oid)+"/status", map[string]any{"status": "completo"}), "order")["status"]; s != "completo" {
		t.Fatalf("complete: %v", s)
	}

	// A cancelled order keeps its results but leaves the trends, and accepts nothing more.
	c := sub(doc.expect(200, "POST", labOrderURL(oid)+"/status", map[string]any{"status": "cancelado", "reason": "Orden duplicada"}), "order")
	if c["status"] != "cancelado" || c["cancel_reason"] != "Orden duplicada" || c["cancelled_at"] == nil || len(c["results"].([]any)) != 1 {
		t.Fatalf("cancelled: %v", c)
	}
	doc.expect(409, "POST", labOrderURL(oid)+"/results", map[string]any{"results": []any{map[string]any{"analyte": "Plaquetas", "value_num": 1}}})
	doc.expect(409, "PUT", labOrderURL(oid), map[string]any{"title": "otra"})
	doc.expect(409, "POST", labOrderURL(oid)+"/status", map[string]any{"status": "completo"})
	if len(doc.expect(200, "GET", "/api/patients/"+pid+"/lab-trends?analyte=Plaquetas", nil)["points"].([]any)) != 0 {
		t.Fatal("a cancelled order shows in the trends")
	}
	if _, err := e.pool.Exec(ctx, `UPDATE lab_orders SET status = 'completo' WHERE id = $1`, oid); err == nil {
		t.Fatal("a cancelled order was reopened")
	}

	// An archived record takes no new orders.
	doc.expect(200, "POST", "/api/patients/"+pid2+"/archive", map[string]any{"reason": "Prueba"})
	doc.expect(409, "POST", "/api/patients/"+pid2+"/lab/orders", map[string]any{"title": "BH"})

	// The audit trail names what happened.
	var n int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM activity_log WHERE clinic_id = $1 AND type IN ('lab_order_create','lab_results_add','lab_order_status','lab_order_update')`, e.clinicA).Scan(&n)
	if n < 5 {
		t.Fatalf("audit entries: %d", n)
	}
}
