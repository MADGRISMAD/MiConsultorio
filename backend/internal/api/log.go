package api

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

func logf(r *http.Request, format string, args ...any) {
	log.Printf("[%s] %s %s: "+format, append([]any{middleware.GetReqID(r.Context()), r.Method, r.URL.Path}, args...)...)
}
