package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Odontogram and body-map snapshots. Each save is a new row (append-only); the history is the list.

const maxChartBytes = 64 << 10

type patientChart struct {
	ID            string          `json:"id"`
	PatientID     string          `json:"patient_id"`
	Kind          string          `json:"kind"`
	Data          json.RawMessage `json:"data"`
	Note          string          `json:"note"`
	EncounterID   *string         `json:"encounter_id"`
	CreatedByName string          `json:"created_by_name"`
	CreatedAt     time.Time       `json:"created_at"`
	// the author's professional data as it is today (empty for charts saved before the author was recorded)
	AuthorTitle  string `json:"author_title"`
	AuthorCedula string `json:"author_cedula"`
	AuthorPhone  string `json:"author_phone"`
	AuthorEmail  string `json:"author_email"`
}

const chartCols = `id, patient_id::text, kind, data, note, encounter_id::text, created_by_name, created_at,
	coalesce((SELECT u.specialty_title FROM users u WHERE u.id = patient_charts.created_by), ''), coalesce((SELECT u.cedula FROM users u WHERE u.id = patient_charts.created_by), ''),
	coalesce((SELECT u.phone FROM users u WHERE u.id = patient_charts.created_by), ''), coalesce((SELECT u.email FROM users u WHERE u.id = patient_charts.created_by), '')`

func scanChart(row pgx.Row) (patientChart, error) {
	var c patientChart
	err := row.Scan(&c.ID, &c.PatientID, &c.Kind, &c.Data, &c.Note, &c.EncounterID, &c.CreatedByName, &c.CreatedAt, &c.AuthorTitle, &c.AuthorCedula, &c.AuthorPhone, &c.AuthorEmail)
	return c, err
}

// ---- odontogram ----------------------------------------------------------------------------

var (
	toothStates   = []string{"caries", "restauracion", "endodoncia", "corona", "extraccion_indicada", "ausente", "sellador", "implante", "fractura", "protesis"}
	toothSurfaces = []string{"V", "L", "P", "M", "D", "O", "I"}
	dentitions    = []string{"adult", "child", "mixed"}
)

// validFDI reports whether n is a permanent (11-48) or deciduous (51-85) tooth in FDI notation.
func validFDI(n int) bool {
	q, t := n/10, n%10
	switch {
	case q >= 1 && q <= 4:
		return t >= 1 && t <= 8
	case q >= 5 && q <= 8:
		return t >= 1 && t <= 5
	}
	return false
}

type toothIn struct {
	State    string            `json:"state"`
	Surfaces map[string]string `json:"surfaces"`
	Note     string            `json:"note"`
}

type odontogramIn struct {
	Dentition string             `json:"dentition"`
	Teeth     map[string]toothIn `json:"teeth"`
}

func strictUnmarshal(raw []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func validateOdontogram(raw []byte) (string, bool) {
	var in odontogramIn
	if err := strictUnmarshal(raw, &in); err != nil {
		return "El odontograma no tiene el formato esperado.", false
	}
	if in.Dentition != "" && !slices.Contains(dentitions, in.Dentition) {
		return "La dentición debe ser adulto, niño o mixta.", false
	}
	if len(in.Teeth) > 52 {
		return "El odontograma tiene demasiadas piezas.", false
	}
	for k, t := range in.Teeth {
		n, err := strconv.Atoi(k)
		if err != nil || !validFDI(n) {
			return "«" + k + "» no es una pieza dental válida (notación FDI 11 a 48 y 51 a 85).", false
		}
		if t.State != "" && !slices.Contains(toothStates, t.State) {
			return "Estado no permitido en la pieza " + k + ".", false
		}
		for surf, st := range t.Surfaces {
			if !slices.Contains(toothSurfaces, surf) {
				return "Superficie inválida en la pieza " + k + ".", false
			}
			if !slices.Contains(toothStates, st) {
				return "Estado no permitido en la pieza " + k + ".", false
			}
		}
		if utf8.RuneCountInString(t.Note) > 300 {
			return "La nota de la pieza " + k + " es demasiado larga.", false
		}
	}
	return "", true
}

// ---- body map ------------------------------------------------------------------------------

var (
	bodyKinds = []string{"dolor", "contractura", "subluxacion", "parestesia", "otro"}
	bodyViews = []string{"front", "back"}
	paired    = func(names ...string) []string {
		var out []string
		for _, n := range names {
			out = append(out, n+"_l", n+"_r")
		}
		return out
	}
	bodyZones = map[string][]string{
		"front": append([]string{"head", "face", "neck", "chest", "abdomen", "groin"},
			paired("shoulder", "upper_arm", "elbow", "forearm", "hand", "hip", "thigh", "knee", "shin", "foot")...),
		"back": append([]string{"head", "cervical", "thoracic", "lumbar", "sacrum"},
			paired("shoulder", "upper_arm", "elbow", "forearm", "hand", "glute", "thigh", "knee", "calf", "foot")...),
	}
)

type bodyZoneIn struct {
	Zone      string `json:"zone"`
	View      string `json:"view"`
	Kind      string `json:"kind"`
	Intensity int    `json:"intensity"`
	Note      string `json:"note"`
}

type bodymapIn struct {
	Zones []bodyZoneIn `json:"zones"`
}

func validateBodymap(raw []byte) (string, bool) {
	var in bodymapIn
	if err := strictUnmarshal(raw, &in); err != nil {
		return "El mapa corporal no tiene el formato esperado.", false
	}
	if len(in.Zones) > 80 {
		return "El mapa corporal tiene demasiados hallazgos.", false
	}
	for _, z := range in.Zones {
		if !slices.Contains(bodyViews, z.View) {
			return "La vista debe ser frente o espalda.", false
		}
		if !slices.Contains(bodyZones[z.View], z.Zone) {
			return "«" + z.Zone + "» no es una zona válida de la vista " + z.View + ".", false
		}
		if !slices.Contains(bodyKinds, z.Kind) {
			return "Tipo de hallazgo no permitido.", false
		}
		if z.Intensity < 0 || z.Intensity > 10 {
			return "La intensidad debe estar entre 0 y 10.", false
		}
		if utf8.RuneCountInString(z.Note) > 300 {
			return "La nota de una zona es demasiado larga.", false
		}
	}
	return "", true
}

// ---- nutrition plan ------------------------------------------------------------------------

type nutritionMealIn struct {
	Name  string `json:"name"`
	Time  string `json:"time"`
	Items string `json:"items"`
	Kcal  int    `json:"kcal"`
}

// one day of a weekly plan
type nutritionDayIn struct {
	Name  string            `json:"name"`
	Meals []nutritionMealIn `json:"meals"`
}

type nutritionPlanIn struct {
	Goal            string            `json:"goal"`
	Basis           string            `json:"basis"`
	Kcal            int               `json:"kcal"`
	ProteinPct      int               `json:"protein_pct"`
	CarbPct         int               `json:"carb_pct"`
	FatPct          int               `json:"fat_pct"`
	WaterLiters     float64           `json:"water_liters"`
	Meals           []nutritionMealIn `json:"meals"`    // plans saved before the weekly format: one typical day
	Days            []nutritionDayIn  `json:"days"`     // the 7 days of the week
	Dislikes        string            `json:"dislikes"` // foods the patient does not like
	Recommendations string            `json:"recommendations"`
	Avoid           string            `json:"avoid"`
	Supplements     string            `json:"supplements"`
	FollowUpDays    int               `json:"follow_up_days"`
}

func validateNutritionPlan(raw []byte) (string, bool) {
	var in nutritionPlanIn
	if err := strictUnmarshal(raw, &in); err != nil {
		return "El plan nutricional no tiene el formato esperado.", false
	}
	long := func(v string, max int) bool { return utf8.RuneCountInString(v) > max }
	switch {
	case long(in.Goal, 300) || long(in.Basis, 400) || long(in.Recommendations, 3000) || long(in.Avoid, 1500) || long(in.Supplements, 600) || long(in.Dislikes, 1000):
		return "Algún texto del plan nutricional es demasiado largo.", false
	case in.Kcal < 0 || in.Kcal > 10000:
		return "Las calorías deben estar entre 0 y 10 000.", false
	case in.ProteinPct < 0 || in.CarbPct < 0 || in.FatPct < 0 || in.ProteinPct+in.CarbPct+in.FatPct > 100:
		return "Los porcentajes de macronutrientes no pueden sumar más de 100 %.", false
	case in.WaterLiters < 0 || in.WaterLiters > 10:
		return "El agua al día debe estar entre 0 y 10 litros.", false
	case in.FollowUpDays < 0 || in.FollowUpDays > 365:
		return "El seguimiento debe ser en un plazo de 0 a 365 días.", false
	case len(in.Meals) > 12:
		return "El plan tiene demasiadas comidas.", false
	case len(in.Days) > 7:
		return "El plan semanal tiene como máximo 7 días.", false
	}
	check := func(meals []nutritionMealIn) (string, bool) {
		if len(meals) > 12 {
			return "Un día tiene demasiadas comidas.", false
		}
		for _, m := range meals {
			if strings.TrimSpace(m.Name) == "" || long(m.Name, 60) || long(m.Time, 20) || long(m.Items, 1200) {
				return "Cada comida necesita un nombre corto y su detalle no puede ser tan largo.", false
			}
			if m.Kcal < 0 || m.Kcal > 10000 {
				return "Las calorías de una comida no son válidas.", false
			}
		}
		return "", true
	}
	if msg, ok := check(in.Meals); !ok {
		return msg, false
	}
	for _, d := range in.Days {
		if strings.TrimSpace(d.Name) == "" || long(d.Name, 40) {
			return "Cada día necesita un nombre corto.", false
		}
		if msg, ok := check(d.Meals); !ok {
			return msg, false
		}
	}
	return "", true
}

// ---- handlers ------------------------------------------------------------------------------

func (s *Server) listCharts(w http.ResponseWriter, r *http.Request) {
	id, _, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "odontogram" && kind != "bodymap" && kind != "nutrition_plan" && !isGenericChartKind(kind) {
		writeError(w, http.StatusBadRequest, "Tipo de esquema inválido.")
		return
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `SELECT `+chartCols+` FROM patient_charts WHERE clinic_id=$1 AND patient_id=$2 AND ($3 = '' OR kind=$3)
		ORDER BY created_at DESC LIMIT 100`, p.ClinicID, id, kind)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	list := []patientChart{}
	for rows.Next() {
		c, err := scanChart(rows)
		if err != nil {
			serverError(w, r, err)
			return
		}
		list = append(list, c)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	var latest *patientChart
	if len(list) > 0 {
		latest = &list[0]
	}
	s.logAccess(r.Context(), p.ClinicID, id, p, "view")
	writeJSON(w, http.StatusOK, map[string]any{"latest": latest, "charts": list})
}

func (s *Server) createChart(w http.ResponseWriter, r *http.Request) {
	id, _, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	var in struct {
		Kind        string          `json:"kind"`
		Data        json.RawMessage `json:"data"`
		Note        string          `json:"note"`
		EncounterID string          `json:"encounter_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	in.Note = strings.TrimSpace(in.Note)
	if len(in.Data) == 0 || len(in.Data) > maxChartBytes {
		writeError(w, http.StatusBadRequest, "El esquema está vacío o es demasiado grande.")
		return
	}
	if utf8.RuneCountInString(in.Note) > 500 {
		writeError(w, http.StatusBadRequest, "La nota es demasiado larga.")
		return
	}
	var msg string
	valid := false
	switch in.Kind {
	case "odontogram":
		msg, valid = validateOdontogram(in.Data)
	case "bodymap":
		msg, valid = validateBodymap(in.Data)
	case "nutrition_plan":
		msg, valid = validateNutritionPlan(in.Data)
	default:
		if isGenericChartKind(in.Kind) {
			var out json.RawMessage
			if msg, valid, out = validateGenericChart(in.Kind, in.Data); valid {
				in.Data = out
			}
		} else {
			msg = "Tipo de esquema inválido."
		}
	}
	if !valid {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	var enc any
	if in.EncounterID != "" {
		if !validUUID(in.EncounterID) || !s.encounterOfPatient(r, p.ClinicID, id, in.EncounterID) {
			writeError(w, http.StatusBadRequest, "El registro de la bitácora no es válido.")
			return
		}
		enc = in.EncounterID
	}
	row := s.db.QueryRow(r.Context(), `INSERT INTO patient_charts (clinic_id, patient_id, kind, data, note, encounter_id, created_by_name, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::uuid) RETURNING `+chartCols, p.ClinicID, id, in.Kind, []byte(in.Data), in.Note, enc, p.actorName(), p.UserID)
	c, err := scanChart(row)
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.logAccess(r.Context(), p.ClinicID, id, p, "view")
	writeJSON(w, http.StatusCreated, map[string]any{"chart": c})
}

func (s *Server) encounterOfPatient(r *http.Request, clinicID, patientID, encounterID string) bool {
	var found bool
	err := s.db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM encounters WHERE clinic_id=$1 AND patient_id=$2 AND id=$3)`, clinicID, patientID, encounterID).Scan(&found)
	return err == nil && found
}
