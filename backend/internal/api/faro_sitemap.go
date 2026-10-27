package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Los artículos del blog que escribe y publica Faro (faro.hyperionmkt.com) viven en su API pública. El mapa del sitio
// los lee de ahí, con caché de 15 min; si Faro no responde se usa lo último que se tuvo (o nada) y el resto del mapa sale igual.
const faroPostsURL = "https://faro.hyperionmkt.com/api/v1/caresia.mx/posts?limit=50"

// faroURL es un artículo de Faro tal como entra al mapa del sitio (lastmod en formato AAAA-MM-DD).
type faroURL struct{ Loc, Lastmod string }

var faroCache struct {
	mu    sync.Mutex
	at    time.Time
	posts []struct{ Slug, Date string }
}

func faroSitemapURLs(ctx context.Context, base string) []faroURL {
	faroCache.mu.Lock()
	defer faroCache.mu.Unlock()
	if time.Since(faroCache.at) > 15*time.Minute {
		faroCache.at = time.Now() // también si falla: no se reintenta en cada visita
		c, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		if req, err := http.NewRequestWithContext(c, "GET", faroPostsURL, nil); err == nil {
			if res, err := http.DefaultClient.Do(req); err == nil {
				defer res.Body.Close()
				var wire struct {
					Posts []struct {
						Slug        string `json:"slug"`
						PublishedAt string `json:"published_at"`
					} `json:"posts"`
				}
				if res.StatusCode == 200 && json.NewDecoder(res.Body).Decode(&wire) == nil {
					faroCache.posts = faroCache.posts[:0]
					for _, p := range wire.Posts {
						d := p.PublishedAt
						if len(d) >= 10 {
							d = d[:10]
						}
						faroCache.posts = append(faroCache.posts, struct{ Slug, Date string }{p.Slug, d})
					}
				}
			}
		}
	}
	out := make([]faroURL, 0, len(faroCache.posts))
	for _, p := range faroCache.posts {
		out = append(out, faroURL{strings.TrimRight(base, "/") + "/blog/" + p.Slug, p.Date})
	}
	return out
}
