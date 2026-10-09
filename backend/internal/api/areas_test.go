package api_test

import "testing"

func TestProfessionalAreas(t *testing.T) {
	b := newBookingEnv(t, false)
	admin, anon := b.login("admin_a"), b.anon()
	b.exec(`UPDATE clinics SET kind = 'GENERAL_MEDICAL', specialties = '{NUTRITION,PSYCHOLOGY}' WHERE id = $1`, b.clinicA)
	var docID string
	for _, p := range admin.expect(200, "GET", "/api/team", nil)["people"].([]any) {
		if m := p.(map[string]any); m["username"] == "doc_a" {
			docID = m["id"].(string)
		}
	}
	admin.expect(400, "PATCH", "/api/team/"+docID, map[string]any{"areas": []string{"VETERINARY"}}) // not a giro of the clinic
	out := sub(admin.expect(200, "PATCH", "/api/team/"+docID, map[string]any{"areas": []string{"NUTRITION"}}), "person")
	if a := out["areas"].([]any); len(a) != 1 || a[0] != "NUTRITION" {
		t.Fatalf("areas: %v", out["areas"])
	}

	// the public page asks for the area, and the professional is offered only in theirs
	info := sub(anon.expect(200, "GET", "/api/public/booking/"+b.slugA, nil), "booking")
	areas := info["areas"].([]any)
	if len(areas) != 1 || areas[0].(map[string]any)["id"] != "NUTRITION" {
		t.Fatalf("offered areas: %v", areas)
	}
	if pa := info["professionals"].([]any)[0].(map[string]any)["areas"].([]any); len(pa) != 1 || pa[0] != "NUTRITION" {
		t.Fatalf("professional areas: %v", pa)
	}

	// a nutritionist does not read nor write psychotherapy records, but works with nutrition ones
	doc := b.login("doc_a")
	pid := sub(doc.expect(201, "POST", "/api/patients/", person(nil)), "patient")["id"].(string)
	url := "/api/patients/" + pid + "/charts"
	doc.expect(403, "GET", url+"?kind=scale", nil)
	doc.expect(403, "POST", url, map[string]any{"kind": "therapy_plan", "data": map[string]any{"goals": []any{}}})
	doc.expect(200, "GET", url, nil)
	// an administrator is not limited
	admin.expect(200, "GET", url+"?kind=scale", nil)
}

// Each giro's forms ask only what its giro asks, and saving them keeps the answers of the other giros.
func TestFormsOnlyOfMyArea(t *testing.T) {
	e := setup(t)
	e.exec(`UPDATE clinics SET kind = 'GENERAL_MEDICAL', specialties = '{PEDIATRICS,GYNECOLOGY}' WHERE id = $1`, e.clinicA)
	admin, doc := e.login("admin_a"), e.login("doc_a")
	has := func(c *client, key string) bool {
		for _, f := range sub(c.expect(200, "GET", "/api/patients/schema", nil), "profile")["person"].([]any) {
			if f.(map[string]any)["key"] == key {
				return true
			}
		}
		return false
	}
	if !has(doc, "birth_weight") || !has(doc, "gestas") {
		t.Fatal("without areas the person fills every giro's form")
	}
	var docID string
	for _, p := range admin.expect(200, "GET", "/api/team", nil)["people"].([]any) {
		if m := p.(map[string]any); m["username"] == "doc_a" {
			docID = m["id"].(string)
		}
	}
	admin.expect(200, "PATCH", "/api/team/"+docID, map[string]any{"areas": []string{"PEDIATRICS"}})
	if !has(doc, "birth_weight") || has(doc, "gestas") {
		t.Fatal("a pediatrician fills pediatric fields only")
	}
	// the gynecology answer someone else saved is kept when the pediatrician saves the form
	pid := sub(admin.expect(201, "POST", "/api/patients/", person(map[string]any{"profile": map[string]any{"allergies_text": "Ninguna", "gestas": 2}})), "patient")["id"].(string)
	got := sub(doc.expect(200, "GET", "/api/patients/"+pid, nil), "patient")
	if _, leaked := got["profile"].(map[string]any)["gestas"]; leaked {
		t.Fatal("another giro's answer must not be shown")
	}
	body := person(map[string]any{"profile": map[string]any{"allergies_text": "Ninguna", "birth_weight": 3.2}})
	doc.expect(200, "PUT", "/api/patients/"+pid, body)
	whole := sub(admin.expect(200, "GET", "/api/patients/"+pid, nil), "patient")["profile"].(map[string]any)
	if whole["gestas"] != float64(2) || whole["birth_weight"] != 3.2 {
		t.Fatalf("profile after save: %v", whole)
	}
}
