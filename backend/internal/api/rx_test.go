package api_test

import (
	"context"
	"strings"
	"testing"
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
