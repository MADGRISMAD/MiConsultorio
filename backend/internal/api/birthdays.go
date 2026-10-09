package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// Birthday greetings by e-mail. People with a birth date and an e-mail get their own; a pet's birthday goes to its owner
// (about the pet); an owner with a birth date gets one as well. Without a birth date nothing is sent for that person or
// pet. Only those who agreed to e-mails (reminders_ok) are written to, every message carries an unsubscribe link, and a
// greeting goes out once a year. They leave from 9:00 clinic time on the birthday.

const (
	birthdayHour     = 9
	birthdayAttempts = 3
)

func (s *Server) runBirthdays(ctx context.Context) {
	tick := time.NewTicker(30 * time.Minute)
	defer tick.Stop()
	for {
		s.birthdayPass(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (s *Server) birthdayPass(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("birthdays: panic: %v", r)
		}
	}()
	if !s.mailEnabled() {
		return
	}
	if err := s.sendBirthdays(ctx, time.Now()); err != nil && ctx.Err() == nil {
		log.Printf("birthdays: %v", err)
	}
}

type birthdayRow struct {
	Kind, ID, Subject, Names, OwnerName, Email, UnsubID, Birth string
}

func (s *Server) sendBirthdays(ctx context.Context, now time.Time) error {
	rows, err := s.db.Query(ctx, `SELECT id::text, name, coalesce(settings->>'timezone', '') FROM clinics`)
	if err != nil {
		return err
	}
	type clinic struct{ ID, Name, TZ string }
	var clinics []clinic
	for rows.Next() {
		var c clinic
		if err := rows.Scan(&c.ID, &c.Name, &c.TZ); err != nil {
			rows.Close()
			return err
		}
		clinics = append(clinics, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, c := range clinics {
		local := now.In(locationOrDefault(c.TZ))
		if local.Hour() < birthdayHour {
			continue
		}
		// a 29 February birthday is celebrated on the 28th in the years without one
		m2, d2 := int(local.Month()), local.Day()
		if local.Month() == time.February && local.Day() == 28 && local.AddDate(0, 0, 1).Month() != time.February {
			m2, d2 = 2, 29
		}
		list, err := s.birthdayCandidates(ctx, c.ID, int(local.Month()), local.Day(), m2, d2, local.Year())
		if err != nil {
			return err
		}
		for _, b := range list {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.sendBirthday(ctx, c.ID, c.Name, b, local.Year())
		}
	}
	return nil
}

// birthdayCandidates are the people, pets and owners of a clinic whose birthday is today, with an e-mail and consent.
func (s *Server) birthdayCandidates(ctx context.Context, clinicID string, m1, d1, m2, d2, year int) ([]birthdayRow, error) {
	rows, err := s.db.Query(ctx, `
		SELECT 'patient', p.id::text, p.subject, p.names,
		       CASE WHEN p.subject = 'animal' THEN coalesce(nullif(o.name, ''), nullif(p.guardian_name, ''), '') ELSE '' END,
		       CASE WHEN p.subject = 'animal' THEN coalesce(nullif(o.email, ''), nullif(p.guardian_email, ''), '')
		            ELSE coalesce(nullif(p.email, ''), nullif(p.guardian_email, ''), '') END,
		       p.id::text, to_char(p.birth_date, 'YYYY-MM-DD')
		FROM patients p LEFT JOIN owners o ON o.id = p.owner_id
		WHERE p.clinic_id = $1 AND p.archived_at IS NULL AND p.birth_date IS NOT NULL
		  AND ((extract(month FROM p.birth_date)::int = $2 AND extract(day FROM p.birth_date)::int = $3)
		    OR (extract(month FROM p.birth_date)::int = $4 AND extract(day FROM p.birth_date)::int = $5))
		  AND (p.reminders_ok OR (p.owner_id IS NOT NULL AND EXISTS (SELECT 1 FROM patients q WHERE q.owner_id = p.owner_id AND q.reminders_ok)))
		  AND NOT EXISTS (SELECT 1 FROM birthday_mail b WHERE b.kind = 'patient' AND b.ref_id = p.id AND b.year = $6 AND (b.sent_at IS NOT NULL OR b.attempts >= $7))
		UNION ALL
		SELECT 'owner', o.id::text, 'owner', o.name, '', o.email,
		       (SELECT q.id::text FROM patients q WHERE q.owner_id = o.id AND q.reminders_ok ORDER BY q.created_at LIMIT 1),
		       to_char(o.birth_date, 'YYYY-MM-DD')
		FROM owners o
		WHERE o.clinic_id = $1 AND o.birth_date IS NOT NULL AND o.email <> ''
		  AND ((extract(month FROM o.birth_date)::int = $2 AND extract(day FROM o.birth_date)::int = $3)
		    OR (extract(month FROM o.birth_date)::int = $4 AND extract(day FROM o.birth_date)::int = $5))
		  AND EXISTS (SELECT 1 FROM patients q WHERE q.owner_id = o.id AND q.reminders_ok)
		  AND NOT EXISTS (SELECT 1 FROM birthday_mail b WHERE b.kind = 'owner' AND b.ref_id = o.id AND b.year = $6 AND (b.sent_at IS NOT NULL OR b.attempts >= $7))`,
		clinicID, m1, d1, m2, d2, year, birthdayAttempts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []birthdayRow
	for rows.Next() {
		var b birthdayRow
		var unsub *string
		if err := rows.Scan(&b.Kind, &b.ID, &b.Subject, &b.Names, &b.OwnerName, &b.Email, &unsub, &b.Birth); err != nil {
			return nil, err
		}
		if b.Email == "" || unsub == nil {
			continue
		}
		b.UnsubID = *unsub
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Server) sendBirthday(ctx context.Context, clinicID, clinicName string, b birthdayRow, year int) {
	// claim the greeting first: another instance (or an earlier pass) never sends it twice
	tag, err := s.db.Exec(ctx, `
		INSERT INTO birthday_mail (kind, ref_id, year, clinic_id, attempts) VALUES ($1, $2, $3, $4, 1)
		ON CONFLICT (kind, ref_id, year) DO UPDATE SET attempts = birthday_mail.attempts + 1, claimed_at = now()
		WHERE birthday_mail.sent_at IS NULL AND birthday_mail.attempts < $5 AND birthday_mail.claimed_at < now() - interval '10 minutes'`,
		b.Kind, b.ID, year, clinicID, birthdayAttempts)
	if err != nil || tag.RowsAffected() == 0 {
		return
	}
	age := 0
	if t, err := time.Parse("2006-01-02", b.Birth); err == nil {
		age = year - t.Year()
	}
	subject, text, html := s.birthdayMail(clinicName, b, age)
	sctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := s.mailer.Send(sctx, mail.Message{To: []string{b.Email}, Subject: subject, Text: text, HTML: html}); err != nil {
		log.Printf("birthday mail (%s %s): %v", b.Kind, b.ID, err)
		return
	}
	_, _ = s.db.Exec(ctx, `UPDATE birthday_mail SET sent_at = now() WHERE kind = $1 AND ref_id = $2 AND year = $3`, b.Kind, b.ID, year)
}

func (s *Server) birthdayMail(clinic string, b birthdayRow, age int) (subject, text, html string) {
	link := s.appLink("/baja/" + s.unsubscribeToken(b.UnsubID))
	var title, lead, wish string
	switch {
	case b.Subject == "animal":
		pet := firstName(b.Names)
		title = "¡Feliz cumpleaños, " + pet + "! 🐾"
		hello := "Hola"
		if o := firstName(b.OwnerName); o != "" {
			hello = "Hola " + o
		}
		lead = hello + ", hoy es el cumpleaños de " + pet + "."
		if age > 0 {
			lead = hello + fmt.Sprintf(", hoy %s cumple %d %s.", pet, age, map[bool]string{true: "año", false: "años"}[age == 1])
		}
		wish = "En " + clinic + " le mandamos un abrazo y deseamos que siga creciendo con mucha salud y felicidad a tu lado. Dale una caricia de nuestra parte."
	default:
		name := firstName(b.Names)
		title = "¡Feliz cumpleaños, " + name + "! 🎉"
		lead = "Hola " + name + ","
		wish = "Todo el equipo de " + clinic + " te desea un muy feliz cumpleaños. Que tengas un día lleno de salud y alegría."
	}
	subject = title + " · " + clinic
	text = lead + "\n\n" + wish + "\n\nSi ya no quieres recibir correos de " + clinic + ", date de baja aquí:\n" + link + "\n"
	body := `<p style="margin:0 0 10px;font-size:16px">` + esc(lead) + `</p><p style="margin:0 0 10px;font-size:16px">` + esc(wish) + `</p>` +
		`<p style="font-size:13px;color:#7a8b9b;margin:18px 0 0">Si ya no quieres recibir correos de ` + esc(clinic) + `, ` + smallLink(link, "date de baja aquí") + `.</p>`
	return subject, text, layout(title, body)
}

// ---- unsubscribe link (stateless: the patient's id and an HMAC of it) ----

func (s *Server) unsubscribeToken(patientID string) string {
	m := hmac.New(sha256.New, s.cfg.JWTSecret)
	m.Write([]byte("baja|" + patientID))
	return patientID + "." + hex.EncodeToString(m.Sum(nil))[:32]
}

func (s *Server) parseUnsubscribe(tok string) (string, bool) {
	id, _, ok := strings.Cut(tok, ".")
	if !ok || !validUUID(id) {
		return "", false
	}
	return id, hmac.Equal([]byte(tok), []byte(s.unsubscribeToken(id)))
}

func (s *Server) mountUnsubscribe(r chi.Router) {
	lim := newRateLimiter(30, 10*time.Minute)
	r.Get("/public/unsubscribe/{token}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseUnsubscribe(chi.URLParam(r, "token"))
		if !ok || !limit(w, lim, "baja|"+clientIP(r)) {
			if !ok {
				writeError(w, http.StatusNotFound, "Enlace no válido.")
			}
			return
		}
		var clinic string
		var active bool
		if err := s.db.QueryRow(r.Context(), `SELECT c.name, p.reminders_ok OR (p.owner_id IS NOT NULL AND EXISTS (SELECT 1 FROM patients q WHERE q.owner_id = p.owner_id AND q.reminders_ok))
			FROM patients p JOIN clinics c ON c.id = p.clinic_id WHERE p.id = $1`, id).Scan(&clinic, &active); err != nil {
			writeError(w, http.StatusNotFound, "Enlace no válido.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"clinic": clinic, "subscribed": active})
	})
	r.Post("/public/unsubscribe/{token}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseUnsubscribe(chi.URLParam(r, "token"))
		if !ok {
			writeError(w, http.StatusNotFound, "Enlace no válido.")
			return
		}
		if !limit(w, lim, "baja|"+clientIP(r)) {
			return
		}
		// the patient and, for a pet, every animal of the same owner
		tag, err := s.db.Exec(r.Context(), `UPDATE patients SET reminders_ok = false, updated_at = now()
			WHERE id = $1 OR (owner_id IS NOT NULL AND owner_id = (SELECT owner_id FROM patients WHERE id = $1))`, id)
		if err != nil {
			serverError(w, r, err)
			return
		}
		if tag.RowsAffected() == 0 {
			writeError(w, http.StatusNotFound, "Enlace no válido.")
			return
		}
		_, _ = s.db.Exec(r.Context(), `DELETE FROM appointment_reminders ar USING appointments ap
			WHERE ar.appointment_id = ap.id AND ar.status = 'pending' AND ap.patient_id = $1`, id)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
}

// RunBirthdaysOnce sends the greetings due at the given instant (used by tests and tooling).
func RunBirthdaysOnce(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, mailer mail.Sender, now time.Time) error {
	return newServer(pool, cfg, mailer).sendBirthdays(ctx, now)
}
