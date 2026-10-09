package api_test

import (
	"context"
	"testing"
)

// The same person (e-mail, name, surname) is one record in every giro, and two records already made are joined into the more complete one.
func TestSamePersonIsOneRecord(t *testing.T) {
	e := setup(t)
	e.exec(`UPDATE clinics SET kind = 'GENERAL_MEDICAL', specialties = '{NUTRITION}' WHERE id = $1`, e.clinicA)
	doc := e.login("doc_a")
	mk := func(extra map[string]any) map[string]any { return person(extra) }
	first := doc.expect(201, "POST", "/api/patients/", mk(map[string]any{"names": "Marta", "last_names": "Ríos", "email": "marta@correo.mx"}))
	pid := sub(first, "patient")["id"].(string)
	// registering her again (as the other giro would) does not duplicate her
	again := doc.expect(200, "POST", "/api/patients/", mk(map[string]any{"names": "marta", "last_names": "RÍOS", "email": "MARTA@correo.mx", "phone": "5512345678"}))
	if again["reused"] != true || sub(again, "patient")["id"] != pid || sub(again, "patient")["phone"] != "5512345678" {
		t.Fatalf("reuse: %v", again)
	}

	// two records that already exist: the more complete one stays and receives the history of the other
	e.exec(`INSERT INTO patients (clinic_id, file_number, subject, names, last_names, email, kinds) VALUES ($1, 900, 'person', 'Luis', 'Soto', 'luis@correo.mx', '{NUTRITION}')`, e.clinicA)
	e.exec(`INSERT INTO patients (clinic_id, file_number, subject, names, last_names, email, phone, address, kinds) VALUES ($1, 901, 'person', 'Luis', 'Soto', 'LUIS@correo.mx', '55', 'Calle 1', '{GENERAL_MEDICAL}')`, e.clinicA)
	var thin string
	if err := e.pool.QueryRow(context.Background(), `SELECT id::text FROM patients WHERE file_number = 900 AND clinic_id = $1`, e.clinicA).Scan(&thin); err != nil {
		t.Fatal(err)
	}
	e.exec(`INSERT INTO encounters (clinic_id, patient_id, reason, author_name) VALUES ($1, $2, 'Plan', 'Nutri')`, e.clinicA, thin)
	e.exec(`INSERT INTO patient_charts (clinic_id, patient_id, kind, data, created_by_name) VALUES ($1, $2, 'problems', '{}', 'Nutri')`, e.clinicA, thin)
	out := e.login("admin_a").expect(200, "POST", "/api/patients/merge-duplicates", nil)
	if out["merged"].(float64) != 1 {
		t.Fatalf("merged: %v", out)
	}
	var left, enc, charts int
	var phone string
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM patients WHERE email ILIKE 'luis@correo.mx' AND clinic_id = $1`, e.clinicA).Scan(&left)
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*), max(p.phone) FROM encounters x JOIN patients p ON p.id = x.patient_id WHERE p.email ILIKE 'luis@correo.mx' AND p.clinic_id = $1`, e.clinicA).Scan(&enc, &phone)
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM patient_charts c JOIN patients p ON p.id = c.patient_id WHERE p.email ILIKE 'luis@correo.mx' AND p.clinic_id = $1`, e.clinicA).Scan(&charts)
	if left != 1 || enc != 1 || charts != 1 || phone != "55" {
		t.Fatalf("after merge: patients=%d encounters=%d charts=%d phone=%q", left, enc, charts, phone)
	}
}
