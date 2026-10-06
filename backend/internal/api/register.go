package api

import (
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

var clinicKinds = map[string]bool{"GENERAL_MEDICAL": true, "DENTAL": true, "VETERINARY": true, "CHIROPRACTIC": true}

type registerRequest struct {
	ClinicName string `json:"clinic_name"`
	Kind       string `json:"kind"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

// register creates a new clinic with its first administrator and signs them in.
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
	if !clinicKinds[req.Kind] {
		bad("Elige el giro de tu consultorio.")
		return
	}
	if !validEmail(req.Email) {
		bad("El correo electrónico no es válido.")
		return
	}
	if utf8.RuneCountInString(req.Phone) > 30 {
		bad("El teléfono es demasiado largo.")
		return
	}
	if n := utf8.RuneCountInString(req.Username); n < 3 || n > 64 {
		bad("El usuario debe tener entre 3 y 64 caracteres.")
		return
	}
	if msg := validPassword(req.Password); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	s.signups.fail(ip) // every attempt that reaches the database counts
	if _, err := db.CreateClinic(r.Context(), s.db, db.ClinicParams{
		Name: req.ClinicName, Kind: req.Kind, Email: req.Email, Phone: req.Phone, Username: req.Username, Password: req.Password,
	}); err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "Ya existe un consultorio registrado con ese correo.")
			return
		}
		serverError(w, r, err)
		return
	}

	var id string
	if err := s.db.QueryRow(r.Context(),
		`SELECT u.id FROM users u JOIN clinics c ON c.id = u.clinic_id WHERE lower(c.email) = $1 AND u.username = $2`,
		req.Email, req.Username).Scan(&id); err != nil {
		serverError(w, r, err)
		return
	}
	token, err := s.issueToken(id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.setSessionCookie(w, token)
	s.writeSession(w, r, id)
}

func validEmail(e string) bool {
	if len(e) > 254 {
		return false
	}
	a, err := mail.ParseAddress(e)
	return err == nil && a.Address == e && strings.Contains(e[strings.LastIndex(e, "@"):], ".")
}
