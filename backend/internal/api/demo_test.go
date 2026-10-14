package api_test

import (
	"strings"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

func TestDemoRequestNotifiesAndCopies(t *testing.T) {
	e := setupWith(t, func(c *config.Config) {
		c.DemoTo = "info@caresia.test"
		c.DemoCC = []string{"uno@caresia.test", "dos@caresia.test"}
	})
	anon := e.anon()

	status, out := anon.do("POST", "/api/public/demo", map[string]string{
		"name": "Ana Ruiz", "email": "ana@clinica.mx", "phone": "664 123 4567", "clinic": "Dental <Ruiz>", "specialty": "Odontología", "message": "Quiero ver el odontograma",
	})
	if status != 204 {
		t.Fatalf("demo: status %d %v", status, out)
	}
	m := e.mail.wait(t, 1)
	if len(m.To) != 1 || m.To[0] != "info@caresia.test" {
		t.Fatalf("destinatario: %v", m.To)
	}
	if len(m.Cc) != 2 || m.Cc[0] != "uno@caresia.test" || m.Cc[1] != "dos@caresia.test" {
		t.Fatalf("copias: %v", m.Cc)
	}
	if !strings.Contains(m.ReplyTo, "ana@clinica.mx") {
		t.Fatalf("responder a: %q", m.ReplyTo)
	}
	if !strings.Contains(m.Subject, "Ana Ruiz") || !strings.Contains(m.Text, "odontograma") {
		t.Fatalf("contenido: %q / %q", m.Subject, m.Text)
	}
	if strings.Contains(m.HTML, "<Ruiz>") || !strings.Contains(m.HTML, "&lt;Ruiz&gt;") {
		t.Fatalf("el HTML debe escapar lo que escribe el visitante: %s", m.HTML)
	}
}

func TestDemoRequestRejectsBadInputAndRobots(t *testing.T) {
	e := setup(t)
	anon := e.anon()

	for name, body := range map[string]map[string]string{
		"sin nombre":    {"email": "a@b.mx"},
		"correo malo":   {"name": "Ana", "email": "no-es-correo"},
		"sin correo":    {"name": "Ana"},
		"campo extraño": {"name": "Ana", "email": "a@b.mx", "otro": "x"},
	} {
		if status, _ := anon.do("POST", "/api/public/demo", body); status != 400 {
			t.Fatalf("%s: se esperaba 400, llegó %d", name, status)
		}
	}
	// trampa de robots: responde bien pero no manda nada
	if status, _ := anon.do("POST", "/api/public/demo", map[string]string{"name": "Bot", "email": "bot@spam.mx", "website": "http://spam"}); status != 204 {
		t.Fatalf("robot: status %d", status)
	}
	if e.mail.count() != 0 {
		t.Fatalf("no debía enviarse correo, hay %d", e.mail.count())
	}
}

func TestDemoRequestIsRateLimited(t *testing.T) {
	e := setupWith(t, func(c *config.Config) { c.DemoTo = "info@caresia.test" })
	anon := e.anon()
	var last int
	for i := 0; i < 10; i++ {
		last, _ = anon.do("POST", "/api/public/demo", map[string]string{"name": "Ana", "email": "ana@clinica.mx"})
	}
	if last != 429 {
		t.Fatalf("tras varios envíos seguidos se esperaba 429, llegó %d", last)
	}
	if n := e.mail.count(); n > 8 {
		t.Fatalf("no debían salir más de 8 correos, salieron %d", n)
	}
}
