package api

import (
	"math"
	"testing"
)

func TestDoseVolume(t *testing.T) {
	// 12 kg x 15 mg/kg = 180 mg; amoxicillin 250 mg/5 mL = 50 mg/mL -> 3.6 mL
	if got := doseVolume(12, 15, 50); math.Abs(got-3.6) > 1e-9 {
		t.Fatalf("doseVolume = %v", got)
	}
	for _, c := range [][3]float64{{0, 15, 50}, {12, 0, 50}, {12, 15, 0}, {-1, 15, 50}} {
		if doseVolume(c[0], c[1], c[2]) != 0 {
			t.Fatalf("invalid input must give 0: %v", c)
		}
	}
}

func TestNormText(t *testing.T) {
	if got := normText("  Ácido   ACETILSALICÍLICO Ñandú "); got != "acido acetilsalicilico nandu" {
		t.Fatalf("normText = %q", got)
	}
}

func TestAllergyFamilies(t *testing.T) {
	item := func(m string) []rxItem { return []rxItem{{Medicine: m}} }
	cases := []struct {
		allergy, medicine string
		want              bool
	}{
		{"Penicilina", "Amoxicilina", true},
		{"PENICILINAS", "Ampicilina", true},
		{"amoxicilina", "Amoxicilina / ácido clavulánico", true},
		{"Alergia a AINEs", "Ibuprofeno", true},
		{"aspirina", "Naproxeno", true},
		{"Sulfas", "Trimetoprima / sulfametoxazol", true},
		{"Sulfas", "Sulfato ferroso", false},
		{"Lidocaína", "Paracetamol", false},
		{"Betalactámicos", "Cefalexina", true},
		{"Látex", "Amoxicilina", false},
		{"Mariscos", "Paracetamol", false},
		{"Quinolonas", "Ciprofloxacino", true},
		{"anestesicos locales", "Lidocaína con epinefrina", true},
		{"Penicilina", "Paracetamol", false},
	}
	for _, c := range cases {
		got := len(allergyConflicts([]string{c.allergy}, item(c.medicine))) > 0
		if got != c.want {
			t.Errorf("allergy %q vs %q: got %v want %v", c.allergy, c.medicine, got, c.want)
		}
	}
}

func TestPatientAllergiesParsing(t *testing.T) {
	got := patientAllergies(map[string]any{"allergies_text": "Penicilina, Sulfas y látex"})
	if len(got) != 3 {
		t.Fatalf("parsed: %v", got)
	}
	for _, none := range []string{"Ninguna conocida", "Niega", "No", ""} {
		if g := patientAllergies(map[string]any{"allergies_text": none}); len(g) != 0 {
			t.Fatalf("%q must mean no allergies: %v", none, g)
		}
	}
	if g := patientAllergies(map[string]any{"dental_anesthesia_allergy": "Sí"}); len(g) != 1 {
		t.Fatalf("dental anesthesia allergy: %v", g)
	}
	s := patientAllergies(map[string]any{"allergies": []any{"Ibuprofeno", map[string]any{"substance": "Penicilina"}}})
	if len(s) != 2 {
		t.Fatalf("structured: %v", s)
	}
}

func TestDoseExceeds(t *testing.T) {
	m, ok := catalogByID("h-paracetamol")
	if !ok || m.MaxMgPerKgDay != 75 {
		t.Fatalf("catalog paracetamol: %v %v", m, ok)
	}
	if _, over := doseExceeds(m, 10, 500, 1); over {
		t.Fatal("500 mg/day for 10 kg is fine")
	}
	if _, over := doseExceeds(m, 10, 750, 1); over {
		t.Fatal("exactly the limit is fine")
	}
	if _, over := doseExceeds(m, 10, 500, 4); !over {
		t.Fatal("2000 mg/day for 10 kg exceeds 75 mg/kg/day")
	}
	if _, over := doseExceeds(m, 0, 500, 4); over {
		t.Fatal("without weight there is nothing to compare")
	}
}

func TestCatalogIntegrity(t *testing.T) {
	ids := map[string]bool{}
	for _, m := range catalogMeds {
		if m.ID == "" || ids[m.ID] || m.Name == "" || len(m.Presentations) == 0 {
			t.Fatalf("bad entry: %+v", m)
		}
		ids[m.ID] = true
		if !contains(append([]string{}, rxRoutes...), m.Route) {
			t.Errorf("%s: unknown route %q", m.ID, m.Route)
		}
		if !contains(rxControl, m.Control) {
			t.Errorf("%s: unknown control %q", m.ID, m.Control)
		}
		if m.MaxMgPerKgDay > 0 && m.MgPerKg > m.MaxMgPerKgDay {
			t.Errorf("%s: single dose above the daily maximum", m.ID)
		}
	}
	if len(catalogMeds) < 150 {
		t.Fatalf("catalog too small: %d", len(catalogMeds))
	}
	seen := map[string]bool{}
	for _, d := range icd10 {
		if seen[d.Code] || d.Name == "" {
			t.Fatalf("bad ICD entry: %+v", d)
		}
		seen[d.Code] = true
	}
}

