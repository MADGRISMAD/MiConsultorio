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
