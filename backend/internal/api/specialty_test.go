package api_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

func signaturePNG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, h/2, color.Black)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func newPerson(t *testing.T, c *client, curp string) string {
	t.Helper()
	x := sub(c.expect(201, "POST", "/api/patients/", person(map[string]any{"curp": curp})), "patient")
	return x["id"].(string)
}

func TestVaccinations(t *testing.T) {
	e := setup(t)
	doc, recep, other := e.login("doc_a"), e.login("recep_a"), e.login("doc_b")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	today := time.Now()
	day := func(n int) string { return today.AddDate(0, 0, n).Format("2006-01-02") }

	doc.expect(400, "POST", "/api/patients/"+pid+"/vaccinations", map[string]any{"name": "", "applied_on": day(0)})
	doc.expect(400, "POST", "/api/patients/"+pid+"/vaccinations", map[string]any{"name": "Td", "applied_on": day(30)})
	doc.expect(400, "POST", "/api/patients/"+pid+"/vaccinations", map[string]any{"name": "Td", "applied_on": day(-1), "next_due": day(-5)})
	doc.expect(400, "POST", "/api/patients/"+pid+"/vaccinations", map[string]any{"name": "Td", "kind": "magia", "applied_on": day(-1)})

	old := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/vaccinations", map[string]any{"name": "Influenza", "applied_on": day(-370), "next_due": day(-5), "lot": "A1"}), "vaccination")
	// A newer application of the same vaccine replaces the overdue one.
	doc.expect(201, "POST", "/api/patients/"+pid+"/vaccinations", map[string]any{"name": "Td", "applied_on": day(-100), "next_due": day(10)})
	doc.expect(201, "POST", "/api/patients/"+pid+"/vaccinations", map[string]any{"name": "Hepatitis B", "applied_on": day(-10), "next_due": day(200)})

	out := doc.expect(200, "GET", "/api/patients/"+pid+"/vaccinations", nil)
	if len(out["vaccinations"].([]any)) != 3 || len(out["suggestions"].([]any)) == 0 {
		t.Fatalf("list: %v", out)
	}
	due := doc.expect(200, "GET", "/api/vaccinations/due?days=30", nil)["due"].([]any)
	if len(due) != 2 {
		t.Fatalf("due in 30 days: %v", due)
	}
	first := due[0].(map[string]any)
	if first["name"] != "Influenza" || first["overdue"] != true || first["phone"] == "" {
		t.Fatalf("first due: %v", first)
	}
	recep.expect(200, "GET", "/api/vaccinations/due", nil)
	other.expect(200, "GET", "/api/vaccinations/due?days=730", nil)
	if n := len(other.expect(200, "GET", "/api/vaccinations/due?days=730", nil)["due"].([]any)); n != 0 {
		t.Fatalf("another clinic sees %d", n)
	}
	doc.expect(400, "GET", "/api/vaccinations/due?days=-1", nil)

	// Re-vaccinating the overdue one removes it from the list.
	doc.expect(201, "POST", "/api/patients/"+pid+"/vaccinations", map[string]any{"name": "influenza", "applied_on": day(0), "next_due": day(365)})
	if n := len(doc.expect(200, "GET", "/api/vaccinations/due?days=30", nil)["due"].([]any)); n != 1 {
		t.Fatalf("due after revaccination: %d", n)
	}

	// Void: needs a reason, only once, other clinics and reception cannot, and a voided row is not due.
	vid := old["id"].(string)
	doc.expect(400, "POST", "/api/vaccinations/"+vid+"/void", map[string]any{"reason": ""})
	recep.expect(403, "POST", "/api/vaccinations/"+vid+"/void", map[string]any{"reason": "x"})
	other.expect(404, "POST", "/api/vaccinations/"+vid+"/void", map[string]any{"reason": "x"})
	doc.expect(200, "POST", "/api/vaccinations/"+vid+"/void", map[string]any{"reason": "Capturada en el paciente equivocado"})
	doc.expect(404, "POST", "/api/vaccinations/"+vid+"/void", map[string]any{"reason": "otra vez"})
	// The database refuses edits and deletes of the card through any path other than voiding.
	if _, err := e.pool.Exec(t.Context(), `UPDATE vaccinations SET name = 'X' WHERE id = $1`, vid); err == nil {
		t.Fatal("a vaccination was edited")
	}

	// permissions and isolation
	recep.expect(403, "GET", "/api/patients/"+pid+"/vaccinations", nil)
	recep.expect(403, "POST", "/api/patients/"+pid+"/vaccinations", map[string]any{"name": "Td", "applied_on": day(0)})
	other.expect(404, "GET", "/api/patients/"+pid+"/vaccinations", nil)
	other.expect(404, "POST", "/api/patients/"+pid+"/vaccinations", map[string]any{"name": "Td", "applied_on": day(0)})
	e.anon().expect(401, "GET", "/api/vaccinations/due", nil)
}

func TestWeightsFromEncounters(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "Control", "measures": map[string]any{"weight_kg": 72.5}})
	doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "Sin peso"})
	w := doc.expect(200, "GET", "/api/patients/"+pid+"/weights", nil)["weights"].([]any)
	if len(w) != 1 || w[0].(map[string]any)["weight_kg"].(float64) != 72.5 {
		t.Fatalf("weights: %v", w)
	}
	e.login("doc_b").expect(404, "GET", "/api/patients/"+pid+"/weights", nil)
}

func TestChartValidationAndHistory(t *testing.T) {
	e := setup(t)
	doc, recep, other := e.login("doc_a"), e.login("recep_a"), e.login("doc_b")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	url := "/api/patients/" + pid + "/charts"

	odo := func(teeth map[string]any) map[string]any {
		return map[string]any{"kind": "odontogram", "data": map[string]any{"dentition": "adult", "teeth": teeth}}
	}
	bad := []map[string]any{
		odo(map[string]any{"19": map[string]any{"state": "caries"}}),                          // no such tooth
		odo(map[string]any{"56": map[string]any{"state": "caries"}}),                          // deciduous have only 5 per quadrant
		odo(map[string]any{"90": map[string]any{"state": "caries"}}),                          // out of range
		odo(map[string]any{"16": map[string]any{"state": "magia"}}),                           // state
		odo(map[string]any{"16": map[string]any{"surfaces": map[string]any{"Z": "caries"}}}),  // surface
		odo(map[string]any{"16": map[string]any{"surfaces": map[string]any{"O": "mordida"}}}), // state on surface
		{"kind": "odontogram", "data": map[string]any{"teeth": map[string]any{}, "extra": 1}}, // unknown field
		{"kind": "bodymap", "data": map[string]any{"zones": []any{map[string]any{"zone": "lumbar", "view": "front", "kind": "dolor", "intensity": 3}}}},
		{"kind": "bodymap", "data": map[string]any{"zones": []any{map[string]any{"zone": "lumbar", "view": "back", "kind": "dolor", "intensity": 11}}}},
		{"kind": "bodymap", "data": map[string]any{"zones": []any{map[string]any{"zone": "lumbar", "view": "back", "kind": "otra", "intensity": 3}}}},
		{"kind": "bodymap", "data": map[string]any{"zones": []any{map[string]any{"zone": "lumbar", "view": "side", "kind": "dolor", "intensity": 3}}}},
		{"kind": "other", "data": map[string]any{}},
		{"kind": "odontogram", "data": map[string]any{"teeth": map[string]any{"16": map[string]any{"note": strings.Repeat("x", 70000)}}}},
	}
	for i, b := range bad {
		if status, _ := doc.do("POST", url, b); status != 400 && status != 413 {
			t.Errorf("case %d accepted with %d", i, status)
		}
	}

	good := odo(map[string]any{
		"16": map[string]any{"state": "corona", "note": "Porcelana"},
		"55": map[string]any{"surfaces": map[string]any{"O": "caries", "M": "restauracion"}},
		"48": map[string]any{"state": "ausente"},
	})
	good["note"] = "Primera revisión"
	doc.expect(201, "POST", url, good)
	good["note"] = "Control"
	doc.expect(201, "POST", url, good)
	doc.expect(201, "POST", url, map[string]any{"kind": "bodymap", "data": map[string]any{"zones": []any{
		map[string]any{"zone": "lumbar", "view": "back", "kind": "contractura", "intensity": 7, "note": "Derecha"},
		map[string]any{"zone": "knee_l", "view": "front", "kind": "dolor", "intensity": 0},
	}}})

	out := doc.expect(200, "GET", url+"?kind=odontogram", nil)
	if len(out["charts"].([]any)) != 2 || sub(out, "latest")["note"] != "Control" {
		t.Fatalf("history: %v", out)
	}
	all := doc.expect(200, "GET", url, nil)["charts"].([]any)
	if len(all) != 3 {
		t.Fatalf("all: %d", len(all))
	}
	doc.expect(400, "GET", url+"?kind=zzz", nil)

	if _, err := e.pool.Exec(t.Context(), `UPDATE patient_charts SET note = 'x'`); err == nil {
		t.Fatal("a snapshot was edited")
	}
	var views int
	_ = e.pool.QueryRow(t.Context(), `SELECT count(*) FROM record_access WHERE patient_id = $1`, pid).Scan(&views)
	if views == 0 {
		t.Fatal("chart reads are not logged")
	}

	recep.expect(403, "GET", url, nil)
	recep.expect(403, "POST", url, good)
	other.expect(404, "GET", url, nil)
	other.expect(404, "POST", url, good)
}

func TestConsentSignature(t *testing.T) {
	e := setup(t)
	doc, recep, other := e.login("doc_a"), e.login("recep_a"), e.login("doc_b")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	url := "/api/patients/" + pid + "/consents"
	body := func(sig string) map[string]any {
		return map[string]any{"kind": "procedimiento", "text_snapshot": "Autorizo la extracción.", "signer_name": "Jorge Medina", "signer_role": "paciente",
			"signature_png": sig, "witness1": "Ana"}
	}
	doc.expect(400, "POST", url, body(""))
	doc.expect(400, "POST", url, body("data:image/png;base64,AAAA"))
	doc.expect(400, "POST", url, body("data:image/png;base64,!!!"))
	doc.expect(400, "POST", url, body("data:image/jpeg;base64,"+base64.StdEncoding.EncodeToString([]byte("\xff\xd8\xff\xe0abc"))))
	doc.expect(400, "POST", url, body("data:image/png;base64,"+base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nnot really"))))
	doc.expect(400, "POST", url, body("data:image/png;base64,"+base64.StdEncoding.EncodeToString(append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 210<<10)...))))
	sig := signaturePNG(t, 200, 80)
	bad := body(sig)
	bad["signer_role"] = "vecino"
	doc.expect(400, "POST", url, bad)
	bad = body(sig)
	bad["kind"] = "otro"
	doc.expect(400, "POST", url, bad)
	bad = body(sig)
	bad["signer_name"] = " "
	doc.expect(400, "POST", url, bad)
	bad = body(sig)
	bad["plan_id"] = "7b6e1c70-0000-4000-8000-000000000000"
	doc.expect(400, "POST", url, bad)

	c := sub(doc.expect(201, "POST", url, body(sig)), "consent")
	if len(c["content_sha256"].(string)) != 64 {
		t.Fatalf("hash: %v", c)
	}
	got := sub(doc.expect(200, "GET", "/api/consents/"+c["id"].(string), nil), "consent")
	if got["signature_png"] != sig || got["text_snapshot"] != "Autorizo la extracción." || got["registered_by_name"] == "" {
		t.Fatalf("consent: %v", got)
	}
	list := doc.expect(200, "GET", url, nil)["consents"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["signature_png"] != nil {
		t.Fatalf("list: %v", list)
	}
	if _, err := e.pool.Exec(t.Context(), `UPDATE consent_signatures SET signer_name = 'otro'`); err == nil {
		t.Fatal("a consent was edited")
	}
	recep.expect(403, "POST", url, body(sig))
	recep.expect(403, "GET", "/api/consents/"+c["id"].(string), nil)
	other.expect(404, "GET", "/api/consents/"+c["id"].(string), nil)
	other.expect(404, "POST", url, body(sig))
	other.expect(404, "GET", url, nil)
}

func planBody(items ...map[string]any) map[string]any {
	return map[string]any{"title": "Rehabilitación", "notes": "Por fases", "items": items}
}

func item(phase int, desc string, qty float64, cents int) map[string]any {
	return map[string]any{"phase": phase, "description": desc, "qty": qty, "unit_price_cents": cents, "tooth": "16"}
}

func TestTreatmentPlanLifecycle(t *testing.T) {
	e := setup(t)
	doc, recep, cash, other := e.login("doc_a"), e.login("recep_a"), e.login("cash_a"), e.login("doc_b")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	url := "/api/patients/" + pid + "/plans"

	doc.expect(400, "POST", url, planBody(item(1, "", 1, 100)))
	doc.expect(400, "POST", url, planBody(item(1, "x", -1, 100)))
	doc.expect(400, "POST", url, planBody(item(1, "x", 1, -5)))
	doc.expect(400, "POST", url, planBody(item(99, "x", 1, 5)))
	doc.expect(400, "POST", url, map[string]any{"title": " ", "items": []any{}})
	recep.expect(403, "POST", url, planBody(item(1, "x", 1, 5)))
	other.expect(404, "POST", url, planBody(item(1, "x", 1, 5)))

	p := sub(doc.expect(201, "POST", url, planBody(item(1, "Limpieza", 1, 80000), item(2, "Resina", 2.5, 50000))), "plan")
	id := p["id"].(string)
	if p["status"] != "draft" || p["total_cents"].(float64) != 80000+125000 || p["patient_name"] == "" || p["version"].(float64) != 1 {
		t.Fatalf("plan: %v", p)
	}
	planURL := "/api/plans/" + id

	// Nothing is accepted yet, so nothing can be marked done; proposing needs items.
	items := p["items"].([]any)
	i0 := items[0].(map[string]any)["id"].(string)
	doc.expect(409, "POST", planURL+"/items/"+i0+"/done", map[string]any{})
	doc.expect(409, "POST", planURL+"/items", map[string]any{"items": []any{item(1, "x", 1, 1)}})
	empty := sub(doc.expect(201, "POST", url, planBody()), "plan")
	doc.expect(400, "POST", "/api/plans/"+empty["id"].(string)+"/propose", nil)

	// Editing a draft replaces its items.
	p = sub(doc.expect(200, "PUT", planURL, planBody(item(1, "Limpieza", 1, 80000), item(2, "Resina", 3, 50000), item(3, "Corona", 1, 300000))), "plan")
	if len(p["items"].([]any)) != 3 || p["total_cents"].(float64) != 80000+150000+300000 {
		t.Fatalf("edited: %v", p)
	}
	p = sub(doc.expect(200, "POST", planURL+"/propose", nil), "plan")
	if p["status"] != "proposed" {
		t.Fatalf("proposed: %v", p["status"])
	}
	doc.expect(409, "POST", planURL+"/propose", nil)
	doc.expect(200, "PUT", planURL, planBody(item(1, "Limpieza", 1, 80000), item(2, "Resina", 3, 50000)))

	// Accepting needs a valid signature.
	sig := signaturePNG(t, 200, 80)
	doc.expect(400, "POST", planURL+"/accept", map[string]any{"signer_name": "Jorge", "signature_png": "data:image/png;base64,AAAA"})
	doc.expect(400, "POST", planURL+"/accept", map[string]any{"signer_name": "", "signature_png": sig})
	recep.expect(403, "POST", planURL+"/accept", map[string]any{"signer_name": "Jorge", "signature_png": sig})
	other.expect(404, "POST", planURL+"/accept", map[string]any{"signer_name": "Jorge", "signature_png": sig})
	p = sub(doc.expect(200, "POST", planURL+"/accept", map[string]any{"signer_name": "Jorge Medina", "signature_png": sig}), "plan")
	if p["status"] != "accepted" || p["accepted_by_name"] != "Jorge Medina" || p["accepted_version"].(float64) != 1 {
		t.Fatalf("accepted: %v", p)
	}
	doc.expect(409, "POST", planURL+"/accept", map[string]any{"signer_name": "Jorge Medina", "signature_png": sig})
	doc.expect(409, "PUT", planURL, planBody(item(1, "Otra cosa", 1, 1)))
	cons := doc.expect(200, "GET", "/api/patients/"+pid+"/consents", nil)["consents"].([]any)
	if len(cons) != 1 || cons[0].(map[string]any)["kind"] != "plan_tratamiento" || cons[0].(map[string]any)["plan_id"] != id {
		t.Fatalf("plan consent: %v", cons)
	}

	// Whoever charges (POS) reads the plan; other clinics cannot.
	got := sub(cash.expect(200, "GET", planURL, nil), "plan")
	if got["total_cents"].(float64) != 80000+150000 {
		t.Fatalf("pos view: %v", got)
	}
	recep.expect(200, "GET", planURL, nil) // reception also charges, so it reads plans
	other.expect(404, "GET", planURL, nil)
	cash.expect(403, "POST", planURL+"/items/"+got["items"].([]any)[0].(map[string]any)["id"].(string)+"/done", map[string]any{})

	// Progress: first item done -> in progress; an added version keeps what was accepted and asks for a signature.
	items = got["items"].([]any)
	a, b := items[0].(map[string]any)["id"].(string), items[1].(map[string]any)["id"].(string)
	enc := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "Limpieza"}), "encounter")["id"].(string)
	doc.expect(400, "POST", planURL+"/items/"+a+"/done", map[string]any{"encounter_id": "7b6e1c70-0000-4000-8000-000000000000"})
	p = sub(doc.expect(200, "POST", planURL+"/items/"+a+"/done", map[string]any{"encounter_id": enc}), "plan")
	if p["status"] != "in_progress" || p["done_cents"].(float64) != 80000 || p["pending_cents"].(float64) != 150000 {
		t.Fatalf("progress: %v", p)
	}
	doc.expect(409, "POST", planURL+"/items/"+a+"/done", map[string]any{})
	p = sub(doc.expect(201, "POST", planURL+"/items", map[string]any{"reason": "Hallazgo nuevo", "items": []any{item(2, "Endodoncia", 1, 200000)}}), "plan")
	if p["version"].(float64) != 2 || p["accepted_version"].(float64) != 1 || len(p["items"].([]any)) == 3 == false {
		t.Fatalf("version 2: %v", p)
	}
	doc.expect(200, "POST", planURL+"/accept", map[string]any{"signer_name": "Jorge Medina", "signature_png": sig})
	doc.expect(400, "POST", planURL+"/items/"+b+"/cancel", map[string]any{"reason": ""})
	p = sub(doc.expect(200, "POST", planURL+"/items/"+b+"/cancel", map[string]any{"reason": "El paciente lo pospone"}), "plan")
	if p["total_cents"].(float64) != 80000+200000 {
		t.Fatalf("total without cancelled: %v", p["total_cents"])
	}
	// Finish everything pending: the plan completes by itself.
	for _, it := range p["items"].([]any) {
		m := it.(map[string]any)
		if m["status"] == "pending" {
			p = sub(doc.expect(200, "POST", planURL+"/items/"+m["id"].(string)+"/done", map[string]any{}), "plan")
		}
	}
	if p["status"] != "completed" {
		t.Fatalf("not completed: %v", p["status"])
	}
	doc.expect(409, "POST", planURL+"/cancel", map[string]any{"reason": "x"})
	if n := len(p["events"].([]any)); n < 6 {
		t.Fatalf("history kept: %d events", n)
	}
	if n := len(doc.expect(200, "GET", url, nil)["plans"].([]any)); n != 2 {
		t.Fatalf("patient plans: %d", n)
	}
	other.expect(404, "GET", url, nil)
}

func TestPlanCancelAndSaleLink(t *testing.T) {
	e := setup(t)
	doc, cash := e.login("doc_a"), e.login("cash_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	p := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/plans", planBody(item(1, "Limpieza", 1, 80000))), "plan")
	id := p["id"].(string)
	doc.expect(400, "POST", "/api/plans/"+id+"/cancel", map[string]any{"reason": ""})
	p = sub(doc.expect(200, "POST", "/api/plans/"+id+"/cancel", map[string]any{"reason": "Ya no lo quiere"}), "plan")
	if p["status"] != "cancelled" || p["cancel_reason"] == "" {
		t.Fatalf("cancelled: %v", p)
	}
	doc.expect(409, "POST", "/api/plans/"+id+"/cancel", map[string]any{"reason": "otra vez"})
	cash.expect(400, "POST", "/api/plans/"+id+"/link-sale", map[string]any{"sale_id": "7b6e1c70-0000-4000-8000-000000000000", "item_ids": []string{p["items"].([]any)[0].(map[string]any)["id"].(string)}})
	doc.expect(403, "POST", "/api/plans/"+id+"/link-sale", map[string]any{"sale_id": "x", "item_ids": []string{}})
}

func TestNutritionPlanChart(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	url := "/api/patients/" + pid + "/charts"
	plan := func(m map[string]any) map[string]any { return map[string]any{"kind": "nutrition_plan", "data": m} }
	for i, b := range []map[string]any{
		plan(map[string]any{"kcal": 20000}),
		plan(map[string]any{"protein_pct": 60, "carb_pct": 40, "fat_pct": 30}),
		plan(map[string]any{"meals": []any{map[string]any{"name": "", "time": "08:00", "items": "x", "kcal": 100}}}),
		plan(map[string]any{"surprise": true}),
	} {
		if status, _ := doc.do("POST", url, b); status != 400 {
			t.Errorf("case %d accepted with %d", i, status)
		}
	}
	good := plan(map[string]any{"goal": "Bajar de peso", "kcal": 1800, "protein_pct": 25, "carb_pct": 45, "fat_pct": 30, "water_liters": 2.5,
		"meals": []any{map[string]any{"name": "Desayuno", "time": "08:00", "items": "Avena con fruta", "kcal": 450}}, "follow_up_days": 30})
	if status, body := doc.do("POST", url, good); status != 201 {
		t.Fatalf("good plan: %d %s", status, body)
	}
	if status, body := doc.do("GET", url+"?kind=nutrition_plan", nil); status != 200 || !strings.Contains(fmt.Sprint(body), "Bajar de peso") {
		t.Fatalf("list: %d %s", status, body)
	}
}

func TestNutritionPlanAI(t *testing.T) {
	var prompts []string
	fish := true // the first answer still uses a disliked food: the server asks again
	gem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Contents []struct {
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		prompts = append(prompts, body.Contents[0].Parts[0]["text"].(string))
		meal := func(n, items string) map[string]any { return map[string]any{"name": n, "items": items} }
		days := []any{}
		for i := 0; i < 7; i++ {
			lunch := "Pollo con arroz"
			if fish && i == 2 {
				lunch = "Filete de Pescado a la plancha"
			}
			days = append(days, map[string]any{"name": "x", "meals": []any{meal("Desayuno", "Avena con fruta"), meal("Colación 1", "Manzana"), meal("Comida", lunch), meal("Colación 2", "Nueces"), meal("Cena", "Quesadilla")}})
		}
		fish = false
		out, _ := json.Marshal(map[string]any{"goal": "Bajar de peso", "days": days, "recommendations": "Come despacio", "avoid": "Refrescos", "supplements": "", "follow_up_days": 30})
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(out)}}}}}})
	}))
	defer gem.Close()
	e := setupWith(t, func(c *config.Config) {
		c.GeminiAPIKey, c.GeminiModel, c.GeminiAPIBase = "gem-key", "test-model", gem.URL
	})
	doc, recep := e.login("doc_a"), e.login("recep_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	url := "/api/patients/" + pid + "/nutrition-plan/ai"

	recep.expect(403, "POST", url, map[string]any{"goal": "Bajar de peso"})
	doc.expect(400, "POST", url, map[string]any{})
	doc.expect(400, "POST", url, map[string]any{"goal": "Bajar de peso"}) // no weight and height: the calories cannot be computed
	out := doc.expect(200, "POST", url, map[string]any{"goal": "Bajar de peso", "weight_kg": 82, "height_cm": 170, "activity_factor": 1.375, "activity": "Ligera",
		"preferences": "Vegetariano", "dislikes": "pescado, hígado", "meals": 5})
	plan := out["plan"].(map[string]any)

	// calories are computed here: patient born 1970-03-12 ("mejj700312"), so 55 years old; Mifflin-St Jeor
	// 10*82 + 6.25*170 - 5*age + sexTerm, times 1.375, minus 500 to lose weight
	days := plan["days"].([]any)
	if len(days) != 7 {
		t.Fatalf("a week has 7 days: %d", len(days))
	}
	kcal := num(plan, "kcal")
	if kcal < 1200 || kcal > 3500 || int(kcal)%10 != 0 {
		t.Fatalf("kcal: %v", kcal)
	}
	for _, d := range days {
		meals := d.(map[string]any)["meals"].([]any)
		sum := 0.0
		for _, m := range meals {
			sum += num(m.(map[string]any), "kcal")
		}
		if len(meals) != 5 || sum != kcal {
			t.Fatalf("each day closes the target exactly: %v vs %v (%d meals)", sum, kcal, len(meals))
		}
	}
	if num(plan, "carb_pct")+num(plan, "protein_pct")+num(plan, "fat_pct") != 100 || num(plan, "protein_pct") < 15 || num(plan, "protein_pct") > 35 {
		t.Fatalf("macros: %v", plan)
	}
	if plan["dislikes"] != "pescado, hígado" {
		t.Fatalf("dislikes must travel with the plan: %v", plan["dislikes"])
	}
	// the first answer used a disliked food, so the model was asked again with the violation
	if len(prompts) != 2 || !strings.Contains(prompts[0], "pescado, hígado") || !strings.Contains(prompts[1], "prohibidos") {
		t.Fatalf("prompts: %d %v", len(prompts), prompts)
	}
	if w, _ := out["warnings"].([]any); len(w) != 0 {
		t.Fatalf("the second answer is clean: %v", w)
	}
	if strings.Contains(prompts[0], "Prueba") || strings.Contains(prompts[0], "mejj7003") {
		t.Fatalf("the prompt must not carry identifying data: %s", prompts[0])
	}
	// without snacks the day has breakfast, lunch and dinner only; with them, five meals
	plain := doc.expect(200, "POST", url, map[string]any{"goal": "Mantener", "weight_kg": 82, "height_cm": 170, "activity_factor": 1.375, "snacks": false})["plan"].(map[string]any)
	if n := len(plain["days"].([]any)[0].(map[string]any)["meals"].([]any)); n != 3 {
		t.Fatalf("no snacks: 3 meals a day, got %d", n)
	}
	if withSnacks := doc.expect(200, "POST", url, map[string]any{"goal": "Mantener", "weight_kg": 82, "height_cm": 170, "activity_factor": 1.375, "snacks": true})["plan"].(map[string]any); len(withSnacks["days"].([]any)[0].(map[string]any)["meals"].([]any)) != 5 {
		t.Fatal("with snacks: 5 meals a day")
	}
	// the free calculator: with body fat it uses Katch-McArdle (lean mass), without it Mifflin-St Jeor
	calc := "/api/patients/" + pid + "/nutrition-plan/calc"
	recep.expect(403, "POST", calc, map[string]any{"goal": "Bajar de peso", "weight_kg": 82, "height_cm": 170})
	doc.expect(400, "POST", calc, map[string]any{"goal": "Bajar de peso"})
	base := doc.expect(200, "POST", calc, map[string]any{"goal": "Bajar de peso", "weight_kg": 82, "height_cm": 170, "activity_factor": 1.375})
	fat := doc.expect(200, "POST", calc, map[string]any{"goal": "Bajar de peso", "weight_kg": 82, "height_cm": 170, "activity_factor": 1.375, "body_fat_pct": 35})
	if base["method"] != "Mifflin-St Jeor" || fat["method"] != "Katch-McArdle" {
		t.Fatalf("methods: %v %v", base["method"], fat["method"])
	}
	// 370 + 21.6 * (82 * 0.65) = 1521
	if num(fat, "bmr") != 1521 || num(base, "bmi") != 28.4 {
		t.Fatalf("bmr %v bmi %v", fat["bmr"], base["bmi"])
	}
	if num(fat, "carb_pct")+num(fat, "protein_pct")+num(fat, "fat_pct") != 100 || num(fat, "adjust") >= 0 {
		t.Fatalf("fat based targets: %v", fat)
	}
	// the draft is not saved by itself
	if got := doc.expect(200, "GET", "/api/patients/"+pid+"/charts?kind=nutrition_plan", nil); len(got["charts"].([]any)) != 0 {
		t.Fatal("the AI draft must not be saved")
	}
	// the weekly plan can be saved as it came
	if status, body := doc.do("POST", "/api/patients/"+pid+"/charts", map[string]any{"kind": "nutrition_plan", "data": plan}); status != 201 {
		t.Fatalf("saving the weekly plan: %d %v", status, body)
	}
}

// A new plan must not copy the previous one: the model is told what was served and asked again when it repeats.
func TestNutritionPlanAIVariesFromPreviousPlans(t *testing.T) {
	var prompts []string
	variant := 0 // 0 = always the same menu; once it is > 0 the menu changes
	gem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Contents []struct {
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		prompts = append(prompts, body.Contents[0].Parts[0]["text"].(string))
		meal := func(n, items string) map[string]any { return map[string]any{"name": n, "items": items} }
		same := variant == 0 || len(prompts) < variant
		days := []any{}
		for i := 0; i < 7; i++ {
			b, c, d := "Avena con plátano y nueces", "Pollo con arroz y ensalada verde", "Quesadilla de queso panela con nopales"
			if !same {
				b, c, d = fmt.Sprintf("Chilaquiles verdes con huevo %d", i), fmt.Sprintf("Salmón al horno con quinoa y brócoli %d", i), fmt.Sprintf("Sopa de verduras con tostadas %d", i)
			}
			days = append(days, map[string]any{"name": "x", "meals": []any{meal("Desayuno", b), meal("Comida", c), meal("Cena", d)}})
		}
		out, _ := json.Marshal(map[string]any{"goal": "Mantener", "days": days, "recommendations": "Come despacio", "avoid": "", "supplements": "", "follow_up_days": 30})
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(out)}}}}}})
	}))
	defer gem.Close()
	e := setupWith(t, func(c *config.Config) {
		c.GeminiAPIKey, c.GeminiModel, c.GeminiAPIBase = "gem-key", "test-model", gem.URL
	})
	doc := e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	url := "/api/patients/" + pid + "/nutrition-plan/ai"
	req := map[string]any{"goal": "Mantener", "weight_kg": 70, "height_cm": 170, "activity_factor": 1.375, "snacks": false}

	// first plan: nothing earlier to vary from
	first := doc.expect(200, "POST", url, req)["plan"]
	if len(prompts) != 1 || strings.Contains(prompts[0], "PLANES ANTERIORES") {
		t.Fatalf("no history: one call and no history in the prompt: %d", len(prompts))
	}
	doc.expect(201, "POST", "/api/patients/"+pid+"/charts", map[string]any{"kind": "nutrition_plan", "data": first})

	// second plan: the history is in the prompt, and a copy of it is asked again until it changes
	prompts, variant = nil, 3 // calls 1 and 2 repeat the menu, the 3rd one changes it
	doc.expect(200, "POST", url, req)
	if len(prompts) != 3 {
		t.Fatalf("the repeated menu is retried: %d calls", len(prompts))
	}
	if !strings.Contains(prompts[0], "PLANES ANTERIORES") || !strings.Contains(prompts[0], "Avena con plátano") {
		t.Fatalf("the prompt lists what was served before: %s", prompts[0])
	}
	if !strings.Contains(prompts[1], "repetía") {
		t.Fatalf("the retry says it repeated: %s", prompts[1])
	}
}
