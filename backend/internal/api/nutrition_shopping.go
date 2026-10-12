package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"
)

// The shopping list of a nutrition menu: the model adds up the foods of the week into what to buy, by section of the
// supermarket. Nothing is saved: the nutritionist or the patient prints it. It costs one magic use, returned on failure.

var shoppingSchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"sections": map[string]any{"type": "ARRAY", "items": map[string]any{
			"type": "OBJECT",
			"properties": map[string]any{
				"name": map[string]any{"type": "STRING"},
				"items": map[string]any{"type": "ARRAY", "items": map[string]any{
					"type":       "OBJECT",
					"properties": map[string]any{"name": map[string]any{"type": "STRING"}, "quantity": map[string]any{"type": "STRING"}},
					"required":   []string{"name", "quantity"},
				}},
			},
			"required": []string{"name", "items"},
		}},
	},
	"required": []string{"sections"},
}

type shoppingReq struct {
	Days    []nutritionDayIn `json:"days"`
	Persons int              `json:"persons"` // how many people eat from this menu (the patient alone by default)
}

type shoppingItem struct {
	Name     string `json:"name"`
	Quantity string `json:"quantity"`
}
type shoppingSection struct {
	Name  string         `json:"name"`
	Items []shoppingItem `json:"items"`
}

func (s *Server) nutritionShopping(w http.ResponseWriter, r *http.Request) {
	id, subject, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	if !s.requireCedula(w, r, "generar planes de alimentación") {
		return
	}
	var in shoppingReq
	if !decode(w, r, &in) {
		return
	}
	if in.Persons < 1 {
		in.Persons = 1
	}
	if subject != "person" || in.Persons > 12 {
		writeError(w, http.StatusBadRequest, "La lista del súper es para personas (hasta 12 comensales).")
		return
	}
	var menu []string
	for _, d := range in.Days {
		if len(d.Meals) > 8 || utf8.RuneCountInString(d.Name) > 40 {
			writeError(w, http.StatusBadRequest, "El menú no es válido.")
			return
		}
		for _, m := range d.Meals {
			if utf8.RuneCountInString(m.Items) > 1200 || utf8.RuneCountInString(m.Name) > 60 {
				writeError(w, http.StatusBadRequest, "El menú no es válido.")
				return
			}
			if t := strings.TrimSpace(m.Items); t != "" {
				menu = append(menu, d.Name+" · "+m.Name+": "+t)
			}
		}
	}
	if len(in.Days) == 0 || len(in.Days) > 7 || len(menu) == 0 {
		writeError(w, http.StatusBadRequest, "Primero arma el menú: la lista se hace con lo que escribas en las comidas.")
		return
	}
	if s.cfg.GeminiAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "La IA no está configurada en el servidor (GEMINI_API_KEY)."})
		return
	}
	p := principalFrom(r.Context())
	if err := s.spendMagic(r.Context(), p); err != nil {
		writeFailure(w, r, err)
		return
	}
	prompt := "Eres un asistente de nutrición en México. Con el siguiente menú, arma la LISTA DEL SÚPER para " + itoa(len(in.Days)) +
		" día(s) y " + itoa(in.Persons) + " persona(s): suma las cantidades de cada alimento (convierte tazas, piezas y gramos a algo que se compre: kg, piezas, paquetes, litros), " +
		"agrúpalo por sección del supermercado (Frutas y verduras, Carnes y pescado, Huevo y lácteos, Cereales y panadería, Leguminosas, Grasas y despensa, Otros) y no incluyas secciones vacías. " +
		"Usa nombres comunes en México y cantidades aproximadas y realistas. Responde SOLO con JSON.\n\nMENÚ:\n" + strings.Join(menu, "\n")
	raw, err := s.gemini(r.Context(), prompt, nil, "", shoppingSchema)
	if err != nil {
		s.refundMagic(r.Context(), p)
		logf(r, "nutrition shopping ai: %v", err)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA no pudo armar la lista. Intenta de nuevo en un momento."})
		return
	}
	var out struct {
		Sections []shoppingSection `json:"sections"`
	}
	list := []shoppingSection{}
	if json.Unmarshal(raw, &out) == nil {
		for _, sec := range out.Sections {
			sec.Name = truncateRunes(strings.TrimSpace(sec.Name), 60)
			var items []shoppingItem
			for _, it := range sec.Items {
				it.Name, it.Quantity = truncateRunes(strings.TrimSpace(it.Name), 80), truncateRunes(strings.TrimSpace(it.Quantity), 40)
				if it.Name != "" && len(items) < 60 {
					items = append(items, it)
				}
			}
			if sec.Name != "" && len(items) > 0 && len(list) < 12 {
				sec.Items = items
				list = append(list, sec)
			}
		}
	}
	if len(list) == 0 {
		s.refundMagic(r.Context(), p)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA devolvió una lista vacía. Intenta de nuevo."})
		return
	}
	_ = id
	writeJSON(w, http.StatusOK, map[string]any{"sections": list})
}
