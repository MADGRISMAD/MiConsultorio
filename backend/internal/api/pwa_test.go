package api_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
)

func TestPWAHeaders(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"index.html":           "<html>shell</html>",
		"service-worker.js":    "self.addEventListener('install',()=>{})",
		"manifest.webmanifest": `{"name":"Caresia"}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e := setupWith(t, func(c *config.Config) { c.StaticDir = dir })
	get := func(path string) (*http.Response, string) {
		res, err := http.Get(e.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res, string(b)
	}

	res, _ := get("/service-worker.js")
	if res.StatusCode != 200 || res.Header.Get("Cache-Control") != "no-cache" || res.Header.Get("Service-Worker-Allowed") != "/" ||
		res.Header.Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatalf("service worker headers: %d %v", res.StatusCode, res.Header)
	}
	res, _ = get("/manifest.webmanifest")
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "application/manifest+json" || res.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("manifest headers: %d %v", res.StatusCode, res.Header)
	}
	// client-side routes (including /offline) fall back to the shell and are never cached by the browser
	res, body := get("/offline")
	if res.StatusCode != 200 || body != "<html>shell</html>" || res.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("offline fallback: %d %q %v", res.StatusCode, body, res.Header)
	}
	// the API keeps its own 404 and is never answered with the shell
	res, body = get("/api/nope")
	if res.StatusCode != 404 || body == "<html>shell</html>" {
		t.Fatalf("api fallback: %d %q", res.StatusCode, body)
	}
}

func TestVetDiagnoses(t *testing.T) {
	e := setup(t)
	doc := e.login("doc_a")
	got := doc.expect(200, "GET", "/api/rx/diagnoses?q=otitis&subject=animal&species=Gato", nil)
	list, _ := got["diagnoses"].([]any)
	if len(list) != 4 || got["disclaimer"] == "" {
		t.Fatalf("vet diagnoses: %v", got)
	}
	for _, d := range list {
		if d.(map[string]any)["code"] != "" {
			t.Fatalf("veterinary terms must not carry codes: %v", d)
		}
	}
	// rabbit-only terms do not show for cats; people keep the partial CIE-10
	rabbit := doc.expect(200, "GET", "/api/rx/diagnoses?q=mixomatosis&subject=animal&species=Gato", nil)
	if n, _ := rabbit["total"].(float64); n != 0 {
		t.Fatalf("species filter: %v", rabbit)
	}
	human := doc.expect(200, "GET", "/api/rx/diagnoses?q=hipertension", nil)
	if first := human["diagnoses"].([]any)[0].(map[string]any); first["code"] != "I10" {
		t.Fatalf("human catalog: %v", human)
	}
}
