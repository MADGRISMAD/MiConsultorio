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
const faroPostsURL = "https://faro.hyperionmkt.com/api/v1/caresia.mx/posts"

var faroCache struct {
	mu   sync.Mutex
	at   time.Time
	slug []string
}

func faroSitemapURLs(ctx context.Context, base string) []string {
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
						Slug string `json:"slug"`
					} `json:"posts"`
				}
				if res.StatusCode == 200 && json.NewDecoder(res.Body).Decode(&wire) == nil {
					faroCache.slug = faroCache.slug[:0]
					for _, p := range wire.Posts {
						faroCache.slug = append(faroCache.slug, p.Slug)
					}
				}
			}
		}
	}
	out := make([]string, 0, len(faroCache.slug))
	for _, s := range faroCache.slug {
		out = append(out, strings.TrimRight(base, "/")+"/blog/"+s)
	}
	return out
}
