package api_test

import (
	"strings"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

func signupBody(plan string) map[string]any {
	return map[string]any{
		"clinic_name": "Dental <Ruiz>", "phone": "664 123 4567", "name": "Ana Ruiz", "email": "ana@clinica.mx", "username": "ana.ruiz",
		"password": "una-clave-larga-123", "accepted_terms": true, "requested_plan": plan,
	}
}

// The "Solicitar demo" card says which plan the person wants to try: the account always starts on Básico, and the platform team
// gets an e-mail with the plan asked for so it can activate it.
func TestSignupNoticeCarriesTheRequestedPlan(t *testing.T) {
	for _, tc := range []struct{ asked, name string }{{"crecimiento", "Crecimiento"}, {"pro", "Pro"}, {"basico", "Básico"}} {
		t.Run(tc.asked, func(t *testing.T) {
			e := setupWith(t, func(c *config.Config) {
				c.DemoTo = "info@caresia.test"
				c.DemoCC = []string{"uno@caresia.test"}
			})
			e.anon().expect(200, "POST", "/api/register", signupBody(tc.asked))

			// the account is Básico on trial whatever was asked
			var plan, status string
			var requested *string
			if err := e.pool.QueryRow(t.Context(), `SELECT plan, billing_status, requested_plan FROM clinics WHERE name = 'Dental <Ruiz>'`).Scan(&plan, &status, &requested); err != nil {
				t.Fatal(err)
			}
			if plan != "basico" || status != "trialing" || requested == nil || *requested != tc.asked {
				t.Fatalf("plan=%q status=%q requested=%v", plan, status, requested)
			}

			// two e-mails leave: the welcome to the person and the notice to the team
			var notice string
			for n := 1; n <= 2; n++ {
				if m := e.mail.wait(t, n); len(m.To) == 1 && m.To[0] == "info@caresia.test" {
					notice = m.Subject
					if len(m.Cc) != 1 || m.Cc[0] != "uno@caresia.test" || !strings.Contains(m.ReplyTo, "ana@clinica.mx") {
						t.Fatalf("copies %v reply-to %q", m.Cc, m.ReplyTo)
					}
					if !strings.Contains(m.Text, "Plan solicitado: "+tc.name) || !strings.Contains(m.Text, "Plan activado: Básico") {
						t.Fatalf("notice text: %s", m.Text)
					}
					if strings.Contains(m.HTML, "<Ruiz>") || !strings.Contains(m.HTML, "&lt;Ruiz&gt;") {
						t.Fatalf("what the visitor typed must be escaped: %s", m.HTML)
					}
				}
			}
			if !strings.Contains(notice, "plan solicitado: "+tc.name) {
				t.Fatalf("no notice for the team (subject %q)", notice)
			}
		})
	}
}

// Without a plan ("Probar gratis") the team is told all the same; a plan nobody offers is ignored.
func TestSignupNoticeWithoutOrWithABadPlan(t *testing.T) {
	for _, asked := range []string{"", "platino"} {
		e := setupWith(t, func(c *config.Config) { c.DemoTo = "info@caresia.test" })
		e.anon().expect(200, "POST", "/api/register", signupBody(asked))
		var requested *string
		if err := e.pool.QueryRow(t.Context(), `SELECT requested_plan FROM clinics WHERE name = 'Dental <Ruiz>'`).Scan(&requested); err != nil || requested != nil {
			t.Fatalf("asked %q: requested_plan = %v (%v)", asked, requested, err)
		}
		found := false
		for n := 1; n <= 2; n++ {
			if m := e.mail.wait(t, n); len(m.To) == 1 && m.To[0] == "info@caresia.test" {
				found = strings.Contains(m.Text, "Plan solicitado: Ninguno")
			}
		}
		if !found {
			t.Fatalf("asked %q: no notice saying that no plan was asked", asked)
		}
	}
}

// The platform panel shows the plan asked for until it is activated.
func TestPlatformShowsTheRequestedPlan(t *testing.T) {
	e := setup(t)
	e.anon().expect(200, "POST", "/api/register", signupBody("crecimiento"))
	root := e.login("root")
	var id string
	if err := e.pool.QueryRow(t.Context(), `SELECT id::text FROM clinics WHERE name = 'Dental <Ruiz>'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if got := sub(root.expect(200, "GET", "/api/platform/clinics/"+id, nil), "clinic")["requested_plan"]; got != "crecimiento" {
		t.Fatalf("requested_plan = %v", got)
	}
	root.expect(200, "PATCH", "/api/platform/clinics/"+id, map[string]any{"plan": "crecimiento", "billing_status": "trialing"})
	if got := sub(root.expect(200, "GET", "/api/platform/clinics/"+id, nil), "clinic")["requested_plan"]; got != "" {
		t.Fatalf("after activating the plan: requested_plan = %v", got)
	}
}
