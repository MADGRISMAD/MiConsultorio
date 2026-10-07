package api_test

import (
	"context"
	"crypto/rand"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/fieldcrypt"
)

var encTestKey = []byte(strings.Repeat("k", 32)) // TokenEncKey of setup()

func encRing(t *testing.T) *fieldcrypt.Ring {
	t.Helper()
	r, err := fieldcrypt.NewRing(encTestKey)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// encSeed writes clinical content through the API: a patient with antecedentes, a consultation with every free-text
// section, an addendum, a private note, a receta and an appointment. Returns the patient id.
func encSeed(t *testing.T, e *env) (patient string) {
	t.Helper()
	doc := rxDoctor(t, e, "doc_a")
	patient = sub(doc.expect(201, "POST", "/api/patients/", person(map[string]any{
		"profile": map[string]any{"allergies_text": "Alergia a las penicilinas SECRETOALERGIA", "chronic_conditions": []any{"Diabetes"}},
	})), "patient")["id"].(string)
	enc := sub(doc.expect(201, "POST", "/api/patients/"+patient+"/encounters", map[string]any{
		"kind": "consulta", "reason": "motivo visible", "subjective": "SECRETOSUBJ refiere dolor", "exam": "SECRETOEXAM sin hallazgos",
		"assessment": "SECRETODIAG cefalea", "plan": "SECRETOPLAN reposo", "notes": "SECRETONOTAS", "measures": map[string]any{"weight_kg": 80},
	}), "encounter")["id"].(string)
	doc.expect(201, "POST", "/api/encounters/"+enc+"/addendum", map[string]any{"reason": "corrección", "text": "SECRETOADENDA peso 81"})
	doc.expect(201, "POST", "/api/patients/"+patient+"/encounters", map[string]any{"kind": "nota", "notes": "SECRETOPRIVADA", "private": true})
	body := rxBody(rxItem("Paracetamol", nil))
	body["diagnosis"], body["instructions"] = "SECRETORXDIAG", "SECRETORXINSTR con alimentos"
	doc.expect(201, "POST", "/api/patients/"+patient+"/prescriptions", body)
	e.login("recep_a").expect(201, "POST", "/api/appointments", map[string]any{"patient_id": patient, "names": "Jorge", "last_names": "Medina",
		"date": "2030-05-01", "startHour": "09:00", "endHour": "10:00", "details": "SECRETOCITA control de diabetes"})
	return patient
}

// encColumnDump is every sealed column of the clinic as one string (what someone with database access would see).
func encColumnDump(t *testing.T, e *env) string {
	t.Helper()
	var sb strings.Builder
	rows, err := e.pool.Query(context.Background(), `
		SELECT concat_ws('|', subjective, exam, assessment, plan, notes) FROM encounters
		UNION ALL SELECT concat_ws('|', diagnosis, instructions, items::text) FROM prescriptions
		UNION ALL SELECT profile::text FROM patients
		UNION ALL SELECT details FROM appointments`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		sb.WriteString(s + "\n")
	}
	return sb.String()
}

func TestEncFieldsAtRest(t *testing.T) {
	e := setup(t)
	pid := encSeed(t, e)

	dump := encColumnDump(t, e)
	for _, secret := range []string{"SECRETO", "penicilinas", "Diabetes", "diabetes"} {
		if strings.Contains(dump, secret) {
			t.Fatalf("%q is stored in clear:\n%s", secret, dump)
		}
	}
	var n int
	e.pool.QueryRow(ctx(), `SELECT count(*) FROM encounters WHERE (subjective <> '' AND subjective NOT LIKE 'enc:v1:%') OR (exam <> '' AND exam NOT LIKE 'enc:v1:%')
		OR (assessment <> '' AND assessment NOT LIKE 'enc:v1:%') OR (plan <> '' AND plan NOT LIKE 'enc:v1:%') OR (notes <> '' AND notes NOT LIKE 'enc:v1:%')`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d encounters with clear text", n)
	}
	e.pool.QueryRow(ctx(), `SELECT count(*) FROM prescriptions WHERE diagnosis NOT LIKE 'enc:v1:%' OR instructions NOT LIKE 'enc:v1:%'`).Scan(&n)
	if n != 0 {
		t.Fatal("receta in clear")
	}
	e.pool.QueryRow(ctx(), `SELECT count(*) FROM appointments WHERE details NOT LIKE 'enc:v1:%'`).Scan(&n)
	if n != 0 {
		t.Fatal("appointment details in clear")
	}
	// what reports and lists group by stays readable
	var reason, med string
	e.pool.QueryRow(ctx(), `SELECT reason FROM encounters WHERE kind = 'consulta' AND patient_id = $1`, pid).Scan(&reason)
	e.pool.QueryRow(ctx(), `SELECT items->0->>'medicine' FROM prescriptions WHERE patient_id = $1`, pid).Scan(&med)
	if reason != "motivo visible" || med != "Paracetamol" {
		t.Fatalf("reason %q / medicine %q must stay readable", reason, med)
	}

	// the API reads everything back in clear and never leaks the sealed form
	doc, admin := e.login("doc_a"), e.login("admin_a")
	bodies := map[string]map[string]any{
		"patient":  doc.expect(200, "GET", "/api/patients/"+pid, nil),
		"list":     doc.expect(200, "GET", "/api/patients/", nil),
		"encs":     doc.expect(200, "GET", "/api/patients/"+pid+"/encounters", nil),
		"rxs":      doc.expect(200, "GET", "/api/patients/"+pid+"/prescriptions", nil),
		"record":   admin.expect(200, "GET", "/api/patients/"+pid+"/record", nil),
		"export":   admin.expect(200, "GET", "/api/patients/"+pid+"/export", nil),
		"agenda":   e.login("recep_a").expect(200, "GET", "/api/appointments?from=2030-05-01&to=2030-05-01", nil),
		"schedule": e.login("recep_a").expect(200, "GET", "/api/appointments", nil),
	}
	for name, b := range bodies {
		s := raw(b)
		if strings.Contains(s, "enc:v1:") || strings.Contains(s, `"_enc":`) {
			t.Fatalf("%s leaks the sealed form: %s", name, s)
		}
	}
	if p := sub(bodies["patient"], "patient")["profile"].(map[string]any); p["allergies_text"] != "Alergia a las penicilinas SECRETOALERGIA" {
		t.Fatalf("profile: %v", p)
	}
	for name, wants := range map[string][]string{
		"encs":   {"SECRETOSUBJ", "SECRETOEXAM", "SECRETODIAG", "SECRETOPLAN", "SECRETONOTAS", "SECRETOADENDA", "SECRETOPRIVADA"},
		"rxs":    {"SECRETORXDIAG", "SECRETORXINSTR"},
		"record": {"SECRETOSUBJ", "SECRETORXINSTR", "SECRETOALERGIA", "SECRETOADENDA"},
		"export": {"SECRETOSUBJ", "SECRETORXDIAG", "SECRETOALERGIA", "SECRETOCITA", "SECRETOADENDA"},
	} {
		s := raw(bodies[name])
		for _, want := range wants {
			if !strings.Contains(s, want) {
				t.Fatalf("%s lacks %q", name, want)
			}
		}
	}
	if !strings.Contains(raw(bodies["schedule"]), "SECRETOCITA") {
		t.Fatalf("agenda lacks the appointment details")
	}

	// the allergy check reads the sealed profile
	code, out := doc.do("POST", "/api/patients/"+pid+"/prescriptions", rxBody(rxItem("Amoxicilina", nil)))
	if code != 409 || out["code"] != "ALLERGY_CONFLICT" {
		t.Fatalf("allergy check over a sealed profile: %d %v", code, out)
	}
	// editing the patient keeps working and keeps the profile sealed
	cur := sub(bodies["patient"], "patient")
	upd := person(map[string]any{"profile": map[string]any{"allergies_text": "Ninguna conocida SECRETOEDIT"}, "birth_date": cur["birth_date"]})
	doc.expect(200, "PUT", "/api/patients/"+pid, upd)
	if strings.Contains(encColumnDump(t, e), "SECRETOEDIT") {
		t.Fatal("an updated profile is stored in clear")
	}
	if got := sub(doc.expect(200, "GET", "/api/patients/"+pid, nil), "patient")["profile"].(map[string]any)["allergies_text"]; got != "Ninguna conocida SECRETOEDIT" {
		t.Fatalf("updated profile: %v", got)
	}
	// editing an appointment reseals its details
	appts := e.login("recep_a").expect(200, "GET", "/api/appointments", nil)["appointments"].([]any)
	aid := appts[0].(map[string]any)["id"].(string)
	e.login("recep_a").expect(200, "PUT", "/api/appointments/"+aid, map[string]any{"patient_id": pid, "names": "Jorge", "last_names": "Medina",
		"date": "2030-05-01", "startHour": "09:00", "endHour": "10:00", "details": "SECRETOCITA2"})
	if strings.Contains(encColumnDump(t, e), "SECRETOCITA2") {
		t.Fatal("updated details stored in clear")
	}
	if got := sub(e.login("recep_a").expect(200, "GET", "/api/appointments/"+aid, nil), "appointment")["details"]; got != "SECRETOCITA2" {
		t.Fatalf("details after update: %v", got)
	}
}

func TestEncFieldsOnlineBookingAndPortal(t *testing.T) {
	p := newPortalEnv(t)
	doc := rxDoctor(t, p.env, "doc_a")
	body := rxBody(rxItem("Paracetamol", nil))
	body["instructions"] = "Tomar con alimentos SECRETOPORTAL"
	doc.expect(201, "POST", "/api/patients/"+p.patA+"/prescriptions", body)
	var instr string
	p.pool.QueryRow(ctx(), `SELECT instructions FROM prescriptions WHERE patient_id = $1`, p.patA).Scan(&instr)
	if !strings.HasPrefix(instr, "enc:v1:") {
		t.Fatalf("stored: %q", instr)
	}
	c := p.signIn("clinica-a", "dueno@mail.mx")
	rx := raw(c.expect(200, "GET", "/api/portal/prescriptions", nil))
	if !strings.Contains(rx, "SECRETOPORTAL") || strings.Contains(rx, "enc:v1:") {
		t.Fatalf("portal rx: %s", rx)
	}
	// the portal list of patients still shows the species (kept in clear next to the sealed profile)
	if me := raw(c.expect(200, "GET", "/api/portal/me", nil)); strings.Contains(me, "enc:v1:") {
		t.Fatalf("portal me: %s", me)
	}
}

func TestEncFieldsLegacyRowsAndCommand(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")
	var pid, eid, rid, aid string
	e.pool.QueryRow(ctx(), `INSERT INTO patients (clinic_id, file_number, subject, names, last_names, profile)
		VALUES ($1, 1, 'person', 'Ana', 'Vieja', '{"allergies_text":"Sulfas LEGADOALERGIA","species":"Perro"}') RETURNING id::text`, e.clinicA).Scan(&pid)
	e.pool.QueryRow(ctx(), `INSERT INTO encounters (clinic_id, patient_id, kind, reason, subjective, exam, assessment, plan, notes, author_name)
		VALUES ($1,$2,'consulta','motivo','LEGADOSUBJ','LEGADOEXAM','LEGADODIAG','LEGADOPLAN','LEGADONOTAS','Dr') RETURNING id::text`, e.clinicA, pid).Scan(&eid)
	e.pool.QueryRow(ctx(), `INSERT INTO prescriptions (clinic_id, patient_id, folio, diagnosis, items, instructions, author_name, verify_token)
		VALUES ($1,$2,1,'LEGADORXDIAG','[]','LEGADORXINSTR','Dr', md5(random()::text)) RETURNING id::text`, e.clinicA, pid).Scan(&rid)
	e.pool.QueryRow(ctx(), `INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, details)
		VALUES ($1,'','Ana','Vieja','2030-05-01','09:00','10:00','LEGADOCITA') RETURNING id::text`, e.clinicA).Scan(&aid)

	readAll := func() string {
		return raw(map[string]any{
			"p": doc.expect(200, "GET", "/api/patients/"+pid, nil), "e": doc.expect(200, "GET", "/api/patients/"+pid+"/encounters", nil),
			"r": doc.expect(200, "GET", "/api/patients/"+pid+"/prescriptions", nil), "a": doc.expect(200, "GET", "/api/appointments/"+aid, nil),
		})
	}
	before := readAll()
	for _, want := range []string{"LEGADOALERGIA", "LEGADOSUBJ", "LEGADOEXAM", "LEGADODIAG", "LEGADOPLAN", "LEGADONOTAS", "LEGADORXDIAG", "LEGADORXINSTR", "LEGADOCITA"} {
		if !strings.Contains(before, want) {
			t.Fatalf("legacy plaintext must stay readable: lacks %s", want)
		}
	}
	snapshot := func() string {
		var s string
		e.pool.QueryRow(ctx(), `SELECT (SELECT string_agg(concat_ws('|', subjective, exam, assessment, plan, notes), ',' ORDER BY id) FROM encounters)
			|| (SELECT string_agg(concat_ws('|', diagnosis, instructions), ',' ORDER BY id) FROM prescriptions)
			|| (SELECT string_agg(profile::text, ',' ORDER BY id) FROM patients) || (SELECT string_agg(details, ',' ORDER BY id) FROM appointments)`).Scan(&s)
		return s
	}
	ring := encRing(t)
	plainSnap := snapshot()

	// dry run: counts, writes nothing
	stats, err := fieldcrypt.EncryptAll(ctx(), e.pool, ring, fieldcrypt.Options{DryRun: true, Batch: 1})
	if err != nil {
		t.Fatal(err)
	}
	sealed := 0
	for _, s := range stats {
		sealed += s.Sealed
	}
	if sealed != 9 || snapshot() != plainSnap {
		t.Fatalf("dry run: would seal %d (want 9), unchanged=%v", sealed, snapshot() == plainSnap)
	}

	// real run (batches of one row to exercise the paging)
	if _, err := fieldcrypt.EncryptAll(ctx(), e.pool, ring, fieldcrypt.Options{Batch: 1}); err != nil {
		t.Fatal(err)
	}
	dump := encColumnDump(t, e)
	if strings.Contains(dump, "LEGADO") {
		t.Fatalf("still in clear after the command:\n%s", dump)
	}
	var species string
	e.pool.QueryRow(ctx(), `SELECT profile->>'species' FROM patients WHERE id = $1`, pid).Scan(&species)
	if species != "Perro" {
		t.Fatalf("species must stay queryable: %q", species)
	}
	if after := readAll(); after != before {
		t.Fatalf("the API answers differently after encrypting:\n%s\n%s", before, after)
	}

	// idempotent: a second run changes nothing
	sealedSnap := snapshot()
	stats, err = fieldcrypt.EncryptAll(ctx(), e.pool, ring, fieldcrypt.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range stats {
		if s.Sealed != 0 || s.Rekeyed != 0 || s.Undecrypt != 0 || s.Already != s.Scanned {
			t.Fatalf("second run: %+v", s)
		}
	}
	if snapshot() != sealedSnap {
		t.Fatal("a second run rewrote values")
	}

	// key rotation: a ring with a new current key re-seals under it, keeps opening the old ones
	key2 := make([]byte, 32)
	_, _ = rand.Read(key2)
	ring2 := encRing(t)
	if err := ring2.Add(2, key2); err != nil {
		t.Fatal(err)
	}
	if err := ring2.SetCurrent(2); err != nil {
		t.Fatal(err)
	}
	stats, err = fieldcrypt.EncryptAll(ctx(), e.pool, ring2, fieldcrypt.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rekeyed := 0
	for _, s := range stats {
		rekeyed += s.Rekeyed
	}
	if rekeyed != 9 {
		t.Fatalf("rekeyed %d, want 9", rekeyed)
	}
	var notes string
	e.pool.QueryRow(ctx(), `SELECT notes FROM encounters WHERE id = $1`, eid).Scan(&notes)
	if got, err := ring2.Open("encounters", "notes", eid, notes); err != nil || got != "LEGADONOTAS" || fieldcrypt.KeyID(notes) != 2 {
		t.Fatalf("rekeyed value: %q %v key %d", got, err, fieldcrypt.KeyID(notes))
	}
}

func TestEncFieldsReportsUnchanged(t *testing.T) {
	e := setup(t)
	rptSeed(e)
	e.exec(`UPDATE encounters SET subjective = 'SECRETO', notes = 'SECRETO'`)
	e.exec(`UPDATE appointments SET details = 'SECRETO'`)
	a := e.login("admin_a")
	paths := []string{"/api/reports/patients" + rptJan, "/api/reports/operations" + rptJan}
	before := map[string]string{}
	for _, p := range paths {
		before[p] = raw(a.expect(200, "GET", p, nil))
	}
	if _, err := fieldcrypt.EncryptAll(ctx(), e.pool, encRing(t), fieldcrypt.Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encColumnDump(t, e), "SECRETO") {
		t.Fatal("not sealed")
	}
	for _, p := range paths {
		if got := raw(a.expect(200, "GET", p, nil)); got != before[p] {
			t.Fatalf("%s changed after encrypting:\n%s\n%s", p, before[p], got)
		}
	}
	// the animals report groups by the species kept in clear
	if !strings.Contains(before["/api/reports/patients"+rptJan], "Perro") {
		t.Fatal("species missing from the report")
	}
}

func TestEncFieldsValueMovedBetweenRows(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")
	pid := sub(doc.expect(201, "POST", "/api/patients/", person(nil)), "patient")["id"].(string)
	mk := func(text string) string {
		return sub(doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "r", "notes": text}), "encounter")["id"].(string)
	}
	a, b := mk("NOTA UNO"), mk("NOTA DOS")
	doc.expect(200, "GET", "/api/patients/"+pid+"/encounters", nil)
	// someone with database access copies the sealed value of row A over row B
	e.exec(`UPDATE encounters SET notes = (SELECT notes FROM encounters WHERE id = $1) WHERE id = $2`, a, b)
	code, out := doc.do("GET", "/api/patients/"+pid+"/encounters", nil)
	if code != 500 || out["code"] != "DECRYPT_FAILED" {
		t.Fatalf("moved value must not open: %d %v", code, out)
	}
	if strings.Contains(raw(out), "NOTA") || strings.Contains(raw(out), "enc:v1:") {
		t.Fatalf("the error leaks content: %v", out)
	}
	var n int
	e.pool.QueryRow(ctx(), `SELECT count(*) FROM activity_log WHERE clinic_id = $1 AND type = 'decrypt_failed'`, e.clinicA).Scan(&n)
	if n != 1 {
		t.Fatalf("audited failures: %d", n)
	}
	// the command leaves unreadable values alone and reports them
	stats, err := fieldcrypt.EncryptAll(ctx(), e.pool, encRing(t), fieldcrypt.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range stats {
		if s.Table == "encounters" && s.Column == "notes" && s.Undecrypt != 0 && s.Undecrypt != 1 {
			t.Fatalf("stats: %+v", s)
		}
	}
}

func TestEncFieldsWrongKeyFailsControlled(t *testing.T) {
	e := setup(t)
	pid := encSeed(t, e)

	// the same database behind a server configured with another JWT_SECRET / TOKEN_ENC_KEY
	cfg := &config.Config{
		JWTSecret: []byte(strings.Repeat("s", 40)), SessionTTL: 3600e9,
		OAuthStateSecret: []byte(strings.Repeat("o", 32)), TokenEncKey: []byte(strings.Repeat("X", 32)),
		MPAPIBase: "http://127.0.0.1:1", MPCurrency: "MXN", APIPublicURL: "http://api.test", AppURL: "http://app.test",
		PlanPriceMonth: map[string]int{}, PlanPriceYear: map[string]int{},
	}
	srv := httptest.NewServer(api.NewRouterWithMailer(e.pool, cfg, e.mail))
	t.Cleanup(srv.Close)
	e2 := *e
	e2.srv = srv
	doc := e2.login("doc_a")

	for _, path := range []string{"/api/patients/" + pid, "/api/patients/" + pid + "/encounters", "/api/patients/" + pid + "/prescriptions",
		"/api/patients/" + pid + "/record", "/api/patients/" + pid + "/export", "/api/appointments"} {
		code, out := doc.do("GET", path, nil)
		if code != 500 || out["code"] != "DECRYPT_FAILED" || out["message"] == nil {
			t.Fatalf("%s: %d %v", path, code, out)
		}
		if s := raw(out); strings.Contains(s, "enc:v1:") || strings.Contains(s, "SECRETO") {
			t.Fatalf("%s shows sealed or clinical content in the error: %s", path, s)
		}
	}
	// what is not sealed keeps working: lists, health, logins
	doc.expect(200, "GET", "/api/patients/", nil)
	e2.anon().expect(200, "GET", "/api/health", nil)
	// the receta check cannot read the allergies: it refuses instead of issuing blind
	if code, out := doc.do("POST", "/api/patients/"+pid+"/prescriptions", rxBody(rxItem("Amoxicilina", nil))); code != 500 || out["code"] != "DECRYPT_FAILED" {
		t.Fatalf("issuing a receta with unreadable allergies: %d %v", code, out)
	}
	// new content is sealed with the new key and read back
	newEnc := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "nuevo", "notes": "NOTA NUEVA"}), "encounter")
	if newEnc["notes"] != "NOTA NUEVA" {
		t.Fatalf("new encounter: %v", newEnc)
	}
	var n int
	e.pool.QueryRow(ctx(), `SELECT count(*) FROM activity_log WHERE clinic_id = $1 AND type = 'decrypt_failed'`, e.clinicA).Scan(&n)
	if n < 6 {
		t.Fatalf("failures must be audited: %d", n)
	}
}
