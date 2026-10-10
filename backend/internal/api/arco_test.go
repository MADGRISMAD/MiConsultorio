package api_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func arcoForm(extra map[string]any) map[string]any {
	m := map[string]any{
		"kind": "acceso", "requester_name": "Laura Pérez Gómez", "requester_email": "laura@example.com", "requester_phone": "5512345678",
		"description": "Quiero una copia de mi expediente clínico completo.", "acknowledged": true,
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

// arcoStaff is the body staff send (no acknowledgement checkbox).
func arcoStaff(extra map[string]any) map[string]any {
	m := arcoForm(extra)
	delete(m, "acknowledged")
	return m
}

func arcoCode(e *env, clinic string) string {
	var c string
	if err := e.pool.QueryRow(context.Background(), `SELECT arco_code FROM clinics WHERE id = $1`, clinic).Scan(&c); err != nil {
		e.t.Fatal(err)
	}
	return c
}

func TestArcoPublicToStaffFlow(t *testing.T) {
	e := setup(t)
	codeA := arcoCode(e, e.clinicA)
	anon := e.anon()
	ctx := context.Background()

	// The public page answers with the clinic name only.
	info := anon.expect(200, "GET", "/api/public/arco/"+codeA, nil)
	if sub(info, "clinic")["name"] != "Clinica a" {
		t.Fatalf("info: %v", info)
	}
	anon.expect(404, "GET", "/api/public/arco/nope-nope", nil)

	// The booking slug works too.
	e.exec(`INSERT INTO agenda_settings (clinic_id, booking_slug) VALUES ($1, 'mi-clinica')`, e.clinicA)
	anon.expect(200, "GET", "/api/public/arco/Mi-Clinica", nil)

	out := anon.expect(201, "POST", "/api/public/arco/"+codeA, arcoForm(nil))
	folio, token := out["folio"].(string), out["token"].(string)
	if !strings.HasPrefix(folio, "ARCO-") || len(token) != 43 {
		t.Fatalf("submit: %v", out)
	}
	ack := e.mail.wait(t, 1)
	if ack.To[0] != "laura@example.com" || !strings.Contains(ack.Text, folio) || !strings.Contains(ack.Text, token) {
		t.Fatalf("ack mail: %+v", ack)
	}

	// Public status: only the minimum.
	st := anon.expect(200, "GET", "/api/public/arco/status/"+token, nil)
	rawSt, _ := json.Marshal(st)
	for _, secret := range []string{"Laura", "laura@example.com", "5512345678", "expediente", "response", "description", "requester", "note"} {
		if strings.Contains(string(rawSt), secret) {
			t.Fatalf("public status leaks %q: %s", secret, rawSt)
		}
	}
	if sub(st, "request")["folio"] != folio || sub(st, "request")["status"] != "recibida" {
		t.Fatalf("status: %v", st)
	}
	anon.expect(404, "GET", "/api/public/arco/status/"+strings.Repeat("a", 43), nil)

	// Staff: the request shows up with its deadline, and the audit log has a line.
	admin := e.login("admin_a")
	list := admin.expect(200, "GET", "/api/arco/", nil)
	reqs := list["requests"].([]any)
	if len(reqs) != 1 || sub(list, "summary")["open"].(float64) != 1 {
		t.Fatalf("list: %v", list)
	}
	r0 := reqs[0].(map[string]any)
	id := r0["id"].(string)
	if r0["created_via"] != "public" || r0["deadline"] != "answer" || r0["days_left"].(float64) < 18 || r0["deadline_state"] != "ok" {
		t.Fatalf("request: %v", r0)
	}
	var n int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM activity_log WHERE clinic_id = $1 AND type = 'arco_received'`, e.clinicA).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit: %d %v", n, err)
	}

	// Nothing can be answered before the identity is verified.
	admin.expect(409, "POST", "/api/arco/"+id+"/respond", map[string]any{"outcome": "procedente", "response_text": "Le entregaremos su copia."})
	admin.expect(409, "PATCH", "/api/arco/"+id, map[string]any{"identity_verified": true}) // needs the method
	admin.expect(200, "PATCH", "/api/arco/"+id, map[string]any{"identity_verified": true, "identity_method": "INE mostrada en mostrador"})
	admin.expect(200, "POST", "/api/arco/"+id+"/status", map[string]any{"status": "en_revision"})
	admin.expect(400, "POST", "/api/arco/"+id+"/status", map[string]any{"status": "requiere_info"}) // needs a note
	admin.expect(200, "POST", "/api/arco/"+id+"/notes", map[string]any{"note": "Se llamó para confirmar."})
	admin.expect(400, "POST", "/api/arco/"+id+"/respond", map[string]any{"outcome": "improcedente", "response_text": "No es posible atenderla."}) // reason missing

	res := sub(admin.expect(200, "POST", "/api/arco/"+id+"/respond", map[string]any{"outcome": "procedente", "response_text": "Le entregaremos su copia.", "send_email": true}), "request")
	if res["status"] != "atendida" || res["pending_execute"] != true || res["deadline"] != "execute" || res["due_execute_at"] == nil {
		t.Fatalf("after respond: %v", res)
	}
	e.mail.wait(t, 2) // the acknowledgement, then the response
	admin.expect(409, "POST", "/api/arco/"+id+"/respond", map[string]any{"outcome": "procedente", "response_text": "Otra vez la misma."})
	done := sub(admin.expect(200, "POST", "/api/arco/"+id+"/execute", nil), "request")
	if done["pending_execute"] != false || done["deadline"] != "" {
		t.Fatalf("after execute: %v", done)
	}
	admin.expect(409, "POST", "/api/arco/"+id+"/execute", nil)

	detail := admin.expect(200, "GET", "/api/arco/"+id, nil)
	if n := len(detail["events"].([]any)); n < 6 {
		t.Fatalf("history has %d lines", n)
	}
	if _, err := e.pool.Exec(ctx, `UPDATE arco_events SET message = 'x'`); err == nil {
		t.Fatal("arco_events must be append-only")
	}
	if got := sub(anon.expect(200, "GET", "/api/public/arco/status/"+token, nil), "request")["answered"]; got != true {
		t.Fatalf("answered: %v", got)
	}
}

func TestArcoPermissionsAndIsolation(t *testing.T) {
	e := setup(t)
	codeA := arcoCode(e, e.clinicA)
	e.anon().expect(201, "POST", "/api/public/arco/"+codeA, arcoForm(nil))
	idA := e.login("admin_a").expect(200, "GET", "/api/arco/", nil)["requests"].([]any)[0].(map[string]any)["id"].(string)

	for _, u := range []string{"doc_a", "recep_a", "cash_a"} {
		c := e.login(u)
		c.expect(403, "GET", "/api/arco/", nil)
		c.expect(403, "GET", "/api/arco/"+idA, nil)
		c.expect(403, "POST", "/api/arco/", arcoStaff(nil))
	}
	e.anon().expect(401, "GET", "/api/arco/", nil)

	// Another clinic never sees nor touches it.
	b := e.login("admin_b")
	if n := len(b.expect(200, "GET", "/api/arco/", nil)["requests"].([]any)); n != 0 {
		t.Fatalf("clinic B sees %d requests", n)
	}
	b.expect(404, "GET", "/api/arco/"+idA, nil)
	b.expect(404, "PATCH", "/api/arco/"+idA, map[string]any{"handled_by_name": "x"})
	b.expect(404, "POST", "/api/arco/"+idA+"/notes", map[string]any{"note": "hola"})
	b.expect(404, "POST", "/api/arco/"+idA+"/respond", map[string]any{"outcome": "improcedente", "response_text": "xxxxxxxxxxxx", "denial_reason": "yyyyyyyyyyyy"})
	b.expect(404, "GET", "/api/arco/"+idA+"/package", nil)
	b.expect(404, "POST", "/api/arco/"+idA+"/archive-patient", nil)

	// Linking a patient of another clinic is refused.
	pb := sub(e.login("doc_b").expect(201, "POST", "/api/patients/", person(nil)), "patient")["id"].(string)
	a := e.login("admin_a")
	a.expect(409, "PATCH", "/api/arco/"+idA, map[string]any{"patient_id": pb})
	a.expect(400, "POST", "/api/arco/", arcoStaff(map[string]any{"patient_id": pb}))
}

func TestArcoCancellationKeepsRecord(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	pid := sub(e.login("doc_a").expect(201, "POST", "/api/patients/", person(nil)), "patient")["id"].(string)
	admin := e.login("admin_a")
	res := admin.expect(201, "POST", "/api/arco/", arcoStaff(map[string]any{"kind": "cancelacion", "patient_id": pid, "received_on": time.Now().AddDate(0, 0, -30).Format("2006-01-02")}))
	r := sub(res, "request")
	id := r["id"].(string)
	if r["deadline_state"] != "overdue" || r["created_via"] != "staff" {
		t.Fatalf("a request received 30 days ago is overdue: %v", r)
	}
	if sum := sub(admin.expect(200, "GET", "/api/arco/summary", nil), "summary"); sum["overdue"].(float64) != 1 {
		t.Fatalf("summary: %v", sum)
	}
	if n := len(admin.expect(200, "GET", "/api/arco/?status=vencida", nil)["requests"].([]any)); n != 1 {
		t.Fatalf("vencida filter: %d", n)
	}

	pkg := sub(admin.expect(200, "GET", "/api/arco/"+id+"/package", nil), "package")
	if !strings.Contains(pkg["retention_note"].(string), "NOM-004") || pkg["export_path"] != nil {
		t.Fatalf("package: %v", pkg)
	}
	admin.expect(409, "POST", "/api/arco/"+id+"/archive-patient", nil) // identity first
	admin.expect(200, "PATCH", "/api/arco/"+id, map[string]any{"identity_verified": true, "identity_method": "INE"})
	admin.expect(200, "POST", "/api/arco/"+id+"/archive-patient", nil)
	admin.expect(409, "POST", "/api/arco/"+id+"/archive-patient", nil)

	var archived bool
	var rows int
	if err := e.pool.QueryRow(ctx, `SELECT archived_at IS NOT NULL FROM patients WHERE id = $1`, pid).Scan(&archived); err != nil || !archived {
		t.Fatalf("patient archived: %v %v", archived, err)
	}
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM patients WHERE id = $1`, pid).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("the patient row must remain: %d %v", rows, err)
	}

	// An access request links the existing export.
	acc := sub(admin.expect(201, "POST", "/api/arco/", arcoStaff(map[string]any{"patient_id": pid})), "request")["id"].(string)
	// the patient's own list (their profile): only theirs
	if mine := admin.expect(200, "GET", "/api/arco/?patient="+pid+"&status=", nil)["requests"].([]any); len(mine) < 2 {
		t.Fatalf("patient filter: %d", len(mine))
	}
	if none := admin.expect(200, "GET", "/api/arco/?patient=00000000-0000-0000-0000-000000000000", nil)["requests"].([]any); len(none) != 0 {
		t.Fatalf("another patient: %d", len(none))
	}
	pk := sub(admin.expect(200, "GET", "/api/arco/"+acc+"/package", nil), "package")
	if pk["export_path"] != "/patients/"+pid+"/export" {
		t.Fatalf("export link: %v", pk)
	}
	admin.expect(200, "PATCH", "/api/arco/"+acc, map[string]any{"identity_verified": true, "identity_method": "INE"})
	den := sub(admin.expect(200, "POST", "/api/arco/"+acc+"/respond", map[string]any{"outcome": "improcedente", "response_text": "No es posible atenderla.", "denial_reason": "No se acreditó la identidad."}), "request")
	if den["status"] != "negada" || den["open"] != false {
		t.Fatalf("denial: %v", den)
	}
}

func TestArcoPublicValidationHoneypotAndRateLimit(t *testing.T) {
	e := setup(t)
	code := arcoCode(e, e.clinicA)
	anon := e.anon()
	path := "/api/public/arco/" + code
	// Honeypot: looks like a success, stores nothing, sends no mail.
	anon.expect(201, "POST", path, arcoForm(map[string]any{"website": "http://spam.example"}))
	anon.expect(400, "POST", path, arcoForm(map[string]any{"kind": "borrar"}))
	anon.expect(400, "POST", path, arcoForm(map[string]any{"requester_name": "A"}))
	anon.expect(400, "POST", path, arcoForm(map[string]any{"requester_email": "no-es-correo"}))
	anon.expect(400, "POST", path, arcoForm(map[string]any{"acknowledged": false}))
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM arco_requests`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("stored %d requests", n)
	}
	if e.mail.count() != 0 {
		t.Fatal("honeypot sent mail")
	}
	// Strict limit per IP: 5 attempts per hour, and 5 were already made.
	anon.expect(429, "POST", path, arcoForm(nil))
}

func TestArcoMailIsOptional(t *testing.T) {
	e := setup(t)
	e.mail.enabled = false
	code := arcoCode(e, e.clinicA)
	e.anon().expect(201, "POST", "/api/public/arco/"+code, arcoForm(nil))
	time.Sleep(100 * time.Millisecond)
	if e.mail.count() != 0 {
		t.Fatal("mail sent while SMTP is disabled")
	}
}

func TestArcoCompliance(t *testing.T) {
	e := setup(t)
	admin := e.login("admin_a")
	find := func() map[string]any {
		for _, it := range admin.expect(200, "GET", "/api/clinic/compliance", nil)["items"].([]any) {
			if it.(map[string]any)["key"] == "arco_requests" {
				return it.(map[string]any)
			}
		}
		t.Fatal("compliance has no arco_requests item")
		return nil
	}
	if find()["status"] != "ok" {
		t.Fatalf("no requests: %v", find())
	}
	admin.expect(201, "POST", "/api/arco/", arcoStaff(nil))
	if find()["status"] != "info" {
		t.Fatalf("open: %v", find())
	}
	admin.expect(201, "POST", "/api/arco/", arcoStaff(map[string]any{"received_on": time.Now().AddDate(0, 0, -40).Format("2006-01-02")}))
	if it := find(); it["status"] != "todo" || !strings.Contains(it["detail"].(string), "asesor legal") {
		t.Fatalf("overdue: %v", it)
	}
}
