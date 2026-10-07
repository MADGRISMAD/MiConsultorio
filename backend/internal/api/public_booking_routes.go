package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// bookingAPI holds the public (no session) handlers of online booking and of the patient's
// confirm/cancel link, with their own rate limiters.
type bookingAPI struct {
	*Server
	reads   *rateLimiter // any public read, per IP
	badTok  *rateLimiter // unknown tokens, per IP
	writes  *rateLimiter // confirm / cancel / opt-out, per IP
	bookIP  *rateLimiter // new bookings, per IP
	bookKey *rateLimiter // new bookings, per contact
}

// mountPublicBooking: Online booking and appointment confirm/cancel by token.
func (s *Server) mountPublicBooking(r chi.Router) {
	b := &bookingAPI{
		Server:  s,
		reads:   newRateLimiter(300, 10*time.Minute),
		badTok:  newRateLimiter(15, 15*time.Minute),
		writes:  newRateLimiter(30, 10*time.Minute),
		bookIP:  newRateLimiter(8, time.Hour),
		bookKey: newRateLimiter(3, time.Hour),
	}
	r.Get("/public/appointments/{token}", b.getByToken)
	r.Post("/public/appointments/{token}/confirm", b.confirmByToken)
	r.Post("/public/appointments/{token}/cancel", b.cancelByToken)
	r.Post("/public/appointments/{token}/optout", b.optoutByToken)

	r.Get("/public/booking/{slug}", b.bookingInfo)
	r.Get("/public/booking/{slug}/availability", b.bookingAvailability)
	r.Post("/public/booking/{slug}/appointments", b.bookingCreate)
}

func tooMany(w http.ResponseWriter) {
	writeError(w, http.StatusTooManyRequests, "Demasiados intentos. Inténtalo de nuevo en un rato.")
}

// limit records one hit on l for the caller and reports whether it is still within the limit.
func limit(w http.ResponseWriter, l *rateLimiter, key string) bool {
	if !l.allow(key) {
		tooMany(w)
		return false
	}
	l.fail(key)
	return true
}

type tokenAppt struct {
	ID, ClinicID, Status, PatientID string
	Names                           string
	Date, Start, End                string
	Reason                          string
	Phone, Email                    string
	StartAt                         time.Time
	Info                            apptInfo
	CancelMinHours                  int
	ShowReminders                   bool
}

const tokenApptSQL = `
	SELECT ap.id, ap.clinic_id, ap.status, coalesce(ap.patient_id::text, ''), ap.names,
	       to_char(ap.date, 'YYYY-MM-DD'), to_char(ap.start_hour, 'HH24:MI'), to_char(ap.end_hour, 'HH24:MI'),
	       ap.phone, ap.email,
	       c.name, c.address, c.phone_number, coalesce(c.settings->>'timezone', ''),
	       coalesce(u.name, ''), coalesce(ci.name, ''),
	       coalesce(s.cancel_min_hours, 2), coalesce(s.booking_enabled, false), s.booking_slug,
	       ap.reminders_consent OR coalesce(p.reminders_ok, false)
	FROM appointments ap
	JOIN clinics c ON c.id = ap.clinic_id
	LEFT JOIN patients p ON p.id = ap.patient_id AND p.clinic_id = ap.clinic_id
	LEFT JOIN users u ON u.id = ap.professional_id
	LEFT JOIN catalog_items ci ON ci.id = ap.service_id AND ci.clinic_id = ap.clinic_id
	LEFT JOIN agenda_settings s ON s.clinic_id = ap.clinic_id
	WHERE ap.confirm_token = $1`

// findByToken loads an appointment by its secret link. forUpdate locks the row (use inside a transaction).
func (b *bookingAPI) findByToken(ctx context.Context, q queryRower, token string, forUpdate bool) (*tokenAppt, error) {
	sql := tokenApptSQL
	if forUpdate {
		sql += ` FOR UPDATE OF ap`
	}
	var a tokenAppt
	var tz string
	var enabled bool
	var slug *string
	err := q.QueryRow(ctx, sql, token).Scan(&a.ID, &a.ClinicID, &a.Status, &a.PatientID, &a.Names, &a.Date, &a.Start, &a.End, &a.Phone, &a.Email,
		&a.Info.ClinicName, &a.Info.ClinicAddress, &a.Info.ClinicPhone, &tz, &a.Info.Professional, &a.Info.Service,
		&a.CancelMinHours, &enabled, &slug, &a.ShowReminders)
	if err != nil {
		return nil, err
	}
	loc := locationOrDefault(tz)
	if a.StartAt, err = localTime(loc, a.Date, a.Start); err != nil {
		return nil, err
	}
	a.Info.Start, a.Info.Token, a.Info.PatientName = a.StartAt, token, firstName(a.Names)
	if enabled && slug != nil {
		a.Info.Slug = *slug
	}
	return &a, nil
}

// lookup validates the token and finds the appointment; every miss is the same 404.
func (b *bookingAPI) lookup(w http.ResponseWriter, r *http.Request, forUpdate bool, q queryRower) (*tokenAppt, bool) {
	token := chi.URLParam(r, "token")
	notFound := func() (*tokenAppt, bool) {
		b.badTok.fail("tok|" + clientIP(r))
		writeError(w, http.StatusNotFound, "No encontramos esa cita.")
		return nil, false
	}
	if !b.badTok.allow("tok|" + clientIP(r)) {
		tooMany(w)
		return nil, false
	}
	if !validToken(token) {
		return notFound()
	}
	a, err := b.findByToken(r.Context(), q, token, forUpdate)
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound()
	}
	if err != nil {
		serverError(w, r, err)
		return nil, false
	}
	return a, true
}

func (a *tokenAppt) canCancel(now time.Time) bool {
	return (a.Status == "scheduled" || a.Status == "confirmed") &&
		a.StartAt.Sub(now) >= time.Duration(a.CancelMinHours)*time.Hour
}

func (a *tokenAppt) view(now time.Time) map[string]any {
	active := a.Status == "scheduled" || a.Status == "confirmed"
	return map[string]any{
		"clinic":           map[string]string{"name": a.Info.ClinicName, "address": a.Info.ClinicAddress, "phone": a.Info.ClinicPhone},
		"professional":     a.Info.Professional,
		"service":          a.Info.Service,
		"patient":          a.Info.PatientName,
		"date":             a.Date,
		"start":            a.Start,
		"end":              a.End,
		"status":           a.Status,
		"past":             !a.StartAt.After(now),
		"can_confirm":      a.Status == "scheduled" && a.StartAt.After(now),
		"can_cancel":       a.canCancel(now),
		"cancel_min_hours": a.CancelMinHours,
		"reminders":        a.ShowReminders && active,
		"rebook_slug":      a.Info.Slug,
	}
}

func (b *bookingAPI) getByToken(w http.ResponseWriter, r *http.Request) {
	if !limit(w, b.reads, "read|"+clientIP(r)) {
		return
	}
	a, ok := b.lookup(w, r, false, b.db)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"appointment": a.view(time.Now())})
}

func (b *bookingAPI) confirmByToken(w http.ResponseWriter, r *http.Request) {
	if !limit(w, b.writes, "act|"+clientIP(r)) {
		return
	}
	b.mutate(w, r, func(ctx context.Context, tx pgx.Tx, a *tokenAppt, now time.Time) (string, error) {
		if !a.StartAt.After(now) || (a.Status != "scheduled" && a.Status != "confirmed") {
			return "Esta cita ya no se puede confirmar.", nil
		}
		if a.Status == "scheduled" {
			if _, err := tx.Exec(ctx, `UPDATE appointments SET status = 'confirmed', updated_at = now() WHERE id = $1 AND clinic_id = $2`, a.ID, a.ClinicID); err != nil {
				return "", err
			}
			a.Status = "confirmed"
			audit(ctx, tx, a.ClinicID, nil, "appointment_confirmed", "Cita confirmada por el paciente desde su enlace", map[string]any{"appointment_id": a.ID})
		}
		return "", nil
	})
}

func (b *bookingAPI) cancelByToken(w http.ResponseWriter, r *http.Request) {
	if !limit(w, b.writes, "act|"+clientIP(r)) {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if r.ContentLength != 0 && !decode(w, r, &req) {
		return
	}
	reason := truncate(strings.TrimSpace(req.Reason), 300)
	b.mutate(w, r, func(ctx context.Context, tx pgx.Tx, a *tokenAppt, now time.Time) (string, error) {
		if a.Status == "cancelled" {
			return "", nil
		}
		if !a.canCancel(now) {
			if a.Status != "scheduled" && a.Status != "confirmed" || !a.StartAt.After(now) {
				return "Esta cita ya no se puede cancelar.", nil
			}
			return fmt.Sprintf("Ya faltan menos de %d horas para tu cita. Para cancelar, comunícate con el consultorio.", a.CancelMinHours), nil
		}
		if _, err := tx.Exec(ctx, `UPDATE appointments SET status = 'cancelled', cancel_reason = $3, updated_at = now() WHERE id = $1 AND clinic_id = $2`,
			a.ID, a.ClinicID, "Paciente: "+reason); err != nil {
			return "", err
		}
		a.Status = "cancelled"
		audit(ctx, tx, a.ClinicID, nil, "appointment_cancelled", "Cita cancelada por el paciente desde su enlace", map[string]any{"appointment_id": a.ID})
		return "", nil
	})
}

func (b *bookingAPI) optoutByToken(w http.ResponseWriter, r *http.Request) {
	if !limit(w, b.writes, "act|"+clientIP(r)) {
		return
	}
	b.mutate(w, r, func(ctx context.Context, tx pgx.Tx, a *tokenAppt, now time.Time) (string, error) {
		// This visit, the patient's record and every other upcoming visit with the same contact.
		if _, err := tx.Exec(ctx, `
			UPDATE appointments SET reminders_consent = false
			WHERE clinic_id = $1 AND (id = $2 OR (date >= current_date - 1 AND (($3 <> '' AND lower(email) = lower($3)) OR ($4 <> '' AND phone = $4))))`,
			a.ClinicID, a.ID, a.Email, a.Phone); err != nil {
			return "", err
		}
		if a.PatientID != "" {
			if _, err := tx.Exec(ctx, `UPDATE patients SET reminders_ok = false, updated_at = now() WHERE clinic_id = $1 AND id = $2`, a.ClinicID, a.PatientID); err != nil {
				return "", err
			}
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM appointment_reminders r USING appointments ap
			WHERE r.appointment_id = ap.id AND r.status = 'pending' AND ap.clinic_id = $1 AND ap.reminders_consent = false
			  AND (ap.patient_id IS NULL OR NOT EXISTS (SELECT 1 FROM patients p WHERE p.id = ap.patient_id AND p.reminders_ok))`, a.ClinicID); err != nil {
			return "", err
		}
		a.ShowReminders = false
		audit(ctx, tx, a.ClinicID, nil, "reminders_optout", "El paciente dejó de recibir recordatorios desde su enlace", map[string]any{"appointment_id": a.ID})
		return "", nil
	})
}

// mutate runs fn on the locked appointment of the token's link, then rebuilds its reminders and
// answers with the updated view. fn returns a message to refuse with 409.
func (b *bookingAPI) mutate(w http.ResponseWriter, r *http.Request, fn func(ctx context.Context, tx pgx.Tx, a *tokenAppt, now time.Time) (string, error)) {
	ctx := r.Context()
	var out *tokenAppt
	var refused string
	var missing bool
	err := inTx(ctx, b.db, func(tx pgx.Tx) error {
		token := chi.URLParam(r, "token")
		if !validToken(token) {
			missing = true
			return nil
		}
		a, err := b.findByToken(ctx, tx, token, true)
		if errors.Is(err, pgx.ErrNoRows) {
			missing = true
			return nil
		}
		if err != nil {
			return err
		}
		now := time.Now()
		if refused, err = fn(ctx, tx, a, now); err != nil || refused != "" {
			return err
		}
		if err := b.scheduleReminders(ctx, tx, a.ClinicID, a.ID); err != nil {
			return err
		}
		out = a
		return nil
	})
	switch {
	case err != nil && !errors.Is(err, pgx.ErrNoRows):
		serverError(w, r, err)
	case missing:
		b.badTok.fail("tok|" + clientIP(r))
		writeError(w, http.StatusNotFound, "No encontramos esa cita.")
	case refused != "":
		writeJSON(w, http.StatusConflict, errorBody{Code: "NOT_ALLOWED", Message: refused})
	default:
		writeJSON(w, http.StatusOK, map[string]any{"appointment": out.view(time.Now())})
	}
}
