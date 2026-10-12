package api_test

import (
	"strings"
	"testing"
)

// A service can be given by video call where that is prudent; the link belongs to the appointment.
func TestVideoServicesAndAppointments(t *testing.T) {
	b := newBookingEnv(t, false)
	admin, anon := b.login("admin_a"), b.anon()
	book := func(extra map[string]any, start string) (int, map[string]any) {
		body := map[string]any{
			"professional_id": b.pro, "date": b.date, "start": start, "names": "Ana", "last_names": "López",
			"email": "ana" + start[:2] + "@correo.mx", "accept_privacy": true,
		}
		for k, v := range extra {
			body[k] = v
		}
		return anon.do("POST", "/api/public/booking/"+b.slugA+"/appointments", body)
	}
	svc := func(name, modality string) string {
		return sub(admin.expect(201, "POST", "/api/pos/items", map[string]any{"kind": "service", "name": name, "price_cents": 50000, "modality": modality}), "item")["id"].(string)
	}

	// general medicine is a video giro: services can be created for it; a bad value is refused
	admin.expect(400, "POST", "/api/pos/items", map[string]any{"kind": "service", "name": "X", "price_cents": 100, "modality": "holograma"})
	video, both, inPerson := svc("Consulta por video", "videollamada"), svc("Seguimiento", "ambas"), svc("Revisión", "presencial")
	info := sub(anon.expect(200, "GET", "/api/public/booking/"+b.slugA, nil), "booking")
	modalities := map[string]string{}
	for _, s := range info["services"].([]any) {
		m := s.(map[string]any)
		modalities[m["name"].(string)] = m["modality"].(string)
	}
	if modalities["Consulta por video"] != "videollamada" || modalities["Seguimiento"] != "ambas" || modalities["Revisión"] != "presencial" {
		t.Fatalf("the booking page tells the modality of each service: %v", modalities)
	}
	if info["professionals"].([]any)[0].(map[string]any)["video"] != true {
		t.Fatalf("the professional can give video calls: %v", info["professionals"])
	}

	// a video service gets a private room for this appointment
	code, out := book(map[string]any{"service_id": video}, "09:00")
	appt := sub(out, "appointment")
	if code != 201 || appt["modality"] != "videollamada" || !strings.HasPrefix(appt["video_url"].(string), "https://meet.jit.si/caresia-") {
		t.Fatalf("video appointment: %d %v", code, out)
	}
	page := sub(anon.expect(200, "GET", "/api/public/appointments/"+appt["token"].(string), nil), "appointment")
	if page["video_url"] != appt["video_url"] {
		t.Fatalf("the patient's page shows the link: %v", page["video_url"])
	}
	m := b.mail.wait(t, 1)
	if !strings.Contains(m.Text, appt["video_url"].(string)) {
		t.Fatalf("the confirmation mail carries the link:\n%s", m.Text)
	}

	// «ambas»: the patient chooses; the default is in person and shows no link
	_, out = book(map[string]any{"service_id": both}, "10:00")
	if a := sub(out, "appointment"); a["modality"] != "presencial" || a["video_url"] != "" {
		t.Fatalf("default is in person: %v", a)
	}
	_, out = book(map[string]any{"service_id": both, "modality": "videollamada"}, "11:00")
	if a := sub(out, "appointment"); a["modality"] != "videollamada" || a["video_url"] == "" {
		t.Fatalf("chosen video: %v", a)
	}
	// an in-person service cannot be taken by video, and a pet is never a video call
	if code, _ := book(map[string]any{"service_id": inPerson, "modality": "videollamada"}, "12:00"); code != 400 {
		t.Fatalf("in-person service by video: %d", code)
	}

	// the professional's own link wins over a private room
	b.exec(`UPDATE professional_settings SET video_url = 'https://meet.google.com/abc-defg-hij' WHERE user_id = $1`, b.pro)
	_, out = book(map[string]any{"service_id": video}, "13:00")
	if a := sub(out, "appointment"); a["video_url"] != "https://meet.google.com/abc-defg-hij" {
		t.Fatalf("own link: %v", a)
	}

	// a clinic without any video giro cannot offer it
	b.exec(`UPDATE clinics SET kind = 'DENTAL', specialties = '{}' WHERE id = $1`, b.clinicA)
	admin.expect(400, "POST", "/api/pos/items", map[string]any{"kind": "service", "name": "Limpieza por video", "price_cents": 100, "modality": "videollamada"})
	if info := sub(anon.expect(200, "GET", "/api/public/booking/"+b.slugA, nil), "booking"); info["professionals"].([]any)[0].(map[string]any)["video"] != false {
		t.Fatal("dental professionals do not give video calls")
	}
	if code, _ := book(map[string]any{"service_id": video}, "14:00"); code != 400 {
		t.Fatalf("video in a dental clinic: %d", code)
	}
}

// The agenda can mark an appointment as a video call (and back).
func TestStaffVideoAppointment(t *testing.T) {
	e := setup(t)
	admin, doc := e.login("admin_a"), e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	pro := e.userID("doc_a")
	date := "2030-03-04"
	body := func(modality string) map[string]any {
		return map[string]any{"patient_id": pid, "names": "Ana", "last_names": "López", "date": date, "startHour": "10:00", "endHour": "10:30", "professional_id": pro, "modality": modality}
	}
	a := sub(admin.expect(201, "POST", "/api/appointments/", body("videollamada")), "appointment")
	if a["modality"] != "videollamada" || !strings.HasPrefix(a["video_url"].(string), "https://meet.jit.si/caresia-") {
		t.Fatalf("staff video appointment: %v", a)
	}
	id := a["id"].(string)
	url := a["video_url"]
	up := sub(admin.expect(200, "PUT", "/api/appointments/"+id, body("presencial")), "appointment")
	if up["modality"] != "presencial" {
		t.Fatalf("back to in person: %v", up)
	}
	// switching to video again keeps the room already made
	again := sub(admin.expect(200, "PUT", "/api/appointments/"+id, body("videollamada")), "appointment")
	if again["video_url"] != url {
		t.Fatalf("the room is kept: %v vs %v", again["video_url"], url)
	}
	admin.expect(400, "POST", "/api/appointments/", body("holograma"))
}
