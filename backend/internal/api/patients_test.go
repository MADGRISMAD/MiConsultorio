package api_test

import (
	"strings"
	"testing"
	"time"
)

func person(extra map[string]any) map[string]any {
	m := map[string]any{
		"names": "Jorge", "last_names": "Medina", "sex": "Hombre", "birth_date": "1970-03-12", "curp": "mejj700312hdfdrr04",
		"phone": "5512345678", "privacy_ack": true,
		"profile": map[string]any{"allergies_text": "Penicilina", "chronic_conditions": []any{"Diabetes"}, "tobacco": "No"},
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestPatientSchemaPerGiro(t *testing.T) {
	e := setup(t)
	e.exec(`UPDATE clinics SET kind = 'VETERINARY' WHERE id = $1`, e.clinicB)
	e.exec(`UPDATE clinics SET kind = 'PEDIATRICS', specialties = '{PSYCHOLOGY}' WHERE id = $1`, e.clinicA)

	keys := func(fields any) map[string]bool {
		out := map[string]bool{}
		for _, f := range fields.([]any) {
			out[f.(map[string]any)["key"].(string)] = true
		}
		return out
	}
	a := e.login("doc_a").expect(200, "GET", "/api/patients/schema", nil)
	if subs := a["subjects"].([]any); len(subs) != 1 || subs[0] != "person" {
		t.Fatalf("a pediatrics clinic sees people: %v", subs)
	}
	pk := keys(sub(a, "profile")["person"])
	for _, want := range []string{"allergies_text", "vaccination", "birth_weight", "prior_therapy", "self_harm_history"} {
		if !pk[want] {
			t.Errorf("pediatrics+psychology form lacks %s", want)
		}
	}
	for _, no := range []string{"species", "breed", "menarche_age", "microchip"} {
		if pk[no] {
			t.Errorf("a person's form must not ask %s", no)
		}
	}
	if a["rx_mode"] != "medication" {
		t.Fatalf("pediatrics prescribes: %v", a["rx_mode"])
	}

	b := e.login("doc_b").expect(200, "GET", "/api/patients/schema", nil)
	if subs := b["subjects"].([]any); len(subs) != 1 || subs[0] != "animal" {
		t.Fatalf("a vet clinic sees animals: %v", subs)
	}
	bk := keys(sub(b, "profile")["animal"])
	if !bk["species"] || !bk["breed"] || !bk["last_deworming"] || bk["tobacco"] || bk["blood_type"] {
		t.Fatalf("animal form: %v", bk)
	}
	mk := keys(sub(b, "measures")["animal"])
	if !mk["mucous"] || mk["bp_sys"] {
		t.Fatalf("animal measures: %v", mk)
	}

	// psychology alone does not prescribe medicines
	e.exec(`UPDATE clinics SET kind = 'PSYCHOLOGY', specialties = '{}' WHERE id = $1`, e.clinicA)
	if m := e.login("doc_a").expect(200, "GET", "/api/patients/schema", nil)["rx_mode"]; m != "instructions" {
		t.Fatalf("psychology: %v", m)
	}
	// a mixed clinic sees both
	e.exec(`UPDATE clinics SET kind = 'GENERAL_MEDICAL', specialties = '{VETERINARY}' WHERE id = $1`, e.clinicA)
	if subs := e.login("doc_a").expect(200, "GET", "/api/patients/schema", nil)["subjects"].([]any); len(subs) != 2 {
		t.Fatalf("mixed clinic: %v", subs)
	}
}

func TestPatientsRegistry(t *testing.T) {
	e := setup(t)
	doc, recep := e.login("doc_a"), e.login("recep_a")

	// validation
	noAck := person(map[string]any{"privacy_ack": false})
	doc.expect(400, "POST", "/api/patients/", noAck)
	doc.expect(400, "POST", "/api/patients/", person(map[string]any{"profile": map[string]any{"chronic_conditions": []any{}}})) // allergies required
	doc.expect(400, "POST", "/api/patients/", person(map[string]any{"profile": map[string]any{"allergies_text": "x", "species": "Perro"}}))
	doc.expect(400, "POST", "/api/patients/", person(map[string]any{"profile": map[string]any{"allergies_text": "x", "tobacco": "Mucho"}}))
	doc.expect(400, "POST", "/api/patients/", person(map[string]any{"curp": "corta"}))
	doc.expect(400, "POST", "/api/patients/", person(map[string]any{"subject": "animal"}))
	doc.expect(400, "POST", "/api/patients/", person(map[string]any{"birth_date": "2999-01-01"}))
	minor := person(map[string]any{"birth_date": time.Now().AddDate(-5, 0, 0).Format("2006-01-02"), "curp": ""})
	doc.expect(400, "POST", "/api/patients/", minor) // a minor needs a responsible adult
	minor["guardian_name"], minor["guardian_phone"], minor["guardian_relation"] = "Marta Medina", "5598765432", "Madre"
	doc.expect(201, "POST", "/api/patients/", minor)

	x := sub(doc.expect(201, "POST", "/api/patients/", person(nil)), "patient")
	if x["curp"] != "MEJJ700312HDFDRR04" || x["file_number"].(float64) != 2 || x["age"].(float64) < 50 || x["incomplete"] != false || x["privacy_notice_at"] == nil {
		t.Fatalf("created: %v", x)
	}
	doc.expect(409, "POST", "/api/patients/", person(nil))
	id := x["id"].(string)

	// reception: finds people to book, never sees clinical data, can register the minimum
	look := recep.expect(200, "GET", "/api/patients/lookup?q=medina", nil)["patients"].([]any)
	if len(look) != 2 {
		t.Fatalf("lookup: %v", look)
	}
	for _, row := range look {
		if _, has := row.(map[string]any)["profile"]; has {
			t.Fatal("lookup must not expose clinical data")
		}
	}
	recep.expect(403, "GET", "/api/patients/", nil)
	recep.expect(403, "GET", "/api/patients/"+id, nil)
	recep.expect(403, "POST", "/api/patients/", person(nil))
	q := sub(recep.expect(201, "POST", "/api/patients/quick", map[string]any{"names": "Lucía", "last_names": "Soto", "phone": "5511112222"}), "patient")
	if q["incomplete"] != true || q["privacy_notice_at"] != nil || len(q["profile"].(map[string]any)) != 0 {
		t.Fatalf("quick registration: %v", q)
	}
	qid := q["id"].(string)
	recep.expect(403, "POST", "/api/patients/"+qid+"/archive", map[string]any{"reason": "x"})
	if sub(recep.expect(200, "POST", "/api/patients/"+qid+"/privacy", nil), "patient")["privacy_notice_at"] == nil {
		t.Fatal("the aviso de privacidad must be recorded")
	}
	recep.expect(409, "POST", "/api/patients/"+qid+"/privacy", nil)

	// the doctor completes the file
	full := person(map[string]any{"names": "Lucía", "last_names": "Soto", "curp": "", "birth_date": "1990-05-05", "sex": "Mujer"})
	got := sub(doc.expect(200, "PUT", "/api/patients/"+qid, full), "patient")
	if got["incomplete"] != false || got["privacy_notice_at"] == nil {
		t.Fatalf("completed: %v", got)
	}

	// search, pending filter, archive instead of delete
	if n := len(doc.expect(200, "GET", "/api/patients/?q=Medina", nil)["patients"].([]any)); n != 2 {
		t.Fatalf("search: %d", n)
	}
	if n := len(doc.expect(200, "GET", "/api/patients/?q=mejj700312hdfdrr04", nil)["patients"].([]any)); n != 1 {
		t.Fatalf("search by curp: %d", n)
	}
	doc.expect(405, "DELETE", "/api/patients/"+id, nil) // there is no way to delete a clinical record
	doc.expect(400, "POST", "/api/patients/"+id+"/archive", map[string]any{"reason": ""})
	doc.expect(200, "POST", "/api/patients/"+id+"/archive", map[string]any{"reason": "Se mudó"})
	if n := len(doc.expect(200, "GET", "/api/patients/?q=mejj700312hdfdrr04", nil)["patients"].([]any)); n != 0 {
		t.Fatalf("archived patients leave the active list: %d", n)
	}
	if n := len(doc.expect(200, "GET", "/api/patients/?archived=1", nil)["patients"].([]any)); n != 1 {
		t.Fatalf("archived list: %d", n)
	}
	doc.expect(200, "GET", "/api/patients/"+id, nil) // the record is still readable
	doc.expect(200, "POST", "/api/patients/"+id+"/unarchive", nil)

	// another clinic sees nothing
	other := e.login("doc_b")
	other.expect(404, "GET", "/api/patients/"+id, nil)
	other.expect(404, "PUT", "/api/patients/"+id, person(nil))
	other.expect(404, "GET", "/api/patients/"+id+"/encounters", nil)
	if len(other.expect(200, "GET", "/api/patients/", nil)["patients"].([]any)) != 0 {
		t.Fatal("tenant leak")
	}

	// who opened the record
	acc := e.login("admin_a").expect(200, "GET", "/api/patients/"+id+"/access", nil)["access"].([]any)
	if len(acc) == 0 || acc[0].(map[string]any)["action"] != "view" {
		t.Fatalf("access log: %v", acc)
	}
	doc.expect(403, "GET", "/api/patients/"+id+"/access", nil)
}

func TestAnimalPatients(t *testing.T) {
	e := setup(t)
	e.exec(`UPDATE clinics SET kind = 'VETERINARY' WHERE id = $1`, e.clinicB)
	vet := e.login("doc_b")
	dog := map[string]any{
		"subject": "animal", "names": "Rocky", "sex": "Macho", "birth_date": "2020-01-10", "privacy_ack": true,
		"guardian_name": "Ana Pérez", "guardian_phone": "5511112222", "guardian_relation": "Propietaria",
		"profile": map[string]any{"species": "Perro", "breed": "Labrador", "allergies_text": "Ninguna conocida", "sterilized": "Sí"},
	}
	bad := func(k string, v any) map[string]any {
		m := map[string]any{}
		for a, b := range dog {
			m[a] = b
		}
		m[k] = v
		return m
	}
	vet.expect(400, "POST", "/api/patients/", bad("curp", "MEJJ700312HDFDRR04"))                     // dogs have no CURP
	vet.expect(400, "POST", "/api/patients/", bad("guardian_phone", ""))                             // the owner must be reachable
	vet.expect(400, "POST", "/api/patients/", bad("profile", map[string]any{"allergies_text": "x"})) // species required
	vet.expect(400, "POST", "/api/patients/", bad("profile", map[string]any{"species": "Perro", "allergies_text": "x", "tobacco": "No"}))
	d := sub(vet.expect(201, "POST", "/api/patients/", dog), "patient")
	if d["subject"] != "animal" || d["last_names"] != "" {
		t.Fatalf("dog: %v", d)
	}
	rows := vet.expect(200, "GET", "/api/patients/", nil)["patients"].([]any)
	if rows[0].(map[string]any)["species"] != "Perro" {
		t.Fatalf("list shows the species: %v", rows[0])
	}
	// a person cannot be registered in a vet clinic
	vet.expect(400, "POST", "/api/patients/", person(nil))
}

func TestEncountersAreAppendOnly(t *testing.T) {
	e := setup(t)
	doc, doc2 := e.login("doc_a"), e.login("admin_a")
	pid := sub(doc.expect(201, "POST", "/api/patients/", person(nil)), "patient")["id"].(string)

	enc := map[string]any{
		"kind": "consulta", "reason": "Dolor de cabeza", "subjective": "Refiere dolor desde hace 3 días, empeora por la tarde.",
		"measures": map[string]any{"weight_kg": "78,5", "bp_sys": 130, "bp_dia": 85, "temp_c": 36.8},
		"exam":     "Sin datos de focalización", "assessment": "Cefalea tensional", "diagnosis_codes": []any{"g44.2"}, "plan": "Paracetamol, hidratación",
	}
	doc.expect(400, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"kind": "consulta"})                                             // empty
	doc.expect(400, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "x", "measures": map[string]any{"mucous": "Rosadas"}}) // an animal measure
	doc.expect(400, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "x", "diagnosis_codes": []any{"cefalea"}})
	doc.expect(400, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "x", "occurred_at": time.Now().AddDate(0, 0, -30).Format(time.RFC3339)})
	doc.expect(400, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "x", "occurred_at": time.Now().AddDate(0, 0, 3).Format(time.RFC3339)})
	e.login("recep_a").expect(403, "POST", "/api/patients/"+pid+"/encounters", enc)

	got := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", enc), "encounter")
	m := got["measures"].(map[string]any)
	if m["weight_kg"].(float64) != 78.5 || got["diagnosis_codes"].([]any)[0] != "G44.2" || got["author_name"] != "DOC_A" || got["author_role"] != "Médico / especialista" {
		t.Fatalf("encounter: %v", got)
	}
	id := got["id"].(string)

	// no editing, no deleting
	doc.expect(404, "PUT", "/api/patients/"+pid+"/encounters/"+id, enc)
	doc.expect(404, "PUT", "/api/encounters/"+id, enc)
	doc.expect(404, "DELETE", "/api/encounters/"+id, nil)

	// a correction is a new entry that points at the old one
	doc.expect(400, "POST", "/api/encounters/"+id+"/addendum", map[string]any{"reason": "", "text": "x"})
	ad := sub(doc.expect(201, "POST", "/api/encounters/"+id+"/addendum", map[string]any{"reason": "Peso mal capturado", "text": "El peso correcto es 79.2 kg."}), "encounter")
	if ad["kind"] != "adenda" || ad["addendum_of"] != id {
		t.Fatalf("addendum: %v", ad)
	}
	list := doc.expect(200, "GET", "/api/patients/"+pid+"/encounters", nil)["encounters"].([]any)
	if len(list) != 2 || list[1].(map[string]any)["reason"] != "Dolor de cabeza" || list[1].(map[string]any)["measures"].(map[string]any)["weight_kg"].(float64) != 78.5 {
		t.Fatalf("the original must stay untouched: %v", list)
	}
	e.login("doc_b").expect(404, "POST", "/api/encounters/"+id+"/addendum", map[string]any{"reason": "x", "text": "y"})

	// private notes: only their author reads them
	priv := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"kind": "nota", "notes": "Contenido confidencial de sesión", "private": true}), "encounter")
	seen := doc2.expect(200, "GET", "/api/patients/"+pid+"/encounters", nil)["encounters"].([]any)
	var hidden map[string]any
	for _, x := range seen {
		if x.(map[string]any)["id"] == priv["id"] {
			hidden = x.(map[string]any)
		}
	}
	if hidden == nil || hidden["hidden"] != true || hidden["notes"] != "" {
		t.Fatalf("someone else's private note must be withheld: %v", hidden)
	}
	doc2.expect(403, "POST", "/api/encounters/"+priv["id"].(string)+"/addendum", map[string]any{"reason": "x", "text": "y"})
	mine := doc.expect(200, "GET", "/api/patients/"+pid+"/encounters", nil)["encounters"].([]any)
	if mine[0].(map[string]any)["notes"] != "Contenido confidencial de sesión" {
		t.Fatalf("the author reads their own note: %v", mine[0])
	}

	// the printable record leaves other people's private notes out, and is logged
	rec := doc2.expect(200, "GET", "/api/patients/"+pid+"/record", nil)
	if n := len(rec["encounters"].([]any)); n != 2 {
		t.Fatalf("record encounters: %d", n)
	}
	if rec["printed_by"] != "Admin a" || rec["clinic"] == nil {
		t.Fatalf("record: %v", rec)
	}
	acc := e.login("admin_a").expect(200, "GET", "/api/patients/"+pid+"/access", nil)["access"].([]any)
	printed := false
	for _, a := range acc {
		printed = printed || a.(map[string]any)["action"] == "print"
	}
	if !printed {
		t.Fatalf("printing must be logged: %v", acc)
	}
	// last visit is kept for the retention rule
	p := sub(doc.expect(200, "GET", "/api/patients/"+pid, nil), "patient")
	if p["last_encounter_at"] == nil {
		t.Fatal("last_encounter_at")
	}
}

func TestPrescriptions(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")
	pid := sub(doc.expect(201, "POST", "/api/patients/", person(nil)), "patient")["id"].(string)
	item := map[string]any{"medicine": "Paracetamol", "brand": "Tempra", "presentation": "Tabletas 500 mg", "dose": "1 tableta", "route": "Oral", "frequency": "Cada 8 horas", "duration": "3 días", "quantity": "1 caja"}
	rx := func(items ...map[string]any) map[string]any {
		arr := []any{}
		for _, i := range items {
			arr = append(arr, i)
		}
		return map[string]any{"diagnosis": "Cefalea tensional", "items": arr, "instructions": "Reposo e hidratación.", "next_visit": "2030-01-10"}
	}

	// no cédula, no receta
	if code, out := doc.do("POST", "/api/patients/"+pid+"/prescriptions", rx(item)); code != 409 || out["code"] != "CEDULA_REQUIRED" {
		t.Fatalf("without cédula: %d %v", code, out)
	}
	doc.expect(400, "PUT", "/api/me", map[string]any{"name": "Dr. Ejemplo", "phone": "", "cedula": "abc"})
	me := sub(doc.expect(200, "PUT", "/api/me", map[string]any{"name": "Dr. Ejemplo", "phone": "", "cedula": "12345678", "cedula_institution": "UNAM", "specialty_title": "Médico Cirujano"}), "session")
	if pro := sub(me, "professional"); pro["cedula"] != "12345678" || pro["institution"] != "UNAM" || pro["title"] != "Médico Cirujano" {
		t.Fatalf("professional data: %v", pro)
	}
	// a later profile edit without those fields does not wipe them
	doc.expect(200, "PUT", "/api/me", map[string]any{"name": "Dr. Ejemplo", "phone": "55"})
	if sub(sub(doc.expect(200, "GET", "/api/session", nil), "session"), "professional")["cedula"] != "12345678" {
		t.Fatal("professional data must survive")
	}

	bad := func(mod func(m map[string]any)) map[string]any {
		it := map[string]any{}
		for k, v := range item {
			it[k] = v
		}
		mod(it)
		return rx(it)
	}
	doc.expect(400, "POST", "/api/patients/"+pid+"/prescriptions", rx())
	doc.expect(400, "POST", "/api/patients/"+pid+"/prescriptions", bad(func(m map[string]any) { m["medicine"] = "" }))
	doc.expect(400, "POST", "/api/patients/"+pid+"/prescriptions", bad(func(m map[string]any) { m["duration"] = "" }))
	doc.expect(400, "POST", "/api/patients/"+pid+"/prescriptions", bad(func(m map[string]any) { m["route"] = "Por el aire" }))
	if code, out := doc.do("POST", "/api/patients/"+pid+"/prescriptions", bad(func(m map[string]any) { m["control"] = "Fracción I o II" })); code != 422 || out["code"] != "CONTROLLED" {
		t.Fatalf("controlled substances: %d %v", code, out)
	}
	over := rx(item)
	over["valid_days"] = 90
	doc.expect(400, "POST", "/api/patients/"+pid+"/prescriptions", over)

	r1 := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/prescriptions", rx(item)), "prescription")
	abx := map[string]any{"medicine": "Amoxicilina", "dose": "500 mg", "route": "Oral", "frequency": "Cada 8 horas", "duration": "7 días", "control": "Antibiótico"}
	r2 := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/prescriptions", rx(item, abx)), "prescription")
	if r1["folio"].(float64) != 1 || r2["folio"].(float64) != 2 || r1["author_license"] != "12345678" || r1["author_title"] != "Médico Cirujano" || r1["valid_until"] == nil || len(r2["items"].([]any)) != 2 {
		t.Fatalf("prescriptions: %v %v", r1, r2)
	}
	list := doc.expect(200, "GET", "/api/patients/"+pid+"/prescriptions", nil)["prescriptions"].([]any)
	if len(list) != 2 {
		t.Fatalf("list: %d", len(list))
	}

	// print data carries the establishment and the patient
	e.login("admin_a").expect(200, "PUT", "/api/clinic/legal", map[string]any{"responsible_name": "Dr. Responsable", "responsible_license": "7654321", "operating_notice": "AF-123", "privacy_email": "privacidad@clinica.mx", "privacy_contact": "Administración"})
	pd := doc.expect(200, "GET", "/api/prescriptions/"+r1["id"].(string), nil)
	if sub(pd, "clinic")["name"] != "Clinica a" || sub(sub(pd, "clinic"), "legal")["responsible_license"] != "7654321" || sub(pd, "patient")["names"] != "Jorge" {
		t.Fatalf("print data: %v", pd)
	}
	e.login("doc_b").expect(404, "GET", "/api/prescriptions/"+r1["id"].(string), nil)
	e.login("recep_a").expect(403, "GET", "/api/prescriptions/"+r1["id"].(string), nil)

	// voiding keeps the record
	other := e.login("admin_a")
	doc.expect(400, "POST", "/api/prescriptions/"+r1["id"].(string)+"/void", map[string]any{"reason": ""})
	doc.expect(200, "POST", "/api/prescriptions/"+r1["id"].(string)+"/void", map[string]any{"reason": "Dosis equivocada"})
	doc.expect(409, "POST", "/api/prescriptions/"+r1["id"].(string)+"/void", map[string]any{"reason": "otra vez"})
	other.expect(200, "POST", "/api/prescriptions/"+r2["id"].(string)+"/void", map[string]any{"reason": "Paciente alérgico"}) // an admin can
	got := doc.expect(200, "GET", "/api/prescriptions/"+r1["id"].(string), nil)
	if sub(got, "prescription")["voided_at"] == nil || sub(got, "prescription")["void_reason"] != "Dosis equivocada" {
		t.Fatalf("voided: %v", got)
	}
}

func TestPrescriptionsWithoutMedicines(t *testing.T) {
	e := setup(t)
	e.exec(`UPDATE clinics SET kind = 'PSYCHOLOGY' WHERE id = $1`, e.clinicA)
	doc := e.login("doc_a")
	doc.expect(200, "PUT", "/api/me", map[string]any{"name": "Psic. Ejemplo", "phone": "", "cedula": "1234567", "cedula_institution": "UNAM", "specialty_title": "Psicólogo clínico"})
	pid := sub(doc.expect(201, "POST", "/api/patients/", person(nil)), "patient")["id"].(string)
	// psychologists give indications, not prescriptions of drugs
	doc.expect(400, "POST", "/api/patients/"+pid+"/prescriptions", map[string]any{"items": []any{map[string]any{"medicine": "Sertralina", "dose": "50 mg", "route": "Oral", "frequency": "24 h", "duration": "30 días"}}, "instructions": "x"})
	doc.expect(400, "POST", "/api/patients/"+pid+"/prescriptions", map[string]any{"instructions": ""})
	r := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/prescriptions", map[string]any{"instructions": "Registro de pensamientos diario. Técnica de respiración 4-7-8 antes de dormir.", "next_visit": "2030-02-01"}), "prescription")
	if r["mode"] != "instructions" || len(r["items"].([]any)) != 0 {
		t.Fatalf("indications sheet: %v", r)
	}

	// a vet in a mixed clinic does prescribe for animals
	e.exec(`UPDATE clinics SET kind = 'PSYCHOLOGY', specialties = '{VETERINARY}' WHERE id = $1`, e.clinicA)
	dog := sub(doc.expect(201, "POST", "/api/patients/", map[string]any{"subject": "animal", "names": "Luna", "privacy_ack": true, "guardian_name": "Ana", "guardian_phone": "55",
		"profile": map[string]any{"species": "Gato", "allergies_text": "Ninguna"}}), "patient")["id"].(string)
	doc.expect(201, "POST", "/api/patients/"+dog+"/prescriptions", map[string]any{"items": []any{map[string]any{"medicine": "Meloxicam", "dose": "0.1 mg/kg", "route": "Oral", "frequency": "24 h", "duration": "3 días"}}})
}

func TestLegalAndCompliance(t *testing.T) {
	e := setup(t)
	admin := e.login("admin_a")
	find := func(items []any, key string) map[string]any {
		for _, i := range items {
			if i.(map[string]any)["key"] == key {
				return i.(map[string]any)
			}
		}
		t.Fatalf("missing checklist item %s", key)
		return nil
	}
	items := admin.expect(200, "GET", "/api/clinic/compliance", nil)["items"].([]any)
	for _, k := range []string{"responsible", "operating_notice", "privacy_contact", "licenses"} {
		if find(items, k)["status"] != "todo" {
			t.Fatalf("%s should be pending on a new clinic", k)
		}
	}
	if find(items, "retention")["status"] != "ok" {
		t.Fatal("retention is guaranteed by design")
	}
	e.login("doc_a").expect(403, "GET", "/api/clinic/compliance", nil)
	e.login("doc_a").expect(403, "PUT", "/api/clinic/legal", map[string]any{})
	admin.expect(400, "PUT", "/api/clinic/legal", map[string]any{"privacy_email": "sin-arroba"})
	admin.expect(200, "PUT", "/api/clinic/legal", map[string]any{"responsible_name": "Dr. R", "responsible_license": "1234567", "operating_notice": "AF-1", "privacy_contact": "Admin", "privacy_email": "p@c.mx"})
	if l := sub(e.login("recep_a").expect(200, "GET", "/api/clinic/legal", nil), "legal"); l["operating_notice"] != "AF-1" {
		t.Fatalf("legal: %v", l)
	}
	items = admin.expect(200, "GET", "/api/clinic/compliance", nil)["items"].([]any)
	for _, k := range []string{"responsible", "operating_notice", "privacy_contact"} {
		if find(items, k)["status"] != "ok" {
			t.Fatalf("%s should be done", k)
		}
	}

	// a quick-registered patient makes the checklist flag the missing aviso de privacidad
	e.login("recep_a").expect(201, "POST", "/api/patients/quick", map[string]any{"names": "Ana", "last_names": "Ruiz"})
	items = admin.expect(200, "GET", "/api/clinic/compliance", nil)["items"].([]any)
	if d := find(items, "patient_notice"); d["status"] != "todo" || !strings.HasPrefix(d["detail"].(string), "1 de 1") {
		t.Fatalf("patient_notice: %v", d)
	}
}

func TestAppointmentsWithPatients(t *testing.T) {
	e := setup(t)
	doc, recep := e.login("doc_a"), e.login("recep_a")
	pid := sub(doc.expect(201, "POST", "/api/patients/", person(nil)), "patient")["id"].(string)

	ok := map[string]any{"patient_id": pid, "names": "Jorge", "last_names": "Medina", "date": "2030-05-01", "startHour": "09:00", "endHour": "10:00", "details": "Control"}
	a := sub(recep.expect(201, "POST", "/api/appointments", ok), "appointment")
	if a["patient_id"] != pid {
		t.Fatalf("appointment: %v", a)
	}
	// an animal has no surname and no CURP
	e.exec(`UPDATE clinics SET kind = 'GENERAL_MEDICAL', specialties = '{VETERINARY}' WHERE id = $1`, e.clinicA)
	dog := sub(doc.expect(201, "POST", "/api/patients/", map[string]any{"subject": "animal", "names": "Luna", "privacy_ack": true, "guardian_name": "Ana", "guardian_phone": "55",
		"profile": map[string]any{"species": "Gato", "allergies_text": "Ninguna"}}), "patient")["id"].(string)
	recep.expect(201, "POST", "/api/appointments", map[string]any{"patient_id": dog, "names": "Luna", "date": "2030-05-02", "startHour": "09:00", "endHour": "09:30"})
	// without a registered patient the surname is still required
	recep.expect(400, "POST", "/api/appointments", map[string]any{"names": "Luna", "date": "2030-05-02", "startHour": "10:00", "endHour": "10:30"})
	// a patient of another clinic cannot be used
	recep.expect(400, "POST", "/api/appointments", map[string]any{"patient_id": sub(e.login("doc_b").expect(201, "POST", "/api/patients/", person(nil)), "patient")["id"], "names": "X", "last_names": "Y", "date": "2030-05-03", "startHour": "09:00", "endHour": "09:30"})
}
