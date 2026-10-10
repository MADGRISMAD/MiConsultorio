package api

import "testing"

func TestInteractionsBetween(t *testing.T) {
	has := func(hits []interactionHit, sev, with string) bool {
		for _, h := range hits {
			if h.Severity == sev && normText(h.With) == normText(with) {
				return true
			}
		}
		return false
	}
	// a new AINE against the warfarin the patient already takes
	hits := interactionsBetween([]string{"Ibuprofeno"}, []drugRef{{"Warfarina", "medicación crónica"}})
	if !has(hits, sevGrave, "Warfarina") || hits[0].WithSource != "medicación crónica" {
		t.Fatalf("aine + warfarin: %+v", hits)
	}
	// two new drugs of the same receta, reported once
	hits = interactionsBetween([]string{"Sildenafil", "Isosorbide"}, nil)
	if len(hits) != 1 || hits[0].Severity != sevGrave {
		t.Fatalf("nitrate + pde5: %+v", hits)
	}
	// two AINEs: duplicity (moderate); the same drug twice is not an interaction
	hits = interactionsBetween([]string{"Ibuprofeno", "Naproxeno"}, nil)
	if len(hits) != 1 || hits[0].Severity != sevModerada {
		t.Fatalf("two aines: %+v", hits)
	}
	if h := interactionsBetween([]string{"Ibuprofeno"}, []drugRef{{"ibuprofeno", "receta vigente #3"}}); len(h) != 0 {
		t.Fatalf("same drug is not an interaction: %+v", h)
	}
	// ketorolac with any other AINE is severe
	hits = interactionsBetween([]string{"Ketorolaco"}, []drugRef{{"Naproxeno", "receta vigente #1"}})
	if len(graveInteractions(hits)) == 0 {
		t.Fatalf("ketorolac: %+v", hits)
	}
	// unrelated drugs and empty names
	if h := interactionsBetween([]string{"Paracetamol", "Loratadina", ""}, []drugRef{{"Amoxicilina", "receta vigente #2"}}); len(h) != 0 {
		t.Fatalf("no interaction expected: %+v", h)
	}
	// severe first
	hits = interactionsBetween([]string{"Ibuprofeno", "Warfarina", "Naproxeno"}, nil)
	if len(hits) < 2 || hits[0].Severity != sevGrave {
		t.Fatalf("order: %+v", hits)
	}
}

func TestInteractionRulesReferToKnownGroups(t *testing.T) {
	for _, r := range interactionRules {
		if _, ok := drugGroups[r.A]; !ok {
			t.Errorf("rule %q+%q: unknown group %q", r.A, r.B, r.A)
		}
		if _, ok := drugGroups[r.B]; !ok {
			t.Errorf("rule %q+%q: unknown group %q", r.A, r.B, r.B)
		}
		if r.Severity != sevGrave && r.Severity != sevModerada {
			t.Errorf("rule %q+%q: bad severity", r.A, r.B)
		}
	}
}
