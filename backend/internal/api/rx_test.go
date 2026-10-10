package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

// rxDoctor returns a doctor with a cédula registered, so receta can be issued.
func rxDoctor(t *testing.T, e *env, user string) *client {
	t.Helper()
	doc := e.login(user)
	doc.expect(200, "PUT", "/api/me", map[string]any{"name": "Dr. Ejemplo", "phone": "", "cedula": "12345678", "cedula_institution": "UNAM", "specialty_title": "Médico Cirujano"})
	return doc
}

func rxBody(items ...map[string]any) map[string]any {
	arr := []any{}
	for _, i := range items {
		arr = append(arr, i)
	}
	return map[string]any{"diagnosis": "Cefalea tensional", "items": arr, "instructions": "Reposo."}
}

func rxItem(name string, extra map[string]any) map[string]any {
	m := map[string]any{"medicine": name, "dose": "1", "route": "Oral", "frequency": "Cada 8 horas", "duration": "3 días"}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestRxVerification(t *testing.T) {
	e := setup(t)
	doc := rxDoctor(t, e, "doc_a")
	pid := sub(doc.expect(201, "POST", "/api/patients/", person(map[string]any{"profile": map[string]any{"allergies_text": "Ninguna conocida"}})), "patient")["id"].(string)
	rx := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/prescriptions", rxBody(rxItem("Amoxicilina", map[string]any{"control": "Antibiótico"}))), "prescription")
	if _, leaked := rx["verify_token"]; leaked {
		t.Fatal("the token must only travel in the print data URL")
	}
	pd := doc.expect(200, "GET", "/api/prescriptions/"+rx["id"].(string), nil)
	url, _ := pd["verify_url"].(string)
	if !strings.HasPrefix(url, "http://app.test/verificar/") {
		t.Fatalf("verify_url: %q", url)
	}
	token := strings.TrimPrefix(url, "http://app.test/verificar/")
	if len(token) < 20 {
		t.Fatalf("token too short: %q", token)
	}
	// the URL stays the same for the same receta
	if again := doc.expect(200, "GET", "/api/prescriptions/"+rx["id"].(string), nil)["verify_url"]; again != url {
		t.Fatalf("verify_url changed: %v", again)
	}

	anon := e.anon()
	got := anon.expect(200, "GET", "/api/public/rx/"+token, nil)
	if got["status"] != "vigente" || got["folio"].(float64) != 1 || got["clinic_name"] != "Clinica a" || got["professional_license"] != "12345678" ||
		got["patient_initials"] != "J. M." || got["retained"] != true {
		t.Fatalf("verification: %v", got)
	}
	for _, forbidden := range []string{"items", "diagnosis", "instructions", "patient", "names", "last_names", "patient_id", "id"} {
		if _, ok := got[forbidden]; ok {
			t.Fatalf("public verification leaks %q: %v", forbidden, got)
		}
	}

	// unknown, malformed and guessed tokens all look the same
	for _, bad := range []string{"x", strings.Repeat("a", 24), "../../etc/passwd", strings.Repeat("b", 200)} {
		anon.expect(404, "GET", "/api/public/rx/"+bad, nil)
	}

	// expired and voided states
	e.exec(`UPDATE prescriptions SET valid_until = current_date - 1 WHERE id = $1`, rx["id"])
	if s := anon.expect(200, "GET", "/api/public/rx/"+token, nil)["status"]; s != "vencida" {
		t.Fatalf("expired: %v", s)
	}
	doc.expect(200, "POST", "/api/prescriptions/"+rx["id"].(string)+"/void", map[string]any{"reason": "Error de captura"})
	voided := anon.expect(200, "GET", "/api/public/rx/"+token, nil)
	if voided["status"] != "anulada" {
		t.Fatalf("voided: %v", voided)
	}
	if _, ok := voided["void_reason"]; ok {
		t.Fatal("the void reason is internal")
	}

	// existing recetas get a token in the migration; new ones are unique
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(DISTINCT verify_token) FROM prescriptions`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("tokens: %d %v", n, err)
	}

	// rate limit per IP
	limited := false
	for i := 0; i < 60 && !limited; i++ {
		code, _ := anon.do("GET", "/api/public/rx/"+strings.Repeat("c", 24), nil)
		limited = code == 429
	}
	if !limited {
		t.Fatal("verification must be rate limited")
	}
}

func TestRxAllergyAndDose(t *testing.T) {
	e := setup(t)
	doc := rxDoctor(t, e, "doc_a")
	post := func(pid string, body map[string]any) (int, map[string]any) {
		return doc.do("POST", "/api/patients/"+pid+"/prescriptions", body)
	}
	allergic := sub(doc.expect(201, "POST", "/api/patients/", person(map[string]any{"profile": map[string]any{"allergies_text": "Alergia a las penicilinas, AINEs y sulfas"}})), "patient")["id"].(string)

	for _, med := range []string{"Amoxicilina", "Ibuprofeno", "Trimetoprima / sulfametoxazol", "ÁMPICILINA"} {
		code, out := post(allergic, rxBody(rxItem(med, nil)))
		if code != 409 || out["code"] != "ALLERGY_CONFLICT" || len(out["conflicts"].([]any)) != 1 {
			t.Fatalf("%s: %d %v", med, code, out)
		}
	}
	if code, out := post(allergic, rxBody(rxItem("Paracetamol", nil))); code != 201 {
		t.Fatalf("paracetamol is not in those families: %d %v", code, out)
	}
	// blank or whitespace reasons do not override
	body := rxBody(rxItem("Amoxicilina", nil))
	body["allergy_override_reason"] = "   "
	if code, _ := post(allergic, body); code != 409 {
		t.Fatalf("blank reason: %d", code)
	}
	body["allergy_override_reason"] = "Desensibilizado, tolera amoxicilina"
	code, out := post(allergic, body)
	if code != 201 || sub(out, "prescription")["allergy_override_reason"] != "Desensibilizado, tolera amoxicilina" {
		t.Fatalf("override: %d %v", code, out)
	}
	var meta string
	if err := e.pool.QueryRow(context.Background(), `SELECT meta::text FROM activity_log WHERE type = 'prescription' ORDER BY created_at DESC LIMIT 1`).Scan(&meta); err != nil || !strings.Contains(meta, "Desensibilizado") {
		t.Fatalf("the override must be audited: %v %q", err, meta)
	}

	// maximum daily dose by weight (paracetamol: 75 mg/kg/day)
	clean := sub(doc.expect(201, "POST", "/api/patients/", person(map[string]any{"curp": "", "profile": map[string]any{"allergies_text": "Ninguna conocida"}})), "patient")["id"].(string)
	over := rxBody(rxItem("Paracetamol", map[string]any{"catalog_id": "h-paracetamol", "dose_mg": 500, "doses_per_day": 4}))
	over["weight_kg"] = 10
	code, out = post(clean, over)
	if code != 409 || out["code"] != "DOSE_WARNING" || len(out["warnings"].([]any)) != 1 {
		t.Fatalf("dose warning: %d %v", code, out)
	}
	over["dose_override_reason"] = "Adulto joven pequeño, dosis validada"
	if code, out = post(clean, over); code != 201 || sub(out, "prescription")["weight_kg"].(float64) != 10 {
		t.Fatalf("dose override: %d %v", code, out)
	}
	ok := rxBody(rxItem("Paracetamol", map[string]any{"catalog_id": "h-paracetamol", "dose_mg": 150, "doses_per_day": 4}))
	ok["weight_kg"] = 10
	if code, out = post(clean, ok); code != 201 {
		t.Fatalf("600 mg/day for 10 kg: %d %v", code, out)
	}
	// without weight there is nothing to compare; unknown catalog ids are ignored
	noWeight := rxBody(rxItem("Paracetamol", map[string]any{"catalog_id": "h-paracetamol", "dose_mg": 5000, "doses_per_day": 4}))
	if code, _ = post(clean, noWeight); code != 201 {
		t.Fatalf("no weight: %d", code)
	}
	bad := rxBody(rxItem("Paracetamol", map[string]any{"dose_mg": -1}))
	if code, _ = post(clean, bad); code != 400 {
		t.Fatalf("negative dose: %d", code)
	}
}

func TestRxCatalogs(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")
	search := func(c *client, path string) []any {
		t.Helper()
		out := c.expect(200, "GET", path, nil)
		if out["disclaimer"] == nil {
			t.Fatal("catalog answers carry a disclaimer")
		}
		for _, k := range []string{"medications", "diagnoses"} {
			if l, ok := out[k].([]any); ok {
				return l
			}
		}
		t.Fatalf("no list: %v", out)
		return nil
	}
	first := func(l []any) map[string]any {
		if len(l) == 0 {
			t.Fatal("no results")
		}
		return l[0].(map[string]any)
	}
	for _, q := range []string{"amoxicilina", "AMOXICILINA", "amoxicilína", "amoxi"} {
		if first(search(doc, "/api/rx/medications?q="+q+"&subject=person"))["name"] != "Amoxicilina" {
			t.Fatalf("query %q", q)
		}
	}
	a := first(search(doc, "/api/rx/medications?q=amoxicilina&subject=person"))
	if a["control"] != "Antibiótico" || a["mg_per_kg"] == nil || len(a["concentrations"].([]any)) == 0 {
		t.Fatalf("pediatric data: %v", a)
	}
	for _, m := range search(doc, "/api/rx/medications?subject=animal&limit=100") {
		if m.(map[string]any)["subject"] != "animal" {
			t.Fatalf("subject filter: %v", m)
		}
	}
	for _, m := range search(doc, "/api/rx/medications?subject=animal&species=Gato&limit=100") {
		sp := m.(map[string]any)["species"].([]any)
		found := false
		for _, s := range sp {
			found = found || s == "Gato"
		}
		if !found {
			t.Fatalf("species filter: %v", m)
		}
	}
	if first(search(doc, "/api/rx/diagnoses?q=hipertension"))["code"] != "I10" {
		t.Fatal("diagnosis by text without accents")
	}
	if first(search(doc, "/api/rx/diagnoses?q=e11.9"))["code"] != "E11.9" || first(search(doc, "/api/rx/diagnoses?q=J45"))["code"] != "J45.0" {
		t.Fatal("diagnosis by code")
	}

	// clinic-own medicines: admin and doctor write, everyone clinical reads, other clinics never see them
	body := map[string]any{"name": "Fórmula magistral Zeta", "control": "No", "route": "Tópica", "presentations": []any{"Crema 30 g"}, "concentrations": []any{map[string]any{"label": "10 mg/mL", "mg_per_ml": 10}}, "max_mg_per_kg_day": 5}
	own := sub(doc.expect(201, "POST", "/api/rx/clinic-medications", body), "medication")
	id := own["id"].(string)
	if first(search(doc, "/api/rx/medications?q=formula%20magistral"))["source"] != "clinic" {
		t.Fatal("clinic medicines are searchable, accents ignored")
	}
	docB := e.login("doc_b")
	if n := len(search(docB, "/api/rx/medications?q=zeta")); n != 0 {
		t.Fatalf("clinic B sees clinic A's medicine: %d", n)
	}
	if n := len(docB.expect(200, "GET", "/api/rx/clinic-medications", nil)["medications"].([]any)); n != 0 {
		t.Fatalf("clinic B list: %d", n)
	}
	docB.expect(404, "PUT", "/api/rx/clinic-medications/"+id, body)
	docB.expect(404, "DELETE", "/api/rx/clinic-medications/"+id, nil)
	body["name"] = "Fórmula Zeta 2"
	if sub(doc.expect(200, "PUT", "/api/rx/clinic-medications/"+id, body), "medication")["name"] != "Fórmula Zeta 2" {
		t.Fatal("update")
	}
	doc.expect(400, "POST", "/api/rx/clinic-medications", map[string]any{"name": ""})
	doc.expect(400, "POST", "/api/rx/clinic-medications", map[string]any{"name": "X", "control": "Fracción I o II"})
	doc.expect(400, "POST", "/api/rx/clinic-medications", map[string]any{"name": "X", "route": "Por el aire"})

	// permissions: reception has no clinical access, cashier neither
	for _, u := range []string{"recep_a", "cash_a"} {
		c := e.login(u)
		c.expect(403, "GET", "/api/rx/medications?q=amox", nil)
		c.expect(403, "GET", "/api/rx/diagnoses?q=i10", nil)
		c.expect(403, "POST", "/api/rx/clinic-medications", body)
	}
	e.anon().expect(401, "GET", "/api/rx/medications?q=amox", nil)
	e.login("admin_a").expect(201, "POST", "/api/rx/clinic-medications", map[string]any{"name": "Otro", "route": "Oral"})
	doc.expect(200, "DELETE", "/api/rx/clinic-medications/"+id, nil)
	if n := len(search(doc, "/api/rx/medications?q=zeta")); n != 0 {
		t.Fatalf("deleted medicine still found: %d", n)
	}
}

// A new receta replaces the earlier ones of the same area; a complement and another area's recetas are untouched.
func TestRxReplacesPreviousOfSameArea(t *testing.T) {
	e := setup(t)
	e.exec(`UPDATE clinics SET kind = 'GENERAL_MEDICAL', specialties = '{NUTRITION}' WHERE id = $1`, e.clinicA)
	doc := rxDoctor(t, e, "doc_a")
	admin := e.login("admin_a")
	pid := sub(doc.expect(201, "POST", "/api/patients/", person(map[string]any{"profile": map[string]any{"allergies_text": "Ninguna conocida"}})), "patient")["id"].(string)
	url := "/api/patients/" + pid + "/prescriptions"
	voided := func() int {
		n := 0
		for _, x := range doc.expect(200, "GET", url, nil)["prescriptions"].([]any) {
			if x.(map[string]any)["voided_at"] != nil {
				n++
			}
		}
		return n
	}
	body := func(extra map[string]any) map[string]any {
		b := rxBody(rxItem("Paracetamol", nil))
		for k, v := range extra {
			b[k] = v
		}
		return b
	}
	first := sub(doc.expect(201, "POST", url, body(nil)), "prescription")
	if first["area"] != "GENERAL_MEDICAL" {
		t.Fatalf("area: %v", first["area"])
	}
	// complementary: the first stays valid
	doc.expect(201, "POST", url, body(map[string]any{"complementary": true}))
	if voided() != 0 {
		t.Fatal("a complement must not replace the earlier receta")
	}
	// another area does not affect them either
	doc.expect(400, "POST", url, body(map[string]any{"area": "VETERINARY"})) // not a giro of the clinic
	doc.expect(201, "POST", url, body(map[string]any{"area": "NUTRITION"}))
	if voided() != 0 {
		t.Fatal("a receta of another area must not replace these")
	}
	// a plain new one replaces the earlier ones of its area (the general ones), not the nutrition one
	out := doc.expect(201, "POST", url, body(nil))
	if out["replaced"].(float64) != 2 || voided() != 2 {
		t.Fatalf("replaced: %v voided: %d", out["replaced"], voided())
	}
	// dropping a giro hides its recetas without deleting them
	before := len(doc.expect(200, "GET", url, nil)["prescriptions"].([]any))
	e.exec(`UPDATE clinics SET specialties = '{}' WHERE id = $1`, e.clinicA)
	if got := len(doc.expect(200, "GET", url, nil)["prescriptions"].([]any)); got != before-1 {
		t.Fatalf("the nutrition receta must be hidden: %d of %d", got, before)
	}
	var stored int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM prescriptions WHERE patient_id = $1`, pid).Scan(&stored); err != nil || stored != before {
		t.Fatalf("recetas must stay stored: %d %v", stored, err)
	}
	e.exec(`UPDATE clinics SET specialties = '{NUTRITION}' WHERE id = $1`, e.clinicA)
	if got := len(doc.expect(200, "GET", url, nil)["prescriptions"].([]any)); got != before {
		t.Fatalf("it must come back with the giro: %d", got)
	}
	_ = admin
}

// Chronic medication feeds the interaction check of the receta; a severe interaction needs a reason, a moderate one is only shown.
func TestChronicMedsAndInteractions(t *testing.T) {
	e := setup(t)
	doc := rxDoctor(t, e, "doc_a")
	pid := sub(doc.expect(201, "POST", "/api/patients/", person(map[string]any{"profile": map[string]any{"allergies_text": "Penicilinas"}})), "patient")["id"].(string)
	chronic := "/api/patients/" + pid + "/chronic-meds"
	rx := "/api/patients/" + pid + "/prescriptions"

	doc.expect(400, "POST", chronic, map[string]any{"name": "x"})
	doc.expect(400, "POST", chronic, map[string]any{"name": "Warfarina", "next_renewal": "mañana"})
	out := doc.expect(201, "POST", chronic, map[string]any{"name": "Warfarina", "dose": "5 mg", "frequency": "Cada 24 horas", "next_renewal": "2030-01-15"})
	med := sub(out, "medication")
	if med["active"] != true || med["next_renewal"] != "2030-01-15" {
		t.Fatalf("medication: %v", med)
	}
	// a medicine the patient is allergic to is flagged when it is recorded
	al := sub(doc.expect(201, "POST", chronic, map[string]any{"name": "Amoxicilina"}), "alerts")
	if len(al["allergies"].([]any)) != 1 {
		t.Fatalf("allergy alert: %v", al)
	}

	// the live check shows the alerts before anything is saved
	check := doc.expect(200, "POST", "/api/patients/"+pid+"/rx-check", map[string]any{"items": []any{map[string]any{"medicine": "Ibuprofeno"}}})
	if len(check["interactions"].([]any)) != 1 || len(check["chronic"].([]any)) != 2 {
		t.Fatalf("rx-check: %v", check)
	}

	// a severe interaction blocks the receta until the prescriber gives a reason
	body := rxBody(rxItem("Ibuprofeno", nil))
	code, res := doc.do("POST", rx, body)
	if code != 409 || res["code"] != "INTERACTION_CONFLICT" {
		t.Fatalf("interaction must ask for a reason: %d %v", code, res)
	}
	body["interaction_override_reason"] = "INR controlado, beneficio mayor al riesgo"
	made := doc.expect(201, "POST", rx, body)
	if sub(made, "prescription")["interaction_override_reason"] == nil || len(made["interactions"].([]any)) != 1 {
		t.Fatalf("receta after the reason: %v", made)
	}
	// the valid receta now counts as «lo que ya toma»: another AINE is flagged against it
	body2 := rxBody(rxItem("Naproxeno", nil))
	body2["complementary"] = true
	code, res = doc.do("POST", rx, body2)
	if code != 409 || res["code"] != "INTERACTION_CONFLICT" {
		t.Fatalf("interaction with the valid receta and the warfarin: %d %v", code, res)
	}

	// stopping keeps the record and needs a reason; a stopped medicine no longer interacts
	doc.expect(400, "PATCH", chronic+"/"+med["id"].(string), map[string]any{"stop": true})
	st := sub(doc.expect(200, "PATCH", chronic+"/"+med["id"].(string), map[string]any{"stop": true, "stopped_reason": "Cambió de tratamiento"}), "medication")
	if st["active"] != false || st["stopped_at"] == nil {
		t.Fatalf("stopped: %v", st)
	}
	list := doc.expect(200, "GET", chronic, nil)["medications"].([]any)
	if len(list) != 2 {
		t.Fatalf("history is kept: %v", list)
	}
	e.login("recep_a").expect(403, "GET", chronic, nil)
}

// The AI summary reads the record without the patient's name, leaves out private notes, and costs one magic use only when it works.
func TestConsultSummaryAI(t *testing.T) {
	var prompts []string
	fail := false
	gem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Contents []struct {
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		prompts = append(prompts, body.Contents[0].Parts[0]["text"].(string))
		if fail {
			w.WriteHeader(500)
			return
		}
		out, _ := json.Marshal(map[string]any{"overview": "Hipertensa en control.", "key_facts": []string{"Metformina vigente"}, "pending": []string{}, "watch_for": []string{"Alergia a penicilina"}})
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(out)}}}}}})
	}))
	defer gem.Close()
	e := setupWith(t, func(c *config.Config) {
		c.GeminiAPIKey, c.GeminiModel, c.GeminiAPIBase = "gem-key", "test-model", gem.URL
	})
	doc := e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	url := "/api/patients/" + pid + "/ai-summary"
	doc.expect(400, "POST", url, map[string]any{}) // nothing to summarize yet
	doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "Control de presión", "assessment": "Hipertensión controlada", "plan": "Continuar enalapril"})
	doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "SECRETO-PRIVADO", "private": true})
	doc.expect(201, "POST", "/api/patients/"+pid+"/chronic-meds", map[string]any{"name": "Metformina", "dose": "850 mg"})

	out := doc.expect(200, "POST", url, map[string]any{})
	sum := out["summary"].(map[string]any)
	if sum["overview"] != "Hipertensa en control." || len(sum["key_facts"].([]any)) != 1 || len(prompts) != 1 {
		t.Fatalf("summary: %v", out)
	}
	if !strings.Contains(prompts[0], "Control de presión") || !strings.Contains(prompts[0], "Metformina") || strings.Contains(prompts[0], "SECRETO-PRIVADO") {
		t.Fatalf("what the model sees: %s", prompts[0])
	}
	fail = true
	doc.expect(502, "POST", url, map[string]any{}) // the use is given back
}
