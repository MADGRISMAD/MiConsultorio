package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Chronic medication: what a patient takes every day. It never changes by itself: a medicine is added, edited, renewed
// or stopped (with a reason), and a stopped one stays in the list as history. Adding one checks it against the allergies
// and the other medicines; the same list feeds the interaction check when a receta is written.

type chronicMed struct {
	ID            string     `json:"id"`
	PatientID     string     `json:"patient_id"`
	Name          string     `json:"name"`
	Dose          string     `json:"dose"`
	Frequency     string     `json:"frequency"`
	Indication    string     `json:"indication"`
	StartedOn     *string    `json:"started_on"`
	NextRenewal   *string    `json:"next_renewal"`
	Active        bool       `json:"active"`
	StoppedAt     *time.Time `json:"stopped_at"`
	StoppedReason string     `json:"stopped_reason"`
	CreatedBy     string     `json:"created_by_name"`
	CreatedAt     time.Time  `json:"created_at"`
}

const chronicCols = `id::text, patient_id::text, name, dose, frequency, indication, to_char(started_on, 'YYYY-MM-DD'), to_char(next_renewal, 'YYYY-MM-DD'),
	active, stopped_at, stopped_reason, created_by_name, created_at`

func scanChronic(row pgx.Row) (chronicMed, error) {
	var m chronicMed
	err := row.Scan(&m.ID, &m.PatientID, &m.Name, &m.Dose, &m.Frequency, &m.Indication, &m.StartedOn, &m.NextRenewal, &m.Active, &m.StoppedAt, &m.StoppedReason, &m.CreatedBy, &m.CreatedAt)
	return m, err
}

func (s *Server) chronicOf(ctx context.Context, q rowsQuerier, clinicID, patientID string) ([]chronicMed, error) {
	rows, err := q.Query(ctx, `SELECT `+chronicCols+` FROM chronic_medications WHERE clinic_id = $1 AND patient_id = $2 ORDER BY active DESC, name`, clinicID, patientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []chronicMed{}
	for rows.Next() {
		m, err := scanChronic(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// otherDrugs are the medicines a patient already takes: the active chronic ones and those of the recetas still valid.
func (s *Server) otherDrugs(ctx context.Context, q rowsQuerier, clinicID, patientID string) ([]drugRef, []chronicMed, error) {
	chronic, err := s.chronicOf(ctx, q, clinicID, patientID)
	if err != nil {
		return nil, nil, err
	}
	var out []drugRef
	for _, m := range chronic {
		if m.Active {
			out = append(out, drugRef{m.Name, "medicación crónica"})
		}
	}
	rows, err := q.Query(ctx, `SELECT folio, items FROM prescriptions WHERE clinic_id = $1 AND patient_id = $2 AND voided_at IS NULL AND superseded_at IS NULL
		AND (valid_until IS NULL OR valid_until >= current_date) AND mode = 'medication' ORDER BY issued_at DESC LIMIT 20`, clinicID, patientID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var folio int
		var raw []byte
		if err := rows.Scan(&folio, &raw); err != nil {
			return nil, nil, err
		}
		var items []rxItem
		_ = json.Unmarshal(raw, &items)
		for _, it := range items {
			out = append(out, drugRef{it.Medicine, "receta vigente #" + itoa(folio)})
		}
	}
	return out, chronic, rows.Err()
}

// rxSafety checks what a receta is about to prescribe against the patient's allergies and the medicines they already take.
func (s *Server) rxSafety(ctx context.Context, clinicID string, pat patient, items []rxItem) (allergies []allergyConflict, hits []interactionHit, chronic []chronicMed, err error) {
	others, chronic, err := s.otherDrugs(ctx, s.db, clinicID, pat.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Medicine)
	}
	return allergyConflicts(patientAllergies(pat.Profile), items), interactionsBetween(names, others), chronic, nil
}

// rxCheck lets the screen show the alerts while the receta is being written, before anything is saved.
func (s *Server) rxCheck(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	var in struct {
		Items []rxItem `json:"items"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Items) > 15 {
		writeError(w, http.StatusBadRequest, "Demasiados medicamentos.")
		return
	}
	p := principalFrom(r.Context())
	pat, err := loadPatient(r.Context(), s.db, p.ClinicID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	var items []rxItem
	for _, it := range in.Items {
		if strings.TrimSpace(it.Medicine) != "" {
			items = append(items, rxItem{Medicine: it.Medicine, Brand: it.Brand})
		}
	}
	allergies, hits, chronic, err := s.rxSafety(r.Context(), p.ClinicID, pat, items)
	if err != nil {
		serverError(w, r, err)
		return
	}
	active := []chronicMed{}
	for _, m := range chronic {
		if m.Active {
			active = append(active, m)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"allergies": nilToEmpty(allergies), "interactions": nilHits(hits), "chronic": active})
}

func nilToEmpty(a []allergyConflict) []allergyConflict {
	if a == nil {
		return []allergyConflict{}
	}
	return a
}
func nilHits(h []interactionHit) []interactionHit {
	if h == nil {
		return []interactionHit{}
	}
	return h
}

// ---- CRUD ------------------------------------------------------------------------------------

type chronicIn struct {
	Name          *string `json:"name"`
	Dose          *string `json:"dose"`
	Frequency     *string `json:"frequency"`
	Indication    *string `json:"indication"`
	StartedOn     *string `json:"started_on"`
	NextRenewal   *string `json:"next_renewal"`
	Stop          bool    `json:"stop"`
	StoppedReason *string `json:"stopped_reason"`
}

func cleanDate(p *string) (any, bool) {
	if p == nil || strings.TrimSpace(*p) == "" {
		return nil, true
	}
	if _, err := time.Parse("2006-01-02", strings.TrimSpace(*p)); err != nil {
		return nil, false
	}
	return strings.TrimSpace(*p), true
}

func (s *Server) listChronicMeds(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	if _, err := s.patientExists(r.Context(), p.ClinicID, id); err != nil {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	list, err := s.chronicOf(r.Context(), s.db, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"medications": list})
}

// checkChronic returns the alerts of a medicine just added: allergies and interactions with what the patient already takes.
func (s *Server) checkChronic(ctx context.Context, clinicID string, pat patient, m chronicMed) map[string]any {
	others, _, err := s.otherDrugs(ctx, s.db, clinicID, pat.ID)
	if err != nil {
		return map[string]any{}
	}
	var rest []drugRef
	for _, o := range others {
		if normText(o.Name) != normText(m.Name) {
			rest = append(rest, o)
		}
	}
	return map[string]any{
		"allergies":    nilToEmpty(allergyConflicts(patientAllergies(pat.Profile), []rxItem{{Medicine: m.Name}})),
		"interactions": nilHits(interactionsBetween([]string{m.Name}, rest)),
	}
}

func (s *Server) createChronicMed(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	var in chronicIn
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	pat, err := loadPatient(r.Context(), s.db, p.ClinicID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	if pat.Subject != "person" {
		writeError(w, http.StatusBadRequest, "La medicación crónica es para personas.")
		return
	}
	str := func(v *string) string {
		if v == nil {
			return ""
		}
		return strings.TrimSpace(*v)
	}
	name, dose, freq, ind := str(in.Name), str(in.Dose), str(in.Frequency), str(in.Indication)
	started, ok1 := cleanDate(in.StartedOn)
	renewal, ok2 := cleanDate(in.NextRenewal)
	switch {
	case utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 120:
		writeError(w, http.StatusBadRequest, "Escribe el nombre del medicamento (el genérico, de preferencia).")
		return
	case utf8.RuneCountInString(dose) > 120 || utf8.RuneCountInString(freq) > 100 || utf8.RuneCountInString(ind) > 200:
		writeError(w, http.StatusBadRequest, "Uno de los textos es demasiado largo.")
		return
	case !ok1 || !ok2:
		writeError(w, http.StatusBadRequest, "Una de las fechas no es válida.")
		return
	}
	row := s.db.QueryRow(r.Context(), `INSERT INTO chronic_medications (clinic_id, patient_id, name, dose, frequency, indication, started_on, next_renewal, created_by_name)
		VALUES ($1,$2,$3,$4,$5,$6,$7::date,$8::date,$9) RETURNING `+chronicCols, p.ClinicID, id, name, dose, freq, ind, started, renewal, p.actorName())
	m, err := scanChronic(row)
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "chronic_med_added", "Agregó medicación crónica: "+name, map[string]any{"patient": id})
	writeJSON(w, http.StatusCreated, map[string]any{"medication": m, "alerts": s.checkChronic(r.Context(), p.ClinicID, pat, m)})
}

func (s *Server) updateChronicMed(w http.ResponseWriter, r *http.Request) {
	id, mid := chi.URLParam(r, "id"), chi.URLParam(r, "mid")
	if !validUUID(id) || !validUUID(mid) {
		writeError(w, http.StatusNotFound, "Medicamento no encontrado.")
		return
	}
	var in chronicIn
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	cur, err := scanChronic(s.db.QueryRow(r.Context(), `SELECT `+chronicCols+` FROM chronic_medications WHERE clinic_id = $1 AND patient_id = $2 AND id = $3`, p.ClinicID, id, mid))
	if err != nil {
		writeError(w, http.StatusNotFound, "Medicamento no encontrado.")
		return
	}
	pick := func(v *string, old string, max int) (string, bool) {
		if v == nil {
			return old, true
		}
		t := strings.TrimSpace(*v)
		return t, utf8.RuneCountInString(t) <= max
	}
	name, ok1 := pick(in.Name, cur.Name, 120)
	dose, ok2 := pick(in.Dose, cur.Dose, 120)
	freq, ok3 := pick(in.Frequency, cur.Frequency, 100)
	ind, ok4 := pick(in.Indication, cur.Indication, 200)
	reason, ok5 := pick(in.StoppedReason, cur.StoppedReason, 200)
	if !(ok1 && ok2 && ok3 && ok4 && ok5) || utf8.RuneCountInString(name) < 2 {
		writeError(w, http.StatusBadRequest, "Revisa los textos del medicamento.")
		return
	}
	started, renewal := any(cur.StartedOn), any(cur.NextRenewal)
	if in.StartedOn != nil {
		var ok bool
		if started, ok = cleanDate(in.StartedOn); !ok {
			writeError(w, http.StatusBadRequest, "La fecha de inicio no es válida.")
			return
		}
	}
	if in.NextRenewal != nil {
		var ok bool
		if renewal, ok = cleanDate(in.NextRenewal); !ok {
			writeError(w, http.StatusBadRequest, "La fecha de renovación no es válida.")
			return
		}
	}
	if in.Stop && cur.Active && reason == "" {
		writeError(w, http.StatusBadRequest, "Escribe por qué se suspende el medicamento.")
		return
	}
	row := s.db.QueryRow(r.Context(), `UPDATE chronic_medications SET name=$4, dose=$5, frequency=$6, indication=$7, started_on=$8::date, next_renewal=$9::date,
			active = CASE WHEN $10 THEN false ELSE active END, stopped_at = CASE WHEN $10 AND active THEN now() ELSE stopped_at END, stopped_reason=$11, updated_at=now()
		WHERE clinic_id=$1 AND patient_id=$2 AND id=$3 RETURNING `+chronicCols, p.ClinicID, id, mid, name, dose, freq, ind, started, renewal, in.Stop, reason)
	m, err := scanChronic(row)
	if err != nil {
		serverError(w, r, err)
		return
	}
	text := "Actualizó la medicación crónica: " + name
	if in.Stop {
		text = "Suspendió la medicación crónica: " + name
	}
	audit(r.Context(), s.db, p.ClinicID, p, "chronic_med_updated", text, map[string]any{"patient": id})
	writeJSON(w, http.StatusOK, map[string]any{"medication": m})
}
