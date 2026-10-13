package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

// Every AI task of the registry is exercised: it lists, it runs once (one magic use), the numbers computed by the server reach
// the model, a failure is not charged, a task of another giro is not available, and the disclaimer is always forced.

type aiCase struct {
	key     string
	kind    string // giro of the clinic
	animal  bool
	param   string
	setup   func(t *testing.T, e *env, doc *client, pid string)
	prompt  []string // fragments the model must have been given
	reply   map[string]any
	noNotes bool
}

func aiNote(t *testing.T, doc *client, pid string, body map[string]any) {
	t.Helper()
	if body["reason"] == nil {
		body["reason"] = "Consulta"
	}
	doc.expect(201, "POST", "/api/patients/"+pid+"/encounters", body)
}

func aiChart(t *testing.T, e *env, pid, kind string, data map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(data)
	e.exec(`INSERT INTO patient_charts (clinic_id, patient_id, kind, data, created_by_name) VALUES ($1, $2, $3, $4::jsonb, 'Doc')`, e.clinicA, pid, kind, string(raw))
}

func TestAITasksRegistry(t *testing.T) {
	var prompt string
	fail := false
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
	doc := e.login("doc_a")
	used := func() int {
		var n int
		_ = e.pool.QueryRow(t.Context(), `SELECT coalesce(sum(used), 0) FROM magic_usage WHERE clinic_id = $1`, e.clinicA).Scan(&n)
		return n
	}
	day := func(n int) string { return time.Now().AddDate(0, 0, n).Format("2006-01-02") }
	generic := map[string]any{"title": "Título", "summary": "Resumen de prueba", "sections": []any{map[string]any{"heading": "Punto", "items": []any{"uno", " "}}}, "text": "Hola [nombre], texto.", "lines": []any{}}

	cases := []aiCase{
		{key: "patient_message", kind: "GENERAL_MEDICAL", param: "Seguimiento", noNotes: true, prompt: []string{"Seguimiento", "[nombre]"}},
		{key: "patient_instructions", kind: "GENERAL_MEDICAL", prompt: []string{"Reposo tres días"}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiNote(t, doc, pid, map[string]any{"plan": "Reposo tres días"})
		}},
		{key: "visit_questions", kind: "INTERNAL_MEDICINE", prompt: []string{"TENDENCIAS DE MEDICIONES", "Peso", "de 80 ", "a 78 "}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiNote(t, doc, pid, map[string]any{"measures": map[string]any{"weight_kg": 80}})
			aiNote(t, doc, pid, map[string]any{"measures": map[string]any{"weight_kg": 78}})
		}},
		{key: "lab_trends", kind: "GENERAL_MEDICAL", noNotes: true, prompt: []string{"Glucosa", "de 90 ", "a 130 "}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			for i, v := range []float64{90, 130} {
				o := sub(doc.expect(201, "POST", "/api/patients/"+pid+"/lab/orders", map[string]any{"title": "Química", "results": []any{map[string]any{"analyte": "Glucosa", "value_num": v, "unit": "mg/dL", "ref_low": 70, "ref_high": 100, "resulted_at": time.Now().AddDate(0, -2+i, 0).Format(time.RFC3339)}}}), "order")
				_ = o
			}
		}},
		{key: "rx_draft", kind: "GENERAL_MEDICAL", param: "Faringitis", prompt: []string{"Faringitis", "Penicilina"},
			reply: map[string]any{"title": "Borrador", "summary": "Sugerencias", "sections": []any{}, "text": "", "lines": []any{map[string]any{"name": "Amoxicilina", "detail": "500 mg cada 8 h por 7 días", "qty": 1}}},
			setup: func(t *testing.T, e *env, doc *client, pid string) {
				aiNote(t, doc, pid, map[string]any{"assessment": "Faringitis"})
			}},
		{key: "icd_suggest", kind: "GENERAL_MEDICAL", param: "Resfriado común", noNotes: true, prompt: []string{"Resfriado común"},
			reply: map[string]any{"title": "Códigos", "summary": "Posibles", "sections": []any{}, "text": "", "lines": []any{map[string]any{"name": "j00", "detail": "x", "qty": 1}, map[string]any{"name": "ZZZ9", "detail": "inventado", "qty": 1}}}},
		{key: "dental_budget", kind: "DENTAL", prompt: []string{"Pieza 16", "caries", "Resina"},
			reply:   map[string]any{"title": "Presupuesto", "summary": "Prioridades", "sections": []any{}, "text": "", "lines": []any{map[string]any{"name": "resina compuesta", "detail": "16", "qty": 1}, map[string]any{"name": "Servicio inventado", "detail": "16", "qty": 1}}},
			noNotes: true, setup: func(t *testing.T, e *env, doc *client, pid string) {
				e.exec(`INSERT INTO catalog_items (clinic_id, kind, name, price_cents) VALUES ($1, 'service', 'Resina compuesta', 85000)`, e.clinicA)
				aiChart(t, e, pid, "odontogram", map[string]any{"dentition": "adult", "teeth": map[string]any{"16": map[string]any{"state": "caries", "surfaces": map[string]any{"O": "caries"}}}})
			}},
		{key: "perio_note", kind: "DENTAL", noNotes: true, prompt: []string{"Sitios sondeados: 2", "≥4 mm: 1", "Sangrado al sondaje: 1 de 2"}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiChart(t, e, pid, "periodontogram", map[string]any{"teeth": map[string]any{"16": map[string]any{"pd": []any{3, 5}, "rec": []any{0, 0}, "bop": []any{true, false}, "mob": 0, "furc": 0, "plaque": true}}, "plaque_pct": 40, "bleeding_pct": 50})
		}},
		{key: "post_procedure", kind: "DENTAL", param: "Extracción de molar", noNotes: true, prompt: []string{"Extracción de molar"}},
		{key: "growth_reading", kind: "PEDIATRICS", prompt: []string{"Edad actual", "Peso"}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiNote(t, doc, pid, map[string]any{"measures": map[string]any{"weight_kg": 12.5, "height_cm": 90}})
		}},
		{key: "recall_reading", kind: "NUTRITION", noNotes: true, prompt: []string{"Avena con fruta", "Agua: 1 litro"}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiChart(t, e, pid, "food_recall", map[string]any{"date": day(-1), "kind": "24h", "meals": []any{map[string]any{"name": "Desayuno", "time": "8:00", "items": "Avena con fruta"}}, "water": "1 litro", "notes": ""})
		}},
		{key: "nutrition_progress", kind: "NUTRITION", prompt: []string{"de 82 ", "a 79 "}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiNote(t, doc, pid, map[string]any{"measures": map[string]any{"weight_kg": 82}})
			aiNote(t, doc, pid, map[string]any{"measures": map[string]any{"weight_kg": 79}})
		}},
		{key: "session_summary", kind: "PSYCHOLOGY", param: "DAP", prompt: []string{"DAP", "Evaluación"}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiNote(t, doc, pid, map[string]any{"subjective": "Refiere insomnio"})
		}},
		{key: "goals_progress", kind: "PSYCHOLOGY", noNotes: true, prompt: []string{"objetivos en curso 1", "tareas hechas 1, pendientes 2 (vencidas 1)"}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiChart(t, e, pid, "therapy_plan", map[string]any{"goals": []any{map[string]any{"text": "Dormir mejor", "status": "active"}}, "tasks": []any{
				map[string]any{"text": "Diario de sueño", "due": day(-3), "done": false}, map[string]any{"text": "Respiración", "due": day(5), "done": false}, map[string]any{"text": "Caminar", "due": "", "done": true}}, "notes": ""})
		}},
		{key: "scale_trend", kind: "PSYCHOLOGY", noNotes: true, prompt: []string{"PHQ9"}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiChart(t, e, pid, "scale", map[string]any{"scale": "phq9", "answers": []any{1, 1, 1, 1, 1, 1, 1, 1, 1}, "score": 9, "severity": "Leve", "label": "Leve"})
		}},
		{key: "therapy_plan_draft", kind: "PSYCHOLOGY", prompt: []string{"Ansiedad laboral"}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiNote(t, doc, pid, map[string]any{"subjective": "Ansiedad laboral"})
			aiNote(t, doc, pid, map[string]any{"subjective": "Mejor sueño"})
		}},
		{key: "skin_routine", kind: "DERMATOLOGY", prompt: []string{"Acné leve"}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiNote(t, doc, pid, map[string]any{"assessment": "Acné leve"})
		}},
		{key: "prenatal_summary", kind: "GYNECOLOGY", noNotes: true, prompt: []string{"Semanas de gestación hoy (calculadas): 17+", "segundo"}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiChart(t, e, pid, "prenatal", map[string]any{"fum": day(-120), "fpp": day(160), "risk": "bajo"})
		}},
		{key: "prenatal_studies", kind: "GYNECOLOGY", noNotes: true, prompt: []string{"Semanas de gestación hoy", "ESTUDIOS REGISTRADOS"}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiChart(t, e, pid, "prenatal", map[string]any{"fum": day(-120), "fpp": day(160)})
		}},
		{key: "postop_rehab", kind: "ORTHOPEDICS", param: "Reconstrucción de LCA", noNotes: true, prompt: []string{"Reconstrucción de LCA", "Hoy es "}},
		{key: "vet_dose_check", kind: "VETERINARY", animal: true, param: "Meloxicam 2 mg", prompt: []string{"Peso más reciente: 10 kg", "0.20 mg/kg"}, setup: func(t *testing.T, e *env, doc *client, pid string) {
			aiNote(t, doc, pid, map[string]any{"measures": map[string]any{"weight_kg": 10}})
		}, noNotes: false},
		{key: "vet_schedule", kind: "VETERINARY", animal: true, noNotes: true, prompt: []string{"Esquema de referencia", "Rabia"}},
	}

	covered := map[string]bool{}
	for _, tc := range cases {
		covered[tc.key] = true
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			e.exec(`UPDATE clinics SET kind = $2, specialties = '{}' WHERE id = $1`, e.clinicA, tc.kind)
			var pid string
			if tc.animal {
				if err := e.pool.QueryRow(t.Context(), `INSERT INTO patients (clinic_id, file_number, subject, names, guardian_name, profile) VALUES ($1, (SELECT coalesce(max(file_number),0)+1 FROM patients WHERE clinic_id=$1), 'animal', 'Luna', 'Marta', '{"species":"Perro"}') RETURNING id`, e.clinicA).Scan(&pid); err != nil {
					t.Fatal(err)
				}
			} else {
				pid = sub(doc.expect(201, "POST", "/api/patients/", person(map[string]any{"curp": ""})), "patient")["id"].(string)
			}
			if tc.setup != nil {
				tc.setup(t, e, doc, pid)
			}
			base := "/api/patients/" + pid + "/ai/"
			// it is listed (and ready)
			found := false
			for _, x := range doc.expect(200, "GET", base+"tasks", nil)["tasks"].([]any) {
				if m := x.(map[string]any); m["key"] == tc.key {
					found = true
					if m["disabled"] != nil && m["disabled"] != "" && !tc.noNotes {
						t.Fatalf("not ready: %v", m)
					}
				}
			}
			if !found {
				t.Fatal("not listed for its giro")
			}
			reply = tc.reply
			if reply == nil {
				reply = generic
			}
			before := used()
			body := map[string]any{}
			if tc.param != "" {
				body["param"] = tc.param
			}
			out := doc.expect(200, "POST", base+tc.key, body)
			res := out["result"].(map[string]any)
			if out["disclaimer"] == "" || res["summary"] == nil || used() != before+1 {
				t.Fatalf("answer/use: %v / %d", out, used()-before)
			}
			for _, frag := range tc.prompt {
				if !strings.Contains(prompt, frag) {
					t.Fatalf("the model was not given %q:\n%s", frag, prompt)
				}
			}
			if strings.Contains(prompt, "Jorge") || strings.Contains(prompt, "Medina") || strings.Contains(prompt, "5512345678") {
				t.Fatalf("the patient's name or contact reached the model:\n%s", prompt)
			}
			// failure is not charged
			fail = true
			doc.expect(502, "POST", base+tc.key, body)
			fail = false
			if used() != before+1 {
				t.Fatalf("failed run charged: %d", used()-before)
			}
			// the same task in another giro is not available
			other := "GENERAL_MEDICAL"
			if tc.kind == other {
				other = "NUTRITION"
			}
			if tc.key != "patient_message" && tc.key != "patient_instructions" {
				e.exec(`UPDATE clinics SET kind = $2 WHERE id = $1`, e.clinicA, other)
				if code, _ := doc.do("POST", base+tc.key, body); code != 404 && tc.key != "visit_questions" && tc.key != "icd_suggest" && tc.key != "rx_draft" && tc.key != "lab_trends" {
					t.Fatalf("should be unavailable in %s: %d", other, code)
				}
			}
			// special post-processing
			switch tc.key {
			case "icd_suggest":
				lines := res["lines"].([]any)
				if len(lines) != 1 || lines[0].(map[string]any)["name"] != "J00" || lines[0].(map[string]any)["detail"] == "x" {
					t.Fatalf("only known codes survive, with the official name: %v", lines)
				}
			case "dental_budget":
				lines := res["lines"].([]any)
				if len(lines) != 1 || lines[0].(map[string]any)["unit_price_cents"].(float64) != 85000 || lines[0].(map[string]any)["catalog_item_id"] == "" {
					t.Fatalf("priced from the catalog, invented services dropped: %v", lines)
				}
			case "rx_draft":
				l := res["lines"].([]any)[0].(map[string]any)
				if l["alerts"] == nil {
					t.Fatalf("allergy alert missing: %v", l)
				}
			}
		})
	}
	// a new task without a test case fails here
	for _, k := range api.AITaskKeys() {
		if !covered[k] {
			t.Errorf("AI task %q has no test case", k)
		}
	}
}
