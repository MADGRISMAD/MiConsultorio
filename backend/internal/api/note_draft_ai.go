package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Loose notes -> an ordered session note, for every giro. The model only reorders what the professional wrote: the
// boxes of the note and the measures of the giro that the notes state explicitly. It is a draft the professional reviews.

var encounterDraftSchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"subjective": map[string]any{"type": "STRING"}, "exam": map[string]any{"type": "STRING"},
		"assessment": map[string]any{"type": "STRING"}, "plan": map[string]any{"type": "STRING"},
		"measures": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "OBJECT", "properties": map[string]any{
			"key": map[string]any{"type": "STRING"}, "value": map[string]any{"type": "STRING"},
		}, "required": []string{"key", "value"}}},
	},
	"required": []string{"subjective", "exam", "assessment", "plan", "measures"},
}

// measures the model may never fill: judgments that belong to the professional.
var aiForbiddenMeasures = []string{"risk"}

func (s *Server) encounterDraftAI(w http.ResponseWriter, r *http.Request) {
	c, ok := s.newAICtx(w, r)
	if !ok {
		return
	}
	if s.cfg.GeminiAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "La IA no está configurada en el servidor (GEMINI_API_KEY)."})
		return
	}
	var in struct {
		Notes  string "json:\"notes\""
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Notes, in.Reason = strings.TrimSpace(in.Notes), strings.TrimSpace(in.Reason)
	if utf8.RuneCountInString(in.Notes) < 10 || utf8.RuneCountInString(in.Notes) > 3000 || utf8.RuneCountInString(in.Reason) > 300 {
		writeError(w, http.StatusBadRequest, "Escribe tus notas sueltas (de 10 a 3000 caracteres).")
		return
	}
	out, ok := s.draftNote(w, c, in.Notes, in.Reason, nil)
	if !ok {
		return
	}
	audit(r.Context(), s.db, c.p.ClinicID, c.p, "ai_note_draft", "Ordenó notas de sesión con IA", map[string]any{"patient_id": c.id})
	writeJSON(w, http.StatusOK, out)
}

// draftNote orders the text into the note and answers the failure itself. It is shared with the voice dictation. The magic use is spent here.
func (s *Server) draftNote(w http.ResponseWriter, c *aiCtx, notes, reason string, audio *geminiBlob) (map[string]any, bool) {
	fields := map[string]Field{}
	var list []string
	for _, f := range measureFields(c.pat.Subject, c.kinds) {
		if slices.Contains(aiForbiddenMeasures, f.Key) || (f.Type != "number" && f.Type != "select" && f.Type != "text") {
			continue
		}
		fields[f.Key] = f
		d := f.Key + " — " + f.Label
		if f.Unit != "" {
			d += " (" + f.Unit + ")"
		}
		if f.Type == "select" {
			d += " [una de: " + strings.Join(f.Options, ", ") + "]"
		}
		if f.Type == "text" {
			d += " [texto]"
		}
		list = append(list, d)
	}
	prompt := "Eres un asistente para profesionales de la salud en México. Ordena las notas sueltas del profesional en una nota de consulta como BORRADOR que él revisará. " +
		"Usa SOLO lo que dicen las notas: no inventes síntomas, hallazgos, diagnósticos ni indicaciones. Lo que no conste, déjalo como texto vacío. " +
		"subjective: lo que refiere el paciente (o el propietario). exam: exploración y hallazgos del profesional. assessment: valoración o impresión. plan: tratamiento realizado e indicaciones. " +
		"measures: lista de {key, value} SOLO de estas mediciones y SOLO si las notas dan el dato de forma explícita (números sin unidad; las de opciones, exactamente una opción):\n" + strings.Join(list, "\n") + "\nSi no hay ninguna, lista vacía. Español.\n"
	if reason != "" {
		prompt += "\nMOTIVO DE CONSULTA: " + reason
	}
	if audio != nil {
		prompt += "\n\n" + c.header() + "\nEl profesional DICTÓ sus notas: están en el audio adjunto. Transcríbelo mentalmente y ordénalo; ignora cualquier nombre propio de personas y no lo incluyas."
	} else {
		prompt += "\n\n" + c.header() + "\nNOTAS DEL PROFESIONAL:\n" + notes
	}

	if err := s.spendMagic(c.r.Context(), c.p); err != nil {
		writeFailure(w, c.r, err)
		return nil, false
	}
	var blobs []geminiBlob
	if audio != nil {
		blobs = append(blobs, *audio)
	}
	rawOut, err := s.geminiBlobs(c.r.Context(), prompt, blobs, encounterDraftSchema)
	var out struct {
		Subjective string "json:\"subjective\""
		Exam       string "json:\"exam\""
		Assessment string "json:\"assessment\""
		Plan       string "json:\"plan\""
		Measures   []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"measures"`
	}
	if err == nil && json.Unmarshal(rawOut, &out) != nil {
		err = fmt.Errorf("respuesta inválida")
	}
	if err == nil && strings.TrimSpace(out.Subjective+out.Exam+out.Assessment+out.Plan) == "" {
		err = fmt.Errorf("respuesta vacía")
	}
	if err != nil {
		s.refundMagic(c.r.Context(), c.p)
		logf(c.r, "note draft ai: %v", err)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA no pudo ordenar las notas. Intenta de nuevo en un momento."})
		return nil, false
	}
	measures := map[string]any{}
	for _, m := range out.Measures {
		f, ok := fields[m.Key]
		v := strings.TrimSpace(m.Value)
		if !ok || v == "" {
			continue
		}
		switch f.Type {
		case "number":
			n, err := strconv.ParseFloat(strings.Replace(v, ",", ".", 1), 64)
			if err != nil || n < 0 || n > 100000 || (m.Key == "pain_eva" && n > 10) || (m.Key == "rom_deg" && n > 360) {
				continue
			}
			measures[m.Key] = n
		case "select":
			if slices.Contains(f.Options, v) {
				measures[m.Key] = v
			}
		default:
			measures[m.Key] = truncateRunes(v, 200)
		}
	}
	return map[string]any{
		"subjective": truncateRunes(strings.TrimSpace(out.Subjective), 3000), "exam": truncateRunes(strings.TrimSpace(out.Exam), 3000),
		"assessment": truncateRunes(strings.TrimSpace(out.Assessment), 3000), "plan": truncateRunes(strings.TrimSpace(out.Plan), 3000),
		"measures": measures, "disclaimer": aiDisclaimerDraft,
	}, true
}

// encounterDictationAI: the professional dictates the note. The audio goes to the model inside this one request and is
// never stored; the answer is the same ordered draft as with typed notes. It costs one magic use.
func (s *Server) encounterDictationAI(w http.ResponseWriter, r *http.Request) {
	c, ok := s.newAICtx(w, r)
	if !ok {
		return
	}
	if s.cfg.GeminiAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "La IA no está configurada en el servidor (GEMINI_API_KEY)."})
		return
	}
	var in struct {
		Audio  string `json:"audio_base64"`
		Mime   string `json:"mime"`
		Reason string `json:"reason"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Solicitud inválida o audio demasiado largo (máximo 3 minutos).")
		return
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if utf8.RuneCountInString(in.Reason) > 300 || !slices.Contains([]string{"audio/wav", "audio/mpeg", "audio/ogg", "audio/flac", "audio/aac"}, in.Mime) {
		writeError(w, http.StatusBadRequest, "Formato de audio no admitido.")
		return
	}
	raw, err := base64.StdEncoding.DecodeString(in.Audio)
	if err != nil || len(raw) < 2000 || len(raw) > 6<<20 {
		writeError(w, http.StatusBadRequest, "El audio no es válido o dura más de 3 minutos.")
		return
	}
	if in.Mime == "audio/wav" && (len(raw) < 12 || string(raw[:4]) != "RIFF" || string(raw[8:12]) != "WAVE") {
		writeError(w, http.StatusBadRequest, "El audio no es válido.")
		return
	}
	out, ok := s.draftNote(w, c, "", in.Reason, &geminiBlob{Data: raw, Mime: in.Mime})
	if !ok {
		return
	}
	audit(r.Context(), s.db, c.p.ClinicID, c.p, "ai_dictation", "Dictó una nota con IA (el audio no se guarda)", map[string]any{"patient_id": c.id})
	writeJSON(w, http.StatusOK, out)
}
