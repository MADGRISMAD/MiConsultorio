package api_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

// Dictation (audio) and photo comparison send files to the model inside the request and nothing else is kept.
func TestAIDictationAndPhotoCompare(t *testing.T) {
	var prompt string
	var mimes []string
	fail := false
	reply := map[string]any{}
	gem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Contents []struct {
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mimes = nil
		for i, p := range body.Contents[0].Parts {
			if i == 0 {
				prompt = p["text"].(string)
				continue
			}
			mimes = append(mimes, p["inline_data"].(map[string]any)["mime_type"].(string))
		}
		if fail {
			w.WriteHeader(500)
			return
		}
		out, _ := json.Marshal(reply)
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(out)}}}}}})
	}))
	defer gem.Close()
	dir := t.TempDir()
	e := setupWith(t, func(c *config.Config) {
		c.GeminiAPIKey, c.GeminiModel, c.GeminiAPIBase = "gem-key", "test-model", gem.URL
		c.UploadsDir, c.MaxUploadBytes = dir, 1<<20
	})
	e.exec(`UPDATE clinics SET kind = 'DERMATOLOGY', specialties = '{}' WHERE id = $1`, e.clinicA)
	doc := e.login("doc_a")
	pid := sub(doc.expect(201, "POST", "/api/patients/", person(nil)), "patient")["id"].(string)
	used := func() int {
		var n int
		_ = e.pool.QueryRow(t.Context(), `SELECT coalesce(sum(used), 0) FROM magic_usage WHERE clinic_id = $1`, e.clinicA).Scan(&n)
		return n
	}

	// ---- dictation ----
	reply = map[string]any{"subjective": "Refiere picazón", "exam": "", "assessment": "Dermatitis", "plan": "Hidratante", "measures": []any{}}
	wav := append([]byte("RIFF\x00\x00\x00\x00WAVEfmt "), bytes.Repeat([]byte{1}, 3000)...)
	url := "/api/patients/" + pid + "/encounter-dictation/ai"
	good := map[string]any{"audio_base64": base64.StdEncoding.EncodeToString(wav), "mime": "audio/wav"}
	doc.expect(400, "POST", url, map[string]any{"audio_base64": good["audio_base64"], "mime": "video/mp4"})
	doc.expect(400, "POST", url, map[string]any{"audio_base64": "!!", "mime": "audio/wav"})
	doc.expect(400, "POST", url, map[string]any{"audio_base64": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 3000)), "mime": "audio/wav"}) // no WAV header
	if used() != 0 {
		t.Fatal("a rejected audio is not charged")
	}
	out := doc.expect(200, "POST", url, good)
	if out["subjective"] != "Refiere picazón" || len(mimes) != 1 || mimes[0] != "audio/wav" || !strings.Contains(prompt, "DICTÓ") || used() != 1 {
		t.Fatalf("dictation: %v / %v / %s", out, mimes, prompt)
	}
	fail = true
	doc.expect(502, "POST", url, good)
	fail = false
	if used() != 1 {
		t.Fatalf("a failed dictation is not charged: %d", used())
	}
	e.login("recep_a").expect(403, "POST", url, good)

	// ---- photo comparison ----
	reply = map[string]any{"title": "Comparación", "summary": "Sin cambios aparentes", "sections": []any{map[string]any{"heading": "Limitaciones", "items": []any{"Luz distinta"}}}, "text": "", "lines": []any{}}
	photo := func(title string, data []byte) string {
		st, o := doc.upload(pid, upload{name: title + ".jpg", data: data, fields: map[string]string{"kind": "photo", "title": title}})
		if st != 201 {
			t.Fatalf("upload: %d %v", st, o)
		}
		return sub(o, "file")["id"].(string)
	}
	a, b := photo("Antes", jpegBytes), photo("Después", append(jpegBytes, 'x'))
	doc2 := pid
	pdf := func() string {
		st, o := doc.upload(doc2, upload{name: "r.pdf", data: pdfBytes, fields: map[string]string{"kind": "lab", "title": "Estudio"}})
		if st != 201 {
			t.Fatalf("pdf: %d %v", st, o)
		}
		return sub(o, "file")["id"].(string)
	}()
	curl := "/api/patients/" + pid + "/ai/photo-compare"
	doc.expect(400, "POST", curl, map[string]any{"before": a, "after": a})
	doc.expect(400, "POST", curl, map[string]any{"before": a, "after": pdf})
	doc.expect(404, "POST", curl, map[string]any{"before": a, "after": "00000000-0000-0000-0000-000000000000"})
	before := used()
	out = doc.expect(200, "POST", curl, map[string]any{"before": a, "after": b, "zone": "Antebrazo"})
	if len(mimes) != 2 || mimes[0] != "image/jpeg" || !strings.Contains(prompt, "ANTES") || !strings.Contains(prompt, "NO diagnostiques") || out["disclaimer"] == "" || used() != before+1 {
		t.Fatalf("photo compare: %v / %v / %s", out, mimes, prompt)
	}
	fail = true
	doc.expect(502, "POST", curl, map[string]any{"before": a, "after": b})
	fail = false
	if used() != before+1 {
		t.Fatal("a failed comparison is not charged")
	}
	// another clinic's photo cannot be used, and a non-dermatology clinic does not get the helper
	e.login("doc_b").expect(404, "POST", "/api/patients/"+pid+"/ai/photo-compare", map[string]any{"before": a, "after": b})
	e.exec(`UPDATE clinics SET kind = 'GENERAL_MEDICAL' WHERE id = $1`, e.clinicA)
	doc.expect(404, "POST", curl, map[string]any{"before": a, "after": b})
}
