package api_test

import "testing"

func TestIndicators(t *testing.T) {
	b := newBookingEnv(t, false)
	admin := b.login("admin_a")
	b.exec(`INSERT INTO patients (clinic_id, file_number, names) VALUES ($1, 1, 'Ana'), ($1, 2, 'Luis')`, b.clinicA)
	b.exec(`INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, professional_id, status)
		VALUES ($1, 'X', 'Ana', 'P', current_date - 1, '09:00', '09:30', $2, 'completed'), ($1, 'X', 'Luis', 'P', current_date - 1, '10:00', '10:30', $2, 'no_show'),
		       ($1, 'X', 'Ana', 'P', current_date - 40, '09:00', '09:30', $2, 'completed')`, b.clinicA, b.pro)
	out := admin.expect(200, "GET", "/api/indicators", nil)
	cur, prev := out["current"].(map[string]any), out["previous"].(map[string]any)
	if cur["appointments"] != float64(2) {
		t.Fatalf("current: %v", cur)
	}
	if cur["new_patients"] != float64(2) || prev["new_patients"] != float64(0) || out["days"] != float64(30) {
		t.Fatalf("patients and window: %v / %v / %v", cur, prev, out["days"])
	}
	if prev["appointments"] != float64(1) {
		t.Fatalf("previous period: %v", prev)
	}
	admin.expect(400, "GET", "/api/indicators?from=2026-02-01&to=2026-01-01", nil)
	b.login("recep_a").expect(403, "GET", "/api/indicators", nil)
}
