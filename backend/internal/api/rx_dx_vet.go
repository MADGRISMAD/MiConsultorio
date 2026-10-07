package api

import (
	"net/http"
	"slices"
	"sort"
	"strings"
)

const vetDxDisclaimer = "Catálogo de referencia: no sustituye el criterio clínico ni la información del producto. Son términos de uso común sin códigos de ningún estándar con licencia; si no encuentras el tuyo, escribe el diagnóstico libremente."

type vetDxEntry struct {
	Code     string   `json:"code"`
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Species  []string `json:"species"`
}

type vetDxRow struct{ cat, sp, name string }

const (
	vdP   = "Perro"
	vdG   = "Gato"
	vdC   = "Conejo"
	vdPG  = "Perro,Gato"
	vdAll = "Perro,Gato,Conejo"
)

// vetDiagnoses is a reference list of common veterinary diagnoses in Spanish; it carries no codes
// because VeNom and SNOMED-CT Veterinary are licensed standards.
var vetDiagnoses = buildVetDx([]vetDxRow{
	{"Dermatología", vdPG, "Dermatitis alérgica por pulgas"},
	{"Dermatología", vdPG, "Dermatitis atópica"},
	{"Dermatología", vdPG, "Alergia alimentaria"},
	{"Dermatología", vdP, "Piodermia superficial"},
	{"Dermatología", vdP, "Piodermia profunda"},
	{"Dermatología", vdP, "Dermatitis húmeda aguda (hot spot)"},
	{"Dermatología", vdP, "Dermatitis por Malassezia"},
	{"Dermatología", vdP, "Demodicosis"},
	{"Dermatología", vdPG, "Sarna sarcóptica"},
	{"Dermatología", vdPG, "Dermatofitosis (tiña)"},
	{"Dermatología", vdPG, "Infestación por pulgas"},
	{"Dermatología", vdPG, "Infestación por garrapatas"},
	{"Dermatología", vdPG, "Seborrea"},
	{"Dermatología", vdP, "Pododermatitis"},
	{"Dermatología", vdPG, "Absceso subcutáneo"},
	{"Dermatología", vdG, "Acné felino"},
	{"Dermatología", vdG, "Complejo granuloma eosinofílico"},
	{"Dermatología", vdG, "Alopecia psicógena"},
	{"Dermatología", vdPG, "Dermatitis por contacto"},
	{"Dermatología", vdPG, "Sacculitis anal"},
	{"Dermatología", vdC, "Dermatitis por ácaros (Cheyletiella)"},
	{"Dermatología", vdC, "Dermatofitosis"},
	{"Dermatología", vdC, "Pododermatitis ulcerativa (patas de jarrete)"},
	{"Dermatología", vdC, "Absceso subcutáneo"},
	{"Dermatología", vdC, "Miasis (gusanera)"},
	{"Dermatología", vdC, "Dermatitis húmeda del pliegue de la papada"},
	{"Otología", vdPG, "Otitis externa"},
	{"Otología", vdPG, "Otitis media"},
	{"Otología", vdPG, "Otitis por Malassezia"},
	{"Otología", vdPG, "Otitis bacteriana"},
	{"Otología", vdPG, "Otoacariasis (ácaros del oído)"},
	{"Otología", vdP, "Otohematoma"},
	{"Otología", vdC, "Otitis externa"},
	{"Otología", vdC, "Otitis media e interna"},
	{"Gastroenterología", vdPG, "Gastroenteritis"},
	{"Gastroenterología", vdP, "Gastroenteritis hemorrágica"},
	{"Gastroenterología", vdPG, "Gastritis aguda"},
	{"Gastroenterología", vdPG, "Gastritis crónica"},
	{"Gastroenterología", vdPG, "Diarrea aguda"},
	{"Gastroenterología", vdPG, "Diarrea crónica"},
	{"Gastroenterología", vdPG, "Colitis"},
	{"Gastroenterología", vdPG, "Enteropatía crónica"},
	{"Gastroenterología", vdPG, "Pancreatitis"},
	{"Gastroenterología", vdPG, "Cuerpo extraño gastrointestinal"},
	{"Gastroenterología", vdPG, "Indiscreción alimentaria"},
	{"Gastroenterología", vdPG, "Estreñimiento"},
	{"Gastroenterología", vdG, "Bolas de pelo (tricobezoar)"},
	{"Gastroenterología", vdG, "Megacolon"},
	{"Gastroenterología", vdG, "Lipidosis hepática"},
	{"Gastroenterología", vdP, "Dilatación-vólvulo gástrico"},
	{"Gastroenterología", vdPG, "Hepatitis"},
	{"Gastroenterología", vdC, "Estasis gastrointestinal"},
	{"Gastroenterología", vdC, "Diarrea"},
	{"Gastroenterología", vdC, "Tricobezoar"},
	{"Gastroenterología", vdC, "Timpanismo (gas intestinal)"},
	{"Gastroenterología", vdC, "Coccidiosis"},
	{"Gastroenterología", vdC, "Disbiosis cecal"},
	{"Odontología", vdPG, "Gingivitis"},
	{"Odontología", vdPG, "Enfermedad periodontal"},
	{"Odontología", vdPG, "Sarro dental"},
	{"Odontología", vdPG, "Fractura dental"},
	{"Odontología", vdG, "Gingivoestomatitis crónica felina"},
	{"Odontología", vdG, "Lesión de reabsorción odontoclástica felina"},
	{"Odontología", vdC, "Maloclusión dental"},
	{"Odontología", vdC, "Sobrecrecimiento dental"},
	{"Odontología", vdC, "Absceso dental"},
	{"Odontología", vdC, "Espolones dentales"},
	{"Parasitología", vdPG, "Parasitosis intestinal"},
	{"Parasitología", vdPG, "Giardiasis"},
	{"Parasitología", vdPG, "Ascariasis"},
	{"Parasitología", vdP, "Anquilostomiasis"},
	{"Parasitología", vdP, "Tricuriasis"},
	{"Parasitología", vdPG, "Cestodiasis (Dipylidium)"},
	{"Parasitología", vdP, "Dirofilariosis (gusano del corazón)"},
	{"Parasitología", vdP, "Ehrlichiosis canina"},
	{"Parasitología", vdP, "Babesiosis"},
	{"Parasitología", vdP, "Anaplasmosis"},
	{"Parasitología", vdG, "Toxoplasmosis"},
	{"Parasitología", vdC, "Oxiuriasis (Passalurus)"},
	{"Parasitología", vdC, "Encefalitozoonosis"},
	{"Enfermedades infecciosas", vdP, "Parvovirosis canina"},
	{"Enfermedades infecciosas", vdP, "Moquillo canino"},
	{"Enfermedades infecciosas", vdP, "Tos de las perreras"},
	{"Enfermedades infecciosas", vdP, "Hepatitis infecciosa canina"},
	{"Enfermedades infecciosas", vdP, "Leptospirosis"},
	{"Enfermedades infecciosas", vdP, "Coronavirosis canina"},
	{"Enfermedades infecciosas", vdG, "Panleucopenia felina"},
	{"Enfermedades infecciosas", vdG, "Rinotraqueítis viral felina"},
	{"Enfermedades infecciosas", vdG, "Calicivirosis felina"},
	{"Enfermedades infecciosas", vdG, "Leucemia viral felina"},
	{"Enfermedades infecciosas", vdG, "Inmunodeficiencia viral felina"},
	{"Enfermedades infecciosas", vdG, "Peritonitis infecciosa felina"},
	{"Enfermedades infecciosas", vdC, "Mixomatosis"},
	{"Enfermedades infecciosas", vdC, "Enfermedad hemorrágica viral del conejo"},
	{"Enfermedades infecciosas", vdC, "Pasteurelosis (rinitis contagiosa)"},
	{"Respiratorio", vdPG, "Neumonía"},
	{"Respiratorio", vdPG, "Bronquitis"},
	{"Respiratorio", vdPG, "Traqueobronquitis"},
	{"Respiratorio", vdG, "Asma felina"},
	{"Respiratorio", vdG, "Complejo respiratorio felino"},
	{"Respiratorio", vdP, "Síndrome braquicefálico"},
	{"Respiratorio", vdP, "Colapso traqueal"},
	{"Respiratorio", vdPG, "Rinitis"},
	{"Respiratorio", vdPG, "Edema pulmonar"},
	{"Respiratorio", vdPG, "Efusión pleural"},
	{"Respiratorio", vdC, "Rinitis (resfriado del conejo)"},
	{"Respiratorio", vdC, "Neumonía"},
	{"Cardiología", vdP, "Enfermedad valvular mitral crónica"},
	{"Cardiología", vdP, "Cardiomiopatía dilatada"},
	{"Cardiología", vdG, "Cardiomiopatía hipertrófica"},
	{"Cardiología", vdPG, "Insuficiencia cardíaca congestiva"},
	{"Cardiología", vdPG, "Arritmia cardíaca"},
	{"Cardiología", vdG, "Tromboembolismo arterial"},
	{"Nefrología y urología", vdPG, "Enfermedad renal crónica"},
	{"Nefrología y urología", vdPG, "Lesión renal aguda"},
	{"Nefrología y urología", vdPG, "Infección del tracto urinario"},
	{"Nefrología y urología", vdPG, "Cistitis"},
	{"Nefrología y urología", vdPG, "Urolitiasis"},
	{"Nefrología y urología", vdG, "Cistitis idiopática felina"},
	{"Nefrología y urología", vdG, "Obstrucción uretral"},
	{"Nefrología y urología", vdP, "Incontinencia urinaria"},
	{"Nefrología y urología", vdPG, "Pielonefritis"},
	{"Nefrología y urología", vdC, "Urolitiasis (lodo de calcio)"},
	{"Nefrología y urología", vdC, "Cistitis"},
	{"Endocrinología", vdPG, "Diabetes mellitus"},
	{"Endocrinología", vdG, "Hipertiroidismo"},
	{"Endocrinología", vdP, "Hipotiroidismo"},
	{"Endocrinología", vdP, "Hiperadrenocorticismo (síndrome de Cushing)"},
	{"Endocrinología", vdP, "Hipoadrenocorticismo (enfermedad de Addison)"},
	{"Endocrinología", vdPG, "Obesidad"},
	{"Endocrinología", vdPG, "Cetoacidosis diabética"},
	{"Oftalmología", vdPG, "Conjuntivitis"},
	{"Oftalmología", vdPG, "Úlcera corneal"},
	{"Oftalmología", vdP, "Queratoconjuntivitis seca"},
	{"Oftalmología", vdP, "Glaucoma"},
	{"Oftalmología", vdPG, "Uveítis"},
	{"Oftalmología", vdP, "Cataratas"},
	{"Oftalmología", vdP, "Prolapso de la glándula del tercer párpado (ojo de cereza)"},
	{"Oftalmología", vdPG, "Epífora"},
	{"Oftalmología", vdG, "Queratitis por herpesvirus felino"},
	{"Oftalmología", vdC, "Conjuntivitis"},
	{"Oftalmología", vdC, "Dacriocistitis"},
	{"Oftalmología", vdC, "Úlcera corneal"},
	{"Traumatología y ortopedia", vdP, "Displasia de cadera"},
	{"Traumatología y ortopedia", vdP, "Displasia de codo"},
	{"Traumatología y ortopedia", vdP, "Luxación de rótula"},
	{"Traumatología y ortopedia", vdP, "Ruptura del ligamento cruzado craneal"},
	{"Traumatología y ortopedia", vdPG, "Osteoartritis"},
	{"Traumatología y ortopedia", vdAll, "Fractura"},
	{"Traumatología y ortopedia", vdPG, "Luxación"},
	{"Traumatología y ortopedia", vdP, "Hernia de disco intervertebral"},
	{"Traumatología y ortopedia", vdPG, "Esguince"},
	{"Traumatología y ortopedia", vdPG, "Claudicación"},
	{"Traumatología y ortopedia", vdC, "Lesión medular por fractura de columna"},
	{"Traumatología y ortopedia", vdC, "Osteoartritis"},
	{"Reproducción y neonatología", vdPG, "Piometra"},
	{"Reproducción y neonatología", vdPG, "Distocia"},
	{"Reproducción y neonatología", vdPG, "Gestación"},
	{"Reproducción y neonatología", vdPG, "Pseudogestación"},
	{"Reproducción y neonatología", vdP, "Hiperplasia prostática benigna"},
	{"Reproducción y neonatología", vdPG, "Mastitis"},
	{"Reproducción y neonatología", vdPG, "Criptorquidia"},
	{"Reproducción y neonatología", vdC, "Adenocarcinoma uterino"},
	{"Reproducción y neonatología", vdC, "Hiperplasia endometrial"},
	{"Neurología", vdPG, "Convulsiones"},
	{"Neurología", vdP, "Epilepsia idiopática"},
	{"Neurología", vdPG, "Síndrome vestibular"},
	{"Neurología", vdC, "Síndrome vestibular"},
	{"Neurología", vdPG, "Traumatismo craneoencefálico"},
	{"Toxicología y urgencias", vdPG, "Intoxicación"},
	{"Toxicología y urgencias", vdAll, "Golpe de calor"},
	{"Toxicología y urgencias", vdPG, "Mordedura"},
	{"Toxicología y urgencias", vdAll, "Herida cortante"},
	{"Toxicología y urgencias", vdPG, "Atropellamiento"},
	{"Toxicología y urgencias", vdAll, "Deshidratación"},
	{"Toxicología y urgencias", vdAll, "Shock"},
	{"Toxicología y urgencias", vdPG, "Reacción alérgica aguda"},
	{"Toxicología y urgencias", vdPG, "Picadura de insecto"},
	{"Hematología y oncología", vdPG, "Anemia"},
	{"Hematología y oncología", vdPG, "Neoplasia mamaria"},
	{"Hematología y oncología", vdP, "Linfoma"},
	{"Hematología y oncología", vdPG, "Lipoma"},
	{"Hematología y oncología", vdP, "Mastocitoma"},
	{"Hematología y oncología", vdPG, "Neoplasia cutánea"},
	{"Preventivo y control", vdAll, "Control de salud"},
	{"Preventivo y control", vdAll, "Vacunación"},
	{"Preventivo y control", vdAll, "Desparasitación"},
	{"Preventivo y control", vdAll, "Revisión postoperatoria"},
	{"Preventivo y control", vdAll, "Esterilización"},
	{"Preventivo y control", vdAll, "Paciente geriátrico"},
})

func buildVetDx(rows []vetDxRow) []vetDxEntry {
	out := make([]vetDxEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, vetDxEntry{Name: r.name, Category: r.cat, Species: strings.Split(r.sp, ",")})
	}
	sort.SliceStable(out, func(i, j int) bool { return normText(out[i].Name) < normText(out[j].Name) })
	return out
}

func searchVetDiagnoses(w http.ResponseWriter, q, species string, limit int) {
	type hit struct {
		e     vetDxEntry
		score int
	}
	var hits []hit
	for _, e := range vetDiagnoses {
		if vetDxSpecies(species) && !slices.ContainsFunc(e.Species, func(sp string) bool { return normText(sp) == species }) {
			continue
		}
		name := normText(e.Name)
		if sc := matchScore(name, name+" "+normText(e.Category), q); sc > 0 {
			hits = append(hits, hit{e, sc})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := []vetDxEntry{}
	for _, h := range hits {
		if len(out) == limit {
			break
		}
		out = append(out, h.e)
	}
	writeJSON(w, http.StatusOK, map[string]any{"diagnoses": out, "total": len(hits), "disclaimer": vetDxDisclaimer})
}

func vetDxSpecies(species string) bool {
	return species == "perro" || species == "gato" || species == "conejo"
}
