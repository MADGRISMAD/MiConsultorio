package api_test

import (
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
)

// at moves the appointment clock to a moment of the 2030-06-03 visits (Mexico City time).
func at(t *testing.T, h, m int) func() {
	t.Helper()
	loc, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		t.Fatal(err)
	}
	return api.SetApptClock(func() time.Time { return time.Date(2030, 6, 3, h, m, 0, 0, loc) })
}

func apptBody(extra map[string]any) map[string]any {
	m := map[string]any{"names": "Ana", "last_names": "Pérez", "date": "2030-06-03", "startHour": "09:00", "endHour": "09:30"}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func errCode(t *testing.T, c *client, method, path string, body any, status int) string {
	t.Helper()
	out := c.expect(status, method, path, body)
	code, _ := out["code"].(string)
	return code
}

func TestAgendaConflicts(t *testing.T) {
	e := setup(t)
	recep := e.login("recep_a")
	doc, doc2 := e.userID("doc_a"), e.userID("admin_a")

	a := sub(recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": doc, "room": "1"})), "appointment")
	if a["status"] != "scheduled" || a["professional_name"] != "DOC_A" || a["source"] != "staff" {
		t.Fatalf("defaults: %v", a)
	}
	var token string
	if err := e.pool.QueryRow(t.Context(), `SELECT confirm_token FROM appointments WHERE id = $1`, a["id"]).Scan(&token); err != nil || len(token) < 40 {
		t.Fatalf("confirm token: %q %v", token, err)
	}

	// same professional overlapping: taken; adjacent: free; other professional, other room: free
	if c := errCode(t, recep, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": doc, "startHour": "09:15", "endHour": "09:45"}), 409); c != "SLOT_TAKEN" {
		t.Fatalf("code %q", c)
	}
	recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": doc, "startHour": "09:30", "endHour": "10:00"}))
	recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": doc2, "room": "2"}))
	// the room is also exclusive
	if c := errCode(t, recep, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": doc2, "room": "1"}), 409); c != "SLOT_TAKEN" {
		t.Fatalf("room code %q", c)
	}
	// overbooking is allowed for taken slots
	o := sub(recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": doc, "overbook": true})), "appointment")
	var n int
	_ = e.pool.QueryRow(t.Context(), `SELECT count(*) FROM activity_log WHERE type = 'appointment_create' AND meta->>'overbook' = 'true'`).Scan(&n)
	if n != 1 {
		t.Fatalf("overbook not audited: %d", n)
	}

	// a cancelled appointment frees the slot
	recep.expect(200, "POST", "/api/appointments/"+o["id"].(string)+"/status", map[string]any{"status": "cancelled", "reason": "x"})
	recep.expect(200, "POST", "/api/appointments/"+a["id"].(string)+"/status", map[string]any{"status": "cancelled"})
	recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": doc, "room": "1"}))

	// moving: conflict on another, free elsewhere; editing details of an overbooked slot keeps working
	b := sub(recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": doc, "startHour": "11:00", "endHour": "11:30"})), "appointment")
	move := apptBody(map[string]any{"professional_id": doc, "startHour": "09:00", "endHour": "09:30", "room": "1"})
	errCode(t, recep, "PUT", "/api/appointments/"+b["id"].(string), move, 409)
	move["startHour"], move["endHour"] = "12:00", "12:30"
	recep.expect(200, "PUT", "/api/appointments/"+b["id"].(string), move)

	// validation of references
	recep.expect(400, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": e.userID("recep_a")}))
	recep.expect(400, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": e.userID("doc_b")}))
	recep.expect(400, "POST", "/api/appointments", apptBody(map[string]any{"service_id": "00000000-0000-0000-0000-000000000001"}))
	recep.expect(400, "POST", "/api/appointments", apptBody(map[string]any{"email": "nope"}))

	// the cancelled appointment cannot be edited
	recep.expect(409, "PUT", "/api/appointments/"+a["id"].(string), apptBody(nil))
}

func TestAgendaServiceAndFilters(t *testing.T) {
	e := setup(t)
	recep := e.login("recep_a")
	var svc, prod, svcB string
	for _, r := range []struct {
		clinic, kind string
		dst          *string
	}{{e.clinicA, "service", &svc}, {e.clinicA, "product", &prod}, {e.clinicB, "service", &svcB}} {
		if err := e.pool.QueryRow(t.Context(), `INSERT INTO catalog_items (clinic_id, kind, name, price_cents) VALUES ($1, $2, 'Limpieza', 500) RETURNING id::text`, r.clinic, r.kind).Scan(r.dst); err != nil {
			t.Fatal(err)
		}
	}
	recep.expect(400, "POST", "/api/appointments", apptBody(map[string]any{"service_id": prod}))
	recep.expect(400, "POST", "/api/appointments", apptBody(map[string]any{"service_id": svcB}))
	a := sub(recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"service_id": svc, "phone": "5512345678", "email": "a@b.mx", "room": "1"})), "appointment")
	if a["service_name"] != "Limpieza" || a["phone"] != "5512345678" {
		t.Fatalf("service: %v", a)
	}
	doc := e.userID("doc_a")
	recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": doc, "date": "2030-06-10", "room": "2"}))

	count := func(q string) int {
		return len(recep.expect(200, "GET", "/api/appointments"+q, nil)["appointments"].([]any))
	}
	for q, want := range map[string]int{"": 2, "?from=2030-06-05": 1, "?to=2030-06-05": 1, "?professional=" + doc: 1, "?status=scheduled": 2, "?status=cancelled": 0, "?room=1": 1, "?from=2030-06-01&to=2030-06-30": 2} {
		if got := count(q); got != want {
			t.Fatalf("filter %q: got %d want %d", q, got, want)
		}
	}
	recep.expect(400, "GET", "/api/appointments?status=bogus", nil)
	recep.expect(400, "GET", "/api/appointments?from=hoy", nil)
	recep.expect(400, "GET", "/api/appointments?professional=x", nil)
}

func TestAgendaBlocks(t *testing.T) {
	e := setup(t)
	recep, admin, doc := e.login("recep_a"), e.login("admin_a"), e.userID("doc_a")

	existing := sub(recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": doc})), "appointment")
	// blocks report the appointments under them but do not cancel them
	out := recep.expect(201, "POST", "/api/agenda/blocks", map[string]any{"professional_id": doc, "date_from": "2030-06-03", "date_to": "2030-06-03", "startHour": "08:00", "endHour": "12:00", "reason": "Congreso"})
	if len(out["affected"].([]any)) != 1 {
		t.Fatalf("affected: %v", out)
	}
	if sub(recep.expect(200, "GET", "/api/appointments/"+existing["id"].(string), nil), "appointment")["status"] != "scheduled" {
		t.Fatal("block must not cancel")
	}
	blockID := sub(out, "block")["id"].(string)

	// blocked slot: never accepted, not even overbooking
	for _, ob := range []bool{false, true} {
		if c := errCode(t, recep, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": doc, "startHour": "10:00", "endHour": "10:30", "overbook": ob}), 409); c != "SLOT_BLOCKED" {
			t.Fatalf("code %q", c)
		}
	}
	// another professional and another hour are free
	recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": e.userID("admin_a"), "startHour": "10:00", "endHour": "10:30"}))
	recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": doc, "startHour": "13:00", "endHour": "13:30"}))
	// moving into a block is refused too
	errCode(t, recep, "PUT", "/api/appointments/"+existing["id"].(string), apptBody(map[string]any{"professional_id": doc, "startHour": "10:00", "endHour": "10:30"}), 409)

	// a whole-clinic all-day block
	all := sub(admin.expect(201, "POST", "/api/agenda/blocks", map[string]any{"date_from": "2030-07-01", "date_to": "2030-07-02", "reason": "Vacaciones"}), "block")
	if c := errCode(t, recep, "POST", "/api/appointments", apptBody(map[string]any{"date": "2030-07-02", "professional_id": doc}), 409); c != "SLOT_BLOCKED" {
		t.Fatalf("code %q", c)
	}
	if n := len(recep.expect(200, "GET", "/api/agenda/blocks?from=2030-06-01&to=2030-06-30", nil)["blocks"].([]any)); n != 1 {
		t.Fatalf("blocks in june: %d", n)
	}

	// validation
	for name, body := range map[string]map[string]any{
		"range":    {"date_from": "2030-07-05", "date_to": "2030-07-01"},
		"too long": {"date_from": "2030-01-01", "date_to": "2032-01-01"},
		"past":     {"date_from": "2020-01-01", "date_to": "2020-01-02"},
		"half":     {"date_from": "2030-07-05", "date_to": "2030-07-05", "startHour": "09:00"},
		"hours":    {"date_from": "2030-07-05", "date_to": "2030-07-05", "startHour": "10:00", "endHour": "09:00"},
		"foreign":  {"date_from": "2030-07-05", "date_to": "2030-07-05", "professional_id": e.userID("doc_b")},
		"nonpro":   {"date_from": "2030-07-05", "date_to": "2030-07-05", "professional_id": e.userID("recep_a")},
	} {
		if got, _ := recep.do("POST", "/api/agenda/blocks", body); got != 400 {
			t.Fatalf("%s: got %d", name, got)
		}
	}

	// roles: doctor and cashier read but cannot write
	for _, u := range []string{"doc_a", "cash_a"} {
		c := e.login(u)
		c.expect(200, "GET", "/api/agenda/blocks", nil)
		c.expect(403, "POST", "/api/agenda/blocks", map[string]any{"date_from": "2030-07-05", "date_to": "2030-07-05"})
		c.expect(403, "DELETE", "/api/agenda/blocks/"+blockID, nil)
	}
	// isolation
	other := e.login("recep_b")
	other.expect(404, "DELETE", "/api/agenda/blocks/"+blockID, nil)
	if len(other.expect(200, "GET", "/api/agenda/blocks", nil)["blocks"].([]any)) != 0 {
		t.Fatal("tenant leak")
	}
	if c := errCode(t, other, "POST", "/api/appointments", apptBody(map[string]any{"date": "2030-07-02"}), 201); c != "" {
		t.Fatal(c)
	}
	recep.expect(204, "DELETE", "/api/agenda/blocks/"+all["id"].(string), nil)
	recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"date": "2030-07-02", "professional_id": doc}))
}

func TestAgendaStatusTransitions(t *testing.T) {
	e := setup(t)
	recep, doc, doc2 := e.login("recep_a"), e.login("doc_a"), e.login("admin_a")
	docID := e.userID("doc_a")
	e.addUser(e.clinicA, "doc2_a", "doctor")
	doc3 := e.login("doc2_a")

	mk := func(start string, prof any) string {
		return sub(recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": prof, "startHour": start, "endHour": start[:3] + "45"})), "appointment")["id"].(string)
	}
	set := func(c *client, id, st string, want int) map[string]any {
		return c.expect(want, "POST", "/api/appointments/"+id+"/status", map[string]any{"status": st, "reason": "motivo"})
	}

	id := mk("09:00", docID)
	set(recep, id, "in_progress", 409) // must arrive first
	set(recep, id, "completed", 409)
	set(recep, id, "scheduled", 400)
	set(recep, id, "bogus", 400)
	set(recep, id, "confirmed", 200)
	set(recep, id, "confirmed", 409)
	// Too early: the visit is on 2030-06-03 09:00, and nothing can be marked a day (or hours) before.
	for _, st := range []string{"arrived"} {
		if code, o := recep.do("POST", "/api/appointments/"+id+"/status", map[string]any{"status": st}); code != 409 || o["code"] != "TOO_EARLY" {
			t.Fatalf("%s a year early: %d %v", st, code, o)
		}
	}
	restore := at(t, 7, 30) // 90 minutes before: still too early (the window opens one hour before)
	defer func() { restore() }()
	if code, o := recep.do("POST", "/api/appointments/"+id+"/status", map[string]any{"status": "arrived"}); code != 409 || o["code"] != "TOO_EARLY" {
		t.Fatalf("arrival 90 minutes early: %d %v", code, o)
	}
	restore()
	restore = at(t, 8, 30) // inside the window
	a := sub(set(recep, id, "arrived", 200), "appointment")
	if a["arrived_at"] == nil || a["started_at"] != nil {
		t.Fatalf("arrived: %v", a)
	}
	// the assigned doctor starts it, another doctor and the cashier cannot
	set(doc3, id, "in_progress", 403)
	e.login("cash_a").expect(403, "POST", "/api/appointments/"+id+"/status", map[string]any{"status": "in_progress"})
	a = sub(set(doc, id, "in_progress", 200), "appointment")
	if a["started_at"] == nil {
		t.Fatalf("started: %v", a)
	}
	set(recep, id, "cancelled", 409) // in consultation cannot be cancelled
	// it cannot be closed before its time even if it was started
	if code, o := doc.do("POST", "/api/appointments/"+id+"/status", map[string]any{"status": "completed"}); code != 409 || o["code"] != "TOO_EARLY" {
		t.Fatalf("completing before the appointment's time: %d %v", code, o)
	}
	restore()
	restore = at(t, 9, 10)
	a = sub(set(doc, id, "completed", 200), "appointment")
	if a["finished_at"] == nil || a["status"] != "completed" {
		t.Fatalf("finished: %v", a)
	}
	set(recep, id, "arrived", 409) // no reopening

	// cancel with reason; no-show; a doctor can cancel their own, not an unassigned one
	c1 := mk("10:00", docID)
	if sub(set(recep, c1, "cancelled", 200), "appointment")["cancel_reason"] != "motivo" {
		t.Fatal("cancel reason")
	}
	set(recep, c1, "confirmed", 409)
	// a visit cannot be marked as missed before its time
	early := mk("10:00", docID)
	if code, o := recep.do("POST", "/api/appointments/"+early+"/status", map[string]any{"status": "no_show"}); code != 409 || o["code"] != "TOO_EARLY" {
		t.Fatalf("no-show before the time: %d %v", code, o)
	}
	restore()
	restore = at(t, 10, 5)
	set(recep, early, "no_show", 200)
	restore()
	restore = at(t, 23, 0)
	set(recep, mk("10:00", docID), "no_show", 200)
	set(doc, mk("11:00", docID), "cancelled", 200)
	set(doc, mk("11:00", e.userID("admin_a")), "cancelled", 403)
	set(doc2, mk("12:00", e.userID("admin_a")), "confirmed", 200)

	// other clinic
	other := e.login("recep_b")
	other.expect(404, "POST", "/api/appointments/"+mk("13:00", docID)+"/status", map[string]any{"status": "confirmed"})
	recep.expect(404, "POST", "/api/appointments/nope/status", map[string]any{"status": "confirmed"})
}

func TestAgendaEncounterLink(t *testing.T) {
	e := setup(t)
	recep, doc := e.login("recep_a"), e.login("doc_a")
	pat := sub(recep.expect(201, "POST", "/api/patients/quick", map[string]any{"names": "Luis", "last_names": "Gómez"}), "patient")
	pid := pat["id"].(string)
	a := sub(recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"patient_id": pid, "professional_id": e.userID("doc_a")})), "appointment")
	id := a["id"].(string)

	// unknown or foreign appointment, or one of another patient
	doc.expect(400, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "Dolor", "appointment_id": "00000000-0000-0000-0000-000000000001"})
	pat2 := sub(recep.expect(201, "POST", "/api/patients/quick", map[string]any{"names": "Eva", "last_names": "Ruiz"}), "patient")
	doc.expect(400, "POST", "/api/patients/"+pat2["id"].(string)+"/encounters", map[string]any{"reason": "Dolor", "appointment_id": id})
	foreign := sub(e.login("recep_b").expect(201, "POST", "/api/appointments", apptBody(nil)), "appointment")["id"]
	doc.expect(400, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "Dolor", "appointment_id": foreign})

	// a note written long before the visit is kept with the appointment but does not close it
	early := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "Antes de tiempo", "appointment_id": id}), "encounter")
	if got := sub(recep.expect(200, "GET", "/api/appointments/"+id, nil), "appointment"); got["status"] != "scheduled" || got["encounter_id"] != early["id"] {
		t.Fatalf("a note before the time must not close the appointment: %v", got)
	}
	restore := at(t, 9, 10)
	defer restore()
	doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "Dolor", "appointment_id": id})
	got := sub(recep.expect(200, "GET", "/api/appointments/"+id, nil), "appointment")
	if got["status"] != "completed" || got["encounter_id"] != early["id"] || got["finished_at"] == nil {
		t.Fatalf("link: %v", got)
	}
}

func TestAgendaSettings(t *testing.T) {
	e := setup(t)
	admin, recep := e.login("admin_a"), e.login("recep_a")
	st := sub(recep.expect(200, "GET", "/api/agenda/settings", nil), "settings")
	if st["slot_minutes"].(float64) != 30 || st["booking_enabled"] != false {
		t.Fatalf("defaults: %v", st)
	}
	good := map[string]any{"slot_minutes": 20, "rooms": []string{"Sala 1", " Sala 2 ", "Sala 1"}, "booking_enabled": true, "booking_slug": "Mi-Clinica", "booking_lead_hours": 4,
		"booking_horizon_days": 60, "booking_message": "Hola", "booking_requires_confirmation": true, "remind_email": true, "remind_whatsapp": false,
		"remind_hours": []int{2, 24}, "reminder_template": "Te esperamos"}
	recep.expect(403, "PUT", "/api/agenda/settings", good)
	e.login("doc_a").expect(403, "PUT", "/api/agenda/settings", good)
	out := sub(admin.expect(200, "PUT", "/api/agenda/settings", good), "settings")
	if out["booking_slug"] != "mi-clinica" || len(out["rooms"].([]any)) != 2 || out["remind_hours"].([]any)[0].(float64) != 24 {
		t.Fatalf("saved: %v", out)
	}
	if sub(recep.expect(200, "GET", "/api/agenda/settings", nil), "settings")["booking_slug"] != "mi-clinica" {
		t.Fatal("not persisted")
	}

	// another clinic cannot take the slug; its own settings are separate
	adminB := e.login("admin_b")
	good["booking_slug"] = "mi-clinica"
	if c := errCode(t, adminB, "PUT", "/api/agenda/settings", good, 409); c != "SLUG_TAKEN" {
		t.Fatalf("code %q", c)
	}
	good["booking_slug"] = "otra-clinica"
	adminB.expect(200, "PUT", "/api/agenda/settings", good)
	if sub(admin.expect(200, "GET", "/api/agenda/settings", nil), "settings")["booking_slug"] != "mi-clinica" {
		t.Fatal("tenant leak")
	}

	for name, mod := range map[string]map[string]any{
		"slug short": {"booking_slug": "a"}, "slug chars": {"booking_slug": "hola mundo"}, "slug accents": {"booking_slug": "clínica"},
		"slug empty enabled": {"booking_slug": ""}, "slot": {"slot_minutes": 2}, "hours zero": {"remind_hours": []int{0}}, "hours big": {"remind_hours": []int{200}},
		"hours many": {"remind_hours": []int{1, 2, 3, 4}}, "horizon": {"booking_horizon_days": 0}, "lead": {"booking_lead_hours": -1},
	} {
		body := map[string]any{}
		for k, v := range good {
			body[k] = v
		}
		body["booking_slug"] = "otra-clinica"
		for k, v := range mod {
			body[k] = v
		}
		if got, _ := admin.do("PUT", "/api/agenda/settings", body); got != 400 {
			t.Fatalf("%s: got %d", name, got)
		}
	}
}

func TestAgendaProfessionals(t *testing.T) {
	e := setup(t)
	admin, recep, doc := e.login("admin_a"), e.login("recep_a"), e.userID("doc_a")
	list := recep.expect(200, "GET", "/api/agenda/professionals", nil)["professionals"].([]any)
	if len(list) != 2 { // the administrator and the doctor; reception and cashier are not professionals
		t.Fatalf("professionals: %v", list)
	}
	good := map[string]any{"bookable": true, "slot_minutes": 45, "color": "#3366cc", "hours": map[string]any{"mon": [][]string{{"09:00", "14:00"}, {"16:00", "19:00"}}}}
	recep.expect(403, "PUT", "/api/agenda/professionals/"+doc, good)
	e.login("doc_a").expect(403, "PUT", "/api/agenda/professionals/"+doc, good)
	pr := sub(admin.expect(200, "PUT", "/api/agenda/professionals/"+doc, good), "professional")
	if pr["bookable"] != true || pr["slot_minutes"].(float64) != 45 || pr["color"] != "#3366cc" {
		t.Fatalf("saved: %v", pr)
	}
	admin.expect(404, "PUT", "/api/agenda/professionals/"+e.userID("doc_b"), good)
	admin.expect(404, "PUT", "/api/agenda/professionals/"+e.userID("recep_a"), good)
	for name, mod := range map[string]map[string]any{
		"color": {"color": "red"}, "slot": {"slot_minutes": 1}, "day": {"hours": map[string]any{"xyz": [][]string{{"09:00", "10:00"}}}},
		"order":   {"hours": map[string]any{"mon": [][]string{{"10:00", "09:00"}}}},
		"overlap": {"hours": map[string]any{"mon": [][]string{{"09:00", "12:00"}, {"11:00", "13:00"}}}},
	} {
		body := map[string]any{}
		for k, v := range good {
			body[k] = v
		}
		for k, v := range mod {
			body[k] = v
		}
		if got, _ := admin.do("PUT", "/api/agenda/professionals/"+doc, body); got != 400 {
			t.Fatalf("%s: got %d", name, got)
		}
	}
	// a deactivated user stops being a professional
	e.exec(`UPDATE users SET disabled = true WHERE username = 'doc_a'`)
	if n := len(recep.expect(200, "GET", "/api/agenda/professionals", nil)["professionals"].([]any)); n != 1 {
		t.Fatalf("after deactivation: %d", n)
	}
}
