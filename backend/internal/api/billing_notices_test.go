package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

func TestTenureLabel(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	at := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 9, 0, 0, 0, time.UTC) }
	for _, c := range []struct {
		since time.Time
		want  string
	}{
		{at(2026, 10, 1), ""},                // less than a month
		{at(2026, 9, 21), ""},                // one day short of the month
		{at(2026, 9, 20), "1 mes"},           // one month
		{at(2026, 8, 5), "2 meses"},          //
		{at(2025, 8, 20), "1 año y 2 meses"}, // the example of the request
		{at(2025, 10, 20), "1 año"},          // exactly a year
		{at(2023, 7, 3), "3 años y 3 meses"}, //
		{at(2024, 10, 20), "2 años"},         //
	} {
		if got := api.TenureLabel(c.since, now); got != c.want {
			t.Errorf("%v: %q, want %q", c.since.Format("2006-01-02"), got, c.want)
		}
	}
}

// billingNoticeEnv: a clinic of the platform's with a Mercado Pago subscription; the team's notices go to info@caresia.test.
func billingNoticeEnv(t *testing.T) (*env, *fakeMP, *config.Config, func(typ, id string)) {
	fake, srv := newFakeMP(t)
	var cfg *config.Config
	e := setupWith(t, func(c *config.Config) {
		cfg = c
		c.MPAccessToken, c.MPAPIBase = "platform-token", srv.URL
		c.PlanPriceMonth["crecimiento"] = 600
		c.DemoTo = "info@caresia.test"
		c.DemoCC = []string{"uno@caresia.test"}
	})
	webhook := func(typ, id string) {
		body, _ := json.Marshal(map[string]any{"type": typ, "data": map[string]any{"id": id}})
		res, err := http.Post(e.srv.URL+"/api/webhooks/mercadopago", "application/json", strings.NewReader(string(body)))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
	}
	return e, fake, cfg, webhook
}

// teamMails are the notices the team got (the ones sent to DEMO_TO), after the mail that is sent in the background settled.
func teamMails(e *env) []mail.Message {
	time.Sleep(400 * time.Millisecond)
	e.mail.mu.Lock()
	defer e.mail.mu.Unlock()
	var out []mail.Message
	for _, m := range e.mail.sent {
		if len(m.To) == 1 && m.To[0] == "info@caresia.test" {
			out = append(out, m)
		}
	}
	return out
}

// The team is told of the first payment of a client, of each renewal (with how long they have been a client) and when a charge fails.
func TestTeamIsToldOfFirstPaymentRenewalsAndFailures(t *testing.T) {
	e, fake, _, webhook := billingNoticeEnv(t)
	admin := e.login("admin_a")
	e.exec(`UPDATE clinics SET plan='basico', billing_status='trialing', trial_ends_at=now() + interval '5 days' WHERE id=$1`, e.clinicA)
	co := admin.expect(201, "POST", "/api/billing/checkout", map[string]any{"plan": "crecimiento", "period": "month"})
	_ = co
	next := time.Now().AddDate(0, 1, 0)
	fake.setPre("pre-1", "authorized", next)
	admin.expect(200, "GET", "/api/billing/checkouts/"+co["id"].(string), nil)

	// first payment
	fake.addCharge("8001", "pre-1", 600, "processed")
	webhook("subscription_authorized_payment", "8001")
	mails := teamMails(e)
	if len(mails) != 1 {
		t.Fatalf("one notice for the first payment, got %d", len(mails))
	}
	m := mails[0]
	for _, want := range []string{"Nombre: ", "Correo: admin_a@clinic.mx", "Plan que paga: Crecimiento", "Monto: $600", "Meses que pagó: 1", "Meses pagados en total: 1", "Antigüedad: Cliente nuevo"} {
		if !strings.Contains(m.Text, want) {
			t.Fatalf("first payment notice lacks %q:\n%s", want, m.Text)
		}
	}
	if !strings.HasPrefix(m.Subject, "Primer pago") || len(m.Cc) != 1 || m.Cc[0] != "uno@caresia.test" {
		t.Fatalf("subject %q copies %v", m.Subject, m.Cc)
	}
	webhook("subscription_authorized_payment", "8001") // a repeated warning does not mail again
	if n := len(teamMails(e)); n != 1 {
		t.Fatalf("a repeated charge must not repeat the notice: %d", n)
	}

	// the client has been paying for 14 months: the next renewal says "1 año y 2 meses"
	e.exec(`UPDATE payments SET created_at = now() - interval '14 months' WHERE clinic_id = $1`, e.clinicA)
	fake.setPre("pre-1", "authorized", next.AddDate(0, 1, 0))
	fake.addCharge("8002", "pre-1", 600, "processed")
	webhook("subscription_authorized_payment", "8002")
	mails = teamMails(e)
	if len(mails) != 2 {
		t.Fatalf("a notice for the renewal, got %d", len(mails))
	}
	if !strings.HasPrefix(mails[1].Subject, "Pago mensual") || !strings.Contains(mails[1].Text, "Antigüedad: Lleva 1 año y 2 meses con nosotros") || !strings.Contains(mails[1].Text, "Meses pagados en total: 2") {
		t.Fatalf("renewal notice:\n%s %s", mails[1].Subject, mails[1].Text)
	}

	// a failed charge (the subscription is paused): the team hears once
	fake.setPre("pre-1", "paused", next.AddDate(0, 2, 0))
	webhook("subscription_preapproval", "pre-1")
	webhook("subscription_preapproval", "pre-1")
	mails = teamMails(e)
	if len(mails) != 3 {
		t.Fatalf("a notice for the failed charge, got %d", len(mails))
	}
	if !strings.HasPrefix(mails[2].Subject, "No pagó") || !strings.Contains(mails[2].Text, "Falló el cobro") || !strings.Contains(mails[2].Text, "Antigüedad: Lleva 1 año y 2 meses con nosotros") {
		t.Fatalf("failed charge notice:\n%s %s", mails[2].Subject, mails[2].Text)
	}
}

// A paid period that ended (after its grace days) and a trial that ended unpaid are told once; old history is not mailed.
func TestTeamIsToldWhenAPeriodOrTrialEndsUnpaid(t *testing.T) {
	e, _, cfg, _ := billingNoticeEnv(t)
	run := func() { api.RunBillingNotices(t.Context(), e.pool, cfg, e.mail) }

	// clinic A: paid, the period ended four days ago (past the 3 days of grace); clinic B: the trial ended yesterday
	e.exec(`UPDATE clinics SET billing_status='active', current_period_end = now() - interval '4 days' WHERE id=$1`, e.clinicA)
	e.exec(`UPDATE clinics SET billing_status='trialing', trial_ends_at = now() - interval '1 day' WHERE id=$1`, e.clinicB)
	e.exec(`INSERT INTO payments (clinic_id, amount_cents, plan, months, period_end, created_at) VALUES ($1, 60000, 'crecimiento', 1, now() - interval '4 days', now() - interval '5 months')`, e.clinicA)
	run()
	mails := teamMails(e)
	if len(mails) != 2 {
		t.Fatalf("one notice per clinic, got %d", len(mails))
	}
	var paid, trial string
	for _, m := range mails {
		if strings.Contains(m.Text, "Terminó su periodo pagado") {
			paid = m.Text
		}
		if strings.Contains(m.Text, "Terminó su prueba") {
			trial = m.Text
		}
	}
	if !strings.Contains(paid, "Antigüedad: Lleva 5 meses con nosotros") || !strings.Contains(paid, "Meses pagados en total: 1") {
		t.Fatalf("period ended:\n%s", paid)
	}
	if !strings.Contains(trial, "Antigüedad: Nunca ha pagado") {
		t.Fatalf("trial ended:\n%s", trial)
	}
	run() // the same events are not told twice
	if n := len(teamMails(e)); n != 2 {
		t.Fatalf("the second pass must not repeat: %d", n)
	}

	// a period that ended long ago is history: not mailed
	e.exec(`DELETE FROM billing_notices`)
	e.exec(`UPDATE clinics SET current_period_end = now() - interval '40 days' WHERE id=$1`, e.clinicA)
	e.exec(`UPDATE clinics SET trial_ends_at = now() - interval '30 days' WHERE id=$1`, e.clinicB)
	run()
	if n := len(teamMails(e)); n != 2 {
		t.Fatalf("history must not be mailed: %d", n)
	}
	// and one still inside the grace days is not an unpaid one yet
	e.exec(`UPDATE clinics SET current_period_end = now() - interval '1 day' WHERE id=$1`, e.clinicA)
	run()
	if n := len(teamMails(e)); n != 2 {
		t.Fatalf("grace days: %d", n)
	}
}
