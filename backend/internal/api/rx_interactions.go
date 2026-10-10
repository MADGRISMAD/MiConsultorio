package api

import (
	"sort"
	"strings"
)

// Drug-drug interactions. A short curated list of the clinically important and common ones, written for the prescriber's
// attention (never a substitute for their judgement, and not exhaustive). Drugs are matched by active ingredient, with the
// same lenient rules as the allergy check (accents and case ignored, "^x" a word prefix, "=x" a whole word).
//
// "grave" interactions must be acknowledged with a reason before the receta is issued; "moderada" ones are shown.

const (
	sevGrave    = "grave"
	sevModerada = "moderada"
)

// drugGroups are sets of active ingredients (stems).
var drugGroups = map[string][]string{
	"aines":             {"ibuprofen", "naproxen", "diclofenac", "ketorolac", "meloxicam", "piroxicam", "indometacin", "celecoxib", "etoricoxib", "nimesulid", "ketoprofen", "acido acetilsalicilico", "aspirin"},
	"ketorolaco":        {"ketorolac"},
	"aspirina":          {"acido acetilsalicilico", "aspirin"},
	"warfarina":         {"warfarin", "acenocumarol"},
	"anticoagulantes":   {"warfarin", "acenocumarol", "rivaroxaban", "apixaban", "dabigatran", "edoxaban", "heparin", "enoxaparin"},
	"antiagregantes":    {"clopidogrel", "ticagrelor", "prasugrel"},
	"ieca":              {"enalapril", "captopril", "lisinopril", "ramipril", "perindopril", "benazepril"},
	"ara2":              {"losartan", "valsartan", "telmisartan", "irbesartan", "candesartan", "olmesartan"},
	"ahorradores_k":     {"espironolactona", "eplerenona", "amilorida", "triamtereno"},
	"potasio":           {"cloruro de potasio", "potasio"},
	"diureticos":        {"hidroclorotiazida", "clortalidona", "indapamida", "furosemida", "torasemida", "bumetanida"},
	"nitratos":          {"isosorbide", "nitroglicerina", "mononitrato", "dinitrato"},
	"pde5":              {"sildenafil", "tadalafil", "vardenafil"},
	"isrs":              {"fluoxetin", "sertralin", "paroxetin", "citalopram", "escitalopram", "fluvoxamin"},
	"serotoninergicos":  {"fluoxetin", "sertralin", "paroxetin", "citalopram", "escitalopram", "fluvoxamin", "venlafaxin", "duloxetin", "tramadol", "dextrometorfano", "sumatriptan", "rizatriptan", "zolmitriptan", "linezolid"},
	"imao":              {"moclobemid", "tranilcipromin", "fenelzin", "selegilin"},
	"tramadol":          {"tramadol"},
	"opioides":          {"tramadol", "codein", "morfin", "fentanil", "oxicodon", "buprenorfin", "metadon"},
	"benzodiacepinas":   {"diazepam", "clonazepam", "alprazolam", "lorazepam", "bromazepam", "midazolam"},
	"estatinas":         {"simvastatin", "atorvastatin", "lovastatin", "rosuvastatin", "pravastatin"},
	"simvastatina":      {"simvastatin", "lovastatin"},
	"macrolidos":        {"claritromicin", "eritromicin", "azitromicin"},
	"inhibidores_3a4":   {"claritromicin", "eritromicin", "ketoconazol", "itraconazol", "ritonavir", "voriconazol"},
	"quinolonas":        {"floxacin"},
	"azoles":            {"fluconazol", "ketoconazol", "itraconazol", "voriconazol"},
	"metronidazol":      {"metronidazol", "tinidazol"},
	"tmp_smx":           {"trimetoprim", "sulfametoxazol", "cotrimoxazol"},
	"metotrexato":       {"metotrexat"},
	"litio":             {"=litio", "carbonato de litio"},
	"digoxina":          {"digoxin"},
	"amiodarona":        {"amiodaron"},
	"betabloqueantes":   {"propranolol", "atenolol", "metoprolol", "bisoprolol", "carvedilol", "nebivolol"},
	"calcioantag_nodhp": {"verapamil", "diltiazem"},
	"sulfonilureas":     {"glibenclamid", "glimepirid", "glipizid"},
	"insulina":          {"insulin"},
	"levotiroxina":      {"levotiroxin"},
	"omeprazol_like":    {"omeprazol", "esomeprazol"},
	"cationes":          {"hidroxido de aluminio", "hidroxido de magnesio", "carbonato de calcio", "sulfato ferroso", "hierro", "sucralfato", "sulfato de zinc", "antiacido"},
	"tetraciclinas":     {"doxiciclin", "tetraciclin", "minociclin"},
	"corticoides":       {"prednison", "prednisolon", "dexametason", "metilprednisolon", "hidrocortison", "betametason"},
	"teofilina":         {"teofilin", "aminofilin"},
	"anticonceptivos":   {"etinilestradiol", "levonorgestrel", "drospirenon", "anticonceptiv", "desogestrel"},
	"inductores":        {"fenitoin", "carbamazepin", "rifampicin", "fenobarbital", "hierba de san juan"},
	"alopurinol":        {"alopurinol"},
	"azatioprina":       {"azatioprin"},
}

type interactionRule struct {
	A, B     string // groups; the same group twice = two different drugs of it
	Severity string
	Message  string
}

var interactionRules = []interactionRule{
	// bleeding
	{"warfarina", "aines", sevGrave, "los antiinflamatorios (AINEs) con warfarina o acenocumarol aumentan mucho el riesgo de sangrado."},
	{"anticoagulantes", "aines", sevGrave, "un AINE con un anticoagulante aumenta el riesgo de sangrado."},
	{"warfarina", "metronidazol", sevGrave, "el metronidazol potencia el efecto de la warfarina (riesgo de sangrado)."},
	{"warfarina", "tmp_smx", sevGrave, "trimetoprima/sulfametoxazol potencia la warfarina (riesgo de sangrado)."},
	{"warfarina", "quinolonas", sevModerada, "las quinolonas pueden potenciar la warfarina: vigilar el INR."},
	{"warfarina", "macrolidos", sevModerada, "los macrólidos pueden potenciar la warfarina: vigilar el INR."},
	{"warfarina", "azoles", sevGrave, "los azoles (fluconazol, ketoconazol…) potencian la warfarina (riesgo de sangrado)."},
	{"warfarina", "amiodarona", sevGrave, "la amiodarona potencia la warfarina: reducir dosis y vigilar el INR."},
	{"warfarina", "inductores", sevModerada, "este fármaco puede reducir el efecto de la warfarina: vigilar el INR."},
	{"antiagregantes", "aines", sevModerada, "un AINE con un antiagregante aumenta el riesgo de sangrado."},
	{"aspirina", "antiagregantes", sevModerada, "doble antiagregación: aumenta el riesgo de sangrado."},
	{"isrs", "aines", sevModerada, "los ISRS junto con AINEs aumentan el riesgo de sangrado digestivo."},
	{"corticoides", "aines", sevModerada, "corticoide con AINE: mayor riesgo de úlcera y sangrado digestivo."},
	{"antiagregantes", "omeprazol_like", sevModerada, "omeprazol/esomeprazol pueden reducir el efecto del clopidogrel; considerar pantoprazol."},
	// duplicates and kidneys
	{"ketorolaco", "aines", sevGrave, "el ketorolaco no debe combinarse con otro AINE ni con aspirina."},
	{"aines", "aines", sevModerada, "dos AINEs a la vez: duplicidad terapéutica con más riesgo digestivo y renal."},
	{"aines", "ieca", sevModerada, "AINE con IECA: riesgo de daño renal y menor efecto antihipertensivo."},
	{"aines", "ara2", sevModerada, "AINE con ARA-II: riesgo de daño renal y menor efecto antihipertensivo."},
	{"aines", "diureticos", sevModerada, "AINE con diurético: menor efecto del diurético y riesgo renal."},
	{"aines", "metotrexato", sevGrave, "los AINEs elevan la toxicidad del metotrexato."},
	{"aines", "litio", sevGrave, "los AINEs elevan el nivel de litio (riesgo de toxicidad)."},
	{"tmp_smx", "metotrexato", sevGrave, "trimetoprima/sulfametoxazol con metotrexato: riesgo de toxicidad de la médula ósea."},
	{"ieca", "ahorradores_k", sevModerada, "IECA con diurético ahorrador de potasio: riesgo de hiperpotasemia."},
	{"ara2", "ahorradores_k", sevModerada, "ARA-II con diurético ahorrador de potasio: riesgo de hiperpotasemia."},
	{"ieca", "potasio", sevModerada, "IECA con suplemento de potasio: riesgo de hiperpotasemia."},
	{"ara2", "potasio", sevModerada, "ARA-II con suplemento de potasio: riesgo de hiperpotasemia."},
	{"ieca", "litio", sevGrave, "los IECA elevan el nivel de litio (riesgo de toxicidad)."},
	{"ara2", "litio", sevGrave, "los ARA-II elevan el nivel de litio (riesgo de toxicidad)."},
	{"diureticos", "litio", sevGrave, "los diuréticos elevan el nivel de litio (riesgo de toxicidad)."},
	// heart
	{"nitratos", "pde5", sevGrave, "nitratos con sildenafil, tadalafil o vardenafil: caída grave de la presión arterial. Contraindicado."},
	{"digoxina", "amiodarona", sevGrave, "la amiodarona eleva el nivel de digoxina: reducir dosis y vigilar."},
	{"digoxina", "macrolidos", sevModerada, "los macrólidos pueden elevar el nivel de digoxina."},
	{"digoxina", "calcioantag_nodhp", sevModerada, "verapamilo o diltiazem elevan el nivel de digoxina y pueden causar bradicardia."},
	{"betabloqueantes", "calcioantag_nodhp", sevGrave, "betabloqueante con verapamilo o diltiazem: riesgo de bradicardia y bloqueo cardiaco."},
	// brain and mood
	{"imao", "serotoninergicos", sevGrave, "un IMAO con un fármaco serotoninérgico: riesgo de síndrome serotoninérgico. Contraindicado."},
	{"isrs", "tramadol", sevGrave, "tramadol con ISRS: riesgo de síndrome serotoninérgico y de convulsiones."},
	{"isrs", "serotoninergicos", sevModerada, "dos fármacos serotoninérgicos: vigilar signos de síndrome serotoninérgico."},
	{"benzodiacepinas", "opioides", sevGrave, "benzodiacepina con opioide: riesgo de depresión respiratoria y sedación profunda."},
	// antibiotics and others
	{"simvastatina", "macrolidos", sevGrave, "claritromicina o eritromicina con simvastatina/lovastatina: riesgo de rabdomiólisis."},
	{"simvastatina", "azoles", sevGrave, "los azoles elevan la simvastatina/lovastatina: riesgo de rabdomiólisis."},
	{"estatinas", "inhibidores_3a4", sevModerada, "este inhibidor del CYP3A4 eleva el nivel de la estatina: riesgo de daño muscular."},
	{"simvastatina", "calcioantag_nodhp", sevModerada, "verapamilo o diltiazem elevan la simvastatina: limitar la dosis."},
	{"quinolonas", "cationes", sevModerada, "los antiácidos, el calcio, el hierro o el zinc reducen la absorción de la quinolona: separar 2 horas."},
	{"tetraciclinas", "cationes", sevModerada, "los antiácidos, el calcio, el hierro o el zinc reducen la absorción de la tetraciclina: separar 2-3 horas."},
	{"quinolonas", "teofilina", sevGrave, "las quinolonas elevan el nivel de teofilina (riesgo de toxicidad)."},
	{"quinolonas", "corticoides", sevModerada, "quinolona con corticoide: mayor riesgo de ruptura de tendón."},
	{"quinolonas", "sulfonilureas", sevModerada, "las quinolonas pueden alterar la glucosa con sulfonilureas (hipoglucemia)."},
	{"azoles", "sulfonilureas", sevModerada, "los azoles potencian las sulfonilureas (riesgo de hipoglucemia)."},
	{"tmp_smx", "sulfonilureas", sevModerada, "trimetoprima/sulfametoxazol potencia las sulfonilureas (riesgo de hipoglucemia)."},
	{"betabloqueantes", "insulina", sevModerada, "el betabloqueante puede ocultar los síntomas de hipoglucemia con insulina."},
	{"levotiroxina", "cationes", sevModerada, "el calcio, el hierro o los antiácidos reducen la absorción de levotiroxina: separar 4 horas."},
	{"levotiroxina", "omeprazol_like", sevModerada, "los inhibidores de bomba de protones pueden reducir la absorción de levotiroxina."},
	{"inductores", "anticonceptivos", sevGrave, "este fármaco reduce la eficacia de los anticonceptivos hormonales: riesgo de embarazo."},
	{"alopurinol", "azatioprina", sevGrave, "el alopurinol eleva mucho la toxicidad de la azatioprina."},
}

type interactionHit struct {
	Drug       string `json:"drug"`
	With       string `json:"with"`
	WithSource string `json:"with_source"` // «esta receta», «medicación crónica», «receta vigente #N»
	Severity   string `json:"severity"`
	Message    string `json:"message"`
}

// drugRef is a drug taking part in the check, and where it comes from.
type drugRef struct {
	Name   string
	Source string
}

func groupsOf(name string) map[string]bool {
	text := normText(name)
	out := map[string]bool{}
	if text == "" {
		return out
	}
	for g, stems := range drugGroups {
		for _, st := range stems {
			if stemMatch(text, st) {
				out[g] = true
				break
			}
		}
	}
	return out
}

// interactionsBetween looks for interactions of every drug of `fresh` with the other fresh drugs and with `existing`.
func interactionsBetween(fresh []string, existing []drugRef) []interactionHit {
	all := make([]drugRef, 0, len(fresh)+len(existing))
	for _, n := range fresh {
		all = append(all, drugRef{n, "esta receta"})
	}
	all = append(all, existing...)
	groups := make([]map[string]bool, len(all))
	for i, d := range all {
		groups[i] = groupsOf(d.Name)
	}
	var hits []interactionHit
	seen := map[string]bool{} // one alert per pair of drugs: the first rule (the most specific) wins
	for i := range fresh {
		for j := range all {
			if i == j || normText(all[i].Name) == normText(all[j].Name) {
				continue
			}
			if j < len(fresh) && j < i { // two new drugs: report the pair once
				continue
			}
			for _, r := range interactionRules {
				if !((groups[i][r.A] && groups[j][r.B]) || (groups[i][r.B] && groups[j][r.A])) {
					continue
				}
				if r.A == r.B && !(groups[i][r.A] && groups[j][r.A]) {
					continue
				}
				a, b := normText(all[i].Name), normText(all[j].Name)
				if a > b {
					a, b = b, a
				}
				key := a + "|" + b
				if seen[key] {
					continue
				}
				seen[key] = true
				hits = append(hits, interactionHit{all[i].Name, all[j].Name, all[j].Source, r.Severity, strings.ToUpper(r.Message[:1]) + r.Message[1:]})
			}
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].Severity == sevGrave && hits[b].Severity != sevGrave })
	return hits
}

func graveInteractions(hits []interactionHit) []interactionHit {
	var out []interactionHit
	for _, h := range hits {
		if h.Severity == sevGrave {
			out = append(out, h)
		}
	}
	return out
}
