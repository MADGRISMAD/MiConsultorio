package api_test

import (
	"testing"
)

func TestEveryClinicStartsWithConsulta(t *testing.T) {
	e := setup(t)
	for _, c := range []string{e.clinicA, e.clinicB} {
		var n int
		if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM catalog_items WHERE clinic_id=$1 AND system_key='consulta' AND kind='service' AND active`, c).Scan(&n); err != nil || n != 1 {
			t.Fatalf("clinic %s must have exactly one default Consulta: n=%d err=%v", c, n, err)
		}
	}
	// a clinic created later gets it too (trigger)
	var created string
	if err := e.pool.QueryRow(t.Context(), `INSERT INTO clinics (name, email, kind) VALUES ('Nueva', 'nueva@mail.mx', 'VETERINARY') RETURNING id::text`).Scan(&created); err != nil {
		t.Fatal(err)
	}
	var price int
	if err := e.pool.QueryRow(t.Context(), `SELECT price_cents FROM catalog_items WHERE clinic_id=$1 AND system_key='consulta'`, created).Scan(&price); err != nil || price != 50000 {
		t.Fatalf("a new clinic starts with Consulta: price=%d err=%v", price, err)
	}
	// it can be re-priced but not deleted
	admin := e.login("admin_a")
	var id string
	for _, it := range admin.expect(200, "GET", "/api/pos/items?q=consulta", nil)["items"].([]any) {
		if m := it.(map[string]any); m["system_key"] == "consulta" {
			id = m["id"].(string)
		}
	}
	if id == "" {
		t.Fatal("the default Consulta must be listed")
	}
	if code, o := admin.do("DELETE", "/api/pos/items/"+id, nil); code != 409 || o["code"] != "SYSTEM_ITEM" {
		t.Fatalf("deleting the default Consulta: %d %v", code, o)
	}
}

func TestFinishedAppointmentGoesToTheRegister(t *testing.T) {
	e := setup(t)
	e.exec(`UPDATE clinics SET plan = 'crecimiento' WHERE id = $1`, e.clinicA)
	e.exec(`UPDATE clinics SET plan = 'basico' WHERE id = $1`, e.clinicB)
	recep, doc, cash := e.login("recep_a"), e.login("doc_a"), e.login("cash_a")
	docID := e.userID("doc_a")
	pat := sub(recep.expect(201, "POST", "/api/patients/quick", map[string]any{"names": "Ana", "last_names": "Pérez"}), "patient")["id"].(string)

	finish := func(c *client, id string) map[string]any {
		restore := at(t, 8, 30)
		defer restore()
		c.expect(200, "POST", "/api/appointments/"+id+"/status", map[string]any{"status": "arrived"})
		c.expect(200, "POST", "/api/appointments/"+id+"/status", map[string]any{"status": "in_progress"})
		restore()
		restore = at(t, 9, 40)
		return c.expect(200, "POST", "/api/appointments/"+id+"/status", map[string]any{"status": "completed"})
	}

	// no service chosen: it goes as the clinic's "Consulta"
	a1 := sub(recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"patient_id": pat, "professional_id": docID})), "appointment")["id"].(string)
	out := finish(doc, a1)
	ch, _ := out["charge"].(map[string]any)
	if ch == nil || num(ch, "total_cents") != 50000 {
		t.Fatalf("finishing must queue the consultation at the register: %v", out)
	}
	queue := cash.expect(200, "GET", "/api/consult-charges?status=sent", nil)["charges"].([]any)
	if len(queue) != 1 {
		t.Fatalf("the register must see it: %v", queue)
	}
	c0 := queue[0].(map[string]any)
	if c0["appointment_id"] != a1 || c0["status"] != "sent" {
		t.Fatalf("queued charge: %v", c0)
	}
	detail := sub(cash.expect(200, "GET", "/api/consult-charges/"+c0["id"].(string), nil), "charge")
	if items, _ := detail["items"].([]any); len(items) != 1 || items[0].(map[string]any)["name"] != "Consulta" {
		t.Fatalf("it carries the Consulta service: %v", detail)
	}

	// an appointment with its own service is charged with that one
	var svc string
	e.exec(`UPDATE catalog_items SET price_cents = 80000 WHERE id = (SELECT id FROM catalog_items WHERE clinic_id = $1 AND system_key = 'consulta')`, e.clinicA)
	if err := e.pool.QueryRow(t.Context(), `INSERT INTO catalog_items (clinic_id, kind, name, price_cents) VALUES ($1, 'service', 'Limpieza', 65000) RETURNING id::text`, e.clinicA).Scan(&svc); err != nil {
		t.Fatal(err)
	}
	a2 := sub(recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"patient_id": pat, "professional_id": docID, "service_id": svc, "startHour": "11:00", "endHour": "11:30"})), "appointment")["id"].(string)
	out = func() map[string]any {
		restore := at(t, 10, 30)
		recep.expect(200, "POST", "/api/appointments/"+a2+"/status", map[string]any{"status": "arrived"})
		recep.expect(200, "POST", "/api/appointments/"+a2+"/status", map[string]any{"status": "in_progress"})
		restore()
		defer at(t, 11, 40)()
		return doc.expect(200, "POST", "/api/appointments/"+a2+"/status", map[string]any{"status": "completed"})
	}()
	if ch, _ := out["charge"].(map[string]any); ch == nil || num(ch, "total_cents") != 65000 {
		t.Fatalf("the appointment's own service is the one charged: %v", out)
	}

	// someone without a registered patient cannot be queued (the register can still charge from the appointment)
	a3 := sub(recep.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"professional_id": docID, "startHour": "12:00", "endHour": "12:30"})), "appointment")["id"].(string)
	restore := at(t, 11, 30)
	recep.expect(200, "POST", "/api/appointments/"+a3+"/status", map[string]any{"status": "arrived"})
	recep.expect(200, "POST", "/api/appointments/"+a3+"/status", map[string]any{"status": "in_progress"})
	restore()
	defer at(t, 12, 40)()
	if out := doc.expect(200, "POST", "/api/appointments/"+a3+"/status", map[string]any{"status": "completed"}); out["charge"] != nil {
		t.Fatalf("no patient, no pre-account: %v", out)
	}

	// without a payments plan nothing is queued
	rb := e.login("recep_b")
	pb := sub(rb.expect(201, "POST", "/api/patients/quick", map[string]any{"names": "Eva", "last_names": "Ruiz"}), "patient")["id"].(string)
	b1 := sub(rb.expect(201, "POST", "/api/appointments", apptBody(map[string]any{"patient_id": pb, "startHour": "13:00", "endHour": "13:30"})), "appointment")["id"].(string)
	restore2 := at(t, 12, 45)
	rb.expect(200, "POST", "/api/appointments/"+b1+"/status", map[string]any{"status": "arrived"})
	rb.expect(200, "POST", "/api/appointments/"+b1+"/status", map[string]any{"status": "in_progress"})
	restore2()
	defer at(t, 13, 40)()
	if out := rb.expect(200, "POST", "/api/appointments/"+b1+"/status", map[string]any{"status": "completed"}); out["charge"] != nil {
		t.Fatalf("a clinic without cobros must not get a pre-account: %v", out)
	}
	var n int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM encounter_charges WHERE clinic_id = $1`, e.clinicB).Scan(&n); err != nil || n != 0 {
		t.Fatalf("clinic without cobros: charges=%d err=%v", n, err)
	}
}
