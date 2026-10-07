package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// The bitácora of a patient: what was said and done in each visit. Entries are append-only
// (NOM-004-SSA3-2012: notes carry date, time and author and are not rewritten); a mistake is corrected
// with an adenda that points to the original.

var icdRe = regexp.MustCompile(`^[A-TV-Z][0-9]{2}(\.[0-9A-Z]{1,4})?$`)

type encounter struct {
	ID             string         `json:"id"`
	PatientID      string         `json:"patient_id"`
	Kind           string         `json:"kind"`
	OccurredAt     time.Time      `json:"occurred_at"`
	AppointmentID  *string        `json:"appointment_id"`
	Reason         string         `json:"reason"`
	Subjective     string         `json:"subjective"`
	Measures       map[string]any `json:"measures"`
	Exam           string         `json:"exam"`
	Assessment     string         `json:"assessment"`
	DiagnosisCodes []string       `json:"diagnosis_codes"`
	Plan           string         `json:"plan"`
	Notes          string         `json:"notes"`
	Private        bool           `json:"private"`
	Hidden         bool           `json:"hidden,omitempty"` // private note of someone else: content withheld
	AddendumOf     *string        `json:"addendum_of"`
	AuthorName     string         `json:"author_name"`
	AuthorRole     string         `json:"author_role"`
	AuthorLicense  string         `json:"author_license"`
	CreatedAt      time.Time      `json:"created_at"`
}

const encounterCols = `id, patient_id, kind, occurred_at, appointment_id::text, reason, subjective, measures, exam, assessment, diagnosis_codes,
	plan, notes, private, addendum_of::text, coalesce(author_id::text, ''), author_name, author_role, author_license, created_at`

func scanEncounter(row pgx.Row, me string) (encounter, error) {
	var e encounter
	var raw []byte
	var authorID string
	err := row.Scan(&e.ID, &e.PatientID, &e.Kind, &e.OccurredAt, &e.AppointmentID, &e.Reason, &e.Subjective, &raw, &e.Exam, &e.Assessment, &e.DiagnosisCodes,
		&e.Plan, &e.Notes, &e.Private, &e.AddendumOf, &authorID, &e.AuthorName, &e.AuthorRole, &e.AuthorLicense, &e.CreatedAt)
	if err != nil {
		return e, err
	}
	e.Measures = map[string]any{}
	_ = json.Unmarshal(raw, &e.Measures)
	if e.DiagnosisCodes == nil {
		e.DiagnosisCodes = []string{}
	}
	if e.Private && authorID != me {
		// Someone else's private note (e.g. psychotherapy): only the fact that it exists is shown.
		e = encounter{ID: e.ID, PatientID: e.PatientID, Kind: e.Kind, OccurredAt: e.OccurredAt, Private: true, Hidden: true,
			AuthorName: e.AuthorName, AuthorRole: e.AuthorRole, CreatedAt: e.CreatedAt, Measures: map[string]any{}, DiagnosisCodes: []string{}}
	}
	return e, nil
}

func (s *Server) patientExists(ctx context.Context, clinicID, id string) (string, error) {
	var subject string
	err := s.db.QueryRow(ctx, `SELECT subject FROM patients WHERE clinic_id=$1 AND id=$2`, clinicID, id).Scan(&subject)
	return subject, err
}

func (s *Server) listEncounters(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	if _, err := s.patientExists(r.Context(), p.ClinicID, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		} else {
			serverError(w, r, err)
		}
		return
	}
	list, err := s.visibleEncounters(r.Context(), p, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"encounters": list})
}

func (s *Server) visibleEncounters(ctx context.Context, p *Principal, patientID string) ([]encounter, error) {
	rows, err := s.db.Query(ctx, `SELECT `+encounterCols+` FROM encounters WHERE clinic_id=$1 AND patient_id=$2 ORDER BY occurred_at DESC, created_at DESC LIMIT 1000`, p.ClinicID, patientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []encounter{}
	for rows.Next() {
		e, err := scanEncounter(rows, p.UserID)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type encounterIn struct {
	Kind           string         `json:"kind"`
	OccurredAt     *time.Time     `json:"occurred_at"`
	AppointmentID  string         `json:"appointment_id"`
	Reason         string         `json:"reason"`
	Subjective     string         `json:"subjective"`
	Measures       map[string]any `json:"measures"`
	Exam           string         `json:"exam"`
	Assessment     string         `json:"assessment"`
	DiagnosisCodes []string       `json:"diagnosis_codes"`
	Plan           string         `json:"plan"`
	Notes          string         `json:"notes"`
	Private        bool           `json:"private"`
}

func (s *Server) createEncounter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	var in encounterIn
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	subject, err := s.patientExists(r.Context(), p.ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	kinds, err := s.clinicKindsFor(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if in.Kind == "" {
		in.Kind = "consulta"
	}
	if !slices.Contains([]string{"consulta", "seguimiento", "procedimiento", "llamada", "nota"}, in.Kind) {
		writeError(w, http.StatusBadRequest, "Tipo de registro inválido.")
		return
	}
	for _, f := range []*string{&in.Reason, &in.Subjective, &in.Exam, &in.Assessment, &in.Plan, &in.Notes} {
		*f = strings.TrimSpace(*f)
		if utf8.RuneCountInString(*f) > maxLong {
			writeError(w, http.StatusBadRequest, "Uno de los textos es demasiado largo.")
			return
		}
	}
	measures, msg := cleanValues(measureFields(subject, kinds), in.Measures, false)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	codes := make([]string, 0, len(in.DiagnosisCodes))
	for _, c := range in.DiagnosisCodes {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if !icdRe.MatchString(c) {
			writeError(w, http.StatusBadRequest, "«"+c+"» no parece un código CIE-10 (por ejemplo J06.9).")
			return
		}
		if !slices.Contains(codes, c) {
			codes = append(codes, c)
		}
	}
	if len(codes) > 10 {
		writeError(w, http.StatusBadRequest, "Máximo 10 códigos de diagnóstico.")
		return
	}
	if in.Reason == "" && in.Subjective == "" && in.Exam == "" && in.Assessment == "" && in.Plan == "" && in.Notes == "" && len(measures) == 0 {
		writeError(w, http.StatusBadRequest, "Escribe al menos el motivo, lo que cuenta el paciente, la exploración, el diagnóstico o el plan.")
		return
	}
	occurred := time.Now()
	if in.OccurredAt != nil {
		if in.OccurredAt.After(time.Now().Add(5*time.Minute)) || in.OccurredAt.Before(time.Now().Add(-7*24*time.Hour)) {
			writeError(w, http.StatusBadRequest, "La fecha de la consulta debe estar dentro de los últimos 7 días. Para registros más antiguos usa una nota con la fecha en el texto.")
			return
		}
		occurred = *in.OccurredAt
	}
	var appt any
	if in.AppointmentID != "" {
		if !validUUID(in.AppointmentID) {
			writeError(w, http.StatusBadRequest, "La cita no es válida.")
			return
		}
		appt = in.AppointmentID
	}
	var license string
	_ = s.db.QueryRow(r.Context(), `SELECT cedula FROM users WHERE id = $1`, p.UserID).Scan(&license)

	var out encounter
	err = inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		row := tx.QueryRow(r.Context(), `
			INSERT INTO encounters (clinic_id, patient_id, kind, occurred_at, appointment_id, reason, subjective, measures, exam, assessment,
				diagnosis_codes, plan, notes, private, author_id, author_name, author_role, author_license)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) RETURNING `+encounterCols,
			p.ClinicID, id, in.Kind, occurred, appt, in.Reason, in.Subjective, jsonOrEmpty(measures), in.Exam, in.Assessment, codes, in.Plan, in.Notes,
			in.Private, p.UserID, p.actorName(), roleLabels[p.Role], license)
		var err error
		if out, err = scanEncounter(row, p.UserID); err != nil {
			return err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE patients SET last_encounter_at = $3, updated_at = now() WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id, occurred); err != nil {
			return err
		}
		if appt == nil {
			return nil
		}
		return s.linkEncounterToAppointment(r.Context(), tx, p, id, in.AppointmentID, out.ID)
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"encounter": out})
}

// addendum corrects or completes an earlier entry without rewriting it.
func (s *Server) createAddendum(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Registro no encontrado.")
		return
	}
	var req struct {
		Reason string `json:"reason"` // what is being corrected
		Text   string `json:"text"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Reason, req.Text = strings.TrimSpace(req.Reason), strings.TrimSpace(req.Text)
	if req.Reason == "" || req.Text == "" || utf8.RuneCountInString(req.Text) > maxLong || utf8.RuneCountInString(req.Reason) > 200 {
		writeError(w, http.StatusBadRequest, "Indica qué se corrige y escribe el texto de la adenda.")
		return
	}
	p := principalFrom(r.Context())
	var patientID, authorID string
	var private bool
	err := s.db.QueryRow(r.Context(), `SELECT patient_id::text, private, coalesce(author_id::text,'') FROM encounters WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id).Scan(&patientID, &private, &authorID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Registro no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if private && authorID != p.UserID {
		writeError(w, http.StatusForbidden, "Es una nota privada de otra persona.")
		return
	}
	var license string
	_ = s.db.QueryRow(r.Context(), `SELECT cedula FROM users WHERE id = $1`, p.UserID).Scan(&license)
	row := s.db.QueryRow(r.Context(), `
		INSERT INTO encounters (clinic_id, patient_id, kind, reason, notes, private, addendum_of, author_id, author_name, author_role, author_license)
		VALUES ($1,$2,'adenda',$3,$4,$5,$6,$7,$8,$9,$10) RETURNING `+encounterCols,
		p.ClinicID, patientID, req.Reason, req.Text, private, id, p.UserID, p.actorName(), roleLabels[p.Role], license)
	out, err := scanEncounter(row, p.UserID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "encounter_addendum", "Agregó una adenda a la bitácora de un paciente", map[string]any{"patient": patientID})
	writeJSON(w, http.StatusCreated, map[string]any{"encounter": out})
}

// jsonOrEmpty is the JSON of a map, or {} when nil.
func jsonOrEmpty(m map[string]any) []byte { return jsonOrEmptyMap(m) }

func jsonOrEmptyMap(m map[string]any) []byte {
	if m == nil {
		m = map[string]any{}
	}
	b, _ := json.Marshal(m)
	return b
}
