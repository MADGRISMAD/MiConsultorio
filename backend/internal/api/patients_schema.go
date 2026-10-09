package api

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// The forms a clinic fills in depend on its giro: nobody asks a dog for a CURP or a person for a breed.
// A field has a stable key (stored in the patient's profile), so adding fields later never breaks old records.

type Field struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // text | longtext | number | date | select | multiselect | bool
	Group       string   `json:"group"`
	Options     []string `json:"options,omitempty"`
	Unit        string   `json:"unit,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Hint        string   `json:"hint,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
}

var (
	yesNo    = []string{"Sí", "No"}
	yesNoUnk = []string{"Sí", "No", "No sabe"}
)

func f(key, label, typ, group string, opts ...func(*Field)) Field {
	x := Field{Key: key, Label: label, Type: typ, Group: group}
	for _, o := range opts {
		o(&x)
	}
	return x
}
func options(v ...string) func(*Field) { return func(x *Field) { x.Options = v } }
func unit(u string) func(*Field)       { return func(x *Field) { x.Unit = u } }
func hint(h string) func(*Field)       { return func(x *Field) { x.Hint = h } }
func required() func(*Field)           { return func(x *Field) { x.Required = true } }
func ph(p string) func(*Field)         { return func(x *Field) { x.Placeholder = p } }

// ---------------------------------------------------------------------------
// Profile (antecedentes) per subject and giro
// ---------------------------------------------------------------------------

const (
	gAnt   = "Antecedentes personales"
	gHered = "Antecedentes heredofamiliares"
	gHab   = "Hábitos"
	gCont  = "Contacto de emergencia"
)

var personBase = []Field{
	f("occupation", "Ocupación", "text", "Datos generales"),
	f("emergency_name", "Nombre", "text", gCont),
	f("emergency_phone", "Teléfono", "text", gCont),
	f("blood_type", "Grupo sanguíneo", "select", gAnt, options("O+", "O-", "A+", "A-", "B+", "B-", "AB+", "AB-", "No sabe")),
	f("allergies_text", "Alergias", "longtext", gAnt, hint("Medicamentos, alimentos, látex, anestésicos… Escribe «Ninguna conocida» si no tiene."), required()),
	f("chronic_conditions", "Enfermedades crónicas", "multiselect", gAnt, options("Diabetes", "Hipertensión", "Cardiopatía", "Asma / EPOC", "Cáncer", "Enfermedad renal", "Enfermedad de la tiroides", "Epilepsia", "Otra")),
	f("chronic_other", "Otras enfermedades o detalle", "longtext", gAnt),
	f("current_medication", "Medicamentos que toma actualmente", "longtext", gAnt),
	f("surgeries", "Cirugías y hospitalizaciones previas", "longtext", gAnt),
	f("hereditary", "Enfermedades en la familia", "multiselect", gHered, options("Diabetes", "Hipertensión", "Cardiopatía", "Cáncer", "Enfermedad mental", "Enfermedad renal", "Otra")),
	f("hereditary_notes", "Detalle (quién y cuál)", "longtext", gHered),
	f("tobacco", "Tabaquismo", "select", gHab, options("No", "Ocasional", "Diario", "Exfumador")),
	f("alcohol", "Alcohol", "select", gHab, options("No", "Ocasional", "Frecuente", "Exconsumo")),
	f("physical_activity", "Actividad física", "select", gHab, options("Ninguna", "Ligera", "Moderada", "Intensa")),
	f("legacy_notes", "Notas del historial anterior", "longtext", "Otros", hint("Datos que venían del sistema anterior.")),
}

var personByKind = map[string][]Field{
	"PEDIATRICS": {
		f("birth_weight", "Peso al nacer", "number", "Pediatría", unit("kg")),
		f("gestation_weeks", "Semanas de gestación", "number", "Pediatría"),
		f("delivery_type", "Tipo de parto", "select", "Pediatría", options("Vaginal", "Cesárea", "No sabe")),
		f("breastfeeding", "Alimentación", "select", "Pediatría", options("Lactancia materna", "Fórmula", "Mixta", "Ya come alimentos")),
		f("vaccination", "Esquema de vacunación", "select", "Pediatría", options("Completo", "Incompleto", "No lo trae", "Desconocido")),
		f("development_notes", "Desarrollo (lenguaje, motricidad, escuela)", "longtext", "Pediatría"),
	},
	"GYNECOLOGY": {
		f("menarche_age", "Edad de la primera menstruación", "number", "Ginecología", unit("años")),
		f("last_period", "Fecha de última menstruación", "date", "Ginecología"),
		f("gestas", "Gestas", "number", "Ginecología"),
		f("partos", "Partos", "number", "Ginecología"),
		f("cesareas", "Cesáreas", "number", "Ginecología"),
		f("abortos", "Abortos", "number", "Ginecología"),
		f("contraception", "Método anticonceptivo", "text", "Ginecología"),
		f("last_pap", "Último Papanicolaou", "date", "Ginecología"),
	},
	"DENTAL": {
		f("dental_anesthesia_allergy", "Alergia a anestésicos locales o látex", "select", "Odontología", options("No", "Sí", "No sabe")),
		f("bleeding_issues", "Sangrado excesivo al cortarse o extraer", "select", "Odontología", options("No", "Sí", "No sabe")),
		f("brushing", "Cepillado al día", "select", "Odontología", options("1 vez", "2 veces", "3 o más", "Casi no")),
		f("flossing", "Usa hilo dental", "select", "Odontología", options("Diario", "A veces", "Nunca")),
		f("last_dental_visit", "Última visita al dentista", "date", "Odontología"),
		f("bruxism", "Rechina o aprieta los dientes", "select", "Odontología", options("No", "Sí", "No sabe")),
		f("orthodontics", "Tratamiento de ortodoncia previo", "select", "Odontología", options("No", "Sí")),
	},
	"PSYCHOLOGY": {
		f("prior_therapy", "Ha estado en terapia", "select", "Salud mental", options("Nunca", "Antes", "Actualmente")),
		f("psychiatric_history", "Antecedentes psicológicos o psiquiátricos", "longtext", "Salud mental"),
		f("psych_medication", "Medicamento psiquiátrico", "longtext", "Salud mental"),
		f("self_harm_history", "Antecedentes de autolesión o ideación suicida", "select", "Salud mental", options("No", "Sí", "Prefiere no decir"), hint("Si es «Sí», registra el plan de seguridad en la bitácora.")),
		f("support_network", "Red de apoyo", "longtext", "Salud mental"),
		f("referred_by", "Quién lo refiere", "text", "Salud mental"),
	},
	"NUTRITION": {
		f("nutrition_goal", "Objetivo", "select", "Nutrición", options("Bajar de peso", "Subir de peso", "Mantener", "Ganar masa muscular", "Control de enfermedad", "Alimentación saludable")),
		f("diet_pattern", "¿Cómo come en un día normal?", "longtext", "Nutrición"),
		f("food_allergies", "Alergias o intolerancias alimentarias", "longtext", "Nutrición"),
		f("meals_per_day", "Comidas al día", "number", "Nutrición"),
		f("water_liters", "Agua al día", "number", "Nutrición", unit("litros")),
		f("supplements", "Suplementos", "text", "Nutrición"),
		f("food_dislikes", "Alimentos que no le gustan", "longtext", "Nutrición", hint("Separados por comas: se evitan al armar el menú.")),
		f("weight_goal_kg", "Peso meta", "number", "Nutrición", unit("kg")),
		f("sleep_hours", "Horas de sueño al día", "number", "Nutrición"),
		f("stress_level", "Nivel de estrés", "select", "Nutrición", options("Bajo", "Medio", "Alto")),
		f("eats_out", "Come fuera de casa", "select", "Nutrición", options("Casi nunca", "A veces", "Seguido")),
		f("digestive_issues", "Problemas digestivos", "longtext", "Nutrición", hint("Reflujo, gastritis, estreñimiento, colitis, intolerancias…")),
	},
	"PHYSIOTHERAPY": physioFields("Fisioterapia"),
	"ORTHOPEDICS":   physioFields("Ortopedia"),
	"CHIROPRACTIC":  physioFields("Quiropráctica"),
	"DERMATOLOGY": {
		f("fitzpatrick", "Fototipo de piel (Fitzpatrick)", "select", "Dermatología", options("I", "II", "III", "IV", "V", "VI", "No sabe")),
		f("sun_exposure", "Exposición al sol", "select", "Dermatología", options("Baja", "Moderada", "Alta")),
		f("skin_history", "Enfermedades de la piel previas", "longtext", "Dermatología"),
		f("skincare_products", "Productos que usa en la piel", "longtext", "Dermatología"),
	},
}

func physioFields(group string) []Field {
	return []Field{
		f("affected_area", "Zona o segmento que le duele", "text", group),
		f("injury_history", "Lesiones o fracturas previas", "longtext", group),
		f("imaging_studies", "Estudios de imagen (rayos X, resonancia…)", "longtext", group),
		f("work_posture", "Postura o esfuerzo en el trabajo", "longtext", group),
	}
}

// Animals: the "owner" lives in the guardian_* columns.
var animalBase = []Field{
	f("species", "Especie", "select", "El animal", options("Perro", "Gato", "Ave", "Conejo", "Roedor", "Reptil", "Equino", "Bovino", "Porcino", "Ovino / caprino", "Otra"), required()),
	f("breed", "Raza", "text", "El animal"),
	f("color", "Color / señas particulares", "text", "El animal"),
	f("microchip", "Número de microchip", "text", "El animal"),
	f("sterilized", "Esterilizado", "select", "El animal", options("Sí", "No", "No sabe")),
	f("vaccines_up_to_date", "Vacunas al corriente", "select", "Prevención", options("Sí", "No", "No sabe")),
	f("last_vaccine", "Última vacuna", "date", "Prevención"),
	f("last_deworming", "Última desparasitación", "date", "Prevención"),
	f("diet", "Alimentación", "text", "Hábitos"),
	f("housing", "Dónde vive", "select", "Hábitos", options("Dentro de casa", "Patio o jardín", "Calle / campo", "Granja / establo")),
	f("other_animals", "Convive con otros animales", "text", "Hábitos"),
	f("allergies_text", "Alergias", "longtext", gAnt, required(), hint("Medicamentos, alimentos, picaduras… Escribe «Ninguna conocida» si no tiene.")),
	f("chronic_other", "Enfermedades crónicas o detalle", "longtext", gAnt),
	f("current_medication", "Medicamentos que toma actualmente", "longtext", gAnt),
	f("surgeries", "Cirugías previas", "longtext", gAnt),
	f("behavior_notes", "Comportamiento (agresivo, miedoso…)", "longtext", "Hábitos"),
	f("legacy_notes", "Notas del historial anterior", "longtext", "Otros"),
}

// clinicKinds returns the giros a clinic works with (main first).
func clinicKindsOf(kind string, specialties []string) []string {
	out := []string{kind}
	for _, s := range specialties {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// subjectsFor says whether the clinic sees people, animals or both.
func subjectsFor(kinds []string) []string {
	vet, other := false, false
	for _, k := range kinds {
		if k == "VETERINARY" {
			vet = true
		} else {
			other = true
		}
	}
	switch {
	case vet && other:
		return []string{"person", "animal"}
	case vet:
		return []string{"animal"}
	}
	return []string{"person"}
}

// Questions of the common history that a giro does not ask about. A question is dropped only when
// every giro of the clinic skips it, so a clinic with several specialties keeps what any of them needs.
var skipProfile = map[string][]string{
	"DENTAL":     {"blood_type", "physical_activity", "hereditary", "hereditary_notes"},
	"PSYCHOLOGY": {"blood_type", "surgeries"},
	"NUTRITION":  {"blood_type"},
}

// Giros for which a drug-allergy answer is not mandatory (the visit is not about the body).
var allergiesOptional = map[string]bool{"PSYCHOLOGY": true}

func profileFields(subject string, kinds []string) []Field {
	if subject == "animal" {
		return slices.Clone(animalBase)
	}
	out := make([]Field, 0, len(personBase))
	for _, x := range personBase {
		skipped, optional := len(kinds) > 0, len(kinds) > 0
		for _, k := range kinds {
			if !slices.Contains(skipProfile[k], x.Key) {
				skipped = false
			}
			if !allergiesOptional[k] {
				optional = false
			}
		}
		if skipped {
			continue
		}
		if x.Key == "allergies_text" && optional {
			x.Required = false
		}
		out = append(out, x)
	}
	seen := map[string]bool{}
	for _, k := range kinds {
		for _, x := range personByKind[k] {
			if !seen[x.Key] {
				seen[x.Key] = true
				out = append(out, x)
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Measures of a consultation (signos vitales y mediciones)
// ---------------------------------------------------------------------------

const gVit = "Signos vitales"

var personMeasures = []Field{
	f("weight_kg", "Peso", "number", gVit, unit("kg")),
	f("height_cm", "Talla", "number", gVit, unit("cm")),
	f("bp_sys", "Presión sistólica", "number", gVit, unit("mmHg")),
	f("bp_dia", "Presión diastólica", "number", gVit, unit("mmHg")),
	f("heart_rate", "Frecuencia cardiaca", "number", gVit, unit("lpm")),
	f("resp_rate", "Frecuencia respiratoria", "number", gVit, unit("rpm")),
	f("temp_c", "Temperatura", "number", gVit, unit("°C")),
	f("spo2", "Saturación de oxígeno", "number", gVit, unit("%")),
	f("glucose", "Glucosa", "number", gVit, unit("mg/dL")),
}

// Pain and range of motion of every session, charted in the patient's Evolución.
var rehabMeasures = []Field{
	f("pain_eva", "Dolor (escala EVA 0 a 10)", "number", "Sesión", hint("0 = sin dolor, 10 = el peor dolor imaginable.")),
	f("rom_deg", "Rango de movimiento", "number", "Sesión", unit("°")),
	f("rom_note", "Articulación o movimiento medido", "text", "Sesión", ph("Ej. flexión de rodilla derecha")),
}

var measuresByKind = map[string][]Field{
	"PEDIATRICS": {f("head_circumference", "Perímetro cefálico", "number", "Pediatría", unit("cm"))},
	"NUTRITION": {
		f("waist_cm", "Cintura", "number", "Antropometría", unit("cm")),
		f("hip_cm", "Cadera", "number", "Antropometría", unit("cm")),
		f("arm_cm", "Brazo (circunferencia)", "number", "Antropometría", unit("cm")),
		f("body_fat", "Grasa corporal", "number", "Antropometría", unit("%")),
		f("muscle_kg", "Masa muscular", "number", "Antropometría", unit("kg")),
		f("visceral_fat", "Grasa visceral", "number", "Antropometría", hint("Nivel que marca la báscula de bioimpedancia.")),
		f("body_water", "Agua corporal", "number", "Antropometría", unit("%")),
	},
	"PHYSIOTHERAPY": rehabMeasures,
	"CHIROPRACTIC":  rehabMeasures,
	"ORTHOPEDICS":   rehabMeasures,
	"GYNECOLOGY":    {f("fundal_height", "Altura del fondo uterino", "number", "Ginecología", unit("cm"))},
	"DENTAL":        {f("teeth", "Piezas dentales tratadas", "text", "Odontología", ph("Ej. 16, 26"))},
	"PSYCHOLOGY": {
		f("session_number", "Número de sesión", "number", "Sesión"),
		f("mood", "Estado de ánimo", "select", "Sesión", options("Muy bajo", "Bajo", "Estable", "Alto", "Elevado")),
		f("risk", "Valoración de riesgo", "select", "Sesión", options("Sin riesgo", "Bajo", "Moderado", "Alto")),
	},
}

var animalMeasures = []Field{
	f("weight_kg", "Peso", "number", gVit, unit("kg")),
	f("temp_c", "Temperatura", "number", gVit, unit("°C")),
	f("heart_rate", "Frecuencia cardiaca", "number", gVit, unit("lpm")),
	f("resp_rate", "Frecuencia respiratoria", "number", gVit, unit("rpm")),
	f("mucous", "Mucosas", "select", gVit, options("Rosadas", "Pálidas", "Ictéricas", "Congestionadas", "Cianóticas")),
	f("hydration", "Hidratación", "select", gVit, options("Normal", "Deshidratación leve", "Moderada", "Severa")),
	f("body_condition", "Condición corporal (1 a 9)", "number", gVit),
}

// Which vital signs each giro takes. A kind not listed here takes all of them.
var vitalsByKind = map[string][]string{
	"PEDIATRICS":    {"weight_kg", "height_cm", "temp_c", "heart_rate", "resp_rate", "spo2"},
	"GYNECOLOGY":    {"weight_kg", "height_cm", "bp_sys", "bp_dia", "heart_rate", "temp_c"},
	"DENTAL":        {"bp_sys", "bp_dia", "heart_rate"},
	"NUTRITION":     {"weight_kg", "height_cm", "bp_sys", "bp_dia", "glucose"},
	"PHYSIOTHERAPY": {"bp_sys", "bp_dia", "heart_rate"},
	"CHIROPRACTIC":  {"bp_sys", "bp_dia", "heart_rate"},
	"ORTHOPEDICS":   {"weight_kg", "height_cm", "bp_sys", "bp_dia", "heart_rate"},
	"DERMATOLOGY":   {},
	"PSYCHOLOGY":    {},
}

// allMeasureFields is every measure a note may carry, used to show notes already saved.
func allMeasureFields(subject string, kinds []string) []Field {
	return measureSet(subject, kinds, func(string) bool { return true })
}

func measureFields(subject string, kinds []string) []Field {
	keep := map[string]bool{}
	all := false
	for _, k := range kinds {
		list, ok := vitalsByKind[k]
		if !ok {
			all = true
		}
		for _, key := range list {
			keep[key] = true
		}
	}
	return measureSet(subject, kinds, func(key string) bool { return all || keep[key] })
}

func measureSet(subject string, kinds []string, vital func(key string) bool) []Field {
	if subject == "animal" {
		return slices.Clone(animalMeasures)
	}
	out := []Field{}
	for _, x := range personMeasures {
		if vital(x.Key) {
			out = append(out, x)
		}
	}
	seen := map[string]bool{}
	for _, k := range kinds {
		for _, x := range measuresByKind[k] {
			if !seen[x.Key] {
				seen[x.Key] = true
				out = append(out, x)
			}
		}
	}
	return out
}

// rxMode: who may prescribe medicines. Psychologists, nutritionists, physiotherapists and chiropractors
// do not prescribe medication in Mexico, so their "receta" is a sheet of indications instead.
func rxModeFor(kinds []string) string {
	for _, k := range kinds {
		switch k {
		case "GENERAL_MEDICAL", "DENTAL", "PEDIATRICS", "INTERNAL_MEDICINE", "DERMATOLOGY", "GYNECOLOGY", "ORTHOPEDICS", "VETERINARY":
			return "medication"
		}
	}
	return "instructions"
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

const maxLong = 4000

// cleanValues keeps only known keys, checks types and options, and returns what to store.
// When requireAll is true the Required fields must be present.
func cleanValues(fields []Field, in map[string]any, requireAll bool) (map[string]any, string) {
	byKey := make(map[string]Field, len(fields))
	for _, x := range fields {
		byKey[x.Key] = x
	}
	for k := range in {
		if _, ok := byKey[k]; !ok {
			return nil, "Campo no reconocido: " + k + "."
		}
	}
	out := map[string]any{}
	for _, x := range fields {
		raw, present := in[x.Key]
		empty := !present || raw == nil || raw == "" || raw == false && x.Type != "bool"
		if arr, ok := raw.([]any); ok && len(arr) == 0 {
			empty = true
		}
		if empty {
			if requireAll && x.Required {
				return nil, x.Label + " es obligatorio."
			}
			continue
		}
		switch x.Type {
		case "text", "longtext":
			s, ok := raw.(string)
			if !ok {
				return nil, x.Label + " no es válido."
			}
			s = strings.TrimSpace(s)
			max := 200
			if x.Type == "longtext" {
				max = maxLong
			}
			if utf8.RuneCountInString(s) > max {
				return nil, x.Label + " es demasiado largo."
			}
			if s != "" {
				out[x.Key] = s
			}
		case "number":
			var n float64
			switch v := raw.(type) {
			case float64:
				n = v
			case string:
				p, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(v), ",", ".", 1), 64)
				if err != nil {
					return nil, x.Label + " debe ser un número."
				}
				n = p
			default:
				return nil, x.Label + " debe ser un número."
			}
			if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 100000 {
				return nil, x.Label + " no es válido."
			}
			out[x.Key] = n
		case "date":
			s, ok := raw.(string)
			if !ok {
				return nil, x.Label + " no es válida."
			}
			if _, err := time.Parse("2006-01-02", s); err != nil {
				return nil, x.Label + " no es una fecha válida."
			}
			out[x.Key] = s
		case "select":
			s, ok := raw.(string)
			if !ok || !slices.Contains(x.Options, s) {
				return nil, x.Label + ": elige una de las opciones."
			}
			out[x.Key] = s
		case "multiselect":
			arr, ok := raw.([]any)
			if !ok {
				return nil, x.Label + " no es válido."
			}
			vals := make([]string, 0, len(arr))
			for _, a := range arr {
				s, ok := a.(string)
				if !ok || !slices.Contains(x.Options, s) {
					return nil, x.Label + ": opción no válida."
				}
				if !slices.Contains(vals, s) {
					vals = append(vals, s)
				}
			}
			out[x.Key] = vals
		case "bool":
			b, ok := raw.(bool)
			if !ok {
				return nil, x.Label + " no es válido."
			}
			out[x.Key] = b
		default:
			return nil, fmt.Sprintf("Tipo de campo desconocido: %s.", x.Type)
		}
	}
	return out, ""
}

// Logbook entry types. Animals can also log the services a veterinary clinic gives besides consultations;
// people can log therapy sessions in the giros that work by sessions.
func encounterKindsFor(subject string, kinds []string) []string {
	out := []string{"consulta", "seguimiento", "procedimiento", "llamada", "nota"}
	if subject == "animal" {
		return append(out, "estetica", "paseo", "adiestramiento", "hospedaje")
	}
	for _, k := range kinds {
		if k == "PSYCHOLOGY" || k == "PHYSIOTHERAPY" || k == "CHIROPRACTIC" {
			return append(out, "sesion")
		}
	}
	return out
}
