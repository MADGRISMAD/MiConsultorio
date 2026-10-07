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

type appointmentFields struct {
	PatientID *string `json:"patient_id" db:"patient_id"`
	Names     string  `json:"names" db:"names"`
	LastNames string  `json:"last_names" db:"last_names"`
	CURP      string  `json:"CURP" db:"curp"`
	Date      string  `json:"date" db:"date"`
	StartHour string  `json:"startHour" db:"start_hour"`
	EndHour   string  `json:"endHour" db:"end_hour"`
	Details   string  `json:"details" db:"details"`
}

type appointment struct {
	ID string `json:"id" db:"id"`
	appointmentFields
}

const appointmentCols = `id, patient_id::text AS patient_id, curp, names, last_names, to_char(date, 'YYYY-MM-DD') AS date,
	to_char(start_hour, 'HH24:MI') AS start_hour, to_char(end_hour, 'HH24:MI') AS end_hour, details`

func (a *appointmentFields) validate() string {
	a.CURP = normalizeCURP(a.CURP)
	for _, f := range []*string{&a.Names, &a.LastNames, &a.Date, &a.StartHour, &a.EndHour, &a.Details} {
		*f = strings.TrimSpace(*f)
	}
	if utf8.RuneCountInString(a.Details) > 2000 || utf8.RuneCountInString(a.Names) > 200 || utf8.RuneCountInString(a.LastNames) > 200 {
		return "Uno de los campos es demasiado largo."
	}
	if a.PatientID != nil && *a.PatientID == "" {
		a.PatientID = nil
	}
	if a.PatientID != nil && !validUUID(*a.PatientID) {
		return "El paciente no es válido."
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

func (s *Server) listAppointments(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(),
		`SELECT `+appointmentCols+` FROM appointments WHERE clinic_id = $1 ORDER BY date, start_hour`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[appointment])
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
	rows, err := s.db.Query(r.Context(),
		`SELECT `+appointmentCols+` FROM appointments WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	a, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[appointment])
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
	var f appointmentFields
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
	rows, err := s.db.Query(r.Context(), `
		INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, details, patient_id)
		VALUES ($1,$2,$3,$4,$5::date,$6::time,$7::time,$8,$9::uuid) RETURNING `+appointmentCols,
		p.ClinicID, f.CURP, f.Names, f.LastNames, f.Date, f.StartHour, f.EndHour, f.Details, f.PatientID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	a, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[appointment])
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"appointment": a})
}

func (s *Server) updateAppointment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	var f appointmentFields
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
	rows, err := s.db.Query(r.Context(), `
		UPDATE appointments SET curp=$3, names=$4, last_names=$5, date=$6::date, start_hour=$7::time, end_hour=$8::time,
			details=$9, patient_id=$10::uuid, updated_at=now()
		WHERE clinic_id=$1 AND id=$2 RETURNING `+appointmentCols,
		p.ClinicID, id, f.CURP, f.Names, f.LastNames, f.Date, f.StartHour, f.EndHour, f.Details, f.PatientID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	a, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[appointment])
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

func (s *Server) deleteAppointment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	p := principalFrom(r.Context())
	tag, err := s.db.Exec(r.Context(), `DELETE FROM appointments WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
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
