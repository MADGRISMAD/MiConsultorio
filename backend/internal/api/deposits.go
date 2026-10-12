package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// Cobro anticipado: a priced service booked online can be held with a percentage paid up front through the
// clinic's own Mercado Pago account. The appointment waits as pending_payment (it already holds the time) and
// becomes a normal scheduled one when the payment is approved; unpaid ones are released after depositHold.
const depositHold = 30 * time.Minute

// depositFor is the amount to charge up front for a booking of this service, or 0 when none applies: the service has
// no price, the percentage is 0, the plan has no cobros or the clinic has not connected Mercado Pago.
func (s *Server) depositFor(ctx context.Context, clinicID, serviceID string) (cents int, title string) {
	if serviceID == "" || !planOf(ctx, s.db, clinicID).Cobros {
		return 0, ""
	}
	var price, percent int
	if s.db.QueryRow(ctx, `
		SELECT ci.price_cents, coalesce(ci.deposit_percent, a.deposit_percent), ci.name
		FROM catalog_items ci JOIN agenda_settings a ON a.clinic_id = ci.clinic_id
		WHERE ci.id = $1 AND ci.clinic_id = $2 AND ci.kind = 'service'`, serviceID, clinicID).Scan(&price, &percent, &title) != nil {
		return 0, ""
	}
	if price <= 0 || percent <= 0 {
		return 0, ""
	}
	if _, err := s.clinicToken(ctx, clinicID); err != nil {
		return 0, ""
	}
	return depositAmount(price, percent), title
}

// depositAmount is the part of a price asked up front (Mercado Pago does not take less than a peso).
func depositAmount(price, percent int) int {
	cents := price * percent / 100
	if cents < 100 {
		cents = min(100, price)
	}
	return cents
}

// createDepositCharge opens the Checkout Pro payment for a pending appointment and returns where to pay.
func (s *Server) createDepositCharge(ctx context.Context, clinicID, apptID, token string, cents int, title string) (string, error) {
	mp, err := s.clinicToken(ctx, clinicID)
	if err != nil {
		return "", err
	}
	ref := "caresia-dep:" + apptID
	back := s.manageLink(token, "")
	pref := map[string]any{
		"items":              []map[string]any{{"title": "Anticipo: " + title, "quantity": 1, "currency_id": s.cfg.MPCurrency, "unit_price": float64(cents) / 100}},
		"external_reference": ref,
		"back_urls":          map[string]string{"success": back, "pending": back, "failure": back},
		"expires":            true,
		"expiration_date_to": time.Now().Add(depositHold).UTC().Format("2006-01-02T15:04:05.000-00:00"),
	}
	if s.cfg.AppURL != "" && len(s.cfg.AppURL) > 8 && s.cfg.AppURL[:8] == "https://" {
		pref["auto_return"] = "approved"
	}
	if s.cfg.APIPublicURL != "" {
		pref["notification_url"] = s.cfg.APIPublicURL + "/api/webhooks/mercadopago?dep=" + url.QueryEscape(apptID)
	}
	var out struct {
		ID               string `json:"id"`
		InitPoint        string `json:"init_point"`
		SandboxInitPoint string `json:"sandbox_init_point"`
	}
	if err := s.mpCall(ctx, mp, http.MethodPost, "/checkout/preferences", pref, &out); err != nil {
		return "", err
	}
	pay := out.InitPoint
	if s.cfg.MPSandbox && out.SandboxInitPoint != "" {
		pay = out.SandboxInitPoint
	}
	err = inTx(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO mp_charges (id, clinic_id, kind, amount_cents, external_ref, pay_url) VALUES ($1,$2,'link',$3,$4,$5)`,
			out.ID, clinicID, cents, ref, pay); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE appointments SET deposit_charge_id = $3 WHERE clinic_id = $1 AND id = $2`, clinicID, apptID, out.ID)
		return err
	})
	return pay, err
}

// depositPayURL is the open payment link of a pending appointment (empty when there is none).
func (s *Server) depositPayURL(ctx context.Context, clinicID, apptID string) string {
	var u string
	_ = s.db.QueryRow(ctx, `
		SELECT coalesce(c.pay_url, '') FROM appointments a JOIN mp_charges c ON c.id = a.deposit_charge_id AND c.clinic_id = a.clinic_id
		WHERE a.clinic_id = $1 AND a.id = $2 AND a.status = 'pending_payment'`, clinicID, apptID).Scan(&u)
	return u
}

// settleDeposit asks Mercado Pago whether a pending appointment was paid and, if so, confirms it.
// It is safe to call as often as needed (webhook, the patient's page, the sweep).
func (s *Server) settleDeposit(ctx context.Context, apptID string) {
	if !validUUID(apptID) {
		return
	}
	var clinicID, status, state string
	var chargeID *string
	if s.db.QueryRow(ctx, `SELECT clinic_id::text, status, deposit_state, deposit_charge_id FROM appointments WHERE id = $1`, apptID).Scan(&clinicID, &status, &state, &chargeID) != nil || chargeID == nil {
		return
	}
	if state != "pending" {
		return
	}
	ch, ref, err := s.loadCharge(ctx, clinicID, *chargeID)
	if err != nil {
		return
	}
	ch, err = s.syncCharge(ctx, clinicID, ch, ref)
	if err != nil || ch.Status != "approved" {
		return
	}
	if status == "pending_payment" {
		s.activateDeposit(ctx, clinicID, apptID)
		return
	}
	// paid after the time was released (or the appointment was cancelled meanwhile): the money goes back
	if err := s.refundDeposit(ctx, clinicID, apptID); err != nil {
		log.Printf("deposit refund %s: %v", apptID, err)
	}
}

// activateDeposit turns a paid pending appointment into a scheduled one and does what a new booking does.
func (s *Server) activateDeposit(ctx context.Context, clinicID, apptID string) {
	var token, date, start, end, names, last, pro, email, phone string
	activated := false
	err := inTx(ctx, s.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE appointments SET status = 'scheduled', deposit_state = 'paid', deposit_expires_at = NULL, updated_at = now()
			WHERE clinic_id = $1 AND id = $2 AND status = 'pending_payment'`, clinicID, apptID)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		activated = true
		if err := tx.QueryRow(ctx, `
			SELECT coalesce(confirm_token, ''), to_char(date, 'YYYY-MM-DD'), to_char(start_hour, 'HH24:MI'), to_char(end_hour, 'HH24:MI'),
			       names, last_names, coalesce(professional_id::text, ''), email, phone
			FROM appointments WHERE clinic_id = $1 AND id = $2`, clinicID, apptID).Scan(&token, &date, &start, &end, &names, &last, &pro, &email, &phone); err != nil {
			return err
		}
		audit(ctx, tx, clinicID, nil, "appointment_deposit_paid", "El paciente pagó el anticipo de su cita", map[string]any{"appointment_id": apptID})
		s.ntfAppointment(ctx, tx, clinicID, pro, "booking_new", "Nueva cita por reserva en línea",
			names+" "+last+" · "+date+" "+start+" · anticipo pagado", "/admin/navegar-citas")
		return s.scheduleReminders(ctx, tx, clinicID, apptID)
	})
	if err != nil {
		log.Printf("deposit activate %s: %v", apptID, err)
		return
	}
	if !activated {
		return
	}
	b := &bookingAPI{Server: s}
	slug := ""
	_ = s.db.QueryRow(ctx, `SELECT coalesce(booking_slug, '') FROM agenda_settings WHERE clinic_id = $1`, clinicID).Scan(&slug)
	c, err := b.loadBookingClinic(ctx, slug)
	if err != nil {
		return
	}
	if info, ok, err := b.loadApptInfo(ctx, s.db, clinicID, apptID); err == nil && ok && email != "" {
		subject, text, html := b.bookingMail(info, c.RequiresConfirmation)
		b.sendMail(mail.Message{To: []string{email}, Subject: subject, Text: text, HTML: html, Attachments: info.calendarAttachments(c.RequiresConfirmation, b.manageLink(info.Token, ""))})
	}
	if pros, err := b.listBookable(ctx, c, pro); err == nil && len(pros) == 1 {
		b.mailSpecialist(ctx, c, pros[0], bookRequest{Names: names, LastNames: last, Date: date, Start: start}, end)
	}
}

// refundDeposit gives the deposit back through Mercado Pago and marks it.
func (s *Server) refundDeposit(ctx context.Context, clinicID, apptID string) error {
	var chargeID *string
	var state string
	if err := s.db.QueryRow(ctx, `SELECT deposit_charge_id, deposit_state FROM appointments WHERE clinic_id = $1 AND id = $2`, clinicID, apptID).Scan(&chargeID, &state); err != nil {
		return err
	}
	if chargeID == nil || (state != "paid" && state != "pending") {
		return nil
	}
	var pay string
	var refunded *time.Time
	var status string
	if err := s.db.QueryRow(ctx, `SELECT status, coalesce(mp_payment_id, ''), refunded_at FROM mp_charges WHERE clinic_id = $1 AND id = $2`, clinicID, *chargeID).Scan(&status, &pay, &refunded); err != nil {
		return err
	}
	if status == "approved" && refunded == nil {
		tok, err := s.clinicToken(ctx, clinicID)
		if err != nil {
			return err
		}
		if err := s.mpCall(ctx, tok, http.MethodPost, "/v1/payments/"+url.PathEscape(pay)+"/refunds", nil, nil); err != nil {
			var me *mpError
			if !(errors.As(err, &me) && refundedRe.MatchString(me.Msg)) {
				return err
			}
		}
		if _, err := s.db.Exec(ctx, `UPDATE mp_charges SET refunded_at = now() WHERE clinic_id = $1 AND id = $2`, clinicID, *chargeID); err != nil {
			return err
		}
		_, err = s.db.Exec(ctx, `UPDATE appointments SET deposit_state = 'refunded' WHERE clinic_id = $1 AND id = $2`, clinicID, apptID)
		return err
	}
	if status != "approved" { // nothing was ever paid
		_, err := s.db.Exec(ctx, `UPDATE appointments SET deposit_state = 'none' WHERE clinic_id = $1 AND id = $2 AND deposit_state = 'pending'`, clinicID, apptID)
		return err
	}
	return nil
}

// depositPass releases the time of appointments whose deposit was not paid in time. A payment that landed
// but whose notice did not is found first.
func (s *Server) depositPass(ctx context.Context) {
	rows, err := s.db.Query(ctx, `SELECT clinic_id::text, id::text FROM appointments WHERE status = 'pending_payment' AND deposit_expires_at < now() + interval '1 minute' LIMIT 100`)
	if err != nil {
		return
	}
	type ap struct{ clinic, id string }
	var list []ap
	for rows.Next() {
		var a ap
		if rows.Scan(&a.clinic, &a.id) == nil {
			list = append(list, a)
		}
	}
	rows.Close()
	for _, a := range list {
		s.settleDeposit(ctx, a.id)
		var expired bool
		_ = s.db.QueryRow(ctx, `SELECT status = 'pending_payment' AND deposit_expires_at < now() FROM appointments WHERE id = $1`, a.id).Scan(&expired)
		if !expired {
			continue
		}
		if tag, err := s.db.Exec(ctx, `UPDATE appointments SET status = 'cancelled', cancel_reason = 'No se pagó el anticipo a tiempo', updated_at = now()
			WHERE clinic_id = $1 AND id = $2 AND status = 'pending_payment'`, a.clinic, a.id); err == nil && tag.RowsAffected() == 1 {
			waitlistWake()
		}
	}
}

// onDepositCancel decides what happens to a deposit when the appointment is cancelled: it goes back unless the
// clinic keeps it (patient cancelled too late is refused earlier, so only staff decisions reach here with keep).
func (s *Server) onDepositCancel(ctx context.Context, clinicID, apptID string, keep bool) {
	if keep {
		_, _ = s.db.Exec(ctx, `UPDATE appointments SET deposit_state = 'kept' WHERE clinic_id = $1 AND id = $2 AND deposit_state = 'paid'`, clinicID, apptID)
		return
	}
	if err := s.refundDeposit(ctx, clinicID, apptID); err != nil {
		log.Printf("deposit refund %s: %v", apptID, err)
	}
}

var refundedRe = regexp.MustCompile(`(?i)already.*refund|ya.*reembols|fully refunded`)

// appointmentDeposit is GET /pos/appointments/{id}/deposit: the anticipo paid online for a visit that is not yet
// used in a sale, so the cashier can apply it as a payment.
func (s *Server) appointmentDeposit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	p := principalFrom(r.Context())
	var charge string
	var cents int
	err := s.db.QueryRow(r.Context(), `
		SELECT c.id, c.amount_cents FROM appointments a JOIN mp_charges c ON c.id = a.deposit_charge_id AND c.clinic_id = a.clinic_id
		WHERE a.clinic_id = $1 AND a.id = $2 AND a.deposit_state = 'paid' AND c.status = 'approved' AND c.sale_id IS NULL AND c.refunded_at IS NULL`,
		p.ClinicID, id).Scan(&charge, &cents)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]any{"deposit": nil})
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deposit": map[string]any{"charge_id": charge, "cents": cents}})
}

// RunDepositsOnce releases the unpaid holds that ran out (used by tests and tooling).
func RunDepositsOnce(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, mailer mail.Sender) {
	newServer(pool, cfg, mailer).depositPass(ctx)
}
