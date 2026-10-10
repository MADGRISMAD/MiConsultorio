package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

// Regenerating one piece of a nutrition menu with the AI: a meal of one day, or a whole day. The model sees the complete
// week so the new piece does not copy a dish served elsewhere, and keeps out the foods the patient dislikes (including the
// one just remembered during the review). Nothing else of the plan changes, and nothing is saved here: the nutritionist
// reviews the result in the editor.

var nutritionFragmentSchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"meals": map[string]any{"type": "ARRAY", "items": map[string]any{
			"type":       "OBJECT",
			"properties": map[string]any{"name": map[string]any{"type": "STRING"}, "items": map[string]any{"type": "STRING"}},
			"required":   []string{"name", "items"},
		}},
	},
	"required": []string{"meals"},
}

type nutritionFragmentReq struct {
	Days     []nutritionDayIn `json:"days"`     // the whole menu as it is now
	Day      int              `json:"day"`      // index of the day to change
	Meal     *int             `json:"meal"`     // index of the meal; omitted = the whole day
	Dislike  string           `json:"dislike"`  // what the patient just said they do not like
	Request  string           `json:"request"`  // anything else the nutritionist wants ("más ligero", "con avena")
	Dislikes string           `json:"dislikes"` // the patient's dislikes already in the plan
}

func (s *Server) nutritionFragmentAI(w http.ResponseWriter, r *http.Request) {
	id, subject, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	if !s.requireCedula(w, r, "generar planes de alimentación") {
		return
	}
	var in nutritionFragmentReq
	if !decode(w, r, &in) {
		return
	}
	in.Dislike, in.Request, in.Dislikes = strings.TrimSpace(in.Dislike), strings.TrimSpace(in.Request), strings.TrimSpace(in.Dislikes)
	switch {
	case subject != "person":
		writeError(w, http.StatusBadRequest, "El plan nutricional es para personas.")
		return
	case len(in.Days) == 0 || len(in.Days) > 7 || in.Day < 0 || in.Day >= len(in.Days):
		writeError(w, http.StatusBadRequest, "Elige el día que quieres cambiar.")
		return
	case in.Meal != nil && (*in.Meal < 0 || *in.Meal >= len(in.Days[in.Day].Meals)):
		writeError(w, http.StatusBadRequest, "Elige la comida que quieres cambiar.")
		return
	case utf8.RuneCountInString(in.Dislike) > 300 || utf8.RuneCountInString(in.Request) > 300 || utf8.RuneCountInString(in.Dislikes) > 1000:
		writeError(w, http.StatusBadRequest, "El texto es demasiado largo.")
		return
	}
	for _, d := range in.Days {
		if len(d.Meals) == 0 || len(d.Meals) > 8 {
			writeError(w, http.StatusBadRequest, "El menú no es válido.")
			return
		}
		for _, m := range d.Meals {
			if utf8.RuneCountInString(m.Items) > 1200 || utf8.RuneCountInString(m.Name) > 60 {
				writeError(w, http.StatusBadRequest, "El menú no es válido.")
				return
			}
		}
	}
	if s.cfg.GeminiAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "La IA no está configurada en el servidor (GEMINI_API_KEY)."})
		return
	}

	p := principalFrom(r.Context())
	var raw []byte
	if err := s.db.QueryRow(r.Context(), `SELECT profile FROM patients WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id).Scan(&raw); err != nil {
		serverError(w, r, err)
		return
	}
	prof, err := decProfile(id, raw)
	if err != nil {
		serverError(w, r, err)
		return
	}
	foodAllergies := profileText(prof, "food_allergies")
	banned := foodTerms(in.Dislike, in.Dislikes, foodAllergies)

	day := in.Days[in.Day]
	first, last := 0, len(day.Meals)
	if in.Meal != nil {
		first, last = *in.Meal, *in.Meal+1
	}
	target := day.Meals[first:last]

	// the rest of the week, to keep the new piece different from it
	var rest []string
	var others []map[string]bool
	for di, d := range in.Days {
		for mi, m := range d.Meals {
			if di == in.Day && mi >= first && mi < last {
				continue
			}
			if t := strings.TrimSpace(m.Items); t != "" {
				rest = append(rest, fmt.Sprintf("- %s · %s: %s", d.Name, m.Name, truncateRunes(t, 140)))
				others = append(others, mealWords(t))
			}
		}
	}
	var want []string
	for _, m := range target {
		want = append(want, fmt.Sprintf("- %s (aprox. %d kcal). Actual: %s", m.Name, m.Kcal, truncateRunes(strings.TrimSpace(m.Items), 200)))
	}

	if err := s.spendMagic(r.Context(), p); err != nil {
		writeFailure(w, r, err)
		return
	}
	scope := "UNA comida"
	if in.Meal == nil {
		scope = "un DÍA completo"
	}
	basePrompt := "Eres un asistente para nutriólogos en México. Estás revisando con el paciente su menú semanal y hay que cambiar SOLO " + scope + " del " + day.Name + ". " +
		"Reescribe únicamente esto (mismos nombres y orden, con porciones concretas y alimentos comunes en México, y con las calorías indicadas):\n" + strings.Join(want, "\n") + "\n\n" +
		"Responde SOLO con JSON con las " + fmt.Sprint(len(target)) + " comida(s)."
	if in.Dislike != "" {
		basePrompt += "\n\nNO LE GUSTA AL PACIENTE (no lo uses; busca algo equivalente en valor nutricional que sí le guste): " + in.Dislike
	}
	if in.Dislikes != "" {
		basePrompt += "\nOtros alimentos que no le gustan: " + in.Dislikes
	}
	if foodAllergies != "" {
		basePrompt += "\nALERGIAS O INTOLERANCIAS ALIMENTARIAS (nunca las incluyas): " + foodAllergies
	}
	if in.Request != "" {
		basePrompt += "\nIndicación del nutriólogo: " + in.Request
	}
	if len(rest) > 0 {
		basePrompt += "\n\nEL RESTO DEL MENÚ DE LA SEMANA (no lo cambies; el nuevo platillo NO debe repetir ni parecerse a ninguno de estos, para que la semana siga variada):\n" + strings.Join(rest, "\n")
	}

	prompt := basePrompt
	var out []nutritionMealIn
	const attempts = 3
	for attempt := 0; attempt < attempts; attempt++ {
		rawOut, err := s.gemini(r.Context(), prompt, nil, "", nutritionFragmentSchema)
		if err != nil {
			s.refundMagic(r.Context(), p)
			logf(r, "nutrition fragment ai: %v", err)
			writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA no pudo cambiar esa parte. Intenta de nuevo en un momento."})
			return
		}
		var got struct {
			Meals []struct {
				Name  string `json:"name"`
				Items string `json:"items"`
			} `json:"meals"`
		}
		if json.Unmarshal(rawOut, &got) != nil || len(got.Meals) < len(target) {
			if attempt == attempts-1 {
				s.refundMagic(r.Context(), p)
				writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA devolvió una respuesta incompleta. Intenta de nuevo."})
				return
			}
			prompt = basePrompt + fmt.Sprintf("\n\nIMPORTANTE: devuelve exactamente %d comida(s).", len(target))
			continue
		}
		out = out[:0]
		for i, m := range target {
			out = append(out, nutritionMealIn{Name: m.Name, Time: m.Time, Items: strings.TrimSpace(got.Meals[i].Items), Kcal: m.Kcal})
		}
		// the piece must keep out what is forbidden and must not copy another meal of the week
		var problems []string
		if hits := forbiddenIn([]nutritionDayIn{{Name: day.Name, Meals: out}}, banned); len(hits) > 0 {
			problems = append(problems, "aparecieron alimentos prohibidos ("+strings.Join(hits[:min(len(hits), 5)], "; ")+")")
		}
		for _, m := range out {
			words := mealWords(m.Items)
			if strings.TrimSpace(m.Items) == "" {
				problems = append(problems, "una comida quedó vacía")
				break
			}
			for _, o := range others {
				if jaccard(words, o) >= 0.5 {
					problems = append(problems, "se parece demasiado a otra comida del menú ("+truncateRunes(m.Items, 60)+")")
					break
				}
			}
		}
		if len(problems) == 0 {
			break
		}
		if attempt == attempts-1 {
			// out of attempts: hand it over anyway, and tell the nutritionist what to check
			writeJSON(w, http.StatusOK, map[string]any{"meals": out, "warnings": problems})
			return
		}
		prompt = basePrompt + "\n\nIMPORTANTE: en el intento anterior " + strings.Join(problems, " y ") + ". Cámbialo por otro platillo distinto."
	}
	writeJSON(w, http.StatusOK, map[string]any{"meals": out})
}
