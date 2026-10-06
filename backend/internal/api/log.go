package api

import (
	"log"
	"net/http"
)

func logf(r *http.Request, format string, args ...any) {
	log.Printf("%s %s: "+format, append([]any{r.Method, r.URL.Path}, args...)...)
}
