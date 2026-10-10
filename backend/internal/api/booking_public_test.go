package api_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
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
		if len(p.(map[string]any)) != 3 { // id, name and who has been attending the pet (empty without history)
			t.Fatalf("only id, name and professional: %v", p)
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

func TestBookingMailsTheSpecialist(t *testing.T) {
	b := newBookingEnv(t, false)
	post := func(start string) {
		t.Helper()
		b.exec(`UPDATE appointments SET status = 'cancelled'`) // one open visit per giro: the previous one is out of the way
		b.anon().expect(201, "POST", "/api/public/booking/"+b.slugA+"/appointments", map[string]any{"professional_id": b.pro, "date": b.date, "start": start,
			"names": "Ana", "last_names": "López", "phone": "5512345678", "reason": "Dolor privado", "accept_privacy": true})
	}
	// no e-mail on the specialist's account: the booking still works and nothing is sent
	b.exec(`UPDATE users SET email = NULL WHERE id = $1`, b.pro)
	post("10:00")
	if n := b.mail.count(); n != 0 {
		t.Fatalf("no address, no mail: %d", n)
	}
	b.exec(`UPDATE users SET email = 'doc@clinica.mx' WHERE id = $1`, b.pro)
	post("10:30")
	m := b.mail.wait(t, 1)
	if m.To[0] != "doc@clinica.mx" || !strings.Contains(m.Text, "Ana López") || !strings.Contains(m.Text, "10:30") || strings.Contains(m.Text+m.HTML, "Dolor privado") {
		t.Fatalf("specialist mail (the reason must not travel by e-mail): %+v", m)
	}
}

func TestRegisteredPatientBooksWithTheirProfessional(t *testing.T) {
	b := newBookingEnv(t, false)
	ctx := context.Background()
	other := b.userID("admin_a")
	b.exec(`INSERT INTO professional_settings (user_id, clinic_id, bookable, slot_minutes) VALUES ($1, $2, true, 30)`, other, b.clinicA)
	var pid string
	if err := b.pool.QueryRow(ctx, `INSERT INTO patients (clinic_id, file_number, names, last_names, phone) VALUES ($1, 1, 'Luis', 'Pérez', '55 1234 5678') RETURNING id`, b.clinicA).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	book := func(pro string, start string) (int, map[string]any) {
		return b.anon().do("POST", "/api/public/booking/"+b.slugA+"/appointments", map[string]any{"professional_id": pro, "date": b.date, "start": start,
			"registered": true, "phone": "5512345678", "accept_privacy": true})
	}
	// no history yet: any specialist
	if st, body := book(other, "10:00"); st != 201 {
		t.Fatalf("a patient with no history may pick anyone: %d %v", st, body)
	}
	b.exec(`INSERT INTO encounters (clinic_id, patient_id, author_id, author_name) VALUES ($1, $2, $3, 'Doc')`, b.clinicA, pid, b.pro)
	look := b.anon().expect(200, "POST", "/api/public/booking/"+b.slugA+"/lookup", map[string]any{"phone": "5512345678"})
	if look["professional_id"] != b.pro || look["person"] != true {
		t.Fatalf("the lookup names who has been attending them: %v", look)
	}
	if st, _ := book(other, "10:30"); st != 409 {
		t.Fatalf("a patient in treatment books only with their professional: %d", st)
	}
	b.exec(`UPDATE appointments SET status = 'cancelled'`) // one open visit per giro
	if st, body := book(b.pro, "11:00"); st != 201 {
		t.Fatalf("their own professional: %d %v", st, body)
	}
}

func TestProfessionalHoursFallBackPerDay(t *testing.T) {
	b := newBookingEnv(t, false)
	// the specialist sets only one day of their own; every other day keeps the clinic's hours, as the settings screen says
	b.exec(`UPDATE professional_settings SET hours = '{"mon":[["09:00","10:00"]]}' WHERE user_id = $1`, b.pro)
	var mon, tue string
	for d, n := time.Now().AddDate(0, 0, 3), 0; n < 14; d, n = d.AddDate(0, 0, 1), n+1 {
		if d.Weekday() == time.Monday && mon == "" {
			mon = d.Format("2006-01-02")
		}
		if d.Weekday() == time.Tuesday && tue == "" {
			tue = d.Format("2006-01-02")
		}
	}
	count := func(date string) int {
		g := b.anon().expect(200, "GET", fmt.Sprintf("/api/public/booking/%s/availability?professional=%s&date=%s", b.slugA, b.pro, date), nil)
		return len(g["slots"].([]any))
	}
	if n := count(mon); n != 2 {
		t.Fatalf("Monday uses the specialist's own 09:00-10:00 (two 30-minute slots): %d", n)
	}
	if n := count(tue); n < 2 {
		t.Fatalf("Tuesday falls back to the clinic's hours, it must not be closed: %d", n)
	}
}

func TestOwnerWhoDoesNotConsultIsNotOffered(t *testing.T) {
	b := newBookingEnv(t, false)
	admin := b.login("admin_a")
	owner := b.userID("admin_a")
	body := func(consults any) map[string]any {
		m := map[string]any{"bookable": true, "slot_minutes": 30, "hours": map[string]any{}, "color": ""}
		if consults != nil {
			m["consults"] = consults
		}
		return m
	}
	hour := 12
	appt := func(want int) {
		t.Helper()
		hour++
		admin.expect(want, "POST", "/api/appointments", map[string]any{"names": "Ana", "last_names": "López", "date": b.date,
			"startHour": fmt.Sprintf("%02d:00", hour), "endHour": fmt.Sprintf("%02d:30", hour), "professional_id": owner})
	}
	// the owner also consults: offered online and in the agenda
	admin.expect(200, "PUT", "/api/agenda/professionals/"+owner, body(true))
	if n := len(sub(b.anon().expect(200, "GET", "/api/public/booking/"+b.slugA, nil), "booking")["professionals"].([]any)); n != 2 {
		t.Fatalf("both consult: %d", n)
	}
	appt(201)
	// "solo soy el dueño": not offered online, no appointments can be given to them, but the settings still list them
	out := sub(admin.expect(200, "PUT", "/api/agenda/professionals/"+owner, body(false)), "professional")
	if out["consults"] != false || out["bookable"] != false {
		t.Fatalf("not consulting also leaves the online booking: %v", out)
	}
	if n := len(sub(b.anon().expect(200, "GET", "/api/public/booking/"+b.slugA, nil), "booking")["professionals"].([]any)); n != 1 {
		t.Fatalf("only the other specialist: %d", n)
	}
	appt(400)
	admin.expect(400, "POST", "/api/agenda/blocks", map[string]any{"professional_id": owner, "date_from": b.date, "date_to": b.date})
	found := false
	for _, p := range admin.expect(200, "GET", "/api/agenda/professionals", nil)["professionals"].([]any) {
		found = found || (p.(map[string]any)["id"] == owner && p.(map[string]any)["consults"] == false)
	}
	if !found {
		t.Fatal("the settings list keeps them so they can switch back")
	}
	// omitting the flag leaves it as it was
	admin.expect(200, "PUT", "/api/agenda/professionals/"+owner, body(nil))
	appt(400)
	admin.expect(200, "PUT", "/api/agenda/professionals/"+owner, body(true))
	appt(201)
}

func TestVideoLinkTravelsInTheAppointmentMail(t *testing.T) {
	b := newBookingEnv(t, false)
	admin := b.login("admin_a")
	body := func(url string) map[string]any {
		return map[string]any{"bookable": true, "slot_minutes": 30, "hours": map[string]any{}, "color": "", "video_url": url}
	}
	admin.expect(400, "PUT", "/api/agenda/professionals/"+b.pro, body("http://inseguro.mx/sala"))
	admin.expect(400, "PUT", "/api/agenda/professionals/"+b.pro, body("javascript:alert(1)"))
	out := sub(admin.expect(200, "PUT", "/api/agenda/professionals/"+b.pro, body("https://meet.example.com/sala-dra")), "professional")
	if out["video_url"] != "https://meet.example.com/sala-dra" {
		t.Fatalf("video_url: %v", out["video_url"])
	}
	b.login("recep_a").expect(201, "POST", "/api/appointments", map[string]any{"names": "Ana", "last_names": "López", "date": b.date, "startHour": "10:00", "endHour": "10:30",
		"professional_id": b.pro, "email": "ana@x.mx", "reminders_consent": true})
	m := b.mail.wait(t, 1)
	if !strings.Contains(m.Text, "Videollamada: https://meet.example.com/sala-dra") {
		t.Fatalf("the booking mail carries the video link: %s", m.Text)
	}
	// leaving the field out keeps it; an empty string removes it
	admin.expect(200, "PUT", "/api/agenda/professionals/"+b.pro, map[string]any{"bookable": true, "slot_minutes": 30, "hours": map[string]any{}, "color": ""})
	has := func() string {
		for _, p := range admin.expect(200, "GET", "/api/agenda/professionals", nil)["professionals"].([]any) {
			if p.(map[string]any)["id"] == b.pro {
				return p.(map[string]any)["video_url"].(string)
			}
		}
		return "?"
	}
	if has() == "" {
		t.Fatal("omitting the field keeps the link")
	}
	admin.expect(200, "PUT", "/api/agenda/professionals/"+b.pro, body(""))
	if has() != "" {
		t.Fatal("an empty string removes it")
	}
}
