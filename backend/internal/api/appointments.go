package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// appointmentIn is what the client sends to create or move an appointment.
type appointmentIn struct {
	PatientID        *string `json:"patient_id"`
	Names            string  `json:"names"`
	LastNames        string  `json:"last_names"`
	CURP             string  `json:"CURP"`
	Date             string  `json:"date"`
	StartHour        string  `json:"startHour"`
	EndHour          string  `json:"endHour"`
	Details          string  `json:"details"`
	ProfessionalID   *string `json:"professional_id"`
	ServiceID        *string `json:"service_id"`
	Room             string  `json:"room"`
	Phone            string  `json:"phone"`
	Email            string  `json:"email"`
	RemindersConsent bool    `json:"reminders_consent"`
	Overbook         bool    `json:"overbook"` // accept a SLOT_TAKEN conflict (never a block)
	// Modality: «videollamada» or «presencial»; empty keeps what the appointment has (new ones are left as before: the professional's link, if any).
	Modality string `json:"modality"`
}

type appointment struct {
	ID               string     `json:"id" db:"id"`
	PatientID        *string    `json:"patient_id" db:"patient_id"`
	Names            string     `json:"names" db:"names"`
	LastNames        string     `json:"last_names" db:"last_names"`
	CURP             string     `json:"CURP" db:"curp"`
	Date             string     `json:"date" db:"date"`
	StartHour        string     `json:"startHour" db:"start_hour"`
	EndHour          string     `json:"endHour" db:"end_hour"`
	Details          string     `json:"details" db:"details"`
	ProfessionalID   *string    `json:"professional_id" db:"professional_id"`
	ProfessionalName string     `json:"professional_name" db:"professional_name"`
	Status           string     `json:"status" db:"status"`
	Room             string     `json:"room" db:"room"`
	Source           string     `json:"source" db:"source"`
	ServiceID        *string    `json:"service_id" db:"service_id"`
	ServiceName      string     `json:"service_name" db:"service_name"`
	Phone            string     `json:"phone" db:"phone"`
	Email            string     `json:"email" db:"email"`
	EncounterID      *string    `json:"encounter_id" db:"encounter_id"`
	SaleID           *string    `json:"sale_id" db:"sale_id"`
	CancelReason     string     `json:"cancel_reason" db:"cancel_reason"`
	ArrivedAt        *time.Time `json:"arrived_at" db:"arrived_at"`
	StartedAt        *time.Time `json:"started_at" db:"started_at"`
	FinishedAt       *time.Time `json:"finished_at" db:"finished_at"`
	RemindersConsent bool       `json:"reminders_consent" db:"reminders_consent"`
	// Modality is how it is given (nil: as before video calls were chosen per appointment) and VideoURL the link of a video call.
	Modality *string `json:"modality" db:"modality"`
	VideoURL string  `json:"video_url" db:"video_url"`
	// DepositCents is the anticipo paid online for this visit and DepositState where it stands
	// (none, pending, paid, refunded, kept, applied).
	DepositCents int    `json:"deposit_cents" db:"deposit_cents"`
	DepositState string `json:"deposit_state" db:"deposit_state"`
	// PackageID is the package of the patient that pays this visit with one of its sessions.
	PackageID *string `json:"package_id" db:"package_id"`
	// StartsAt is the appointment's date and time in the clinic's time zone as an instant; OpensAt is when it can
	// start being worked (arrival, consultation). Both are filled in by decorateAppointments.
	StartsAt *time.Time `json:"starts_at" db:"-"`
	OpensAt  *time.Time `json:"opens_at" db:"-"`
}

// apptEarlyWindow is how long before its time an appointment can be marked as arrived or started.
const apptEarlyWindow = 60 * time.Minute

// decorateAppointments computes StartsAt and OpensAt for rows read with appointmentSelect.
func decorateAppointments(loc *time.Location, list []appointment) {
	for i := range list {
		t, err := time.ParseInLocation("2006-01-02 15:04", list[i].Date+" "+list[i].StartHour, loc)
		if err != nil {
			continue
		}
		o := t.Add(-apptEarlyWindow)
		list[i].StartsAt, list[i].OpensAt = &t, &o
	}
}

const appointmentSelect = `SELECT a.id::text AS id, a.patient_id::text AS patient_id, a.names, a.last_names, a.curp,
		to_char(a.date, 'YYYY-MM-DD') AS date, to_char(a.start_hour, 'HH24:MI') AS start_hour, to_char(a.end_hour, 'HH24:MI') AS end_hour,
		a.details, a.professional_id::text AS professional_id, coalesce(pr.name, '') AS professional_name, a.status, a.room, a.source,
		a.service_id::text AS service_id, coalesce(ci.name, '') AS service_name, a.phone, a.email, a.encounter_id::text AS encounter_id,
		a.sale_id::text AS sale_id, a.cancel_reason, a.arrived_at, a.started_at, a.finished_at, a.reminders_consent, a.modality, a.video_url, a.deposit_cents, a.deposit_state, a.package_id::text AS package_id
	FROM appointments a
	LEFT JOIN users pr ON pr.id = a.professional_id
	LEFT JOIN catalog_items ci ON ci.id = a.service_id `

type rowsQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func loadAppointment(ctx context.Context, q rowsQuerier, clinicID, id string) (appointment, error) {
	rows, err := q.Query(ctx, appointmentSelect+`WHERE a.clinic_id = $1 AND a.id = $2`, clinicID, id)
	if err != nil {
		return appointment{}, err
	}
	a, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[appointment])
	if err != nil {
		return a, err
	}
	if a.Details, err = decField("appointments", "details", a.ID, a.Details); err != nil {
		return appointment{}, err
	}
	one := []appointment{a}
	decorateAppointments(clinicLocation(ctx, q, clinicID), one)
	return one[0], nil
}

// openAppointmentDetails decrypts the details of rows read with appointmentSelect.
func openAppointmentDetails(list []appointment) error {
	for i := range list {
		d, err := decField("appointments", "details", list[i].ID, list[i].Details)
		if err != nil {
			return err
		}
		list[i].Details = d
	}
	return nil
}

func (a *appointmentIn) validate() string {
	a.CURP = normalizeCURP(a.CURP)
	for _, f := range []*string{&a.Names, &a.LastNames, &a.Date, &a.StartHour, &a.EndHour, &a.Details, &a.Room, &a.Phone, &a.Email} {
		*f = strings.TrimSpace(*f)
	}
	if utf8.RuneCountInString(a.Details) > 2000 || utf8.RuneCountInString(a.Names) > 200 || utf8.RuneCountInString(a.LastNames) > 200 ||
		utf8.RuneCountInString(a.Room) > 40 || utf8.RuneCountInString(a.Phone) > 30 || utf8.RuneCountInString(a.Email) > 200 {
		return "Uno de los campos es demasiado largo."
	}
	if a.Email != "" && !validEmail(a.Email) {
		return "El correo no es válido."
	}
	for _, id := range []**string{&a.PatientID, &a.ProfessionalID, &a.ServiceID} {
		if *id != nil && **id == "" {
			*id = nil
		}
		if *id != nil && !validUUID(**id) {
			return "Uno de los datos seleccionados no es válido."
		}
	}
	// Someone without a registered patient still needs full name; a registered patient (a person or an animal) may have no surname.
	if a.Names == "" || (a.PatientID == nil && a.LastNames == "") || a.Date == "" || a.StartHour == "" || a.EndHour == "" {
		return "Faltan campos por llenar."
	}
	if a.CURP != "" && !curpRe.MatchString(a.CURP) {
		return "La CURP debe tener 18 caracteres alfanuméricos."
	}
	if _, err := time.Parse("2006-01-02", a.Date); err != nil {
		return "La fecha no es válida."
	}
	start, err1 := time.Parse("15:04", a.StartHour)
	end, err2 := time.Parse("15:04", a.EndHour)
	if err1 != nil || err2 != nil {
		return "La hora no es válida."
	}
	if !end.After(start) {
		return "La hora de finalización debe ser posterior a la de inicio."
	}
	return ""
}

// checkRefs verifies the professional and the service belong to the clinic.
func (s *Server) checkRefs(ctx context.Context, q queryRower, clinicID string, a *appointmentIn) string {
	if a.ProfessionalID != nil && !s.isConsulting(ctx, q, clinicID, *a.ProfessionalID) {
		return "El profesional no existe o no está activo en este consultorio."
	}
	if a.ServiceID != nil {
		var ok bool
		if q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM catalog_items WHERE clinic_id = $1 AND id = $2 AND kind = 'service')`, clinicID, *a.ServiceID).Scan(&ok) != nil || !ok {
			return "El servicio no existe en el catálogo de este consultorio."
		}
	}
	return ""
}

// isProfessional is true for an active administrator or doctor of the clinic.
func (s *Server) isProfessional(ctx context.Context, q queryRower, clinicID, userID string) bool {
	var ok bool
	return q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE clinic_id = $1 AND id = $2 AND NOT disabled AND role IN ('admin', 'doctor'))`, clinicID, userID).Scan(&ok) == nil && ok
}

// isConsulting is isProfessional for whoever appointments may be given to: an owner who marked
// "no atiendo consultas" keeps the role but is not offered in the agenda.
func (s *Server) isConsulting(ctx context.Context, q queryRower, clinicID, userID string) bool {
	var ok bool
	return q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users u WHERE u.clinic_id = $1 AND u.id = $2 AND NOT u.disabled AND u.role IN ('admin', 'doctor')
		AND coalesce((SELECT ps.consults FROM professional_settings ps WHERE ps.user_id = u.id), true))`, clinicID, userID).Scan(&ok) == nil && ok
}

// lockSlotDay serializes everything that takes a time of one professional on one day (staff, online booking, the waiting
// list, follow-ups) so two requests cannot take the same slot. It always takes the same two locks in the same order:
// the professional's day first (empty when the visit has no professional), then the clinic's day.
func lockSlotDay(ctx context.Context, tx pgx.Tx, clinicID, professionalID, date string) error {
	if professionalID != "" {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "booking|"+clinicID+"|"+professionalID+"|"+date); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "agenda:"+clinicID+":"+date)
	return err
}

// checkSlot returns a 409 error when the slot is taken (unless overbooking) or blocked.
func (s *Server) checkSlot(ctx context.Context, q queryRower, clinicID string, in *appointmentIn, excludeID string) error {
	code, err := s.slotConflict(ctx, q, clinicID, in.ProfessionalID, in.Room, in.Date, in.StartHour, in.EndHour, excludeID)
	if err != nil {
		return err
	}
	switch {
	case code == SlotBlocked:
		e := fail(http.StatusConflict, "Ese horario está bloqueado en la agenda.")
		e.Code = SlotBlocked
		return e
	case code == SlotTaken && !in.Overbook:
		e := fail(http.StatusConflict, "Ese horario ya está ocupado para el profesional o la sala.")
		e.Code = SlotTaken
		return e
	}
	return nil
}

func (s *Server) listAppointments(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	q := r.URL.Query()
	where, args := []string{"a.clinic_id = $1"}, []any{p.ClinicID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "$$", "$"+itoa(len(args))))
	}
	for _, f := range []struct{ key, cond string }{{"from", "a.date >= $$::date"}, {"to", "a.date <= $$::date"}} {
		if v := q.Get(f.key); v != "" {
			if _, err := time.Parse("2006-01-02", v); err != nil {
				writeError(w, http.StatusBadRequest, "La fecha no es válida.")
				return
			}
			add(f.cond, v)
		}
	}
	for _, f := range []struct{ key, cond string }{{"professional", "a.professional_id = $$::uuid"}, {"patient", "a.patient_id = $$::uuid"}} {
		if v := q.Get(f.key); v != "" {
			if !validUUID(v) {
				writeError(w, http.StatusBadRequest, "El filtro no es válido.")
				return
			}
			add(f.cond, v)
		}
	}
	if v := q.Get("status"); v != "" {
		if !validStatus(v) {
			writeError(w, http.StatusBadRequest, "El estado no es válido.")
			return
		}
		add("a.status = $$", v)
	}
	if v := q.Get("room"); v != "" {
		add("a.room = $$", v)
	}
	// Visits of patients from a giro the clinic no longer works with stay stored but are not shown (as the patients themselves).
	kinds, err := s.clinicKindsFor(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	args = append(args, kinds)
	where = append(where, "(a.patient_id IS NULL OR EXISTS (SELECT 1 FROM patients hp WHERE hp.id = a.patient_id AND "+tagsShown("hp.kinds", len(args))+"))")
	rows, err := s.db.Query(r.Context(), appointmentSelect+`WHERE `+strings.Join(where, " AND ")+` ORDER BY a.date, a.start_hour, a.created_at LIMIT 5000`, args...)
	if err != nil {
		serverError(w, r, err)
		return
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[appointment])
	if err == nil {
		err = openAppointmentDetails(list)
		if err == nil {
			decorateAppointments(clinicLocation(r.Context(), s.db, p.ClinicID), list)
		}
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"appointments": list})
}

func (s *Server) getAppointment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	p := principalFrom(r.Context())
	a, err := loadAppointment(r.Context(), s.db, p.ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"appointment": a})
}

func (s *Server) createAppointment(w http.ResponseWriter, r *http.Request) {
	var f appointmentIn
	if !decode(w, r, &f) {
		return
	}
	// No end time: the service's duration (or the professional's slot) decides it.
	s.slotFillEnd(r.Context(), s.db, principalFrom(r.Context()).ClinicID, &f)
	if msg := f.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	if !s.patientBelongs(r.Context(), p.ClinicID, f.PatientID) {
		writeError(w, http.StatusBadRequest, "El paciente no existe en este consultorio.")
		return
	}
	modality, videoURL, mmsg := s.staffModality(r.Context(), p.ClinicID, f)
	if mmsg != "" {
		writeError(w, http.StatusBadRequest, mmsg)
		return
	}
	if msg := s.checkRefs(r.Context(), s.db, p.ClinicID, &f); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	var out appointment
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if err := lockSlotDay(r.Context(), tx, p.ClinicID, deref(f.ProfessionalID), f.Date); err != nil {
			return err
		}
		if err := s.checkSlot(r.Context(), tx, p.ClinicID, &f, ""); err != nil {
			return err
		}
		if err := s.openInSameArea(r.Context(), tx, p.ClinicID, deref(f.PatientID), f.Email, f.Phone, f.ProfessionalID, ""); err != nil {
			return err
		}
		id := newRowID()
		sealed, err := encField("appointments", "details", id, f.Details)
		if err != nil {
			return err
		}
		err = tx.QueryRow(r.Context(), `
			INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, details, patient_id,
				professional_id, service_id, room, phone, email, confirm_token, reminders_consent, id, modality, video_url)
			VALUES ($1,$2,$3,$4,$5::date,$6::time,$7::time,$8,$9::uuid,$10::uuid,$11::uuid,$12,$13,$14,$15,$16,$17::uuid,$18,$19) RETURNING id::text`,
			p.ClinicID, f.CURP, f.Names, f.LastNames, f.Date, f.StartHour, f.EndHour, sealed, f.PatientID,
			f.ProfessionalID, f.ServiceID, f.Room, f.Phone, f.Email, mustToken(), f.RemindersConsent, id, nilIfEmpty(modality), videoURL).Scan(&id)
		if err != nil {
			return err
		}
		if out, err = loadAppointment(r.Context(), tx, p.ClinicID, id); err != nil {
			return err
		}
		if err := s.scheduleReminders(r.Context(), tx, p.ClinicID, id); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "appointment_create", "Creó una cita", map[string]any{"appointment": id, "date": f.Date, "start": f.StartHour, "overbook": f.Overbook})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	s.mailBooked(r.Context(), p.ClinicID, out.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"appointment": out})
}

func (s *Server) updateAppointment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	var f appointmentIn
	if !decode(w, r, &f) {
		return
	}
	if msg := f.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	if !s.patientBelongs(r.Context(), p.ClinicID, f.PatientID) {
		writeError(w, http.StatusBadRequest, "El paciente no existe en este consultorio.")
		return
	}
	modality, videoURL, mmsg := s.staffModality(r.Context(), p.ClinicID, f)
	if mmsg != "" {
		writeError(w, http.StatusBadRequest, mmsg)
		return
	}
	var out appointment
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		cur, err := loadAppointment(r.Context(), tx, p.ClinicID, id)
		if err != nil {
			return err
		}
		if msg := s.checkRefs(r.Context(), tx, p.ClinicID, &f); msg != "" {
			// A professional who has since been deactivated may stay on an old appointment as long as it is not changed.
			keepsProfessional := f.ProfessionalID != nil && deref(cur.ProfessionalID) == *f.ProfessionalID
			if !keepsProfessional || strings.Contains(msg, "servicio") {
				return fail(http.StatusBadRequest, msg)
			}
		}
		switch cur.Status {
		case "cancelled", "no_show", "completed":
			e := fail(http.StatusConflict, "Una cita cancelada, completada o sin asistencia ya no se puede modificar.")
			e.Code = "APPOINTMENT_CLOSED"
			return e
		}
		if err := lockSlotDay(r.Context(), tx, p.ClinicID, deref(f.ProfessionalID), f.Date); err != nil {
			return err
		}
		sameSlot := cur.Date == f.Date && cur.StartHour == f.StartHour && cur.EndHour == f.EndHour && cur.Room == f.Room &&
			deref(cur.ProfessionalID) == deref(f.ProfessionalID)
		if !sameSlot {
			if err := s.checkSlot(r.Context(), tx, p.ClinicID, &f, id); err != nil {
				return err
			}
			if err := s.openInSameArea(r.Context(), tx, p.ClinicID, deref(f.PatientID), f.Email, f.Phone, f.ProfessionalID, id); err != nil {
				return err
			}
		}
		sealed, err := encField("appointments", "details", id, f.Details)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `
			UPDATE appointments SET curp=$3, names=$4, last_names=$5, date=$6::date, start_hour=$7::time, end_hour=$8::time,
				details=$9, patient_id=$10::uuid, professional_id=$11::uuid, service_id=$12::uuid, room=$13, phone=$14, email=$15,
				confirm_token = coalesce(confirm_token, $16), reminders_consent=$17,
				modality = CASE WHEN $18 <> '' THEN $18 ELSE modality END,
				video_url = CASE WHEN $18 = 'videollamada' THEN coalesce(nullif(video_url, ''), $19) ELSE video_url END, updated_at=now()
			WHERE clinic_id=$1 AND id=$2`,
			p.ClinicID, id, f.CURP, f.Names, f.LastNames, f.Date, f.StartHour, f.EndHour, sealed, f.PatientID,
			f.ProfessionalID, f.ServiceID, f.Room, f.Phone, f.Email, mustToken(), f.RemindersConsent, modality, videoURL); err != nil {
			return err
		}
		if out, err = loadAppointment(r.Context(), tx, p.ClinicID, id); err != nil {
			return err
		}
		if err := s.scheduleReminders(r.Context(), tx, p.ClinicID, id); err != nil {
			return err
		}
		action := "Editó una cita"
		if !sameSlot {
			action = "Reprogramó una cita"
		}
		audit(r.Context(), tx, p.ClinicID, p, "appointment_update", action, map[string]any{"appointment": id, "from": cur.Date + " " + cur.StartHour, "to": f.Date + " " + f.StartHour, "overbook": f.Overbook && !sameSlot})
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	waitlistWake() // the old slot may be free now
	writeJSON(w, http.StatusOK, map[string]any{"appointment": out})
}

func (s *Server) deleteAppointment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	p := principalFrom(r.Context())
	// A deposit paid online goes back to the patient before the appointment disappears (its charge is found through the
	// appointment), and the session taken from a package is given back: deleting must not leave either one behind.
	var exists bool
	if err := s.db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM appointments WHERE clinic_id = $1 AND id = $2)`, p.ClinicID, id).Scan(&exists); err != nil {
		serverError(w, r, err)
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	if err := s.refundDeposit(r.Context(), p.ClinicID, id); err != nil {
		logf(r, "deposit refund on delete %s: %v", id, err)
		writeError(w, http.StatusBadGateway, "No se pudo reembolsar el anticipo de esta cita. Cancélala para reintentarlo o avisa al paciente; la cita no se eliminó.")
		return
	}
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if err := releasePackage(r.Context(), tx, p.ClinicID, id); err != nil {
			return err
		}
		_, err := tx.Exec(r.Context(), `DELETE FROM appointments WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id)
		return err
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "appointment_delete", "Eliminó una cita", map[string]any{"appointment": id})
	waitlistWake()
	w.WriteHeader(http.StatusNoContent)
}

func validUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if c != '-' {
				return false
			}
		case !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'):
			return false
		}
	}
	return true
}

// patientBelongs is true when no patient is given or the patient is one of this clinic's.
func (s *Server) patientBelongs(ctx context.Context, clinicID string, id *string) bool {
	if id == nil {
		return true
	}
	var ok bool
	return s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM patients WHERE clinic_id = $1 AND id = $2)`, clinicID, *id).Scan(&ok) == nil && ok
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// staffModality validates the modality chosen on the agenda and prepares the video link of a video call: the professional's own link,
// or a private room made for the appointment. Empty means the appointment stays as it is.
func (s *Server) staffModality(ctx context.Context, clinicID string, f appointmentIn) (modality, videoURL, msg string) {
	switch f.Modality {
	case "":
		return "", "", ""
	case "presencial":
		return "presencial", "", ""
	case "videollamada":
	default:
		return "", "", "La modalidad debe ser presencial o videollamada."
	}
	kinds, err := s.clinicKindsFor(ctx, clinicID)
	if err != nil {
		return "", "", "No se pudo validar la modalidad."
	}
	var areas []string
	own := ""
	if f.ProfessionalID != nil {
		_ = s.db.QueryRow(ctx, `SELECT coalesce(u.areas, '{}'), coalesce(ps.video_url, '') FROM users u LEFT JOIN professional_settings ps ON ps.user_id = u.id WHERE u.id = $1 AND u.clinic_id = $2`,
			*f.ProfessionalID, clinicID).Scan(&areas, &own)
	}
	if !canVideo(areas, kinds) {
		return "", "", "Este profesional no da consultas por videollamada."
	}
	if f.PatientID != nil {
		var subject string
		_ = s.db.QueryRow(ctx, `SELECT subject FROM patients WHERE id = $1 AND clinic_id = $2`, *f.PatientID, clinicID).Scan(&subject)
		if subject == "animal" {
			return "", "", "La videoconsulta es solo para personas."
		}
	}
	if own != "" {
		return "videollamada", own, ""
	}
	return "videollamada", newVideoRoom(), ""
}
