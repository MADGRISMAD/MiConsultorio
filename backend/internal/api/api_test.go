package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
	"golang.org/x/crypto/bcrypt"
)

var allPerms = []string{"adminUsers", "adminAppointments", "adminHistorials", "navHistorials", "navAppointments"}

type env struct {
	t   *testing.T
	srv *httptest.Server
}

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
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	seed := func(email string) {
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO clinics (name, email) VALUES ($1, $2) RETURNING id`, email, email).Scan(&id); err != nil {
			t.Fatal(err)
		}
		users := map[string][]string{"admin": allPerms, "recep": {"navAppointments", "adminAppointments"}, "doc": {"navHistorials", "navAppointments"}}
		for u, perms := range users {
			if _, err := pool.Exec(ctx, `INSERT INTO users (clinic_id, username, password_hash, permissions) VALUES ($1,$2,$3,$4)`, id, u, string(hash), perms); err != nil {
				t.Fatal(err)
			}
		}
	}
	seed("a@clinic.mx")
	seed("b@clinic.mx")

	cfg := &config.Config{JWTSecret: []byte(strings.Repeat("s", 40)), SessionTTL: 3600e9}
	srv := httptest.NewServer(api.NewRouter(pool, cfg))
	t.Cleanup(srv.Close)
	return &env{t: t, srv: srv}
}

type client struct {
	e *env
	c *http.Client
}

func (e *env) login(clinicEmail, user string) *client {
	jar, _ := cookiejar.New(nil)
	c := &client{e: e, c: &http.Client{Jar: jar}}
	status, _ := c.do("POST", "/api/login", map[string]string{"email": clinicEmail, "username": user, "password": "password123"})
	if status != 200 {
		e.t.Fatalf("login %s/%s: status %d", clinicEmail, user, status)
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

func TestAuth(t *testing.T) {
	e := setup(t)
	anon := &client{e: e, c: &http.Client{}}
	anon.expect(401, "GET", "/api/session", nil)
	anon.expect(401, "GET", "/api/appointments", nil)
	anon.expect(401, "POST", "/api/login", map[string]string{"email": "a@clinic.mx", "username": "admin", "password": "wrong-password"})
	anon.expect(401, "POST", "/api/login", map[string]string{"email": "nobody@x.mx", "username": "admin", "password": "password123"})
	anon.expect(400, "POST", "/api/login", map[string]string{"email": "a@clinic.mx"})

	c := e.login("A@Clinic.mx", "admin") // email is case-insensitive
	out := c.expect(200, "GET", "/api/session", nil)
	if out["session"].(map[string]any)["username"] != "admin" {
		t.Fatalf("session: %v", out)
	}
	c.expect(204, "POST", "/api/logout", nil)
	c.expect(401, "GET", "/api/session", nil)
}

func TestLoginRateLimit(t *testing.T) {
	e := setup(t)
	anon := &client{e: e, c: &http.Client{}}
	bad := map[string]string{"email": "a@clinic.mx", "username": "admin", "password": "nope-nope"}
	for i := 0; i < 8; i++ {
		anon.expect(401, "POST", "/api/login", bad)
	}
	anon.expect(429, "POST", "/api/login", bad)
	// even the right password is blocked while locked out
	anon.expect(429, "POST", "/api/login", map[string]string{"email": "a@clinic.mx", "username": "admin", "password": "password123"})
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

func TestPermissions(t *testing.T) {
	e := setup(t)
	recep := e.login("a@clinic.mx", "recep")
	doc := e.login("a@clinic.mx", "doc")

	recep.expect(403, "GET", "/api/users", nil)
	recep.expect(403, "GET", "/api/expedients", nil)  // reception can't read clinical data
	doc.expect(200, "GET", "/api/expedients", nil)    // doctor can
	doc.expect(403, "POST", "/api/expedients", nil)   // but not edit
	doc.expect(403, "POST", "/api/appointments", nil) // nor create appointments
	recep.expect(200, "GET", "/api/appointments", nil)
}

var appt = map[string]any{"names": "Ana", "last_names": "Pérez", "CURP": "pepa800101mdfrrn01", "date": "2030-05-01", "startHour": "09:00", "endHour": "10:00", "details": "Limpieza"}

func TestAppointmentsCRUD(t *testing.T) {
	e := setup(t)
	recep := e.login("a@clinic.mx", "recep")

	out := recep.expect(201, "POST", "/api/appointments", appt)
	a := out["appointment"].(map[string]any)
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

	appt2 := map[string]any{}
	for k, v := range appt {
		appt2[k] = v
	}
	appt2["details"] = "Cambio"
	recep.expect(200, "PUT", "/api/appointments/"+id, appt2)
	got := recep.expect(200, "GET", "/api/appointments/"+id, nil)
	if got["appointment"].(map[string]any)["details"] != "Cambio" {
		t.Fatalf("update not applied: %v", got)
	}
	recep.expect(404, "GET", "/api/appointments/not-a-uuid", nil)

	// another clinic can't see, edit or delete it
	other := e.login("b@clinic.mx", "recep")
	other.expect(404, "GET", "/api/appointments/"+id, nil)
	other.expect(404, "PUT", "/api/appointments/"+id, appt2)
	other.expect(404, "DELETE", "/api/appointments/"+id, nil)
	list := other.expect(200, "GET", "/api/appointments", nil)
	if len(list["appointments"].([]any)) != 0 {
		t.Fatalf("tenant leak: %v", list)
	}

	recep.expect(204, "DELETE", "/api/appointments/"+id, nil)
	recep.expect(404, "GET", "/api/appointments/"+id, nil)
}

var exp = map[string]any{"CURP": "mejj700312hdfdrr04", "names": "Jorge", "last_names": "Medina", "sex": "Hombre", "date_of_birth": "1970-03-12", "weight": "78", "diabetes": true}

func TestExpedientsCRUD(t *testing.T) {
	e := setup(t)
	admin := e.login("a@clinic.mx", "admin")

	out := admin.expect(201, "POST", "/api/expedients", exp)
	x := out["expedient"].(map[string]any)
	if x["CURP"] != "MEJJ700312HDFDRR04" || x["date_of_birth"] != "1970-03-12" || x["diabetes"] != true || x["age"].(float64) < 50 {
		t.Fatalf("created: %v", x)
	}
	admin.expect(409, "POST", "/api/expedients", exp)

	invalid := map[string]any{"CURP": "MEJJ700312HDFDRR04", "names": "J", "last_names": "M", "sex": "Otro", "date_of_birth": "1970-03-12"}
	admin.expect(400, "POST", "/api/expedients", invalid)
	invalid["sex"] = "Hombre"
	invalid["date_of_birth"] = "2999-01-01"
	admin.expect(400, "POST", "/api/expedients", invalid)

	upd := map[string]any{}
	for k, v := range exp {
		upd[k] = v
	}
	upd["diabetes"] = false
	upd["allergies"] = true
	admin.expect(200, "PUT", "/api/expedients/MEJJ700312HDFDRR04", upd)
	got := admin.expect(200, "GET", "/api/expedients/mejj700312hdfdrr04", nil)["expedient"].(map[string]any)
	if got["diabetes"] != false || got["allergies"] != true {
		t.Fatalf("update: %v", got)
	}

	other := e.login("b@clinic.mx", "admin")
	other.expect(404, "GET", "/api/expedients/MEJJ700312HDFDRR04", nil)
	other.expect(404, "DELETE", "/api/expedients/MEJJ700312HDFDRR04", nil)
	// same CURP in another clinic is allowed
	other.expect(201, "POST", "/api/expedients", exp)

	admin.expect(204, "DELETE", "/api/expedients/MEJJ700312HDFDRR04", nil)
	admin.expect(404, "GET", "/api/expedients/MEJJ700312HDFDRR04", nil)
}

func TestUserManagement(t *testing.T) {
	e := setup(t)
	admin := e.login("a@clinic.mx", "admin")

	users := admin.expect(200, "GET", "/api/users", nil)["users"].([]any)
	if len(users) != 3 {
		t.Fatalf("users: %v", users)
	}
	if _, leaked := users[0].(map[string]any)["password_hash"]; leaked {
		t.Fatal("password hash leaked")
	}

	admin.expect(201, "POST", "/api/users", map[string]any{"username": "nuevo", "password": "long-enough-pw", "permissions": []string{"navAppointments"}})
	admin.expect(409, "POST", "/api/users", map[string]any{"username": "nuevo", "password": "long-enough-pw", "permissions": []string{}})
	admin.expect(400, "POST", "/api/users", map[string]any{"username": "x", "password": "short", "permissions": []string{}})
	admin.expect(400, "POST", "/api/users", map[string]any{"username": "x", "password": "long-enough-pw", "permissions": []string{"root"}})

	// new user can sign in; password change takes effect
	admin.expect(204, "PUT", "/api/users/nuevo", map[string]any{"password": "another-password"})
	anon := &client{e: e, c: &http.Client{}}
	anon.expect(401, "POST", "/api/login", map[string]string{"email": "a@clinic.mx", "username": "nuevo", "password": "long-enough-pw"})
	anon.expect(200, "POST", "/api/login", map[string]string{"email": "a@clinic.mx", "username": "nuevo", "password": "another-password"})

	// guardrails
	admin.expect(400, "DELETE", "/api/users/admin", nil) // self
	admin.expect(204, "PUT", "/api/users/recep", map[string]any{"permissions": []string{"adminUsers"}})
	admin.expect(400, "DELETE", "/api/users/recep", nil) // holds adminUsers
	admin.expect(404, "DELETE", "/api/users/ghost", nil)
	admin.expect(204, "DELETE", "/api/users/nuevo", nil)

	// can't strip the last user-admin, atomically across a batch
	admin.expect(400, "PUT", "/api/users/permissions", map[string]any{"users": map[string][]string{"admin": {"navHistorials"}, "recep": {}}})
	admin.expect(204, "PUT", "/api/users/permissions", map[string]any{"users": map[string][]string{"admin": {"navHistorials"}}}) // recep still has adminUsers
	admin.expect(403, "GET", "/api/users", nil)                                                                                  // admin lost the permission instantly

	// users are scoped per clinic
	b := e.login("b@clinic.mx", "admin")
	for _, u := range b.expect(200, "GET", "/api/users", nil)["users"].([]any) {
		if u.(map[string]any)["username"] == "nuevo" {
			t.Fatal("tenant leak")
		}
	}
	b.expect(404, "DELETE", "/api/users/doc-nope", nil)
}

func TestDeletedUserSessionRevoked(t *testing.T) {
	e := setup(t)
	admin := e.login("a@clinic.mx", "admin")
	doc := e.login("a@clinic.mx", "doc")
	doc.expect(200, "GET", "/api/session", nil)
	admin.expect(204, "DELETE", "/api/users/doc", nil)
	doc.expect(401, "GET", "/api/session", nil)
}
