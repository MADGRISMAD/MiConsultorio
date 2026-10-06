package api

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

var curpRe = regexp.MustCompile(`^[A-Z0-9]{18}$`)

// expedientFields is the editable part of a patient record; it is both the
// request body and (embedded in expedient) the response body.
type expedientFields struct {
	CURP             string `json:"CURP" db:"curp"`
	Names            string `json:"names" db:"names"`
	LastNames        string `json:"last_names" db:"last_names"`
	Sex              string `json:"sex" db:"sex"`
	DateOfBirth      string `json:"date_of_birth" db:"date_of_birth"`
	Education        string `json:"education" db:"education"`
	Occupation       string `json:"occupation" db:"occupation"`
	Weight           string `json:"weight" db:"weight"`
	ClothesSize      string `json:"clothes_size" db:"clothes_size"`
	Height           string `json:"height" db:"height"`
	Ethnicity        string `json:"ethnicity" db:"ethnicity"`
	PhysicalActivity string `json:"physical_activity" db:"physical_activity"`
	Hobbies          string `json:"hobbies" db:"hobbies"`
	Child            string `json:"child" db:"child"`

	Diabetes          bool `json:"diabetes" db:"diabetes"`
	RheumaticDiseases bool `json:"rheumatic_diseases" db:"rheumatic_diseases"`
	Fractures         bool `json:"fractures" db:"fractures"`
	Allergies         bool `json:"allergies" db:"allergies"`
	Layed             bool `json:"layed" db:"layed"`
	Contractures      bool `json:"contractures" db:"contractures"`
	Cancer            bool `json:"cancer" db:"cancer"`
	Accidents         bool `json:"accidents" db:"accidents"`
	Transfusions      bool `json:"transfusions" db:"transfusions"`
	Cardiopathies     bool `json:"cardiopathies" db:"cardiopathies"`
	Surgeries         bool `json:"surgeries" db:"surgeries"`
	Tabaquism         bool `json:"tabaquism" db:"tabaquism"`
	Alcoholism        bool `json:"alcoholism" db:"alcoholism"`
	Automedication    bool `json:"automedication" db:"automedication"`
	DrugUse           bool `json:"drug_use" db:"drug_use"`
	Pregnant          bool `json:"pregnant" db:"pregnant"`
}

type expedient struct {
	ID  string `json:"id" db:"id"`
	Age int    `json:"age" db:"age"`
	expedientFields
}

const expedientCols = `id, date_part('year', age(date_of_birth))::int AS age, curp, names, last_names, sex,
	to_char(date_of_birth, 'YYYY-MM-DD') AS date_of_birth, education, occupation, weight, clothes_size, height,
	ethnicity, physical_activity, hobbies, child, diabetes, rheumatic_diseases, fractures, allergies, layed,
	contractures, cancer, accidents, transfusions, cardiopathies, surgeries, tabaquism, alcoholism,
	automedication, drug_use, pregnant`

func normalizeCURP(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// validate trims and checks the record, returning a user-facing message on failure.
func (e *expedientFields) validate() string {
	e.CURP = normalizeCURP(e.CURP)
	for _, f := range []*string{&e.Names, &e.LastNames, &e.Education, &e.Occupation, &e.Weight, &e.ClothesSize,
		&e.Height, &e.Ethnicity, &e.PhysicalActivity, &e.Hobbies, &e.Child, &e.DateOfBirth} {
		*f = strings.TrimSpace(*f)
		if utf8.RuneCountInString(*f) > 200 {
			return "Uno de los campos es demasiado largo."
		}
	}
	switch {
	case e.Names == "" || e.LastNames == "" || e.CURP == "" || e.DateOfBirth == "" || e.Sex == "":
		return "Faltan campos por llenar."
	case !curpRe.MatchString(e.CURP):
		return "La CURP debe tener 18 caracteres alfanuméricos."
	case e.Sex != "Hombre" && e.Sex != "Mujer":
		return "El sexo debe ser Hombre o Mujer."
	}
	dob, err := time.Parse("2006-01-02", e.DateOfBirth)
	if err != nil {
		return "La fecha de nacimiento no es válida."
	}
	if dob.After(time.Now()) || dob.Year() < 1900 {
		return "La fecha de nacimiento está fuera de rango."
	}
	return ""
}

func (s *Server) listExpedients(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(),
		`SELECT `+expedientCols+` FROM expedients WHERE clinic_id = $1 ORDER BY last_names, names`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[expedient])
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"expedients": list})
}

func (s *Server) getExpedient(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(),
		`SELECT `+expedientCols+` FROM expedients WHERE clinic_id = $1 AND curp = $2`, p.ClinicID, normalizeCURP(chi.URLParam(r, "curp")))
	if err != nil {
		serverError(w, r, err)
		return
	}
	e, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[expedient])
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "No hay un expediente con esa CURP.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"expedient": e})
}

func (s *Server) createExpedient(w http.ResponseWriter, r *http.Request) {
	var f expedientFields
	if !decode(w, r, &f) {
		return
	}
	if msg := f.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `
		INSERT INTO expedients (clinic_id, curp, names, last_names, sex, date_of_birth, education, occupation, weight,
			clothes_size, height, ethnicity, physical_activity, hobbies, child, diabetes, rheumatic_diseases, fractures,
			allergies, layed, contractures, cancer, accidents, transfusions, cardiopathies, surgeries, tabaquism,
			alcoholism, automedication, drug_use, pregnant)
		VALUES ($1,$2,$3,$4,$5,$6::date,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31)
		RETURNING `+expedientCols,
		p.ClinicID, f.CURP, f.Names, f.LastNames, f.Sex, f.DateOfBirth, f.Education, f.Occupation, f.Weight,
		f.ClothesSize, f.Height, f.Ethnicity, f.PhysicalActivity, f.Hobbies, f.Child, f.Diabetes, f.RheumaticDiseases,
		f.Fractures, f.Allergies, f.Layed, f.Contractures, f.Cancer, f.Accidents, f.Transfusions, f.Cardiopathies,
		f.Surgeries, f.Tabaquism, f.Alcoholism, f.Automedication, f.DrugUse, f.Pregnant)
	if err != nil {
		serverError(w, r, err)
		return
	}
	e, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[expedient])
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "Ya existe un expediente con esa CURP.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"expedient": e})
}

func (s *Server) updateExpedient(w http.ResponseWriter, r *http.Request) {
	var f expedientFields
	if !decode(w, r, &f) {
		return
	}
	if msg := f.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `
		UPDATE expedients SET curp=$3, names=$4, last_names=$5, sex=$6, date_of_birth=$7::date, education=$8,
			occupation=$9, weight=$10, clothes_size=$11, height=$12, ethnicity=$13, physical_activity=$14, hobbies=$15,
			child=$16, diabetes=$17, rheumatic_diseases=$18, fractures=$19, allergies=$20, layed=$21, contractures=$22,
			cancer=$23, accidents=$24, transfusions=$25, cardiopathies=$26, surgeries=$27, tabaquism=$28,
			alcoholism=$29, automedication=$30, drug_use=$31, pregnant=$32, updated_at=now()
		WHERE clinic_id=$1 AND curp=$2
		RETURNING `+expedientCols,
		p.ClinicID, normalizeCURP(chi.URLParam(r, "curp")), f.CURP, f.Names, f.LastNames, f.Sex, f.DateOfBirth,
		f.Education, f.Occupation, f.Weight, f.ClothesSize, f.Height, f.Ethnicity, f.PhysicalActivity, f.Hobbies,
		f.Child, f.Diabetes, f.RheumaticDiseases, f.Fractures, f.Allergies, f.Layed, f.Contractures, f.Cancer,
		f.Accidents, f.Transfusions, f.Cardiopathies, f.Surgeries, f.Tabaquism, f.Alcoholism, f.Automedication,
		f.DrugUse, f.Pregnant)
	if err != nil {
		serverError(w, r, err)
		return
	}
	e, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[expedient])
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "No hay un expediente con esa CURP.")
		return
	}
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "Ya existe un expediente con esa CURP.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"expedient": e})
}

func (s *Server) deleteExpedient(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	tag, err := s.db.Exec(r.Context(), `DELETE FROM expedients WHERE clinic_id = $1 AND curp = $2`,
		p.ClinicID, normalizeCURP(chi.URLParam(r, "curp")))
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "No hay un expediente con esa CURP.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
