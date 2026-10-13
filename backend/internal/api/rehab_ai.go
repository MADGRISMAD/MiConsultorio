package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// AI helpers for the rehabilitation giros (physiotherapy, chiropractic, orthopedics). Everything the model writes is a
// SUGGESTION: the specialist evaluates, edits and decides. Numbers (pain and range of motion trends) are computed here,
// never by the model, and the model never sees the patient's name or contact data.

var rehabGiros = []string{"PHYSIOTHERAPY", "CHIROPRACTIC", "ORTHOPEDICS"}

// isRehabClinic: the clinic works in a rehabilitation giro and this person is not limited to other areas.
func (s *Server) isRehabClinic(r *http.Request, p *Principal) bool {
	kinds, err := s.clinicKindsFor(r.Context(), p.ClinicID)
	if err != nil {
		return false
	}
	for _, k := range kinds {
		if slices.Contains(rehabGiros, k) {
			return p.worksIn(rehabGiros...)
		}
	}
	return false
}

func (s *Server) rehabGate(w http.ResponseWriter, r *http.Request) (id string, ok bool) {
	id, subject, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return "", false
	}
	if subject != "person" {
		writeError(w, http.StatusBadRequest, "Esta ayuda es para pacientes personas.")
		return "", false
	}
	if !s.isRehabClinic(r, principalFrom(r.Context())) {
		writeError(w, http.StatusForbidden, "Esta ayuda es del área de rehabilitación.")
		return "", false
	}
	if !s.aiReady(w) {
		return "", false
	}
	return id, true
}

// ---- 1. exercise sheet draft ----

var exercisesAISchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"items": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "OBJECT", "properties": map[string]any{
			"name": map[string]any{"type": "STRING"}, "sets": map[string]any{"type": "STRING"}, "reps": map[string]any{"type": "STRING"}, "notes": map[string]any{"type": "STRING"},
		}, "required": []string{"name", "sets", "reps", "notes"}}},
		"frequency": map[string]any{"type": "STRING"},
		"notes":     map[string]any{"type": "STRING"},
	},
	"required": []string{"items", "frequency", "notes"},
}

type exercisesAIReq struct {
	Area         string `json:"area"`
	Diagnosis    string `json:"diagnosis"`
	Goal         string `json:"goal"`
	Level        string `json:"level"`
	Equipment    string `json:"equipment"`
	Restrictions string `json:"restrictions"`
	Count        int    `json:"count"`
}

type exerciseItem struct {
	Name  string `json:"name"`
	Sets  string `json:"sets"`
	Reps  string `json:"reps"`
	Notes string `json:"notes"`
}

func (s *Server) exercisesAI(w http.ResponseWriter, r *http.Request) {
	id, ok := s.rehabGate(w, r)
	if !ok {
		return
	}
	if !s.requireCedula(w, r, "generar hojas de ejercicios") {
		return
	}
	var in exercisesAIReq
	if !decode(w, r, &in) {
		return
	}
	for _, f := range []*string{&in.Area, &in.Diagnosis, &in.Goal, &in.Level, &in.Equipment, &in.Restrictions} {
		*f = strings.TrimSpace(*f)
		if utf8.RuneCountInString(*f) > 400 {
			writeError(w, http.StatusBadRequest, "Un texto es demasiado largo (máximo 400 caracteres).")
			return
		}
	}
	if in.Area == "" && in.Diagnosis == "" {
		writeError(w, http.StatusBadRequest, "Indica la zona a tratar o el diagnóstico.")
		return
	}
	if in.Count == 0 {
		in.Count = 6
	}
	in.Count = max(3, min(in.Count, 8))

	p := principalFrom(r.Context())
	var age *int
	var raw []byte
	if err := s.db.QueryRow(r.Context(), `SELECT CASE WHEN birth_date IS NULL THEN NULL ELSE date_part('year', age(birth_date))::int END, profile
		FROM patients WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id).Scan(&age, &raw); err != nil {
		serverError(w, r, err)
		return
	}
	prof, err := decProfile(id, raw)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var ctx []string
	add := func(label, v string) {
		if v != "" {
			ctx = append(ctx, label+": "+v)
		}
	}
	if age != nil {
		add("Edad", fmt.Sprintf("%d años", *age))
	}
	add("Zona a tratar", in.Area)
	add("Diagnóstico o motivo", in.Diagnosis)
	add("Objetivo", in.Goal)
	add("Nivel del paciente", in.Level)
	add("Material disponible", in.Equipment)
	add("Restricciones o precauciones indicadas por el especialista", in.Restrictions)
	add("Zona afectada (expediente)", profileText(prof, "affected_area"))
	add("Antecedentes de lesiones", truncateRunes(profileText(prof, "injury_history"), 400))
	add("Estudios de imagen", truncateRunes(profileText(prof, "imaging_studies"), 400))
	add("Postura laboral", truncateRunes(profileText(prof, "work_posture"), 200))
	add("Enfermedades crónicas", strings.Join(nonEmpty(profileText(prof, "chronic_conditions"), profileText(prof, "chronic_other")), "; "))

	prompt := "Eres un asistente para fisioterapeutas y especialistas en rehabilitación en México. Propón una hoja de ejercicios domiciliarios como BORRADOR SUGERIDO que el especialista revisará, corregirá y aprobará. " +
		fmt.Sprintf("Propón exactamente %d ejercicios seguros y comunes, ordenados de menor a mayor exigencia. ", in.Count) +
		"Para cada uno: name (nombre claro), sets (series, p. ej. \"3\"), reps (repeticiones o tiempo, p. ej. \"12\" o \"30 s\") y notes (una indicación breve de técnica o precaución). " +
		"frequency: con qué frecuencia hacer la rutina (p. ej. \"1 vez al día, 5 días por semana\"). notes: 1 o 2 oraciones con avisos generales (suspender si aumenta el dolor, etc.). " +
		"Respeta estrictamente las restricciones indicadas; no propongas ejercicios que las contradigan, ni dosis de medicamentos, ni diagnósticos. Responde en español.\n\nDATOS (sin nombre):\n" + strings.Join(ctx, "\n")

	if err := s.spendMagic(r.Context(), p); err != nil {
		writeFailure(w, r, err)
		return
	}
	rawOut, err := s.gemini(r.Context(), prompt, nil, "", exercisesAISchema)
	var out struct {
		Items     []exerciseItem `json:"items"`
		Frequency string         `json:"frequency"`
		Notes     string         `json:"notes"`
	}
	if err == nil && json.Unmarshal(rawOut, &out) != nil {
		err = fmt.Errorf("respuesta inválida")
	}
	items := []exerciseItem{}
	for _, it := range out.Items {
		it.Name = truncateRunes(strings.TrimSpace(it.Name), 120)
		if it.Name == "" || len(items) >= in.Count {
			continue
		}
		it.Sets, it.Reps, it.Notes = truncateRunes(strings.TrimSpace(it.Sets), 30), truncateRunes(strings.TrimSpace(it.Reps), 30), truncateRunes(strings.TrimSpace(it.Notes), 300)
		items = append(items, it)
	}
	if err == nil && len(items) == 0 {
		err = fmt.Errorf("sin ejercicios")
	}
	if err != nil {
		s.refundMagic(r.Context(), p)
		logf(r, "exercises ai: %v", err)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA no pudo proponer ejercicios. Intenta de nuevo en un momento."})
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "ai_exercises", "Pidió una sugerencia de ejercicios con IA", map[string]any{"patient_id": id})
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "frequency": truncateRunes(strings.TrimSpace(out.Frequency), 120),
		"notes": truncateRunes(strings.TrimSpace(out.Notes), 600), "disclaimer": aiDisclaimerDraft})
}

// ---- evolution trend (computed here) and the advance suggestion ----

type rehabPoint struct {
	Date string
	EVA  *float64
	ROM  *float64
	Note string
}

// rehabSeries reads the pain/ROM measures of the encounters (any order) into a chronological series.
func rehabSeries(encs []encounter) []rehabPoint {
	var pts []rehabPoint
	for _, e := range encs {
		if e.Hidden || e.Private {
			continue
		}
		pt := rehabPoint{Date: e.OccurredAt.Format("2006-01-02"), EVA: fnum(e.Measures["pain_eva"]), ROM: fnum(e.Measures["rom_deg"])}
		pt.Note, _ = e.Measures["rom_note"].(string)
		if pt.EVA != nil || pt.ROM != nil {
			pts = append(pts, pt)
		}
	}
	sort.SliceStable(pts, func(i, j int) bool { return pts[i].Date < pts[j].Date })
	return pts
}

// rehabTrendText describes the series in numbers: first and last value, the change and the number of sessions.
func rehabTrendText(sessions int, pts []rehabPoint) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Sesiones registradas: %d.\n", sessions)
	line := func(label, unit string, get func(rehabPoint) *float64) {
		var first, last *rehabPoint
		var fv, lv float64
		n := 0
		for i := range pts {
			if v := get(pts[i]); v != nil {
				if first == nil {
					first, fv = &pts[i], *v
				}
				last, lv = &pts[i], *v
				n++
			}
		}
		switch {
		case n == 0:
		case n == 1:
			fmt.Fprintf(&b, "%s: una sola medición, %g%s (%s).\n", label, lv, unit, last.Date)
		default:
			fmt.Fprintf(&b, "%s: de %g%s (%s) a %g%s (%s); cambio %+g%s en %d mediciones.\n", label, fv, unit, first.Date, lv, unit, last.Date, math.Round((lv-fv)*100)/100, unit, n)
		}
	}
	line("Dolor EVA (0 a 10)", "", func(p rehabPoint) *float64 { return p.EVA })
	line("Movilidad (ROM)", "°", func(p rehabPoint) *float64 { return p.ROM })
	return b.String()
}

// latestExercises lists the items of the latest exercise sheet of the patient (empty when there is none).
func (s *Server) latestExercises(r *http.Request, clinicID, patientID string) string {
	var raw []byte
	if err := s.db.QueryRow(r.Context(), `SELECT data FROM patient_charts WHERE clinic_id=$1 AND patient_id=$2 AND kind='exercises' ORDER BY created_at DESC LIMIT 1`, clinicID, patientID).Scan(&raw); err != nil {
		return ""
	}
	var d struct {
		Items     []exerciseItem `json:"items"`
		Frequency string         `json:"frequency"`
	}
	if json.Unmarshal(raw, &d) != nil {
		return ""
	}
	var parts []string
	for i, it := range d.Items {
		if i >= 12 {
			break
		}
		parts = append(parts, strings.TrimSpace(fmt.Sprintf("%s %sx%s", truncateRunes(it.Name, 80), it.Sets, it.Reps)))
	}
	if len(parts) == 0 {
		return ""
	}
	out := strings.Join(parts, "; ")
	if d.Frequency != "" {
		out += " (" + truncateRunes(d.Frequency, 80) + ")"
	}
	return out
}

// rehabEvolutionBlock is added to the consult summary of rehab giros so the model reports computed trends.
func (s *Server) rehabEvolutionBlock(r *http.Request, p *Principal, patientID string, notes []encounter) string {
	if !s.isRehabClinic(r, p) {
		return ""
	}
	pts := rehabSeries(notes)
	txt := rehabTrendText(len(notes), pts)
	if ex := s.latestExercises(r, p.ClinicID, patientID); ex != "" {
		txt += "Hoja de ejercicios vigente: " + ex + "\n"
	}
	return "\nEVOLUCIÓN CALCULADA POR EL SISTEMA (cifras exactas; repórtalas tal cual en key_facts, sin recalcular):\n" + txt
}

var rehabAdviceSchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"stance":  map[string]any{"type": "STRING", "enum": []string{"continuar", "ajustar", "valorar_alta"}},
		"summary": map[string]any{"type": "STRING"},
		"reasons": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
		"verify":  map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
	},
	"required": []string{"stance", "summary", "reasons", "verify"},
}

func (s *Server) rehabAdviceAI(w http.ResponseWriter, r *http.Request) {
	id, ok := s.rehabGate(w, r)
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	encs, err := s.visibleEncounters(r.Context(), p, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var notes []encounter
	for _, e := range encs {
		if !e.Hidden && !e.Private {
			notes = append(notes, e)
		}
	}
	if len(notes) < 2 {
		writeError(w, http.StatusBadRequest, "Se necesitan al menos 2 sesiones registradas para sugerir el avance.")
		return
	}
	sort.SliceStable(notes, func(i, j int) bool { return notes[i].OccurredAt.Before(notes[j].OccurredAt) })
	// the numbers come from every visible session; only the text sent to the model is capped to the latest ones
	trend := rehabTrendText(len(notes), rehabSeries(notes))
	recent := notes
	if len(recent) > 10 {
		recent = recent[len(recent)-10:]
	}

	var b strings.Builder
	b.WriteString(trend)
	if ex := s.latestExercises(r, p.ClinicID, id); ex != "" {
		b.WriteString("Hoja de ejercicios vigente: " + ex + "\n")
	}
	b.WriteString("\nSESIONES (de la más antigua a la más reciente):\n")
	for _, e := range recent {
		fmt.Fprintf(&b, "[%s] Motivo: %s | Subjetivo: %s | Exploración: %s | Valoración: %s | Plan: %s\n", e.OccurredAt.Format("2006-01-02"),
			truncateRunes(e.Reason, 150), truncateRunes(e.Subjective, 300), truncateRunes(e.Exam, 300), truncateRunes(e.Assessment, 300), truncateRunes(e.Plan, 300))
	}
	prompt := "Eres un asistente para especialistas en rehabilitación en México. Con la evolución registrada, ofrece una SUGERENCIA (nunca una decisión) sobre el rumbo del tratamiento; el especialista es quien valora al paciente. " +
		"Usa SOLO los datos que siguen y las cifras ya calculadas; no inventes mediciones ni diagnósticos. " +
		"stance: continuar (hay mejoría y conviene mantener el plan), ajustar (poca o nula mejoría, o empeora: conviene revisar el plan) o valorar_alta (la evolución permitiría considerar el alta o espaciar sesiones). " +
		"Redacta con lenguaje de posibilidad (\"Podría considerarse…\", \"Los datos sugieren…\"), nunca en tono de orden. summary: 2 oraciones. reasons: hasta 4 datos concretos que sustentan la sugerencia. " +
		"verify: de 2 a 4 cosas que el especialista debe comprobar en su valoración clínica antes de decidir (por ejemplo banderas rojas, funcionalidad, adherencia a los ejercicios). Español.\n\nDATOS:\n" + b.String()

	if err := s.spendMagic(r.Context(), p); err != nil {
		writeFailure(w, r, err)
		return
	}
	rawOut, err := s.gemini(r.Context(), prompt, nil, "", rehabAdviceSchema)
	var out struct {
		Stance  string   `json:"stance"`
		Summary string   `json:"summary"`
		Reasons []string `json:"reasons"`
		Verify  []string `json:"verify"`
	}
	if err == nil && (json.Unmarshal(rawOut, &out) != nil || strings.TrimSpace(out.Summary) == "") {
		err = fmt.Errorf("respuesta incompleta")
	}
	if err != nil {
		s.refundMagic(r.Context(), p)
		logf(r, "rehab advice ai: %v", err)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA no pudo preparar la sugerencia. Intenta de nuevo en un momento."})
		return
	}
	switch out.Stance {
	case "continuar", "ajustar", "valorar_alta":
	default:
		out.Stance = "continuar"
	}
	verify := cleanLines(out.Verify, 4)
	if len(verify) == 0 {
		verify = []string{"Tu valoración clínica del paciente en consulta."}
	}
	audit(r.Context(), s.db, p.ClinicID, p, "ai_rehab_advice", "Pidió una sugerencia de avance con IA", map[string]any{"patient_id": id})
	writeJSON(w, http.StatusOK, map[string]any{
		"suggestion": map[string]any{"stance": out.Stance, "summary": truncateRunes(strings.TrimSpace(out.Summary), 600), "reasons": cleanLines(out.Reasons, 4), "verify": verify},
		"trend":      trend, "disclaimer": aiDisclaimerSuggestion, "generated_at": time.Now().UTC(), "based_on": len(recent),
	})
}
