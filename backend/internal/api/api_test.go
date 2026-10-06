package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/api"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

const (
	pw    = "password123"
	newPw = "long-enough-pw" // password given to people created through the API
)

type env struct {
	t       *testing.T
	srv     *httptest.Server
	pool    *pgxpool.Pool
	clinicA string
	clinicB string
}

// setup builds a fresh database with two clinics (A on the Clínica plan, B too), each with an
// admin, a doctor, a receptionist and a cashier, plus a platform admin ("root") and support ("help").
func setup(t *testing.T) *env {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, pool: pool}
	e.clinicA = e.seedClinic("a")
	e.clinicB = e.seedClinic("b")
	for _, u := range []struct{ user, role string }{{"root", "platform_admin"}, {"help", "platform_support"}} {
		if _, err := db.CreatePlatformUser(ctx, pool, db.UserParams{Name: u.user, Email: u.user + "@caresia.com", Username: u.user, Password: pw, Role: u.role}); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{JWTSecret: []byte(strings.Repeat("s", 40)), SessionTTL: 3600e9}
	e.srv = httptest.NewServer(api.NewRouter(pool, cfg))
	t.Cleanup(e.srv.Close)
	return e
}

// seedClinic creates clinic "<x>" with users admin_<x>, doc_<x>, recep_<x>, cash_<x>.
func (e *env) seedClinic(x string) string {
	e.t.Helper()
	ctx := context.Background()
	id, err := db.CreateClinic(ctx, e.pool, db.ClinicParams{
		Name: "Clinica " + x, Plan: "clinica", Status: "active",
		AdminName: "Admin " + x, AdminEmail: "admin_" + x + "@clinic.mx", AdminUsername: "admin_" + x, AdminPassword: pw,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	for _, r := range []struct{ prefix, role string }{{"doc", "doctor"}, {"recep", "reception"}, {"cash", "cashier"}} {
		e.addUser(id, r.prefix+"_"+x, r.role)
	}
	return id
}

func (e *env) addUser(clinicID, username, role string) {
	e.t.Helper()
	tx, err := e.pool.Begin(context.Background())
	if err != nil {
		e.t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := db.InsertUser(context.Background(), tx, db.UserParams{ClinicID: clinicID, Name: strings.ToUpper(username), Email: username + "@clinic.mx", Username: username, Password: pw, Role: role}); err != nil {
		e.t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) userID(username string) string {
	e.t.Helper()
	var id string
	if err := e.pool.QueryRow(context.Background(), `SELECT id FROM users WHERE username = $1`, username).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

type client struct {
	e *env
	c *http.Client
}

func (e *env) anon() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, c: &http.Client{Jar: jar}}
}

func (e *env) login(identifier string) *client { return e.loginPw(identifier, pw) }

func (e *env) loginPw(identifier, password string) *client {
	c := e.anon()
	if status, out := c.do("POST", "/api/login", map[string]string{"identifier": identifier, "password": password}); status != 200 {
		e.t.Fatalf("login %s: status %d %v", identifier, status, out)
	}
	return c
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.e.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.c.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func (c *client) expect(want int, method, path string, body any) map[string]any {
	c.e.t.Helper()
	got, out := c.do(method, path, body)
	if got != want {
		c.e.t.Fatalf("%s %s: got %d want %d (%v)", method, path, got, want, out)
	}
	return out
}

func sub(m map[string]any, key string) map[string]any { return m[key].(map[string]any) }

// ---------------------------------------------------------------------------
// Sign in
// ---------------------------------------------------------------------------

func TestLogin(t *testing.T) {
	e := setup(t)
	anon := e.anon()
	anon.expect(401, "GET", "/api/session", nil)
	anon.expect(401, "GET", "/api/appointments", nil)
	anon.expect(401, "POST", "/api/login", map[string]string{"identifier": "admin_a", "password": "wrong-password"})
	anon.expect(401, "POST", "/api/login", map[string]string{"identifier": "ghost", "password": pw})
	anon.expect(400, "POST", "/api/login", map[string]string{"identifier": "admin_a"})

	// by username, by e-mail, case-insensitive, with surrounding spaces
	for _, ident := range []string{"admin_a", "ADMIN_A", "admin_a@clinic.mx", " Admin_A@Clinic.MX "} {
		out := e.anon().expect(200, "POST", "/api/login", map[string]string{"identifier": ident, "password": pw})
		s := sub(out, "session")
		if s["role"] != "admin" || s["username"] != "admin_a" || len(s["permissions"].([]any)) != 5 {
			t.Fatalf("session for %q: %v", ident, s)
		}
		if sub(s, "billing")["usable"] != true || sub(s, "billing")["plan"] != "clinica" {
			t.Fatalf("billing: %v", s["billing"])
		}
	}

	c := e.login("admin_a@clinic.mx")
	c.expect(200, "GET", "/api/session", nil)
	c.expect(204, "POST", "/api/logout", nil)
	c.expect(401, "GET", "/api/session", nil)

	// platform staff have no clinic and no billing
	s := sub(e.login("root").expect(200, "GET", "/api/session", nil), "session")
	if s["role"] != "platform_admin" || s["clinicId"] != "" || s["billing"] != nil || len(s["permissions"].([]any)) != 0 {
		t.Fatalf("platform session: %v", s)
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := setup(t)
	anon := e.anon()
	bad := map[string]string{"identifier": "admin_a", "password": "nope-nope"}
	for i := 0; i < 8; i++ {
		anon.expect(401, "POST", "/api/login", bad)
	}
	anon.expect(429, "POST", "/api/login", bad)
	anon.expect(429, "POST", "/api/login", map[string]string{"identifier": "admin_a", "password": pw}) // locked out even with the right one
}

func TestCrossOriginWriteBlocked(t *testing.T) {
	e := setup(t)
	req, _ := http.NewRequest("POST", e.srv.URL+"/api/login", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://evil.example")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 403 {
		t.Fatalf("got %d want 403", res.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Roles
// ---------------------------------------------------------------------------

func TestRoleMatrix(t *testing.T) {
	e := setup(t)
	// status for: GET team, GET expedients, POST expedients (empty body => 400 when allowed),
	//             GET appointments, POST appointments (empty body => 400 when allowed), GET /platform/overview
	cases := []struct {
		user                                 string
		team, expGet, expPost, apGet, apPost int
		platform                             int
	}{
		{"admin_a", 200, 200, 400, 200, 400, 403},
		{"doc_a", 403, 200, 400, 200, 403, 403},
		{"recep_a", 403, 403, 403, 200, 400, 403},
		{"cash_a", 403, 403, 403, 200, 403, 403},
	}
	for _, tc := range cases {
		c := e.login(tc.user)
		got := []int{
			status(c, "GET", "/api/team"), status(c, "GET", "/api/expedients"), status(c, "POST", "/api/expedients"),
			status(c, "GET", "/api/appointments"), status(c, "POST", "/api/appointments"), status(c, "GET", "/api/platform/overview"),
		}
		want := []int{tc.team, tc.expGet, tc.expPost, tc.apGet, tc.apPost, tc.platform}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: got %v want %v", tc.user, got, want)
		}
	}
	// platform staff can't touch clinic data at all
	for _, u := range []string{"root", "help"} {
		c := e.login(u)
		for _, p := range []string{"/api/clinic", "/api/team", "/api/expedients", "/api/appointments"} {
			if got := status(c, "GET", p); got != 403 {
				t.Errorf("%s GET %s: got %d want 403", u, p, got)
			}
		}
	}
}

func status(c *client, method, path string) int {
	got, _ := c.do(method, path, map[string]any{})
	return got
}

// ---------------------------------------------------------------------------
// Clinic data
// ---------------------------------------------------------------------------

var appt = map[string]any{"names": "Ana", "last_names": "Pérez", "CURP": "pepa800101mdfrrn01", "date": "2030-05-01", "startHour": "09:00", "endHour": "10:00", "details": "Limpieza"}

func TestAppointmentsCRUD(t *testing.T) {
	e := setup(t)
	recep := e.login("recep_a")

	a := sub(recep.expect(201, "POST", "/api/appointments", appt), "appointment")
	id := a["id"].(string)
	if a["CURP"] != "PEPA800101MDFRRN01" || a["startHour"] != "09:00" {
		t.Fatalf("normalization: %v", a)
	}

	bad := map[string]any{}
	for k, v := range appt {
		bad[k] = v
	}
	bad["endHour"] = "08:00"
	recep.expect(400, "POST", "/api/appointments", bad)
	bad["endHour"] = "10:00"
	bad["CURP"] = "short"
	recep.expect(400, "POST", "/api/appointments", bad)

	upd := map[string]any{}
	for k, v := range appt {
		upd[k] = v
	}
	upd["details"] = "Cambio"
	recep.expect(200, "PUT", "/api/appointments/"+id, upd)
	if sub(recep.expect(200, "GET", "/api/appointments/"+id, nil), "appointment")["details"] != "Cambio" {
		t.Fatal("update not applied")
	}
	recep.expect(404, "GET", "/api/appointments/not-a-uuid", nil)

	// another clinic can't see, edit or delete it
	other := e.login("recep_b")
	other.expect(404, "GET", "/api/appointments/"+id, nil)
	other.expect(404, "PUT", "/api/appointments/"+id, upd)
	other.expect(404, "DELETE", "/api/appointments/"+id, nil)
	if len(other.expect(200, "GET", "/api/appointments", nil)["appointments"].([]any)) != 0 {
		t.Fatal("tenant leak")
	}

	recep.expect(204, "DELETE", "/api/appointments/"+id, nil)
	recep.expect(404, "GET", "/api/appointments/"+id, nil)
}

var exp = map[string]any{"CURP": "mejj700312hdfdrr04", "names": "Jorge", "last_names": "Medina", "sex": "Hombre", "date_of_birth": "1970-03-12", "weight": "78", "diabetes": true}

func TestExpedientsCRUD(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")

	x := sub(doc.expect(201, "POST", "/api/expedients", exp), "expedient")
	if x["CURP"] != "MEJJ700312HDFDRR04" || x["date_of_birth"] != "1970-03-12" || x["diabetes"] != true || x["age"].(float64) < 50 {
		t.Fatalf("created: %v", x)
	}
	doc.expect(409, "POST", "/api/expedients", exp)

	invalid := map[string]any{"CURP": "MEJJ700312HDFDRR04", "names": "J", "last_names": "M", "sex": "Otro", "date_of_birth": "1970-03-12"}
	doc.expect(400, "POST", "/api/expedients", invalid)
	invalid["sex"] = "Hombre"
	invalid["date_of_birth"] = "2999-01-01"
	doc.expect(400, "POST", "/api/expedients", invalid)

	upd := map[string]any{}
	for k, v := range exp {
		upd[k] = v
	}
	upd["diabetes"] = false
	upd["allergies"] = true
	doc.expect(200, "PUT", "/api/expedients/MEJJ700312HDFDRR04", upd)
	got := sub(doc.expect(200, "GET", "/api/expedients/mejj700312hdfdrr04", nil), "expedient")
	if got["diabetes"] != false || got["allergies"] != true {
		t.Fatalf("update: %v", got)
	}

	other := e.login("doc_b")
	other.expect(404, "GET", "/api/expedients/MEJJ700312HDFDRR04", nil)
	other.expect(404, "DELETE", "/api/expedients/MEJJ700312HDFDRR04", nil)
	other.expect(201, "POST", "/api/expedients", exp) // same CURP in another clinic is fine

	doc.expect(204, "DELETE", "/api/expedients/MEJJ700312HDFDRR04", nil)
	doc.expect(404, "GET", "/api/expedients/MEJJ700312HDFDRR04", nil)
}

// ---------------------------------------------------------------------------
// Team
// ---------------------------------------------------------------------------

func member(name, user, role string) map[string]any {
	return map[string]any{"name": name, "email": user + "@nuevo.mx", "username": user, "password": "long-enough-pw", "role": role}
}

func TestTeamManagement(t *testing.T) {
	e := setup(t)
	admin := e.login("admin_a")

	list := admin.expect(200, "GET", "/api/team", nil)
	if n := len(list["people"].([]any)); n != 4 {
		t.Fatalf("team size: %d", n)
	}
	for _, p := range list["people"].([]any) {
		if _, leaked := p.(map[string]any)["password_hash"]; leaked {
			t.Fatal("password hash leaked")
		}
	}

	// create + validations
	created := sub(admin.expect(201, "POST", "/api/team", member("Laura Gómez", "laura", "reception")), "person")
	lid := created["id"].(string)
	if created["role"] != "reception" || created["role_label"] != "Recepción" {
		t.Fatalf("created: %v", created)
	}
	admin.expect(409, "POST", "/api/team", member("Otra Laura", "laura", "reception"))                                                                  // username taken
	admin.expect(409, "POST", "/api/team", func() map[string]any { m := member("Otra", "otra", "cashier"); m["email"] = "LAURA@nuevo.mx"; return m }()) // e-mail taken
	admin.expect(409, "POST", "/api/team", member("Cruzada", "admin_b", "cashier"))                                                                     // usernames are global: admin_b lives in clinic B
	admin.expect(400, "POST", "/api/team", member("X", "x1", "reception"))                                                                              // name too short
	admin.expect(400, "POST", "/api/team", member("Con Arroba", "a@b", "reception"))                                                                    // '@' not allowed in usernames
	admin.expect(400, "POST", "/api/team", member("Rol Raro", "rolraro", "platform_admin"))                                                             // clinics can't mint platform staff
	admin.expect(400, "POST", "/api/team", func() map[string]any {
		m := member("Clave Corta", "corta", "cashier")
		m["password"] = "short"
		return m
	}())

	// the new person can sign in by e-mail or username, with the permissions of their role
	laura := e.loginPw("laura@nuevo.mx", newPw)
	laura.expect(200, "GET", "/api/appointments", nil)
	laura.expect(403, "GET", "/api/team", nil)

	// role change: ends their session, takes effect after signing in again
	admin.expect(200, "PATCH", "/api/team/"+lid, map[string]any{"role": "doctor"})
	laura.expect(401, "GET", "/api/appointments", nil)
	laura = e.loginPw("laura", newPw)
	laura.expect(200, "GET", "/api/expedients", nil)
	admin.expect(400, "PATCH", "/api/team/"+lid, map[string]any{"role": "emperor"})

	// password reset by the admin ends the session and the old password stops working
	admin.expect(204, "POST", "/api/team/"+lid+"/password", map[string]any{"password": "brand-new-password"})
	laura.expect(401, "GET", "/api/expedients", nil)
	e.anon().expect(401, "POST", "/api/login", map[string]string{"identifier": "laura", "password": pw})
	e.anon().expect(401, "POST", "/api/login", map[string]string{"identifier": "laura", "password": "long-enough-pw"})
	e.anon().expect(200, "POST", "/api/login", map[string]string{"identifier": "laura", "password": "brand-new-password"})

	// deactivation locks them out at once, even with a live session
	laura = e.anon()
	laura.expect(200, "POST", "/api/login", map[string]string{"identifier": "laura", "password": "brand-new-password"})
	laura.expect(200, "GET", "/api/expedients", nil)
	admin.expect(204, "POST", "/api/team/"+lid+"/deactivate", nil)
	laura.expect(401, "GET", "/api/expedients", nil)
	e.anon().expect(403, "POST", "/api/login", map[string]string{"identifier": "laura", "password": "brand-new-password"})
	admin.expect(204, "POST", "/api/team/"+lid+"/reactivate", nil)
	e.anon().expect(200, "POST", "/api/login", map[string]string{"identifier": "laura", "password": "brand-new-password"})

	// guardrails: nobody acts on themselves; a clinic always keeps an active administrator
	me := e.userID("admin_a")
	admin.expect(400, "PATCH", "/api/team/"+me, map[string]any{"role": "doctor"})
	admin.expect(400, "POST", "/api/team/"+me+"/deactivate", nil)
	admin.expect(400, "POST", "/api/team/"+me+"/password", map[string]any{"password": "whatever-password"})

	second := sub(admin.expect(201, "POST", "/api/team", member("Segundo Admin", "admin2", "admin")), "person")
	sid := second["id"].(string)
	admin2 := e.loginPw("admin2", newPw)
	admin2.expect(200, "PATCH", "/api/team/"+me, map[string]any{"role": "doctor"}) // admin2 demotes admin_a: fine, admin2 remains
	admin.expect(401, "GET", "/api/team", nil)                                     // admin_a's session ended with the role change
	admin2.expect(400, "POST", "/api/team/"+sid+"/deactivate", nil)                // can't deactivate yourself
	admin = e.login("admin_a")                                                     // now a doctor
	admin.expect(403, "GET", "/api/team", nil)

	// other clinics are invisible and untouchable
	b := e.login("admin_b")
	b.expect(404, "PATCH", "/api/team/"+lid, map[string]any{"role": "cashier"})
	b.expect(404, "POST", "/api/team/"+lid+"/deactivate", nil)
	for _, p := range b.expect(200, "GET", "/api/team", nil)["people"].([]any) {
		if p.(map[string]any)["username"] == "laura" {
			t.Fatal("tenant leak")
		}
	}

	// every change is in the log
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM activity_log WHERE clinic_id = $1`, e.clinicA).Scan(&n); err != nil || n < 6 {
		t.Fatalf("activity log entries: %d (%v)", n, err)
	}
}

func TestLastAdminCannotBeRemoved(t *testing.T) {
	e := setup(t)
	admin := e.login("admin_a")
	doc := e.userID("doc_a")
	admin.expect(204, "POST", "/api/team/"+doc+"/deactivate", nil)

	// two admins that act on each other: the second removal must fail
	second := sub(admin.expect(201, "POST", "/api/team", member("Dos", "dos", "admin")), "person")["id"].(string)
	dos := e.loginPw("dos", newPw)
	admin.expect(204, "POST", "/api/team/"+second+"/deactivate", nil) // admin_a removes dos: ok, admin_a stays
	dos.expect(401, "GET", "/api/team", nil)
	// admin_a is now the only admin and cannot be removed by anyone else (nobody else is an admin)
	var admins int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE clinic_id = $1 AND role = 'admin' AND NOT disabled`, e.clinicA).Scan(&admins)
	if admins != 1 {
		t.Fatalf("admins left: %d", admins)
	}
}

func TestSeatLimits(t *testing.T) {
	e := setup(t)
	// A clinic on the Consultorio plan: 3 accounts, 1 doctor.
	id, err := db.CreateClinic(context.Background(), e.pool, db.ClinicParams{
		Name: "Chica", Plan: "consultorio", Status: "active",
		AdminName: "Dueña", AdminEmail: "duena@chica.mx", AdminUsername: "duena", AdminPassword: pw,
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = id
	admin := e.loginPw("duena", pw)
	seats := sub(admin.expect(200, "GET", "/api/team", nil), "seats")
	if seats["max_users"].(float64) != 3 || seats["max_doctors"].(float64) != 1 || seats["used_users"].(float64) != 1 {
		t.Fatalf("seats: %v", seats)
	}
	doc := sub(admin.expect(201, "POST", "/api/team", member("Doctora Uno", "doc1", "doctor")), "person")["id"].(string)
	out := admin.expect(409, "POST", "/api/team", member("Doctora Dos", "doc2", "doctor"))
	if out["code"] != "SEAT_LIMIT" {
		t.Fatalf("doctor limit: %v", out)
	}
	admin.expect(201, "POST", "/api/team", member("Recep", "recep1", "reception")) // third seat
	if admin.expect(409, "POST", "/api/team", member("Cajero", "cash1", "cashier"))["code"] != "SEAT_LIMIT" {
		t.Fatal("user limit")
	}
	// freeing a seat frees the room, and reactivating needs room again
	admin.expect(204, "POST", "/api/team/"+doc+"/deactivate", nil)
	cash := sub(admin.expect(201, "POST", "/api/team", member("Cajero", "cash1", "cashier")), "person")["id"].(string)
	admin.expect(409, "POST", "/api/team/"+doc+"/reactivate", nil)
	_ = cash
}

// ---------------------------------------------------------------------------
// Subscriptions
// ---------------------------------------------------------------------------

func TestSubscriptionGate(t *testing.T) {
	e := setup(t)
	root := e.login("root")
	admin := e.login("admin_a")
	admin.expect(200, "GET", "/api/appointments", nil)

	// suspended: data is blocked, but the session still works and says why
	root.expect(200, "POST", "/api/platform/clinics/"+e.clinicA+"/suspend", map[string]any{"reason": "Pago pendiente"})
	out := admin.expect(403, "GET", "/api/appointments", nil)
	if out["code"] != "SUBSCRIPTION_REQUIRED" {
		t.Fatalf("gate: %v", out)
	}
	admin.expect(403, "GET", "/api/team", nil)
	b := sub(sub(admin.expect(200, "GET", "/api/session", nil), "session"), "billing")
	if b["state"] != "suspended" || b["usable"] != false || b["suspended_reason"] != "Pago pendiente" {
		t.Fatalf("billing: %v", b)
	}
	admin.expect(200, "GET", "/api/clinic", nil)
	e.login("recep_b").expect(200, "GET", "/api/appointments", nil) // others are unaffected

	root.expect(200, "POST", "/api/platform/clinics/"+e.clinicA+"/reactivate", nil)
	admin.expect(200, "GET", "/api/appointments", nil)

	// an expired trial blocks too, and a recorded payment reopens it
	root.expect(200, "PATCH", "/api/platform/clinics/"+e.clinicA, map[string]any{"billing_status": "trialing", "trial_ends_on": "2020-01-01"})
	if sub(sub(admin.expect(200, "GET", "/api/session", nil), "session"), "billing")["state"] != "trial_expired" {
		t.Fatal("expected trial_expired")
	}
	admin.expect(403, "GET", "/api/appointments", nil)
	root.expect(201, "POST", "/api/platform/clinics/"+e.clinicA+"/payments", map[string]any{"amount": 1199, "months": 1, "note": "SPEI"})
	admin.expect(200, "GET", "/api/appointments", nil)

	// an active plan whose paid period ended long ago becomes past due by itself
	e.exec(`UPDATE clinics SET billing_status = 'active', current_period_end = now() - interval '20 days' WHERE id = $1`, e.clinicA)
	if sub(sub(admin.expect(200, "GET", "/api/session", nil), "session"), "billing")["state"] != "past_due" {
		t.Fatal("expected past_due")
	}
	admin.expect(403, "GET", "/api/appointments", nil)
}

// ---------------------------------------------------------------------------
// Platform panel
// ---------------------------------------------------------------------------

func TestPlatformPanel(t *testing.T) {
	e := setup(t)
	root, help := e.login("root"), e.login("help")
	admin := e.login("admin_a")

	// clinic accounts have no access to the platform at all
	admin.expect(403, "GET", "/api/platform/clinics", nil)
	e.anon().expect(401, "GET", "/api/platform/clinics", nil)

	// both roles can look
	for _, c := range []*client{root, help} {
		list := c.expect(200, "GET", "/api/platform/clinics", nil)
		if len(list["clinics"].([]any)) != 2 {
			t.Fatalf("clinics: %v", list)
		}
		c.expect(200, "GET", "/api/platform/clinics/"+e.clinicA, nil)
		c.expect(200, "GET", "/api/platform/plans", nil)
	}
	d := root.expect(200, "GET", "/api/platform/clinics/"+e.clinicA, nil)
	if len(d["people"].([]any)) != 4 || sub(d, "seats")["used_users"].(float64) != 4 {
		t.Fatalf("detail: %v", d)
	}
	if n := len(root.expect(200, "GET", "/api/platform/clinics?q=clinica%20b", nil)["clinics"].([]any)); n != 1 {
		t.Fatalf("search: %d", n)
	}

	// money is for administrators only; support can't change anything
	ov := root.expect(200, "GET", "/api/platform/overview", nil)
	if ov["mrr"].(float64) != 2*1199 {
		t.Fatalf("mrr: %v", ov["mrr"])
	}
	if _, has := help.expect(200, "GET", "/api/platform/overview", nil)["mrr"]; has {
		t.Fatal("support must not see revenue")
	}
	help.expect(403, "PATCH", "/api/platform/clinics/"+e.clinicA, map[string]any{"name": "Hack"})
	help.expect(403, "POST", "/api/platform/clinics/"+e.clinicA+"/suspend", map[string]any{})
	help.expect(403, "POST", "/api/platform/clinics/"+e.clinicA+"/payments", map[string]any{"amount": 1})
	help.expect(403, "GET", "/api/platform/staff", nil)
	help.expect(403, "GET", "/api/platform/activity", nil)

	// edit data and plan
	c := sub(root.expect(200, "PATCH", "/api/platform/clinics/"+e.clinicA, map[string]any{"name": "Renombrada", "kind": "DENTAL", "plan": "empresarial"}), "clinic")
	if c["name"] != "Renombrada" || c["plan"] != "empresarial" || c["kind"] != "DENTAL" {
		t.Fatalf("patch: %v", c)
	}
	root.expect(400, "PATCH", "/api/platform/clinics/"+e.clinicA, map[string]any{"plan": "gratis"})
	root.expect(400, "PATCH", "/api/platform/clinics/"+e.clinicA, map[string]any{"billing_status": "free"})
	root.expect(404, "PATCH", "/api/platform/clinics/00000000-0000-0000-0000-000000000000", map[string]any{"name": "Nadie"})

	// downgrading below the accounts in use is refused until they free seats
	root.expect(409, "PATCH", "/api/platform/clinics/"+e.clinicA, map[string]any{"plan": "consultorio"})

	// payments extend the period and show up in the detail
	root.expect(201, "POST", "/api/platform/clinics/"+e.clinicB+"/payments", map[string]any{"amount": 1199, "months": 3})
	root.expect(400, "POST", "/api/platform/clinics/"+e.clinicB+"/payments", map[string]any{"amount": -5})
	root.expect(400, "POST", "/api/platform/clinics/"+e.clinicB+"/payments", map[string]any{"amount": 10, "months": 99})
	if n := len(root.expect(200, "GET", "/api/platform/clinics/"+e.clinicB, nil)["payments"].([]any)); n != 1 {
		t.Fatalf("payments: %d", n)
	}
	if ov := root.expect(200, "GET", "/api/platform/overview", nil); ov["paid_this_month_cents"].(float64) != 119900 {
		t.Fatalf("paid this month: %v", ov["paid_this_month_cents"])
	}

	// attention queue
	e.exec(`UPDATE clinics SET billing_status = 'active', current_period_end = now() - interval '30 days' WHERE id = $1`, e.clinicA)
	att := root.expect(200, "GET", "/api/platform/overview", nil)["attention"].([]any)
	if len(att) != 1 || att[0].(map[string]any)["kind"] != "past_due" {
		t.Fatalf("attention: %v", att)
	}
	if n := len(root.expect(200, "GET", "/api/platform/clinics?state=past_due", nil)["clinics"].([]any)); n != 1 {
		t.Fatalf("state filter: %d", n)
	}

	// the activity log recorded what the administrator did
	acts := root.expect(200, "GET", "/api/platform/activity", nil)["activity"].([]any)
	if len(acts) < 2 {
		t.Fatalf("activity: %v", acts)
	}
	if acts[0].(map[string]any)["actor_name"] != "root" {
		t.Fatalf("actor: %v", acts[0])
	}
}

func TestPlatformStaff(t *testing.T) {
	e := setup(t)
	root := e.login("root")

	person := func(name, user, role string) map[string]any {
		return map[string]any{"name": name, "email": user + "@caresia.com", "username": user, "password": "long-enough-pw", "role": role}
	}
	second := sub(root.expect(201, "POST", "/api/platform/staff", person("Segundo Admin", "second", "platform_admin")), "person")
	root.expect(409, "POST", "/api/platform/staff", person("Repetido", "second", "platform_support"))
	root.expect(409, "POST", "/api/platform/staff", person("Choca con clínica", "admin_a", "platform_support"))
	root.expect(400, "POST", "/api/platform/staff", person("Rol malo", "malo", "admin"))
	root.expect(400, "POST", "/api/platform/staff", person("Rol malo", "malo", "platform_root"))

	list := root.expect(200, "GET", "/api/platform/staff", nil)["people"].([]any)
	if len(list) != 3 {
		t.Fatalf("staff: %d", len(list))
	}
	sid := second["id"].(string)
	me := e.userID("root")
	root.expect(400, "PATCH", "/api/platform/staff/"+me, map[string]any{"role": "platform_support"})
	root.expect(400, "POST", "/api/platform/staff/"+me+"/deactivate", nil)
	root.expect(404, "PATCH", "/api/platform/staff/"+e.userID("admin_a"), map[string]any{"role": "platform_support"}) // clinic accounts aren't staff

	// demoting ends the session; the new role applies after signing in again
	sec := e.loginPw("second", newPw)
	sec.expect(200, "GET", "/api/platform/staff", nil)
	root.expect(200, "PATCH", "/api/platform/staff/"+sid, map[string]any{"role": "platform_support"})
	sec.expect(401, "GET", "/api/platform/staff", nil)
	e.loginPw("second", newPw).expect(403, "GET", "/api/platform/staff", nil)

	// deactivation and password reset
	root.expect(204, "POST", "/api/platform/staff/"+sid+"/deactivate", nil)
	e.anon().expect(403, "POST", "/api/login", map[string]string{"identifier": "second", "password": newPw})
	root.expect(204, "POST", "/api/platform/staff/"+sid+"/reactivate", nil)
	root.expect(204, "POST", "/api/platform/staff/"+sid+"/password", map[string]any{"password": "another-password-1"})
	e.anon().expect(200, "POST", "/api/login", map[string]string{"identifier": "second@caresia.com", "password": "another-password-1"})
}

// ---------------------------------------------------------------------------
// Registration and account
// ---------------------------------------------------------------------------

func TestRegister(t *testing.T) {
	e := setup(t)
	anon := e.anon()
	good := map[string]any{"clinic_name": "Dental Sol", "kind": "DENTAL", "phone": "55 1", "name": "Dra. Sol", "email": "Nuevo@Sol.mx", "username": "dra.sol", "password": "una-clave-larga"}
	with := func(k string, v any) map[string]any {
		m := map[string]any{}
		for a, b := range good {
			m[a] = b
		}
		m[k] = v
		return m
	}
	anon.expect(400, "POST", "/api/register", with("kind", "BAKERY"))
	anon.expect(400, "POST", "/api/register", with("email", "no-es-correo"))
	anon.expect(400, "POST", "/api/register", with("password", "corta"))
	anon.expect(400, "POST", "/api/register", with("username", "ab"))
	anon.expect(400, "POST", "/api/register", with("username", "con espacio"))
	anon.expect(400, "POST", "/api/register", with("name", " "))
	anon.expect(400, "POST", "/api/register", with("clinic_name", " "))

	s := sub(anon.expect(200, "POST", "/api/register", good), "session")
	if s["role"] != "admin" || len(s["permissions"].([]any)) != 5 {
		t.Fatalf("session: %v", s)
	}
	b := sub(s, "billing")
	if b["state"] != "trialing" || b["plan"] != "consultorio" || b["trial_days_left"].(float64) < 13 {
		t.Fatalf("a new clinic starts a 14-day trial: %v", b)
	}
	clinic := sub(anon.expect(200, "GET", "/api/clinic", nil), "clinic")
	if clinic["kind"] != "DENTAL" || clinic["name"] != "Dental Sol" {
		t.Fatalf("clinic: %v", clinic)
	}
	if n := len(anon.expect(200, "GET", "/api/team", nil)["people"].([]any)); n != 1 {
		t.Fatalf("new clinic should only have its admin, got %d", n)
	}
	anon.expect(409, "POST", "/api/register", with("username", "otra"))  // e-mail already used
	anon.expect(409, "POST", "/api/register", with("email", "x@sol.mx")) // username already used
	e.anon().expect(200, "POST", "/api/login", map[string]string{"identifier": "nuevo@sol.mx", "password": "una-clave-larga"})
	e.anon().expect(200, "POST", "/api/login", map[string]string{"identifier": "dra.sol", "password": "una-clave-larga"})
}

func TestRegisterRateLimit(t *testing.T) {
	e := setup(t)
	anon := e.anon()
	for i := 0; i < 5; i++ {
		anon.expect(200, "POST", "/api/register", map[string]any{
			"clinic_name": "Clinica", "kind": "VETERINARY", "name": "Dueño", "email": fmt.Sprintf("c%d@x.mx", i), "username": fmt.Sprintf("dueno%d", i), "password": "una-clave-larga"})
	}
	anon.expect(429, "POST", "/api/register", map[string]any{
		"clinic_name": "Clinica", "kind": "VETERINARY", "name": "Dueño", "email": "otra@x.mx", "username": "otrodueno", "password": "una-clave-larga"})
}

func TestAccount(t *testing.T) {
	e := setup(t)
	c := e.login("doc_a")
	other := e.login("doc_a") // a second device

	s := sub(c.expect(200, "PUT", "/api/me", map[string]any{"name": "Dra. Nueva", "phone": "55 9"}), "session")
	if s["name"] != "Dra. Nueva" {
		t.Fatalf("profile: %v", s)
	}
	c.expect(400, "PUT", "/api/me", map[string]any{"name": "x"})

	c.expect(400, "PUT", "/api/me/password", map[string]any{"current_password": "wrong-wrong", "new_password": "new-password-1"})
	c.expect(400, "PUT", "/api/me/password", map[string]any{"current_password": pw, "new_password": "short"})
	c.expect(200, "PUT", "/api/me/password", map[string]any{"current_password": pw, "new_password": "new-password-1"})
	c.expect(200, "GET", "/api/session", nil)     // this session stays signed in
	other.expect(401, "GET", "/api/session", nil) // every other one ends
	e.anon().expect(401, "POST", "/api/login", map[string]string{"identifier": "doc_a", "password": pw})
	e.anon().expect(200, "POST", "/api/login", map[string]string{"identifier": "doc_a", "password": "new-password-1"})
}

func TestResetPassword(t *testing.T) {
	e := setup(t)
	c := e.login("admin_a")
	c.expect(200, "GET", "/api/session", nil)

	if _, err := db.ResetPassword(context.Background(), e.pool, "Admin_A@clinic.mx", "brand-new-pass-1"); err != nil { // by e-mail, any case
		t.Fatal(err)
	}
	c.expect(401, "GET", "/api/session", nil) // open sessions end
	e.anon().expect(401, "POST", "/api/login", map[string]string{"identifier": "admin_a", "password": pw})
	e.anon().expect(200, "POST", "/api/login", map[string]string{"identifier": "admin_a", "password": "brand-new-pass-1"})

	if _, err := db.ResetPassword(context.Background(), e.pool, "root", "short"); err == nil {
		t.Fatal("weak passwords must be refused")
	}
	if _, err := db.ResetPassword(context.Background(), e.pool, "nobody", "long-enough-pw"); err == nil {
		t.Fatal("unknown accounts must be reported")
	}
}

// ---------------------------------------------------------------------------
// Migration of data created before roles existed
// ---------------------------------------------------------------------------

func TestMigrationFromPermissions(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	// Apply only migrations 001 and 002 by hand, add legacy data, then run the full migrator.
	for _, f := range []string{"migrations/001_init.sql", "migrations/002_clinic_kind.sql"} {
		sqlBytes, err := os.ReadFile("../db/" + f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());
		INSERT INTO schema_migrations (version) VALUES ('001_init.sql'), ('002_clinic_kind.sql');
		INSERT INTO clinics (id, name, email) VALUES ('11111111-1111-1111-1111-111111111111', 'Vieja 1', 'uno@vieja.mx'), ('22222222-2222-2222-2222-222222222222', 'Vieja 2', 'dos@vieja.mx');
		INSERT INTO users (clinic_id, username, password_hash, permissions, created_at) VALUES
		 ('11111111-1111-1111-1111-111111111111', 'admin', 'x', '{adminUsers,adminAppointments,adminHistorials,navHistorials,navAppointments}', '2024-01-01'),
		 ('11111111-1111-1111-1111-111111111111', 'doc',   'x', '{navHistorials,adminHistorials}', '2024-01-02'),
		 ('11111111-1111-1111-1111-111111111111', 'recep', 'x', '{adminAppointments,navAppointments}', '2024-01-03'),
		 ('11111111-1111-1111-1111-111111111111', 'solo',  'x', '{}', '2024-01-04'),
		 ('22222222-2222-2222-2222-222222222222', 'admin', 'x', '{adminUsers}', '2024-02-01')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	roles := map[string]string{}
	rows, _ := pool.Query(ctx, `SELECT username, role FROM users WHERE clinic_id = '11111111-1111-1111-1111-111111111111'`)
	for rows.Next() {
		var u, r string
		_ = rows.Scan(&u, &r)
		roles[u] = r
	}
	if fmt.Sprint(roles) != "map[admin:admin doc:doctor recep:reception solo:cashier]" {
		t.Fatalf("roles: %v", roles)
	}
	// the second "admin" was renamed to keep usernames unique, and the first admin of each clinic got the clinic e-mail
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(DISTINCT lower(username)) FROM users`).Scan(&n)
	if n != 5 {
		t.Fatalf("usernames must be unique, got %d distinct", n)
	}
	var email string
	if err := pool.QueryRow(ctx, `SELECT email FROM users WHERE clinic_id = '11111111-1111-1111-1111-111111111111' AND role = 'admin'`).Scan(&email); err != nil || email != "uno@vieja.mx" {
		t.Fatalf("admin e-mail: %q %v", email, err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT billing_status FROM clinics WHERE name = 'Vieja 1'`).Scan(&status); err != nil || status != "active" {
		t.Fatalf("existing clinics are grandfathered as active: %q %v", status, err)
	}
	var _ pgx.Rows
}
