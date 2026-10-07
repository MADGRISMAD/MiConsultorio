package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Vaccination and deworming card for people and animals. Append-only: a wrong entry is voided with a reason.

var vaccinationKinds = []string{"vaccine", "deworming_internal", "deworming_external", "other"}

type vaccination struct {
	ID                 string     `json:"id"`
	PatientID          string     `json:"patient_id"`
	Kind               string     `json:"kind"`
	Name               string     `json:"name"`
	AppliedOn          string     `json:"applied_on"`
	NextDue            *string    `json:"next_due"`
	Lot                string     `json:"lot"`
	Dose               string     `json:"dose"`
	AdministeredByName string     `json:"administered_by_name"`
	Notes              string     `json:"notes"`
	CatalogItemID      *string    `json:"catalog_item_id"`
	VoidedAt           *time.Time `json:"voided_at"`
	VoidedByName       string     `json:"voided_by_name"`
	VoidReason         string     `json:"void_reason"`
	CreatedByName      string     `json:"created_by_name"`
	CreatedAt          time.Time  `json:"created_at"`
}

const vaccinationCols = `id, patient_id::text, kind, name, to_char(applied_on,'YYYY-MM-DD'), to_char(next_due,'YYYY-MM-DD'), lot, dose,
	administered_by_name, notes, catalog_item_id::text, voided_at, voided_by_name, void_reason, created_by_name, created_at`

func scanVaccination(row pgx.Row) (vaccination, error) {
	var v vaccination
	err := row.Scan(&v.ID, &v.PatientID, &v.Kind, &v.Name, &v.AppliedOn, &v.NextDue, &v.Lot, &v.Dose, &v.AdministeredByName, &v.Notes,
		&v.CatalogItemID, &v.VoidedAt, &v.VoidedByName, &v.VoidReason, &v.CreatedByName, &v.CreatedAt)
	return v, err
}

// vaccineSuggestion is a catalog entry the form offers; IntervalDays drives the suggested booster date.
type vaccineSuggestion struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	IntervalDays int    `json:"interval_days"` // 0 = no booster
	Note         string `json:"note,omitempty"`
}

func vs(kind, name string, days int, note string) vaccineSuggestion {
	return vaccineSuggestion{Kind: kind, Name: name, IntervalDays: days, Note: note}
}

var vaccineCatalog = map[string][]vaccineSuggestion{
	"Perro": {
		vs("vaccine", "Puppy (DA2PP/DHPP) - cachorro", 21, "Cada 3 a 4 semanas hasta las 16 semanas"),
		vs("vaccine", "Múltiple (DA2PP/DHPPi) - refuerzo anual", 365, ""),
		vs("vaccine", "Leptospirosis", 365, ""),
		vs("vaccine", "Rabia", 365, "Obligatoria; refuerzo anual"),
		vs("vaccine", "Bordetella (tos de las perreras)", 365, ""),
		vs("vaccine", "Giardia", 365, ""),
		vs("deworming_internal", "Desparasitante interno", 90, "Cada 3 meses (cada mes en cachorros)"),
		vs("deworming_external", "Antipulgas y garrapatas", 30, "Mensual según producto"),
	},
	"Gato": {
		vs("vaccine", "Triple felina (FVRCP) - gatito", 21, "Cada 3 a 4 semanas hasta las 16 semanas"),
		vs("vaccine", "Triple felina (FVRCP) - refuerzo anual", 365, ""),
		vs("vaccine", "Leucemia felina (FeLV)", 365, ""),
		vs("vaccine", "Rabia", 365, "Obligatoria; refuerzo anual"),
		vs("deworming_internal", "Desparasitante interno", 90, "Cada 3 meses (cada mes en gatitos)"),
		vs("deworming_external", "Antipulgas", 30, "Mensual según producto"),
	},
	"Equino": {
		vs("vaccine", "Tétanos", 365, ""),
		vs("vaccine", "Influenza equina", 180, ""),
		vs("vaccine", "Encefalomielitis equina", 365, ""),
		vs("vaccine", "Rabia", 365, ""),
		vs("vaccine", "Rinoneumonitis (herpesvirus equino)", 180, ""),
		vs("deworming_internal", "Desparasitante interno", 90, "Según conteo de huevecillos"),
	},
	"Bovino": {
		vs("vaccine", "Clostridiasis (7 vías)", 365, ""),
		vs("vaccine", "Derriengue (rabia paralítica)", 365, "Zonas endémicas"),
		vs("vaccine", "Brucelosis (cepa 19 / RB51)", 0, "Terneras 3 a 8 meses; campaña oficial"),
		vs("vaccine", "IBR / DVB / PI3", 365, ""),
		vs("deworming_internal", "Desparasitante interno", 180, ""),
		vs("deworming_external", "Garrapaticida", 21, "Según infestación"),
	},
	"Porcino": {
		vs("vaccine", "Fiebre porcina clásica", 180, "Campaña oficial"),
		vs("vaccine", "Parvovirus / Erisipela", 180, ""),
		vs("deworming_internal", "Desparasitante interno", 120, ""),
	},
	"Ovino / caprino": {
		vs("vaccine", "Clostridiasis", 365, ""),
		vs("vaccine", "Rabia", 365, "Zonas endémicas"),
		vs("deworming_internal", "Desparasitante interno", 90, ""),
	},
	"Conejo": {
		vs("vaccine", "Enfermedad hemorrágica viral", 365, ""),
		vs("vaccine", "Mixomatosis", 365, ""),
		vs("deworming_internal", "Desparasitante interno", 90, ""),
	},
	// Esquema básico nacional de vacunación (México): editable suggestions, the doctor decides.
	"person": {
		vs("vaccine", "BCG (tuberculosis)", 0, "Dosis única al nacer"),
		vs("vaccine", "Hepatitis B", 30, "Al nacer, 2 y 6 meses (3 dosis)"),
		vs("vaccine", "Hexavalente acelular (DPaT+VIP+Hib)", 60, "2, 4, 6 y 18 meses"),
		vs("vaccine", "Rotavirus", 60, "2 y 4 meses"),
		vs("vaccine", "Neumocócica conjugada", 60, "2, 4 y 12 meses"),
		vs("vaccine", "Influenza", 365, "Anual (6 meses en adelante)"),
		vs("vaccine", "SRP (sarampión, rubéola, parotiditis)", 0, "12 meses y 6 años"),
		vs("vaccine", "DPT (refuerzo)", 0, "4 años"),
		vs("vaccine", "VPH (virus del papiloma humano)", 180, "Niñas y niños de 5.º de primaria: 2 dosis"),
		vs("vaccine", "Td (tétanos y difteria)", 3650, "Refuerzo cada 10 años"),
		vs("vaccine", "SR (sarampión y rubéola)", 0, "Adolescentes y adultos sin esquema"),
		vs("vaccine", "COVID-19", 365, "Según lineamientos vigentes"),
		vs("vaccine", "Neumococo 23 valente", 0, "Adultos mayores de 60 años o con riesgo"),
		vs("other", "Otra", 0, ""),
	},
}

func (s *Server) vaccineSuggestionsFor(subject, species string) []vaccineSuggestion {
	var list []vaccineSuggestion
	if subject == "animal" {
		list = vaccineCatalog[species]
		if list == nil {
			list = []vaccineSuggestion{vs("deworming_internal", "Desparasitante interno", 90, ""), vs("deworming_external", "Antiparasitario externo", 30, ""), vs("vaccine", "Rabia", 365, "")}
		}
	} else {
		list = vaccineCatalog["person"]
	}
	return slices.Clone(list)
}

// patientForSpecialty loads the subject and species of a patient of this clinic, answering 404 itself.
func (s *Server) patientForSpecialty(w http.ResponseWriter, r *http.Request) (id, subject, species string, ok bool) {
	id = chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	var raw []byte
	err := s.db.QueryRow(r.Context(), `SELECT subject, profile FROM patients WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id).Scan(&subject, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	var prof map[string]any
	_ = json.Unmarshal(raw, &prof)
	species, _ = prof["species"].(string)
	return id, subject, species, true
}

func (s *Server) listVaccinations(w http.ResponseWriter, r *http.Request) {
	id, subject, species, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `SELECT `+vaccinationCols+` FROM vaccinations WHERE clinic_id=$1 AND patient_id=$2 ORDER BY applied_on DESC, created_at DESC LIMIT 1000`, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []vaccination{}
	for rows.Next() {
		v, err := scanVaccination(rows)
		if err != nil {
			serverError(w, r, err)
			return
		}
		out = append(out, v)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	s.logAccess(r.Context(), p.ClinicID, id, p, "view")
	writeJSON(w, http.StatusOK, map[string]any{"vaccinations": out, "suggestions": s.vaccineSuggestionsFor(subject, species), "subject": subject, "species": species})
}

type vaccinationIn struct {
	Kind               string `json:"kind"`
	Name               string `json:"name"`
	AppliedOn          string `json:"applied_on"`
	NextDue            string `json:"next_due"`
	Lot                string `json:"lot"`
	Dose               string `json:"dose"`
	AdministeredByName string `json:"administered_by_name"`
	Notes              string `json:"notes"`
	CatalogItemID      string `json:"catalog_item_id"`
}

func parseDate(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", s)
	return t, err == nil
}

func (s *Server) createVaccination(w http.ResponseWriter, r *http.Request) {
	id, _, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	var in vaccinationIn
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	in.Name, in.Lot, in.Dose, in.Notes = strings.TrimSpace(in.Name), strings.TrimSpace(in.Lot), strings.TrimSpace(in.Dose), strings.TrimSpace(in.Notes)
	in.AdministeredByName = strings.TrimSpace(in.AdministeredByName)
	if in.Kind == "" {
		in.Kind = "vaccine"
	}
	switch {
	case !slices.Contains(vaccinationKinds, in.Kind):
		writeError(w, http.StatusBadRequest, "Tipo de aplicación inválido.")
		return
	case in.Name == "" || utf8.RuneCountInString(in.Name) > 120:
		writeError(w, http.StatusBadRequest, "Escribe el nombre de la vacuna o desparasitante.")
		return
	case utf8.RuneCountInString(in.Lot) > 60 || utf8.RuneCountInString(in.Dose) > 60 || utf8.RuneCountInString(in.AdministeredByName) > 120 || utf8.RuneCountInString(in.Notes) > 500:
		writeError(w, http.StatusBadRequest, "Alguno de los textos es demasiado largo.")
		return
	}
	applied, okDate := parseDate(in.AppliedOn)
	if !okDate {
		writeError(w, http.StatusBadRequest, "La fecha de aplicación no es válida.")
		return
	}
	if applied.After(time.Now().AddDate(0, 0, 1)) {
		writeError(w, http.StatusBadRequest, "La fecha de aplicación no puede ser futura.")
		return
	}
	if applied.Before(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)) {
		writeError(w, http.StatusBadRequest, "La fecha de aplicación no es válida.")
		return
	}
	var next any
	if in.NextDue != "" {
		nd, okNext := parseDate(in.NextDue)
		if !okNext || !nd.After(applied) || nd.After(applied.AddDate(30, 0, 0)) {
			writeError(w, http.StatusBadRequest, "La fecha del próximo refuerzo debe ser posterior a la aplicación.")
			return
		}
		next = in.NextDue
	}
	var item any
	if in.CatalogItemID != "" {
		var found bool
		if !validUUID(in.CatalogItemID) {
			writeError(w, http.StatusBadRequest, "El producto del catálogo no es válido.")
			return
		}
		if err := s.db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM catalog_items WHERE clinic_id=$1 AND id=$2)`, p.ClinicID, in.CatalogItemID).Scan(&found); err != nil {
			serverError(w, r, err)
			return
		}
		if !found {
			writeError(w, http.StatusBadRequest, "El producto del catálogo no existe.")
			return
		}
		item = in.CatalogItemID
	}
	if in.AdministeredByName == "" {
		in.AdministeredByName = p.actorName()
	}
	row := s.db.QueryRow(r.Context(), `
		INSERT INTO vaccinations (clinic_id, patient_id, kind, name, applied_on, next_due, lot, dose, administered_by_name, notes, catalog_item_id, created_by_name)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING `+vaccinationCols,
		p.ClinicID, id, in.Kind, in.Name, in.AppliedOn, next, in.Lot, in.Dose, in.AdministeredByName, in.Notes, item, p.actorName())
	v, err := scanVaccination(row)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"vaccination": v})
}

func (s *Server) voidVaccination(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Registro no encontrado.")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" || utf8.RuneCountInString(req.Reason) > 300 {
		writeError(w, http.StatusBadRequest, "Escribe el motivo de la anulación.")
		return
	}
	p := principalFrom(r.Context())
	row := s.db.QueryRow(r.Context(), `UPDATE vaccinations SET voided_at=now(), voided_by_name=$3, void_reason=$4
		WHERE clinic_id=$1 AND id=$2 AND voided_at IS NULL RETURNING `+vaccinationCols, p.ClinicID, id, p.actorName(), req.Reason)
	v, err := scanVaccination(row)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Registro no encontrado o ya anulado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "vaccination_void", "Anuló una aplicación del carnet de vacunación", map[string]any{"patient": v.PatientID})
	writeJSON(w, http.StatusOK, map[string]any{"vaccination": v})
}

type dueVaccination struct {
	VaccinationID string  `json:"vaccination_id"`
	PatientID     string  `json:"patient_id"`
	PatientName   string  `json:"patient_name"`
	Subject       string  `json:"subject"`
	Kind          string  `json:"kind"`
	Name          string  `json:"name"`
	AppliedOn     string  `json:"applied_on"`
	NextDue       string  `json:"next_due"`
	DaysLeft      int     `json:"days_left"` // negative = overdue
	Overdue       bool    `json:"overdue"`
	ContactName   string  `json:"contact_name"`
	Phone         string  `json:"phone"`
	Email         string  `json:"email"`
	RemindersOK   bool    `json:"reminders_ok"`
	Species       *string `json:"species"`
}

// dueVaccinations lists, per patient and product, the latest application whose booster is overdue or falls
// within the next `days`. A newer application of the same product replaces the older one.
func (s *Server) dueVaccinations(w http.ResponseWriter, r *http.Request) {
	days := 30
	if q := r.URL.Query().Get("days"); q != "" {
		n, err := strconv.Atoi(q)
		if err != nil || n < 0 || n > 730 {
			writeError(w, http.StatusBadRequest, "El número de días no es válido.")
			return
		}
		days = n
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `
		SELECT * FROM (
		  SELECT DISTINCT ON (v.patient_id, v.kind, lower(v.name))
		         v.id::text, v.patient_id::text, trim(p.names || ' ' || p.last_names) AS pname, p.subject, v.kind, v.name,
		         to_char(v.applied_on,'YYYY-MM-DD'), to_char(v.next_due,'YYYY-MM-DD') AS next_due, (v.next_due - current_date) AS days_left,
		         CASE WHEN p.subject = 'animal' OR p.guardian_name <> '' THEN p.guardian_name ELSE trim(p.names || ' ' || p.last_names) END,
		         CASE WHEN p.guardian_phone <> '' THEN p.guardian_phone ELSE p.phone END,
		         CASE WHEN p.guardian_email <> '' THEN p.guardian_email ELSE p.email END,
		         p.reminders_ok, nullif(p.profile->>'species','')
		  FROM vaccinations v JOIN patients p ON p.id = v.patient_id
		  WHERE v.clinic_id = $1 AND v.voided_at IS NULL AND p.archived_at IS NULL
		  ORDER BY v.patient_id, v.kind, lower(v.name), v.applied_on DESC, v.created_at DESC
		) x WHERE x.next_due IS NOT NULL AND x.days_left <= $2::int ORDER BY x.next_due, x.pname LIMIT 500`, p.ClinicID, days)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []dueVaccination{}
	for rows.Next() {
		var d dueVaccination
		var next *string
		if err := rows.Scan(&d.VaccinationID, &d.PatientID, &d.PatientName, &d.Subject, &d.Kind, &d.Name, &d.AppliedOn, &next, &d.DaysLeft,
			&d.ContactName, &d.Phone, &d.Email, &d.RemindersOK, &d.Species); err != nil {
			serverError(w, r, err)
			return
		}
		if next != nil {
			d.NextDue = *next
		}
		d.Overdue = d.DaysLeft < 0
		out = append(out, d)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"due": out, "days": days})
}

type weightPoint struct {
	Date        string  `json:"date"`
	WeightKg    float64 `json:"weight_kg"`
	EncounterID string  `json:"encounter_id"`
}

// patientWeights derives the weight history from the weight_kg measure of each visit.
func (s *Server) patientWeights(w http.ResponseWriter, r *http.Request) {
	id, _, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `SELECT id::text, occurred_at, measures FROM encounters
		WHERE clinic_id=$1 AND patient_id=$2 AND measures ? 'weight_kg' AND (NOT private OR author_id = $3)
		ORDER BY occurred_at ASC LIMIT 2000`, p.ClinicID, id, p.UserID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []weightPoint{}
	for rows.Next() {
		var eid string
		var at time.Time
		var raw []byte
		if err := rows.Scan(&eid, &at, &raw); err != nil {
			serverError(w, r, err)
			return
		}
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		var kg float64
		switch v := m["weight_kg"].(type) {
		case float64:
			kg = v
		case string:
			kg, _ = strconv.ParseFloat(strings.Replace(strings.TrimSpace(v), ",", ".", 1), 64)
		}
		if kg > 0 {
			out = append(out, weightPoint{Date: at.Format("2006-01-02"), WeightKg: kg, EncounterID: eid})
		}
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"weights": out})
}
