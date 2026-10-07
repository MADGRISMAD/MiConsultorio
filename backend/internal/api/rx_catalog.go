package api

import "strings"

// Reference catalogs for the receta builder. They only help to fill the form faster and to run safety
// checks (allergies, maximum daily dose); they never replace the prescriber's judgement.

const catalogDisclaimer = "Catálogo de referencia: no sustituye el criterio clínico ni la información del producto. Verifica dosis, presentaciones y contraindicaciones antes de recetar."
const icdDisclaimer = "Catálogo CIE-10 parcial con los diagnósticos más frecuentes en consulta; si no encuentras el tuyo, escribe el diagnóstico libremente."

type catConc struct {
	Label   string  `json:"label"`
	MgPerMl float64 `json:"mg_per_ml"`
}

type catMed struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"` // denominación genérica
	Brand          string    `json:"brand,omitempty"`
	Subject        string    `json:"subject"` // person | animal
	Species        []string  `json:"species,omitempty"`
	Category       string    `json:"category"`
	Control        string    `json:"control"` // No | Antibiótico | Fracción III | Fracción I o II
	Route          string    `json:"route"`
	Presentations  []string  `json:"presentations"`
	TypicalDose    string    `json:"typical_dose"`
	MgPerKg        float64   `json:"mg_per_kg,omitempty"`         // single dose
	MaxMgPerKgDay  float64   `json:"max_mg_per_kg_day,omitempty"` // total per day
	Concentrations []catConc `json:"concentrations,omitempty"`
	Notes          string    `json:"notes,omitempty"`
	Source         string    `json:"source"` // catalog | clinic
}

var accents = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n", "à", "a", "è", "e", "ì", "i", "ò", "o", "ù", "u")

// normText lowercases and removes accents so "Amoxicilína" matches "amoxicilina".
func normText(s string) string {
	return strings.Join(strings.Fields(accents.Replace(strings.ToLower(s))), " ")
}

func slug(s string) string {
	var b strings.Builder
	for _, r := range normText(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// med builds a human catalog entry; chain kg/conc/note for weight-based data.
func med(name, category, control, route, dose string, pres ...string) catMed {
	return catMed{Name: name, Subject: "person", Category: category, Control: control, Route: route, TypicalDose: dose, Presentations: pres, Source: "catalog"}
}

func (m catMed) kg(dose, maxDay float64) catMed { m.MgPerKg, m.MaxMgPerKgDay = dose, maxDay; return m }
func (m catMed) conc(label string, mgPerMl float64) catMed {
	m.Concentrations = append(m.Concentrations, catConc{label, mgPerMl})
	return m
}
func (m catMed) note(s string) catMed { m.Notes = s; return m }

// vet builds a veterinary entry for the given species (Perro, Gato, Conejo).
func vet(species, name, category, control, route, dose string, pres ...string) catMed {
	m := med(name, category, control, route, dose, pres...)
	m.Subject, m.Species = "animal", strings.Split(species, ",")
	return m
}

var catalogMeds = buildCatalog()

func buildCatalog() []catMed {
	all := append(append([]catMed{}, humanMeds()...), vetMeds()...)
	seen := map[string]bool{}
	for i := range all {
		id := slug(all[i].Name)
		if all[i].Subject == "animal" {
			id = "v-" + strings.ToLower(strings.Join(all[i].Species, "")) + "-" + id
		} else {
			id = "h-" + id
		}
		for seen[id] {
			id += "-2"
		}
		seen[id] = true
		all[i].ID = id
		if all[i].Presentations == nil {
			all[i].Presentations = []string{}
		}
	}
	return all
}

func catalogByID(id string) (catMed, bool) {
	for _, m := range catalogMeds {
		if m.ID == id {
			return m, true
		}
	}
	return catMed{}, false
}

// matchScore ranks how well a normalized haystack matches the normalized query (0 = no match).
func matchScore(name, hay, q string) int {
	switch {
	case q == "":
		return 1
	case name == q:
		return 100
	case strings.HasPrefix(name, q):
		return 80
	case strings.Contains(" "+name, " "+q):
		return 60
	case strings.Contains(hay, q):
		return 30
	}
	return 0
}

// doseVolume returns the mL of a liquid that hold the dose: weight (kg) x mg/kg / concentration (mg/mL).
// It returns 0 when any input is not positive.
func doseVolume(weightKg, mgPerKg, mgPerMl float64) float64 {
	if weightKg <= 0 || mgPerKg <= 0 || mgPerMl <= 0 {
		return 0
	}
	return weightKg * mgPerKg / mgPerMl
}
