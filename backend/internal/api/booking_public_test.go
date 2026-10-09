package api_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestProfessionalBlocksOwnTimeOnly(t *testing.T) {
	b := newBookingEnv(t, false)
	doc := b.login("doc_a")
	other := b.userID("recep_a")
	// a specialist without agenda-admin rights may block their own time, not other people's nor the whole clinic
	doc.expect(403, "POST", "/api/agenda/blocks", map[string]any{"date_from": b.date, "date_to": b.date})
	doc.expect(403, "POST", "/api/agenda/blocks", map[string]any{"professional_id": other, "date_from": b.date, "date_to": b.date})
	made := doc.expect(201, "POST", "/api/agenda/blocks", map[string]any{"professional_id": b.pro, "date_from": b.date, "date_to": b.date, "reason": "Vacaciones"})
	id := sub(made, "block")["id"].(string)
	// the public page then shows that specialist as unavailable that day
	got := b.anon().expect(200, "GET", fmt.Sprintf("/api/public/booking/%s/availability?professional=%s&date=%s", b.slugA, b.pro, b.date), nil)
	if got["unavailable"] != true || len(got["slots"].([]any)) != 0 {
		t.Fatalf("blocked day: %v", got)
	}
	// someone else's block cannot be removed by a specialist
	adminBlock := b.login("recep_a").expect(201, "POST", "/api/agenda/blocks", map[string]any{"professional_id": nil, "date_from": b.date, "date_to": b.date})
	doc.expect(404, "DELETE", "/api/agenda/blocks/"+sub(adminBlock, "block")["id"].(string), nil)
	doc.expect(204, "DELETE", "/api/agenda/blocks/"+id, nil)
}

func TestBookingHoldsTheTime(t *testing.T) {
	b := newBookingEnv(t, false)
	a, c := b.anon(), b.anon()
	hold := func(cl *client, holder string, want int) {
		t.Helper()
		cl.expect(want, "POST", "/api/public/booking/"+b.slugA+"/hold", map[string]any{"professional_id": b.pro, "date": b.date, "start": "10:00", "holder": holder})
	}
	hold(a, "holder-aaaaaaaaaaaaaaaa", 200)
	hold(a, "holder-aaaaaaaaaaaaaaaa", 200) // the same visitor may pick it again
	hold(c, "holder-bbbbbbbbbbbbbbbb", 409) // somebody else cannot
	slots := func(holder string) string {
		g := c.expect(200, "GET", fmt.Sprintf("/api/public/booking/%s/availability?professional=%s&date=%s&holder=%s", b.slugA, b.pro, b.date, holder), nil)
		var starts []string
		for _, sl := range g["slots"].([]any) {
			starts = append(starts, sl.(map[string]any)["start"].(string))
		}
		return strings.Join(starts, " ")
	}
	if strings.Contains(slots("holder-bbbbbbbbbbbbbbbb"), "10:00") || !strings.Contains(slots("holder-aaaaaaaaaaaaaaaa"), "10:00") {
		t.Fatal("a held time is hidden from others and still offered to its holder")
	}
	body := func(holder string) map[string]any {
		return map[string]any{"professional_id": b.pro, "date": b.date, "start": "10:00", "names": "Ana", "last_names": "López", "phone": "5512345678",
			"accept_privacy": true, "holder": holder}
	}
	c.expect(409, "POST", "/api/public/booking/"+b.slugA+"/appointments", body("holder-bbbbbbbbbbbbbbbb"))
	a.expect(201, "POST", "/api/public/booking/"+b.slugA+"/appointments", body("holder-aaaaaaaaaaaaaaaa"))
	// an expired hold frees the time
	hold(c, "holder-bbbbbbbbbbbbbbbb", 409) // now it is a real appointment
	b.exec(`UPDATE appointments SET status = 'cancelled' WHERE clinic_id = $1`, b.clinicA)
	hold(a, "holder-aaaaaaaaaaaaaaaa", 200)
	b.exec(`UPDATE booking_holds SET expires_at = now() - interval '1 minute'`)
	hold(c, "holder-bbbbbbbbbbbbbbbb", 200)
}

func TestBookingRegisteredByPhoneAndPets(t *testing.T) {
	b := newBookingEnv(t, false)
	ctx := context.Background()
	b.exec(`UPDATE clinics SET kind = 'VETERINARY', specialties = '{}' WHERE id = $1`, b.clinicA)
	b.exec(`INSERT INTO patients (clinic_id, file_number, subject, names, last_names, guardian_name, guardian_phone) VALUES ($1, 1, 'animal', 'Luna', 'Ruiz', 'Marta Ruiz', '55 1234 5678')`, b.clinicA)
	b.exec(`INSERT INTO patients (clinic_id, file_number, subject, names, last_names, guardian_name, guardian_phone) VALUES ($1, 2, 'animal', 'Max', 'Ruiz', 'Marta Ruiz', '(55) 1234-5678')`, b.clinicA)
	b.exec(`INSERT INTO patients (clinic_id, file_number, subject, names, last_names, guardian_name, guardian_phone) VALUES ($1, 3, 'animal', 'Otro', 'Gómez', 'Pepe', '5599999999')`, b.clinicA)
	a := b.anon()
	info := sub(a.expect(200, "GET", "/api/public/booking/"+b.slugA, nil), "booking")
	if info["animals"] != true || info["people"] != false {
		t.Fatalf("a veterinary clinic asks for pets: %v", info)
	}
	// the lookup returns pet names only, and only for that phone
	look := a.expect(200, "POST", "/api/public/booking/"+b.slugA+"/lookup", map[string]any{"phone": "5512345678"})
	pets := look["pets"].([]any)
	if len(pets) != 2 || look["person"] != false {
		t.Fatalf("lookup: %v", look)
	}
	for _, p := range pets {
		if len(p.(map[string]any)) != 2 {
			t.Fatalf("only id and name: %v", p)
		}
	}
	if none := a.expect(200, "POST", "/api/public/booking/"+b.slugA+"/lookup", map[string]any{"phone": "5500000000"}); len(none["pets"].([]any)) != 0 {
		t.Fatalf("unknown phone: %v", none)
	}
	a.expect(400, "POST", "/api/public/booking/"+b.slugA+"/lookup", map[string]any{"phone": "12"})

	req := func(extra map[string]any) map[string]any {
		m := map[string]any{"professional_id": b.pro, "date": b.date, "start": "10:00", "accept_privacy": true, "animal": true}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	url := "/api/public/booking/" + b.slugA + "/appointments"
	a.expect(400, "POST", url, req(map[string]any{"registered": true, "phone": "5512345678"}))                                                       // which pet?
	a.expect(409, "POST", url, req(map[string]any{"registered": true, "phone": "5588888888", "patient_id": pets[0].(map[string]any)["id"]}))         // no such phone
	a.expect(409, "POST", url, req(map[string]any{"registered": true, "phone": "5512345678", "patient_id": "11111111-1111-1111-1111-111111111111"})) // not one of theirs
	ok := a.expect(201, "POST", url, req(map[string]any{"registered": true, "phone": "5512345678", "patient_id": pets[0].(map[string]any)["id"]}))
	var linked *string
	var names string
	if err := b.pool.QueryRow(ctx, `SELECT patient_id::text, names FROM appointments WHERE clinic_id = $1 AND confirm_token = $2`, b.clinicA, b.token(ok)).Scan(&linked, &names); err != nil || linked == nil {
		t.Fatalf("a registered booking attaches to the record: %v %v", linked, err)
	}
	// a new client with a pet: the owner, the kind of pet and the reason
	a.expect(400, "POST", url, req(map[string]any{"start": "10:30", "names": "Pepe", "last_names": "Gómez", "phone": "5577777777", "species": "Dragón"}))
	a.expect(201, "POST", url, req(map[string]any{"start": "10:30", "names": "Pepe", "last_names": "Gómez", "phone": "5577777777", "species": "Gato", "pet_name": "Tom", "reason": "Vacuna"}))
	var n int
	_ = b.pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE clinic_id = $1 AND body LIKE '%mascota%'`, b.clinicA).Scan(&n)
	if n == 0 {
		t.Fatal("the clinic is notified, saying it is a pet")
	}
	// a people-only visit to a veterinary is refused
	a.expect(400, "POST", url, req(map[string]any{"animal": false, "start": "11:00", "names": "Pepe", "last_names": "Gómez", "phone": "5577777777"}))
}
