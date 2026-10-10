package api_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

func TestLabScanReadsAnyLayoutAndSavesNothing(t *testing.T) {
	var prompts []string
	var sawFile bool
	gem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Contents []struct {
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, p := range body.Contents[0].Parts {
			if s, ok := p["text"].(string); ok {
				prompts = append(prompts, s)
			}
			if _, ok := p["inline_data"]; ok {
				sawFile = true
			}
		}
		// rows in any order, mixed sections, a decimal comma, text and half-open ranges
		out, _ := json.Marshal(map[string]any{"study": "Química sanguínea", "lab_name": "Laboratorio Azul", "date": "2026-10-09", "results": []any{
			map[string]any{"section": "Química", "analyte": "Glucosa en ayuno", "value": "92,5", "unit": "mg/dL", "reference": "70 - 99"},
			map[string]any{"section": "Perfil de lípidos", "analyte": "Colesterol total", "value": "215", "unit": "mg/dL", "reference": "< 200", "flag": "alto"},
			map[string]any{"section": "Orina", "analyte": "Nitritos", "value": "Negativo", "unit": "", "reference": ""},
			map[string]any{"section": "Química", "analyte": "Glucosa en ayuno", "value": "92,5", "unit": "mg/dL", "reference": "70 - 99"}, // repeated
			map[string]any{"section": "Química", "analyte": "", "value": "5"},                                                             // no name
			map[string]any{"section": "Química", "analyte": "Vitamina D", "value": "> 30", "unit": "ng/mL", "reference": "> 30"},
		}})
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(out)}}}}}})
	}))
	defer gem.Close()
	e := setupWith(t, func(c *config.Config) {
		c.GeminiAPIKey, c.GeminiModel, c.GeminiAPIBase = "gem-key", "test-model", gem.URL
	})
	doc, recep := e.login("doc_a"), e.login("recep_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	url := "/api/patients/" + pid + "/lab/scan"
	png := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nfake"))

	recep.expect(403, "POST", url, map[string]any{"image_base64": png, "mime": "image/png"})
	e.anon().expect(401, "POST", url, map[string]any{"image_base64": png, "mime": "image/png"})
	doc.expect(400, "POST", url, map[string]any{"image_base64": png, "mime": "text/plain"})
	doc.expect(400, "POST", url, map[string]any{"image_base64": "!!", "mime": "image/png"})

	out := doc.expect(200, "POST", url, map[string]any{"image_base64": png, "mime": "image/png"})
	if !sawFile || len(prompts) != 1 || !strings.Contains(prompts[0], "sin importar su formato") || !strings.Contains(prompts[0], "Hemoglobina") {
		t.Fatalf("the model gets the file and the analyte names of the catalog: %v", prompts)
	}
	if strings.Contains(prompts[0], "Perez") || out["lab_name"] != "Laboratorio Azul" || out["date"] != "2026-10-09" || out["study"] != "Química sanguínea" {
		t.Fatalf("header: %v", out)
	}
	rows := out["results"].([]any)
	if len(rows) != 4 {
		t.Fatalf("duplicates and nameless rows are dropped: %v", rows)
	}
	glu := rows[0].(map[string]any)
	if glu["value_num"].(float64) != 92.5 || glu["ref_low"].(float64) != 70 || glu["ref_high"].(float64) != 99 {
		t.Fatalf("decimal comma and range: %v", glu)
	}
	chol := rows[1].(map[string]any)
	if chol["ref_high"].(float64) != 200 || chol["ref_low"] != nil {
		t.Fatalf("half-open range: %v", chol)
	}
	nit := rows[2].(map[string]any)
	if nit["value_num"] != nil || nit["value_text"] != "Negativo" {
		t.Fatalf("text result: %v", nit)
	}
	vit := rows[3].(map[string]any)
	if vit["value_num"] != nil || vit["value_text"] != "> 30" || vit["ref_low"].(float64) != 30 {
		t.Fatalf("value with a sign stays text: %v", vit)
	}
	// reading is not saving
	if n := len(doc.expect(200, "GET", "/api/patients/"+pid+"/lab/orders", nil)["orders"].([]any)); n != 0 {
		t.Fatalf("nothing is saved by a scan: %d orders", n)
	}
}

func TestLabScanNeedsThePlan(t *testing.T) {
	e := setup(t) // no Gemini key in this server
	doc := e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	png := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nfake"))
	doc.expect(503, "POST", "/api/patients/"+pid+"/lab/scan", map[string]any{"image_base64": png, "mime": "image/png"})
}

func TestLabOrderKeepsTheRequestedStudies(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")
	pid := newPerson(t, doc, "mejj700312hdfdrr04")
	base := "/api/patients/" + pid + "/lab/orders"
	o := sub(doc.expect(201, "POST", base, map[string]any{"title": "Chequeo", "requested": []string{"Biometría hemática", " Química sanguínea de 12 elementos ", "Biometría hemática", ""}}), "order")
	req := o["requested"].([]any)
	if len(req) != 2 || req[0] != "Biometría hemática" || req[1] != "Química sanguínea de 12 elementos" {
		t.Fatalf("requested is trimmed and without repeats: %v", req)
	}
	// editing the data without the field keeps what was asked
	up := sub(doc.expect(200, "PUT", labOrderURL(o["id"].(string)), map[string]any{"title": "Chequeo anual", "lab_name": "", "notes": "", "attachment_id": ""}), "order")
	if len(up["requested"].([]any)) != 2 {
		t.Fatalf("kept: %v", up["requested"])
	}
	none := sub(doc.expect(201, "POST", base, map[string]any{"title": "Sin lista"}), "order")
	if none["requested"] == nil || len(none["requested"].([]any)) != 0 {
		t.Fatalf("an order without a list has an empty one: %v", none["requested"])
	}
}
