package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

const (
	reminderBatch       = 20
	reminderMaxAttempts = 3
	// A reminder whose time already passed is still worth sending when the visit is at least this far away.
	reminderLateGrace = 30 * time.Minute
)

// reminderBackoff is how long to wait before retrying after the n-th failed attempt.
func reminderBackoff(attempts int) time.Duration {
	return time.Duration(5*pow4(attempts-1)) * time.Minute // 5, 20 minutes
}

func pow4(n int) int {
	r := 1
	for ; n > 0; n-- {
		r *= 4
	}
	return r
}

// runReminders sends the queued appointment reminders until ctx ends.
func (s *Server) runReminders(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		s.reminderPass(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (s *Server) reminderPass(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("reminders: panic: %v", r)
		}
	}()
	for i := 0; i < 50; i++ { // bounded: a backlog is drained over several minutes
		n, err := s.processDueReminders(ctx)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("reminders: %v", err)
			}
			return
		}
		if n < reminderBatch {
			return
		}
	}
}

// RunRemindersOnce processes the reminders that are due right now (used by tests and tooling).
func RunRemindersOnce(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, mailer mail.Sender) (int, error) {
	return newServer(pool, cfg, mailer).processDueReminders(ctx)
}

// reminderTarget is everything needed to decide whether and how to remind about one appointment.
type reminderTarget struct {
	Status      string
	Consent     bool
	Email       string
	Phone       string
	Start       time.Time
	Info        apptInfo
	RemindEmail bool
	RemindWA    bool
	Hours       []int32
	Note        string
}

func (t reminderTarget) active() bool { return t.Status == "scheduled" || t.Status == "confirmed" }

// loadReminderTarget reads an appointment with its contact data and the clinic's reminder settings.
// ok is false when the appointment does not belong to the clinic.
func (s *Server) loadReminderTarget(ctx context.Context, q queryRower, clinicID, appointmentID string) (t reminderTarget, ok bool, err error) {
	var date, start, tz string
	var apEmail, apPhone, pEmail, pPhone, gEmail, gPhone string
	var apConsent, pConsent bool
	var slug *string
	var enabled bool
	err = q.QueryRow(ctx, `
		SELECT ap.status, to_char(ap.date, 'YYYY-MM-DD'), to_char(ap.start_hour, 'HH24:MI'), ap.names, coalesce(ap.confirm_token, ''),
		       ap.email, ap.phone, ap.reminders_consent,
		       coalesce(p.email, ''), coalesce(p.phone, ''), coalesce(p.guardian_email, ''), coalesce(p.guardian_phone, ''), coalesce(p.reminders_ok, false),
		       c.name, c.address, c.phone_number, coalesce(c.settings->>'timezone', ''),
		       coalesce(u.name, ''), coalesce(ci.name, ''),
		       coalesce(s.remind_email, true), coalesce(s.remind_whatsapp, false), coalesce(s.remind_hours, '{24,2}'), coalesce(s.reminder_template, ''),
		       coalesce(s.booking_enabled, false), s.booking_slug
		FROM appointments ap
		JOIN clinics c ON c.id = ap.clinic_id
		LEFT JOIN patients p ON p.id = ap.patient_id AND p.clinic_id = ap.clinic_id
		LEFT JOIN users u ON u.id = ap.professional_id
		LEFT JOIN catalog_items ci ON ci.id = ap.service_id AND ci.clinic_id = ap.clinic_id
		LEFT JOIN agenda_settings s ON s.clinic_id = ap.clinic_id
		WHERE ap.clinic_id = $1 AND ap.id = $2`, clinicID, appointmentID).
		Scan(&t.Status, &date, &start, &t.Info.PatientName, &t.Info.Token,
			&apEmail, &apPhone, &apConsent,
			&pEmail, &pPhone, &gEmail, &gPhone, &pConsent,
			&t.Info.ClinicName, &t.Info.ClinicAddress, &t.Info.ClinicPhone, &tz,
			&t.Info.Professional, &t.Info.Service,
			&t.RemindEmail, &t.RemindWA, &t.Hours, &t.Note,
			&enabled, &slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, false, nil
	}
	if err != nil {
		return t, false, err
	}
	t.Info.PatientName = firstName(t.Info.PatientName)
	if enabled && slug != nil {
		t.Info.Slug = *slug
	}
	t.Consent = pConsent || apConsent
	t.Email = firstNonEmpty(apEmail, pEmail, gEmail)
	t.Phone = firstNonEmpty(apPhone, pPhone, gPhone)
	t.Start, err = localTime(locationOrDefault(tz), date, start)
	t.Info.Start = t.Start
	return t, err == nil, err
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

// scheduleReminders recomputes the pending reminders of one appointment (call it after the
// appointment is created, moved, confirmed or cancelled). q is the pool or a transaction.
//
// Pending reminders are dropped and rebuilt from the clinic's settings. Nothing is queued unless the
// appointment is scheduled or confirmed, in the future, reachable by a configured channel and the
// person agreed to reminders. A reminder whose time already passed is only created when the visit is
// more than 30 minutes away, and then only the one closest to the visit (a patient booking for
// tomorrow afternoon must not get "24 hours before" and "2 hours before" at once).
func (s *Server) scheduleReminders(ctx context.Context, q execer, clinicID, appointmentID string) error {
	if _, err := q.Exec(ctx, `DELETE FROM appointment_reminders WHERE clinic_id = $1 AND appointment_id = $2 AND status = 'pending'`, clinicID, appointmentID); err != nil {
		return err
	}
	rq, ok := q.(queryRower)
	if !ok {
		rq = s.db
	}
	t, found, err := s.loadReminderTarget(ctx, rq, clinicID, appointmentID)
	if err != nil || !found {
		return err
	}
	now := time.Now()
	if !t.active() || !t.Start.After(now) || !t.Consent {
		return nil
	}
	if t.Info.Token == "" {
		tok, err := newToken()
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `UPDATE appointments SET confirm_token = $3 WHERE clinic_id = $1 AND id = $2 AND confirm_token IS NULL`, clinicID, appointmentID, tok); err != nil {
			return err
		}
	}
	type channel struct {
		name string
		on   bool
	}
	for _, ch := range []channel{{"email", t.RemindEmail && t.Email != ""}, {"whatsapp", t.RemindWA && t.Phone != ""}} {
		if !ch.on {
			continue
		}
		var lateHours int32 = -1
		future := false
		for _, h := range t.Hours {
			if h <= 0 {
				continue
			}
			at := t.Start.Add(-time.Duration(h) * time.Hour)
			if at.After(now) {
				future = true
				if err := s.queueReminder(ctx, q, clinicID, appointmentID, ch.name, int(h), at); err != nil {
					return err
				}
			} else if lateHours == -1 || h < lateHours {
				lateHours = h
			}
		}
		if lateHours != -1 && !future && t.Start.Sub(now) > reminderLateGrace {
			if err := s.queueReminder(ctx, q, clinicID, appointmentID, ch.name, int(lateHours), now); err != nil {
				return err
			}
		}
	}
	return nil
}

// queueReminder inserts a reminder, or re-arms one that was already handled for a different time
// (the appointment moved). A reminder already sent for the same time is left alone.
func (s *Server) queueReminder(ctx context.Context, q execer, clinicID, appointmentID, channel string, hours int, at time.Time) error {
	_, err := q.Exec(ctx, `
		INSERT INTO appointment_reminders (clinic_id, appointment_id, channel, hours_before, send_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (appointment_id, channel, hours_before) DO UPDATE
		   SET send_at = EXCLUDED.send_at, status = 'pending', attempts = 0, error = '', sent_at = NULL
		 WHERE appointment_reminders.status <> 'pending' AND abs(extract(epoch FROM appointment_reminders.send_at - EXCLUDED.send_at)) > 60
		    OR appointment_reminders.status = 'pending'`,
		clinicID, appointmentID, channel, hours, at)
	return err
}

type reminderRow struct {
	ID, ClinicID, AppointmentID, Channel string
	Hours, Attempts                      int
}

// processDueReminders handles one batch of due reminders. The rows stay locked (FOR UPDATE SKIP
// LOCKED) until the batch commits, so several API instances never send the same reminder twice.
func (s *Server) processDueReminders(ctx context.Context) (int, error) {
	var n int
	err := inTx(ctx, s.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, clinic_id, appointment_id, channel, hours_before, attempts
			FROM appointment_reminders
			WHERE status = 'pending' AND send_at <= now()
			ORDER BY send_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED`, reminderBatch)
		if err != nil {
			return err
		}
		batch, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (reminderRow, error) {
			var x reminderRow
			err := r.Scan(&x.ID, &x.ClinicID, &x.AppointmentID, &x.Channel, &x.Hours, &x.Attempts)
			return x, err
		})
		if err != nil {
			return err
		}
		for _, r := range batch {
			status, msg, retry := s.deliverReminder(ctx, tx, r)
			n++
			switch {
			case status == "sent":
				_, err = tx.Exec(ctx, `UPDATE appointment_reminders SET status = 'sent', sent_at = now(), attempts = attempts + 1, error = '' WHERE id = $1`, r.ID)
			case status == "skipped":
				_, err = tx.Exec(ctx, `UPDATE appointment_reminders SET status = 'skipped', error = $2 WHERE id = $1`, r.ID, msg)
			case retry && r.Attempts+1 < reminderMaxAttempts:
				_, err = tx.Exec(ctx, `UPDATE appointment_reminders SET attempts = attempts + 1, error = $2, send_at = $3 WHERE id = $1`,
					r.ID, msg, time.Now().Add(reminderBackoff(r.Attempts+1)))
			default:
				_, err = tx.Exec(ctx, `UPDATE appointment_reminders SET status = 'failed', attempts = attempts + 1, error = $2 WHERE id = $1`, r.ID, msg)
				if err == nil {
					s.ntfAppointmentByID(ctx, tx, r.ClinicID, r.AppointmentID, "reminder_failed", "No se pudo enviar un recordatorio", "/agenda")
				}
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	return n, err
}

// deliverReminder re-checks the appointment and sends one reminder. status is sent, skipped or
// failed; retry says whether a failure is worth trying again.
func (s *Server) deliverReminder(ctx context.Context, q queryRower, r reminderRow) (status, msg string, retry bool) {
	t, ok, err := s.loadReminderTarget(ctx, q, r.ClinicID, r.AppointmentID)
	if err != nil {
		return "failed", "No se pudo leer la cita: " + err.Error(), true
	}
	switch {
	case !ok || !t.active():
		return "skipped", "La cita ya no está vigente.", false
	case !t.Start.After(time.Now()):
		return "skipped", "La cita ya pasó.", false
	case !t.Consent:
		return "skipped", "Sin consentimiento para recibir recordatorios.", false
	}
	switch r.Channel {
	case "email":
		if !t.RemindEmail {
			return "skipped", "Los recordatorios por correo están desactivados.", false
		}
		if t.Email == "" {
			return "skipped", "La cita no tiene correo.", false
		}
		if !s.mailEnabled() {
			return "skipped", "El envío de correo no está configurado.", false
		}
		if t.Info.Token == "" {
			return "skipped", "La cita no tiene enlace de confirmación.", false
		}
		subject, text, html := s.reminderMail(t.Info, t.Note)
		sctx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		if err := s.mailer.Send(sctx, mail.Message{To: []string{t.Email}, Subject: subject, Text: text, HTML: html}); err != nil {
			return "failed", truncate("Correo: "+err.Error(), 300), true
		}
		return "sent", "", false
	case "whatsapp":
		if !t.RemindWA {
			return "skipped", "Los recordatorios por WhatsApp están desactivados.", false
		}
		if !s.whatsAppEnabled() {
			return "skipped", "WhatsApp no está configurado.", false
		}
		phone := normalizePhoneMX(t.Phone)
		if phone == "" {
			return "skipped", "El teléfono no es válido para WhatsApp.", false
		}
		sctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := s.sendWhatsAppReminder(sctx, phone, t.Info); err != nil {
			var we *waError
			permanent := errors.As(err, &we) && we.Permanent
			return "failed", truncate("WhatsApp: "+err.Error(), 300), !permanent
		}
		return "sent", "", false
	}
	return "skipped", fmt.Sprintf("Canal desconocido: %s.", r.Channel), false
}
