package api_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

var sixDigits = regexp.MustCompile(`\n(\d{6})\n`)

func verifyEnv(t *testing.T) *env {
	return setupWith(t, func(c *config.Config) { c.SignupVerify = true })
}

// requestCode asks for the code of an address and returns the one that was mailed.
func requestCode(t *testing.T, e *env, email string) string {
	t.Helper()
	e.mail.mu.Lock()
	before := len(e.mail.sent)
	e.mail.mu.Unlock()
	e.anon().expect(200, "POST", "/api/register/code", map[string]any{"email": email})
	m := e.mail.wait(t, before+1)
	if m.To[0] != email || !strings.HasSuffix(strings.Split(m.Subject, " es tu")[0], m.Subject[:6]) {
		t.Fatalf("mail: to %v subject %q", m.To, m.Subject)
	}
	g := sixDigits.FindStringSubmatch(m.Text)
	if g == nil {
		t.Fatalf("no code in the mail:\n%s", m.Text)
	}
	return g[1]
}

func signupWith(email, user, code string) map[string]any {
	b := signupBody("")
	b["email"], b["username"] = email, user
	if code != "" {
		b["email_code"] = code
	}
	return b
}

func wrongCode(code string) string {
	if code == "000000" {
		return "111111"
	}
	return "000000"
}

// The account is only created with the code mailed to the address.
func TestSignupNeedsTheEmailCode(t *testing.T) {
	e := verifyEnv(t)
	anon := e.anon()
	code := requestCode(t, e, "ana@clinica.mx")

	if out := anon.expect(400, "POST", "/api/register", signupWith("ana@clinica.mx", "ana.uno", "")); out["code"] != "EMAIL_CODE_REQUIRED" {
		t.Fatalf("without a code: %v", out)
	}
	if out := anon.expect(400, "POST", "/api/register", signupWith("ana@clinica.mx", "ana.uno", wrongCode(code))); out["code"] != "EMAIL_CODE_WRONG" {
		t.Fatalf("wrong code: %v", out)
	}
	// the code of another address does not work either
	other := requestCode(t, e, "beto@clinica.mx")
	if other != code {
		if out := anon.expect(400, "POST", "/api/register", signupWith("ana@clinica.mx", "ana.uno", other)); out["code"] != "EMAIL_CODE_WRONG" {
			t.Fatalf("the code of another address: %v", out)
		}
	}
	var n int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM users WHERE lower(email) = 'ana@clinica.mx'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("nothing is created without the code: %d (%v)", n, err)
	}

	// a field error does not burn the code: a taken user name is fixed and tried again with the same one
	anon.expect(409, "POST", "/api/register", signupWith("ana@clinica.mx", "admin_a", code))
	anon.expect(200, "POST", "/api/register", signupWith("ana@clinica.mx", "ana.uno", code))
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM users WHERE lower(email) = 'ana@clinica.mx'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("the account exists: %d (%v)", n, err)
	}

	// the code is spent with the account
	var left int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM email_codes WHERE email = 'ana@clinica.mx'`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("the code must be gone: %d (%v)", left, err)
	}
	// and what is kept is not the code in the clear
	var hash string
	if err := e.pool.QueryRow(t.Context(), `SELECT code_hash FROM email_codes WHERE email = 'beto@clinica.mx'`).Scan(&hash); err != nil || strings.Contains(hash, other) {
		t.Fatalf("hash %q (%v)", hash, err)
	}
	// an address that already has an account gets no code
	anon.expect(409, "POST", "/api/register/code", map[string]any{"email": "ana@clinica.mx"})
}

func TestSignupCodeExpiresAndDiesAfterFiveWrongTries(t *testing.T) {
	e := verifyEnv(t)
	anon := e.anon()

	code := requestCode(t, e, "ana@clinica.mx")
	e.exec(`UPDATE email_codes SET expires_at = now() - interval '1 second' WHERE email = 'ana@clinica.mx'`)
	if out := anon.expect(400, "POST", "/api/register", signupWith("ana@clinica.mx", "ana.uno", code)); out["code"] != "EMAIL_CODE_EXPIRED" {
		t.Fatalf("expired: %v", out)
	}

	e.exec(`UPDATE email_codes SET sent_at = now() - interval '2 minutes' WHERE email = 'ana@clinica.mx'`)
	code = requestCode(t, e, "ana@clinica.mx")
	for i := 1; i < 5; i++ {
		if out := anon.expect(400, "POST", "/api/register", signupWith("ana@clinica.mx", "ana.uno", wrongCode(code))); out["code"] != "EMAIL_CODE_WRONG" {
			t.Fatalf("try %d: %v", i, out)
		}
	}
	if out := anon.expect(400, "POST", "/api/register", signupWith("ana@clinica.mx", "ana.uno", wrongCode(code))); out["code"] != "EMAIL_CODE_EXPIRED" {
		t.Fatalf("the fifth wrong try kills the code: %v", out)
	}
	// even the right code no longer works: a new one has to be asked for
	if out := anon.expect(400, "POST", "/api/register", signupWith("ana@clinica.mx", "ana.uno", code)); out["code"] != "EMAIL_CODE_EXPIRED" {
		t.Fatalf("after the tries are gone: %v", out)
	}
}

func TestSignupCodeResendLimits(t *testing.T) {
	e := verifyEnv(t)
	anon := e.anon()
	requestCode(t, e, "ana@clinica.mx")
	anon.expect(429, "POST", "/api/register/code", map[string]any{"email": "ana@clinica.mx"}) // wait a minute between sends

	// five sends in the last hour is the limit
	e.exec(`UPDATE email_codes SET sent_at = now() - interval '2 minutes', sends = ARRAY[now(), now(), now(), now(), now()] WHERE email = 'ana@clinica.mx'`)
	out := anon.expect(429, "POST", "/api/register/code", map[string]any{"email": "ana@clinica.mx"})
	if !strings.Contains(out["message"].(string), "varios códigos") {
		t.Fatalf("hourly cap: %v", out)
	}
	anon.expect(400, "POST", "/api/register/code", map[string]any{"email": "no-es-correo"})

	// a request per address, but the same network cannot ask for more than ten codes an hour
	for i := 0; i < 12; i++ {
		anon.do("POST", "/api/register/code", map[string]any{"email": "x" + string(rune('a'+i)) + "@clinica.mx"})
	}
	if status, _ := anon.do("POST", "/api/register/code", map[string]any{"email": "zz@clinica.mx"}); status != 429 {
		t.Fatalf("per-network limit: %d", status)
	}
}

// Without outgoing mail, or with SIGNUP_VERIFY=off, there is no code to ask for.
func TestSignupWithoutVerification(t *testing.T) {
	e := setup(t) // SignupVerify is off
	out := e.anon().expect(200, "POST", "/api/register/code", map[string]any{"email": "ana@clinica.mx"})
	if out["required"] != false {
		t.Fatalf("not required: %v", out)
	}
	e.anon().expect(200, "POST", "/api/register", signupWith("ana@clinica.mx", "ana.uno", ""))

	e2 := verifyEnv(t)
	e2.mail.enabled = false // the server cannot send mail
	if out := e2.anon().expect(200, "POST", "/api/register/code", map[string]any{"email": "ana@clinica.mx"}); out["required"] != false {
		t.Fatalf("without mail: %v", out)
	}
	e2.anon().expect(200, "POST", "/api/register", signupWith("ana@clinica.mx", "ana.uno", ""))
}

// If the mail does not leave, no code is left waiting.
func TestSignupCodeMailDown(t *testing.T) {
	e := verifyEnv(t)
	e.mail.failing = true
	e.anon().expect(502, "POST", "/api/register/code", map[string]any{"email": "ana@clinica.mx"})
	var n int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM email_codes`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("no code must be kept: %d (%v)", n, err)
	}
}
