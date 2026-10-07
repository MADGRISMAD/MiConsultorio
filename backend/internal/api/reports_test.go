package api_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func rptNum(m map[string]any, keys ...string) float64 {
	var cur any = m
	for _, k := range keys {
		cur = cur.(map[string]any)[k]
	}
	return cur.(float64)
}

func (e *env) rptPatient(clinic string, n int, subject, names, sex, birth, created, last, species string) string {
	e.t.Helper()
	var id string
	err := e.pool.QueryRow(context.Background(), `
		INSERT INTO patients (clinic_id, file_number, subject, names, last_names, sex, birth_date, created_at, last_encounter_at, profile, guardian_name, guardian_phone, guardian_email)
		VALUES ($1, $2, $3, $4, 'Prueba', $5, nullif($6,'')::date, $7::timestamptz, nullif($8,'')::timestamptz,
		        CASE WHEN $9 = '' THEN '{}'::jsonb ELSE jsonb_build_object('species', $9::text) END,
		        CASE WHEN $3 = 'animal' THEN 'Tutor ' || $4 ELSE '' END, CASE WHEN $3 = 'animal' THEN '5512345678' ELSE '' END, '')
		RETURNING id::text`, clinic, n, subject, names, sex, birth, created, last, species).Scan(&id)
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) rptEncounter(clinic, patient, author, authorName, kind, at, reason string, private bool) {
	e.t.Helper()
	e.exec(`INSERT INTO encounters (clinic_id, patient_id, kind, occurred_at, reason, private, author_id, author_name)
		VALUES ($1, $2, $3, $4::timestamptz, $5, $6, $7, $8)`, clinic, patient, kind, at, reason, private, author, authorName)
}

func (c *client) rptRaw(path string) (int, string, string) {
	c.e.t.Helper()
	res, err := c.c.Get(c.e.srv.URL + path)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header.Get("Content-Type"), string(b)
}

// rptSeed fills clinic A with a known January 2025 and gives clinic B one stray patient.
func rptSeed(e *env) (doc, admin string) {
	doc, admin = e.userID("doc_a"), e.userID("admin_a")
	a := e.clinicA

	p1 := e.rptPatient(a, 1, "person", "Ana", "Mujer", time.Now().AddDate(-40, 0, 0).Format("2006-01-02"), "2024-12-01", "2025-01-10 12:00+00", "")
	p2 := e.rptPatient(a, 2, "person", "Beto", "Hombre", time.Now().AddDate(-14, 0, 0).Format("2006-01-02"), "2025-01-12", "2025-01-25 12:00+00", "")
	p3 := e.rptPatient(a, 3, "animal", "Firulais", "Macho", "", "2025-01-15", "2025-01-15 12:00+00", "Perro")
	e.rptPatient(a, 4, "person", "Reciente", "Otro", "", "2025-01-20", "", "")
	e.exec(`UPDATE patients SET last_encounter_at = now() WHERE clinic_id = $1 AND file_number = 4`, a)
	e.rptPatient(e.clinicB, 1, "person", "Ajeno", "Mujer", "", "2025-01-02", "2025-01-02 12:00+00", "")

	e.rptEncounter(a, p1, doc, "DOC_A", "consulta", "2024-12-10 12:00+00", "Dolor de cabeza", false)
	e.rptEncounter(a, p1, doc, "DOC_A", "consulta", "2025-01-10 12:00+00", "Dolor de Cabeza", false)
	e.rptEncounter(a, p2, doc, "DOC_A", "consulta", "2025-01-12 12:00+00", "dolor de cabeza ", false)
	e.rptEncounter(a, p2, doc, "DOC_A", "seguimiento", "2025-01-20 12:00+00", "Revisión", false)
	e.rptEncounter(a, p2, doc, "DOC_A", "consulta", "2025-01-25 12:00+00", "secreto", true)
	e.rptEncounter(a, p2, doc, "DOC_A", "nota", "2025-01-26 12:00+00", "no cuenta", false)
	e.rptEncounter(a, p3, admin, "ADMIN_A", "consulta", "2025-01-15 12:00+00", "REVISION", false)

	appt := func(pro, status, source, hour string) {
		e.exec(`INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, professional_id, status, source)
			VALUES ($1, '', 'X', 'Y', '2025-01-06', $2::time, ($2::time + interval '1 hour'), $3, $4, $5)`, a, hour, pro, status, source)
	}
	appt(doc, "completed", "staff", "10:00")
	appt(doc, "completed", "online", "11:00")
	appt(doc, "no_show", "staff", "12:00")
	appt(doc, "cancelled", "portal", "13:00")
	e.exec(`UPDATE appointments SET arrived_at = '2025-01-06 10:00+00', started_at = '2025-01-06 10:05+00', finished_at = '2025-01-06 10:35+00'
		WHERE clinic_id = $1 AND start_hour = '10:00'`, a)
	e.exec(`UPDATE appointments SET arrived_at = '2025-01-06 11:00+00', started_at = '2025-01-06 11:15+00', finished_at = '2025-01-06 12:05+00'
		WHERE clinic_id = $1 AND start_hour = '11:00'`, a)
	// someone else's appointment, not seen by the doctor
	appt(admin, "scheduled", "staff", "15:00")
	e.exec(`INSERT INTO time_blocks (clinic_id, professional_id, date_from, date_to, reason) VALUES ($1, $2, '2025-01-07', '2025-01-07', 'Vacaciones')`, a, doc)

	e.exec(`INSERT INTO vaccinations (clinic_id, patient_id, kind, name, applied_on, next_due) VALUES
		($1, $2, 'vaccine', 'Rabia', '2024-01-01', '2025-01-01'),
		($1, $2, 'vaccine', 'Rabia', '2025-01-01', '2036-01-01'),
		($1, $2, 'vaccine', 'Parvovirus', '2024-01-01', '2025-01-01')`, a, p3)
	e.exec(`INSERT INTO vaccinations (clinic_id, patient_id, kind, name, applied_on, next_due, voided_at) VALUES ($1, $2, 'vaccine', 'Anulada', '2024-01-01', '2025-01-01', now())`, a, p3)
	return doc, admin
}

const rptJan = "?from=2025-01-01&to=2025-01-31"

func TestReportsOperations(t *testing.T) {
	e := setup(t)
	rptSeed(e)
	admin := e.login("admin_a")

	out := admin.expect(200, "GET", "/api/reports/operations"+rptJan, nil)
	rep := sub(out, "report")
	if rep["scope"] != "clinic" || rep["granularity"] != "day" {
		t.Fatalf("scope/granularity: %v %v", rep["scope"], rep["granularity"])
	}
	if got := rptNum(rep, "encounters", "total"); got != 5 {
		t.Fatalf("encounters: %v (notes and other months must not count)", got)
	}
	if got := rptNum(rep, "appointments", "total"); got != 5 {
		t.Fatalf("appointments: %v", got)
	}
	if got := rptNum(rep, "appointments", "cancellation_rate"); got != 20 {
		t.Fatalf("cancellation rate: %v", got)
	}
	// 4 past appointments not cancelled: 2 completed, 1 no-show, 1 scheduled -> 25%
	if rptNum(rep, "no_show", "eligible") != 4 || rptNum(rep, "no_show", "count") != 1 || rptNum(rep, "no_show", "rate") != 25 {
		t.Fatalf("no show: %v", rep["no_show"])
	}
	if rptNum(rep, "times", "avg_attention_min") != 40 || rptNum(rep, "times", "avg_wait_min") != 10 {
		t.Fatalf("times: %v", rep["times"])
	}
	// 120 booked minutes (completed ones); the doctor works 22 weekdays x 540 min (one day blocked)
	// and the receptionist-admin with an appointment counts too (default clinic hours)
	occ := sub(rep, "occupancy")
	if occ["booked_min"].(float64) != 180 || occ["available_min"].(float64) != 22*540+23*540 {
		t.Fatalf("occupancy: %v", occ)
	}
	src := map[string]float64{}
	for _, x := range rep["appointments"].(map[string]any)["by_source"].([]any) {
		m := x.(map[string]any)
		src[m["key"].(string)] = m["count"].(float64)
	}
	if src["staff"] != 3 || src["online"] != 1 || src["portal"] != 1 {
		t.Fatalf("sources: %v", src)
	}
	if len(rep["professionals"].([]any)) != 2 {
		t.Fatalf("professionals: %v", rep["professionals"])
	}
	// Monday 10:00 has one appointment
	if got := rep["peak_hours"].(map[string]any)["heatmap"].([]any)[0].([]any)[10].(float64); got != 1 {
		t.Fatalf("heatmap: %v", got)
	}

	// filter by professional
	only := sub(admin.expect(200, "GET", "/api/reports/operations"+rptJan+"&professional="+e.userID("admin_a"), nil), "report")
	if rptNum(only, "appointments", "total") != 1 || rptNum(only, "encounters", "total") != 1 {
		t.Fatalf("professional filter: %v", only)
	}

	code, ctype, body := admin.rptRaw("/api/reports/operations.csv" + rptJan)
	if code != 200 || !strings.HasPrefix(ctype, "text/csv") || !strings.Contains(body, "Reporte operativo") {
		t.Fatalf("csv: %d %s", code, ctype)
	}
}

func TestReportsPermissionsAndIsolation(t *testing.T) {
	e := setup(t)
	doc, _ := rptSeed(e)

	for _, c := range []struct {
		user, path string
		want       int
	}{
		{"admin_a", "/api/reports/operations", 200}, {"doc_a", "/api/reports/operations", 200},
		{"recep_a", "/api/reports/operations", 403}, {"cash_a", "/api/reports/operations", 403},
		{"admin_a", "/api/reports/patients", 200}, {"doc_a", "/api/reports/patients", 403},
		{"recep_a", "/api/reports/patients", 403}, {"cash_a", "/api/reports/patients.csv", 403},
		{"doc_a", "/api/reports/patients/inactive.csv", 403}, {"admin_a", "/api/reports/patients/inactive.csv", 200},
		{"doc_a", "/api/reports/operations.csv", 200}, {"recep_a", "/api/reports/operations.csv", 403},
	} {
		if got := status(e.login(c.user), "GET", c.path); got != c.want {
			t.Errorf("%s %s: got %d want %d", c.user, c.path, got, c.want)
		}
	}

	// the doctor only gets their own numbers, even when asking for someone else's
	d := e.login("doc_a")
	for _, q := range []string{"", "&professional=" + e.userID("admin_a")} {
		rep := sub(d.expect(200, "GET", "/api/reports/operations"+rptJan+q, nil), "report")
		if rep["scope"] != "own" || rptNum(rep, "encounters", "total") != 4 || rptNum(rep, "appointments", "total") != 4 {
			t.Fatalf("doctor scope%q: %v", q, rep)
		}
		pros := rep["professionals"].([]any)
		if len(pros) != 1 || pros[0].(map[string]any)["id"] != doc {
			t.Fatalf("doctor sees other professionals: %v", pros)
		}
	}

	// clinic B sees nothing of clinic A
	b := e.login("admin_b")
	rep := sub(b.expect(200, "GET", "/api/reports/operations"+rptJan, nil), "report")
	if rptNum(rep, "encounters", "total") != 0 || rptNum(rep, "appointments", "total") != 0 || rptNum(rep, "occupancy", "booked_min") != 0 {
		t.Fatalf("clinic B leak: %v", rep)
	}
	pr := sub(b.expect(200, "GET", "/api/reports/patients"+rptJan, nil), "report")
	if rptNum(pr, "inactive", "total") != 1 || rptNum(pr, "visits", "patients") != 0 || rptNum(pr, "vaccines", "total") != 0 || rptNum(pr, "people", "total") != 1 {
		t.Fatalf("clinic B patients: %v", pr)
	}
	// a clinic B admin cannot target a clinic A professional
	other := sub(b.expect(200, "GET", "/api/reports/operations"+rptJan+"&professional="+doc, nil), "report")
	if rptNum(other, "encounters", "total") != 0 {
		t.Fatalf("clinic B read A's professional: %v", other)
	}
}

func TestReportsPatients(t *testing.T) {
	e := setup(t)
	rptSeed(e)
	a := e.login("admin_a")
	rep := sub(a.expect(200, "GET", "/api/reports/patients"+rptJan, nil), "report")

	// Ana had a consultation in December: returning. Beto and Firulais are new.
	if rptNum(rep, "visits", "new_total") != 2 || rptNum(rep, "visits", "returning_total") != 1 ||
		rptNum(rep, "visits", "patients") != 3 || rptNum(rep, "visits", "repeat") != 1 {
		t.Fatalf("visits: %v", rep["visits"])
	}
	if rptNum(rep, "signups", "person") != 2 || rptNum(rep, "signups", "animal") != 1 {
		t.Fatalf("signups: %v", rep["signups"])
	}
	reasons := rep["reasons"].([]any)
	if len(reasons) != 2 || reasons[0].(map[string]any)["label"] != "dolor de cabeza" || reasons[0].(map[string]any)["count"].(float64) != 2 ||
		reasons[1].(map[string]any)["label"] != "revision" || reasons[1].(map[string]any)["count"].(float64) != 2 {
		t.Fatalf("reasons (accents merged, private and notes excluded): %v", reasons)
	}
	// Reciente has a visit today; the other three are inactive
	if rptNum(rep, "inactive", "total") != 3 {
		t.Fatalf("inactive: %v", rep["inactive"])
	}
	first := rep["inactive"].(map[string]any)["items"].([]any)[0].(map[string]any)
	if first["name"] != "Ana Prueba" {
		t.Fatalf("oldest lapsed first: %v", first)
	}
	paged := sub(a.expect(200, "GET", "/api/reports/patients"+rptJan+"&inactive_days=180&limit=1&page=2", nil), "report")
	if len(paged["inactive"].(map[string]any)["items"].([]any)) != 1 || rptNum(paged, "inactive", "total") != 3 {
		t.Fatalf("pagination: %v", paged["inactive"])
	}
	// the dog's owner is the contact
	if items := paged["inactive"].(map[string]any)["items"].([]any); items[0].(map[string]any)["contact_name"] != "Tutor Firulais" {
		t.Fatalf("guardian contact: %v", items[0])
	}

	// only the latest application of each vaccine counts, voided ones never
	vac := rep["vaccines"].(map[string]any)
	if vac["available"] != true || vac["total"].(float64) != 1 || vac["items"].([]any)[0].(map[string]any)["vaccine"] != "Parvovirus" ||
		vac["items"].([]any)[0].(map[string]any)["overdue"] != true {
		t.Fatalf("vaccines: %v", vac)
	}
	if rptNum(rep, "animals", "total") != 1 || rep["animals"].(map[string]any)["by_species"].([]any)[0].(map[string]any)["label"] != "Perro" {
		t.Fatalf("animals: %v", rep["animals"])
	}
	if rptNum(rep, "people", "total") != 3 {
		t.Fatalf("people: %v", rep["people"])
	}
	bands := map[string]float64{}
	for _, x := range rep["people"].(map[string]any)["by_age"].([]any) {
		m := x.(map[string]any)
		bands[m["key"].(string)] = m["total"].(float64)
	}
	if bands["12-17"] != 1 || bands["30-44"] != 1 || bands["unknown"] != 1 {
		t.Fatalf("age bands: %v", bands)
	}

	code, ctype, body := a.rptRaw("/api/reports/patients/inactive.csv?inactive_days=180")
	if code != 200 || !strings.HasPrefix(ctype, "text/csv") || !strings.Contains(body, "Tutor Firulais") {
		t.Fatalf("inactive csv: %d %s", code, ctype)
	}
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM activity_log WHERE clinic_id = $1 AND type = 'report_export'`, e.clinicA).Scan(&n); err != nil || n != 1 {
		t.Fatalf("export must be audited: %d %v", n, err)
	}
	if code, _, body := a.rptRaw("/api/reports/patients.csv" + rptJan); code != 200 || !strings.Contains(body, "Reporte de pacientes") {
		t.Fatalf("patients csv: %d", code)
	}
}

func TestReportsInvalidRanges(t *testing.T) {
	e := setup(t)
	a := e.login("admin_a")
	for _, q := range []string{
		"?from=2025-02-01&to=2025-01-01", "?from=nope", "?to=2025-13-45", "?from=2020-01-01&to=2025-01-01",
		"?professional=not-a-uuid",
	} {
		for _, p := range []string{"/api/reports/operations", "/api/reports/operations.csv"} {
			if got := status(a, "GET", p+q); got != http.StatusBadRequest {
				t.Errorf("%s%s: got %d want 400", p, q, got)
			}
		}
	}
	for _, q := range []string{"?from=2020-01-01&to=2025-01-01", "?inactive_days=5", "?inactive_days=x", "?limit=1000"} {
		if got := status(a, "GET", "/api/reports/patients"+q); got != http.StatusBadRequest {
			t.Errorf("patients%s: got %d want 400", q, got)
		}
	}
	// exactly two years is fine, and the default range works
	a.expect(200, "GET", "/api/reports/operations?from=2024-01-01&to=2025-12-31", nil)
	a.expect(200, "GET", "/api/reports/operations", nil)
	a.expect(200, "GET", "/api/reports/patients", nil)
}
