package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// Magic inventory and magic pricing: Gemini turns a pasted list or a photo of a catalog into
// products, and suggests public prices. Each call uses one "magic use" from the plan's monthly allowance.

func magicMonth() string { return time.Now().Format("2006-01") }

// spendMagic takes one use, or reports that the allowance is gone. It is refunded if the call fails.
func (s *Server) spendMagic(ctx context.Context, p *Principal) error {
	plan, _ := planByID(p.Billing.Plan)
	if plan.MagicUses == 0 {
		return fail(http.StatusForbidden, "Tu plan no incluye usos de magia.")
	}
	var used int
	err := s.db.QueryRow(ctx, `
		INSERT INTO magic_usage (clinic_id, month, used) VALUES ($1,$2,1)
		ON CONFLICT (clinic_id, month) DO UPDATE SET used = magic_usage.used + 1
		WHERE magic_usage.used < $3 RETURNING used`, p.ClinicID, magicMonth(), plan.MagicUses).Scan(&used)
	if err != nil {
		return &httpError{Status: http.StatusPaymentRequired, Code: "MAGIC_LIMIT", Msg: "Ya usaste los " + itoa(plan.MagicUses) + " usos de magia de este mes."}
	}
	return nil
}

func (s *Server) refundMagic(ctx context.Context, p *Principal) {
	_, _ = s.db.Exec(ctx, `UPDATE magic_usage SET used = greatest(used - 1, 0) WHERE clinic_id = $1 AND month = $2`, p.ClinicID, magicMonth())
}

func (s *Server) magicStatus(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	plan, _ := planByID(p.Billing.Plan)
	var used int
	_ = s.db.QueryRow(r.Context(), `SELECT used FROM magic_usage WHERE clinic_id=$1 AND month=$2`, p.ClinicID, magicMonth()).Scan(&used)
	writeJSON(w, http.StatusOK, map[string]any{"available": s.cfg.GeminiAPIKey != "", "limit": plan.MagicUses, "used": used})
}

type magicItem struct {
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Category   string  `json:"category"`
	PriceCents int     `json:"price_cents"`
	CostCents  int     `json:"cost_cents"`
	Stock      float64 `json:"stock"`
	Unit       string  `json:"unit"`
	Barcode    string  `json:"barcode"`
}

// gemini asks the model for JSON that follows a schema. image may be nil.
func (s *Server) gemini(ctx context.Context, prompt string, image []byte, mime string, schema map[string]any) ([]byte, error) {
	parts := []map[string]any{{"text": prompt}}
	if len(image) > 0 {
		parts = append(parts, map[string]any{"inline_data": map[string]any{"mime_type": mime, "data": base64.StdEncoding.EncodeToString(image)}})
	}
	body, _ := json.Marshal(map[string]any{
		"contents":         []map[string]any{{"role": "user", "parts": parts}},
		"generationConfig": map[string]any{"responseMimeType": "application/json", "responseSchema": schema, "temperature": 0.2},
	})
	endpoint := s.cfg.GeminiAPIBase + "/v1beta/models/" + url.PathEscape(s.cfg.GeminiModel) + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", s.cfg.GeminiAPIKey)
	res, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode >= 300 {
		return nil, errors.New("gemini: " + res.Status)
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &out); err != nil || len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return nil, errors.New("gemini: respuesta vacía")
	}
	return []byte(out.Candidates[0].Content.Parts[0].Text), nil
}

var magicItemSchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{"items": map[string]any{"type": "ARRAY", "items": map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"name": map[string]any{"type": "STRING"}, "kind": map[string]any{"type": "STRING", "enum": []string{"service", "product"}},
			"category": map[string]any{"type": "STRING"}, "price": map[string]any{"type": "NUMBER"}, "cost": map[string]any{"type": "NUMBER"},
			"stock": map[string]any{"type": "NUMBER"}, "unit": map[string]any{"type": "STRING"}, "barcode": map[string]any{"type": "STRING"},
		},
		"required": []string{"name", "kind"},
	}}},
	"required": []string{"items"},
}

// magicInventory reads a pasted list and/or a photo and proposes catalog items to review.
// Nothing is saved: the person confirms, then the items go through the normal import.
func (s *Server) magicInventory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text   string `json:"text"`
		Image  string `json:"image_base64"`
		Mime   string `json:"mime"`
		Format string `json:"business"` // free text: what kind of business, helps categories
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Solicitud inválida o imagen demasiado grande (máximo 5 MB).")
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	var image []byte
	if req.Image != "" {
		var err error
		if image, err = base64.StdEncoding.DecodeString(req.Image); err != nil || len(image) > 5<<20 {
			writeError(w, http.StatusBadRequest, "La imagen no es válida o pesa más de 5 MB.")
			return
		}
		if req.Mime != "image/jpeg" && req.Mime != "image/png" && req.Mime != "image/webp" {
			writeError(w, http.StatusBadRequest, "Usa una imagen JPG, PNG o WebP.")
			return
		}
	}
	if req.Text == "" && image == nil {
		writeError(w, http.StatusBadRequest, "Pega una lista o sube una foto.")
		return
	}
	if utf8.RuneCountInString(req.Text) > 20000 {
		writeError(w, http.StatusBadRequest, "La lista es demasiado larga; divídela en partes.")
		return
	}
	if s.cfg.GeminiAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "El Inventario Mágico no está configurado en el servidor (GEMINI_API_KEY)."})
		return
	}
	p := principalFrom(r.Context())
	if err := s.spendMagic(r.Context(), p); err != nil {
		writeFailure(w, r, err)
		return
	}
	prompt := "Eres el asistente de inventario de un consultorio o clínica en México. Extrae del texto o de la imagen los artículos que se venden: " +
		"servicios (consultas, estudios, procedimientos) y productos (medicamentos, insumos, artículos). Responde SOLO con JSON. " +
		"Precios y costos en pesos mexicanos (números, sin símbolo); si un dato no aparece usa 0. Categoría corta en español. Unidad: pza, caja, ml, etc. " +
		"No inventes artículos que no estén en la fuente. Los servicios no llevan existencias."
	if req.Text != "" {
		prompt += "\n\nTEXTO:\n" + req.Text
	}
	raw, err := s.gemini(r.Context(), prompt, image, req.Mime, magicItemSchema)
	if err != nil {
		s.refundMagic(r.Context(), p)
		logf(r, "magic inventory: %v", err)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA no pudo leer el contenido. Intenta con otra foto o una lista más clara."})
		return
	}
	var parsed struct {
		Items []struct {
			Name     string  `json:"name"`
			Kind     string  `json:"kind"`
			Category string  `json:"category"`
			Price    float64 `json:"price"`
			Cost     float64 `json:"cost"`
			Stock    float64 `json:"stock"`
			Unit     string  `json:"unit"`
			Barcode  string  `json:"barcode"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		s.refundMagic(r.Context(), p)
		writeJSON(w, http.StatusBadGateway, errorBody{Code: "PROVIDER", Message: "La IA respondió algo que no pudimos entender. Intenta de nuevo."})
		return
	}
	items := []magicItem{}
	for _, it := range parsed.Items {
		name := strings.TrimSpace(it.Name)
		if name == "" || len(items) >= 300 {
			continue
		}
		kind := it.Kind
		if kind != "service" && kind != "product" {
			kind = "product"
		}
		m := magicItem{Name: truncate(name, 160), Kind: kind, Category: truncate(strings.TrimSpace(it.Category), 60), Unit: truncate(strings.TrimSpace(it.Unit), 20),
			PriceCents: toCents(it.Price), CostCents: toCents(it.Cost), Barcode: truncate(strings.TrimSpace(it.Barcode), 60)}
		if kind == "product" && it.Stock > 0 && it.Stock < 1_000_000 {
			m.Stock = it.Stock
		}
		items = append(items, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func toCents(v float64) int {
	if math.IsNaN(v) || v < 0 || v > 1_000_000 {
		return 0
	}
	return int(math.Round(v * 100))
}

// magicPrice suggests public prices from costs. With a Gemini key it also reasons about the market;
// without one it still applies the target margin and rounds to friendly prices.
func (s *Server) magicPrice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MarginPct float64 `json:"margin_pct"`
		Items     []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			CostCents int    `json:"cost_cents"`
		} `json:"items"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.Items) == 0 || len(req.Items) > 100 {
		writeError(w, http.StatusBadRequest, "Elige entre 1 y 100 artículos.")
		return
	}
	if req.MarginPct <= 0 || req.MarginPct >= 90 {
		writeError(w, http.StatusBadRequest, "El margen debe estar entre 1 y 89 %.")
		return
	}
	for _, it := range req.Items {
		if !okCents(it.CostCents) || it.CostCents <= 0 {
			writeError(w, http.StatusBadRequest, "«"+truncate(it.Name, 40)+"» no tiene un costo válido.")
			return
		}
	}
	p := principalFrom(r.Context())
	type suggestion struct {
		ID         string `json:"id"`
		PriceCents int    `json:"price_cents"`
		Note       string `json:"note"`
	}
	out := make([]suggestion, len(req.Items))
	for i, it := range req.Items { // deterministic base: cost / (1 - margin), rounded up to a friendly price
		raw := float64(it.CostCents) / (1 - req.MarginPct/100)
		out[i] = suggestion{ID: it.ID, PriceCents: friendlyPrice(int(math.Ceil(raw))), Note: "Margen objetivo " + strings.TrimSuffix(strings.TrimSuffix(cents(int(req.MarginPct*100)), "0"), ".0") + " %"}
	}
	usedAI := false
	if s.cfg.GeminiAPIKey != "" {
		if err := s.spendMagic(r.Context(), p); err == nil {
			type in struct {
				I    int     `json:"i"`
				Name string  `json:"name"`
				Cost float64 `json:"cost"`
				Base float64 `json:"base"`
			}
			list := make([]in, len(req.Items))
			for i, it := range req.Items {
				list[i] = in{i, it.Name, float64(it.CostCents) / 100, float64(out[i].PriceCents) / 100}
			}
			payload, _ := json.Marshal(list)
			schema := map[string]any{"type": "OBJECT", "properties": map[string]any{"prices": map[string]any{"type": "ARRAY", "items": map[string]any{
				"type": "OBJECT", "properties": map[string]any{"i": map[string]any{"type": "INTEGER"}, "price": map[string]any{"type": "NUMBER"}, "note": map[string]any{"type": "STRING"}},
				"required": []string{"i", "price"}}}}, "required": []string{"prices"}}
			prompt := "Eres un asesor de precios para un consultorio o farmacia en México. Para cada artículo propón un precio al público en MXN " +
				"con IVA incluido que respete al menos el margen indicado (" + strings.TrimSpace(strings.TrimSuffix(cents(int(req.MarginPct*100)), ".00")) + " % sobre el precio) " +
				"y sea competitivo y fácil de cobrar (termina en 0, 5 o 9). `base` es el cálculo mínimo; no bajes de él. " +
				"Devuelve SOLO JSON con `prices` y, en `note`, una frase corta de por qué.\n" + string(payload)
			if raw, err := s.gemini(r.Context(), prompt, nil, "", schema); err == nil {
				var ai struct {
					Prices []struct {
						I     int     `json:"i"`
						Price float64 `json:"price"`
						Note  string  `json:"note"`
					} `json:"prices"`
				}
				if json.Unmarshal(raw, &ai) == nil {
					usedAI = true
					for _, pr := range ai.Prices {
						if pr.I >= 0 && pr.I < len(out) {
							if c := toCents(pr.Price); c >= out[pr.I].PriceCents && c <= out[pr.I].PriceCents*3 {
								out[pr.I].PriceCents, out[pr.I].Note = c, truncate(strings.TrimSpace(pr.Note), 120)
							}
						}
					}
				}
			}
			if !usedAI {
				s.refundMagic(r.Context(), p)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": out, "ai": usedAI})
}

// friendlyPrice rounds up to a price that is easy to charge: whole pesos under $100 end in 0, 5 or 9;
// higher prices round to 5 or 10 pesos.
func friendlyPrice(cents int) int {
	pesos := int(math.Ceil(float64(cents) / 100))
	switch {
	case pesos < 20:
		return pesos * 100
	case pesos < 100:
		return ((pesos + 4) / 5 * 5) * 100
	default:
		return ((pesos + 9) / 10 * 10) * 100
	}
}
