package api

import (
	"context"

	"errors"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// A professional of any giro can recommend the next visit (with themselves, or with someone else of the team: a referral); it lands in the agenda as pending confirmation
// (status "scheduled"), for the front desk to confirm with the patient.

type followUpIn struct {
	Date           string  `json:"date"`
	StartHour      string  `json:"start_hour"` // optional: otherwise the first free time of the day
	ProfessionalID *string `json:"professional_id"`
	Reason         string  `json:"reason"`
}

func (s *Server) recommendFollowUp(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	if !validUUID(pid) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	var in followUpIn
	if !decode(w, r, &in) {
		return
	}
	in.Date, in.StartHour, in.Reason = strings.TrimSpace(in.Date), strings.TrimSpace(in.StartHour), strings.TrimSpace(in.Reason)
	p := principalFrom(r.Context())
	loc := clinicLocation(r.Context(), s.db, p.ClinicID)
	day, err := time.ParseInLocation("2006-01-02", in.Date, loc)
	today := time.Now().In(loc).Format("2006-01-02")
	switch {
	case err != nil:
		writeError(w, http.StatusBadRequest, "La fecha de la cita no es válida.")
		return
	case in.Date < today:
		writeError(w, http.StatusBadRequest, "La cita recomendada no puede ser en el pasado.")
		return
	case utf8.RuneCountInString(in.Reason) > 300:
		writeError(w, http.StatusBadRequest, "El motivo es demasiado largo.")
		return
	}

	var names, last, curp, phone, email string
	err = s.db.QueryRow(r.Context(), `SELECT names, last_names, curp, phone, email FROM patients WHERE clinic_id = $1 AND id = $2 AND archived_at IS NULL`, p.ClinicID, pid).
		Scan(&names, &last, &curp, &phone, &email)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}

	f := appointmentIn{PatientID: &pid, Names: names, LastNames: last, CURP: curp, Phone: phone, Email: email, Date: in.Date, StartHour: in.StartHour,
		ProfessionalID: in.ProfessionalID, Details: "Seguimiento recomendado" + map[bool]string{true: ": " + in.Reason, false: ""}[in.Reason != ""]}
	if f.ProfessionalID == nil && p.Role == RoleDoctor {
		f.ProfessionalID = &p.UserID
	}
	if f.ProfessionalID != nil && *f.ProfessionalID == "" {
		f.ProfessionalID = nil
	}
	if msg := s.checkRefs(r.Context(), s.db, p.ClinicID, &f); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	// With someone else of the team it is a referral (a general physician sending the patient to physiotherapy):
	// it says who sent it, the other person is told, and the record becomes visible in their giro.
	referral := f.ProfessionalID != nil && *f.ProfessionalID != p.UserID
	var targetAreas []string
	if referral {
		var consults bool
		if err := s.db.QueryRow(r.Context(), `
			SELECT coalesce(ps.consults, true), coalesce(u.areas, '{}') FROM users u LEFT JOIN professional_settings ps ON ps.user_id = u.id
			WHERE u.clinic_id = $1 AND u.id = $2 AND NOT u.disabled AND u.role IN ('admin', 'doctor')`, p.ClinicID, *f.ProfessionalID).Scan(&consults, &targetAreas); err != nil || !consults {
			writeError(w, http.StatusBadRequest, "Esa persona del equipo no atiende pacientes.")
			return
		}
		f.Details = "Derivado por " + p.Name + map[bool]string{true: ": " + in.Reason, false: ""}[in.Reason != ""]
	}

	var out appointment
	err = inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if err := lockAgendaDay(r.Context(), tx, p.ClinicID, f.Date); err != nil {
			return err
		}
		if f.StartHour == "" {
			if err := s.firstFreeSlot(r.Context(), tx, p.ClinicID, &f, day); err != nil {
				return err
			}
		} else {
			s.slotFillEnd(r.Context(), tx, p.ClinicID, &f)
			if err := s.checkSlot(r.Context(), tx, p.ClinicID, &f, ""); err != nil {
				return err
			}
		}
		if msg := f.validate(); msg != "" {
			return fail(http.StatusBadRequest, msg)
		}
		if err := s.openInSameArea(r.Context(), tx, p.ClinicID, pid, "", "", f.ProfessionalID, ""); err != nil {
			return err
		}
		id := newRowID()
		sealed, err := encField("appointments", "details", id, f.Details)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(r.Context(), `
			INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, details, patient_id,
				professional_id, room, phone, email, confirm_token, reminders_consent, id)
			VALUES ($1,$2,$3,$4,$5::date,$6::time,$7::time,$8,$9::uuid,$10::uuid,'',$11,$12,$13,$15,$14::uuid) RETURNING id::text`,
			p.ClinicID, f.CURP, f.Names, f.LastNames, f.Date, f.StartHour, f.EndHour, sealed, pid, f.ProfessionalID, f.Phone, f.Email, newConfirmToken(), id, f.Email != "" || f.Phone != "").Scan(&id); err != nil {
			return err
		}
		if out, err = loadAppointment(r.Context(), tx, p.ClinicID, id); err != nil {
			return err
		}
		if err := s.scheduleReminders(r.Context(), tx, p.ClinicID, id); err != nil {
			return err
		}
		msg := "Recomendó una cita de seguimiento (por confirmar)"
		if referral {
			msg = "Derivó al paciente a otro profesional del equipo (por confirmar)"
			if len(targetAreas) > 0 {
				// the record has to be visible in the giro that will see the patient
				if _, err := tx.Exec(r.Context(), `
					UPDATE patients SET kinds = (SELECT array_agg(DISTINCT x) FROM unnest(kinds || $3::text[]) x) WHERE clinic_id = $1 AND id = $2 AND cardinality(kinds) > 0`,
					p.ClinicID, pid, targetAreas); err != nil {
					return err
				}
			}
			s.notify(r.Context(), tx, p.ClinicID, ntfNotice{UserID: *f.ProfessionalID, Kind: "referral", Title: p.Name + " te derivó un paciente",
				Body: names + " " + last + " · " + f.Date + " " + f.StartHour + map[bool]string{true: " · " + in.Reason, false: ""}[in.Reason != ""], Link: "/pacientes/" + pid})
		}
		audit(r.Context(), tx, p.ClinicID, p, "appointment_create", msg, map[string]any{"appointment": id, "date": f.Date, "start": f.StartHour, "recommended": true, "referral": referral})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	s.mailBooked(r.Context(), p.ClinicID, out.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"appointment": out})
}

// firstFreeSlot picks the first free time of the day inside the clinic's opening hours for that professional.
func (s *Server) firstFreeSlot(ctx context.Context, q queryRower, clinicID string, f *appointmentIn, day time.Time) error {
	c, err := loadClinic(ctx, q, clinicID)
	if err != nil {
		return err
	}
	key := weekdays[(int(day.Weekday())+6)%7]
	h := c.Settings.Hours[key]
	if !h.Open {
		return fail(http.StatusConflict, "El consultorio no abre ese día. Elige otra fecha.")
	}
	open, err1 := time.Parse("15:04", h.Start)
	closeAt, err2 := time.Parse("15:04", h.End)
	if err1 != nil || err2 != nil {
		return fail(http.StatusConflict, "El horario del consultorio no es válido.")
	}
	step := time.Duration(c.Settings.AppointmentMinutes) * time.Minute
	for t := open; !t.Add(step).After(closeAt); t = t.Add(step) {
		f.StartHour, f.EndHour = t.Format("15:04"), ""
		s.slotFillEnd(ctx, q, clinicID, f)
		code, err := s.slotConflict(ctx, q, clinicID, f.ProfessionalID, f.Room, f.Date, f.StartHour, f.EndHour, "")
		if err != nil {
			return err
		}
		if code == SlotFree {
			return nil
		}
	}
	return fail(http.StatusConflict, "No hay horario libre ese día. Elige otra fecha u hora.")
}

// mailBooked e-mails the patient as soon as an appointment is booked from the app (by staff, or as a follow-up
// recommended by a professional), when it has an e-mail and the patient receives reminders. The mail carries the
// links to confirm, reschedule or cancel. A failure never affects the booking.
func (s *Server) mailBooked(ctx context.Context, clinicID, appointmentID string) {
	if !s.mailEnabled() {
		return
	}
	// same contact rules as the reminders: the visit's own e-mail, else the patient's, else the owner's (pets)
	t, ok, err := s.loadReminderTarget(ctx, s.db, clinicID, appointmentID)
	if err != nil || !ok || t.Status != "scheduled" || !t.Consent || t.Email == "" || t.Info.Token == "" || !t.Start.After(time.Now()) {
		return
	}
	subject, text, html := s.bookingMail(t.Info, false)
	s.sendMail(mail.Message{To: []string{t.Email}, Subject: subject, Text: text, HTML: html, Attachments: t.Info.calendarAttachments(false, s.manageLink(t.Info.Token, ""))})
}
