package api

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

type registerRequest struct {
	ClinicName string `json:"clinic_name"`
	Kind       string `json:"kind"`
	Phone      string `json:"phone"`
	Name       string `json:"name"` // the person registering: becomes the clinic's administrator
	Email      string `json:"email"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

// register creates a clinic (14-day trial) with its first administrator and signs them in.
// The type of business is chosen afterwards, in the setup wizard.
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.signups.allow(ip) {
		writeError(w, http.StatusTooManyRequests, "Demasiados registros desde esta red. Inténtalo más tarde.")
		return
	}
	var req registerRequest
	if !decode(w, r, &req) {
		return
	}
	req.ClinicName = strings.TrimSpace(req.ClinicName)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Username = strings.TrimSpace(req.Username)

	bad := func(msg string) { writeError(w, http.StatusBadRequest, msg) }
	if n := utf8.RuneCountInString(req.ClinicName); n < 2 || n > 120 {
		bad("El nombre del consultorio debe tener entre 2 y 120 caracteres.")
		return
	}
	if req.Kind == "" {
		req.Kind = "GENERAL_MEDICAL" // the setup wizard asks for the real one
	}
	if !clinicKinds[req.Kind] {
		bad("El giro del consultorio no es válido.")
		return
	}
	if utf8.RuneCountInString(req.Phone) > 30 {
		bad("El teléfono es demasiado largo.")
		return
	}
	for _, msg := range []string{db.ValidateName(req.Name), db.ValidateEmail(req.Email), db.ValidateUsername(req.Username), db.ValidatePassword(req.Password)} {
		if msg != "" {
			bad(msg)
			return
		}
	}

	s.signups.fail(ip) // every attempt that reaches the database counts
	clinicID, err := db.CreateClinic(r.Context(), s.db, db.ClinicParams{
		Name: req.ClinicName, Kind: req.Kind, Phone: req.Phone, Plan: "basico", Status: "trialing",
		AdminName: req.Name, AdminEmail: req.Email, AdminUsername: req.Username, AdminPassword: req.Password,
	})
	if err != nil {
		if msg := uniqueMessage(err); msg != "" {
			writeError(w, http.StatusConflict, msg)
			return
		}
		serverError(w, r, err)
		return
	}

	var id string
	if err := s.db.QueryRow(r.Context(), `SELECT id FROM users WHERE clinic_id = $1 AND role = 'admin'`, clinicID).Scan(&id); err != nil {
		serverError(w, r, err)
		return
	}
	p, err := loadPrincipal(r.Context(), s.db, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, clinicID, p, "clinic_created", "Creó el consultorio "+req.ClinicName, map[string]any{"kind": req.Kind})
	_, _ = s.db.Exec(r.Context(), `UPDATE users SET last_login_at = now() WHERE id = $1`, id)
	s.welcomeEmail(req.Email, req.Name, req.ClinicName)
	s.startSession(w, r, p)
}
