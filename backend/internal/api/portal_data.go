package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// What the portal shows is an allow-list: appointments, recetas (without diagnosis) and the vaccination
// card. Notes, bitácora, exploración, diagnósticos and private notes never leave through these routes.

type portalPatient struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Species string `json:"species,omitempty"`
}

// portalPatients are the active patients of the clinic whose own or guardian's e-mail is this one.
func (s *Server) portalPatients(ctx context.Context, clinicID, email string) ([]portalPatient, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, btrim(names || ' ' || last_names), subject, coalesce(profile->>'species', '')
		FROM patients
		WHERE clinic_id = $1 AND archived_at IS NULL AND (lower(btrim(email)) = $2 OR lower(btrim(guardian_email)) = $2)
		ORDER BY file_number`, clinicID, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []portalPatient{}
	for rows.Next() {
		var p portalPatient
		if err := rows.Scan(&p.ID, &p.Name, &p.Subject, &p.Species); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func patientIDs(list []portalPatient) []string {
	ids := make([]string, len(list))
	for i, p := range list {
		ids[i] = p.ID
	}
	return ids
}

func patientNames(list []portalPatient) map[string]string {
	m := map[string]string{}
	for _, p := range list {
		m[p.ID] = p.Name
	}
	return m
}

// portalLogAccess records that the portal read each patient's data (record_access, actor "Portal (correo)").
func (s *Server) portalLogAccess(ctx context.Context, sess *portalSession, ids []string) {
	if len(ids) == 0 {
		return
	}
	_, _ = s.db.Exec(ctx, `INSERT INTO record_access (clinic_id, patient_id, user_name, action)
		SELECT $1, unnest($2::uuid[]), $3, 'view'`, sess.ClinicID, ids, "Portal ("+sess.Email+")")
}

// portalContext loads the clinic and the linked patients, answering the request itself on failure.
func (s *Server) portalContext(w http.ResponseWriter, r *http.Request) (*portalSession, *portalClinic, []portalPatient, bool) {
	sess := portalFrom(r.Context())
	c, err := s.loadPortalClinic(r.Context(), "", sess.ClinicID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Portal no disponible.")
		return nil, nil, nil, false
	}
	if err != nil {
		serverError(w, r, err)
		return nil, nil, nil, false
	}
	pts, err := s.portalPatients(r.Context(), sess.ClinicID, sess.Email)
	if err != nil {
		serverError(w, r, err)
		return nil, nil, nil, false
	}
	return sess, c, pts, true
}

func (s *Server) portalMe(w http.ResponseWriter, r *http.Request) {
	sess, c, pts, ok := s.portalContext(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"email":    sess.Email,
		"clinic":   map[string]any{"name": c.Name, "address": c.Address, "phone": c.Phone, "welcome": c.Welcome, "slug": c.Slug, "cancel_min_hours": c.CancelMinHours},
		"patients": pts,
	})
}

type portalAppt struct {
	ID           string `json:"id"`
	PatientID    string `json:"patient_id"`
	PatientName  string `json:"patient_name"`
	Date         string `json:"date"`
	StartHour    string `json:"start_hour"`
	EndHour      string `json:"end_hour"`
	Status       string `json:"status"`
	Professional string `json:"professional"`
	Service      string `json:"service"`
	CanCancel    bool   `json:"can_cancel"`
}

func (s *Server) portalAppointments(w http.ResponseWriter, r *http.Request) {
	sess, c, pts, ok := s.portalContext(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT a.id::text, coalesce(a.patient_id::text, ''), btrim(a.names || ' ' || a.last_names), to_char(a.date, 'YYYY-MM-DD'),
		       to_char(a.start_hour, 'HH24:MI'), to_char(a.end_hour, 'HH24:MI'), a.status, coalesce(u.name, ''), coalesce(ci.name, '')
		FROM appointments a
		LEFT JOIN users u ON u.id = a.professional_id AND u.clinic_id = a.clinic_id
		LEFT JOIN catalog_items ci ON ci.id = a.service_id AND ci.clinic_id = a.clinic_id
		WHERE a.clinic_id = $1 AND (a.patient_id = ANY($2::uuid[]) OR (a.patient_id IS NULL AND lower(btrim(a.email)) = $3))
		ORDER BY a.date DESC, a.start_hour DESC LIMIT 300`, sess.ClinicID, patientIDs(pts), sess.Email)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	names := patientNames(pts)
	now := time.Now()
	upcoming, history := []portalAppt{}, []portalAppt{}
	for rows.Next() {
		var a portalAppt
		if err := rows.Scan(&a.ID, &a.PatientID, &a.PatientName, &a.Date, &a.StartHour, &a.EndHour, &a.Status, &a.Professional, &a.Service); err != nil {
			serverError(w, r, err)
			return
		}
		if n := names[a.PatientID]; n != "" {
			a.PatientName = n
		}
		start, err := localTime(c.Loc, a.Date, a.StartHour)
		live := a.Status == "scheduled" || a.Status == "confirmed"
		if err == nil && live && start.After(now) {
			a.CanCancel = start.Sub(now) >= time.Duration(c.CancelMinHours)*time.Hour
			upcoming = append(upcoming, a)
		} else {
			history = append(history, a)
		}
	}
	if err := rows.Err(); err != nil {
		serverError(w, r, err)
		return
	}
	// rows came newest first; upcoming reads better soonest first
	for i, j := 0, len(upcoming)-1; i < j; i, j = i+1, j-1 {
		upcoming[i], upcoming[j] = upcoming[j], upcoming[i]
	}
	s.portalLogAccess(r.Context(), sess, patientIDs(pts))
	writeJSON(w, http.StatusOK, map[string]any{"upcoming": upcoming, "history": history, "cancel_min_hours": c.CancelMinHours})
}

func (s *Server) portalCancelAppointment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if r.ContentLength != 0 && !decode(w, r, &req) {
		return
	}
	reason := truncate(strings.TrimSpace(req.Reason), 300)
	sess, c, pts, ok := s.portalContext(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var refused string
	missing := false
	err := inTx(ctx, s.db, func(tx pgx.Tx) error {
		var status, date, hour string
		err := tx.QueryRow(ctx, `
			SELECT status, to_char(date, 'YYYY-MM-DD'), to_char(start_hour, 'HH24:MI') FROM appointments
			WHERE clinic_id = $1 AND id = $2 AND (patient_id = ANY($3::uuid[]) OR (patient_id IS NULL AND lower(btrim(email)) = $4))
			FOR UPDATE`, sess.ClinicID, id, patientIDs(pts), sess.Email).Scan(&status, &date, &hour)
		if errors.Is(err, pgx.ErrNoRows) {
			missing = true
			return nil
		}
		if err != nil {
			return err
		}
		start, err := localTime(c.Loc, date, hour)
		now := time.Now()
		switch {
		case err != nil || (status != "scheduled" && status != "confirmed") || !start.After(now):
			refused = "Esta cita ya no se puede cancelar."
			return nil
		case start.Sub(now) < time.Duration(c.CancelMinHours)*time.Hour:
			refused = "Ya faltan menos de " + itoa(c.CancelMinHours) + " horas para tu cita. Para cancelar, comunícate con el consultorio."
			return nil
		}
		why := "Paciente (portal)"
		if reason != "" {
			why += ": " + reason
		}
		if _, err := tx.Exec(ctx, `UPDATE appointments SET status = 'cancelled', cancel_reason = $3, updated_at = now() WHERE id = $1 AND clinic_id = $2`, id, sess.ClinicID, why); err != nil {
			return err
		}
		if err := s.scheduleReminders(ctx, tx, sess.ClinicID, id); err != nil {
			return err
		}
		portalAudit(ctx, tx, sess.ClinicID, sess.Email, "appointment_cancelled", "Cita cancelada por el paciente desde el portal", map[string]any{"appointment_id": id})
		s.ntfAppointmentByID(ctx, tx, sess.ClinicID, id, "appointment_cancelled_patient", "Un paciente canceló su cita", "/agenda")
		return nil
	})
	switch {
	case err != nil:
		serverError(w, r, err)
	case missing:
		writeError(w, http.StatusNotFound, "Cita no encontrada.")
	case refused != "":
		writeError(w, http.StatusConflict, refused)
	default:
		waitlistWake()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

type portalRx struct {
	ID                string    `json:"id"`
	PatientID         string    `json:"patient_id"`
	PatientName       string    `json:"patient_name"`
	Folio             int       `json:"folio"`
	Mode              string    `json:"mode"`
	IssuedAt          time.Time `json:"issued_at"`
	ValidUntil        *string   `json:"valid_until"`
	Items             []rxItem  `json:"items"`
	Instructions      string    `json:"instructions"`
	NextVisit         *string   `json:"next_visit"`
	AuthorName        string    `json:"author_name"`
	AuthorTitle       string    `json:"author_title"`
	AuthorLicense     string    `json:"author_license"`
	AuthorInstitution string    `json:"author_institution"`
	Voided            bool      `json:"voided"`
	VoidedAt          *string   `json:"voided_at"`
}

func (s *Server) portalPrescriptions(w http.ResponseWriter, r *http.Request) {
	sess, c, pts, ok := s.portalContext(w, r)
	if !ok {
		return
	}
	names := patientNames(pts)
	rows, err := s.db.Query(r.Context(), `
		SELECT id::text, patient_id::text, folio, mode, issued_at, to_char(valid_until, 'YYYY-MM-DD'), items, instructions,
		       to_char(next_visit, 'YYYY-MM-DD'), author_name, author_title, author_license, author_institution, to_char(voided_at AT TIME ZONE 'UTC', 'YYYY-MM-DD')
		FROM prescriptions WHERE clinic_id = $1 AND patient_id = ANY($2::uuid[]) ORDER BY issued_at DESC LIMIT 200`, sess.ClinicID, patientIDs(pts))
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	list := []portalRx{}
	for rows.Next() {
		var x portalRx
		var raw []byte
		if err := rows.Scan(&x.ID, &x.PatientID, &x.Folio, &x.Mode, &x.IssuedAt, &x.ValidUntil, &raw, &x.Instructions, &x.NextVisit,
			&x.AuthorName, &x.AuthorTitle, &x.AuthorLicense, &x.AuthorInstitution, &x.VoidedAt); err != nil {
			serverError(w, r, err)
			return
		}
		x.Items = []rxItem{}
		_ = json.Unmarshal(raw, &x.Items)
		x.Voided, x.PatientName = x.VoidedAt != nil, names[x.PatientID]
		list = append(list, x)
	}
	if err := rows.Err(); err != nil {
		serverError(w, r, err)
		return
	}
	s.portalLogAccess(r.Context(), sess, patientIDs(pts))
	writeJSON(w, http.StatusOK, map[string]any{
		"prescriptions": list,
		"clinic":        map[string]any{"name": c.Name, "address": c.Address, "phone": c.Phone},
	})
}

type portalVaccine struct {
	ID          string  `json:"id"`
	PatientID   string  `json:"patient_id"`
	PatientName string  `json:"patient_name"`
	Kind        string  `json:"kind"`
	Name        string  `json:"name"`
	AppliedOn   string  `json:"applied_on"`
	NextDue     *string `json:"next_due"`
	Lot         string  `json:"lot"`
	Dose        string  `json:"dose"`
	AppliedBy   string  `json:"administered_by_name"`
}

func (s *Server) portalVaccinations(w http.ResponseWriter, r *http.Request) {
	sess, _, pts, ok := s.portalContext(w, r)
	if !ok {
		return
	}
	names := patientNames(pts)
	rows, err := s.db.Query(r.Context(), `
		SELECT id::text, patient_id::text, kind, name, to_char(applied_on, 'YYYY-MM-DD'), to_char(next_due, 'YYYY-MM-DD'), lot, dose, administered_by_name
		FROM vaccinations WHERE clinic_id = $1 AND patient_id = ANY($2::uuid[]) AND voided_at IS NULL
		ORDER BY applied_on DESC LIMIT 500`, sess.ClinicID, patientIDs(pts))
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	list := []portalVaccine{}
	for rows.Next() {
		var v portalVaccine
		if err := rows.Scan(&v.ID, &v.PatientID, &v.Kind, &v.Name, &v.AppliedOn, &v.NextDue, &v.Lot, &v.Dose, &v.AppliedBy); err != nil {
			serverError(w, r, err)
			return
		}
		v.PatientName = names[v.PatientID]
		list = append(list, v)
	}
	if err := rows.Err(); err != nil {
		serverError(w, r, err)
		return
	}
	s.portalLogAccess(r.Context(), sess, patientIDs(pts))
	writeJSON(w, http.StatusOK, map[string]any{"vaccinations": list})
}

// ---------------------------------------------------------------------------
// Clinic settings (staff side)
// ---------------------------------------------------------------------------

func (s *Server) getPortalSettings(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	var enabled bool
	var welcome, slug string
	err := s.db.QueryRow(r.Context(), `SELECT portal_enabled, portal_welcome, coalesce(booking_slug, '') FROM agenda_settings WHERE clinic_id = $1`, p.ClinicID).Scan(&enabled, &welcome, &slug)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"portal": map[string]any{"enabled": enabled, "welcome": welcome, "slug": slug}})
}

func (s *Server) updatePortalSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool   `json:"enabled"`
		Welcome string `json:"welcome"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Welcome = strings.TrimSpace(req.Welcome)
	if utf8.RuneCountInString(req.Welcome) > 600 {
		writeError(w, http.StatusBadRequest, "El mensaje de bienvenida es demasiado largo (máximo 600 caracteres).")
		return
	}
	p := principalFrom(r.Context())
	var slug string
	_ = s.db.QueryRow(r.Context(), `SELECT coalesce(booking_slug, '') FROM agenda_settings WHERE clinic_id = $1`, p.ClinicID).Scan(&slug)
	if req.Enabled && slug == "" {
		writeJSON(w, http.StatusConflict, errorBody{Code: "SLUG_REQUIRED", Message: "Primero define la dirección de reserva en línea en los ajustes de la agenda: el portal usa la misma dirección."})
		return
	}
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO agenda_settings (clinic_id, portal_enabled, portal_welcome) VALUES ($1, $2, $3)
			ON CONFLICT (clinic_id) DO UPDATE SET portal_enabled = $2, portal_welcome = $3, updated_at = now()`, p.ClinicID, req.Enabled, req.Welcome); err != nil {
			return err
		}
		msg := "Desactivó el portal del paciente"
		if req.Enabled {
			msg = "Activó el portal del paciente"
		}
		audit(r.Context(), tx, p.ClinicID, p, "portal_settings", msg, nil)
		return nil
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"portal": map[string]any{"enabled": req.Enabled, "welcome": req.Welcome, "slug": slug}})
}
