package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

var portalCodeRe = regexp.MustCompile(`es (\d{6})\.`)

// portalEnv is clinic A with the portal on at /portal/clinica-a and a patient (a dog) whose owner is dueno@mail.mx;
// clinic B has the same owner address on another patient.
type portalEnv struct {
	*env
	patA, patB, patOther string
}

func newPortalEnv(t *testing.T) *portalEnv {
	e := setup(t)
	p := &portalEnv{env: e}
	e.exec(`INSERT INTO agenda_settings (clinic_id, booking_slug, portal_enabled, portal_welcome) VALUES ($1, 'clinica-a', true, 'Hola')`, e.clinicA)
	e.exec(`INSERT INTO agenda_settings (clinic_id, booking_slug, portal_enabled) VALUES ($1, 'clinica-b', true)`, e.clinicB)
	p.patA = p.patient(e.clinicA, 1, "Firulais", "perro", "animal", "", "Dueno@Mail.mx")
	p.patOther = p.patient(e.clinicA, 2, "Otra", "", "person", "otra@mail.mx", "")
	p.patB = p.patient(e.clinicB, 1, "Ajeno", "", "person", "dueno@mail.mx", "")
	return p
}

func (p *portalEnv) patient(clinic string, n int, names, species, subject, email, guardian string) string {
	var id string
	err := p.pool.QueryRow(ctx(), `INSERT INTO patients (clinic_id, file_number, subject, names, email, guardian_email, profile)
		VALUES ($1,$2,$3,$4,$5,$6, jsonb_build_object('species', $7::text)) RETURNING id::text`, clinic, n, subject, names, email, guardian, species).Scan(&id)
	if err != nil {
		p.t.Fatal(err)
	}
	return id
}

// signIn requests a code and exchanges it, returning a client holding the portal cookie.
func (p *portalEnv) signIn(slug, email string) *client {
	p.t.Helper()
	c := p.anon()
	before := p.mail.count()
	c.expect(200, "POST", "/api/portal/"+slug+"/code", map[string]any{"email": email})
	m := p.mail.wait(p.t, before+1)
	code := portalCodeRe.FindStringSubmatch(m.Text)
	if code == nil {
		p.t.Fatalf("no code in %q", m.Text)
	}
	c.expect(200, "POST", "/api/portal/"+slug+"/login", map[string]any{"email": email, "code": code[1]})
	return c
}

func ctx() context.Context { return context.Background() }

func mustURL(s string) *url.URL { u, _ := url.Parse(s); return u }

func raw(m map[string]any) string { b, _ := json.Marshal(m); return string(b) }

func TestPortalCodeFlow(t *testing.T) {
	p := newPortalEnv(t)
	anon := p.anon()

	// unknown slug and bad e-mail
	anon.expect(404, "POST", "/api/portal/nope/code", map[string]any{"email": "dueno@mail.mx"})
	anon.expect(400, "POST", "/api/portal/clinica-a/code", map[string]any{"email": "no-es-correo"})

	// no enumeration: same answer for a known and an unknown address, and only the known one gets mail
	known := raw(anon.expect(200, "POST", "/api/portal/clinica-a/code", map[string]any{"email": " DUENO@mail.mx "}))
	unknown := raw(anon.expect(200, "POST", "/api/portal/clinica-a/code", map[string]any{"email": "nadie@mail.mx"}))
	if known != unknown {
		t.Fatalf("answers differ: %s vs %s", known, unknown)
	}
	m := p.mail.wait(t, 1)
	time.Sleep(100 * time.Millisecond)
	if p.mail.count() != 1 || m.To[0] != "dueno@mail.mx" {
		t.Fatalf("mail: %d %v", p.mail.count(), m.To)
	}
	code := portalCodeRe.FindStringSubmatch(m.Text)[1]
	var stored string
	p.pool.QueryRow(ctx(), `SELECT encode(code_hash, 'hex') FROM portal_otps LIMIT 1`).Scan(&stored)
	if strings.Contains(stored, code) && len(stored) < 10 {
		t.Fatal("code stored in clear")
	}

	// wrong code, then the right one works once
	wrong := fmt.Sprintf("%06d", (atoi(code)+1)%1000000)
	anon.expect(401, "POST", "/api/portal/clinica-a/login", map[string]any{"email": "dueno@mail.mx", "code": wrong})
	anon.expect(401, "POST", "/api/portal/clinica-a/login", map[string]any{"email": "nadie@mail.mx", "code": code})
	anon.expect(401, "POST", "/api/portal/clinica-b/login", map[string]any{"email": "dueno@mail.mx", "code": code}) // other clinic
	anon.expect(200, "POST", "/api/portal/clinica-a/login", map[string]any{"email": "dueno@mail.mx", "code": code})
	anon.expect(401, "POST", "/api/portal/clinica-a/login", map[string]any{"email": "dueno@mail.mx", "code": code}) // single use

	// failed attempts are in the activity log, never with the code
	var n int
	p.pool.QueryRow(ctx(), `SELECT count(*) FROM activity_log WHERE type = 'portal_login_failed' AND clinic_id = $1 AND actor_name = 'Portal (dueno@mail.mx)'`, p.clinicA).Scan(&n)
	if n < 2 {
		t.Fatalf("failed attempts logged: %d", n)
	}
	var leaked int
	p.pool.QueryRow(ctx(), `SELECT count(*) FROM activity_log WHERE meta::text LIKE '%' || $1 || '%' OR message LIKE '%' || $1 || '%'`, code).Scan(&leaked)
	if leaked != 0 {
		t.Fatal("code leaked to the activity log")
	}
}

func TestPortalOTPAttemptsAndExpiry(t *testing.T) {
	p := newPortalEnv(t)
	anon := p.anon()
	anon.expect(200, "POST", "/api/portal/clinica-a/code", map[string]any{"email": "dueno@mail.mx"})
	code := portalCodeRe.FindStringSubmatch(p.mail.wait(t, 1).Text)[1]
	wrong := fmt.Sprintf("%06d", (atoi(code)+1)%1000000)
	for i := 0; i < 5; i++ {
		anon.expect(401, "POST", "/api/portal/clinica-a/login", map[string]any{"email": "dueno@mail.mx", "code": wrong})
	}
	// five tries burned the code: even the right one is refused now
	anon.expect(401, "POST", "/api/portal/clinica-a/login", map[string]any{"email": "dueno@mail.mx", "code": code})

	// a code past its 10 minutes is refused
	anon.expect(200, "POST", "/api/portal/clinica-a/code", map[string]any{"email": "dueno@mail.mx"})
	code = portalCodeRe.FindStringSubmatch(p.mail.wait(t, 2).Text)[1]
	p.exec(`UPDATE portal_otps SET expires_at = now() - interval '1 minute' WHERE used_at IS NULL`)
	anon.expect(401, "POST", "/api/portal/clinica-a/login", map[string]any{"email": "dueno@mail.mx", "code": code})

	// a newer code replaces the older one
	anon.expect(200, "POST", "/api/portal/clinica-a/code", map[string]any{"email": "dueno@mail.mx"})
	first := portalCodeRe.FindStringSubmatch(p.mail.wait(t, 3).Text)[1]
	anon.expect(200, "POST", "/api/portal/clinica-a/code", map[string]any{"email": "dueno@mail.mx"})
	second := portalCodeRe.FindStringSubmatch(p.mail.wait(t, 4).Text)[1]
	if first != second {
		anon.expect(401, "POST", "/api/portal/clinica-a/login", map[string]any{"email": "dueno@mail.mx", "code": first})
	}
	anon.expect(200, "POST", "/api/portal/clinica-a/login", map[string]any{"email": "dueno@mail.mx", "code": second})
}

func atoi(s string) int { var n int; fmt.Sscanf(s, "%d", &n); return n }

func TestPortalRateLimit(t *testing.T) {
	p := newPortalEnv(t)
	anon := p.anon()
	for i := 0; i < 5; i++ {
		anon.expect(200, "POST", "/api/portal/clinica-a/code", map[string]any{"email": "nadie@mail.mx"})
	}
	anon.expect(429, "POST", "/api/portal/clinica-a/code", map[string]any{"email": "nadie@mail.mx"})
	// per-IP budget: 12 requests an hour whatever the address
	p2 := newPortalEnv(t)
	for i := 0; i < 12; i++ {
		p2.anon().expect(200, "POST", "/api/portal/clinica-a/code", map[string]any{"email": fmt.Sprintf("u%d@mail.mx", i)})
	}
	p2.anon().expect(429, "POST", "/api/portal/clinica-a/code", map[string]any{"email": "otro@mail.mx"})
}

func TestPortalDisabled(t *testing.T) {
	p := newPortalEnv(t)
	c := p.signIn("clinica-a", "dueno@mail.mx")
	c.expect(200, "GET", "/api/portal/me", nil)
	p.exec(`UPDATE agenda_settings SET portal_enabled = false WHERE clinic_id = $1`, p.clinicA)
	for _, path := range []string{"/api/portal/me", "/api/portal/appointments", "/api/portal/prescriptions", "/api/portal/vaccinations"} {
		c.expect(404, "GET", path, nil)
	}
	c.expect(404, "POST", "/api/portal/appointments/"+p.patA+"/cancel", nil)
	p.anon().expect(404, "POST", "/api/portal/clinica-a/code", map[string]any{"email": "dueno@mail.mx"})
	p.anon().expect(404, "POST", "/api/portal/clinica-a/login", map[string]any{"email": "dueno@mail.mx", "code": "123456"})
	p.anon().expect(404, "GET", "/api/portal/clinica-a/info", nil)
	// the other clinic is unaffected
	p.anon().expect(200, "GET", "/api/portal/clinica-b/info", nil)
}

func TestPortalSessionSeparation(t *testing.T) {
	p := newPortalEnv(t)
	portal := p.signIn("clinica-a", "dueno@mail.mx")
	// a portal cookie opens nothing of the staff API
	for _, path := range []string{"/api/session", "/api/patients/", "/api/appointments", "/api/clinic", "/api/team"} {
		portal.expect(401, "GET", path, nil)
	}
	// and a staff session opens nothing of the portal
	staff := p.login("admin_a")
	for _, path := range []string{"/api/portal/me", "/api/portal/appointments", "/api/portal/prescriptions", "/api/portal/vaccinations"} {
		staff.expect(401, "GET", path, nil)
	}
	p.anon().expect(401, "GET", "/api/portal/me", nil)
	// the staff JWT value placed in the portal cookie is not accepted either
	var staffTok string
	for _, ck := range staff.c.Jar.Cookies(mustURL(p.srv.URL)) {
		if ck.Name == "caresia_session" {
			staffTok = ck.Value
		}
	}
	req, _ := http.NewRequest("GET", p.srv.URL+"/api/portal/me", nil)
	req.AddCookie(&http.Cookie{Name: "caresia_portal", Value: staffTok})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 401 {
		t.Fatalf("staff token as portal cookie: %d", res.StatusCode)
	}
	// logout ends the portal session
	portal.expect(204, "POST", "/api/portal/logout", nil)
	portal.expect(401, "GET", "/api/portal/me", nil)
}

func TestPortalIsolation(t *testing.T) {
	p := newPortalEnv(t)
	c := p.signIn("clinica-a", "dueno@mail.mx")
	me := c.expect(200, "GET", "/api/portal/me", nil)
	pts := me["patients"].([]any)
	if len(pts) != 1 || pts[0].(map[string]any)["id"] != p.patA || pts[0].(map[string]any)["species"] != "perro" {
		t.Fatalf("patients: %v", pts)
	}
	if strings.Contains(raw(me), "Ajeno") || strings.Contains(raw(me), "Otra") {
		t.Fatalf("leak: %v", me)
	}

	// appointments of three kinds: linked, someone else's, unlinked with this e-mail, unlinked with another
	day := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	ins := func(clinic, patient, names, email string) string {
		var id string
		var pid any
		if patient != "" {
			pid = patient
		}
		if err := p.pool.QueryRow(ctx(), `INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, patient_id, email)
			VALUES ($1,'',$2,'X',$3,'10:00','10:30',$4::uuid,$5) RETURNING id::text`, clinic, names, day, pid, email).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	mine := ins(p.clinicA, p.patA, "Firulais", "")
	ins(p.clinicA, p.patOther, "Otra", "dueno@mail.mx") // linked to another patient: the e-mail on it does not matter
	unlinked := ins(p.clinicA, "", "Don Dueno", "DUENO@mail.mx")
	ins(p.clinicA, "", "Extraño", "otro@mail.mx")
	ins(p.clinicB, p.patB, "Ajeno", "dueno@mail.mx")

	out := c.expect(200, "GET", "/api/portal/appointments", nil)
	up := out["upcoming"].([]any)
	ids := map[string]bool{}
	for _, a := range up {
		ids[a.(map[string]any)["id"].(string)] = true
	}
	if len(up) != 2 || !ids[mine] || !ids[unlinked] {
		t.Fatalf("upcoming: %v", up)
	}
	// cannot cancel what is not yours
	other := ins(p.clinicA, p.patOther, "Otra", "dueno@mail.mx")
	c.expect(404, "POST", "/api/portal/appointments/"+other+"/cancel", nil)
	c.expect(404, "POST", "/api/portal/appointments/not-a-uuid/cancel", nil)

	// another address in the same clinic sees only its own
	o := p.signIn("clinica-a", "otra@mail.mx")
	if n := len(o.expect(200, "GET", "/api/portal/appointments", nil)["upcoming"].([]any)); n != 2 {
		t.Fatalf("other e-mail sees %d (own + linked)", n)
	}
	if strings.Contains(raw(o.expect(200, "GET", "/api/portal/me", nil)), "Firulais") {
		t.Fatal("e-mail A patient visible to e-mail B")
	}
	// same address in clinic B only sees clinic B
	b := p.signIn("clinica-b", "dueno@mail.mx")
	if raw(b.expect(200, "GET", "/api/portal/me", nil)) == "" || strings.Contains(raw(b.expect(200, "GET", "/api/portal/appointments", nil)), "Firulais") {
		t.Fatal("clinic leak")
	}
	// an archived patient disappears from the portal and blocks its code
	p.exec(`UPDATE patients SET archived_at = now() WHERE id = $1`, p.patA)
	if len(c.expect(200, "GET", "/api/portal/me", nil)["patients"].([]any)) != 0 {
		t.Fatal("archived patient still linked")
	}
	n0 := p.mail.count()
	p.anon().expect(200, "POST", "/api/portal/clinica-a/code", map[string]any{"email": "dueno@mail.mx"})
	time.Sleep(150 * time.Millisecond)
	if p.mail.count() != n0 {
		t.Fatal("code sent for an address with only archived patients")
	}
}

func TestPortalCancelMargin(t *testing.T) {
	p := newPortalEnv(t)
	c := p.signIn("clinica-a", "dueno@mail.mx")
	loc, _ := time.LoadLocation("America/Mexico_City")
	appt := func(start time.Time, status string) string {
		var id string
		if err := p.pool.QueryRow(ctx(), `INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, patient_id, status)
			VALUES ($1,'','Firulais','X',$2::date,$3::time,(CASE WHEN $3::time > '23:39' THEN '23:59:59'::time ELSE $3::time + interval '20 minutes' END),$4,$5) RETURNING id::text`,
			p.clinicA, start.In(loc).Format("2006-01-02"), start.In(loc).Format("15:04"), p.patA, status).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	// default margin is 2 h
	soon := appt(time.Now().Add(30*time.Minute), "scheduled")
	c.expect(409, "POST", "/api/portal/appointments/"+soon+"/cancel", nil)
	past := appt(time.Now().Add(-3*time.Hour), "scheduled")
	c.expect(409, "POST", "/api/portal/appointments/"+past+"/cancel", nil)
	done := appt(time.Now().Add(48*time.Hour), "completed")
	c.expect(409, "POST", "/api/portal/appointments/"+done+"/cancel", nil)

	later := appt(time.Now().Add(72*time.Hour), "confirmed")
	list := c.expect(200, "GET", "/api/portal/appointments", nil)["upcoming"].([]any)
	if len(list) != 2 { // soon (cannot cancel) and later
		t.Fatalf("upcoming: %v", list)
	}
	for _, a := range list {
		m := a.(map[string]any)
		if m["id"] == soon && m["can_cancel"] != false || m["id"] == later && m["can_cancel"] != true {
			t.Fatalf("can_cancel: %v", m)
		}
	}
	c.expect(200, "POST", "/api/portal/appointments/"+later+"/cancel", map[string]any{"reason": "Viaje"})
	var status, reason string
	p.pool.QueryRow(ctx(), `SELECT status, cancel_reason FROM appointments WHERE id = $1`, later).Scan(&status, &reason)
	if status != "cancelled" || !strings.Contains(reason, "Viaje") {
		t.Fatalf("after cancel: %s %q", status, reason)
	}
	c.expect(409, "POST", "/api/portal/appointments/"+later+"/cancel", nil) // already cancelled
	hist := c.expect(200, "GET", "/api/portal/appointments", nil)["history"].([]any)
	if len(hist) != 3 {
		t.Fatalf("history: %d", len(hist))
	}

	// a bigger margin closes the window
	p.exec(`UPDATE agenda_settings SET cancel_min_hours = 100 WHERE clinic_id = $1`, p.clinicA)
	again := appt(time.Now().Add(72*time.Hour), "scheduled")
	c.expect(409, "POST", "/api/portal/appointments/"+again+"/cancel", nil)
}

func TestPortalNoClinicalLeak(t *testing.T) {
	p := newPortalEnv(t)
	doc := p.userID("doc_a")
	p.exec(`INSERT INTO encounters (clinic_id, patient_id, reason, subjective, exam, assessment, plan, notes, author_name) VALUES ($1,$2,'MOTIVOSECRETO','SUBJSECRETO','EXAMSECRETO','DIAGSECRETO','PLANSECRETO','NOTASECRETA','Dr')`, p.clinicA, p.patA)
	p.exec(`INSERT INTO encounters (clinic_id, patient_id, notes, private, author_name) VALUES ($1,$2,'PRIVADASECRETA',true,'Dr')`, p.clinicA, p.patA)
	p.exec(`INSERT INTO prescriptions (clinic_id, patient_id, folio, diagnosis, items, instructions, author_id, author_name, author_title, author_license, author_institution, verify_token)
		VALUES ($1,$2,1,'DIAGSECRETO','[{"medicine":"Amoxicilina","dose":"1 tab","route":"Oral","frequency":"8 h","duration":"7 días"}]','Con alimentos',$3,'Dra. Doc','Médico','123','UNAM', md5(random()::text))`, p.clinicA, p.patA, doc)
	p.exec(`INSERT INTO prescriptions (clinic_id, patient_id, folio, diagnosis, items, author_id, author_name, voided_at, voided_by, void_reason, verify_token)
		VALUES ($1,$2,2,'DIAGSECRETO','[]',$3,'Dra. Doc', now(), 'Admin', 'MOTIVOANULACIONSECRETO', md5(random()::text))`, p.clinicA, p.patA, doc)
	p.exec(`INSERT INTO prescriptions (clinic_id, patient_id, folio, items, author_name, verify_token) VALUES ($1,$2,1,'[]','Otra clínica', md5(random()::text))`, p.clinicB, p.patB)
	p.exec(`INSERT INTO vaccinations (clinic_id, patient_id, kind, name, applied_on, next_due, lot, dose, administered_by_name, notes) VALUES ($1,$2,'vaccine','Rabia','2026-01-10','2027-01-10','L1','1 ml','Dra. Doc','NOTAVACUNASECRETA')`, p.clinicA, p.patA)
	p.exec(`INSERT INTO vaccinations (clinic_id, patient_id, kind, name, applied_on, administered_by_name, voided_at) VALUES ($1,$2,'vaccine','Anulada','2026-01-10','x', now())`, p.clinicA, p.patA)

	c := p.signIn("clinica-a", "dueno@mail.mx")
	rx := c.expect(200, "GET", "/api/portal/prescriptions", nil)
	list := rx["prescriptions"].([]any)
	if len(list) != 2 {
		t.Fatalf("recetas: %v", list)
	}
	first := list[1].(map[string]any) // folio 1 is the older one
	if first["voided"] != false || first["items"].([]any)[0].(map[string]any)["medicine"] != "Amoxicilina" || list[0].(map[string]any)["voided"] != true {
		t.Fatalf("rx shape: %v", list)
	}
	vac := c.expect(200, "GET", "/api/portal/vaccinations", nil)
	if len(vac["vaccinations"].([]any)) != 1 {
		t.Fatalf("vaccinations: %v", vac)
	}
	appts := c.expect(200, "GET", "/api/portal/appointments", nil)
	me := c.expect(200, "GET", "/api/portal/me", nil)
	for name, body := range map[string]string{"rx": raw(rx), "vac": raw(vac), "appts": raw(appts), "me": raw(me)} {
		for _, secret := range []string{"SECRETO", "SECRETA", "Otra clínica", "Ajeno"} {
			if strings.Contains(body, secret) {
				t.Fatalf("%s leaks %q: %s", name, secret, body)
			}
		}
	}
	// every read of patient data left a trace with the portal as the actor
	var n int
	p.pool.QueryRow(ctx(), `SELECT count(*) FROM record_access WHERE clinic_id = $1 AND patient_id = $2 AND user_name = 'Portal (dueno@mail.mx)' AND user_id IS NULL`, p.clinicA, p.patA).Scan(&n)
	if n < 3 {
		t.Fatalf("record_access rows: %d", n)
	}
}

func TestPortalSettings(t *testing.T) {
	e := setup(t)
	admin, recep := e.login("admin_a"), e.login("recep_a")
	recep.expect(403, "GET", "/api/clinic/portal", nil)
	recep.expect(403, "PUT", "/api/clinic/portal", map[string]any{"enabled": true})
	e.anon().expect(401, "GET", "/api/clinic/portal", nil)

	got := sub(admin.expect(200, "GET", "/api/clinic/portal", nil), "portal")
	if got["enabled"] != false {
		t.Fatalf("default: %v", got)
	}
	// the portal shares the booking address, which must exist first
	admin.expect(409, "PUT", "/api/clinic/portal", map[string]any{"enabled": true, "welcome": "x"})
	e.exec(`INSERT INTO agenda_settings (clinic_id, booking_slug) VALUES ($1, 'clinica-a')`, e.clinicA)
	admin.expect(400, "PUT", "/api/clinic/portal", map[string]any{"enabled": true, "welcome": strings.Repeat("x", 601)})
	admin.expect(200, "PUT", "/api/clinic/portal", map[string]any{"enabled": true, "welcome": " Bienvenidos "})
	got = sub(admin.expect(200, "GET", "/api/clinic/portal", nil), "portal")
	if got["enabled"] != true || got["welcome"] != "Bienvenidos" || got["slug"] != "clinica-a" {
		t.Fatalf("saved: %v", got)
	}
	if sub(e.anon().expect(200, "GET", "/api/portal/clinica-a/info", nil), "clinic")["welcome"] != "Bienvenidos" {
		t.Fatal("welcome not public")
	}
	// clinic B's admin does not touch clinic A
	if sub(e.login("admin_b").expect(200, "GET", "/api/clinic/portal", nil), "portal")["enabled"] != false {
		t.Fatal("tenant leak")
	}
	admin.expect(200, "PUT", "/api/clinic/portal", map[string]any{"enabled": false, "welcome": ""})
	e.anon().expect(404, "GET", "/api/portal/clinica-a/info", nil)
}
