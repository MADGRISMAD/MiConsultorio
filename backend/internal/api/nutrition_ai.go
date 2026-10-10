package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// AI draft of a weekly nutrition plan. The numbers (calories, macronutrients, water, kcal per meal) are computed here
// with Mifflin-St Jeor so they are right; Gemini only chooses the foods and portions of the 7 days, and the nutritionist
// reviews and edits everything before saving. Only what the plan needs leaves the server: age, sex, body measures and
// the food-related history. Never the name or contact data.

var nutritionDaySchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"goal": map[string]any{"type": "STRING"},
		"days": map[string]any{"type": "ARRAY", "items": map[string]any{
			"type": "OBJECT",
			"properties": map[string]any{
				"name": map[string]any{"type": "STRING"},
				"meals": map[string]any{"type": "ARRAY", "items": map[string]any{
					"type":       "OBJECT",
					"properties": map[string]any{"name": map[string]any{"type": "STRING"}, "items": map[string]any{"type": "STRING"}},
					"required":   []string{"name", "items"},
				}},
			},
			"required": []string{"name", "meals"},
		}},
		"recommendations": map[string]any{"type": "STRING"},
		"avoid":           map[string]any{"type": "STRING"},
		"supplements":     map[string]any{"type": "STRING"},
		"follow_up_days":  map[string]any{"type": "NUMBER"},
	},
	"required": []string{"goal", "days", "recommendations"},
}

type nutritionAIReq struct {
	Goal           string  `json:"goal"`
	WeightKg       float64 `json:"weight_kg"`
	HeightCm       float64 `json:"height_cm"`
	ActivityFactor float64 `json:"activity_factor"`
	Activity       string  `json:"activity"`
	Kcal           int     `json:"kcal"`   // optional: the nutritionist's own target
	Meals          int     `json:"meals"`  // 3 to 6 meals a day (used when Snacks is not given)
	Snacks         *bool   `json:"snacks"` // true: breakfast, lunch, dinner and two snacks; false: only the three meals
	Preferences    string  `json:"preferences"`
	Dislikes       string  `json:"dislikes"` // foods the patient does not like
	BodyFatPct     float64 `json:"body_fat_pct"`
}

var nutritionWeek = []string{"Lunes", "Martes", "Miércoles", "Jueves", "Viernes", "Sábado", "Domingo"}

type mealSlot struct {
	Name, Time string
	Pct        int
}

// the day's structure by number of meals; the shares add up to 100
var nutritionSlots = map[int][]mealSlot{
	3: {{"Desayuno", "08:00", 30}, {"Comida", "14:30", 40}, {"Cena", "20:30", 30}},
	4: {{"Desayuno", "08:00", 25}, {"Snack", "11:30", 10}, {"Comida", "14:30", 35}, {"Cena", "20:30", 30}},
	5: {{"Desayuno", "08:00", 25}, {"Snack", "11:00", 10}, {"Comida", "14:30", 30}, {"Snack", "17:30", 10}, {"Cena", "20:30", 25}},
	6: {{"Desayuno", "08:00", 20}, {"Snack", "10:30", 10}, {"Comida", "14:00", 30}, {"Snack", "17:00", 10}, {"Cena", "20:00", 20}, {"Snack", "22:00", 10}},
}

var nutritionAdjust = map[string]int{"Bajar de peso": -500, "Subir de peso": 400, "Ganar masa muscular": 300}

// protein in g per kg of body weight and fat share of the calories, by goal (anything else is maintenance)
var nutritionProtein = map[string]float64{"Bajar de peso": 1.8, "Ganar masa muscular": 2.0, "Subir de peso": 1.6, "Control de enfermedad": 1.2, "Alimentación saludable": 1.2}
var nutritionFat = map[string]int{"Bajar de peso": 30, "Ganar masa muscular": 25, "Subir de peso": 30, "Control de enfermedad": 30}

type nutritionTargets struct {
	BMR        int     `json:"bmr"`
	TDEE       int     `json:"tdee"`
	Adjust     int     `json:"adjust"`
	Kcal       int     `json:"kcal"`
	ProteinPct int     `json:"protein_pct"`
	CarbPct    int     `json:"carb_pct"`
	FatPct     int     `json:"fat_pct"`
	ProteinG   int     `json:"protein_g"`
	CarbG      int     `json:"carb_g"`
	FatG       int     `json:"fat_g"`
	WaterL     float64 `json:"water_liters"`
	BMI        float64 `json:"bmi,omitempty"`
	Method     string  `json:"method"`
	Basis      string  `json:"basis"`
}

// computeTargets is Katch-McArdle (from the lean mass) when the body fat is known and Mifflin-St Jeor otherwise, times
// the activity factor, adjusted to the goal (never more than 20 % under or 15 % over, never below a safe floor).
// Protein is set per kg of body weight, fat by goal, and carbohydrates are the rest.
func computeTargets(sex string, age int, weightKg, heightCm, activity, bodyFatPct float64, goal string) nutritionTargets {
	var t nutritionTargets
	var b float64
	if bodyFatPct >= 3 && bodyFatPct <= 60 {
		lean := weightKg * (1 - bodyFatPct/100)
		b = 370 + 21.6*lean
		t.Method = "Katch-McArdle"
	} else {
		sexTerm := -78.0 // another / unspecified: halfway between the two
		switch sex {
		case "Hombre":
			sexTerm = 5
		case "Mujer":
			sexTerm = -161
		}
		b = 10*weightKg + 6.25*heightCm - 5*float64(age) + sexTerm
		t.Method = "Mifflin-St Jeor"
	}
	if activity < 1.2 || activity > 2.0 {
		activity = 1.375
	}
	tdee := b * activity
	adj := float64(nutritionAdjust[goal])
	if adj < 0 {
		adj = math.Max(adj, -0.2*tdee)
	} else {
		adj = math.Min(adj, 0.15*tdee)
	}
	floor := 1200
	if sex == "Hombre" {
		floor = 1500
	}
	t.BMR, t.TDEE = int(math.Round(b)), int(math.Round(tdee))
	t.Kcal = max(floor, int(math.Round((tdee+adj)/10))*10)
	t.Adjust = t.Kcal - t.TDEE

	gkg, ok := nutritionProtein[goal]
	if !ok {
		gkg = 1.4
	}
	fat, ok := nutritionFat[goal]
	if !ok {
		fat = 30
	}
	t.ProteinPct = min(35, max(15, int(math.Round(gkg*weightKg*4/float64(t.Kcal)*100))))
	t.FatPct = fat
	t.CarbPct = 100 - t.ProteinPct - t.FatPct
	t.ProteinG = int(math.Round(float64(t.Kcal*t.ProteinPct) / 400))
	t.CarbG = int(math.Round(float64(t.Kcal*t.CarbPct) / 400))
	t.FatG = int(math.Round(float64(t.Kcal*t.FatPct) / 900))
	t.WaterL = math.Min(4, math.Max(1.5, math.Round(weightKg*0.035*10)/10))
	if heightCm > 0 {
		m := heightCm / 100
		t.BMI = math.Round(weightKg/(m*m)*10) / 10
	}
	t.Basis = fmt.Sprintf("Calorías calculadas con %s: metabolismo basal %d kcal × actividad %.3g, ajustado al objetivo (%+d kcal). Proteína %d g (%.1f g/kg), carbohidratos %d g, grasas %d g.",
		t.Method, t.BMR, activity, t.Adjust, t.ProteinG, float64(t.ProteinG)/weightKg, t.CarbG, t.FatG)
	return t
}

func profileText(prof map[string]any, key string) string {
	switch v := prof[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return strings.Join(out, ", ")
	}
	return ""
}

func nonEmpty(v ...string) []string {
	out := []string{}
	for _, x := range v {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

var accentless = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")

func foldText(s string) string { return accentless.Replace(strings.ToLower(s)) }

var termSplit = regexp.MustCompile(`[,;\n]+|\s+y\s+|\s+ni\s+`)

// foodTerms are the foods to keep out, from a free text ("pescado, hígado y brócoli").
func foodTerms(texts ...string) []string {
	var out []string
	for _, t := range texts {
		for _, part := range termSplit.Split(foldText(t), -1) {
			part = strings.Trim(strings.TrimSpace(part), ".")
			part = strings.TrimPrefix(part, "no le gusta el ")
			part = strings.TrimPrefix(part, "no le gusta la ")
			if utf8.RuneCountInString(part) >= 3 && utf8.RuneCountInString(part) <= 40 {
				out = append(out, part)
			}
		}
	}
	return out
}

// forbiddenIn lists where a day's foods mention something that must stay out ("Lunes · Comida: pescado").
func forbiddenIn(days []nutritionDayIn, terms []string) []string {
	var hits []string
	for _, d := range days {
		for _, m := range d.Meals {
			text := foldText(m.Items)
			for _, t := range terms {
				if regexp.MustCompile(`\b` + regexp.QuoteMeta(t) + `(e?s)?\b`).MatchString(text) {
					hits = append(hits, d.Name+" · "+m.Name+": "+t)
				}
			}
		}
	}
	return hits
}

type nutritionAIOut struct {
	Goal            string  `json:"goal"`
	Recommendations string  `json:"recommendations"`
	Avoid           string  `json:"avoid"`
	Supplements     string  `json:"supplements"`
	FollowUpDays    float64 `json:"follow_up_days"`
	Days            []struct {
		Name  string `json:"name"`
		Meals []struct {
			Name  string `json:"name"`
			Items string `json:"items"`
		} `json:"meals"`
	} `json:"days"`
}

func (s *Server) nutritionPlanAI(w http.ResponseWriter, r *http.Request) {
	id, subject, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	if !s.requireCedula(w, r, "generar planes de alimentación") {
		return
	}
	var in nutritionAIReq
	if !decode(w, r, &in) {
		return
	}
	in.Goal, in.Activity, in.Preferences, in.Dislikes = strings.TrimSpace(in.Goal), strings.TrimSpace(in.Activity), strings.TrimSpace(in.Preferences), strings.TrimSpace(in.Dislikes)
	switch {
	case subject != "person":
		writeError(w, http.StatusBadRequest, "El plan nutricional es para personas.")
		return
	case in.Goal == "" || utf8.RuneCountInString(in.Goal) > 200:
		writeError(w, http.StatusBadRequest, "Escribe el objetivo del plan (máximo 200 caracteres).")
		return
	case utf8.RuneCountInString(in.Preferences) > 1500 || utf8.RuneCountInString(in.Activity) > 100 || utf8.RuneCountInString(in.Dislikes) > 1000:
		writeError(w, http.StatusBadRequest, "El texto es demasiado largo.")
		return
	case in.WeightKg < 0 || in.WeightKg > 400 || in.HeightCm < 0 || in.HeightCm > 260 || in.Kcal < 0 || in.Kcal > 8000:
		writeError(w, http.StatusBadRequest, "Revisa el peso, la talla y las calorías.")
		return
	}
	if in.Snacks != nil {
		in.Meals = map[bool]int{true: 5, false: 3}[*in.Snacks]
	}
	if in.Meals < 3 || in.Meals > 6 {
		in.Meals = 5
	}
	if s.cfg.GeminiAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "La IA no está configurada en el servidor (GEMINI_API_KEY)."})
		return
	}

	p := principalFrom(r.Context())
	var sex string
	var age *int
	var raw []byte
	if err := s.db.QueryRow(r.Context(), `SELECT sex, CASE WHEN birth_date IS NULL THEN NULL ELSE date_part('year', age(birth_date))::int END, profile
		FROM patients WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id).Scan(&sex, &age, &raw); err != nil {
		serverError(w, r, err)
		return
	}
	prof, err := decProfile(id, raw)
	if err != nil {
		serverError(w, r, err)
		return
	}

	// the numbers are computed here, not by the model
	target, basis := in.Kcal, "Calorías indicadas por el nutriólogo."
	var tg nutritionTargets
	if in.WeightKg > 0 && in.HeightCm > 0 && age != nil {
		tg = computeTargets(sex, *age, in.WeightKg, in.HeightCm, in.ActivityFactor, in.BodyFatPct, in.Goal)
	}
	if target == 0 {
		if tg.Kcal == 0 {
			writeError(w, http.StatusBadRequest, "Para calcular las calorías escribe el peso y la talla (y registra la fecha de nacimiento del paciente), o indica las calorías tú.")
			return
		}
		target, basis = tg.Kcal, tg.Basis
	}
	macros := [3]int{50, 20, 30} // carb, protein, fat
	if tg.Kcal > 0 {
		macros = [3]int{tg.CarbPct, tg.ProteinPct, tg.FatPct}
	}
	slots := nutritionSlots[in.Meals]
	water := 2.0
	if in.WeightKg > 0 {
		water = tg.WaterL
		if water == 0 {
			water = math.Min(4, math.Max(1.5, math.Round(in.WeightKg*0.035*10)/10))
		}
	}

	if err := s.spendMagic(r.Context(), p); err != nil {
		writeFailure(w, r, err)
		return
	}

	var ctx []string
	add := func(label, v string) {
		if v != "" {
			ctx = append(ctx, label+": "+v)
		}
	}
	if age != nil {
		add("Edad", fmt.Sprintf("%d años", *age))
	}
	add("Sexo", sex)
	if in.WeightKg > 0 {
		add("Peso", fmt.Sprintf("%.1f kg", in.WeightKg))
	}
	if in.HeightCm > 0 {
		add("Talla", fmt.Sprintf("%.0f cm", in.HeightCm))
	}
	if tg.BMI > 0 {
		add("IMC", fmt.Sprintf("%.1f", tg.BMI))
	}
	if in.BodyFatPct > 0 {
		add("Grasa corporal", fmt.Sprintf("%.1f %%", in.BodyFatPct))
	}
	add("Actividad física", in.Activity)
	foodAllergies := profileText(prof, "food_allergies")
	add("Alergias (medicamentos y otras)", profileText(prof, "allergies_text"))
	add("Alergias o intolerancias alimentarias", foodAllergies)
	add("Enfermedades crónicas", strings.Join(nonEmpty(profileText(prof, "chronic_conditions"), profileText(prof, "chronic_other")), "; "))
	add("Medicamentos actuales", profileText(prof, "current_medication"))
	add("Cómo come en un día normal", profileText(prof, "diet_pattern"))

	var shares []string
	for _, sl := range slots {
		shares = append(shares, fmt.Sprintf("- %s (%s): aprox. %d kcal", sl.Name, sl.Time, int(math.Round(float64(target*sl.Pct)/1000))*10))
	}
	basePrompt := "Eres un asistente para nutriólogos en México. Redacta el menú de UNA SEMANA (los 7 días: " + strings.Join(nutritionWeek, ", ") + ") como BORRADOR que el nutriólogo revisará y corregirá. " +
		"Responde SOLO con JSON. Cada día tiene exactamente estas " + fmt.Sprint(in.Meals) + " comidas (los snacks son colaciones ligeras entre comidas), con esos nombres y en este orden, y las porciones deben acercarse a las calorías indicadas:\n" + strings.Join(shares, "\n") + "\n" +
		fmt.Sprintf("Objetivo: %s. Total diario: %d kcal; proteínas %d %%, carbohidratos %d %%, grasas %d %%. ", in.Goal, target, macros[1], macros[0], macros[2]) +
		"En cada comida escribe los alimentos con porciones concretas (tazas, piezas, gramos). Usa alimentos comunes y accesibles en México y VARÍA el menú: no repitas el mismo platillo principal más de dos veces en la semana. " +
		"NUNCA incluyas alimentos a los que el paciente sea alérgico o intolerante, y respeta sus enfermedades y su medicación. " +
		"No propongas suplementos con dosis; si dudas, deja suplementos vacío. En recomendaciones da 4 a 6 consejos breves, en 'avoid' lo que conviene limitar, y sugiere en follow_up_days cuándo darle seguimiento."
	if in.Dislikes != "" {
		basePrompt += "\n\nALIMENTOS QUE NO LE GUSTAN AL PACIENTE (no los uses en ninguna comida; busca sustitutos con valor nutricional equivalente que sí le gusten): " + in.Dislikes
	}
	basePrompt += "\n\nDATOS DEL PACIENTE (sin nombre):\n" + strings.Join(ctx, "\n")
	if in.Preferences != "" {
		basePrompt += "\n\nPREFERENCIAS Y RESTRICCIONES QUE INDICA EL NUTRIÓLOGO:\n" + in.Preferences
	}
	banned := foodTerms(in.Dislikes, foodAllergies)

	// the patient's earlier plans: this week must look different, so the model is told what was served before
	history := s.previousMenus(r.Context(), p.ClinicID, id, 3)
	attempts := 2
	if len(history) > 0 {
		basePrompt += "\n\n" + menuHistoryPrompt(history)
		attempts = 3 // one more try when the first draft still copies the previous menu
	}

	var plan nutritionPlanIn
	var warnings []string
	prompt := basePrompt
	for attempt := 0; attempt < attempts; attempt++ {
		rawPlan, err := s.gemini(r.Context(), prompt, nil, "", nutritionDaySchema)
		if err != nil {
			s.refundMagic(r.Context(), p)
			logf(r, "nutrition ai: %v", err)
			writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA no pudo armar el plan. Intenta de nuevo en un momento."})
			return
		}
		var out nutritionAIOut
		if json.Unmarshal(rawPlan, &out) != nil || len(out.Days) != 7 {
			if attempt == attempts-1 {
				s.refundMagic(r.Context(), p)
				writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA devolvió un plan incompleto. Intenta de nuevo."})
				return
			}
			prompt = basePrompt + "\n\nIMPORTANTE: la respuesta anterior no traía los 7 días completos. Devuelve exactamente 7 días."
			continue
		}
		plan = nutritionPlanIn{Goal: out.Goal, Kcal: target, CarbPct: macros[0], ProteinPct: macros[1], FatPct: macros[2], WaterLiters: water,
			Recommendations: out.Recommendations, Avoid: out.Avoid, Supplements: out.Supplements, FollowUpDays: int(out.FollowUpDays + 0.5), Dislikes: in.Dislikes}
		complete := true
		for i, d := range out.Days {
			day := nutritionDayIn{Name: nutritionWeek[i]}
			if len(d.Meals) < len(slots) {
				complete = false
				break
			}
			left := target
			for j, sl := range slots {
				k := int(math.Round(float64(target*sl.Pct)/1000)) * 10
				if j == len(slots)-1 {
					k = left // the last meal closes the day exactly
				}
				left -= k
				day.Meals = append(day.Meals, nutritionMealIn{Name: sl.Name, Time: sl.Time, Items: strings.TrimSpace(d.Meals[j].Items), Kcal: k})
			}
			plan.Days = append(plan.Days, day)
		}
		if !complete {
			if attempt == attempts-1 {
				s.refundMagic(r.Context(), p)
				writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA devolvió comidas incompletas. Intenta de nuevo."})
				return
			}
			prompt = basePrompt + fmt.Sprintf("\n\nIMPORTANTE: cada día debe traer las %d comidas indicadas.", in.Meals)
			continue
		}
		hits := forbiddenIn(plan.Days, banned)
		if len(hits) > 0 {
			warnings = hits
			prompt = basePrompt + "\n\nIMPORTANTE: en el intento anterior aparecieron alimentos prohibidos (" + strings.Join(hits[:min(len(hits), 6)], "; ") + "). Reemplázalos por otros y no los uses en ninguna comida."
			continue
		}
		warnings = nil
		if len(history) > 0 {
			if share := repeatedShare(plan.Days, history); share > repeatShareMax && attempt < attempts-1 {
				prompt = basePrompt + fmt.Sprintf("\n\nIMPORTANTE: el borrador anterior repetía el %d %% de las comidas de los planes previos. Cámbialo: otros platillos principales, otros desayunos y otras colaciones, sin repetir los de los planes anteriores.", int(share*100))
				continue
			}
		}
		break
	}
	cleanNutritionPlan(&plan)
	plan.Basis = basis + " El menú es un borrador generado con IA: revísalo y ajústalo con tu criterio clínico."
	if _, ok := validateNutritionPlanStruct(plan); !ok {
		s.refundMagic(r.Context(), p)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA devolvió un plan fuera de rango. Intenta de nuevo."})
		return
	}
	if len(warnings) > 5 {
		warnings = warnings[:5]
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan, "warnings": warnings})
}

// cleanNutritionPlan trims and clamps text so the plan always passes validation.
func cleanNutritionPlan(p *nutritionPlanIn) {
	cut := func(v string, n int) string {
		v = strings.TrimSpace(v)
		if utf8.RuneCountInString(v) > n {
			v = string([]rune(v)[:n])
		}
		return v
	}
	clamp := func(v, lo, hi int) int { return max(lo, min(hi, v)) }
	p.Goal, p.Basis, p.Dislikes = cut(p.Goal, 300), "", cut(p.Dislikes, 1000)
	p.Recommendations, p.Avoid, p.Supplements = cut(p.Recommendations, 3000), cut(p.Avoid, 1500), cut(p.Supplements, 600)
	p.FollowUpDays = clamp(p.FollowUpDays, 0, 365)
	for i := range p.Days {
		for j := range p.Days[i].Meals {
			m := &p.Days[i].Meals[j]
			m.Name, m.Time, m.Items = cut(m.Name, 60), cut(m.Time, 20), cut(m.Items, 1200)
		}
	}
}

func validateNutritionPlanStruct(p nutritionPlanIn) (string, bool) {
	raw, _ := json.Marshal(p)
	return validateNutritionPlan(raw)
}

// nutritionCalc gives the energy and macronutrient targets for the measures typed in the form. It is free (no AI).
func (s *Server) nutritionCalc(w http.ResponseWriter, r *http.Request) {
	id, subject, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	var in nutritionAIReq
	if !decode(w, r, &in) {
		return
	}
	if subject != "person" {
		writeError(w, http.StatusBadRequest, "El plan nutricional es para personas.")
		return
	}
	if in.WeightKg <= 0 || in.WeightKg > 400 || in.HeightCm <= 0 || in.HeightCm > 260 || in.BodyFatPct < 0 || in.BodyFatPct > 80 {
		writeError(w, http.StatusBadRequest, "Escribe un peso y una talla válidos.")
		return
	}
	var sex string
	var age *int
	if err := s.db.QueryRow(r.Context(), `SELECT sex, CASE WHEN birth_date IS NULL THEN NULL ELSE date_part('year', age(birth_date))::int END
		FROM patients WHERE clinic_id=$1 AND id=$2`, principalFrom(r.Context()).ClinicID, id).Scan(&sex, &age); err != nil {
		serverError(w, r, err)
		return
	}
	if age == nil {
		writeError(w, http.StatusBadRequest, "Registra la fecha de nacimiento del paciente para calcular.")
		return
	}
	writeJSON(w, http.StatusOK, computeTargets(sex, *age, in.WeightKg, in.HeightCm, in.ActivityFactor, in.BodyFatPct, in.Goal))
}

// ---- variety against the patient's earlier plans ----

// repeatShareMax is how much of the new week may resemble earlier plans before the model is asked again.
const repeatShareMax = 0.25

// previousMenus are the weekly menus of the patient's last saved plans, newest first.
func (s *Server) previousMenus(ctx context.Context, clinicID, patientID string, n int) [][]nutritionDayIn {
	rows, err := s.db.Query(ctx, `SELECT data FROM patient_charts WHERE clinic_id = $1 AND patient_id = $2 AND kind = 'nutrition_plan' ORDER BY created_at DESC LIMIT $3`, clinicID, patientID, n)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out [][]nutritionDayIn
	for rows.Next() {
		var raw []byte
		var pl nutritionPlanIn
		if rows.Scan(&raw) != nil || json.Unmarshal(raw, &pl) != nil {
			continue
		}
		days := pl.Days
		if len(days) == 0 && len(pl.Meals) > 0 {
			days = []nutritionDayIn{{Name: "Día tipo", Meals: pl.Meals}}
		}
		if len(days) > 0 {
			out = append(out, days)
		}
	}
	return out
}

// menuHistoryPrompt lists what each earlier plan served, by meal, so the model can avoid it.
func menuHistoryPrompt(history [][]nutritionDayIn) string {
	var b strings.Builder
	b.WriteString("PLANES ANTERIORES DE ESTE PACIENTE (ya los comió; la dieta debe CAMBIAR para que no coma lo mismo semana tras semana). " +
		"Propón platillos principales, desayunos y colaciones DISTINTOS a los de abajo: no los copies ni los reacomodes. Puedes conservar como máximo uno o dos favoritos si son muy adecuados. " +
		"Mantén las porciones y el aporte calórico indicados; cambia los alimentos, no la meta.\n")
	for i, plan := range history {
		seen := map[string]map[string]bool{}
		var order []string
		for _, d := range plan {
			for _, m := range d.Meals {
				if seen[m.Name] == nil {
					seen[m.Name] = map[string]bool{}
					order = append(order, m.Name)
				}
				if t := strings.TrimSpace(m.Items); t != "" && len(seen[m.Name]) < 10 {
					seen[m.Name][truncateRunes(t, 90)] = true
				}
			}
		}
		fmt.Fprintf(&b, "Plan anterior %d:\n", i+1)
		for _, name := range order {
			var items []string
			for it := range seen[name] {
				items = append(items, it)
			}
			sort.Strings(items)
			fmt.Fprintf(&b, "- %s: %s\n", name, strings.Join(items, " | "))
		}
	}
	return b.String()
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

var mealStop = map[string]bool{"taza": true, "tazas": true, "pieza": true, "piezas": true, "cucharada": true, "cucharadas": true, "cucharadita": true, "cucharaditas": true,
	"rebanada": true, "rebanadas": true, "porcion": true, "porciones": true, "gramos": true, "con": true, "para": true, "sin": true, "una": true, "unas": true, "unos": true, "del": true, "las": true, "los": true, "mediana": true, "mediano": true, "picada": true, "picado": true, "cocida": true, "cocido": true, "natural": true, "pequena": true, "grande": true}

// mealWords is the bag of food words of a meal, without amounts, so "2 tazas de avena" and "1 taza de avena" match.
func mealWords(items string) map[string]bool {
	out := map[string]bool{}
	for _, w := range regexp.MustCompile(`[a-zñ]+`).FindAllString(foldText(items), -1) {
		if len(w) > 3 && !mealStop[w] {
			out[w] = true
		}
	}
	return out
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for w := range a {
		if b[w] {
			inter++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}

// repeatedShare is the fraction of the new week's meals that closely copy a meal of the same slot in an earlier plan.
func repeatedShare(days []nutritionDayIn, history [][]nutritionDayIn) float64 {
	prev := map[string][]map[string]bool{}
	for _, plan := range history {
		for _, d := range plan {
			for _, m := range d.Meals {
				prev[m.Name] = append(prev[m.Name], mealWords(m.Items))
			}
		}
	}
	total, same := 0, 0
	for _, d := range days {
		for _, m := range d.Meals {
			words := mealWords(m.Items)
			total++
			for _, o := range prev[m.Name] {
				if jaccard(words, o) >= 0.6 {
					same++
					break
				}
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(same) / float64(total)
}
