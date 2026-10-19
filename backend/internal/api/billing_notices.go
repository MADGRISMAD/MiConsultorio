package api

import (
	"context"
	"fmt"
	"html"
	"log"
	netmail "net/mail"
	"strings"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// Notices to the platform team about the money: the first payment of a client, each renewal, and the payments that did not
// arrive. They go to DEMO_TO (or the address the platform mail is sent from) with copy to DEMO_CC, like the sign-up notice.

// teamRecipients is where the team's notices go: DEMO_TO, or the address of the platform mail, and DEMO_CC.
func (s *Server) teamRecipients() (to string, cc []string) {
	to = s.cfg.DemoTo
	if to == "" {
		if from, err := netmail.ParseAddress(s.cfg.MailFrom); err == nil {
			to = from.Address
		}
	}
	return to, s.cfg.DemoCC
}

// billingContact is who to talk to about a clinic's payments: the clinic and its first administrator.
type billingContact struct{ Clinic, Name, Email string }

func (s *Server) billingContactOf(ctx context.Context, q queryRower, clinicID string) billingContact {
	var c billingContact
	_ = q.QueryRow(ctx, `
		SELECT c.name, coalesce(u.name, ''), coalesce(u.email, '')
		FROM clinics c LEFT JOIN LATERAL (
			SELECT name, email FROM users WHERE clinic_id = c.id AND role = 'admin' AND email IS NOT NULL AND NOT disabled ORDER BY created_at LIMIT 1
		) u ON true WHERE c.id = $1`, clinicID).Scan(&c.Clinic, &c.Name, &c.Email)
	return c
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return itoa(n) + " " + many
}

// tenureLabel says how long someone has been a client, in years and months: "2 meses", "1 año y 2 meses", "3 años".
// It is empty before the first whole month.
func tenureLabel(since, now time.Time) string {
	m := (now.Year()-since.Year())*12 + int(now.Month()) - int(since.Month())
	if now.Day() < since.Day() {
		m--
	}
	if m < 1 {
		return ""
	}
	var parts []string
	if y := m / 12; y > 0 {
		parts = append(parts, plural(y, "año", "años"))
	}
	if mo := m % 12; mo > 0 {
		parts = append(parts, plural(mo, "mes", "meses"))
	}
	return strings.Join(parts, " y ")
}

// paidHistory is what a clinic had paid before: how many payments, how many months, and since when.
type paidHistory struct {
	Payments int
	Months   int
	Since    *time.Time
}

func paidBefore(ctx context.Context, q queryRower, clinicID string) paidHistory {
	var h paidHistory
	_ = q.QueryRow(ctx, `SELECT count(*), coalesce(sum(months), 0), min(created_at) FROM payments WHERE clinic_id = $1 AND amount_cents > 0`, clinicID).
		Scan(&h.Payments, &h.Months, &h.Since)
	return h
}

// tenureLine is "Cliente nuevo", "Lleva 2 meses con nosotros" or "Lleva 1 año y 2 meses con nosotros".
func tenureLine(h paidHistory, now time.Time) string {
	if h.Since == nil {
		return "Nunca ha pagado"
	}
	if l := tenureLabel(*h.Since, now); l != "" {
		return "Lleva " + l + " con nosotros"
	}
	return "Cliente nuevo (menos de 1 mes)"
}

func (s *Server) sendTeam(subject string, lines [][2]string, footer, clinicID string) {
	to, cc := s.teamRecipients()
	if to == "" {
		return
	}
	link := s.appLink("/plataforma/negocios/" + clinicID)
	text, rows := "", ""
	for _, l := range lines {
		if l[1] == "" {
			continue
		}
		text += l[0] + ": " + l[1] + "\n"
		rows += fmt.Sprintf("<tr><td style=\"padding:4px 12px 4px 0;color:#5d6e7d\">%s</td><td style=\"padding:4px 0\">%s</td></tr>", html.EscapeString(l[0]), html.EscapeString(l[1]))
	}
	text += "\n" + footer + "\n" + link + "\n"
	body := "<table style=\"font:14px system-ui,sans-serif\">" + rows + "</table><p>" + html.EscapeString(footer) + " <a href=\"" + html.EscapeString(link) + "\">Ver el negocio en plataforma</a>.</p>"
	s.sendMail(mail.Message{To: []string{to}, Cc: cc, Subject: subject, Text: text, HTML: body})
}

// paymentTeamNotice tells the team a payment arrived: the first of a client, or a renewal. before is what the clinic had paid
// until this payment (read before recording it).
func (s *Server) paymentTeamNotice(ctx context.Context, clinicID, planName string, months, amountCents int, periodEnd time.Time, before paidHistory) {
	if amountCents <= 0 {
		return
	}
	c := s.billingContactOf(ctx, s.db, clinicID)
	first := before.Payments == 0
	kind := "Pago mensual"
	switch {
	case first:
		kind = "Primer pago"
	case months >= 12:
		kind = "Pago anual"
	}
	// someone who has paid before is counted from their first payment; the first payment starts the count now
	h := before
	if first {
		now := time.Now()
		h.Since = &now
	}
	s.sendTeam(kind+" · "+c.Clinic+" · "+planName,
		[][2]string{
			{"Nombre", c.Name}, {"Correo", c.Email}, {"Consultorio", c.Clinic}, {"Plan que paga", planName},
			{"Monto", "$" + cents(amountCents)}, {"Meses que pagó", itoa(months)},
			{"Meses pagados en total", itoa(before.Months + months)}, {"Pagado hasta", periodEnd.Format("02/01/2006")},
			{"Antigüedad", tenureLine(h, time.Now())},
		}, "Pago recibido.", clinicID)
}

// missedPaymentNotice tells the team a payment did not arrive, once per event (kind + ref).
func (s *Server) missedPaymentNotice(ctx context.Context, clinicID, kind, ref, reason string) {
	tag, err := s.db.Exec(ctx, `INSERT INTO billing_notices (clinic_id, kind, ref) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, clinicID, kind, ref)
	if err != nil || tag.RowsAffected() == 0 {
		return
	}
	c := s.billingContactOf(ctx, s.db, clinicID)
	var plan string
	_ = s.db.QueryRow(ctx, `SELECT plan FROM clinics WHERE id = $1`, clinicID).Scan(&plan)
	planName := plan
	if pl, ok := planByID(plan); ok {
		planName = pl.Name
	}
	h := paidBefore(ctx, s.db, clinicID)
	s.sendTeam("No pagó · "+c.Clinic+" · "+planName,
		[][2]string{
			{"Nombre", c.Name}, {"Correo", c.Email}, {"Consultorio", c.Clinic}, {"Plan", planName},
			{"Qué pasó", reason}, {"Meses pagados en total", itoa(h.Months)}, {"Antigüedad", tenureLine(h, time.Now())},
		}, "No llegó el pago.", clinicID)
}

// billingNoticePass looks for the payments that did not arrive: a paid period that ended (after its grace days) with no new
// payment, and a trial that ended unpaid. Only the last few days are looked at, so the first run never mails the old history.
func (s *Server) billingNoticePass(ctx context.Context) {
	notBranch := ` AND NOT (c.organization_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM organizations o WHERE o.matrix_clinic_id = c.id))`
	grace := "make_interval(days => " + itoa(pastDueGraceDays) + ")"

	rows, err := s.db.Query(ctx, `
		SELECT c.id::text, c.current_period_end, c.cancel_at_period_end FROM clinics c
		WHERE c.billing_status IN ('active', 'past_due') AND c.current_period_end IS NOT NULL
		  AND c.current_period_end + `+grace+` < now() AND c.current_period_end + `+grace+` > now() - interval '3 days'`+notBranch)
	if err != nil {
		log.Printf("billing notices: %v", err)
		return
	}
	type ended struct {
		id        string
		end       time.Time
		cancelled bool
	}
	var due []ended
	for rows.Next() {
		var e ended
		if rows.Scan(&e.id, &e.end, &e.cancelled) == nil {
			due = append(due, e)
		}
	}
	rows.Close()
	for _, e := range due {
		reason := "Terminó su periodo pagado el " + e.end.Format("02/01/2006") + " y no hubo un nuevo pago (ya pasaron los " + itoa(pastDueGraceDays) + " días de gracia)"
		if e.cancelled {
			reason = "Canceló su suscripción y terminó su periodo pagado el " + e.end.Format("02/01/2006")
		}
		s.missedPaymentNotice(ctx, e.id, "period_ended", e.end.Format(time.RFC3339), reason)
	}

	rows, err = s.db.Query(ctx, `
		SELECT c.id::text, c.trial_ends_at FROM clinics c
		WHERE c.billing_status = 'trialing' AND c.trial_ends_at < now() AND c.trial_ends_at > now() - interval '3 days'
		  AND NOT EXISTS (SELECT 1 FROM payments p WHERE p.clinic_id = c.id AND p.amount_cents > 0)`+notBranch)
	if err != nil {
		log.Printf("billing notices: %v", err)
		return
	}
	var trials []ended
	for rows.Next() {
		var e ended
		if rows.Scan(&e.id, &e.end) == nil {
			trials = append(trials, e)
		}
	}
	rows.Close()
	for _, e := range trials {
		s.missedPaymentNotice(ctx, e.id, "trial_ended", e.end.Format(time.RFC3339), "Terminó su prueba el "+e.end.Format("02/01/2006")+" sin pagar")
	}
}
