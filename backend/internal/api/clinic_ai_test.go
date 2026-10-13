package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

func TestClinicAI(t *testing.T) {
	fail := false
	var prompt string
	var reply map[string]any
	gem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Contents []struct {
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		prompt = body.Contents[0].Parts[0]["text"].(string)
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
	admin, doc, recep := e.login("admin_a"), e.login("doc_a"), e.login("recep_a")
	used := func() int {
		var n int
		_ = e.pool.QueryRow(t.Context(), `SELECT coalesce(sum(used), 0) FROM magic_usage WHERE clinic_id = $1`, e.clinicA).Scan(&n)
		return n
	}
	reply = map[string]any{"title": "T", "summary": "Resumen", "sections": []any{map[string]any{"heading": "H", "items": []any{"x"}}}, "text": "Texto", "lines": []any{map[string]any{"name": "Amable", "detail": "Hola [nombre], te esperamos.", "qty": 1}}}

	// permissions: owner-level numbers are for the administrator
	doc.expect(403, "POST", "/api/ai/clinic/period-summary", map[string]any{"range": "semana"})
	recep.expect(403, "POST", "/api/ai/clinic/review-reply", map[string]any{"id": "x"})
	admin.expect(400, "POST", "/api/ai/clinic/period-summary", map[string]any{"range": "siempre"})

	before := used()
	out := admin.expect(200, "POST", "/api/ai/clinic/period-summary", map[string]any{"range": "semana"})
	if out["disclaimer"] == "" || used() != before+1 || !strings.Contains(prompt, "Periodo:") || !strings.Contains(prompt, "Pacientes que dejaron de venir") {
		t.Fatalf("period summary: %v / %s", out, prompt)
	}
	fail = true
	admin.expect(502, "POST", "/api/ai/clinic/period-summary", map[string]any{"range": "mes"})
	fail = false
	if used() != before+1 {
		t.Fatalf("a failed summary is not charged")
	}

	// patients who stopped coming: listed to the screen, never to the model
	admin.expect(400, "POST", "/api/ai/clinic/at-risk", map[string]any{}) // nobody yet
	e.exec(`INSERT INTO patients (clinic_id, file_number, subject, names, last_names, last_encounter_at, reminders_ok) VALUES ($1, 501, 'person', 'Ximena', 'Quintanilla', $2, true)`, e.clinicA, time.Now().AddDate(0, 0, -200))
	e.exec(`INSERT INTO patients (clinic_id, file_number, subject, names, last_names, last_encounter_at) VALUES ($1, 502, 'person', 'Reciente', 'Visita', $2)`, e.clinicA, time.Now().AddDate(0, 0, -10))
	out = recep.expect(200, "POST", "/api/ai/clinic/at-risk", map[string]any{})
	extra := out["result"].(map[string]any)["extra"].([]any)
	if len(extra) != 1 || extra[0].(map[string]any)["name"] != "Ximena Quintanilla" || extra[0].(map[string]any)["reminders_ok"] != true {
		t.Fatalf("at risk list: %v", out)
	}
	if strings.Contains(prompt, "Ximena") || strings.Contains(prompt, "Quintanilla") || !strings.Contains(prompt, "De 6 a 12 meses: 1") {
		t.Fatalf("the model must see counts, not names:\n%s", prompt)
	}

	// reply to a survey
	var appt, sid string
	_ = e.pool.QueryRow(t.Context(), `INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour) VALUES ($1, 'x', 'A', 'B', current_date, '10:00', '10:30') RETURNING id::text`, e.clinicA).Scan(&appt)
	if err := e.pool.QueryRow(t.Context(), `INSERT INTO satisfaction_surveys (clinic_id, appointment_id, token, rating, comment, answered_at) VALUES ($1, $2, 'tok-ai-1', 2, 'Esperé mucho tiempo', now()) RETURNING id::text`, e.clinicA, appt).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	admin.expect(404, "POST", "/api/ai/clinic/review-reply", map[string]any{"id": "00000000-0000-0000-0000-000000000000"})
	e.login("admin_b").expect(404, "POST", "/api/ai/clinic/review-reply", map[string]any{"id": sid})
	out = admin.expect(200, "POST", "/api/ai/clinic/review-reply", map[string]any{"id": sid})
	if out["result"].(map[string]any)["text"] != "Texto" || !strings.Contains(prompt, "Calificación: 2 de 5") || !strings.Contains(prompt, "Esperé mucho tiempo") {
		t.Fatalf("review reply: %v / %s", out, prompt)
	}

	// public page texts
	out = admin.expect(200, "POST", "/api/ai/clinic/public-page", map[string]any{})
	if !strings.Contains(prompt, "PROHIBIDO inventar") || out["draft"] != true {
		t.Fatalf("public page: %v", out)
	}
}
