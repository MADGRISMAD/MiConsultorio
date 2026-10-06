package api

import (
	"net/http"
	"strings"

	"github.com/madgrismad/miconsultorio/backend/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// updateProfile lets anyone edit their own name and phone.
func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	if !decode(w, r, &req) {
		return
	}
	if msg := db.ValidateName(req.Name); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if len(req.Phone) > 30 {
		writeError(w, http.StatusBadRequest, "El teléfono es demasiado largo.")
		return
	}
	p := principalFrom(r.Context())
	if _, err := s.db.Exec(r.Context(), `UPDATE users SET name = $2, phone = $3 WHERE id = $1`, p.UserID, strings.TrimSpace(req.Name), strings.TrimSpace(req.Phone)); err != nil {
		serverError(w, r, err)
		return
	}
	fresh, err := loadPrincipal(r.Context(), s.db, p.UserID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionResponse{Session: sessionOf(fresh)})
}

// changeOwnPassword needs the current password, ends every other session, and keeps this one.
func (s *Server) changeOwnPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if msg := db.ValidatePassword(req.NewPassword); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	var hash string
	if err := s.db.QueryRow(r.Context(), `SELECT password_hash FROM users WHERE id = $1`, p.UserID).Scan(&hash); err != nil {
		serverError(w, r, err)
		return
	}
	key := "pw|" + p.UserID
	if !s.limiter.allow(key) {
		writeError(w, http.StatusTooManyRequests, "Demasiados intentos. Inténtalo de nuevo en unos minutos.")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.CurrentPassword)) != nil {
		s.limiter.fail(key)
		writeError(w, http.StatusBadRequest, "La contraseña actual no es correcta.")
		return
	}
	s.limiter.reset(key)
	newHash, err := db.HashPassword(req.NewPassword)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if _, err := s.db.Exec(r.Context(), `UPDATE users SET password_hash = $2, token_version = token_version + 1 WHERE id = $1`, p.UserID, newHash); err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "password_changed", p.actorName()+" cambió su contraseña", nil)
	fresh, err := loadPrincipal(r.Context(), s.db, p.UserID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.startSession(w, r, fresh) // a new cookie with the new token version
}
