package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// mountRx Medication / diagnosis catalogs and receta extras.
func (s *Server) mountRx(r chi.Router) {
	read := require(PermNavHistorials, PermAdminHistorials)
	write := require(PermAdminHistorials)
	r.With(read).Get("/rx/medications", s.searchMedications)
	r.With(read).Get("/rx/diagnoses", s.searchDiagnoses)
	r.With(read).Get("/rx/clinic-medications", s.listClinicMeds)
	r.With(write).Post("/rx/clinic-medications", s.saveClinicMed(false))
	r.With(write).Put("/rx/clinic-medications/{id}", s.saveClinicMed(true))
	r.With(write).Delete("/rx/clinic-medications/{id}", s.deleteClinicMed)
}

func queryLimit(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if n <= 0 {
		return 30
	}
	return min(n, 100)
}

const clinicMedCols = `id::text, name, brand, subject, species, category, control, route, presentations, typical_dose,
	mg_per_kg::float8, max_mg_per_kg_day::float8, concentrations, notes`

func scanClinicMed(row pgx.Row) (catMed, error) {
	var m catMed
	var mg, mx *float64
	var raw []byte
	err := row.Scan(&m.ID, &m.Name, &m.Brand, &m.Subject, &m.Species, &m.Category, &m.Control, &m.Route, &m.Presentations, &m.TypicalDose, &mg, &mx, &raw, &m.Notes)
	if err != nil {
		return m, err
	}
	if mg != nil {
		m.MgPerKg = *mg
	}
	if mx != nil {
		m.MaxMgPerKgDay = *mx
	}
	_ = json.Unmarshal(raw, &m.Concentrations)
	if m.Presentations == nil {
		m.Presentations = []string{}
	}
	m.Source = "clinic"
	return m, nil
}

func (s *Server) clinicMeds(ctx context.Context, clinicID string) ([]catMed, error) {
	rows, err := s.db.Query(ctx, `SELECT `+clinicMedCols+` FROM clinic_medications WHERE clinic_id = $1 AND active ORDER BY name`, clinicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []catMed{}
	for rows.Next() {
		m, err := scanClinicMed(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// lookupMed finds a medicine by catalog id or by the id of one of the clinic's own.
func (s *Server) lookupMed(ctx context.Context, clinicID, id string) (catMed, bool) {
	if m, ok := catalogByID(id); ok {
		return m, true
	}
	if !validUUID(id) {
		return catMed{}, false
	}
	m, err := scanClinicMed(s.db.QueryRow(ctx, `SELECT `+clinicMedCols+` FROM clinic_medications WHERE clinic_id = $1 AND id = $2 AND active`, clinicID, id))
	return m, err == nil
}

func (s *Server) searchMedications(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	q := normText(r.URL.Query().Get("q"))
	subject := r.URL.Query().Get("subject")
	if subject == "animal" || subject == "person" {
		// ok
	} else {
		subject = ""
	}
	species := normText(r.URL.Query().Get("species"))
	own, err := s.clinicMeds(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	type hit struct {
		m     catMed
		score int
	}
	var hits []hit
	for _, m := range append(own, catalogMeds...) {
		if subject != "" && m.Subject != subject {
			continue
		}
		if species != "" && m.Subject == "animal" && len(m.Species) > 0 &&
			!slices.ContainsFunc(m.Species, func(sp string) bool { return normText(sp) == species }) {
			continue
		}
		name := normText(m.Name)
		hay := name + " " + normText(m.Brand) + " " + normText(m.Category)
		sc := matchScore(name, hay, q)
		if sc == 0 {
			continue
		}
		if m.Source == "clinic" {
			sc += 5
		}
		hits = append(hits, hit{m, sc})
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	limit := queryLimit(r)
	out := make([]catMed, 0, min(limit, len(hits)))
	for _, h := range hits {
		if len(out) == limit {
			break
		}
		out = append(out, h.m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"medications": out, "total": len(hits), "disclaimer": catalogDisclaimer})
}

func (s *Server) searchDiagnoses(w http.ResponseWriter, r *http.Request) {
	q := normText(r.URL.Query().Get("q"))
	limit := queryLimit(r)
	type hit struct {
		e     icdEntry
		score int
	}
	var hits []hit
	for _, e := range icd10 {
		code := normText(e.Code)
		sc := matchScore(normText(e.Name), code+" "+normText(e.Name), q)
		if q != "" && strings.HasPrefix(code, q) {
			sc = max(sc, 90)
		}
		if sc > 0 {
			hits = append(hits, hit{e, sc})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := []icdEntry{}
	for _, h := range hits {
		if len(out) == limit {
			break
		}
		out = append(out, h.e)
	}
	writeJSON(w, http.StatusOK, map[string]any{"diagnoses": out, "total": len(hits), "disclaimer": icdDisclaimer})
}

func (s *Server) listClinicMeds(w http.ResponseWriter, r *http.Request) {
	list, err := s.clinicMeds(r.Context(), principalFrom(r.Context()).ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"medications": list})
}

type clinicMedIn struct {
	Name           string    `json:"name"`
	Brand          string    `json:"brand"`
	Subject        string    `json:"subject"`
	Species        []string  `json:"species"`
	Category       string    `json:"category"`
	Control        string    `json:"control"`
	Route          string    `json:"route"`
	Presentations  []string  `json:"presentations"`
	TypicalDose    string    `json:"typical_dose"`
	MgPerKg        *float64  `json:"mg_per_kg"`
	MaxMgPerKgDay  *float64  `json:"max_mg_per_kg_day"`
	Concentrations []catConc `json:"concentrations"`
	Notes          string    `json:"notes"`
}

func cleanList(in []string, max, maxLen int) []string {
	out := []string{}
	for _, x := range in {
		if x = strings.TrimSpace(x); x != "" && len(out) < max && utf8.RuneCountInString(x) <= maxLen {
			out = append(out, x)
		}
	}
	return out
}

func (s *Server) saveClinicMed(update bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if update && !validUUID(id) {
			writeError(w, http.StatusNotFound, "Medicamento no encontrado.")
			return
		}
		var in clinicMedIn
		if !decode(w, r, &in) {
			return
		}
		for _, f := range []*string{&in.Name, &in.Brand, &in.Category, &in.TypicalDose, &in.Notes} {
			*f = strings.TrimSpace(*f)
		}
		if in.Subject == "" {
			in.Subject = "person"
		}
		if in.Control == "" {
			in.Control = "No"
		}
		if in.Route == "" {
			in.Route = "Oral"
		}
		bad := func(msg string) { writeError(w, http.StatusBadRequest, msg) }
		switch {
		case in.Name == "" || utf8.RuneCountInString(in.Name) > 120 || utf8.RuneCountInString(in.Brand) > 120 || utf8.RuneCountInString(in.Category) > 60:
			bad("Escribe la denominación genérica (máximo 120 caracteres).")
			return
		case in.Subject != "person" && in.Subject != "animal":
			bad("El tipo de paciente no es válido.")
			return
		case !slices.Contains([]string{"No", "Antibiótico", "Fracción III"}, in.Control):
			bad("Categoría de control no válida.")
			return
		case !slices.Contains(rxRoutes, in.Route):
			bad("Vía de administración no válida.")
			return
		case utf8.RuneCountInString(in.TypicalDose) > 200 || utf8.RuneCountInString(in.Notes) > 500:
			bad("Uno de los textos es demasiado largo.")
			return
		case (in.MgPerKg != nil && (*in.MgPerKg < 0 || *in.MgPerKg > 10000)) || (in.MaxMgPerKgDay != nil && (*in.MaxMgPerKgDay < 0 || *in.MaxMgPerKgDay > 10000)):
			bad("Las dosis por kilo no son válidas.")
			return
		case len(in.Concentrations) > 10:
			bad("Máximo 10 concentraciones.")
			return
		}
		for _, c := range in.Concentrations {
			if strings.TrimSpace(c.Label) == "" || c.MgPerMl <= 0 || c.MgPerMl > 100000 || utf8.RuneCountInString(c.Label) > 80 {
				bad("Cada concentración necesita nombre y mg/mL mayores a cero.")
				return
			}
		}
		if in.Concentrations == nil {
			in.Concentrations = []catConc{}
		}
		species := cleanList(in.Species, 10, 40)
		if in.Subject == "person" {
			species = []string{}
		}
		conc, _ := json.Marshal(in.Concentrations)
		p := principalFrom(r.Context())
		var row pgx.Row
		if update {
			row = s.db.QueryRow(r.Context(), `
				UPDATE clinic_medications SET name=$3, brand=$4, subject=$5, species=$6, category=$7, control=$8, route=$9, presentations=$10,
					typical_dose=$11, mg_per_kg=$12, max_mg_per_kg_day=$13, concentrations=$14, notes=$15, updated_at=now()
				WHERE clinic_id=$1 AND id=$2 AND active RETURNING `+clinicMedCols,
				p.ClinicID, id, in.Name, in.Brand, in.Subject, species, in.Category, in.Control, in.Route, cleanList(in.Presentations, 20, 100),
				in.TypicalDose, in.MgPerKg, in.MaxMgPerKgDay, conc, in.Notes)
		} else {
			row = s.db.QueryRow(r.Context(), `
				INSERT INTO clinic_medications (clinic_id, name, brand, subject, species, category, control, route, presentations, typical_dose, mg_per_kg, max_mg_per_kg_day, concentrations, notes)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING `+clinicMedCols,
				p.ClinicID, in.Name, in.Brand, in.Subject, species, in.Category, in.Control, in.Route, cleanList(in.Presentations, 20, 100),
				in.TypicalDose, in.MgPerKg, in.MaxMgPerKgDay, conc, in.Notes)
		}
		m, err := scanClinicMed(row)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "Medicamento no encontrado.")
			return
		}
		if err != nil {
			serverError(w, r, err)
			return
		}
		audit(r.Context(), s.db, p.ClinicID, p, "clinic_medication", map[bool]string{false: "Agregó", true: "Editó"}[update]+" el medicamento propio "+m.Name, nil)
		status := http.StatusCreated
		if update {
			status = http.StatusOK
		}
		writeJSON(w, status, map[string]any{"medication": m})
	}
}

func (s *Server) deleteClinicMed(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p := principalFrom(r.Context())
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Medicamento no encontrado.")
		return
	}
	var name string
	err := s.db.QueryRow(r.Context(), `DELETE FROM clinic_medications WHERE clinic_id = $1 AND id = $2 RETURNING name`, p.ClinicID, id).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Medicamento no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "clinic_medication", "Eliminó el medicamento propio "+name, nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
