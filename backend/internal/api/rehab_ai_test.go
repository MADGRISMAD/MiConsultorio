package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

// The AI helpers of the rehabilitation giros: exercise sheet draft, ordered session note, evolution and the advance
// suggestion. They are drafts/suggestions: one magic use each, given back on failure, and the disclaimer is always forced.
func TestRehabAI(t *testing.T) {
	fail := false
	var prompts []string
	var reply map[string]any
	gem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Contents []struct {
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		prompts = append(prompts, body.Contents[0].Parts[0]["text"].(string))
		if fail {
			w.WriteHeader(500)
			return
		}
		out, _ := json.Marshal(reply)
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(out)}}}}}})
	}))
	defer gem.Close()
	e := setupWith(t, func(c *config.Config) {
		c.GeminiAPIKey, c.GeminiModel, c.GeminiAPIBase = "gem-key", "test-model", gem.URL
	})
	e.exec(`UPDATE clinics SET kind = 'PHYSIOTHERAPY' WHERE id = $1`, e.clinicA)
	doc := e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	used := func() int {
		var n int
		_ = e.pool.QueryRow(t.Context(), `SELECT coalesce(sum(used), 0) FROM magic_usage WHERE clinic_id = $1`, e.clinicA).Scan(&n)
		return n
	}
	base := "/api/patients/" + pid

	// exercise sheet: needs the cédula and a zone or diagnosis
	reply = map[string]any{"items": []any{
		map[string]any{"name": "Puente de glúteo", "sets": "3", "reps": "12", "notes": "Sin arquear la espalda"},
		map[string]any{"name": " ", "sets": "1", "reps": "1", "notes": ""},
	}, "frequency": "1 vez al día", "notes": "Suspende si aumenta el dolor."}
	doc.expect(409, "POST", base+"/exercises/ai", map[string]any{"area": "Lumbar"})
	e.exec(`UPDATE users SET cedula = '1234567' WHERE id = $1`, e.userID("doc_a"))
	doc.expect(400, "POST", base+"/exercises/ai", map[string]any{})
	before := used()
	out := doc.expect(200, "POST", base+"/exercises/ai", map[string]any{"area": "Lumbar", "restrictions": "evitar flexión con carga", "equipment": "banda elástica"})
	if items := out["items"].([]any); len(items) != 1 || items[0].(map[string]any)["name"] != "Puente de glúteo" || out["disclaimer"] == "" {
		t.Fatalf("exercises: %v", out)
	}
	if !strings.Contains(prompts[0], "evitar flexión con carga") || !strings.Contains(prompts[0], "banda elástica") || used() != before+1 {
		t.Fatalf("prompt/use: %s / %d", prompts[0], used()-before)
	}
	fail = true
	doc.expect(502, "POST", base+"/exercises/ai", map[string]any{"area": "Lumbar"})
	if used() != before+1 {
		t.Fatalf("failed draft is not charged: %d", used()-before)
	}
	fail = false

	// loose notes -> note; measures only when explicit and in range
	reply = map[string]any{"subjective": "Dolor lumbar al sentarse", "exam": "", "assessment": "Contractura", "plan": "Calor local", "measures": []any{
		map[string]any{"key": "pain_eva", "value": "4"}, map[string]any{"key": "rom_deg", "value": "400"}, map[string]any{"key": "rom_note", "value": "Flexión lumbar"}, map[string]any{"key": "risk", "value": "Alto"}, map[string]any{"key": "inventada", "value": "1"}}}
	doc.expect(400, "POST", base+"/encounter-draft/ai", map[string]any{"notes": "corto"})
	out = doc.expect(200, "POST", base+"/encounter-draft/ai", map[string]any{"notes": "Refiere dolor lumbar al sentarse, EVA 4, flexión limitada."})
	ms := out["measures"].(map[string]any)
	if out["subjective"] != "Dolor lumbar al sentarse" || ms["pain_eva"].(float64) != 4 || ms["rom_deg"] != nil || ms["risk"] != nil || ms["inventada"] != nil || ms["rom_note"] != "Flexión lumbar" {
		t.Fatalf("draft: %v", out)
	}

	// advance suggestion: needs 2 sessions; the numbers come from the server; the disclaimer is forced
	reply = map[string]any{"stance": "inventada", "summary": "Podría considerarse espaciar las sesiones.", "reasons": []any{"Baja el dolor"}, "verify": []any{}}
	doc.expect(400, "POST", base+"/rehab-advice/ai", map[string]any{})
	doc.expect(201, "POST", base+"/encounters", map[string]any{"reason": "Lumbalgia", "measures": map[string]any{"pain_eva": 8, "rom_deg": 40}})
	doc.expect(201, "POST", base+"/encounters", map[string]any{"reason": "Control", "private": true, "measures": map[string]any{"pain_eva": 1}})
	doc.expect(201, "POST", base+"/encounters", map[string]any{"reason": "Control", "measures": map[string]any{"pain_eva": 3, "rom_deg": 70}})
	before = used()
	out = doc.expect(200, "POST", base+"/rehab-advice/ai", map[string]any{})
	sg := out["suggestion"].(map[string]any)
	last := prompts[len(prompts)-1]
	if sg["stance"] != "continuar" || len(sg["verify"].([]any)) == 0 || !strings.Contains(out["disclaimer"].(string), "sugerencia") || used() != before+1 {
		t.Fatalf("advice: %v", out)
	}
	if !strings.Contains(last, "de 8 ") || !strings.Contains(last, "a 3 ") || strings.Contains(last, "SECRET") {
		t.Fatalf("computed trend missing: %s", last)
	}
	// the consult summary of a rehab clinic carries the same computed block
	reply = map[string]any{"overview": "Mejora.", "key_facts": []any{}, "pending": []any{}, "watch_for": []any{}}
	doc.expect(200, "POST", base+"/ai-summary", map[string]any{})
	if !strings.Contains(prompts[len(prompts)-1], "EVOLUCIÓN CALCULADA") {
		t.Fatalf("summary lacks trend: %s", prompts[len(prompts)-1])
	}
	// other giros do not get these helpers
	e.exec(`UPDATE clinics SET kind = 'GENERAL_MEDICAL' WHERE id = $1`, e.clinicA)
	doc.expect(403, "POST", base+"/rehab-advice/ai", map[string]any{})
}
