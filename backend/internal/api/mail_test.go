package api_test

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// fakeMailer records what the API sends.
type fakeMailer struct {
	mu      sync.Mutex
	enabled bool
	sent    []mail.Message
}

func (f *fakeMailer) Enabled() bool { return f.enabled }
func (f *fakeMailer) Send(_ context.Context, m mail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, m)
	return nil
}

// next waits for the n-th message (mail is sent in the background).
func (f *fakeMailer) wait(t *testing.T, n int) mail.Message {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		if len(f.sent) >= n {
			m := f.sent[n-1]
			f.mu.Unlock()
			return m
		}
		f.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected %d e-mail(s), got %d", n, len(f.sent))
	return mail.Message{}
}

func (f *fakeMailer) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

var tokenRe = regexp.MustCompile(`token=([A-Za-z0-9_%-]+)`)

func TestPasswordRecovery(t *testing.T) {
	e := setup(t)
	anon := e.anon()

	// unknown account: same answer, nothing sent
	anon.expect(200, "POST", "/api/forgot", map[string]any{"identifier": "nadie@nada.mx"})
	anon.expect(400, "POST", "/api/forgot", map[string]any{"identifier": " "})
	// disabled account: nothing sent either
	e.exec(`UPDATE users SET disabled = true WHERE username = 'doc_a'`)
	anon.expect(200, "POST", "/api/forgot", map[string]any{"identifier": "doc_a"})
	time.Sleep(150 * time.Millisecond)
	if e.mail.count() != 0 {
		t.Fatalf("no mail expected for unknown or disabled accounts, got %d", e.mail.count())
	}

	// real account, by username
	old := e.login("recep_a") // an open session that must die
	anon.expect(200, "POST", "/api/forgot", map[string]any{"identifier": "recep_a"})
	m := e.mail.wait(t, 1)
	if m.To[0] != "recep_a@clinic.mx" || !strings.Contains(m.Text, "/restablecer?token=") || !strings.Contains(m.HTML, "Elegir contraseña nueva") {
		t.Fatalf("recovery mail: %+v", m)
	}
	token, _ := url.QueryUnescape(tokenRe.FindStringSubmatch(m.Text)[1])

	anon.expect(400, "POST", "/api/reset-password", map[string]any{"token": token, "password": "corta"})
	anon.expect(400, "POST", "/api/reset-password", map[string]any{"token": "inventado", "password": newPw})
	anon.expect(200, "POST", "/api/reset-password", map[string]any{"token": token, "password": newPw})
	anon.expect(400, "POST", "/api/reset-password", map[string]any{"token": token, "password": "otra-clave-larga-2"}) // single use

	e.loginPw("recep_a", newPw)
	if code, _ := e.anon().do("POST", "/api/login", map[string]string{"identifier": "recep_a", "password": pw}); code != 401 {
		t.Fatalf("the old password must stop working, got %d", code)
	}
	if code := status(old, "GET", "/api/session"); code != 401 {
		t.Fatalf("an old session must end after a reset, got %d", code)
	}

	// expired link
	anon.expect(200, "POST", "/api/forgot", map[string]any{"identifier": "recep_a@clinic.mx"})
	m2 := e.mail.wait(t, 2)
	tok2, _ := url.QueryUnescape(tokenRe.FindStringSubmatch(m2.Text)[1])
	e.exec(`UPDATE password_resets SET expires_at = now() - interval '1 minute' WHERE used_at IS NULL`)
	anon.expect(400, "POST", "/api/reset-password", map[string]any{"token": tok2, "password": newPw + "x"})

	// platform admins that are permanent never get a self-service reset
	e.exec(`UPDATE users SET email = 'madgrismad@gmail.com' WHERE username = 'root'`)
	before := e.mail.count()
	anon.expect(200, "POST", "/api/forgot", map[string]any{"identifier": "root"})
	time.Sleep(150 * time.Millisecond)
	if e.mail.count() != before {
		t.Fatal("permanent admins must not receive recovery links")
	}

	// mail not configured
	e.mail.mu.Lock()
	e.mail.enabled = false
	e.mail.mu.Unlock()
	if code, out := anon.do("POST", "/api/forgot", map[string]any{"identifier": "recep_a"}); code != 503 || out["code"] != "NOT_CONFIGURED" {
		t.Fatalf("without SMTP: %d %v", code, out)
	}
}

func TestForgotRateLimit(t *testing.T) {
	e := setup(t)
	anon := e.anon()
	for i := 0; i < 5; i++ {
		anon.expect(200, "POST", "/api/forgot", map[string]any{"identifier": "x@y.mx"})
	}
	anon.expect(429, "POST", "/api/forgot", map[string]any{"identifier": "x@y.mx"})
}

func TestWelcomeAndTicketEmails(t *testing.T) {
	e := setup(t)
	anon := e.anon()
	anon.expect(200, "POST", "/api/register", map[string]any{
		"clinic_name": "Clínica Sol", "phone": "5512345678", "name": "Dra. Sol", "email": "nuevo@sol.mx", "username": "dra.sol", "password": "clave-muy-segura-1",
	})
	if m := e.mail.wait(t, 1); m.To[0] != "nuevo@sol.mx" || !strings.Contains(m.Subject, "Bienvenido") || !strings.Contains(m.Text, "Clínica Sol") {
		t.Fatalf("welcome mail: %+v", m)
	}

	// ticket by e-mail
	admin, cash := e.login("admin_a"), e.login("cash_a")
	svc := newItem(admin, map[string]any{"kind": "service", "name": "Consulta <b>", "price_cents": 50000})
	cash.expect(201, "POST", "/api/pos/cash/open", map[string]any{"opening_cents": 0})
	sid := sub(cash.expect(201, "POST", "/api/pos/sales", map[string]any{"lines": []map[string]any{{"item_id": svc, "qty": 1}},
		"payments": []map[string]any{{"method": "cash", "amount_cents": 50000}}}), "sale")["id"].(string)
	cash.expect(400, "POST", "/api/pos/sales/"+sid+"/email", map[string]any{"email": "no-es-correo"})
	cash.expect(400, "POST", "/api/pos/sales/"+sid+"/email", map[string]any{"email": "a@b.mx\r\nBcc: x@y.mx"})
	e.login("admin_b").expect(404, "POST", "/api/pos/sales/"+sid+"/email", map[string]any{"email": "a@b.mx"})
	cash.expect(200, "POST", "/api/pos/sales/"+sid+"/email", map[string]any{"email": "paciente@mail.mx"})
	m := e.mail.wait(t, 2)
	if m.To[0] != "paciente@mail.mx" || !strings.Contains(m.Text, "TOTAL  $500.00") || strings.Contains(m.HTML, "<b>") || !strings.Contains(m.HTML, "&lt;b&gt;") {
		t.Fatalf("ticket mail must show the sale and escape names: %+v", m)
	}
}
