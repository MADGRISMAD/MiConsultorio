package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type errorBody struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Message: msg})
}

func serverError(w http.ResponseWriter, r *http.Request, err error) {
	logf(r, "internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "Error interno del servidor.")
}

// decode reads a JSON body strictly: unknown fields and oversized bodies are rejected.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "La solicitud es demasiado grande.")
		} else {
			writeError(w, http.StatusBadRequest, "Solicitud inválida.")
		}
		return false
	}
	return true
}

// hasPermission reports whether list contains want.
func hasPermission(list []string, want string) bool {
	for _, p := range list {
		if p == want {
			return true
		}
	}
	return false
}

// hasAnyPermission reports whether list contains at least one of wants.
func hasAnyPermission(list []string, wants ...string) bool {
	for _, w := range wants {
		if hasPermission(list, w) {
			return true
		}
	}
	return false
}

func sameOrigin(origin, host string) bool {
	origin = strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
	return origin == host
}
