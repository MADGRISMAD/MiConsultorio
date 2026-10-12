package api_test

import "testing"

// A package of sessions: created by the clinic's specialists, sold at the POS to a patient and drawn down by appointments.
func TestSessionPackages(t *testing.T) {
	e := setup(t)
	e.exec(`UPDATE clinics SET kind = 'PSYCHOLOGY' WHERE id = $1`, e.clinicA)
	admin, doc, recep := e.login("admin_a"), e.login("doc_a"), e.login("recep_a")

	svc := sub(admin.expect(201, "POST", "/api/pos/items", map[string]any{"kind": "service", "name": "Sesión de terapia", "price_cents": 80000}), "item")["id"].(string)
	other := sub(admin.expect(201, "POST", "/api/pos/items", map[string]any{"kind": "service", "name": "Evaluación", "price_cents": 90000}), "item")["id"].(string)

	// specialists create packages; reception only looks; invalid ones are refused
	recep.expect(403, "POST", "/api/packages", map[string]any{"name": "X", "price_cents": 100, "sessions": 5})
	doc.expect(400, "POST", "/api/packages", map[string]any{"name": "Paquete", "price_cents": 300000, "sessions": 1})
	doc.expect(400, "POST", "/api/packages", map[string]any{"name": "", "price_cents": 300000, "sessions": 5})
	doc.expect(400, "POST", "/api/packages", map[string]any{"name": "Paquete", "price_cents": 300000, "sessions": 5, "service_id": "no-es-uuid"})
	pk := sub(doc.expect(200, "POST", "/api/packages", map[string]any{"name": "Terapia ×5", "price_cents": 350000, "sessions": 5, "service_id": svc, "valid_days": 1825, "public": true}), "package")
	if pk["sessions"].(float64) != 5 || pk["service_name"] != "Sesión de terapia" {
		t.Fatalf("package: %v", pk)
	}
	pid := pk["id"].(string)
	doc.expect(200, "PUT", "/api/packages/"+pid, map[string]any{"name": "Terapia ×5 (promo)", "price_cents": 340000, "sessions": 5, "service_id": svc, "valid_days": 1825, "public": true})
	if n := len(recep.expect(200, "GET", "/api/packages", nil)["packages"].([]any)); n != 1 {
		t.Fatalf("list: %d", n)
	}
	// a package is not a service you can book
	e.exec(`INSERT INTO agenda_settings (clinic_id, booking_enabled, booking_slug) VALUES ($1, true, 'psico-a')`, e.clinicA)
	for _, s := range sub(e.anon().expect(200, "GET", "/api/public/booking/psico-a", nil), "booking")["services"].([]any) {
		if s.(map[string]any)["name"] == "Terapia ×5 (promo)" {
			t.Fatal("packages are not offered for online booking")
		}
	}
	// clinics without a giro that sells packages cannot create them
	e.login("admin_b").expect(409, "POST", "/api/packages", map[string]any{"name": "Paquete", "price_cents": 300000, "sessions": 5})

	// sale: needs the patient
	patient := newPerson(t, doc, "mejj700312hdfdrr04")
	admin.expect(201, "POST", "/api/pos/cash/open", map[string]any{"opening_cents": 0})
	pay := []map[string]any{{"method": "cash", "amount_cents": 340000}}
	admin.expect(400, "POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": pid, "qty": 1}}, "payments": pay})
	sale := sub(admin.expect(201, "POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": pid, "qty": 1}}, "payments": pay, "patient_id": patient}), "sale")
	list := recep.expect(200, "GET", "/api/patients/"+patient+"/packages", nil)["packages"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["sessions_total"].(float64) != 5 || list[0].(map[string]any)["sessions_used"].(float64) != 0 || list[0].(map[string]any)["expires_on"] == nil {
		t.Fatalf("patient packages: %v", list)
	}
	mine := list[0].(map[string]any)["id"].(string)

	// an appointment of that service uses a session; another service does not
	appt := func(service string, hour string) string {
		return sub(recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{
			"professional_id": e.userID("doc_a"), "patient_id": patient, "service_id": service, "startHour": hour, "endHour": hour[:2] + ":30"})), "appointment")["id"].(string)
	}
	// the patient has one open visit per giro, so each step creates its appointment after cancelling the last
	a2 := appt(other, "10:00")
	recep.expect(409, "POST", "/api/appointments/"+a2+"/package", map[string]any{"package_id": mine})
	recep.expect(200, "POST", "/api/appointments/"+a2+"/status", map[string]any{"status": "cancelled", "reason": "x"})
	a1 := appt(svc, "09:00")
	if out := recep.expect(200, "POST", "/api/appointments/"+a1+"/package", map[string]any{"package_id": mine}); out["sessions_left"].(float64) != 4 {
		t.Fatalf("redeem: %v", out)
	}
	recep.expect(409, "POST", "/api/appointments/"+a1+"/package", map[string]any{"package_id": mine}) // already paid with a package
	if got := sub(recep.expect(200, "GET", "/api/appointments/"+a1, nil), "appointment")["package_id"]; got != mine {
		t.Fatalf("the appointment shows its package: %v", got)
	}

	// cancelling gives the session back; so does undoing it
	recep.expect(200, "POST", "/api/appointments/"+a1+"/status", map[string]any{"status": "cancelled", "reason": "x"})
	used := func() float64 {
		return recep.expect(200, "GET", "/api/patients/"+patient+"/packages", nil)["packages"].([]any)[0].(map[string]any)["sessions_used"].(float64)
	}
	if used() != 0 {
		t.Fatalf("a cancelled visit returns its session: %v", used())
	}
	a3 := appt(svc, "11:00")
	recep.expect(200, "POST", "/api/appointments/"+a3+"/package", map[string]any{"package_id": mine})
	recep.expect(204, "DELETE", "/api/appointments/"+a3+"/package", nil)
	if used() != 0 {
		t.Fatalf("undo returns the session: %v", used())
	}

	// voiding the sale cancels the package
	admin.expect(200, "POST", "/api/pos/sales/"+sale["id"].(string)+"/void", map[string]any{"reason": "error de captura"})
	recep.expect(409, "POST", "/api/appointments/"+a3+"/package", map[string]any{"package_id": mine})
}
