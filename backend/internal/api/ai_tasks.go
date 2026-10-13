package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
)

// A registry of AI tasks per patient. Each task reads what it needs (numbers computed by the server), asks the model for a
// common answer shape and hands it back as a DRAFT or a SUGGESTION: nothing is saved, the professional reviews and decides.
// Each run uses one magic use, given back when the model fails. The disclaimer is forced by the server.

const (
	aiDisclaimerSuggestion = "Es solo una sugerencia generada con IA a partir de los datos registrados. No sustituye la valoración clínica: el profesional evalúa y decide."
	aiDisclaimerDraft      = "Borrador generado con IA: revísalo y ajústalo antes de usarlo o entregarlo."
)

type aiParam struct {
	Label    string   `json:"label"`
	Options  []string `json:"options,omitempty"` // a closed list; empty = free text
	Required bool     `json:"required"`
	Hint     string   `json:"hint,omitempty"`
}

type aiTask struct {
	Key      string
	Label    string
	Desc     string
	Giros    []string // empty = every giro
	Subject  string   // "", "person" or "animal"
	MinNotes int
	Param    *aiParam
	Draft    bool // a text for the patient or the record (draft) instead of a clinical suggestion
	// Build returns the instructions for the model, the data it reads and, apart, the numbers computed by the system (shown
	// to the professional as the basis). A non-empty Problem stops the run before anything is spent.
	Build func(c *aiCtx, param string) aiPlan
	// Post adjusts the answer with data only the server has (catalog prices, alerts).
	Post func(c *aiCtx, out *aiOut)
}

type aiPlan struct {
	Instr, Data, Computed, Problem string
}

type aiSection struct {
	Heading string   `json:"heading"`
	Items   []string `json:"items"`
}

type aiLine struct {
	Name           string   `json:"name"`
	Detail         string   `json:"detail"`
	Qty            float64  `json:"qty"`
	CatalogItemID  string   `json:"catalog_item_id,omitempty"`
	UnitPriceCents int      `json:"unit_price_cents,omitempty"`
	TaxRate        float64  `json:"tax_rate,omitempty"`
	Alerts         []string `json:"alerts,omitempty"`
}

type aiOut struct {
	Title    string      `json:"title"`
	Summary  string      `json:"summary"`
	Sections []aiSection `json:"sections"`
	Text     string      `json:"text"`
	Lines    []aiLine    `json:"lines"`
	Notices  []string    `json:"notices,omitempty"`
	Extra    any         `json:"extra,omitempty"` // data only the server has (the patients to call back)
}

var aiOutSchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"title":   map[string]any{"type": "STRING"},
		"summary": map[string]any{"type": "STRING"},
		"sections": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "OBJECT", "properties": map[string]any{
			"heading": map[string]any{"type": "STRING"}, "items": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
		}, "required": []string{"heading", "items"}}},
		"text": map[string]any{"type": "STRING"},
		"lines": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "OBJECT", "properties": map[string]any{
			"name": map[string]any{"type": "STRING"}, "detail": map[string]any{"type": "STRING"}, "qty": map[string]any{"type": "NUMBER"},
		}, "required": []string{"name", "detail", "qty"}}},
	},
	"required": []string{"title", "summary", "sections", "text", "lines"},
}

func (o *aiOut) clean() {
	o.Title = truncateRunes(strings.TrimSpace(o.Title), 140)
	o.Summary = truncateRunes(strings.TrimSpace(o.Summary), 1200)
	o.Text = truncateRunes(strings.TrimSpace(o.Text), 4000)
	secs := []aiSection{}
	for _, s := range o.Sections {
		h := truncateRunes(strings.TrimSpace(s.Heading), 100)
		items := cleanLines(s.Items, 10)
		if h != "" && len(items) > 0 && len(secs) < 8 {
			secs = append(secs, aiSection{h, items})
		}
	}
	o.Sections = secs
	lines := []aiLine{}
	for _, l := range o.Lines {
		l.Name = truncateRunes(strings.TrimSpace(l.Name), 160)
		l.Detail = truncateRunes(strings.TrimSpace(l.Detail), 300)
		if l.Qty <= 0 || l.Qty > 999 {
			l.Qty = 1
		}
		if l.Name != "" && len(lines) < 20 {
			lines = append(lines, l)
		}
	}
	o.Lines = lines
}

func (o *aiOut) empty() bool {
	return o.Summary == "" && o.Text == "" && len(o.Sections) == 0 && len(o.Lines) == 0
}

var aiTasks []aiTask

func aiTaskByKey(k string) *aiTask {
	for i := range aiTasks {
		if aiTasks[i].Key == k {
			return &aiTasks[i]
		}
	}
	return nil
}

// available: the clinic works in one of the giros, this person is not limited to others, and the subject fits.
func (t *aiTask) available(c *aiCtx) bool {
	if t.Subject != "" && t.Subject != c.pat.Subject {
		return false
	}
	if len(t.Giros) == 0 {
		return true
	}
	return c.hasKind(t.Giros...) && c.p.worksIn(t.Giros...)
}

func (s *Server) newAICtx(w http.ResponseWriter, r *http.Request) (*aiCtx, bool) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return nil, false
	}
	p := principalFrom(r.Context())
	pat, err := loadPatient(r.Context(), s.db, p.ClinicID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return nil, false
	}
	kinds, err := s.clinicKindsFor(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return nil, false
	}
	return &aiCtx{s: s, r: r, p: p, id: id, pat: pat, kinds: kinds}, true
}

type aiTaskInfo struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Desc     string   `json:"desc"`
	Draft    bool     `json:"draft"`
	Param    *aiParam `json:"param,omitempty"`
	Disabled string   `json:"disabled,omitempty"` // why it cannot run yet
}

// aiTaskList: the tasks of this patient (by giro and subject) and which ones are ready.
func (s *Server) aiTaskList(w http.ResponseWriter, r *http.Request) {
	c, ok := s.newAICtx(w, r)
	if !ok {
		return
	}
	out := []aiTaskInfo{}
	for i := range aiTasks {
		t := &aiTasks[i]
		if !t.available(c) {
			continue
		}
		info := aiTaskInfo{Key: t.Key, Label: t.Label, Desc: t.Desc, Draft: t.Draft, Param: t.Param}
		if t.MinNotes > 0 && len(c.visibleNotes()) < t.MinNotes {
			info.Disabled = fmt.Sprintf("Necesita al menos %d consulta%s registrada%s.", t.MinNotes, map[bool]string{true: "s", false: ""}[t.MinNotes > 1], map[bool]string{true: "s", false: ""}[t.MinNotes > 1])
		}
		out = append(out, info)
	}
	if c.hasKind("DERMATOLOGY") && c.p.worksIn("DERMATOLOGY") && !c.isAnimal() {
		out = append(out, aiTaskInfo{Key: "photo_compare", Label: "Comparar fotos (antes y después)", Desc: "Describe en palabras neutrales qué cambió entre dos fotos de la piel. No diagnostica."})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": out})
}

func (s *Server) aiTaskRun(w http.ResponseWriter, r *http.Request) {
	c, ok := s.newAICtx(w, r)
	if !ok {
		return
	}
	t := aiTaskByKey(chi.URLParam(r, "task"))
	if t == nil || !t.available(c) {
		writeError(w, http.StatusNotFound, "Esta ayuda no está disponible para este paciente.")
		return
	}
	if s.cfg.GeminiAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "La IA no está configurada en el servidor (GEMINI_API_KEY)."})
		return
	}
	var in struct {
		Param string `json:"param"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Param = strings.TrimSpace(in.Param)
	if utf8.RuneCountInString(in.Param) > 400 {
		writeError(w, http.StatusBadRequest, "El texto es demasiado largo (máximo 400 caracteres).")
		return
	}
	if t.Param != nil {
		if t.Param.Required && in.Param == "" {
			writeError(w, http.StatusBadRequest, "Falta: "+strings.ToLower(t.Param.Label)+".")
			return
		}
		if len(t.Param.Options) > 0 && in.Param != "" && !slices.Contains(t.Param.Options, in.Param) {
			writeError(w, http.StatusBadRequest, "Elige una de las opciones.")
			return
		}
	}
	if t.MinNotes > 0 && len(c.visibleNotes()) < t.MinNotes {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Se necesitan al menos %d consultas registradas para esta ayuda.", t.MinNotes))
		return
	}
	plan := t.Build(c, in.Param)
	if plan.Problem != "" {
		writeError(w, http.StatusBadRequest, plan.Problem)
		return
	}
	prompt := aiPrompt(plan.Instr, c.header(), plan.Data)
	s.finishAI(w, r, c.p, c.id, t.Key, t.Label, t.Draft, prompt, plan.Computed, func(o *aiOut) {
		if t.Post != nil {
			t.Post(c, o)
		}
	})
}

// aiPrompt is the fixed frame of every AI task: a suggestion or draft for the professional, built only on the data.
func aiPrompt(instr, header, data string) string {
	return "Eres un asistente para profesionales de la salud en México. Todo lo que escribes es una SUGERENCIA o un BORRADOR que el profesional revisará, corregirá y decidirá; nunca una orden ni un diagnóstico. " +
		"Usa SOLO los datos que siguen; no inventes cifras, fechas, diagnósticos ni dosis, y si algo no consta no lo menciones. Las cifras ya calculadas por el sistema se citan tal cual, sin recalcular. " +
		"Con las sugerencias clínicas usa lenguaje de posibilidad (\"Podría considerarse…\", \"Los datos sugieren…\"). Español claro y conciso. " +
		"Responde con el JSON pedido: title (título corto), summary (resumen breve), sections (listas con encabezado), text (solo si la tarea pide un texto para copiar o imprimir; si no, vacío) y lines (solo si la tarea pide líneas; si no, lista vacía).\n\n" +
		"TAREA: " + instr + "\n\n" + header + "\nDATOS:\n" + data
}

// finishAI spends one magic use, asks the model, cleans the answer and writes it; the use is given back when it fails.
// patientID may be empty for the tasks of the clinic.
func (s *Server) finishAI(w http.ResponseWriter, r *http.Request, p *Principal, patientID, key, label string, draft bool, prompt, basis string, post func(*aiOut)) {
	s.finishAIBlobs(w, r, p, patientID, key, label, draft, prompt, nil, basis, post)
}

// finishAIBlobs is finishAI with files (photos) sent next to the prompt.
func (s *Server) finishAIBlobs(w http.ResponseWriter, r *http.Request, p *Principal, patientID, key, label string, draft bool, prompt string, blobs []geminiBlob, basis string, post func(*aiOut)) {
	disclaimer := aiDisclaimerSuggestion
	if draft {
		disclaimer = aiDisclaimerDraft
	}
	if err := s.spendMagic(r.Context(), p); err != nil {
		writeFailure(w, r, err)
		return
	}
	raw, err := s.geminiBlobs(r.Context(), prompt, blobs, aiOutSchema)
	var out aiOut
	if err == nil && json.Unmarshal(raw, &out) != nil {
		err = fmt.Errorf("respuesta inválida")
	}
	if err == nil {
		out.clean()
		if post != nil {
			post(&out)
		}
		if out.empty() {
			err = fmt.Errorf("respuesta vacía")
		}
	}
	if err != nil {
		s.refundMagic(r.Context(), p)
		logf(r, "ai task %s: %v", key, err)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA no pudo preparar esto. Intenta de nuevo en un momento."})
		return
	}
	meta := map[string]any{"task": key}
	if patientID != "" {
		meta["patient_id"] = patientID
	}
	audit(r.Context(), s.db, p.ClinicID, p, "ai_task", "Usó la ayuda de IA «"+label+"»", meta)
	writeJSON(w, http.StatusOK, map[string]any{"task": key, "label": label, "draft": draft, "result": out, "basis": basis, "disclaimer": disclaimer, "generated_at": time.Now().UTC()})
}
