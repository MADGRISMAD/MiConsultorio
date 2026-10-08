package api_test

import (
	"testing"
)

func TestOwnersGroupPetsAndTellSameNamedPetsApart(t *testing.T) {
	e := setup(t)
	e.exec(`UPDATE clinics SET kind = 'VETERINARY' WHERE id = $1`, e.clinicA)
	vet, recep := e.login("doc_a"), e.login("recep_a")
	pet := func(c *client, name, owner, phone string, extra map[string]any) map[string]any {
		body := map[string]any{"subject": "animal", "names": name, "privacy_ack": true, "guardian_name": owner, "guardian_phone": phone,
			"profile": map[string]any{"species": "Perro", "allergies_text": "Ninguna", "sterilized": "Sí"}}
		for k, v := range extra {
			body[k] = v
		}
		return sub(c.expect(201, "POST", "/api/patients/", body), "patient")
	}

	max1 := pet(vet, "Max", "Luis Pérez", "664 111 2222", nil)
	luna := pet(vet, "Luna", "luis  pérez", "(664) 1112222", nil) // the same person, typed differently
	max2 := pet(vet, "Max", "Marco Díaz", "664 333 4444", nil)
	other := pet(vet, "Toby", "Luis Pérez", "664 999 0000", nil) // another Luis Pérez: another phone

	if max1["owner_id"] == nil || max1["owner_id"] != luna["owner_id"] {
		t.Fatalf("two pets of the same person must share the owner: %v %v", max1["owner_id"], luna["owner_id"])
	}
	if max2["owner_id"] == max1["owner_id"] || other["owner_id"] == max1["owner_id"] || other["owner_id"] == max2["owner_id"] {
		t.Fatalf("different people must not be merged: %v %v %v", max1["owner_id"], max2["owner_id"], other["owner_id"])
	}

	// searching the owner finds all of their pets, grouped
	g := vet.expect(200, "GET", "/api/patients/grouped?q=marco", nil)["groups"].([]any)
	if len(g) != 1 || len(g[0].(map[string]any)["pets"].([]any)) != 1 {
		t.Fatalf("grouped by owner name: %v", g)
	}
	g = vet.expect(200, "GET", "/api/patients/grouped?q=664%20111", nil)["groups"].([]any) // by phone
	if len(g) != 1 || len(g[0].(map[string]any)["pets"].([]any)) != 2 {
		t.Fatalf("grouped by phone: %v", g)
	}
	g = vet.expect(200, "GET", "/api/patients/grouped?q=p%C3%A9rez", nil)["groups"].([]any) // two different Luis Pérez
	if len(g) != 2 {
		t.Fatalf("both owners named Pérez: %v", g)
	}
	for _, x := range g {
		for _, p := range x.(map[string]any)["pets"].([]any) {
			if p.(map[string]any)["matched"] != true {
				t.Fatalf("found through the owner, every pet is a result: %v", p)
			}
		}
	}

	// searching a pet name that two owners share shows each with its owner; the owner's other pets are not results
	g = vet.expect(200, "GET", "/api/patients/grouped?q=max", nil)["groups"].([]any)
	if len(g) != 2 {
		t.Fatalf("two owners have a Max: %v", g)
	}
	owners := map[string]bool{}
	for _, x := range g {
		grp := x.(map[string]any)
		owners[grp["owner"].(map[string]any)["name"].(string)] = true
		for _, p := range grp["pets"].([]any) {
			m := p.(map[string]any)
			if (m["names"] == "Max") != (m["matched"] == true) {
				t.Fatalf("only the Max is the result: %v", m)
			}
		}
	}
	if !owners["Luis Pérez"] || !owners["Marco Díaz"] {
		t.Fatalf("owners: %v", owners)
	}
	// the plain list still shows whose each pet is
	for _, r := range vet.expect(200, "GET", "/api/patients/?q=max", nil)["patients"].([]any) {
		m := r.(map[string]any)
		if m["guardian_name"] == "" || m["owner_id"] == nil {
			t.Fatalf("a pet row names its owner: %v", m)
		}
	}

	// the front desk's lookup also tells the species (it helps tell two pets apart) but not the visit history
	for _, r := range recep.expect(200, "GET", "/api/patients/lookup?q=max", nil)["patients"].([]any) {
		if m := r.(map[string]any); m["species"] != "Perro" || m["last_encounter_at"] != nil || m["guardian_name"] == "" {
			t.Fatalf("lookup row: %v", m)
		}
	}

	// the picker of the forms
	got := recep.expect(200, "GET", "/api/patients/owners?q=luis", nil)["owners"].([]any)
	if len(got) != 2 {
		t.Fatalf("owner search: %v", got)
	}
	for _, o := range got {
		if m := o.(map[string]any); m["id"] == max1["owner_id"] && int(num(m, "pet_count")) != 2 {
			t.Fatalf("Luis has two pets: %v", m)
		}
	}
	// a third pet is added to an existing owner picked from the list
	rex := pet(vet, "Rex", "", "", map[string]any{"owner_id": max1["owner_id"]})
	if rex["owner_id"] != max1["owner_id"] || rex["guardian_name"] != "Luis Pérez" || rex["guardian_phone"] != "664 111 2222" {
		t.Fatalf("an existing owner is used as is: %v", rex)
	}
	sib := vet.expect(200, "GET", "/api/patients/"+rex["id"].(string)+"/owner", nil)
	if len(sib["siblings"].([]any)) != 2 || sub(sib, "owner")["name"] != "Luis Pérez" {
		t.Fatalf("siblings: %v", sib)
	}

	// correcting the owner's data reaches every pet
	oid := max1["owner_id"].(string)
	recep.expect(400, "PUT", "/api/patients/owners/"+oid, map[string]any{"name": "", "phone": "1"})
	recep.expect(200, "PUT", "/api/patients/owners/"+oid, map[string]any{"name": "Luis A. Pérez", "phone": "664 111 2299", "email": "luis@mail.mx"})
	if p := sub(vet.expect(200, "GET", "/api/patients/"+luna["id"].(string), nil), "patient"); p["guardian_name"] != "Luis A. Pérez" || p["guardian_phone"] != "664 111 2299" || p["guardian_email"] != "luis@mail.mx" {
		t.Fatalf("the pets must follow the owner: %v", p)
	}
	e.login("cash_a").expect(403, "PUT", "/api/patients/owners/"+oid, map[string]any{"name": "X"})
	e.login("recep_b").expect(404, "PUT", "/api/patients/owners/"+oid, map[string]any{"name": "Hack"})
	if n := len(e.login("recep_b").expect(200, "GET", "/api/patients/owners?q=luis", nil)["owners"].([]any)); n != 0 {
		t.Fatalf("another clinic sees %d owners", n)
	}
	// an owner of another clinic cannot be used
	foreign := e.login("doc_b")
	e.exec(`UPDATE clinics SET kind = 'VETERINARY' WHERE id = $1`, e.clinicB)
	foreign.expect(400, "POST", "/api/patients/", map[string]any{"subject": "animal", "names": "Spy", "privacy_ack": true, "owner_id": oid,
		"profile": map[string]any{"species": "Perro", "allergies_text": "x", "sterilized": "Sí"}})

	// the same person registered twice can be merged: pets move, the empty owner goes away
	recep.expect(403, "POST", "/api/patients/owners/"+other["owner_id"].(string)+"/merge", map[string]any{"into": oid})
	merged := vet.expect(200, "POST", "/api/patients/owners/"+other["owner_id"].(string)+"/merge", map[string]any{"into": oid})
	if num(merged, "moved") != 1 {
		t.Fatalf("merge: %v", merged)
	}
	if p := sub(vet.expect(200, "GET", "/api/patients/"+other["id"].(string), nil), "patient"); p["owner_id"] != oid || p["guardian_phone"] != "664 111 2299" {
		t.Fatalf("merged pet: %v", p)
	}
	vet.expect(400, "POST", "/api/patients/owners/"+oid+"/merge", map[string]any{"into": oid})
}
