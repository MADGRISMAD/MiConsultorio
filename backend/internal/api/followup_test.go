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
	// a second recommendation the same day for the same professional takes the next free time
	second := sub(doc.expect(201, "POST", url, map[string]any{"date": date}), "appointment")
	if second["startHour"] == first["startHour"] {
		t.Fatalf("the same professional cannot have two visits at once: %v %v", first["startHour"], second["startHour"])
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
