package api

import (
	"fmt"
	"regexp"
	"strings"
)

// Allergy checks. Allergy text is free text written by staff, so matching is lenient: accents and case are
// ignored, and a drug family named in the allergy ("penicilinas", "AINEs", "sulfas") matches every member.

type allergyFamily struct {
	Name     string
	Triggers []string // what a patient record may say (normalized)
	Members  []string // drug-name stems; "^x" = word prefix, others = substring
}

var allergyFamilies = []allergyFamily{
	{"Penicilinas", []string{"penicilin", "betalactam", "beta lactam", "amoxicilin", "ampicilin", "dicloxacilin", "cloxacilin", "oxacilin"},
		[]string{"penicilin", "amoxicilin", "ampicilin", "dicloxacilin", "cloxacilin", "oxacilin", "piperacilin"}},
	{"Cefalosporinas", []string{"cefalosporin", "betalactam", "beta lactam", "^cef"}, []string{"^cef", "cefalosporin"}},
	{"Macrólidos", []string{"macrolid", "azitromicin", "claritromicin", "eritromicin"}, []string{"macrolid", "azitromicin", "claritromicin", "eritromicin"}},
	{"Quinolonas", []string{"quinolon", "floxacin"}, []string{"floxacin", "quinolon"}},
	{"Tetraciclinas", []string{"tetraciclin", "doxiciclin", "minociclin"}, []string{"tetraciclin", "doxiciclin", "minociclin"}},
	{"Aminoglucósidos", []string{"aminoglucosid", "gentamicin", "amikacin", "tobramicin", "neomicin", "estreptomicin"}, []string{"aminoglucosid", "gentamicin", "amikacin", "tobramicin", "neomicin", "estreptomicin"}},
	{"Lincosamidas", []string{"lincosamid", "clindamicin", "lincomicin"}, []string{"clindamicin", "lincomicin"}},
	{"Sulfonamidas", []string{"=sulfa", "=sulfas", "^sulfonamid", "^sulfametox", "^sulfadiazin"}, []string{"^sulfonamid", "^sulfametox", "^sulfadiazin"}},
	{"Nitroimidazoles", []string{"nitroimidazol", "metronidazol", "tinidazol", "secnidazol"}, []string{"metronidazol", "tinidazol", "secnidazol"}},
	{"AINEs", []string{"=aine", "=aines", "^antiinflamat", "aspirin", "acido acetilsalicilico", "salicilat", "ibuprofen", "naproxen", "diclofenac", "ketorolac", "meloxicam", "piroxicam", "indometacin", "celecoxib", "etoricoxib", "nimesulid", "ketoprofen"},
		[]string{"ibuprofen", "naproxen", "diclofenac", "ketorolac", "meloxicam", "piroxicam", "indometacin", "celecoxib", "etoricoxib", "nimesulid", "ketoprofen", "acido acetilsalicilico", "aspirin", "firocoxib", "carprofen", "robenacoxib"}},
	{"Pirazolonas (metamizol)", []string{"metamizol", "dipirona", "pirazolon"}, []string{"metamizol", "dipirona"}},
	{"Paracetamol", []string{"paracetamol", "acetaminofen"}, []string{"paracetamol", "acetaminofen"}},
	{"Opioides", []string{"opioide", "tramadol", "codein", "morfin", "fentanil"}, []string{"tramadol", "codein", "morfin", "fentanil", "opioide"}},
	{"Anestésicos locales", []string{"anestesic", "anestesia local", "lidocain", "bupivacain", "articain", "mepivacain"}, []string{"lidocain", "bupivacain", "articain", "mepivacain"}},
	{"Yodo", []string{"yodo", "povidona", "iodo"}, []string{"povidona", "yodo"}},
	{"Benzodiacepinas", []string{"benzodiacepin", "diazepam", "clonazepam", "alprazolam", "lorazepam"}, []string{"diazepam", "clonazepam", "alprazolam", "lorazepam", "bromazepam"}},
}

var noAllergy = regexp.MustCompile(`^(ninguna|ningun|niega|negada|negadas|no|sin alergias|n/a|na|desconoce)\b`)
var allergySplit = regexp.MustCompile(`[,;\n/]+|\s+y\s+|\s+e\s+|\.\s+`)

// stemMatch reports whether text matches the stem: "^x" matches word prefixes, "=x" whole words,
// anything else is a substring.
func stemMatch(text, stem string) bool {
	if stem[0] == '^' || stem[0] == '=' {
		for _, w := range strings.FieldsFunc(text, func(r rune) bool { return r < 'a' || r > 'z' }) {
			if (stem[0] == '^' && strings.HasPrefix(w, stem[1:])) || w == stem[1:] {
				return true
			}
		}
		return false
	}
	return strings.Contains(text, stem)
}

type allergyConflict struct {
	Medicine string `json:"medicine"`
	Allergy  string `json:"allergy"`
	Family   string `json:"family,omitempty"`
	Message  string `json:"message"`
}

// patientAllergies lists the allergy phrases recorded in a patient's profile.
func patientAllergies(profile map[string]any) []string {
	var out []string
	add := func(s string) {
		for _, part := range allergySplit.Split(s, -1) {
			if part = strings.TrimSpace(part); part != "" && !noAllergy.MatchString(normText(part)) {
				out = append(out, part)
			}
		}
	}
	for _, k := range []string{"allergies_text", "allergies", "drug_allergies"} {
		switch v := profile[k].(type) {
		case string:
			add(v)
		case []any:
			for _, x := range v {
				switch y := x.(type) {
				case string:
					add(y)
				case map[string]any:
					for _, f := range []string{"substance", "name", "agent", "allergen"} {
						if s, ok := y[f].(string); ok {
							add(s)
							break
						}
					}
				}
			}
		}
	}
	if v, _ := profile["dental_anesthesia_allergy"].(string); v == "Sí" {
		out = append(out, "Anestésicos locales")
	}
	return out
}

// allergyConflicts compares the medicines of a receta with the allergy phrases of the patient.
func allergyConflicts(allergies []string, items []rxItem) []allergyConflict {
	var out []allergyConflict
	for _, it := range items {
		name := normText(it.Medicine)
		drug := normText(it.Medicine + " " + it.Brand)
	next:
		for _, al := range allergies {
			a := normText(al)
			if len(a) < 4 {
				continue
			}
			if strings.Contains(drug, a) || (len(name) >= 5 && strings.Contains(a, name)) {
				out = append(out, allergyConflict{it.Medicine, al, "", fmt.Sprintf("%s: el paciente tiene registrada alergia a «%s».", it.Medicine, al)})
				continue
			}
			for _, fam := range allergyFamilies {
				triggered := false
				for _, t := range fam.Triggers {
					if stemMatch(a, t) {
						triggered = true
						break
					}
				}
				if !triggered {
					continue
				}
				for _, m := range fam.Members {
					if stemMatch(drug, m) {
						out = append(out, allergyConflict{it.Medicine, al, fam.Name, fmt.Sprintf("%s pertenece a la familia de %s y el paciente tiene registrada alergia a «%s».", it.Medicine, fam.Name, al)})
						continue next
					}
				}
			}
		}
	}
	return out
}

type doseWarning struct {
	Medicine      string  `json:"medicine"`
	DailyMg       float64 `json:"daily_mg"`
	MaxDailyMg    float64 `json:"max_daily_mg"`
	MaxMgPerKgDay float64 `json:"max_mg_per_kg_day"`
	WeightKg      float64 `json:"weight_kg"`
	Message       string  `json:"message"`
}

// doseExceeds checks the daily total (dose x doses per day) against max mg/kg/day for the patient's weight.
func doseExceeds(m catMed, weightKg, doseMg, perDay float64) (doseWarning, bool) {
	if m.MaxMgPerKgDay <= 0 || weightKg <= 0 || doseMg <= 0 || perDay <= 0 {
		return doseWarning{}, false
	}
	daily, limit := doseMg*perDay, m.MaxMgPerKgDay*weightKg
	if daily <= limit*1.0000001 {
		return doseWarning{}, false
	}
	return doseWarning{m.Name, daily, limit, m.MaxMgPerKgDay, weightKg,
		fmt.Sprintf("%s: %.4g mg al día superan el máximo de referencia de %.4g mg/kg/día (%.4g mg para %.4g kg).", m.Name, daily, m.MaxMgPerKgDay, limit, weightKg)}, true
}
