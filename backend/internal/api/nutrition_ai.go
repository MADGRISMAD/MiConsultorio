package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
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

// carb, protein, fat shares by goal (the goals offered in the form; anything else is maintenance)
var nutritionMacros = map[string][3]int{
	"Bajar de peso": {45, 25, 30}, "Ganar masa muscular": {45, 30, 25}, "Control de enfermedad": {45, 20, 35},
}
var nutritionAdjust = map[string]int{"Bajar de peso": -500, "Subir de peso": 400, "Ganar masa muscular": 300}

// energyTarget is Mifflin-St Jeor times the activity factor, adjusted for the goal and never below a safe floor.
func energyTarget(sex string, age int, weightKg, heightCm, activity float64, goal string) (bmr, target int) {
	sexTerm := -78.0 // another / unspecified: halfway between the two
	switch sex {
	case "Hombre":
		sexTerm = 5
	case "Mujer":
		sexTerm = -161
	}
	b := 10*weightKg + 6.25*heightCm - 5*float64(age) + sexTerm
	if activity < 1.2 || activity > 2.0 {
		activity = 1.375
	}
	floor := 1200
	if sex == "Hombre" {
		floor = 1500
	}
	t := int(math.Round((b*activity+float64(nutritionAdjust[goal]))/10)) * 10
	return int(math.Round(b)), max(floor, t)
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
	if target == 0 {
		if in.WeightKg <= 0 || in.HeightCm <= 0 || age == nil {
			writeError(w, http.StatusBadRequest, "Para calcular las calorías escribe el peso y la talla (y registra la fecha de nacimiento del paciente), o indica las calorías tú.")
			return
		}
		var bmr int
		bmr, target = energyTarget(sex, *age, in.WeightKg, in.HeightCm, in.ActivityFactor, in.Goal)
		basis = fmt.Sprintf("Calorías calculadas con la fórmula de Mifflin-St Jeor: metabolismo basal %d kcal × actividad %.3g, ajustado al objetivo (%+d kcal).", bmr, math.Max(in.ActivityFactor, 1.2), nutritionAdjust[in.Goal])
	}
	macros, okMacros := nutritionMacros[in.Goal]
	if !okMacros {
		macros = [3]int{50, 20, 30}
	}
	slots := nutritionSlots[in.Meals]
	water := 2.0
	if in.WeightKg > 0 {
		water = math.Min(4, math.Max(1.5, math.Round(in.WeightKg*0.035*10)/10))
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

	var plan nutritionPlanIn
	var warnings []string
	prompt := basePrompt
	for attempt := 0; attempt < 2; attempt++ {
		rawPlan, err := s.gemini(r.Context(), prompt, nil, "", nutritionDaySchema)
		if err != nil {
			s.refundMagic(r.Context(), p)
			logf(r, "nutrition ai: %v", err)
			writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA no pudo armar el plan. Intenta de nuevo en un momento."})
			return
		}
		var out nutritionAIOut
		if json.Unmarshal(rawPlan, &out) != nil || len(out.Days) != 7 {
			if attempt == 1 {
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
			if attempt == 1 {
				s.refundMagic(r.Context(), p)
				writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA devolvió comidas incompletas. Intenta de nuevo."})
				return
			}
			prompt = basePrompt + fmt.Sprintf("\n\nIMPORTANTE: cada día debe traer las %d comidas indicadas.", in.Meals)
			continue
		}
		hits := forbiddenIn(plan.Days, banned)
		if len(hits) == 0 {
			warnings = nil
			break
		}
		warnings = hits
		prompt = basePrompt + "\n\nIMPORTANTE: en el intento anterior aparecieron alimentos prohibidos (" + strings.Join(hits[:min(len(hits), 6)], "; ") + "). Reemplázalos por otros y no los uses en ninguna comida."
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
