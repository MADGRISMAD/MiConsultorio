package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// An AI summary of the patient's record for the specialist who is about to see them. It reads the last consultations,
// the chronic medication, the allergies and the latest recetas, and answers in a few lines. The model never sees the
// patient's name or contact data, and private notes are left out. Nothing is saved: it is a reading aid, one magic use.

var consultSummarySchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"overview":  map[string]any{"type": "STRING"},
		"key_facts": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
		"pending":   map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
		"watch_for": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
	},
	"required": []string{"overview", "key_facts", "pending", "watch_for"},
}

type consultSummary struct {
	Overview string   `json:"overview"`
	KeyFacts []string `json:"key_facts"`
	Pending  []string `json:"pending"`
	WatchFor []string `json:"watch_for"`
}

func cleanLines(in []string, max int) []string {
	out := []string{}
	for _, l := range in {
		if l = strings.TrimSpace(l); l != "" && len(out) < max {
			out = append(out, truncateRunes(l, 300))
		}
	}
	return out
}

func (s *Server) consultSummaryAI(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	if s.cfg.GeminiAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "La IA no está configurada en el servidor (GEMINI_API_KEY)."})
		return
	}
	p := principalFrom(r.Context())
	pat, err := loadPatient(r.Context(), s.db, p.ClinicID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	encs, err := s.visibleEncounters(r.Context(), p, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var notes []encounter
	for _, e := range encs { // newest first
		if !e.Hidden && !e.Private {
			notes = append(notes, e)
		}
	}
	if len(notes) == 0 {
		writeError(w, http.StatusBadRequest, "Aún no hay consultas registradas que resumir.")
		return
	}
	if len(notes) > 6 {
		notes = notes[:6]
	}
	sort.SliceStable(notes, func(i, j int) bool { return notes[i].OccurredAt.Before(notes[j].OccurredAt) })
	chronic, err := s.chronicOf(r.Context(), s.db, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	rxs, err := s.prescriptionsOf(r.Context(), p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}

	var b strings.Builder
	subject := "persona"
	if pat.Subject == "animal" {
		subject = "animal (paciente veterinario)"
	}
	fmt.Fprintf(&b, "Paciente: %s", subject)
	if pat.Age != nil {
		fmt.Fprintf(&b, ", %d años", *pat.Age)
	}
	if pat.Sex != "" {
		fmt.Fprintf(&b, ", sexo %s", pat.Sex)
	}
	b.WriteString("\n")
	if al := patientAllergies(pat.Profile); len(al) > 0 {
		b.WriteString("Alergias registradas: " + strings.Join(al, ", ") + "\n")
	}
	var meds []string
	for _, m := range chronic {
		if m.Active {
			meds = append(meds, strings.TrimSpace(m.Name+" "+m.Dose+" "+m.Frequency))
		}
	}
	if len(meds) > 0 {
		b.WriteString("Medicación crónica activa: " + strings.Join(meds, "; ") + "\n")
	}
	b.WriteString("\nCONSULTAS (de la más antigua a la más reciente):\n")
	for _, e := range notes {
		fmt.Fprintf(&b, "[%s · %s] Motivo: %s | Subjetivo: %s | Exploración: %s | Valoración: %s", e.OccurredAt.Format("2006-01-02"), e.Kind,
			truncateRunes(e.Reason, 200), truncateRunes(e.Subjective, 500), truncateRunes(e.Exam, 400), truncateRunes(e.Assessment, 400))
		if len(e.DiagnosisCodes) > 0 {
			b.WriteString(" | CIE-10: " + strings.Join(e.DiagnosisCodes, ", "))
		}
		fmt.Fprintf(&b, " | Plan: %s", truncateRunes(e.Plan, 500))
		if len(e.Measures) > 0 {
			ms, _ := json.Marshal(e.Measures)
			b.WriteString(" | Mediciones: " + string(ms))
		}
		if e.NextVisit != nil {
			b.WriteString(" | Siguiente consulta sugerida: " + *e.NextVisit)
		}
		b.WriteString("\n")
	}
	shown := 0
	for _, rx := range rxs {
		if rx.VoidedAt != nil || shown >= 3 {
			continue
		}
		var names []string
		for _, it := range rx.Items {
			names = append(names, strings.TrimSpace(it.Medicine+" "+it.Dose+" "+it.Frequency+" "+it.Duration))
		}
		if shown == 0 {
			b.WriteString("\nRECETAS RECIENTES:\n")
		}
		fmt.Fprintf(&b, "[%s] %s\n", rx.IssuedAt.Format("2006-01-02"), strings.Join(names, "; "))
		shown++
	}

	prompt := "Eres un asistente clínico para profesionales de la salud en México. Con el expediente que sigue, prepara un resumen breve para el especialista que va a atender al paciente hoy. " +
		"Usa SOLO lo que está escrito; no inventes diagnósticos, dosis ni datos, y si algo no consta no lo menciones. Escribe en español claro y conciso.\n" +
		"- overview: 2 o 3 oraciones con la historia y el estado actual.\n" +
		"- key_facts: hasta 6 datos clave (diagnósticos, tendencias de mediciones, tratamientos vigentes).\n" +
		"- pending: hasta 5 pendientes o seguimientos (estudios, controles, indicaciones por revisar).\n" +
		"- watch_for: hasta 4 cuidados a vigilar (alergias, interacciones, riesgos mencionados).\n" +
		"Las listas pueden quedar vacías.\n\nEXPEDIENTE:\n" + b.String()

	if err := s.spendMagic(r.Context(), p); err != nil {
		writeFailure(w, r, err)
		return
	}
	raw, err := s.gemini(r.Context(), prompt, nil, "", consultSummarySchema)
	var out consultSummary
	if err == nil && (json.Unmarshal(raw, &out) != nil || strings.TrimSpace(out.Overview) == "") {
		err = fmt.Errorf("respuesta incompleta")
	}
	if err != nil {
		s.refundMagic(r.Context(), p)
		logf(r, "consult summary ai: %v", err)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA no pudo preparar el resumen. Intenta de nuevo en un momento."})
		return
	}
	out.Overview = truncateRunes(strings.TrimSpace(out.Overview), 900)
	out.KeyFacts, out.Pending, out.WatchFor = cleanLines(out.KeyFacts, 6), cleanLines(out.Pending, 5), cleanLines(out.WatchFor, 4)
	audit(r.Context(), s.db, p.ClinicID, p, "ai_summary", "Generó un resumen de consulta con IA", map[string]any{"patient_id": id})
	writeJSON(w, http.StatusOK, map[string]any{"summary": out, "generated_at": time.Now().UTC(), "based_on": len(notes)})
}
