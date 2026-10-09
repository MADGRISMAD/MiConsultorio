package api

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// waitlistOfferTTL is how long a freed slot is held for the person it was offered to.
const waitlistOfferTTL = 30 * time.Minute

// waitlistEntryTTL is how long someone stays on the list without being served before it lapses.
const waitlistEntryTTL = 60 * 24 * time.Hour

// wlLive is an entry being served, as the staff list shows it.
type wlOfferOut struct {
	ID           string    `json:"id"`
	Professional string    `json:"professional"`
	Date         string    `json:"date"`
	Start        string    `json:"start"`
	End          string    `json:"end"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type wlEntryOut struct {
	ID               string      `json:"id"`
	PatientID        *string     `json:"patient_id"`
	Name             string      `json:"name"`
	Phone            string      `json:"phone"`
	Email            string      `json:"email"`
	ProfessionalID   *string     `json:"professional_id"`
	ProfessionalName string      `json:"professional_name"`
	ServiceID        *string     `json:"service_id"`
	ServiceName      string      `json:"service_name"`
	Days             []int16     `json:"days"`
	FromTime         *string     `json:"from_time"`
	ToTime           *string     `json:"to_time"`
	Notes            string      `json:"notes"`
	Consent          bool        `json:"consent"`
	Status           string      `json:"status"`
	CreatedVia       string      `json:"created_via"`
	CreatedAt        time.Time   `json:"created_at"`
	Offer            *wlOfferOut `json:"offer"`
}

const wlEntrySelect = `
	SELECT e.id::text, e.patient_id::text, e.name, e.phone, e.email, e.professional_id::text, coalesce(u.name, ''),
	       e.service_id::text, coalesce(ci.name, ''), e.days, to_char(e.from_time, 'HH24:MI'), to_char(e.to_time, 'HH24:MI'),
	       e.notes, e.consent, e.status, e.created_via, e.created_at,
	       o.id::text, coalesce(ou.name, ''), to_char(o.date, 'YYYY-MM-DD'), to_char(o.start_hour, 'HH24:MI'), to_char(o.end_hour, 'HH24:MI'), o.expires_at
	FROM waitlist_entries e
	LEFT JOIN users u ON u.id = e.professional_id
	LEFT JOIN catalog_items ci ON ci.id = e.service_id
	LEFT JOIN waitlist_offers o ON o.entry_id = e.id AND o.status = 'offered' AND o.expires_at > now()
	LEFT JOIN users ou ON ou.id = o.professional_id `

func wlScanEntry(row pgx.Row) (wlEntryOut, error) {
	var e wlEntryOut
	var oid, opro, odate, ostart, oend *string
	var oexp *time.Time
	if err := row.Scan(&e.ID, &e.PatientID, &e.Name, &e.Phone, &e.Email, &e.ProfessionalID, &e.ProfessionalName,
		&e.ServiceID, &e.ServiceName, &e.Days, &e.FromTime, &e.ToTime, &e.Notes, &e.Consent, &e.Status, &e.CreatedVia, &e.CreatedAt,
		&oid, &opro, &odate, &ostart, &oend, &oexp); err != nil {
		return e, err
	}
	if e.Days == nil {
		e.Days = []int16{}
	}
	if oid != nil {
		e.Offer = &wlOfferOut{ID: *oid, Professional: *opro, Date: *odate, Start: *ostart, End: *oend, ExpiresAt: *oexp}
	}
	return e, nil
}

func (s *Server) wlLoadEntry(ctx context.Context, q queryRower, clinicID, id string) (wlEntryOut, error) {
	return wlScanEntry(q.QueryRow(ctx, wlEntrySelect+`WHERE e.clinic_id = $1 AND e.id = $2`, clinicID, id))
}

// mountWaitlistStaff: the waitlist as the team sees it.
func (s *Server) mountWaitlistStaff(r chi.Router) {
	view := require(PermNavAppointments, PermAdminAppointments)
	manage := require(PermAdminAppointments)
	r.With(view).Get("/waitlist", s.listWaitlist)
	r.With(manage).Post("/waitlist", s.createWaitlist)
	r.With(manage).Post("/waitlist/scan", s.scanWaitlistNow)
	r.With(manage).Put("/waitlist/{id}", s.updateWaitlist)
	r.With(manage).Delete("/waitlist/{id}", s.deleteWaitlist)
}

func (s *Server) listWaitlist(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	where := `e.clinic_id = $1 AND e.status IN ('waiting', 'offered')`
	switch r.URL.Query().Get("status") {
	case "all":
		where = `e.clinic_id = $1 AND (e.status IN ('waiting', 'offered') OR e.updated_at > now() - interval '14 days')`
	case "":
	default:
		writeError(w, http.StatusBadRequest, "El filtro no es válido.")
		return
	}
	rows, err := s.db.Query(r.Context(), wlEntrySelect+`WHERE `+where+` ORDER BY e.created_at, e.id LIMIT 500`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []wlEntryOut{}
	for rows.Next() {
		e, err := wlScanEntry(rows)
		if err != nil {
			serverError(w, r, err)
			return
		}
		out = append(out, e)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out})
}

type wlIn struct {
	PatientID      *string `json:"patient_id"`
	Name           string  `json:"name"`
	Phone          string  `json:"phone"`
	Email          string  `json:"email"`
	ProfessionalID *string `json:"professional_id"`
	ServiceID      *string `json:"service_id"`
	Days           []int   `json:"days"`
	FromTime       string  `json:"from_time"`
	ToTime         string  `json:"to_time"`
	Notes          string  `json:"notes"`
	Consent        bool    `json:"consent"`
	Status         string  `json:"status"` // update only: waiting, booked or cancelled
}

// validate cleans the input; the phone comes back in E.164 form.
func (in *wlIn) validate() string {
	for _, f := range []*string{&in.Name, &in.Phone, &in.Email, &in.FromTime, &in.ToTime, &in.Notes} {
		*f = strings.TrimSpace(*f)
	}
	in.Email = strings.ToLower(in.Email)
	switch {
	case in.Name == "" || utf8.RuneCountInString(in.Name) > 200:
		return "Escribe el nombre de la persona."
	case utf8.RuneCountInString(in.Notes) > 500:
		return "Las notas son demasiado largas (máximo 500 caracteres)."
	case in.Phone == "" && in.Email == "":
		return "Escribe un teléfono o un correo para poder avisarle."
	}
	if in.Phone != "" {
		if in.Phone = normalizePhoneMX(in.Phone); in.Phone == "" {
			return "El teléfono debe tener 10 dígitos."
		}
	}
	if in.Email != "" && !validEmail(in.Email) {
		return "El correo no es válido."
	}
	for _, id := range []**string{&in.PatientID, &in.ProfessionalID, &in.ServiceID} {
		if *id != nil && **id == "" {
			*id = nil
		}
		if *id != nil && !validUUID(**id) {
			return "Uno de los datos seleccionados no es válido."
		}
	}
	seen := map[int]bool{}
	days := []int{}
	for _, d := range in.Days {
		if d < 0 || d > 6 {
			return "Día de la semana no válido."
		}
		if !seen[d] {
			seen[d] = true
			days = append(days, d)
		}
	}
	sort.Ints(days)
	in.Days = days
	if (in.FromTime == "") != (in.ToTime == "") {
		return "Indica la hora de inicio y la de fin del horario preferido, o deja ambas vacías."
	}
	if in.FromTime != "" {
		a, e1 := time.Parse("15:04", in.FromTime)
		b, e2 := time.Parse("15:04", in.ToTime)
		if e1 != nil || e2 != nil || !b.After(a) {
			return "El horario preferido no es válido."
		}
	}
	switch in.Status {
	case "", "waiting", "booked", "cancelled":
	default:
		return "El estado no es válido."
	}
	return ""
}

// wlCheckRefs verifies the patient, professional and service belong to the clinic.
func (s *Server) wlCheckRefs(ctx context.Context, q queryRower, clinicID string, in *wlIn) string {
	if in.PatientID != nil {
		var ok bool
		if q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM patients WHERE clinic_id = $1 AND id = $2)`, clinicID, *in.PatientID).Scan(&ok) != nil || !ok {
			return "El paciente no existe en este consultorio."
		}
	}
	if in.ProfessionalID != nil && !s.isConsulting(ctx, q, clinicID, *in.ProfessionalID) {
		return "El profesional no existe o no está activo en este consultorio."
	}
	if in.ServiceID != nil {
		var ok bool
		if q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM catalog_items WHERE clinic_id = $1 AND id = $2 AND kind = 'service')`, clinicID, *in.ServiceID).Scan(&ok) != nil || !ok {
			return "El servicio no existe en el catálogo de este consultorio."
		}
	}
	return ""
}

func (s *Server) createWaitlist(w http.ResponseWriter, r *http.Request) {
	var in wlIn
	if !decode(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	if msg := s.wlCheckRefs(r.Context(), s.db, p.ClinicID, &in); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	token, err := newToken()
	if err != nil {
		serverError(w, r, err)
		return
	}
	var out wlEntryOut
	err = inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var id string
		if err := tx.QueryRow(r.Context(), `
			INSERT INTO waitlist_entries (clinic_id, patient_id, name, phone, email, professional_id, service_id, days, from_time, to_time, notes, consent, token, created_via)
			VALUES ($1, $2::uuid, $3, $4, $5, $6::uuid, $7::uuid, $8::smallint[], NULLIF($9, '')::time, NULLIF($10, '')::time, $11, $12, $13, 'staff') RETURNING id::text`,
			p.ClinicID, in.PatientID, in.Name, in.Phone, in.Email, in.ProfessionalID, in.ServiceID, in.Days, in.FromTime, in.ToTime, in.Notes, in.Consent, token).Scan(&id); err != nil {
			return err
		}
		var err error
		if out, err = s.wlLoadEntry(r.Context(), tx, p.ClinicID, id); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "waitlist_create", "Agregó a alguien a la lista de espera", map[string]any{"entry": id})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	waitlistWake()
	writeJSON(w, http.StatusCreated, map[string]any{"entry": out})
}

func (s *Server) updateWaitlist(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "No se encontró a esa persona en la lista.")
		return
	}
	var in wlIn
	if !decode(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	if msg := s.wlCheckRefs(r.Context(), s.db, p.ClinicID, &in); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	var out wlEntryOut
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var cur string
		if err := tx.QueryRow(r.Context(), `SELECT status FROM waitlist_entries WHERE clinic_id = $1 AND id = $2 FOR UPDATE`, p.ClinicID, id).Scan(&cur); err != nil {
			return err
		}
		next := cur
		if in.Status != "" {
			next = in.Status
		}
		if cur == "booked" && next != "booked" {
			return fail(http.StatusConflict, "Esta persona ya agendó su cita.")
		}
		if next == "waiting" && cur == "offered" {
			next = "offered" // the live offer decides
		}
		if _, err := tx.Exec(r.Context(), `
			UPDATE waitlist_entries SET patient_id = $3::uuid, name = $4, phone = $5, email = $6, professional_id = $7::uuid, service_id = $8::uuid,
				days = $9::smallint[], from_time = NULLIF($10, '')::time, to_time = NULLIF($11, '')::time, notes = $12, consent = $13,
				status = $14, updated_at = now()
			WHERE clinic_id = $1 AND id = $2`,
			p.ClinicID, id, in.PatientID, in.Name, in.Phone, in.Email, in.ProfessionalID, in.ServiceID, in.Days, in.FromTime, in.ToTime, in.Notes, in.Consent, next); err != nil {
			return err
		}
		if next == "cancelled" || next == "booked" {
			if _, err := tx.Exec(r.Context(), `UPDATE waitlist_offers SET status = 'cancelled', responded_at = now() WHERE entry_id = $1 AND status = 'offered'`, id); err != nil {
				return err
			}
		}
		var err error
		if out, err = s.wlLoadEntry(r.Context(), tx, p.ClinicID, id); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "waitlist_update", "Editó la lista de espera", map[string]any{"entry": id, "status": next})
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "No se encontró a esa persona en la lista.")
		return
	}
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	waitlistWake()
	writeJSON(w, http.StatusOK, map[string]any{"entry": out})
}

// deleteWaitlist takes someone off the list. The row stays (cancelled) so the history is readable.
func (s *Server) deleteWaitlist(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "No se encontró a esa persona en la lista.")
		return
	}
	p := principalFrom(r.Context())
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `UPDATE waitlist_entries SET status = 'cancelled', updated_at = now() WHERE clinic_id = $1 AND id = $2 AND status IN ('waiting', 'offered')`, p.ClinicID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		if _, err := tx.Exec(r.Context(), `UPDATE waitlist_offers SET status = 'cancelled', responded_at = now() WHERE entry_id = $1 AND status = 'offered'`, id); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "waitlist_delete", "Quitó a alguien de la lista de espera", map[string]any{"entry": id})
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "No se encontró a esa persona en la lista.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	waitlistWake()
	w.WriteHeader(http.StatusNoContent)
}

// scanWaitlistNow is POST /waitlist/scan: look for free slots for the list right now.
func (s *Server) scanWaitlistNow(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	n, err := s.wlScanClinic(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"offers": n})
}
