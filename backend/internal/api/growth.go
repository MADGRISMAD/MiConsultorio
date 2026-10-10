package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Growth charts: the patient's measurements come from the consultations (encounters.measures); the reference
// curves come only from tables an administrator imported (growth_references). Nothing official is embedded.

var growthStandardRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._-]{1,39}$`)

const growthMaxBody = 8 << 20

type growthImportIn struct {
	Standard   string `json:"standard"`
	SourceName string `json:"source_name"`
	FileName   string `json:"file_name"`
	CSV        string `json:"csv"`
	Confirm    bool   `json:"confirm"`
}

type growthGroupSummary struct {
	Indicator string  `json:"indicator"`
	Sex       string  `json:"sex"`
	Rows      int     `json:"rows"`
	MinAge    float64 `json:"min_age_months"`
	MaxAge    float64 `json:"max_age_months"`
}

type growthImportRow struct {
	ID         string    `json:"id"`
	Standard   string    `json:"standard"`
	Version    int       `json:"version"`
	SourceName string    `json:"source_name"`
	FileName   string    `json:"file_name"`
	SHA256     string    `json:"sha256"`
	RowCount   int       `json:"row_count"`
	CreatedBy  string    `json:"created_by_name"`
	CreatedAt  time.Time `json:"created_at"`
	// Platform is true for the tables that come with Caresia (not loaded by the clinic).
	Platform bool `json:"platform"`
}

func (s *Server) listGrowthImports(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `SELECT id::text, standard, version, source_name, file_name, sha256, row_count, created_by_name, created_at, clinic_id IS NULL
		FROM growth_imports WHERE clinic_id=$1 OR clinic_id IS NULL ORDER BY (clinic_id IS NULL), created_at DESC LIMIT 200`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []growthImportRow{}
	for rows.Next() {
		var x growthImportRow
		if err := rows.Scan(&x.ID, &x.Standard, &x.Version, &x.SourceName, &x.FileName, &x.SHA256, &x.RowCount, &x.CreatedBy, &x.CreatedAt, &x.Platform); err != nil {
			serverError(w, r, err)
			return
		}
		out = append(out, x)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"imports": out, "indicators": growthIndicators})
}

// importGrowthReferences validates a CSV (preview) and, when confirmed, stores it as a new version of its standard.
func (s *Server) importGrowthReferences(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, growthMaxBody)
	var in growthImportIn
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "El archivo es demasiado grande.")
		} else {
			writeError(w, http.StatusBadRequest, "Solicitud inválida.")
		}
		return
	}
	in.Standard, in.SourceName, in.FileName = strings.TrimSpace(in.Standard), strings.TrimSpace(in.SourceName), strings.TrimSpace(in.FileName)
	switch {
	case !growthStandardRe.MatchString(in.Standard):
		writeError(w, http.StatusBadRequest, "Escribe el nombre del estándar (por ejemplo OMS o CDC), de 2 a 40 caracteres.")
		return
	case in.SourceName == "" || labTooLong(in.SourceName, 200):
		writeError(w, http.StatusBadRequest, "Escribe la fuente de las tablas (nombre y dirección de donde las obtuviste).")
		return
	case labTooLong(in.FileName, 120) || strings.TrimSpace(in.CSV) == "":
		writeError(w, http.StatusBadRequest, "Carga un archivo CSV.")
		return
	}
	p := principalFrom(r.Context())
	res := parseGrowthCSV(in.CSV)
	sum := sha256.Sum256([]byte(in.CSV))
	digest := hex.EncodeToString(sum[:])

	var nextVersion int
	if err := s.db.QueryRow(r.Context(), `SELECT coalesce(max(version), 0) + 1 FROM growth_imports WHERE clinic_id=$1 AND standard=$2`, p.ClinicID, in.Standard).Scan(&nextVersion); err != nil {
		serverError(w, r, err)
		return
	}
	groups := map[string]*growthGroupSummary{}
	for _, row := range res.Rows {
		k := row.Indicator + "|" + row.Sex
		g := groups[k]
		if g == nil {
			g = &growthGroupSummary{Indicator: row.Indicator, Sex: row.Sex, MinAge: row.Age, MaxAge: row.Age}
			groups[k] = g
		}
		g.Rows++
		g.MinAge, g.MaxAge = math.Min(g.MinAge, row.Age), math.Max(g.MaxAge, row.Age)
	}
	summary := make([]growthGroupSummary, 0, len(groups))
	for _, g := range groups {
		summary = append(summary, *g)
	}
	sort.Slice(summary, func(i, j int) bool {
		if summary[i].Indicator != summary[j].Indicator {
			return summary[i].Indicator < summary[j].Indicator
		}
		return summary[i].Sex < summary[j].Sex
	})
	sample := []map[string]any{}
	for i, row := range res.Rows {
		if i >= 6 {
			break
		}
		m := map[string]any{"indicator": row.Indicator, "sex": row.Sex, "age_months": row.Age}
		if row.L != nil {
			m["l"], m["m"], m["s"] = *row.L, *row.M, *row.S
		}
		for k, v := range row.Pcts {
			m["p"+strconv.Itoa(k)] = v
		}
		sample = append(sample, m)
	}
	preview := map[string]any{
		"valid": res.Total == 0, "errors": res.Errors, "error_count": res.Total, "rows": len(res.Rows), "has_lms": res.HasLMS,
		"groups": summary, "sample": sample, "standard": in.Standard, "next_version": nextVersion, "sha256": digest, "columns": res.Columns,
	}
	if !in.Confirm {
		writeJSON(w, http.StatusOK, map[string]any{"preview": preview})
		return
	}
	if res.Total > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "El archivo tiene errores: corrígelos y vuelve a cargarlo.", "preview": preview})
		return
	}
	var out growthImportRow
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, p.ClinicID+"|growth|"+in.Standard); err != nil {
			return err
		}
		if err := tx.QueryRow(r.Context(), `SELECT coalesce(max(version), 0) + 1 FROM growth_imports WHERE clinic_id=$1 AND standard=$2`, p.ClinicID, in.Standard).Scan(&nextVersion); err != nil {
			return err
		}
		err := tx.QueryRow(r.Context(), `INSERT INTO growth_imports (clinic_id, standard, version, source_name, file_name, sha256, row_count, created_by_name)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id::text, standard, version, source_name, file_name, sha256, row_count, created_by_name, created_at`,
			p.ClinicID, in.Standard, nextVersion, in.SourceName, in.FileName, digest, len(res.Rows), p.actorName()).
			Scan(&out.ID, &out.Standard, &out.Version, &out.SourceName, &out.FileName, &out.SHA256, &out.RowCount, &out.CreatedBy, &out.CreatedAt)
		if err != nil {
			return err
		}
		batch := &pgx.Batch{}
		for _, row := range res.Rows {
			pcts := map[string]float64{}
			for k, v := range row.Pcts {
				pcts[strconv.Itoa(k)] = v
			}
			raw, _ := json.Marshal(pcts)
			batch.Queue(`INSERT INTO growth_references (clinic_id, import_id, standard, indicator, sex, age_months, l, m, s, pcts) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb)`,
				p.ClinicID, out.ID, in.Standard, row.Indicator, row.Sex, row.Age, row.L, row.M, row.S, string(raw))
		}
		br := tx.SendBatch(r.Context(), batch)
		for range res.Rows {
			if _, err := br.Exec(); err != nil {
				_ = br.Close()
				return err
			}
		}
		return br.Close()
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "growth_import", fmt.Sprintf("Importó tablas de crecimiento «%s» versión %d", in.Standard, out.Version),
		map[string]any{"standard": in.Standard, "version": out.Version, "source": in.SourceName, "rows": out.RowCount, "sha256": digest, "file": in.FileName})
	writeJSON(w, http.StatusCreated, map[string]any{"import": out, "preview": preview})
}

// ---- patient growth ----------------------------------------------------------------------------------------------

type growthMeasurement struct {
	EncounterID string   `json:"encounter_id"`
	Date        string   `json:"date"`
	AgeMonths   *float64 `json:"age_months"`
	WeightKg    *float64 `json:"weight_kg"`
	HeightCm    *float64 `json:"height_cm"`
	BMI         *float64 `json:"bmi"`
	HeadCm      *float64 `json:"head_cm"`
}

type growthPoint struct {
	Date       string   `json:"date"`
	AgeMonths  *float64 `json:"age_months"`
	Value      float64  `json:"value"`
	Z          *float64 `json:"z"`
	Percentile *float64 `json:"percentile"`
}

type growthCurvePoint struct {
	AgeMonths float64 `json:"age_months"`
	Value     float64 `json:"value"`
}

type growthIndicatorOut struct {
	Indicator    string                        `json:"indicator"`
	Unit         string                        `json:"unit"`
	HasReference bool                          `json:"has_reference"`
	Points       []growthPoint                 `json:"points"`
	Curves       map[string][]growthCurvePoint `json:"curves"`
	MinAge       *float64                      `json:"ref_min_age_months"`
	MaxAge       *float64                      `json:"ref_max_age_months"`
}

type growthStandardOut struct {
	Standard   string    `json:"standard"`
	Version    int       `json:"version"`
	SourceName string    `json:"source_name"`
	ImportedAt time.Time `json:"imported_at"`
}

// growthNumber reads a measure that may have been stored as a number or as text ("12,5").
func growthNumber(v any, max float64) *float64 {
	var x float64
	switch t := v.(type) {
	case float64:
		x = t
	case string:
		var err error
		if x, err = strconv.ParseFloat(strings.Replace(strings.TrimSpace(t), ",", ".", 1), 64); err != nil {
			return nil
		}
	default:
		return nil
	}
	if math.IsNaN(x) || x <= 0 || x > max {
		return nil
	}
	return &x
}

func growthRound(x float64, places int) float64 {
	f := math.Pow(10, float64(places))
	return math.Round(x*f) / f
}

// growthAgeMonths is the age in months (30.4375-day months) at a date, or nil when the birth date is unknown.
func growthAgeMonths(birth *time.Time, at time.Time) *float64 {
	if birth == nil {
		return nil
	}
	days := at.Sub(*birth).Hours() / 24
	if days < 0 {
		return nil
	}
	m := growthRound(days/30.4375, 2)
	return &m
}

var growthCurveKeys = []int{3, 15, 50, 85, 97}

func (s *Server) patientGrowth(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	var subject, sex string
	var birth *time.Time
	err := s.db.QueryRow(r.Context(), `SELECT subject, sex, birth_date FROM patients WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id).Scan(&subject, &sex, &birth)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT id::text, occurred_at, measures FROM encounters
		WHERE clinic_id=$1 AND patient_id=$2 AND (measures ? 'weight_kg' OR measures ? 'height_cm' OR measures ? 'head_circumference') AND (NOT private OR author_id = $3)
		ORDER BY occurred_at ASC LIMIT 2000`, p.ClinicID, id, p.UserID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	meas := []growthMeasurement{}
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
		g := growthMeasurement{EncounterID: eid, Date: at.Format("2006-01-02"), WeightKg: growthNumber(m["weight_kg"], 1500),
			HeightCm: growthNumber(m["height_cm"], 300), HeadCm: growthNumber(m["head_circumference"], 100)}
		if g.WeightKg == nil && g.HeightCm == nil && g.HeadCm == nil {
			continue
		}
		if g.WeightKg != nil && g.HeightCm != nil {
			b := growthRound(*g.WeightKg/math.Pow(*g.HeightCm/100, 2), 2)
			g.BMI = &b
		}
		day, _ := time.Parse("2006-01-02", g.Date)
		g.AgeMonths = growthAgeMonths(birth, day)
		meas = append(meas, g)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}

	resp := map[string]any{"subject": subject, "sex": sex, "birth_date": nil, "measurements": meas, "standards": []growthStandardOut{}, "standard": "", "indicators": map[string]growthIndicatorOut{}}
	if birth != nil {
		resp["birth_date"] = birth.Format("2006-01-02")
		resp["age_months"] = growthAgeMonths(birth, time.Now())
	}
	refSex := map[string]string{"Hombre": "M", "Mujer": "F"}[sex]
	switch {
	case subject != "person":
		resp["reference_status"] = "animal"
	case refSex == "":
		resp["reference_status"] = "no_sex"
	case birth == nil:
		resp["reference_status"] = "no_birth_date"
	default:
		resp["reference_status"] = "ok"
	}
	if resp["reference_status"] == "ok" {
		if err := s.growthAttachReferences(r, p.ClinicID, refSex, strings.TrimSpace(r.URL.Query().Get("standard")), meas, resp); err != nil {
			serverError(w, r, err)
			return
		}
		if resp["standard"] == "" {
			resp["reference_status"] = "no_references"
		}
	}
	s.logAccess(r.Context(), p.ClinicID, id, p, "view")
	writeJSON(w, http.StatusOK, resp)
}

// growthAttachReferences fills resp with the standards available for the sex and, for the chosen one, the curves and
// the score of each measurement.
func (s *Server) growthAttachReferences(r *http.Request, clinicID, sex, wanted string, meas []growthMeasurement, resp map[string]any) error {
	rows, err := s.db.Query(r.Context(), `SELECT DISTINCT ON (i.standard) i.standard, i.version, i.source_name, i.created_at
		FROM growth_imports i WHERE (i.clinic_id=$1 OR i.clinic_id IS NULL) AND EXISTS (SELECT 1 FROM growth_references g WHERE g.import_id = i.id AND g.sex=$2)
		ORDER BY i.standard, (i.clinic_id IS NULL), i.version DESC`, clinicID, sex)
	if err != nil {
		return err
	}
	standards := []growthStandardOut{}
	for rows.Next() {
		var x growthStandardOut
		if err := rows.Scan(&x.Standard, &x.Version, &x.SourceName, &x.ImportedAt); err != nil {
			rows.Close()
			return err
		}
		standards = append(standards, x)
	}
	rows.Close()
	if rows.Err() != nil {
		return rows.Err()
	}
	resp["standards"] = standards
	if len(standards) == 0 {
		return nil
	}
	chosen := standards[0].Standard
	for _, x := range standards {
		if x.Standard == wanted {
			chosen = wanted
		}
	}
	resp["standard"] = chosen
	out := map[string]growthIndicatorOut{}
	for _, ind := range growthIndicators {
		refs, err := s.growthLoadRefs(r, clinicID, chosen, ind, sex)
		if err != nil {
			return err
		}
		out[ind] = growthBuildIndicator(ind, refs, meas)
	}
	resp["indicators"] = out
	return nil
}

// growthLoadRefs loads the rows of the latest import of the standard that has this indicator and sex.
func (s *Server) growthLoadRefs(r *http.Request, clinicID, standard, indicator, sex string) ([]growthRef, error) {
	rows, err := s.db.Query(r.Context(), `SELECT g.age_months::float8, g.l::float8, g.m::float8, g.s::float8, g.pcts::text FROM growth_references g
		WHERE g.indicator=$3 AND g.sex=$4 AND g.import_id = (
			SELECT i.id FROM growth_imports i WHERE (i.clinic_id=$1 OR i.clinic_id IS NULL) AND i.standard=$2
			AND EXISTS (SELECT 1 FROM growth_references x WHERE x.import_id = i.id AND x.indicator=$3 AND x.sex=$4)
			ORDER BY (i.clinic_id IS NULL), i.version DESC LIMIT 1) ORDER BY g.age_months`, clinicID, standard, indicator, sex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []growthRef
	for rows.Next() {
		var g growthRef
		var raw string
		if err := rows.Scan(&g.Age, &g.L, &g.M, &g.S, &raw); err != nil {
			return nil, err
		}
		var m map[string]float64
		if json.Unmarshal([]byte(raw), &m) == nil && len(m) > 0 {
			g.Pcts = map[int]float64{}
			for k, v := range m {
				if n, err := strconv.Atoi(k); err == nil {
					g.Pcts[n] = v
				}
			}
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

var growthUnits = map[string]string{"weight_for_age": "kg", "length_height_for_age": "cm", "bmi_for_age": "kg/m²", "head_circumference_for_age": "cm"}

// growthBuildIndicator draws the percentile curves and scores the measurements of one indicator.
func growthBuildIndicator(indicator string, refs []growthRef, meas []growthMeasurement) growthIndicatorOut {
	out := growthIndicatorOut{Indicator: indicator, Unit: growthUnits[indicator], Points: []growthPoint{}, Curves: map[string][]growthCurvePoint{}}
	for _, m := range meas {
		var v *float64
		switch indicator {
		case "weight_for_age":
			v = m.WeightKg
		case "length_height_for_age":
			v = m.HeightCm
		case "bmi_for_age":
			v = m.BMI
		default:
			v = m.HeadCm
		}
		if v == nil {
			continue
		}
		pt := growthPoint{Date: m.Date, AgeMonths: m.AgeMonths, Value: *v}
		if len(refs) > 0 && m.AgeMonths != nil {
			if ref, ok := growthInterp(refs, *m.AgeMonths); ok {
				if z, pct, ok := growthScore(ref, *v); ok {
					z, pct = growthRound(z, 2), growthRound(pct, 1)
					pt.Z, pt.Percentile = &z, &pct
				}
			}
		}
		out.Points = append(out.Points, pt)
	}
	if len(refs) == 0 {
		return out
	}
	out.HasReference = true
	lo, hi := refs[0].Age, refs[len(refs)-1].Age
	out.MinAge, out.MaxAge = &lo, &hi
	for _, k := range growthCurveKeys {
		key := "p" + strconv.Itoa(k)
		for _, ref := range refs {
			var v float64
			var ok bool
			if ref.hasLMS() {
				v, ok = growthValueAtZ(growthZFromPercentile(float64(k)), *ref.L, *ref.M, *ref.S)
			} else {
				v, ok = ref.Pcts[k]
			}
			if ok {
				out.Curves[key] = append(out.Curves[key], growthCurvePoint{AgeMonths: ref.Age, Value: growthRound(v, 3)})
			}
		}
	}
	return out
}

// growthImportDetail is what one table covers: for each indicator and sex, how many rows and from which age to which.
// Tables that come with Caresia are visible to every clinic; a clinic's own are visible only to it.
func (s *Server) growthImportDetail(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "No encontramos esa tabla.")
		return
	}
	var out growthImportRow
	err := s.db.QueryRow(r.Context(), `SELECT id::text, standard, version, source_name, file_name, sha256, row_count, created_by_name, created_at, clinic_id IS NULL
		FROM growth_imports WHERE id = $1 AND (clinic_id = $2 OR clinic_id IS NULL)`, id, p.ClinicID).
		Scan(&out.ID, &out.Standard, &out.Version, &out.SourceName, &out.FileName, &out.SHA256, &out.RowCount, &out.CreatedBy, &out.CreatedAt, &out.Platform)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "No encontramos esa tabla.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT indicator, sex, count(*), min(age_months)::float8, max(age_months)::float8, bool_or(l IS NOT NULL)
		FROM growth_references WHERE import_id = $1 GROUP BY indicator, sex ORDER BY indicator, sex`, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	groups := []map[string]any{}
	for rows.Next() {
		var ind, sex string
		var n int
		var lo, hi float64
		var lms bool
		if err := rows.Scan(&ind, &sex, &n, &lo, &hi, &lms); err != nil {
			serverError(w, r, err)
			return
		}
		groups = append(groups, map[string]any{"indicator": ind, "sex": sex, "rows": n, "min_age_months": lo, "max_age_months": hi, "has_lms": lms})
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"import": out, "groups": groups})
}
