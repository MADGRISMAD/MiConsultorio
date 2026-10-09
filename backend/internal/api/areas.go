package api

import (
	"context"
	"slices"
	"strings"
)

// Areas. A professional can be tied to some of the clinic's giros (nutrition, psychology...). Without areas they
// work in all of them, as every account did before. Areas decide whose agenda a patient may pick on the public page
// and which specialty records someone works with.

// areaLabels are the names of the people of each giro (what the clinic's team and the public page show).
var areaLabels = map[string]string{
	"GENERAL_MEDICAL": "Medicina general", "DENTAL": "Odontología", "PEDIATRICS": "Pediatría", "INTERNAL_MEDICINE": "Medicina interna",
	"PHYSIOTHERAPY": "Fisioterapia", "NUTRITION": "Nutrición", "PSYCHOLOGY": "Psicología", "DERMATOLOGY": "Dermatología",
	"GYNECOLOGY": "Ginecología", "ORTHOPEDICS": "Ortopedia", "VETERINARY": "Veterinaria", "CHIROPRACTIC": "Quiropráctica",
}

// cleanAreas keeps only the giros the clinic works with, without repeats. A message means the list is not valid.
func (s *Server) cleanAreas(ctx context.Context, clinicID string, in []string) ([]string, string) {
	out := []string{}
	if len(in) == 0 {
		return out, ""
	}
	kinds, err := s.clinicKindsFor(ctx, clinicID)
	if err != nil {
		return nil, "No se pudo validar las áreas."
	}
	for _, a := range in {
		if !slices.Contains(kinds, a) {
			return nil, "Una de las áreas no es de este consultorio."
		}
		if !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out, ""
}

// worksIn is true for an administrator, for someone without areas, and for someone who has one of the giros.
func (p *Principal) worksIn(giros ...string) bool {
	if p.Role == RoleAdmin || len(p.Areas) == 0 || len(giros) == 0 {
		return true
	}
	for _, g := range giros {
		if slices.Contains(p.Areas, g) {
			return true
		}
	}
	return false
}

// chartGiros are the giros a kind of specialty record belongs to. A kind not listed is common to everyone.
var chartGiros = map[string][]string{
	"odontogram": {"DENTAL"}, "periodontogram": {"DENTAL"}, "ortho_visit": {"DENTAL"},
	"nutrition_plan": {"NUTRITION"}, "food_recall": {"NUTRITION"},
	"scale": {"PSYCHOLOGY"}, "therapy_plan": {"PSYCHOLOGY"},
	"prenatal":   {"GYNECOLOGY"},
	"milestones": {"PEDIATRICS"},
	"exercises":  {"PHYSIOTHERAPY", "CHIROPRACTIC", "ORTHOPEDICS"},
}

// confidentialCharts are only for the professionals of the giro (psychotherapy notes are not shared with the rest of the team).
var confidentialCharts = []string{"scale", "therapy_plan"}

func areaNames(areas []string) string {
	names := make([]string, 0, len(areas))
	for _, a := range areas {
		names = append(names, areaLabels[a])
	}
	return strings.ToLower(strings.Join(names, ", "))
}

// rxArea is the giro a receta belongs to: the author's only area; otherwise the one they choose among theirs (or the
// clinic's, if they work in all); by default the clinic's main giro.
func rxArea(p *Principal, kinds []string, chosen string) (string, string) {
	pool := kinds
	if len(p.Areas) > 0 && p.Role != RoleAdmin {
		pool = slices.DeleteFunc(slices.Clone(p.Areas), func(a string) bool { return !slices.Contains(kinds, a) })
	}
	if chosen != "" {
		if !slices.Contains(pool, chosen) {
			return "", "Esa área no es una de las tuyas."
		}
		return chosen, ""
	}
	if len(pool) > 0 {
		return pool[0], ""
	}
	return "", ""
}

// chartsOfDroppedGiros are the kinds of record that belong only to giros the clinic no longer works in: they stay stored but are not shown.
func chartsOfDroppedGiros(active []string) []string {
	out := []string{}
	for kind, giros := range chartGiros {
		if !slices.ContainsFunc(giros, func(g string) bool { return slices.Contains(active, g) }) {
			out = append(out, kind)
		}
	}
	return out
}

// areaKeys are all the giros there are.
var areaKeys = func() []string {
	out := make([]string, 0, len(areaLabels))
	for k := range areaLabels {
		out = append(out, k)
	}
	return out
}()

// myKinds are the giros whose forms this person fills: their areas among the clinic's, or all of the clinic's.
func (p *Principal) myKinds(clinicKinds []string) []string {
	if p.Role == RoleAdmin || len(p.Areas) == 0 {
		return clinicKinds
	}
	mine := slices.DeleteFunc(slices.Clone(p.Areas), func(a string) bool { return !slices.Contains(clinicKinds, a) })
	if len(mine) == 0 {
		return clinicKinds
	}
	return mine
}

// scopeProfile leaves in the profile only the answers the fields ask for (the rest belongs to other giros).
func scopeProfile(profile map[string]any, fields []Field) map[string]any {
	keys := make(map[string]bool, len(fields))
	for _, f := range fields {
		keys[f.Key] = true
	}
	out := map[string]any{}
	for k, v := range profile {
		if keys[k] {
			out[k] = v
		}
	}
	return out
}
