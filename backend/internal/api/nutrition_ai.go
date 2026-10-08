package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

// AI draft of a nutrition plan. Gemini proposes; the nutritionist reviews and edits it before saving.
// Only what the plan needs leaves the server: age, sex, body measures and the food-related history. Never the name or contact data.

var nutritionPlanSchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"goal":        map[string]any{"type": "STRING"},
		"kcal":        map[string]any{"type": "NUMBER"},
		"protein_pct": map[string]any{"type": "NUMBER"}, "carb_pct": map[string]any{"type": "NUMBER"}, "fat_pct": map[string]any{"type": "NUMBER"},
		"water_liters": map[string]any{"type": "NUMBER"},
		"meals": map[string]any{"type": "ARRAY", "items": map[string]any{
			"type": "OBJECT",
			"properties": map[string]any{
				"name": map[string]any{"type": "STRING"}, "time": map[string]any{"type": "STRING"},
				"items": map[string]any{"type": "STRING"}, "kcal": map[string]any{"type": "NUMBER"},
			},
			"required": []string{"name", "items", "kcal"},
		}},
		"recommendations": map[string]any{"type": "STRING"},
		"avoid":           map[string]any{"type": "STRING"},
		"supplements":     map[string]any{"type": "STRING"},
		"follow_up_days":  map[string]any{"type": "NUMBER"},
	},
	"required": []string{"goal", "kcal", "protein_pct", "carb_pct", "fat_pct", "meals", "recommendations"},
}

type nutritionAIReq struct {
	Goal        string  `json:"goal"`
	WeightKg    float64 `json:"weight_kg"`
	HeightCm    float64 `json:"height_cm"`
	Activity    string  `json:"activity"`
	Kcal        int     `json:"kcal"`
	Meals       int     `json:"meals"`
	Preferences string  `json:"preferences"`
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

func (s *Server) nutritionPlanAI(w http.ResponseWriter, r *http.Request) {
	id, subject, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	var in nutritionAIReq
	if !decode(w, r, &in) {
		return
	}
	in.Goal, in.Activity, in.Preferences = strings.TrimSpace(in.Goal), strings.TrimSpace(in.Activity), strings.TrimSpace(in.Preferences)
	switch {
	case subject != "person":
		writeError(w, http.StatusBadRequest, "El plan nutricional es para personas.")
		return
	case in.Goal == "" || utf8.RuneCountInString(in.Goal) > 200:
		writeError(w, http.StatusBadRequest, "Escribe el objetivo del plan (máximo 200 caracteres).")
		return
	case utf8.RuneCountInString(in.Preferences) > 1500 || utf8.RuneCountInString(in.Activity) > 100:
		writeError(w, http.StatusBadRequest, "El texto es demasiado largo.")
		return
	case in.WeightKg < 0 || in.WeightKg > 400 || in.HeightCm < 0 || in.HeightCm > 260 || in.Kcal < 0 || in.Kcal > 8000:
		writeError(w, http.StatusBadRequest, "Revisa el peso, la talla y las calorías.")
		return
	}
	if in.Meals < 3 || in.Meals > 7 {
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
	add("Alergias (medicamentos y otras)", profileText(prof, "allergies_text"))
	add("Alergias o intolerancias alimentarias", profileText(prof, "food_allergies"))
	add("Enfermedades crónicas", strings.Join(nonEmpty(profileText(prof, "chronic_conditions"), profileText(prof, "chronic_other")), "; "))
	add("Medicamentos actuales", profileText(prof, "current_medication"))
	add("Cómo come en un día normal", profileText(prof, "diet_pattern"))
	add("Comidas al día actuales", profileText(prof, "meals_per_day"))
	add("Suplementos actuales", profileText(prof, "supplements"))

	prompt := "Eres un asistente para nutriólogos en México. Redacta un BORRADOR de plan nutricional de un día tipo que el nutriólogo revisará y corregirá. " +
		"Usa alimentos comunes y accesibles en México, porciones concretas (tazas, piezas, gramos) y lenguaje claro para el paciente. Responde SOLO con JSON. " +
		fmt.Sprintf("Objetivo del plan: %s. Reparte el día en %d comidas, con hora (formato 24 h HH:MM) y calorías que sumen aproximadamente el total diario. ", in.Goal, in.Meals)
	if in.Kcal > 0 {
		prompt += fmt.Sprintf("Usa %d kcal al día. ", in.Kcal)
	} else {
		prompt += "Calcula las calorías diarias adecuadas al objetivo y al paciente. "
	}
	prompt += "Los porcentajes de proteínas, carbohidratos y grasas deben sumar 100. " +
		"NUNCA incluyas alimentos a los que el paciente sea alérgico o intolerante, y respeta sus enfermedades y su medicación. " +
		"No propongas déficits extremos (no menos de 1200 kcal en mujeres ni 1500 en hombres) ni suplementos con dosis; si dudas, deja suplementos vacío. " +
		"En recomendaciones da 4 a 6 consejos breves, en 'avoid' lo que conviene limitar, y sugiere en follow_up_days cuándo darle seguimiento.\n\n" +
		"DATOS DEL PACIENTE (sin nombre):\n" + strings.Join(ctx, "\n")
	if in.Preferences != "" {
		prompt += "\n\nPREFERENCIAS Y RESTRICCIONES QUE INDICA EL NUTRIÓLOGO:\n" + in.Preferences
	}

	rawPlan, err := s.gemini(r.Context(), prompt, nil, "", nutritionPlanSchema)
	if err != nil {
		s.refundMagic(r.Context(), p)
		logf(r, "nutrition ai: %v", err)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA no pudo armar el plan. Intenta de nuevo en un momento."})
		return
	}
	// the model may answer 1800.0 or 1800.5 for a whole number, so read numbers as floats first
	var f struct {
		Goal            string  `json:"goal"`
		Kcal            float64 `json:"kcal"`
		ProteinPct      float64 `json:"protein_pct"`
		CarbPct         float64 `json:"carb_pct"`
		FatPct          float64 `json:"fat_pct"`
		WaterLiters     float64 `json:"water_liters"`
		Recommendations string  `json:"recommendations"`
		Avoid           string  `json:"avoid"`
		Supplements     string  `json:"supplements"`
		FollowUpDays    float64 `json:"follow_up_days"`
		Meals           []struct {
			Name  string  `json:"name"`
			Time  string  `json:"time"`
			Items string  `json:"items"`
			Kcal  float64 `json:"kcal"`
		} `json:"meals"`
	}
	if err := json.Unmarshal(rawPlan, &f); err != nil {
		s.refundMagic(r.Context(), p)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA devolvió un plan que no se pudo leer. Intenta de nuevo."})
		return
	}
	out := nutritionPlanIn{Goal: f.Goal, Kcal: int(f.Kcal + 0.5), ProteinPct: int(f.ProteinPct + 0.5), CarbPct: int(f.CarbPct + 0.5), FatPct: int(f.FatPct + 0.5),
		WaterLiters: f.WaterLiters, Recommendations: f.Recommendations, Avoid: f.Avoid, Supplements: f.Supplements, FollowUpDays: int(f.FollowUpDays + 0.5)}
	for _, m := range f.Meals {
		out.Meals = append(out.Meals, nutritionMealIn{Name: m.Name, Time: m.Time, Items: m.Items, Kcal: int(m.Kcal + 0.5)})
	}
	cleanNutritionPlan(&out)
	if _, ok := validateNutritionPlanStruct(out); !ok {
		s.refundMagic(r.Context(), p)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA devolvió un plan fuera de rango. Intenta de nuevo."})
		return
	}
	out.Basis = "Borrador generado con IA a partir de los datos del expediente. Revísalo y ajústalo con tu criterio clínico."
	writeJSON(w, http.StatusOK, map[string]any{"plan": out})
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

// cleanNutritionPlan trims and clamps what the model returned so it always passes validation.
func cleanNutritionPlan(p *nutritionPlanIn) {
	cut := func(v string, n int) string {
		v = strings.TrimSpace(v)
		if utf8.RuneCountInString(v) > n {
			v = string([]rune(v)[:n])
		}
		return v
	}
	clamp := func(v, lo, hi int) int { return max(lo, min(hi, v)) }
	p.Goal, p.Basis = cut(p.Goal, 300), ""
	p.Recommendations, p.Avoid, p.Supplements = cut(p.Recommendations, 3000), cut(p.Avoid, 1500), cut(p.Supplements, 600)
	p.Kcal = clamp(p.Kcal, 0, 10000)
	p.ProteinPct, p.CarbPct, p.FatPct = clamp(p.ProteinPct, 0, 100), clamp(p.CarbPct, 0, 100), clamp(p.FatPct, 0, 100)
	if sum := p.ProteinPct + p.CarbPct + p.FatPct; sum > 100 {
		p.ProteinPct, p.CarbPct, p.FatPct = p.ProteinPct*100/sum, p.CarbPct*100/sum, p.FatPct*100/sum
	}
	if p.WaterLiters < 0 || p.WaterLiters > 10 {
		p.WaterLiters = 2
	}
	p.FollowUpDays = clamp(p.FollowUpDays, 0, 365)
	meals := make([]nutritionMealIn, 0, len(p.Meals))
	for _, m := range p.Meals {
		m.Name, m.Time, m.Items = cut(m.Name, 60), cut(m.Time, 20), cut(m.Items, 1200)
		m.Kcal = clamp(m.Kcal, 0, 10000)
		if m.Name != "" && len(meals) < 12 {
			meals = append(meals, m)
		}
	}
	p.Meals = meals
}

func validateNutritionPlanStruct(p nutritionPlanIn) (string, bool) {
	raw, _ := json.Marshal(p)
	return validateNutritionPlan(raw)
}
