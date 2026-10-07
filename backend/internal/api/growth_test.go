package api_test

import (
	"context"
	"testing"
)

// Every number below is synthetic and only exercises the maths: they are NOT WHO or CDC values.
const syntheticLMS = `indicator,sex,age_months,l,m,s
weight_for_age,M,0,1,3,0.1
weight_for_age,M,12,1,9,0.1
weight_for_age,M,24,1,12,0.1
`

const syntheticPct = `indicator,sex,age_months,p3,p50,p97
length_height_for_age,F,0,45,50,55
length_height_for_age,F,12,70,75,80
`

func growthImport(c *client, want int, body map[string]any) map[string]any {
	return c.expect(want, "POST", "/api/growth/references/import", body)
}

func TestGrowthImportValidation(t *testing.T) {
	e := setup(t)
	admin, doc, other := e.login("admin_a"), e.login("doc_a"), e.login("admin_b")
	ctx := context.Background()
	count := func() int {
		var n int
		_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM growth_references`).Scan(&n)
		return n
	}
	body := func(csv string, confirm bool) map[string]any {
		return map[string]any{"standard": "SINTETICO", "source_name": "Prueba sintética", "file_name": "t.csv", "csv": csv, "confirm": confirm}
	}

	doc.expect(403, "POST", "/api/growth/references/import", body(syntheticLMS, false))
	doc.expect(403, "GET", "/api/growth/references", nil)
	e.anon().expect(401, "POST", "/api/growth/references/import", body(syntheticLMS, false))
	growthImport(admin, 400, map[string]any{"standard": "!", "source_name": "x", "csv": syntheticLMS})
	growthImport(admin, 400, map[string]any{"standard": "OMS", "source_name": "", "csv": syntheticLMS})
	growthImport(admin, 400, map[string]any{"standard": "OMS", "source_name": "x", "csv": "  "})
	admin.expect(400, "POST", "/api/growth/references/import", map[string]any{"standard": "OMS", "source_name": "x", "csv": "a", "extra": 1})

	bad := map[string]string{
		"unknown column":       "indicator,sex,age_months,l,m,s,peso\nweight_for_age,M,0,1,3,0.1,5\n",
		"missing required":     "indicator,age_months,l,m,s\nweight_for_age,0,1,3,0.1\n",
		"partial lms":          "indicator,sex,age_months,l,m\nweight_for_age,M,0,1,3\n",
		"no values columns":    "indicator,sex,age_months\nweight_for_age,M,0\n",
		"bad indicator":        "indicator,sex,age_months,l,m,s\npeso,M,0,1,3,0.1\n",
		"bad sex":              "indicator,sex,age_months,l,m,s\nweight_for_age,X,0,1,3,0.1\n",
		"negative age":         "indicator,sex,age_months,l,m,s\nweight_for_age,M,-1,1,3,0.1\n",
		"age too large":        "indicator,sex,age_months,l,m,s\nweight_for_age,M,500,1,3,0.1\n",
		"zero M":               "indicator,sex,age_months,l,m,s\nweight_for_age,M,0,1,0,0.1\n",
		"negative S":           "indicator,sex,age_months,l,m,s\nweight_for_age,M,0,1,3,-0.1\n",
		"decimal comma":        "indicator,sex,age_months,l,m,s\nweight_for_age,M,0,1,\"3,5\",0.1\n",
		"NaN":                  "indicator,sex,age_months,l,m,s\nweight_for_age,M,0,NaN,3,0.1\n",
		"duplicate row":        "indicator,sex,age_months,l,m,s\nweight_for_age,M,0,1,3,0.1\nweight_for_age,M,0,1,3,0.1\n",
		"short row":            "indicator,sex,age_months,l,m,s\nweight_for_age,M,0,1,3\n",
		"descending pct":       "indicator,sex,age_months,p3,p50,p97\nweight_for_age,M,0,5,4,6\n",
		"duplicate column":     "indicator,sex,age_months,l,m,s,l\nweight_for_age,M,0,1,3,0.1,1\n",
		"empty row of values":  "indicator,sex,age_months,l,m,s\nweight_for_age,M,0,,,\n",
		"header only":          "indicator,sex,age_months,l,m,s\n",
		"pct column not known": "indicator,sex,age_months,p4\nweight_for_age,M,0,5\n",
	}
	for name, csv := range bad {
		pv := sub(growthImport(admin, 200, body(csv, false)), "preview")
		if pv["valid"] != false || pv["error_count"].(float64) < 1 || len(pv["errors"].([]any)) == 0 {
			t.Fatalf("%s: accepted: %v", name, pv)
		}
		growthImport(admin, 400, body(csv, true))
	}
	if count() != 0 {
		t.Fatal("an invalid file left rows behind")
	}

	// Preview does not store anything.
	pv := sub(growthImport(admin, 200, body(syntheticLMS, false)), "preview")
	if pv["valid"] != true || pv["rows"].(float64) != 3 || pv["next_version"].(float64) != 1 || pv["has_lms"] != true || len(pv["groups"].([]any)) != 1 {
		t.Fatalf("preview: %v", pv)
	}
	if count() != 0 {
		t.Fatal("the preview stored rows")
	}

	// Confirm appends version 1, then 2: nothing is overwritten.
	r1 := sub(growthImport(admin, 201, body(syntheticLMS, true)), "import")
	r2 := sub(growthImport(admin, 201, body("\xef\xbb\xbf"+syntheticLMS, true)), "import")
	if r1["version"].(float64) != 1 || r2["version"].(float64) != 2 || r1["source_name"] != "Prueba sintética" || count() != 6 {
		t.Fatalf("imports: %v %v count %d", r1, r2, count())
	}
	list := admin.expect(200, "GET", "/api/growth/references", nil)["imports"].([]any)
	if len(list) != 2 {
		t.Fatalf("list: %v", list)
	}
	if n := len(other.expect(200, "GET", "/api/growth/references", nil)["imports"].([]any)); n != 0 {
		t.Fatalf("another clinic sees %d imports", n)
	}
	var audited int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM activity_log WHERE clinic_id = $1 AND type = 'growth_import' AND meta->>'source' = 'Prueba sintética'`, e.clinicA).Scan(&audited)
	if audited != 2 {
		t.Fatalf("audited imports: %d", audited)
	}
	if _, err := e.pool.Exec(ctx, `UPDATE growth_references SET m = 99`); err == nil {
		t.Fatal("reference rows were edited")
	}
	if _, err := e.pool.Exec(ctx, `UPDATE growth_imports SET source_name = 'x'`); err == nil {
		t.Fatal("an import record was edited")
	}
}

func TestPatientGrowth(t *testing.T) {
	e := setup(t)
	admin, doc, recep, other := e.login("admin_a"), e.login("doc_a"), e.login("recep_a"), e.login("doc_b")
	boy := sub(doc.expect(201, "POST", "/api/patients/", person(map[string]any{"names": "Beto", "guardian_name": "Madre", "guardian_phone": "5512345678", "birth_date": "2025-01-01", "sex": "Hombre", "curp": "mejj700312hdfdrr04"})), "patient")["id"].(string)
	girl := sub(doc.expect(201, "POST", "/api/patients/", person(map[string]any{"names": "Bea", "guardian_name": "Madre", "guardian_phone": "5512345678", "birth_date": "2025-01-01", "sex": "Mujer", "curp": "mejj700312mdfdrr05"})), "patient")["id"].(string)
	ctx := context.Background()
	doctor := e.userID("doc_a")
	enc := func(patient, at, measures string) {
		e.exec(`INSERT INTO encounters (clinic_id, patient_id, occurred_at, reason, measures, author_id, author_name) VALUES ($1,$2,$3::timestamptz,'Control',$4::jsonb,$5,'Doc')`,
			e.clinicA, patient, at, measures, doctor)
	}
	enc(boy, "2025-01-01T12:00:00Z", `{"weight_kg": 3.3, "height_cm": 50, "head_circumference": 34}`)
	enc(boy, "2027-07-01T12:00:00Z", `{"weight_kg": "13,5"}`)
	enc(girl, "2025-01-01T12:00:00Z", `{"height_cm": 50}`)
	e.exec(`INSERT INTO encounters (clinic_id, patient_id, occurred_at, reason, measures, author_id, author_name) VALUES ($1,$2,'2025-02-01T12:00:00Z','Sin medidas','{"temp_c": 36}', $3, 'Doc')`, e.clinicA, boy, doctor)

	// Without references: only the patient's data, with a clear status.
	g := doc.expect(200, "GET", "/api/patients/"+boy+"/growth", nil)
	ms := g["measurements"].([]any)
	if g["reference_status"] != "no_references" || len(ms) != 2 || g["standard"] != "" {
		t.Fatalf("no references: %v", g)
	}
	first := ms[0].(map[string]any)
	if first["age_months"].(float64) != 0 || first["bmi"].(float64) != 13.2 || first["head_cm"].(float64) != 34 {
		t.Fatalf("first measurement: %v", first)
	}
	if ms[1].(map[string]any)["weight_kg"].(float64) != 13.5 || ms[1].(map[string]any)["age_months"].(float64) < 29 {
		t.Fatalf("second measurement: %v", ms[1])
	}
	recep.expect(403, "GET", "/api/patients/"+boy+"/growth", nil)
	other.expect(404, "GET", "/api/patients/"+boy+"/growth", nil)

	growthImport(admin, 201, map[string]any{"standard": "SINTETICO", "source_name": "Prueba", "csv": syntheticLMS, "confirm": true})
	growthImport(admin, 201, map[string]any{"standard": "OTRO", "source_name": "Prueba 2", "csv": syntheticPct, "confirm": true})

	// LMS: weight 3.3 at age 0 with L=1, M=3, S=0.1 is exactly Z = 1.
	g = doc.expect(200, "GET", "/api/patients/"+boy+"/growth?standard=SINTETICO", nil)
	w := sub(sub(g, "indicators"), "weight_for_age")
	pts := w["points"].([]any)
	p0 := pts[0].(map[string]any)
	if g["reference_status"] != "ok" || w["has_reference"] != true || p0["z"].(float64) != 1 || p0["percentile"].(float64) != 84.1 {
		t.Fatalf("z score: %v", g)
	}
	if p1 := pts[1].(map[string]any); p1["z"] != nil || p1["percentile"] != nil {
		t.Fatalf("age outside the table must not be scored: %v", p1)
	}
	curves := w["curves"].(map[string]any)
	p50 := curves["p50"].([]any)
	if len(p50) != 3 || p50[0].(map[string]any)["value"].(float64) != 3 || p50[1].(map[string]any)["value"].(float64) != 9 {
		t.Fatalf("p50 curve: %v", p50)
	}
	p97 := curves["p97"].([]any)[0].(map[string]any)["value"].(float64)
	if p97 < 3.5 || p97 > 3.7 { // 3 * (1 + 0.1*1.8808) = 3.564
		t.Fatalf("p97 at birth: %v", p97)
	}
	if h := sub(sub(g, "indicators"), "length_height_for_age"); h["has_reference"] != false {
		t.Fatalf("an indicator without table has a reference: %v", h)
	}

	// The standard with percentile columns is scored by interpolating between percentiles; girls only see girls' rows.
	g = doc.expect(200, "GET", "/api/patients/"+girl+"/growth", nil)
	if g["standard"] != "OTRO" || len(g["standards"].([]any)) != 1 {
		t.Fatalf("girl standards: %v", g["standards"])
	}
	h := sub(sub(g, "indicators"), "length_height_for_age")["points"].([]any)[0].(map[string]any)
	if h["percentile"].(float64) != 50 || h["z"].(float64) != 0 {
		t.Fatalf("percentile table: %v", h)
	}
	if g := doc.expect(200, "GET", "/api/patients/"+boy+"/growth?standard=OTRO", nil); g["standard"] != "SINTETICO" {
		t.Fatalf("a standard without rows for the sex must not be offered: %v", g["standard"])
	}

	// Another clinic never sees these references.
	obaby := sub(other.expect(201, "POST", "/api/patients/", person(map[string]any{"names": "Otro", "guardian_name": "Madre", "guardian_phone": "5512345678", "birth_date": "2025-01-01", "curp": "mejj700312hdfdrr04"})), "patient")["id"].(string)
	if g := other.expect(200, "GET", "/api/patients/"+obaby+"/growth", nil); g["reference_status"] != "no_references" || len(g["standards"].([]any)) != 0 {
		t.Fatalf("isolation: %v", g)
	}

	// Private consultations of somebody else do not leak measurements.
	e.exec(`INSERT INTO encounters (clinic_id, patient_id, occurred_at, reason, measures, private, author_id, author_name) VALUES ($1,$2,'2025-03-01T12:00:00Z','Privada','{"weight_kg": 5}', true, $3, 'Admin')`,
		e.clinicA, boy, e.userID("admin_a"))
	if n := len(doc.expect(200, "GET", "/api/patients/"+boy+"/growth", nil)["measurements"].([]any)); n != 2 {
		t.Fatalf("private measurement leaked: %d", n)
	}
	var seen int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM record_access WHERE patient_id = $1 AND action = 'view'`, boy).Scan(&seen)
	if seen == 0 {
		t.Fatal("growth reads are not in record_access")
	}
}
