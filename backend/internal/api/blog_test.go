package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

func blogArticle(title, slug string) map[string]any {
	para := strings.Repeat("Una agenda ordenada ayuda a que el consultorio funcione mejor cada día. ", 12)
	body := []any{map[string]any{"type": "p", "text": para}}
	for _, h := range []string{"Por qué faltan los pacientes", "Cómo enviar recordatorios", "Qué medir cada mes", "Cómo lo resuelve Caresia"} {
		body = append(body,
			map[string]any{"type": "h2", "text": "## " + h},
			map[string]any{"type": "p", "text": para},
			map[string]any{"type": "ul", "items": []any{"Confirma un día antes", " ", "Ofrece reagendar"}},
			map[string]any{"type": "tip", "text": "Escribe el recordatorio en **tono amable**."},
			map[string]any{"type": "video", "text": "bloque desconocido"},
			map[string]any{"type": "p", "text": "   "},
		)
	}
	return map[string]any{
		"title": title, "slug": slug, "description": "Aprende a reducir las citas perdidas en tu consultorio con recordatorios y confirmaciones.",
		"excerpt": "Recordatorios que sí funcionan.", "keyword": "Recordatorios de Citas", "category": "Inventada",
		"body": body, "faqs": []any{map[string]any{"q": "¿Cuándo enviar el recordatorio?", "a": "Un día antes funciona bien."}, map[string]any{"q": "", "a": "sin pregunta"}},
	}
}

func TestBlog(t *testing.T) {
	var mu sync.Mutex
	var prompts []string
	reply := blogArticle("Recordatorios de citas que sí funcionan", "Recordatorios de Citas: guía práctica!")
	gem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Contents []struct {
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		prompts = append(prompts, body.Contents[0].Parts[0]["text"].(string)+"\nMODEL:"+r.URL.Path)
		out, _ := json.Marshal(reply)
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": string(out)}}}}}})
	}))
	defer gem.Close()
	e := setupWith(t, func(c *config.Config) {
		c.GeminiAPIKey, c.GeminiModel, c.GeminiAPIBase, c.BlogModel = "gem-key", "test-model", gem.URL, "blog-model"
		c.BlogAutopilot, c.BlogAutoPublish = true, true
	})
	root, help, admin, anon := e.login("root"), e.login("help"), e.login("admin_a"), e.anon()

	// only platform staff sees the panel, only administrators write
	admin.expect(403, "GET", "/api/platform/blog", nil)
	help.expect(403, "POST", "/api/platform/blog/generate", map[string]any{})
	panel := help.expect(200, "GET", "/api/platform/blog", nil)
	if panel["ai_ready"] != true || panel["autopilot"] != true || panel["next_run"] == nil || panel["next_topic"] != "recordatorios de citas para consultorio" {
		t.Fatalf("panel: %v", panel)
	}

	waitIdle := func() map[string]any {
		t.Helper()
		for i := 0; i < 200; i++ {
			p := root.expect(200, "GET", "/api/platform/blog", nil)
			if p["generating"] == false {
				return p
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Fatal("el artículo no terminó de escribirse")
		return nil
	}

	// a draft: saved, not public yet
	root.expect(202, "POST", "/api/platform/blog/generate", map[string]any{"publish": false})
	p := waitIdle()
	posts := p["posts"].([]any)
	if len(posts) != 1 || p["last_error"] != "" {
		t.Fatalf("draft: %v", p)
	}
	draft := posts[0].(map[string]any)
	if draft["status"] != "draft" || draft["slug"] != "recordatorios-de-citas-guia-practica" || draft["category"] != "Gestión" {
		t.Fatalf("draft fields: %v", draft)
	}
	mu.Lock()
	first := prompts[0]
	mu.Unlock()
	if !strings.Contains(first, "recordatorios de citas para consultorio") || !strings.Contains(first, "MODEL:/v1beta/models/blog-model:generateContent") || !strings.Contains(first, "NOM-004") {
		t.Fatalf("prompt: %s", first)
	}
	anon.expect(404, "GET", "/api/public/blog/recordatorios-de-citas-guia-practica", nil)
	if got := anon.expect(200, "GET", "/api/public/blog", nil)["posts"].([]any); len(got) != 0 {
		t.Fatalf("drafts are not public: %v", got)
	}

	// the team reads the full draft, then publishes it
	full := root.expect(200, "GET", "/api/platform/blog/"+draft["id"].(string), nil)["post"].(map[string]any)
	body := full["body"].([]any)
	for _, b := range body {
		bl := b.(map[string]any)
		text, _ := bl["text"].(string)
		if bl["type"] == "video" || (bl["type"] == "p" && strings.TrimSpace(text) == "") || strings.HasPrefix(text, "#") {
			t.Fatalf("malformed blocks are dropped: %v", bl)
		}
		if bl["type"] == "ul" && len(bl["items"].([]any)) != 2 {
			t.Fatalf("empty list items are dropped: %v", bl)
		}
	}
	if len(full["faqs"].([]any)) != 1 || full["keyword"] != "recordatorios de citas" {
		t.Fatalf("faqs/keyword: %v %v", full["faqs"], full["keyword"])
	}
	root.expect(204, "PATCH", "/api/platform/blog/"+draft["id"].(string), map[string]any{"status": "published"})
	pub := anon.expect(200, "GET", "/api/public/blog/recordatorios-de-citas-guia-practica", nil)
	if pub["post"].(map[string]any)["published_at"] == nil || pub["post"].(map[string]any)["topic"] != "" {
		t.Fatalf("public post: %v", pub)
	}

	// the next one takes the next topic of the bank, publishes on its own and gets a free slug
	root.expect(202, "POST", "/api/platform/blog/generate", map[string]any{"publish": true})
	waitIdle()
	mu.Lock()
	second := prompts[1]
	mu.Unlock()
	if !strings.Contains(second, "software para consultorio dental") || !strings.Contains(second, "Recordatorios de citas que sí funcionan") {
		t.Fatalf("second prompt must take the next topic and know the first title: %s", second[:400])
	}
	list := anon.expect(200, "GET", "/api/public/blog", nil)["posts"].([]any)
	if len(list) != 2 || list[0].(map[string]any)["slug"] != "recordatorios-de-citas-guia-practica-2" {
		t.Fatalf("public list: %v", list)
	}
	rel := anon.expect(200, "GET", "/api/public/blog/recordatorios-de-citas-guia-practica", nil)["related"].([]any)
	if len(rel) != 1 {
		t.Fatalf("related: %v", rel)
	}

	// a topic typed by the team
	root.expect(202, "POST", "/api/platform/blog/generate", map[string]any{"topic": "Cómo cobrar consultas de seguimiento", "publish": true})
	waitIdle()
	mu.Lock()
	if !strings.Contains(prompts[2], "Tema pedido por el equipo de Caresia: Cómo cobrar consultas de seguimiento") {
		t.Fatalf("custom topic prompt")
	}
	mu.Unlock()

	// an article that comes back too short is not saved and the panel says why
	mu.Lock()
	reply = map[string]any{"title": "Corto", "slug": "corto", "description": "d", "excerpt": "e", "keyword": "k", "category": "Legal", "body": []any{map[string]any{"type": "p", "text": "Muy corto."}}, "faqs": []any{}}
	mu.Unlock()
	root.expect(202, "POST", "/api/platform/blog/generate", map[string]any{"publish": true})
	if p := waitIdle(); !strings.Contains(p["last_error"].(string), "incompleto") || len(p["posts"].([]any)) != 3 {
		t.Fatalf("short article: %v", p["last_error"])
	}

	// hidden posts leave the site and the sitemap; deleted ones are gone
	res, err := http.Get(e.srv.URL + "/api/public/sitemap.xml")
	if err != nil {
		t.Fatal(err)
	}
	xml, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(xml), "/blog/recordatorios-de-citas-guia-practica</loc>") || !strings.Contains(string(xml), "/blog</loc>") {
		t.Fatalf("sitemap: %s", xml)
	}
	root.expect(204, "PATCH", "/api/platform/blog/"+draft["id"].(string), map[string]any{"status": "draft"})
	anon.expect(404, "GET", "/api/public/blog/recordatorios-de-citas-guia-practica", nil)
	root.expect(400, "PATCH", "/api/platform/blog/"+draft["id"].(string), map[string]any{"status": "borrado"})
	help.expect(403, "DELETE", "/api/platform/blog/"+draft["id"].(string), nil)
	root.expect(204, "DELETE", "/api/platform/blog/"+draft["id"].(string), nil)
	root.expect(404, "DELETE", "/api/platform/blog/"+draft["id"].(string), nil)
}

func TestBlogNeedsAI(t *testing.T) {
	e := setup(t)
	root := e.login("root")
	if p := root.expect(200, "GET", "/api/platform/blog", nil); p["ai_ready"] != false || p["autopilot"] != false || p["next_run"] != nil {
		t.Fatalf("without a Gemini key the writer is off: %v", p)
	}
	root.expect(503, "POST", "/api/platform/blog/generate", map[string]any{})
}
