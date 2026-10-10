package api

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The patient comes back from the laboratory with a printed or PDF report. Gemini reads it, whatever its layout or order, and
// proposes the rows of the capture screen. Nothing is saved: the professional reviews every value before saving (a wrong digit in a
// clinical result is not something to trust to a model). The patient's data is never sent: only the picture or PDF of the report.

var labScanSchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"study":    map[string]any{"type": "STRING"},
		"lab_name": map[string]any{"type": "STRING"},
		"date":     map[string]any{"type": "STRING"},
		"results": map[string]any{"type": "ARRAY", "items": map[string]any{
			"type": "OBJECT",
			"properties": map[string]any{
				"section":   map[string]any{"type": "STRING"},
				"analyte":   map[string]any{"type": "STRING"},
				"value":     map[string]any{"type": "STRING"},
				"unit":      map[string]any{"type": "STRING"},
				"reference": map[string]any{"type": "STRING"},
				"flag":      map[string]any{"type": "STRING"},
			},
			"required": []string{"analyte", "value"},
		}},
	},
	"required": []string{"results"},
}

type labScanRow struct {
	Section   string   `json:"section"`
	Analyte   string   `json:"analyte"`
	ValueNum  *float64 `json:"value_num"`
	ValueText string   `json:"value_text"`
	Unit      string   `json:"unit"`
	RefLow    *float64 `json:"ref_low"`
	RefHigh   *float64 `json:"ref_high"`
	Flag      string   `json:"flag"`
}

var (
	labNumRe  = regexp.MustCompile(`^[-+]?\d+(?:[.,]\d+)?$`)
	labRefRng = regexp.MustCompile(`^\s*([-+]?\d+(?:[.,]\d+)?)\s*(?:-|–|—|a|to)\s*([-+]?\d+(?:[.,]\d+)?)`)
	labRefMax = regexp.MustCompile(`^\s*(?:<|≤|<=|menor(?: que)?|hasta|máx\.?)\s*([-+]?\d+(?:[.,]\d+)?)`)
	labRefMin = regexp.MustCompile(`^\s*(?:>|≥|>=|mayor(?: que)?|desde|mín\.?)\s*([-+]?\d+(?:[.,]\d+)?)`)
)

func labFloat(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(s), ",", ".", 1), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e9 {
		return 0, false
	}
	return v, true
}

// labParseRef reads a printed reference such as "7 - 20", "< 100" or "> 40" into its bounds.
func labParseRef(s string) (low, high *float64) {
	s = strings.TrimSpace(s)
	if m := labRefRng.FindStringSubmatch(s); m != nil {
		a, okA := labFloat(m[1])
		b, okB := labFloat(m[2])
		if okA && okB && a <= b {
			return &a, &b
		}
		return nil, nil
	}
	if m := labRefMax.FindStringSubmatch(s); m != nil {
		if v, ok := labFloat(m[1]); ok {
			return nil, &v
		}
	}
	if m := labRefMin.FindStringSubmatch(s); m != nil {
		if v, ok := labFloat(m[1]); ok {
			return &v, nil
		}
	}
	return nil, nil
}

func (s *Server) scanLabResults(w http.ResponseWriter, r *http.Request) {
	if _, _, _, ok := s.labPatient(w, r); !ok {
		return
	}
	var req struct {
		Image string `json:"image_base64"`
		Mime  string `json:"mime"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 14<<20)
	if !decodeLoose(w, r, &req) {
		return
	}
	if req.Mime != "image/jpeg" && req.Mime != "image/png" && req.Mime != "image/webp" && req.Mime != "application/pdf" {
		writeError(w, http.StatusBadRequest, "Usa una foto JPG, PNG o WebP, o un PDF.")
		return
	}
	file, err := base64.StdEncoding.DecodeString(req.Image)
	if err != nil || len(file) == 0 {
		writeError(w, http.StatusBadRequest, "No se pudo leer el archivo.")
		return
	}
	if len(file) > 10<<20 {
		writeError(w, http.StatusBadRequest, "El archivo pesa demasiado (máximo 10 MB).")
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
	// The names of the catalog help the model write each analyte the way the system already knows it (so ranges and trends line up).
	names := []string{}
	for _, panel := range labCatalogPanels {
		for _, a := range panel.Analytes {
			names = append(names, a.Name)
		}
	}
	prompt := "Eres un asistente que transcribe reportes de resultados de laboratorio clínico (humano o veterinario) de México. " +
		"Lee el documento COMPLETO, sin importar su formato, columnas, orden, idioma ni si es foto, escaneo o PDF de varias páginas. " +
		"Extrae CADA resultado como una fila: section (el apartado o estudio al que pertenece, como aparece, por ejemplo «Biometría hemática»), analyte (nombre del análisis), " +
		"value (el resultado EXACTAMENTE como está impreso, incluidos signos como < o >, o texto como «Negativo»), unit (la unidad impresa, vacía si no hay), " +
		"reference (el rango de referencia impreso para ese análisis, tal cual; vacío si no aparece) y flag (si el documento marca el valor como alto, bajo o alterado: «alto», «bajo», «alterado»; vacío si no lo marca). " +
		"Además: study (nombre general del estudio o reporte), lab_name (el laboratorio que lo emite) y date (fecha del resultado o de toma de muestra en formato AAAA-MM-DD; vacío si no aparece). " +
		"Reglas: NO inventes ni calcules valores; si no se lee con seguridad un dato, déjalo vacío o omite la fila. No incluyas datos personales del paciente (nombre, fecha de nacimiento, folio). " +
		"No repitas filas. Si un análisis ya existe con este nombre en la lista, usa exactamente ese nombre: " + strings.Join(names, "; ") + ". Responde SOLO con JSON."
	raw, err := s.gemini(r.Context(), prompt, file, req.Mime, labScanSchema)
	if err != nil {
		s.refundMagic(r.Context(), p)
		logf(r, "lab scan: %v", err)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA no pudo leer el documento. Prueba con una foto más nítida o un PDF."})
		return
	}
	var parsed struct {
		Study   string `json:"study"`
		LabName string `json:"lab_name"`
		Date    string `json:"date"`
		Results []struct {
			Section   string `json:"section"`
			Analyte   string `json:"analyte"`
			Value     string `json:"value"`
			Unit      string `json:"unit"`
			Reference string `json:"reference"`
			Flag      string `json:"flag"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		s.refundMagic(r.Context(), p)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA respondió algo que no pudimos entender. Intenta de nuevo."})
		return
	}
	rows := []labScanRow{}
	seen := map[string]bool{}
	for _, x := range parsed.Results {
		analyte, value := strings.TrimSpace(x.Analyte), strings.TrimSpace(x.Value)
		if analyte == "" || value == "" || len(rows) >= 300 {
			continue
		}
		row := labScanRow{Section: truncateRunes(strings.TrimSpace(x.Section), 120), Analyte: truncateRunes(analyte, 120), Unit: truncateRunes(strings.TrimSpace(x.Unit), 30)}
		key := strings.ToLower(row.Section + "|" + row.Analyte)
		if seen[key] {
			continue
		}
		seen[key] = true
		if labNumRe.MatchString(value) {
			if v, ok := labFloat(value); ok {
				row.ValueNum = &v
			}
		}
		if row.ValueNum == nil {
			row.ValueText = truncateRunes(value, 200)
			switch strings.ToLower(strings.TrimSpace(x.Flag)) {
			case "alterado", "alto", "bajo", "anormal":
				row.Flag = "anormal"
			}
		}
		row.RefLow, row.RefHigh = labParseRef(x.Reference)
		rows = append(rows, row)
	}
	date := ""
	if t, err := time.Parse("2006-01-02", strings.TrimSpace(parsed.Date)); err == nil && t.Year() >= 1990 && !t.After(time.Now().Add(24*time.Hour)) {
		date = t.Format("2006-01-02")
	}
	audit(r.Context(), s.db, p.ClinicID, p, "lab_scan", "Leyó un reporte de laboratorio con IA", map[string]any{"rows": len(rows)})
	writeJSON(w, http.StatusOK, map[string]any{
		"study": truncateRunes(strings.TrimSpace(parsed.Study), 160), "lab_name": truncateRunes(strings.TrimSpace(parsed.LabName), 120), "date": date, "results": rows,
	})
}
