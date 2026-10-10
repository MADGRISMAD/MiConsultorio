package api_test

import (
	"testing"
	"time"
)

func TestRecommendedFollowUpGoesToTheAgendaPendingConfirmation(t *testing.T) {
	e := setup(t)
	doc, recep, cash := e.login("doc_a"), e.login("recep_a"), e.login("cash_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	url := "/api/patients/" + pid + "/follow-up"

	day := time.Now().AddDate(0, 0, 8)
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDate(0, 0, 1)
	}
	date := day.Format("2006-01-02")

	first := sub(doc.expect(201, "POST", url, map[string]any{"date": date, "reason": "Revisión de resultados"}), "appointment")
	if first["status"] != "scheduled" || first["patient_id"] != pid || first["professional_name"] == "" {
		t.Fatalf("the recommended visit must wait for confirmation: %v", first)
	}
	// the patient already has an open visit in this giro: no second one until that is moved or cancelled
	if code, o := doc.do("POST", url, map[string]any{"date": date}); code != 409 || o["code"] != "ALREADY_BOOKED_AREA" {
		t.Fatalf("one open visit per giro: %d %v", code, o)
	}
	// another professional of the clinic can see patients at the very same time
	body := map[string]any{"date": date, "start_hour": first["startHour"], "professional_id": nil}
	recep.expect(201, "POST", url, body)

	doc.expect(400, "POST", url, map[string]any{"date": time.Now().AddDate(0, 0, -2).Format("2006-01-02")})
	doc.expect(400, "POST", url, map[string]any{"date": "no es fecha"})
	cash.expect(403, "POST", url, map[string]any{"date": date})
}

func TestQuickRegistrationCanRecordThePrivacyNotice(t *testing.T) {
	e := setup(t)
	recep := e.login("recep_a")
	with := sub(recep.expect(201, "POST", "/api/patients/quick", map[string]any{"names": "Luis", "last_names": "Gómez", "phone": "664 111 2222", "privacy_ack": true}), "patient")
	if with["privacy_notice_at"] == nil || with["privacy_notice_by"] == "" {
		t.Fatalf("the notice must be recorded: %v", with)
	}
	without := sub(recep.expect(201, "POST", "/api/patients/quick", map[string]any{"names": "Eva", "last_names": "Ruiz", "phone": "664 111 3333"}), "patient")
	if without["privacy_notice_at"] != nil {
		t.Fatalf("without the box the notice stays pending: %v", without)
	}
}

// A doctor can send the patient to someone else of the team: the visit says who referred, and the other person is told.
func TestReferralToAnotherProfessional(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	day := time.Now().AddDate(0, 0, 8)
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDate(0, 0, 1)
	}
	target := e.userID("admin_a")
	a := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/follow-up", map[string]any{"date": day.Format("2006-01-02"), "professional_id": target, "reason": "Rehabilitación de rodilla"}), "appointment")
	if a["professional_id"] != target {
		t.Fatalf("with the other professional: %v", a)
	}
	if n := e.scalar(`SELECT count(*) FROM notifications WHERE clinic_id = $1 AND user_id = $2 AND kind = 'referral'`, e.clinicA, target); n != 1 {
		t.Fatalf("the other professional is told: %v", n)
	}
	doc.expect(400, "POST", "/api/patients/"+pid+"/follow-up", map[string]any{"date": day.Format("2006-01-02"), "professional_id": e.userID("cash_a")})
	doc.expect(400, "POST", "/api/patients/"+pid+"/follow-up", map[string]any{"date": day.Format("2006-01-02"), "professional_id": e.userID("doc_b")})
}

// A patient with an open appointment in a giro cannot book another in the same giro, but can in a different one.
func TestOneOpenAppointmentPerGiro(t *testing.T) {
	e := setup(t)
	admin, doc := e.login("admin_a"), e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	e.exec(`UPDATE clinics SET plan = 'crecimiento', specialties = '{PHYSIOTHERAPY}' WHERE id = $1`, e.clinicA)
	e.exec(`UPDATE users SET areas = '{PHYSIOTHERAPY}' WHERE id = $1`, e.userID("admin_a"))
	day := time.Now().AddDate(0, 0, 8)
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDate(0, 0, 1)
	}
	date := day.Format("2006-01-02")
	url := "/api/patients/" + pid + "/follow-up"
	first := sub(doc.expect(201, "POST", url, map[string]any{"date": date}), "appointment") // general medicine
	if code, o := doc.do("POST", url, map[string]any{"date": date}); code != 409 || o["code"] != "ALREADY_BOOKED_AREA" {
		t.Fatalf("same giro: %d %v", code, o)
	}
	// another giro (physiotherapy) is fine
	sub(doc.expect(201, "POST", url, map[string]any{"date": date, "professional_id": e.userID("admin_a")}), "appointment")
	// moving the first one is not blocked by itself
	admin.do("GET", "/api/appointments/"+first["id"].(string), nil)
}

// Someone being attended can be given the next visit even with another one pending in the giro; free slots come from the calendar.
func TestFollowUpWhileAttendedAndFreeSlots(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	day := time.Now().AddDate(0, 0, 8)
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDate(0, 0, 1)
	}
	date := day.Format("2006-01-02")
	url := "/api/patients/" + pid + "/follow-up"
	doc.expect(201, "POST", url, map[string]any{"date": date})
	if code, _ := doc.do("POST", url, map[string]any{"date": date}); code != 409 {
		t.Fatalf("pending in the giro: %d", code)
	}
	e.exec(`INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, patient_id, professional_id, status)
		VALUES ($1, 'X', 'A', 'B', (now() AT TIME ZONE 'America/Mexico_City')::date, '00:00', '00:30', $2, $3, 'in_progress')`, e.clinicA, pid, e.userID("doc_a"))
	doc.expect(201, "POST", url, map[string]any{"date": date}) // now being attended: allowed

	out := doc.expect(200, "GET", "/api/agenda/free-slots?date="+date, nil)
	if _, ok := out["slots"].([]any); !ok {
		t.Fatalf("slots: %v", out)
	}
	doc.expect(400, "GET", "/api/agenda/free-slots?date=nope", nil)
}

func TestPendingCheckBeforeSaving(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	day := time.Now().AddDate(0, 0, 8)
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDate(0, 0, 1)
	}
	check := "/api/agenda/pending-check?patient=" + pid + "&professional=" + e.userID("doc_a")
	if out := doc.expect(200, "GET", check, nil); out["pending"] != nil {
		t.Fatalf("nothing pending yet: %v", out)
	}
	doc.expect(201, "POST", "/api/patients/"+pid+"/follow-up", map[string]any{"date": day.Format("2006-01-02")})
	out := doc.expect(200, "GET", check, nil)
	if pend, _ := out["pending"].(map[string]any); pend == nil || pend["with"] == "" {
		t.Fatalf("pending: %v", out)
	}
	doc.expect(400, "GET", "/api/agenda/pending-check?patient=x", nil)
}
