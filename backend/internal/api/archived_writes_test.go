package api_test

import "testing"

// An archived record (also what an ARCO block does) is read-only: nothing new is added to it until it is restored.
func TestArchivedRecordRefusesNewEntries(t *testing.T) {
	e := setup(t)
	e.exec(`UPDATE users SET cedula = '1234567', cedula_institution = 'UNAM', specialty_title = 'Médico' WHERE id = $1`, e.userID("doc_a"))
	doc := e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	base := "/api/patients/" + pid
	attempts := func() map[string][2]any {
		return map[string][2]any{
			"encounter":    {base + "/encounters", map[string]any{"reason": "Dolor"}},
			"prescription": {base + "/prescriptions", map[string]any{"diagnosis": "x", "items": []any{map[string]any{"medicine": "Paracetamol", "dose": "500 mg", "frequency": "8 h", "duration": "3 días"}}}},
			"certificate":  {base + "/certificates", map[string]any{"purpose": "Escuela", "statement": "Sano."}},
			"chronic med":  {base + "/chronic-meds", map[string]any{"name": "Metformina", "dose": "850 mg"}},
			"vaccination":  {base + "/vaccinations", map[string]any{"name": "Td", "applied_on": "2026-01-01"}},
			"chart":        {base + "/charts", map[string]any{"kind": "problems", "data": map[string]any{"problems": []any{}}}},
			"plan":         {base + "/plans", map[string]any{"title": "Plan", "items": []any{}}},
			"consent":      {base + "/consents", map[string]any{"kind": "privacidad", "signer_name": "Jorge"}},
		}
	}
	e.exec(`UPDATE patients SET archived_at = now(), archive_reason = 'prueba' WHERE id = $1`, pid)
	for name, a := range attempts() {
		if code := errCode(t, doc, "POST", a[0].(string), a[1], 409); code != "PATIENT_ARCHIVED" {
			t.Fatalf("%s on an archived record: code %q", name, code)
		}
	}
	// reading is still allowed
	doc.expect(200, "GET", base+"/encounters", nil)
	// restored: the same kind of entry works again
	e.exec(`UPDATE patients SET archived_at = NULL WHERE id = $1`, pid)
	doc.expect(201, "POST", base+"/encounters", map[string]any{"reason": "Dolor"})
}
