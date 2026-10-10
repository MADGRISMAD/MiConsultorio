package api

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// newVerifyToken is the secret in the QR of a receta: 18 random bytes, URL-safe.
func newVerifyToken() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *Server) verifyURL(token string) string {
	return s.cfg.AppURL + "/verificar/" + token
}

// rxLimiters holds one verification rate limiter per server (tests start many servers in one process).
var rxLimiters sync.Map

func (s *Server) rxLimiter() *rateLimiter {
	l, _ := rxLimiters.LoadOrStore(s, newRateLimiter(40, time.Minute))
	return l.(*rateLimiter)
}

func validVerifyToken(t string) bool {
	if len(t) < 16 || len(t) > 128 {
		return false
	}
	for _, c := range t {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// initials turns "Juan Pérez" + "García López" into "J. P. G. L." so the patient is not named publicly.
func initials(names, lastNames string) string {
	var out []string
	for _, w := range strings.Fields(names + " " + lastNames) {
		out = append(out, strings.ToUpper(string([]rune(w)[:1]))+".")
	}
	return strings.Join(out, " ")
}

// mountPublicRx: Receta verification by QR token. It reveals the minimum to prove a receta is real:
// never the medicines, the diagnosis or the patient's name.
func (s *Server) mountPublicRx(r chi.Router) {
	r.Get("/public/rx/{token}", s.verifyPrescription)
}

func (s *Server) verifyPrescription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	lim, key := s.rxLimiter(), "rxv|"+clientIP(r)
	if !lim.allow(key) {
		writeError(w, http.StatusTooManyRequests, "Demasiadas consultas. Intenta de nuevo en un minuto.")
		return
	}
	lim.fail(key) // every lookup counts, hit or miss
	token := chi.URLParam(r, "token")
	notFound := func() { writeError(w, http.StatusNotFound, "No encontramos una receta con este código.") }
	if !validVerifyToken(token) {
		notFound()
		return
	}
	var (
		folio                          int
		issued                         time.Time
		validUntil                     *string
		voided                         bool
		expired                        bool
		clinic, author, title, license string
		names, lastNames               string
		retained                       bool
	)
	err := s.db.QueryRow(r.Context(), `
		SELECT x.folio, x.issued_at, to_char(x.valid_until,'YYYY-MM-DD'), x.voided_at IS NOT NULL,
			COALESCE(x.valid_until < current_date, false), c.name, x.author_name, x.author_title, x.author_license, pt.names, pt.last_names,
			EXISTS (SELECT 1 FROM jsonb_array_elements(x.items) i WHERE i->>'control' IN ('Antibiótico','Fracción III'))
		FROM prescriptions x
		JOIN clinics c ON c.id = x.clinic_id
		JOIN patients pt ON pt.id = x.patient_id
		WHERE x.verify_token = $1`, token).Scan(&folio, &issued, &validUntil, &voided, &expired, &clinic, &author, &title, &license, &names, &lastNames, &retained)
	if errors.Is(err, pgx.ErrNoRows) {
		s.verifyCertificate(w, r, token, notFound)
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	status := "vigente"
	switch {
	case voided:
		status = "anulada"
	case expired:
		status = "vencida"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": status, "folio": folio, "issued_at": issued, "valid_until": validUntil,
		"clinic_name": clinic, "professional_name": author, "professional_title": title, "professional_license": license,
		"patient_initials": initials(names, lastNames), "retained": retained,
	})
}

// verifyCertificate answers the same QR check for a certificate: that it exists, who issued it and whether it still stands.
func (s *Server) verifyCertificate(w http.ResponseWriter, r *http.Request, token string, notFound func()) {
	var (
		folio                          int
		issued                         time.Time
		validUntil                     *string
		voided, expired                bool
		kind                           string
		clinic, author, title, license string
		names, lastNames               string
	)
	err := s.db.QueryRow(r.Context(), `
		SELECT x.folio, x.issued_at, to_char(x.valid_until,'YYYY-MM-DD'), x.voided_at IS NOT NULL, COALESCE(x.valid_until < current_date, false), x.kind,
			c.name, x.author_name, x.author_title, x.author_license, pt.names, pt.last_names
		FROM certificates x JOIN clinics c ON c.id = x.clinic_id JOIN patients pt ON pt.id = x.patient_id
		WHERE x.verify_token = $1`, token).Scan(&folio, &issued, &validUntil, &voided, &expired, &kind, &clinic, &author, &title, &license, &names, &lastNames)
	if errors.Is(err, pgx.ErrNoRows) {
		notFound()
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	status := "vigente"
	switch {
	case voided:
		status = "anulada"
	case expired:
		status = "vencida"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": status, "folio": folio, "issued_at": issued, "valid_until": validUntil, "document": "certificate_" + kind,
		"clinic_name": clinic, "professional_name": author, "professional_title": title, "professional_license": license,
		"patient_initials": initials(names, lastNames), "retained": false,
	})
}
