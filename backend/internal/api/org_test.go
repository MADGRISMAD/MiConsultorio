package api_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"golang.org/x/crypto/bcrypt"
)

// Organizations and branches: the point of these tests is isolation. A branch is a clinic; only the owner,
// through POST /org/switch, ever gets a session in more than one of them.

func orgSetup(t *testing.T) *env {
	t.Helper()
	e := setupWith(t, func(c *config.Config) { c.UploadsDir = t.TempDir(); c.MaxUploadBytes = 64 << 10 })
	e.exec(`UPDATE clinics SET plan = 'pro'`) // branches come with Pro (10); Crecimiento runs one clinic
	return e
}

func orgNewBranch(t *testing.T, c *client, name string) string {
	t.Helper()
	out := c.expect(201, "POST", "/api/org/branches", map[string]any{"name": name, "kind": "DENTAL", "phone_number": "5511112222", "address": "Calle 1"})
	return sub(out, "branch")["id"].(string)
}

func orgSwitch(t *testing.T, c *client, id string) map[string]any {
	t.Helper()
	return sub(c.expect(200, "POST", "/api/org/switch", map[string]any{"branch_id": id}), "session")
}

// orgTokenClient builds a client holding a hand-made session cookie.
func orgTokenClient(e *env, claims jwt.MapClaims) *client {
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(strings.Repeat("s", 40)))
	c := e.anon()
	u, _ := url.Parse(e.srv.URL)
	c.c.Jar.SetCookies(u, []*http.Cookie{{Name: "caresia_session", Value: tok, Path: "/"}})
	return c
}

func orgClaims(sub string, tv int, otv *int) jwt.MapClaims {
	m := jwt.MapClaims{"sub": sub, "tv": tv, "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix()}
	if otv != nil {
		m["otv"] = *otv
	}
	return m
}

func (e *env) orgLinkedUser(clinicID string) string {
	e.t.Helper()
	var id string
	if err := e.pool.QueryRow(context.Background(), `SELECT id FROM users WHERE clinic_id = $1 AND linked_owner_id IS NOT NULL`, clinicID).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func TestOrgBranchCreationAndLimits(t *testing.T) {
	e := orgSetup(t)
	owner := e.login("admin_a")

	// before any branch: nothing to show, but the first administrator may start one
	ov := owner.expect(200, "GET", "/api/org", nil)
	if ov["organization"] != nil || ov["can_create"] != true || ov["is_owner"] != false || ov["branch_limit"].(float64) != 10 {
		t.Fatalf("overview: %v", ov)
	}
	if e.login("doc_a").expect(200, "GET", "/api/org", nil)["can_create"] != false {
		t.Fatal("a doctor must not be offered to create branches")
	}
	e.login("doc_a").expect(403, "POST", "/api/org/branches", map[string]any{"name": "Norte"})
	e.login("recep_a").expect(403, "POST", "/api/org/branches", map[string]any{"name": "Norte"})

	// a second administrator is not the owner: cannot found an organization
	e.addUser(e.clinicA, "admin2_a", "admin")
	second := e.login("admin2_a")
	if second.expect(200, "GET", "/api/org", nil)["can_create"] != false {
		t.Fatal("only the first administrator may found the organization")
	}
	if code, out := second.do("POST", "/api/org/branches", map[string]any{"name": "Intruso"}); code != 403 || out["code"] != "NOT_ORG_OWNER" {
		t.Fatalf("second admin founding: %d %v", code, out)
	}
	var n int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM organizations`).Scan(&n)
	if n != 0 {
		t.Fatal("no organization should exist yet")
	}

	owner.expect(400, "POST", "/api/org/branches", map[string]any{"name": "x"})
	owner.expect(400, "POST", "/api/org/branches", map[string]any{"name": "Norte", "kind": "NOPE"})

	north := orgNewBranch(t, owner, "Sucursal Norte")
	ov = owner.expect(200, "GET", "/api/org", nil)
	if sub(ov, "organization")["name"] != "Clinica a" || ov["is_owner"] != true || len(ov["branches"].([]any)) != 2 {
		t.Fatalf("overview after: %v", ov)
	}
	first := ov["branches"].([]any)[0].(map[string]any)
	if first["is_matrix"] != true || first["id"] != e.clinicA || first["current"] != true {
		t.Fatalf("matrix first: %v", first)
	}

	// the new branch is a normal clinic: setup done, plan and subscription copied from the matrix
	var plan, status string
	var setup bool
	if err := e.pool.QueryRow(context.Background(), `SELECT plan, billing_status, setup_completed_at IS NOT NULL FROM clinics WHERE id = $1`, north).Scan(&plan, &status, &setup); err != nil {
		t.Fatal(err)
	}
	if plan != "pro" || status != "active" || !setup {
		t.Fatalf("branch: %s %s %v", plan, status, setup)
	}

	// Crecimiento runs a single clinic; Pro allows 10 and the plan follows the matrix to every branch
	e.exec(`UPDATE clinics SET plan = 'crecimiento' WHERE id = $1`, e.clinicB)
	if code, out := e.login("admin_b").do("POST", "/api/org/branches", map[string]any{"name": "Otra"}); code != 409 || out["code"] != "BRANCH_LIMIT" {
		t.Fatalf("crecimiento: %d %v", code, out)
	}
	e.exec(`UPDATE clinics SET plan = 'pro' WHERE id = $1`, e.clinicB)
	e.exec(`UPDATE clinics SET plan = 'basico' WHERE id = $1`, north) // a branch cannot hold another plan
	_ = e.pool.QueryRow(context.Background(), `SELECT plan FROM clinics WHERE id = $1`, north).Scan(&plan)
	if plan != "pro" {
		t.Fatalf("branch plan must follow the matrix, got %s", plan)
	}
	orgNewBranch(t, owner, "Sucursal Sur")
	for i := 0; i < 7; i++ { // matrix + north + south + 7 = 10
		orgNewBranch(t, owner, "Extra "+string(rune('A'+i)))
	}
	if code, out := owner.do("POST", "/api/org/branches", map[string]any{"name": "Once"}); code != 409 || out["code"] != "BRANCH_LIMIT" {
		t.Fatalf("pro limit: %d %v", code, out)
	}

	// Básico: a single clinic, so the organization is never even created
	e.exec(`UPDATE clinics SET plan = 'basico' WHERE id = $1`, e.clinicB)
	if code, out := e.login("admin_b").do("POST", "/api/org/branches", map[string]any{"name": "Otra"}); code != 409 || out["code"] != "BRANCH_LIMIT" {
		t.Fatalf("basico: %d %v", code, out)
	}
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM organizations WHERE matrix_clinic_id = $1`, e.clinicB).Scan(&n)
	if n != 0 {
		t.Fatal("a rejected founding must not leave an organization behind")
	}
}

func TestOrgSwitchGivesBranchSessionOnly(t *testing.T) {
	e := orgSetup(t)
	owner := e.login("admin_a")
	north := orgNewBranch(t, owner, "Sucursal Norte")

	ownerID := e.userID("admin_a")
	s := orgSwitch(t, owner, north)
	linked := e.orgLinkedUser(north)
	if s["clinicId"] != north || s["userId"] != linked || s["role"] != "admin" || s["email"] != "" || s["name"] != "ADMIN A" && s["name"] != "Admin a" {
		t.Fatalf("branch session: %v", s)
	}
	// the cookie now belongs to the branch: the session reports it, and it can come back
	if got := sub(owner.expect(200, "GET", "/api/session", nil), "session")["clinicId"]; got != north {
		t.Fatalf("session after switch: %v", got)
	}
	ov := owner.expect(200, "GET", "/api/org", nil)
	if ov["is_owner"] != true || ov["current_clinic_id"] != north {
		t.Fatalf("overview from the branch: %v", ov)
	}
	back := orgSwitch(t, owner, e.clinicA)
	if back["clinicId"] != e.clinicA || back["userId"] != ownerID {
		t.Fatalf("back to matrix: %v", back)
	}

	// the linked account cannot be used to sign in directly, with any password, even one set by hand
	var username string
	_ = e.pool.QueryRow(context.Background(), `SELECT username FROM users WHERE id = $1`, linked).Scan(&username)
	anon := e.anon()
	anon.expect(401, "POST", "/api/login", map[string]string{"identifier": username, "password": pw})
	h, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	e.exec(`UPDATE users SET password_hash = $2 WHERE id = $1`, linked, string(h))
	anon.expect(401, "POST", "/api/login", map[string]string{"identifier": username, "password": pw})
	anon.expect(200, "POST", "/api/forgot", map[string]string{"identifier": username})
	if len(e.mail.sent) != 0 {
		t.Fatal("the branch account has no e-mail: nothing may be sent")
	}

	// it takes no seat of the branch
	team := orgSwitchedTeam(t, e, owner, north)
	if sub(team, "seats")["used_users"].(float64) != 0 {
		t.Fatalf("seats: %v", team["seats"])
	}
	// and the branch's own administrators cannot touch it
	e.addUser(north, "admin_n", "admin")
	local := e.login("admin_n")
	local.expect(403, "PATCH", "/api/team/"+linked, map[string]any{"name": "Otro nombre", "role": "doctor"})
	local.expect(403, "POST", "/api/team/"+linked+"/password", map[string]any{"password": newPw})
	local.expect(403, "POST", "/api/team/"+linked+"/deactivate", nil)
	local.expect(403, "POST", "/api/team/"+linked+"/reactivate", nil)
}

func orgSwitchedTeam(t *testing.T, e *env, owner *client, branch string) map[string]any {
	t.Helper()
	orgSwitch(t, owner, branch)
	defer orgSwitch(t, owner, e.clinicA)
	return owner.expect(200, "GET", "/api/team", nil)
}

func TestOrgSwitchRejectsEveryoneElse(t *testing.T) {
	e := orgSetup(t)
	owner := e.login("admin_a")
	north := orgNewBranch(t, owner, "Sucursal Norte")
	e.exec(`UPDATE clinics SET plan = 'pro' WHERE id = $1`, e.clinicB)
	otherOwner := e.login("admin_b")
	mine := orgNewBranch(t, otherOwner, "Sucursal de B")

	body := map[string]any{"branch_id": north}
	// no organization at all
	if code, out := e.login("doc_b").do("POST", "/api/org/switch", body); code != 403 {
		t.Fatalf("doctor: %d %v", code, out)
	}
	e.anon().expect(401, "POST", "/api/org/switch", body)
	e.login("root").expect(403, "POST", "/api/org/switch", body)
	e.login("help").expect(403, "POST", "/api/org/switch", body)
	// the owner of ANOTHER organization cannot enter this one, nor can a second administrator of the same clinic
	if code, out := otherOwner.do("POST", "/api/org/switch", body); code != 404 {
		t.Fatalf("foreign owner: %d %v", code, out)
	}
	e.addUser(e.clinicA, "admin2_a", "admin")
	if code, out := e.login("admin2_a").do("POST", "/api/org/switch", body); code != 403 || out["code"] != "NOT_ORG_OWNER" {
		t.Fatalf("second admin: %d %v", code, out)
	}
	// an administrator that belongs to the branch itself is not the owner either: cannot hop to the matrix
	e.addUser(north, "admin_n", "admin")
	local := e.login("admin_n")
	if code, out := local.do("POST", "/api/org/switch", map[string]any{"branch_id": e.clinicA}); code != 403 || out["code"] != "NOT_ORG_OWNER" {
		t.Fatalf("branch admin: %d %v", code, out)
	}
	local.expect(403, "GET", "/api/org/reports/summary", nil)
	local.expect(403, "POST", "/api/org/branches", map[string]any{"name": "Colada"})
	if local.expect(200, "GET", "/api/org", nil)["organization"] != nil {
		t.Fatal("a branch administrator must not see the organization")
	}
	// invalid, unknown and foreign ids all look the same
	for _, id := range []string{"", "nope", "00000000-0000-0000-0000-000000000000", e.clinicB, mine} {
		if code, _ := owner.do("POST", "/api/org/switch", map[string]any{"branch_id": id}); code != 404 {
			t.Errorf("owner A switching to %q: %d", id, code)
		}
	}
	if code, _ := otherOwner.do("POST", "/api/org/switch", map[string]any{"branch_id": mine}); code != 200 {
		t.Fatalf("B owner into own branch: %d", code)
	}
	// the owner's session never moved to a foreign clinic
	if got := sub(owner.expect(200, "GET", "/api/session", nil), "session")["clinicId"]; got != e.clinicA {
		t.Fatalf("A owner is now in %v", got)
	}
}

func TestOrgManipulatedTokens(t *testing.T) {
	e := orgSetup(t)
	owner := e.login("admin_a")
	north := orgNewBranch(t, owner, "Sucursal Norte")
	linked := e.orgLinkedUser(north)
	ownerID := e.userID("admin_a")
	var otv, tv, ltv int
	_ = e.pool.QueryRow(context.Background(), `SELECT token_version FROM users WHERE id = $1`, ownerID).Scan(&otv)
	_ = e.pool.QueryRow(context.Background(), `SELECT token_version FROM users WHERE id = $1`, linked).Scan(&ltv)
	tv = ltv

	// the right claims work (that is exactly what the switch signs) ...
	right := orgTokenClient(e, orgClaims(linked, tv, &otv))
	right.expect(200, "GET", "/api/session", nil)
	// ... but the branch account needs the owner's version, and nothing else will do
	orgTokenClient(e, orgClaims(linked, tv, nil)).expect(401, "GET", "/api/session", nil)
	wrong := otv + 1
	orgTokenClient(e, orgClaims(linked, tv, &wrong)).expect(401, "GET", "/api/session", nil)
	badTV := tv + 5
	orgTokenClient(e, orgClaims(linked, badTV, &otv)).expect(401, "GET", "/api/session", nil)
	// an ordinary account cannot borrow the claim, and the claim does not change whose session it is
	orgTokenClient(e, orgClaims(ownerID, otv, &otv)).expect(401, "GET", "/api/session", nil)
	orgTokenClient(e, orgClaims(ownerID, otv, nil)).expect(200, "GET", "/api/session", nil)
	// signed with another key, or unsigned
	bad, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, orgClaims(linked, tv, &otv)).SignedString([]byte(strings.Repeat("x", 40)))
	c := e.anon()
	u, _ := url.Parse(e.srv.URL)
	c.c.Jar.SetCookies(u, []*http.Cookie{{Name: "caresia_session", Value: bad, Path: "/"}})
	c.expect(401, "GET", "/api/session", nil)
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, orgClaims(linked, tv, &otv)).SignedString(jwt.UnsafeAllowNoneSignatureType)
	c.c.Jar.SetCookies(u, []*http.Cookie{{Name: "caresia_session", Value: none, Path: "/"}})
	c.expect(401, "GET", "/api/session", nil)

	// the branch session dies with the owner's: password change, deactivation, role change
	inside := e.login("admin_a")
	orgSwitch(t, inside, north)
	inside.expect(200, "GET", "/api/patients/", nil)
	e.exec(`UPDATE users SET token_version = token_version + 1 WHERE id = $1`, ownerID)
	inside.expect(401, "GET", "/api/patients/", nil)

	owner2 := e.loginPw("admin_a", pw)
	orgSwitch(t, owner2, north)
	e.exec(`UPDATE users SET disabled = true WHERE id = $1`, ownerID)
	owner2.expect(401, "GET", "/api/patients/", nil)
	e.exec(`UPDATE users SET disabled = false WHERE id = $1`, ownerID)

	// a demoted owner is no longer an owner, even with a still-valid token
	owner3 := e.loginPw("admin_a", pw)
	orgSwitch(t, owner3, north)
	e.exec(`UPDATE users SET role = 'doctor' WHERE id = $1`, ownerID)
	owner3.expect(401, "GET", "/api/patients/", nil)
}

func TestOrgIsolationBetweenBranches(t *testing.T) {
	e := orgSetup(t)
	owner := e.login("admin_a")
	north := orgNewBranch(t, owner, "Sucursal Norte")
	ownerB := e.login("admin_b")

	// seed: one patient with a record, an appointment, a sale and a file in the matrix (A), the branch (N) and a stranger (B)
	type seeded struct{ patient, encounter, appointment, sale, file string }
	seed := func(c *client, clinic string) seeded {
		var s seeded
		s.patient = sub(c.expect(201, "POST", "/api/patients/", person(nil)), "patient")["id"].(string)
		s.encounter = sub(c.expect(201, "POST", "/api/patients/"+s.patient+"/encounters", map[string]any{"kind": "consulta", "reason": "Dolor", "assessment": "Cefalea"}), "encounter")["id"].(string)
		if err := e.pool.QueryRow(context.Background(), `INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour) VALUES ($1, 'X', 'N', 'L', current_date, '10:00', '10:30') RETURNING id`, clinic).Scan(&s.appointment); err != nil {
			t.Fatal(err)
		}
		if err := e.pool.QueryRow(context.Background(), `INSERT INTO sales (clinic_id, folio, subtotal_cents, total_cents) VALUES ($1, 1, 1000, 1000) RETURNING id`, clinic).Scan(&s.sale); err != nil {
			t.Fatal(err)
		}
		st, out := c.upload(s.patient, upload{name: "x.jpg", data: jpegBytes})
		if st != 201 {
			t.Fatalf("upload: %d %v", st, out)
		}
		s.file = sub(out, "file")["id"].(string)
		return s
	}
	sa := seed(owner, e.clinicA)
	orgSwitch(t, owner, north)
	sn := seed(owner, north)
	orgSwitch(t, owner, e.clinicA)
	sb := seed(ownerB, e.clinicB)

	// every read route, tried with a session of one clinic against the records of another
	type probe struct{ path string }
	probes := func(s seeded) []string {
		return []string{
			"/api/patients/" + s.patient, "/api/patients/" + s.patient + "/encounters", "/api/patients/" + s.patient + "/prescriptions",
			"/api/patients/" + s.patient + "/record", "/api/patients/" + s.patient + "/files",
			"/api/patients/" + s.patient + "/export", "/api/files/" + s.file, "/api/appointments/" + s.appointment, "/api/pos/sales/" + s.sale,
		}
	}
	listHas := func(c *client, path, key, id string) bool {
		out := c.expect(200, "GET", path, nil)
		for _, x := range out[key].([]any) {
			if x.(map[string]any)["id"] == id {
				return true
			}
		}
		return false
	}
	check := func(who string, c *client, allowed, forbidden seeded) {
		pos := !strings.Contains(who, "doctor")
		t.Helper()
		for _, path := range probes(forbidden) {
			if !pos && strings.Contains(path, "/pos/") {
				continue
			}
			if code, out := c.do("GET", path, nil); code != 404 {
				t.Errorf("%s reading %s: %d %v", who, path, code, out)
			}
		}
		for _, path := range probes(allowed) {
			if !pos && strings.Contains(path, "/pos/") {
				continue
			}
			if code, out := c.do("GET", path, nil); code != 200 {
				t.Errorf("%s own %s: %d %v", who, path, code, out)
			}
		}
		if listHas(c, "/api/patients/", "patients", forbidden.patient) || !listHas(c, "/api/patients/", "patients", allowed.patient) {
			t.Errorf("%s patient list mixes clinics", who)
		}
		if pos && (listHas(c, "/api/pos/sales", "sales", forbidden.sale) || !listHas(c, "/api/pos/sales", "sales", allowed.sale)) {
			t.Errorf("%s sales list mixes clinics", who)
		}
		// the access trail of a foreign patient is empty, not an error that confirms it exists
		if pos { // administrators only
			if out := c.expect(200, "GET", "/api/patients/"+forbidden.patient+"/access", nil); len(out["access"].([]any)) != 0 {
				t.Errorf("%s reads the access trail of a foreign patient: %v", who, out)
			}
		}
		// writes too
		c.expect(404, "POST", "/api/patients/"+forbidden.patient+"/encounters", map[string]any{"kind": "consulta", "reason": "x"})
		if pos {
			c.expect(404, "PUT", "/api/appointments/"+forbidden.appointment, apptBody(nil))
		}
		if pos {
			c.expect(404, "POST", "/api/pos/sales/"+forbidden.sale+"/void", map[string]any{"reason": "no"})
		}
		c.expect(404, "POST", "/api/files/"+forbidden.file+"/archive", map[string]any{"reason": "no"})
		if st, _ := c.upload(forbidden.patient, upload{name: "x.jpg", data: jpegBytes}); st != 404 {
			t.Errorf("%s uploading to a foreign patient: %d", who, st)
		}
	}
	check("A matrix", owner, sa, sn)
	check("A matrix vs B", owner, sa, sb)
	check("B owner vs N", ownerB, sb, sn)
	check("B owner vs A", ownerB, sb, sa)
	check("doctor of B vs N", e.login("doc_b"), seeded{patient: sb.patient, file: sb.file, appointment: sb.appointment, sale: sb.sale, encounter: sb.encounter}, seeded{patient: sn.patient, file: sn.file, appointment: "00000000-0000-0000-0000-000000000000", sale: sn.sale, encounter: sn.encounter})
	check("doctor of A vs N", e.login("doc_a"), seeded{patient: sa.patient, file: sa.file, appointment: sa.appointment, sale: sa.sale, encounter: sa.encounter}, seeded{patient: sn.patient, file: sn.file, appointment: sn.appointment, sale: sn.sale, encounter: sn.encounter})

	// the owner inside the branch sees only the branch
	orgSwitch(t, owner, north)
	check("N session vs A", owner, sn, sa)
	check("N session vs B", owner, sn, sb)
	// a branch administrator of N, hired locally, sees only N as well
	e.addUser(north, "doc_n", "doctor")
	check("doctor of N vs A", e.login("doc_n"), seeded{patient: sn.patient, file: sn.file, appointment: sn.appointment, sale: sn.sale, encounter: sn.encounter}, seeded{patient: sa.patient, file: sa.file, appointment: "00000000-0000-0000-0000-000000000000", sale: sa.sale, encounter: sa.encounter})
	orgSwitch(t, owner, e.clinicA)

	// platform: support reads the clinic list as before (every clinic, branches included) but never clinical data
	help := e.login("help")
	help.expect(403, "GET", "/api/patients/"+sn.patient, nil)
	help.expect(403, "GET", "/api/org", nil)
	for _, id := range []string{e.clinicA, north, e.clinicB} {
		help.expect(200, "GET", "/api/platform/clinics/"+id, nil)
	}
	// the clinical access trail of the branch patient names the person who really entered
	var who int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM record_access WHERE clinic_id <> $1 AND patient_id = $2`, north, sn.patient).Scan(&who)
	if who != 0 {
		t.Fatal("record_access of branch patient written under another clinic")
	}
}

func TestOrgSuspendBranch(t *testing.T) {
	e := orgSetup(t)
	owner := e.login("admin_a")
	north := orgNewBranch(t, owner, "Sucursal Norte")
	south := orgNewBranch(t, owner, "Sucursal Sur")
	orgSwitch(t, owner, north)
	pid := sub(owner.expect(201, "POST", "/api/patients/", person(nil)), "patient")["id"].(string)
	e.addUser(north, "doc_n", "doctor")
	orgSwitch(t, owner, e.clinicA)

	// rules
	owner.expect(409, "POST", "/api/org/branches/"+e.clinicA+"/suspend", nil) // the matrix carries the subscription
	owner.expect(404, "POST", "/api/org/branches/"+e.clinicB+"/suspend", nil)
	owner.expect(404, "POST", "/api/org/branches/nope/suspend", nil)
	e.login("doc_a").expect(403, "POST", "/api/org/branches/"+north+"/suspend", nil)
	e.login("admin_b").expect(403, "POST", "/api/org/branches/"+north+"/suspend", nil)
	e.login("root").expect(403, "POST", "/api/org/branches/"+north+"/suspend", nil)
	orgSwitch(t, owner, south)
	owner.expect(409, "POST", "/api/org/branches/"+south+"/suspend", nil) // not the one you are standing in
	orgSwitch(t, owner, e.clinicA)

	localDoc := e.login("doc_n")
	localDoc.expect(200, "GET", "/api/patients/", nil)
	out := owner.expect(200, "POST", "/api/org/branches/"+north+"/suspend", nil)
	if len(out["branches"].([]any)) != 3 {
		t.Fatalf("branches: %v", out)
	}
	// the branch is locked like a suspended clinic; its data stays
	if code, out := localDoc.do("GET", "/api/patients/", nil); code != 403 || out["code"] != "SUBSCRIPTION_REQUIRED" {
		t.Fatalf("suspended branch user: %d %v", code, out)
	}
	if code, out := owner.do("POST", "/api/org/switch", map[string]any{"branch_id": north}); code != 409 || out["code"] != "BRANCH_SUSPENDED" {
		t.Fatalf("switch into suspended: %d %v", code, out)
	}
	var kept int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM patients WHERE id = $1`, pid).Scan(&kept)
	if kept != 1 {
		t.Fatal("suspending a branch must never delete clinical data")
	}
	// the freed slot can be used, and then reactivation is limited by the plan
	var extra string
	for i := 0; i < 8; i++ { // fill Pro's 10 branches (matrix + south + 8)
		extra = orgNewBranch(t, owner, "Extra "+string(rune('A'+i)))
	}
	if code, out := owner.do("POST", "/api/org/branches/"+north+"/reactivate", nil); code != 409 || out["code"] != "BRANCH_LIMIT" {
		t.Fatalf("reactivate over limit: %d %v", code, out)
	}
	e.exec(`UPDATE clinics SET branch_suspended_at = now() WHERE id = $1`, extra)
	owner.expect(200, "POST", "/api/org/branches/"+north+"/reactivate", nil)
	localDoc.expect(200, "GET", "/api/patients/", nil)
	orgSwitch(t, owner, north)
}

func TestOrgSubscriptionSharedAndBilling(t *testing.T) {
	e := setupWith(t, func(c *config.Config) {
		c.MPAccessToken, c.MPAPIBase = "tok", "http://127.0.0.1:1"
		c.PlanPriceMonth["basico"], c.PlanPriceMonth["crecimiento"] = 500, 1200
	})
	e.exec(`UPDATE clinics SET plan = 'pro'`) // branches come with Pro
	owner := e.login("admin_a")
	north := orgNewBranch(t, owner, "Sucursal Norte")
	orgNewBranch(t, owner, "Sucursal Sur")

	// suspending the matrix suspends every branch, and the other way round when it comes back
	e.exec(`UPDATE clinics SET billing_status = 'suspended', suspended_reason = 'impago' WHERE id = $1`, e.clinicA)
	var status string
	_ = e.pool.QueryRow(context.Background(), `SELECT billing_status FROM clinics WHERE id = $1`, north).Scan(&status)
	if status != "suspended" {
		t.Fatalf("branch status: %s", status)
	}
	e.exec(`UPDATE clinics SET billing_status = 'active', suspended_reason = '' WHERE id = $1`, e.clinicA)
	_ = e.pool.QueryRow(context.Background(), `SELECT billing_status FROM clinics WHERE id = $1`, north).Scan(&status)
	if status != "active" {
		t.Fatalf("branch status after: %s", status)
	}
	// paying happens at the matrix; the branch cannot buy a plan of its own, and the plan cannot drop below the branches in service
	orgSwitch(t, owner, north)
	if code, out := owner.do("POST", "/api/billing/checkout", map[string]any{"plan": "crecimiento"}); code != 409 || !strings.Contains(out["message"].(string), "matriz") {
		t.Fatalf("branch checkout: %d %v", code, out)
	}
	orgSwitch(t, owner, e.clinicA)
	if code, out := owner.do("POST", "/api/billing/checkout", map[string]any{"plan": "basico"}); code != 409 || out["code"] != "BRANCH_LIMIT" {
		t.Fatalf("downgrade with branches: %d %v", code, out)
	}
}

func TestOrgConsolidatedReports(t *testing.T) {
	e := orgSetup(t)
	owner := e.login("admin_a")
	north := orgNewBranch(t, owner, "Sucursal Norte")
	ownerB := e.login("admin_b")

	today := time.Now().Format("2006-01-02")
	sale := func(clinic string, folio, cents int, status string) {
		e.exec(`INSERT INTO sales (clinic_id, folio, subtotal_cents, total_cents, status) VALUES ($1, $2, $3, $3, $4)`, clinic, folio, cents, status)
	}
	appt := func(clinic, status string, date string) {
		e.exec(`INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, status) VALUES ($1, 'X', 'N', 'L', $2::date, '10:00', '10:30', $3)`, clinic, date, status)
	}
	enc := func(clinic string) {
		e.exec(`INSERT INTO patients (clinic_id, file_number, names) VALUES ($1, (SELECT coalesce(max(file_number), 0) + 1 FROM patients WHERE clinic_id = $1), 'P')`, clinic)
		e.exec(`INSERT INTO encounters (clinic_id, patient_id, kind, author_name, reason) SELECT $1, id, 'consulta', 'Dr', 'x' FROM patients WHERE clinic_id = $1 ORDER BY created_at DESC LIMIT 1`, clinic)
	}
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	sale(e.clinicA, 1, 10000, "paid")
	sale(e.clinicA, 2, 20000, "paid")
	sale(e.clinicA, 3, 99900, "void")
	sale(north, 1, 5000, "paid")
	sale(e.clinicB, 1, 777700, "paid") // not ours
	appt(e.clinicA, "completed", yesterday)
	appt(e.clinicA, "no_show", yesterday)
	appt(e.clinicA, "cancelled", yesterday)
	appt(north, "no_show", yesterday)
	appt(north, "completed", yesterday)
	appt(e.clinicB, "no_show", yesterday)
	appt(e.clinicB, "no_show", yesterday)
	enc(e.clinicA)
	enc(e.clinicA)
	enc(north)
	enc(e.clinicB)

	sum := owner.expect(200, "GET", "/api/org/reports/summary?from="+yesterday+"&to="+today, nil)
	branches := sum["branches"].([]any)
	if len(branches) != 2 {
		t.Fatalf("branches in report: %v", branches)
	}
	by := map[string]map[string]any{}
	for _, b := range branches {
		m := b.(map[string]any)
		by[m["id"].(string)] = m
	}
	if _, foreign := by[e.clinicB]; foreign || by[e.clinicA] == nil || by[north] == nil {
		t.Fatalf("report covers the wrong clinics: %v", by)
	}
	a, n, total := by[e.clinicA], by[north], sum["total"].(map[string]any)
	if sub(a, "sales")["total_cents"].(float64) != 30000 || sub(a, "sales")["count"].(float64) != 2 || sub(a, "sales")["avg_ticket_cents"].(float64) != 15000 || sub(a, "sales")["void_count"].(float64) != 1 {
		t.Fatalf("A sales: %v", a["sales"])
	}
	if sub(n, "sales")["total_cents"].(float64) != 5000 || sub(total, "sales")["total_cents"].(float64) != 35000 || sub(total, "sales")["avg_ticket_cents"].(float64) != 11666 {
		t.Fatalf("N / total sales: %v / %v", n["sales"], total["sales"])
	}
	if sub(a, "appointments")["total"].(float64) != 3 || sub(a, "appointments")["no_show"].(float64) != 1 || sub(a, "appointments")["eligible"].(float64) != 2 || sub(a, "appointments")["no_show_rate"].(float64) != 50 {
		t.Fatalf("A appointments: %v", a["appointments"])
	}
	if sub(total, "appointments")["total"].(float64) != 5 || sub(total, "appointments")["no_show"].(float64) != 2 {
		t.Fatalf("total appointments: %v", total["appointments"])
	}
	if a["consultations"].(float64) != 2 || n["consultations"].(float64) != 1 || total["consultations"].(float64) != 3 || total["new_patients"].(float64) != 3 {
		t.Fatalf("consultations: %v %v %v", a["consultations"], n["consultations"], total)
	}
	if len(sum["daily"].([]any)) != 2 {
		t.Fatalf("daily: %v", sum["daily"])
	}

	// client-supplied ids change nothing; the same totals come from inside a branch
	again := owner.expect(200, "GET", "/api/org/reports/summary?from="+yesterday+"&to="+today+"&clinic_id="+e.clinicB+"&branch_id="+e.clinicB, nil)
	if sub(sub(again, "total"), "sales")["total_cents"].(float64) != 35000 || len(again["branches"].([]any)) != 2 {
		t.Fatalf("injection: %v", again["total"])
	}
	orgSwitch(t, owner, north)
	inside := owner.expect(200, "GET", "/api/org/reports/summary?from="+yesterday+"&to="+today, nil)
	if sub(sub(inside, "total"), "sales")["total_cents"].(float64) != 35000 {
		t.Fatalf("report from a branch session: %v", inside["total"])
	}
	orgSwitch(t, owner, e.clinicA)

	// B has its own world
	e.exec(`UPDATE clinics SET plan = 'pro' WHERE id = $1`, e.clinicB)
	bBranch := orgNewBranch(t, ownerB, "Sucursal B2")
	bsum := ownerB.expect(200, "GET", "/api/org/reports/summary?from="+yesterday+"&to="+today, nil)
	if sub(sub(bsum, "total"), "sales")["total_cents"].(float64) != 777700 || len(bsum["branches"].([]any)) != 2 {
		t.Fatalf("B report: %v", bsum["total"])
	}
	for _, b := range bsum["branches"].([]any) {
		if id := b.(map[string]any)["id"]; id != e.clinicB && id != bBranch {
			t.Fatalf("B report leaks %v", id)
		}
	}

	// permissions and validation
	for _, who := range []string{"doc_a", "recep_a", "cash_a"} {
		e.login(who).expect(403, "GET", "/api/org/reports/summary", nil)
	}
	e.addUser(e.clinicA, "admin2_a", "admin")
	e.login("admin2_a").expect(403, "GET", "/api/org/reports/summary", nil)
	e.login("admin2_a").expect(403, "GET", "/api/org/reports/summary.csv", nil)
	e.login("root").expect(403, "GET", "/api/org/reports/summary", nil)
	e.anon().expect(401, "GET", "/api/org/reports/summary", nil)
	owner.expect(400, "GET", "/api/org/reports/summary?from=garbage", nil)
	owner.expect(400, "GET", "/api/org/reports/summary?from=2020-01-01&to=2025-01-01", nil)

	// CSV
	res, body := owner.raw("GET", "/api/org/reports/summary.csv?from="+yesterday+"&to="+today)
	csv := string(body)
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/csv") || !strings.Contains(csv, "Sucursal Norte") || !strings.Contains(csv, "Clinica a") || strings.Contains(csv, "Clinica b") || strings.Contains(csv, "7777.00") || !strings.Contains(csv, "350.00") {
		t.Fatalf("csv: %d %s", res.StatusCode, csv)
	}

	// without cobros the branches show no sales at all, even if rows exist
	e.exec(`UPDATE clinics SET plan = 'basico' WHERE id = $1`, e.clinicA)
	nosales := owner.expect(200, "GET", "/api/org/reports/summary?from="+yesterday+"&to="+today, nil)
	if sub(sub(nosales, "total"), "sales")["enabled"] != false || sub(sub(nosales, "total"), "sales")["total_cents"].(float64) != 0 {
		t.Fatalf("basico sales: %v", sub(nosales, "total")["sales"])
	}
}
