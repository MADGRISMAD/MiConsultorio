package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// Follow-up reminders by e-mail, so patients come back without anyone calling them:
//   - a vaccine or deworming that is about to be due (or just became due),
//   - the next consultation the professional suggested in the note (if nothing is booked yet),
//   - the dental check-up six months after the last visit,
//   - the yearly Papanicolaou.
// Like the birthday greetings they go only to people who agreed to e-mails (reminders_ok), carry an unsubscribe link,
// leave from 9:00 clinic time and are sent once per due date.

func (s *Server) recallPass(ctx context.Context) {
	if !s.mailEnabled() {
		return
	}
	if err := s.sendRecalls(ctx, time.Now()); err != nil && ctx.Err() == nil {
		log.Printf("recalls: %v", err)
	}
}

// RunRecallsOnce sends the follow-up reminders due at the given instant (used by tests and tooling).
func RunRecallsOnce(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, mailer mail.Sender, now time.Time) error {
	return newServer(pool, cfg, mailer).sendRecalls(ctx, now)
}

type recallRow struct {
	Kind, Ref, Due, PatientID, Subject, Names, Owner, Email, Detail string
}

// the columns every recall query returns, and what makes a patient reachable
const recallWho = `CASE WHEN p.subject = 'animal' THEN coalesce(nullif(o.name, ''), nullif(p.guardian_name, ''), '') ELSE '' END,
	CASE WHEN p.subject = 'animal' THEN coalesce(nullif(o.email, ''), nullif(p.guardian_email, ''), '')
	     ELSE coalesce(nullif(p.email, ''), nullif(p.guardian_email, ''), '') END`
const recallSelect = `p.id::text, p.subject, p.names, ` + recallWho
const recallReachable = `p.archived_at IS NULL AND (p.reminders_ok OR (p.owner_id IS NOT NULL AND EXISTS (SELECT 1 FROM patients q WHERE q.owner_id = p.owner_id AND q.reminders_ok)))`
const recallNoFutureVisit = `NOT EXISTS (SELECT 1 FROM appointments a WHERE a.clinic_id = p.clinic_id AND a.patient_id = p.id AND a.date >= $2::date AND a.status IN ('scheduled', 'confirmed'))`
const recallNotSent = `NOT EXISTS (SELECT 1 FROM recall_mail m WHERE m.kind = '%s' AND m.ref = %s AND m.due = %s AND (m.sent_at IS NOT NULL OR m.attempts >= 3))`

func (s *Server) sendRecalls(ctx context.Context, now time.Time) error {
	rows, err := s.db.Query(ctx, `SELECT c.id::text, c.name, coalesce(c.settings->>'timezone', ''), CASE WHEN a.booking_enabled THEN coalesce(a.booking_slug, '') ELSE '' END, c.phone_number
		FROM clinics c LEFT JOIN agenda_settings a ON a.clinic_id = c.id`)
	if err != nil {
		return err
	}
	type clinic struct{ ID, Name, TZ, Slug, Phone string }
	var clinics []clinic
	for rows.Next() {
		var c clinic
		if err := rows.Scan(&c.ID, &c.Name, &c.TZ, &c.Slug, &c.Phone); err != nil {
			rows.Close()
			return err
		}
		clinics = append(clinics, c)
	}
	rows.Close()
	for _, c := range clinics {
		local := now.In(locationOrDefault(c.TZ))
		if local.Hour() < birthdayHour {
			continue
		}
		today := local.Format("2006-01-02")
		list, err := s.recallCandidates(ctx, c.ID, today)
		if err != nil {
			return err
		}
		for _, r := range list {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.sendRecall(ctx, c.ID, c.Name, c.Phone, c.Slug, r)
		}
	}
	return nil
}

func (s *Server) recallCandidates(ctx context.Context, clinicID, today string) ([]recallRow, error) {
	var out []recallRow
	collect := func(sql string, args ...any) error {
		rows, err := s.db.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r recallRow
			if err := rows.Scan(&r.Kind, &r.Ref, &r.Due, &r.PatientID, &r.Subject, &r.Names, &r.Owner, &r.Email, &r.Detail); err != nil {
				return err
			}
			if r.Email != "" {
				out = append(out, r)
			}
		}
		return rows.Err()
	}
	// vaccines and dewormings: the latest dose of each one, due from a week before to two weeks after
	if err := collect(`
		SELECT 'vaccine', x.id, x.due, x.pid, x.subject, x.names, x.owner, x.email, x.name FROM (
			SELECT DISTINCT ON (v.patient_id, v.kind, lower(v.name))
			       v.id::text AS id, to_char(v.next_due, 'YYYY-MM-DD') AS due, v.next_due AS nd, v.name, v.voided_at, `+recallSelect+`
			FROM vaccinations v JOIN patients p ON p.id = v.patient_id LEFT JOIN owners o ON o.id = p.owner_id
			WHERE v.clinic_id = $1 AND v.voided_at IS NULL AND `+recallReachable+`
			ORDER BY v.patient_id, v.kind, lower(v.name), v.applied_on DESC, v.created_at DESC
		) x(id, due, nd, name, voided_at, pid, subject, names, owner, email)
		WHERE x.nd BETWEEN $2::date - 14 AND $2::date + 7 AND `+fmt.Sprintf(recallNotSent, "vaccine", "x.id", "x.nd"), clinicID, today); err != nil {
		return nil, err
	}
	// the next consultation suggested in a note, three days ahead, when nothing is booked yet
	if err := collect(`
		SELECT 'visit', e.id::text, to_char(e.next_visit, 'YYYY-MM-DD'), `+recallSelect+`, ''
		FROM encounters e JOIN patients p ON p.id = e.patient_id LEFT JOIN owners o ON o.id = p.owner_id
		WHERE e.clinic_id = $1 AND e.addendum_of IS NULL AND e.next_visit BETWEEN $2::date AND $2::date + 3 AND `+recallReachable+` AND `+recallNoFutureVisit+`
		  AND `+fmt.Sprintf(recallNotSent, "visit", "e.id::text", "e.next_visit"), clinicID, today); err != nil {
		return nil, err
	}
	// dental check-up six months after the last visit
	if err := collect(`
		SELECT 'cleaning', p.id::text, to_char((p.last_encounter_at + interval '6 months')::date, 'YYYY-MM-DD'), `+recallSelect+`, ''
		FROM patients p LEFT JOIN owners o ON o.id = p.owner_id
		WHERE p.clinic_id = $1 AND p.subject = 'person' AND 'DENTAL' = ANY(p.kinds) AND p.last_encounter_at IS NOT NULL
		  AND (p.last_encounter_at + interval '6 months')::date BETWEEN $2::date - 30 AND $2::date AND `+recallReachable+` AND `+recallNoFutureVisit+`
		  AND `+fmt.Sprintf(recallNotSent, "cleaning", "p.id::text", "(p.last_encounter_at + interval '6 months')::date"), clinicID, today); err != nil {
		return nil, err
	}
	// yearly Papanicolaou: the date lives in the (sealed) profile, so it is read here
	prows, err := s.db.Query(ctx, `
		SELECT p.id::text, p.profile, p.names, `+recallWho+`
		FROM patients p LEFT JOIN owners o ON o.id = p.owner_id
		WHERE p.clinic_id = $1 AND p.subject = 'person' AND 'GYNECOLOGY' = ANY(p.kinds) AND `+recallReachable+` AND `+recallNoFutureVisit, clinicID, today)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	t0, _ := time.Parse("2006-01-02", today)
	for prows.Next() {
		var id, names, owner, email string
		var raw []byte
		if err := prows.Scan(&id, &raw, &names, &owner, &email); err != nil {
			return nil, err
		}
		prof, err := decProfile(id, raw)
		if err != nil || email == "" {
			continue
		}
		if last, err := time.Parse("2006-01-02", profileText(prof, "last_pap")); err == nil {
			due := last.AddDate(1, 0, 0)
			if !due.After(t0) && !due.Before(t0.AddDate(0, 0, -30)) {
				out = append(out, recallRow{Kind: "pap", Ref: id, Due: due.Format("2006-01-02"), PatientID: id, Subject: "person", Names: names, Owner: owner, Email: email})
			}
		}
		// the contraceptive method's next application or renewal, from a week before
		if due, err := time.Parse("2006-01-02", profileText(prof, "contraception_renewal")); err == nil && !due.Before(t0.AddDate(0, 0, -14)) && !due.After(t0.AddDate(0, 0, 7)) {
			out = append(out, recallRow{Kind: "contraception", Ref: id, Due: due.Format("2006-01-02"), PatientID: id, Subject: "person", Names: names, Owner: owner, Email: email})
		}
	}
	return out, prows.Err()
}

func (s *Server) sendRecall(ctx context.Context, clinicID, clinicName, clinicPhone, slug string, r recallRow) {
	tag, err := s.db.Exec(ctx, `
		INSERT INTO recall_mail (kind, ref, due, clinic_id, attempts) VALUES ($1, $2, $3::date, $4, 1)
		ON CONFLICT (kind, ref, due) DO UPDATE SET attempts = recall_mail.attempts + 1, claimed_at = now()
		WHERE recall_mail.sent_at IS NULL AND recall_mail.attempts < 3 AND recall_mail.claimed_at < now() - interval '10 minutes'`,
		r.Kind, r.Ref, r.Due, clinicID)
	if err != nil || tag.RowsAffected() == 0 {
		return
	}
	subject, text, html := s.recallMail(clinicName, clinicPhone, slug, r)
	sctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := s.mailer.Send(sctx, mail.Message{To: []string{r.Email}, Subject: subject, Text: text, HTML: html}); err != nil {
		log.Printf("recall mail (%s %s): %v", r.Kind, r.Ref, err)
		return
	}
	_, _ = s.db.Exec(ctx, `UPDATE recall_mail SET sent_at = now() WHERE kind = $1 AND ref = $2 AND due = $3::date`, r.Kind, r.Ref, r.Due)
}

func (s *Server) recallMail(clinic, phone, slug string, r recallRow) (subject, text, html string) {
	animal := r.Subject == "animal"
	who := firstName(r.Names)
	hello := "Hola " + who
	if animal {
		hello = "Hola"
		if o := firstName(r.Owner); o != "" {
			hello = "Hola " + o
		}
	}
	due, _ := time.Parse("2006-01-02", r.Due)
	day := longDateES(due)
	var title, line string
	switch r.Kind {
	case "vaccine":
		if animal {
			title = "Toca " + r.Detail + " de " + who
			line = fmt.Sprintf("se acerca la siguiente dosis de %s de %s (%s).", r.Detail, who, day)
		} else {
			title = "Toca tu " + r.Detail
			line = fmt.Sprintf("se acerca tu siguiente dosis de %s (%s).", r.Detail, day)
		}
	case "visit":
		title = "Tu siguiente consulta"
		if animal {
			title = "La siguiente consulta de " + who
			line = fmt.Sprintf("la siguiente consulta de %s quedó sugerida para el %s.", who, day)
		} else {
			line = fmt.Sprintf("tu siguiente consulta quedó sugerida para el %s.", day)
		}
	case "cleaning":
		title = "Toca tu revisión dental"
		line = "ya pasaron seis meses desde tu última visita: es buen momento para tu revisión y limpieza dental."
	case "contraception":
		title = "Toca renovar tu método anticonceptivo"
		line = fmt.Sprintf("se acerca la fecha de tu siguiente aplicación o renovación del método anticonceptivo (%s).", day)
	case "pap":
		title = "Toca tu Papanicolaou"
		line = "ha pasado un año desde tu último Papanicolaou: es momento de tu revisión anual."
	}
	subject = title + " · " + clinic
	link := ""
	if slug != "" {
		link = s.appLink("/reservar/" + slug)
	}
	unsub := s.appLink("/baja/" + s.unsubscribeToken(r.PatientID))
	text = hello + ", " + line + "\n\n"
	body := `<p style="margin:0 0 10px;font-size:16px">` + esc(hello) + `, ` + esc(line) + `</p>`
	if link != "" {
		text += "Agenda tu cita aquí: " + link + "\n\n"
		body += button(link, "Agendar cita")
	} else if phone != "" {
		text += "Para agendar llama al " + phone + ".\n\n"
		body += `<p style="margin:0 0 10px;font-size:15px">Para agendar llama al <strong>` + esc(phone) + `</strong>.</p>`
	}
	text += "Si ya no quieres recibir correos de " + clinic + ", date de baja aquí:\n" + unsub + "\n"
	body += `<p style="font-size:13px;color:#7a8b9b;margin:18px 0 0">Si ya no quieres recibir correos de ` + esc(clinic) + `, ` + smallLink(unsub, "date de baja aquí") + `.</p>`
	return subject, text, layout(title, body)
}
