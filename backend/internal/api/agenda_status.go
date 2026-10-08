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

// Allowed status changes of an appointment. A closed appointment (completed, no_show, cancelled) is never reopened.
var statusTransitions = map[string][]string{
	"scheduled":   {"confirmed", "arrived", "cancelled", "no_show"},
	"confirmed":   {"arrived", "cancelled", "no_show"},
	"arrived":     {"in_progress", "cancelled", "no_show"},
	"in_progress": {"completed"},
}

func validStatus(s string) bool {
	switch s {
	case "scheduled", "confirmed", "arrived", "in_progress", "completed", "no_show", "cancelled":
		return true
	}
	return false
}

var statusLabels = map[string]string{
	"confirmed": "Confirmó", "arrived": "Marcó como llegada", "in_progress": "Inició la consulta de", "completed": "Terminó la consulta de",
	"no_show": "Marcó como no asistió", "cancelled": "Canceló",
}

// changeAppointmentStatus is POST /appointments/{id}/status. Administrators and reception may change any appointment;
// a doctor only the ones assigned to them; cashiers never.
func (s *Server) changeAppointmentStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	var in struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !validStatus(in.Status) || in.Status == "scheduled" {
		writeError(w, http.StatusBadRequest, "El estado no es válido.")
		return
	}
	if utf8.RuneCountInString(in.Reason) > 500 {
		writeError(w, http.StatusBadRequest, "El motivo es demasiado largo.")
		return
	}
	p := principalFrom(r.Context())
	var out appointment
	var chargeID string
	var chargeCents int
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var cur, prof, date, start string
		err := tx.QueryRow(r.Context(), `SELECT status, coalesce(professional_id::text, ''), to_char(date, 'YYYY-MM-DD'), to_char(start_hour, 'HH24:MI')
			FROM appointments WHERE clinic_id = $1 AND id = $2 FOR UPDATE`, p.ClinicID, id).Scan(&cur, &prof, &date, &start)
		if err != nil {
			return err
		}
		if !hasPermission(p.Permissions, PermAdminAppointments) && !(p.Role == RoleDoctor && prof == p.UserID) {
			return fail(http.StatusForbidden, "No tienes permiso para cambiar esta cita.")
		}
		if !contains(statusTransitions[cur], in.Status) {
			e := fail(http.StatusConflict, "No se puede pasar una cita de «"+statusName(cur)+"» a «"+statusName(in.Status)+"».")
			e.Code = "INVALID_TRANSITION"
			return e
		}
		if he := apptTimeGate(clinicLocation(r.Context(), tx, p.ClinicID), date, start, in.Status, apptNow()); he != nil {
			return he
		}
		if _, err := tx.Exec(r.Context(), `
			UPDATE appointments SET status = $3,
				arrived_at  = CASE WHEN $3 IN ('arrived', 'in_progress') THEN coalesce(arrived_at, now()) ELSE arrived_at END,
				started_at  = CASE WHEN $3 = 'in_progress' THEN now() ELSE started_at END,
				finished_at = CASE WHEN $3 = 'completed' THEN now() ELSE finished_at END,
				cancel_reason = CASE WHEN $3 IN ('cancelled', 'no_show') THEN $4 ELSE cancel_reason END,
				updated_at = now()
			WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id, in.Status, in.Reason); err != nil {
			return err
		}
		if out, err = loadAppointment(r.Context(), tx, p.ClinicID, id); err != nil {
			return err
		}
		if err := s.scheduleReminders(r.Context(), tx, p.ClinicID, id); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "appointment_status", statusLabels[in.Status]+" una cita",
			map[string]any{"appointment": id, "from": cur, "to": in.Status, "reason": in.Reason})
		if in.Status == "completed" {
			chargeID, chargeCents = s.tryAutoCharge(r.Context(), tx, p, id)
		}
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
	if in.Status == "cancelled" || in.Status == "no_show" {
		waitlistWake()
	}
	resp := map[string]any{"appointment": out}
	if chargeID != "" {
		resp["charge"] = map[string]any{"id": chargeID, "total_cents": chargeCents}
	}
	writeJSON(w, http.StatusOK, resp)
}

// apptNow is the clock the appointment rules use (tests move it).
var apptNow = time.Now

// linkEncounterToAppointment ties a new bitácora entry to the appointment it came from and closes the appointment.
func (s *Server) linkEncounterToAppointment(ctx context.Context, tx pgx.Tx, p *Principal, patientID, apptID, encounterID string) error {
	var cur, date, start string
	var apptPatient *string
	err := tx.QueryRow(ctx, `SELECT status, patient_id::text, to_char(date, 'YYYY-MM-DD'), to_char(start_hour, 'HH24:MI')
		FROM appointments WHERE clinic_id = $1 AND id = $2 FOR UPDATE`, p.ClinicID, apptID).Scan(&cur, &apptPatient, &date, &start)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(http.StatusBadRequest, "La cita no existe en este consultorio.")
	}
	if err != nil {
		return err
	}
	if apptPatient != nil && *apptPatient != patientID {
		return fail(http.StatusBadRequest, "La cita pertenece a otro paciente.")
	}
	if apptTimeGate(clinicLocation(ctx, tx, p.ClinicID), date, start, "in_progress", apptNow()) != nil {
		// A note written before the visit's time is kept with the appointment, but does not close it.
		_, err := tx.Exec(ctx, `UPDATE appointments SET encounter_id = coalesce(encounter_id, $3::uuid), updated_at = now() WHERE clinic_id = $1 AND id = $2`, p.ClinicID, apptID, encounterID)
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE appointments SET encounter_id = coalesce(encounter_id, $3::uuid), status = 'completed',
			arrived_at = coalesce(arrived_at, now()), started_at = coalesce(started_at, now()), finished_at = coalesce(finished_at, now()), updated_at = now()
		WHERE clinic_id = $1 AND id = $2`, p.ClinicID, apptID, encounterID); err != nil {
		return err
	}
	if cur != "completed" {
		if err := s.scheduleReminders(ctx, tx, p.ClinicID, apptID); err != nil {
			return err
		}
		audit(ctx, tx, p.ClinicID, p, "appointment_status", "Terminó la consulta de una cita", map[string]any{"appointment": apptID, "from": cur, "to": "completed"})
		s.tryAutoCharge(ctx, tx, p, apptID)
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func statusName(s string) string {
	switch s {
	case "scheduled":
		return "Programada"
	case "confirmed":
		return "Confirmada"
	case "arrived":
		return "Llegó"
	case "in_progress":
		return "En consulta"
	case "completed":
		return "Completada"
	case "no_show":
		return "No asistió"
	case "cancelled":
		return "Cancelada"
	}
	return s
}
