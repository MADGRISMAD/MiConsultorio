package api

import "testing"

func keysOf(fs []Field) map[string]bool {
	m := map[string]bool{}
	for _, f := range fs {
		m[f.Key] = true
	}
	return m
}

func TestMeasuresFollowTheGiro(t *testing.T) {
	dental := keysOf(measureFields("person", []string{"DENTAL"}))
	if dental["weight_kg"] || dental["height_cm"] || dental["temp_c"] || !dental["bp_sys"] || !dental["teeth"] {
		t.Fatalf("dental measures: %v", dental)
	}
	if got := measureFields("person", []string{"PSYCHOLOGY"}); len(keysOf(got)) != 3 || keysOf(got)["weight_kg"] {
		t.Fatalf("psychology measures: %v", keysOf(got))
	}
	// a clinic with a general giro keeps every vital sign, and several giros add up
	if g := keysOf(measureFields("person", []string{"DENTAL", "GENERAL_MEDICAL"})); !g["weight_kg"] || !g["spo2"] || !g["teeth"] {
		t.Fatalf("mixed measures: %v", g)
	}
	if all := keysOf(allMeasureFields("person", []string{"DENTAL"})); !all["weight_kg"] {
		t.Fatalf("display set must keep every measure: %v", all)
	}
}

func TestProfileFollowsTheGiro(t *testing.T) {
	d := keysOf(profileFields("person", []string{"DENTAL"}))
	if d["physical_activity"] || d["hereditary"] || !d["allergies_text"] || !d["dental_anesthesia_allergy"] {
		t.Fatalf("dental profile: %v", d)
	}
	for _, f := range profileFields("person", []string{"PSYCHOLOGY"}) {
		if f.Key == "allergies_text" && f.Required {
			t.Fatal("allergies must be optional for psychology")
		}
	}
	for _, f := range profileFields("person", []string{"DENTAL"}) {
		if f.Key == "allergies_text" && !f.Required {
			t.Fatal("allergies stay required for dental")
		}
	}
}
