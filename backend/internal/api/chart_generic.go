package api

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// The versioned charts that need no special geometry: the shape is checked generically (an object, bounded depth,
// array and text sizes, known top-level keys) and the ones that compute something (scales) are scored here, never by
// the browser.

var genericChartKeys = map[string][]string{
	"scale":          {"scale", "answers", "score", "severity", "label", "flag"},
	"therapy_plan":   {"goals", "tasks", "notes", "next_session"},
	"certificate":    {"type", "text", "diagnosis", "days", "from", "to", "destination", "species_note", "folio_note", "restrictions"},
	"prenatal":       {"fum", "fpp", "gestas", "risk", "blood_type", "notes", "studies"},
	"periodontogram": {"teeth", "notes", "plaque_pct", "bleeding_pct"},
	"ortho_visit":    {"date", "upper_arch", "lower_arch", "elastics", "appliance", "adjustments", "hygiene", "notes", "next_visit"},
	"milestones":     {"done", "notes"},
	"food_recall":    {"date", "kind", "meals", "water", "notes"},
	"exercises":      {"items", "notes", "frequency"},
	"problems":       {"problems", "medications", "notes"},
}

func isGenericChartKind(k string) bool { _, ok := genericChartKeys[k]; return ok }

// checkShape walks the decoded JSON: depth, number of items and length of every text are bounded.
func checkShape(v any, depth int) string {
	if depth > 5 {
		return "El contenido tiene demasiados niveles."
	}
	switch x := v.(type) {
	case map[string]any:
		if len(x) > 80 {
			return "El contenido tiene demasiados campos."
		}
		for k, e := range x {
			if utf8.RuneCountInString(k) > 40 {
				return "Un nombre de campo es demasiado largo."
			}
			if msg := checkShape(e, depth+1); msg != "" {
				return msg
			}
		}
	case []any:
		if len(x) > 200 {
			return "Una lista tiene demasiados elementos."
		}
		for _, e := range x {
			if msg := checkShape(e, depth+1); msg != "" {
				return msg
			}
		}
	case string:
		if utf8.RuneCountInString(x) > 3000 {
			return "Un texto es demasiado largo (máximo 3000 caracteres)."
		}
	}
	return ""
}

// validateGenericChart returns the (possibly completed) data to store.
func validateGenericChart(kind string, raw json.RawMessage) (string, bool, json.RawMessage) {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return "El contenido no es válido.", false, nil
	}
	allowed := map[string]bool{}
	for _, k := range genericChartKeys[kind] {
		allowed[k] = true
	}
	for k := range m {
		if !allowed[k] {
			return fmt.Sprintf("El campo «%s» no corresponde a este documento.", k), false, nil
		}
	}
	if msg := checkShape(m, 0); msg != "" {
		return msg, false, nil
	}
	if kind == "scale" {
		return scoreScale(m)
	}
	out, _ := json.Marshal(m)
	return "", true, out
}

// ---- validated scales (public-domain questionnaires) ----

type scaleBand struct {
	max   int
	label string
}

type scaleDef struct {
	items    int
	maxItem  int
	reversed map[int]bool // 0-based items scored backwards
	bands    []scaleBand
}

var scaleDefs = map[string]scaleDef{
	// Patient Health Questionnaire (depression), 0-27
	"phq9": {9, 3, nil, []scaleBand{{4, "Mínima"}, {9, "Leve"}, {14, "Moderada"}, {19, "Moderadamente grave"}, {27, "Grave"}}},
	// Generalized Anxiety Disorder, 0-21
	"gad7": {7, 3, nil, []scaleBand{{4, "Mínima"}, {9, "Leve"}, {14, "Moderada"}, {21, "Grave"}}},
	// Perceived Stress Scale, 0-40 (items 4, 5, 7 and 8 are reversed)
	"pss10": {10, 4, map[int]bool{3: true, 4: true, 6: true, 7: true}, []scaleBand{{13, "Estrés bajo"}, {26, "Estrés moderado"}, {40, "Estrés alto"}}},
}

func scoreScale(m map[string]any) (string, bool, json.RawMessage) {
	name, _ := m["scale"].(string)
	def, ok := scaleDefs[name]
	if !ok {
		return "La escala no existe.", false, nil
	}
	arr, _ := m["answers"].([]any)
	if len(arr) != def.items {
		return fmt.Sprintf("Responde las %d preguntas de la escala.", def.items), false, nil
	}
	score := 0
	answers := make([]int, len(arr))
	for i, a := range arr {
		f, ok := a.(float64)
		if !ok || f != float64(int(f)) || f < 0 || int(f) > def.maxItem {
			return "Una respuesta no es válida.", false, nil
		}
		answers[i] = int(f)
		if def.reversed[i] {
			score += def.maxItem - int(f)
		} else {
			score += int(f)
		}
	}
	label := ""
	for _, b := range def.bands {
		if score <= b.max {
			label = b.label
			break
		}
	}
	out := map[string]any{"scale": name, "answers": answers, "score": score, "severity": label, "label": label}
	// the 9th PHQ-9 item asks about thoughts of self-harm: any answer above zero is flagged for the professional
	if name == "phq9" && answers[8] > 0 {
		out["flag"] = "self_harm"
	}
	b, _ := json.Marshal(out)
	return "", true, b
}
