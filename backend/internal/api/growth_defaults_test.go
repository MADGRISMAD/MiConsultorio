package api_test

import (
	"context"
	"math"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
)

// The WHO and CDC tables come already loaded: a clinic that imported nothing still gets curves, and its own import wins.
func TestGrowthDefaultsShipWithTheProduct(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, doc, other := e.login("admin_a"), e.login("doc_a"), e.login("admin_b")
	boy := sub(doc.expect(201, "POST", "/api/patients/", person(map[string]any{"names": "Beto", "guardian_name": "Madre", "guardian_phone": "5512345678", "birth_date": "2025-01-01", "sex": "Hombre", "curp": "mejj700312hdfdrr04"})), "patient")["id"].(string)
	e.exec(`INSERT INTO encounters (clinic_id, patient_id, occurred_at, reason, measures, author_id, author_name) VALUES ($1,$2,'2025-01-01T12:00:00Z','Control','{"weight_kg": 3.3464, "height_cm": 49.8842}'::jsonb,$3,'Doc')`, e.clinicA, boy, e.userID("doc_a"))

	g := doc.expect(200, "GET", "/api/patients/"+boy+"/growth", nil)
	if g["reference_status"] != "no_references" {
		t.Fatalf("before the tables are loaded: %v", g["reference_status"])
	}
	for i := 0; i < 2; i++ { // twice: it must not duplicate
		if err := api.SeedGrowthDefaults(ctx, e.pool); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM growth_imports WHERE clinic_id IS NULL`).Scan(&n)
	if n != 2 {
		t.Fatalf("two shipped standards, got %d", n)
	}

	g = doc.expect(200, "GET", "/api/patients/"+boy+"/growth?standard=OMS", nil)
	if g["standard"] != "OMS" || len(g["standards"].([]any)) != 2 {
		t.Fatalf("OMS and CDC are offered: %v", g["standards"])
	}
	// the WHO median of a newborn boy: weight 3.3464 kg and length 49.8842 cm are exactly Z = 0
	for ind, want := range map[string]float64{"weight_for_age": 3.3464, "length_height_for_age": 49.8842} {
		pts := sub(sub(g, "indicators"), ind)["points"].([]any)
		if len(pts) != 1 {
			t.Fatalf("%s points: %v", ind, pts)
		}
		z := pts[0].(map[string]any)["z"].(float64)
		if math.Abs(z) > 0.001 {
			t.Fatalf("%s z = %v for the WHO median %v", ind, z, want)
		}
	}
	if len(sub(sub(g, "indicators"), "weight_for_age")["curves"].(map[string]any)) == 0 {
		t.Fatal("curves from the shipped table")
	}
	// other clinics see them too, and the list marks them as Caresia's
	if len(other.expect(200, "GET", "/api/growth/references", nil)["imports"].([]any)) < 2 {
		t.Fatal("the shipped tables appear in every clinic's list")
	}
	list := admin.expect(200, "GET", "/api/growth/references", nil)["imports"].([]any)
	if list[0].(map[string]any)["platform"] != true {
		t.Fatalf("shipped tables are flagged: %v", list[0])
	}

	// a clinic's own OMS import wins for what it covers (weight), the rest still comes from the shipped table
	growthImport(admin, 201, map[string]any{"standard": "OMS", "source_name": "Propia", "csv": "indicator,sex,age_months,l,m,s\nweight_for_age,M,0,1,3,0.1\nweight_for_age,M,12,1,9,0.1\n", "confirm": true})
	g = doc.expect(200, "GET", "/api/patients/"+boy+"/growth?standard=OMS", nil)
	z := sub(sub(g, "indicators"), "weight_for_age")["points"].([]any)[0].(map[string]any)["z"].(float64)
	if math.Abs(z-1.1547) > 0.01 { // (3.3464/3 - 1) / 0.1
		t.Fatalf("own import wins for weight: z = %v", z)
	}
	if h := sub(sub(g, "indicators"), "length_height_for_age")["points"].([]any)[0].(map[string]any)["z"].(float64); math.Abs(h) > 0.001 {
		t.Fatalf("length falls back to the shipped table: %v", h)
	}
}
