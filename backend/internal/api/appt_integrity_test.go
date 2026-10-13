package api_test

import "testing"

// Writing a note on a visit that was cancelled or missed keeps the note linked to it but never reopens, completes or charges it.
func TestNoteDoesNotCloseCancelledVisit(t *testing.T) {
	e := setup(t)
	recep, doc := e.login("recep_a"), e.login("doc_a")
	pid := sub(recep.expect(201, "POST", "/api/patients/quick", map[string]any{"names": "Luis", "last_names": "Gómez"}), "patient")["id"].(string)
	restore := at(t, 9, 10)
	defer restore()
	for _, status := range []string{"cancelled", "no_show"} {
		id := sub(recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"patient_id": pid, "professional_id": e.userID("doc_a")})), "appointment")["id"].(string)
		recep.expect(200, "POST", "/api/appointments/"+id+"/status", map[string]any{"status": status, "reason": "x"})
		doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", map[string]any{"reason": "Nota tardía", "appointment_id": id})
		got := sub(recep.expect(200, "GET", "/api/appointments/"+id, nil), "appointment")
		if got["status"] != status || got["encounter_id"] == nil {
			t.Fatalf("%s: a note must link without changing the status: %v", status, got)
		}
	}
}

// Deleting an appointment gives back the session it took from a package.
func TestDeleteAppointmentReleasesPackage(t *testing.T) {
	e := setup(t)
	admin, recep := e.login("admin_a"), e.login("recep_a")
	pid := sub(recep.expect(201, "POST", "/api/patients/quick", map[string]any{"names": "Luis", "last_names": "Gómez"}), "patient")["id"].(string)
	id := sub(recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"patient_id": pid, "professional_id": e.userID("doc_a")})), "appointment")["id"].(string)
	var pkg string
	if err := e.pool.QueryRow(t.Context(), `INSERT INTO patient_packages (clinic_id, patient_id, name, sessions_total, sessions_used) VALUES ($1, $2, 'Terapia ×5', 5, 1) RETURNING id::text`, e.clinicA, pid).Scan(&pkg); err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE appointments SET package_id = $2 WHERE id = $1`, id, pkg)
	admin.expect(204, "DELETE", "/api/appointments/"+id, nil)
	var used int
	_ = e.pool.QueryRow(t.Context(), `SELECT sessions_used FROM patient_packages WHERE id = $1`, pkg).Scan(&used)
	if used != 0 {
		t.Fatalf("the session of a deleted appointment must go back to the package: used=%d", used)
	}
	admin.expect(404, "DELETE", "/api/appointments/"+id, nil)
}
