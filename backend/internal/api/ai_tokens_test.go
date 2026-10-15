package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

// What the model bills is kept per clinic and month so the allowance can be compared with its real cost.
func TestAITokensAreRecordedPerClinic(t *testing.T) {
	gem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, _ := json.Marshal(map[string]any{"title": "T", "summary": "S", "sections": []any{}, "text": "x", "lines": []any{}})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"usageMetadata": map[string]any{"promptTokenCount": 1200, "candidatesTokenCount": 300},
			"candidates":    []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(out)}}}}},
		})
	}))
	defer gem.Close()
	e := setupWith(t, func(c *config.Config) {
		c.GeminiAPIKey, c.GeminiModel, c.GeminiAPIBase = "gem-key", "test-model", gem.URL
	})
	doc := e.login("doc_a")
	pid := newPerson(t, doc, "tokj700312hdfdrr04")
	doc.expect(200, "POST", "/api/patients/"+pid+"/ai/patient_message", map[string]any{"param": "Seguimiento"})
	doc.expect(200, "POST", "/api/patients/"+pid+"/ai/patient_message", map[string]any{"param": "Seguimiento"})

	var used int
	var in, out int64
	if err := e.pool.QueryRow(t.Context(), `SELECT sum(used), sum(prompt_tokens), sum(output_tokens) FROM magic_usage WHERE clinic_id = $1`, e.clinicA).Scan(&used, &in, &out); err != nil {
		t.Fatal(err)
	}
	if used != 2 || in != 2400 || out != 600 {
		t.Fatalf("used=%d prompt=%d output=%d", used, in, out)
	}

	// the platform admin sees it in the clinic's detail
	detail := e.login("root").expect(200, "GET", "/api/platform/clinics/"+e.clinicA, nil)
	months, _ := detail["ai_usage"].([]any)
	if len(months) != 1 || months[0].(map[string]any)["prompt_tokens"] != float64(2400) {
		t.Fatalf("ai_usage: %v", detail["ai_usage"])
	}
}
