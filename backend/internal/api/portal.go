package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	appmail "github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// Patient portal. A portal session is NOT a user: it is a signed claim "this browser proved control of
// this e-mail for this clinic". It is signed with its own key and lives in its own cookie, so it can
// never be accepted by the staff routes and a staff token can never be accepted here.

const (
	portalCookie   = "caresia_portal"
	portalScope    = "portal"
	portalIssuer   = "caresia-portal"
	portalTTL      = 2 * time.Hour
	portalOTPTTL   = 10 * time.Minute
	portalOTPTries = 5
	portalCookieAt = "/api/portal" // the cookie is only ever sent to portal routes
)

type portalLimits struct {
	codeIP, codeKey *rateLimiter // code requests per IP and per clinic+e-mail
	failIP, failKey *rateLimiter // failed sign-ins
}

func newPortalLimits() *portalLimits {
	return &portalLimits{
		codeIP: newRateLimiter(12, time.Hour), codeKey: newRateLimiter(5, time.Hour),
		failIP: newRateLimiter(30, 15*time.Minute), failKey: newRateLimiter(10, 15*time.Minute),
	}
}

type portalClaims struct {
	jwt.RegisteredClaims
	ClinicID string `json:"clinic_id"`
	Email    string `json:"email"`
	Scope    string `json:"scope"`
}

// portalSession is what the portal middleware leaves in the context.
type portalSession struct{ ClinicID, Email string }

type portalCtxKey struct{}

func portalFrom(ctx context.Context) *portalSession {
	p, _ := ctx.Value(portalCtxKey{}).(*portalSession)
	return p
}

func hmacSum(key []byte, parts ...string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(strings.Join(parts, "|")))
	return m.Sum(nil)
}

// portalSigningKey is the session key: the configured one, or derived from the JWT secret with its own label.
func (s *Server) portalSigningKey() []byte {
	if len(s.cfg.PortalKey) > 0 {
		return s.cfg.PortalKey
	}
	return hmacSum(s.cfg.JWTSecret, "portal-session")
}

func (s *Server) portalCodeHash(clinicID, email, code string) []byte {
	return hmacSum(hmacSum(s.cfg.JWTSecret, "portal-otp"), clinicID, email, code)
}

func (s *Server) issuePortalToken(clinicID, email string) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, portalClaims{
		RegisteredClaims: jwt.RegisteredClaims{Issuer: portalIssuer, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(portalTTL))},
		ClinicID:         clinicID, Email: email, Scope: portalScope,
	}).SignedString(s.portalSigningKey())
}

func (s *Server) parsePortalToken(raw string) (*portalSession, error) {
	var c portalClaims
	tok, err := jwt.ParseWithClaims(raw, &c, func(*jwt.Token) (any, error) { return s.portalSigningKey(), nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuer(portalIssuer))
	if err != nil || !tok.Valid || c.Scope != portalScope || !validUUID(c.ClinicID) || c.Email == "" {
		return nil, errors.New("invalid portal token")
	}
	return &portalSession{ClinicID: c.ClinicID, Email: c.Email}, nil
}

func (s *Server) setPortalCookie(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: portalCookie, Value: token, Path: portalCookieAt, MaxAge: maxAge,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

// portalClinic is a clinic whose portal is open, as the public pages see it.
type portalClinic struct {
	ID, Name, Address, Phone, Welcome, Slug string
	CancelMinHours                          int
	Loc                                     *time.Location
}

// loadPortalClinic resolves a slug (or, with a clinic id, a session) to a clinic with the portal on. Unknown slug,
// portal off and lapsed subscription are the same miss.
func (s *Server) loadPortalClinic(ctx context.Context, slug, clinicID string) (*portalClinic, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if clinicID == "" && !slugShape.MatchString(slug) {
		return nil, pgx.ErrNoRows
	}
	var c portalClinic
	var tz string
	err := s.db.QueryRow(ctx, `
		SELECT c.id, c.name, c.address, c.phone_number, coalesce(c.settings->>'timezone', ''), a.portal_welcome, coalesce(a.booking_slug, ''), a.cancel_min_hours
		FROM agenda_settings a JOIN clinics c ON c.id = a.clinic_id
		WHERE a.portal_enabled AND c.billing_status IN ('active', 'trialing', 'past_due') AND c.branch_suspended_at IS NULL
		  AND (($1 <> '' AND lower(a.booking_slug) = $1) OR ($2 <> '' AND c.id = NULLIF($2, '')::uuid))`, slug, clinicID).
		Scan(&c.ID, &c.Name, &c.Address, &c.Phone, &tz, &c.Welcome, &c.Slug, &c.CancelMinHours)
	if err != nil {
		return nil, err
	}
	c.Loc = locationOrDefault(tz)
	return &c, nil
}

// requirePortal resolves the portal cookie into a session and re-checks that the portal is still open.
func (s *Server) requirePortal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(portalCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Ingresa con tu correo para ver tu información.")
			return
		}
		sess, err := s.parsePortalToken(c.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Tu sesión no es válida o expiró. Ingresa de nuevo.")
			return
		}
		if _, err := s.loadPortalClinic(r.Context(), "", sess.ClinicID); errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "Portal no disponible.")
			return
		} else if err != nil {
			serverError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), portalCtxKey{}, sess)))
	})
}

func normalizeEmail(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || len(s) > 200 {
		return "", false
	}
	if !validEmail(s) {
		return "", false
	}
	return s, true
}

// portalAudit writes to the activity log on behalf of a portal visitor (no user id). The code is never part of it.
func portalAudit(ctx context.Context, q execer, clinicID, email, typ, msg string, meta map[string]any) {
	raw, _ := json.Marshal(meta)
	if _, err := q.Exec(ctx, `INSERT INTO activity_log (clinic_id, actor_name, type, message, meta) VALUES ($1,$2,$3,$4,$5)`,
		clinicID, "Portal ("+email+")", typ, msg, raw); err != nil {
		log.Printf("activity log: %v", err)
	}
}

func (s *Server) portalInfo(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadPortalClinic(r.Context(), chi.URLParam(r, "slug"), "")
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Portal no disponible.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clinic": map[string]any{"name": c.Name, "welcome": c.Welcome, "slug": c.Slug}})
}

// portalRequestCode e-mails a one-time code when the address belongs to a patient of the clinic. The answer is
// the same whether it does or not.
func (s *Server) portalRequestCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, err := s.loadPortalClinic(ctx, chi.URLParam(r, "slug"), "")
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Portal no disponible.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &req) {
		return
	}
	email, ok := normalizeEmail(req.Email)
	if !ok {
		writeError(w, http.StatusBadRequest, "Escribe un correo válido.")
		return
	}
	if !s.mailEnabled() {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "El envío de correos no está disponible. Comunícate con el consultorio."})
		return
	}
	ip, key := "pcode|"+clientIP(r), "pcode|"+c.ID+"|"+email
	if !s.portal.codeIP.allow(ip) || !s.portal.codeKey.allow(key) {
		writeError(w, http.StatusTooManyRequests, "Demasiados intentos. Inténtalo de nuevo en un rato.")
		return
	}
	s.portal.codeIP.fail(ip)
	s.portal.codeKey.fail(key)

	reply := func() { writeJSON(w, http.StatusOK, map[string]any{"ok": true}) }
	pts, err := s.portalPatients(ctx, c.ID, email)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if len(pts) == 0 {
		reply()
		return
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		serverError(w, r, err)
		return
	}
	code := fmt.Sprintf("%06d", n.Int64())
	err = inTx(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE portal_otps SET used_at = now() WHERE clinic_id = $1 AND email = $2 AND used_at IS NULL`, c.ID, email); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM portal_otps WHERE created_at < now() - interval '1 day'`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO portal_otps (clinic_id, email, code_hash, expires_at, ip) VALUES ($1,$2,$3,$4,$5)`,
			c.ID, email, s.portalCodeHash(c.ID, email, code), time.Now().Add(portalOTPTTL), clientIP(r))
		return err
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.sendMail(appmail.Message{
		To:      []string{email},
		Subject: "Tu código para entrar a " + c.Name,
		Text:    "Tu código de acceso a " + c.Name + " es " + code + ".\n\nVale 10 minutos y solo sirve una vez. Si no lo pediste tú, ignora este correo.\n",
		HTML: layout("Tu código de acceso", "<p>Usa este código para entrar al portal de <strong>"+esc(c.Name)+"</strong>:</p>"+
			`<p style="font-size:32px;font-weight:700;letter-spacing:8px;margin:20px 0">`+code+"</p>"+
			"<p>Vale <strong>10 minutos</strong> y solo sirve una vez. Si no lo pediste tú, ignora este correo.</p>"),
	})
	reply()
}

// portalLogin exchanges e-mail + code for the portal cookie.
func (s *Server) portalLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, err := s.loadPortalClinic(ctx, chi.URLParam(r, "slug"), "")
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Portal no disponible.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !decode(w, r, &req) {
		return
	}
	email, ok := normalizeEmail(req.Email)
	code := strings.TrimSpace(req.Code)
	if !ok || utf8.RuneCountInString(code) != 6 {
		writeError(w, http.StatusBadRequest, "Escribe tu correo y el código de 6 dígitos.")
		return
	}
	ip, key := "plogin|"+clientIP(r), "plogin|"+c.ID+"|"+email
	if !s.portal.failIP.allow(ip) || !s.portal.failKey.allow(key) {
		writeError(w, http.StatusTooManyRequests, "Demasiados intentos. Inténtalo de nuevo en unos minutos.")
		return
	}

	good := false
	err = inTx(ctx, s.db, func(tx pgx.Tx) error {
		var id string
		var hash []byte
		var attempts int
		err := tx.QueryRow(ctx, `
			SELECT id, code_hash, attempts FROM portal_otps
			WHERE clinic_id = $1 AND email = $2 AND used_at IS NULL AND expires_at > now()
			ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, c.ID, email).Scan(&id, &hash, &attempts)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if attempts >= portalOTPTries {
			return nil
		}
		if hmac.Equal(hash, s.portalCodeHash(c.ID, email, code)) {
			good = true
			_, err = tx.Exec(ctx, `UPDATE portal_otps SET used_at = now(), attempts = attempts + 1 WHERE id = $1`, id)
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE portal_otps SET attempts = attempts + 1 WHERE id = $1`, id)
		return err
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	if good {
		// The code only proves the address; the address must still belong to a patient today.
		pts, err := s.portalPatients(ctx, c.ID, email)
		if err != nil {
			serverError(w, r, err)
			return
		}
		good = len(pts) > 0
	}
	if !good {
		s.portal.failIP.fail(ip)
		s.portal.failKey.fail(key)
		portalAudit(ctx, s.db, c.ID, email, "portal_login_failed", "Intento fallido de ingreso al portal del paciente", map[string]any{"ip": clientIP(r)})
		writeError(w, http.StatusUnauthorized, "El código es incorrecto o ya venció. Pide uno nuevo.")
		return
	}
	s.portal.failKey.reset(key)
	token, err := s.issuePortalToken(c.ID, email)
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.setPortalCookie(w, token, int(portalTTL.Seconds()))
	portalAudit(ctx, s.db, c.ID, email, "portal_login", "Ingreso al portal del paciente", map[string]any{"ip": clientIP(r)})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) portalLogout(w http.ResponseWriter, _ *http.Request) {
	s.setPortalCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}
