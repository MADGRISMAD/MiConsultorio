package api

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "caresia_session"

// dummyHash lets a login for an unknown user cost the same as a real one.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("caresia-dummy-password"), bcrypt.DefaultCost)

type claims struct {
	jwt.RegisteredClaims
	TV int `json:"tv"` // token version: bumping it on the user ends every session of that account
}

func (s *Server) issueToken(userID string, tokenVersion int) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.SessionTTL)),
		},
		TV: tokenVersion,
	}).SignedString(s.cfg.JWTSecret)
}

func (s *Server) parseToken(raw string) (userID string, tokenVersion int, err error) {
	var c claims
	tok, err := jwt.ParseWithClaims(raw, &c, func(*jwt.Token) (any, error) { return s.cfg.JWTSecret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !tok.Valid || c.Subject == "" {
		return "", 0, errors.New("invalid token")
	}
	return c.Subject, c.TV, nil
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		MaxAge:   int(s.cfg.SessionTTL.Seconds()),
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

// startSession signs the user in: sets the cookie and writes the session.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, p *Principal) {
	token, err := s.issueToken(p.UserID, p.TokenVersion)
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, sessionResponse{Session: sessionOf(p)})
}

type sessionResponse struct {
	Session sessionInfo `json:"session"`
}

type sessionInfo struct {
	UserID      string       `json:"userId"`
	ClinicID    string       `json:"clinicId"`
	Username    string       `json:"username"`
	Name        string       `json:"name"`
	Email       string       `json:"email"`
	Role        string       `json:"role"`
	RoleLabel   string       `json:"roleLabel"`
	Permissions []string     `json:"permissions"`
	Billing     *billingInfo `json:"billing"` // nil for platform staff
}

func sessionOf(p *Principal) sessionInfo {
	info := sessionInfo{
		UserID: p.UserID, ClinicID: p.ClinicID, Username: p.Username, Name: p.Name, Email: p.Email,
		Role: p.Role, RoleLabel: roleLabels[p.Role], Permissions: p.Permissions,
	}
	if p.Billing != nil {
		b := p.Billing.info(time.Now())
		info.Billing = &b
	}
	return info
}

type loginRequest struct {
	Identifier string `json:"identifier"` // e-mail or username
	Password   string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decode(w, r, &req) {
		return
	}
	ident := strings.ToLower(strings.TrimSpace(req.Identifier))
	if ident == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "Escribe tu correo o usuario y tu contraseña.")
		return
	}
	key := clientIP(r) + "|" + ident
	if !s.limiter.allow(key) {
		writeError(w, http.StatusTooManyRequests, "Demasiados intentos. Inténtalo de nuevo en unos minutos.")
		return
	}

	// E-mails always contain '@' and usernames never do, so at most one account matches.
	var id, hash string
	err := s.db.QueryRow(r.Context(),
		`SELECT id, password_hash FROM users WHERE lower(email) = $1 OR lower(username) = $1`, ident).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password))
		s.limiter.fail(key)
		writeError(w, http.StatusUnauthorized, "Credenciales incorrectas.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		s.limiter.fail(key)
		writeError(w, http.StatusUnauthorized, "Credenciales incorrectas.")
		return
	}
	s.limiter.reset(key)

	p, err := loadPrincipal(r.Context(), s.db, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	// Said only after the password checked out, so it can't be used to probe for accounts.
	if p.Disabled {
		writeError(w, http.StatusForbidden, "Esta cuenta está desactivada. Contacta al administrador de tu consultorio.")
		return
	}
	_, _ = s.db.Exec(r.Context(), `UPDATE users SET last_login_at = now() WHERE id = $1`, id)
	s.startSession(w, r, p)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sessionResponse{Session: sessionOf(principalFrom(r.Context()))})
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// rateLimiter allows `max` recorded events per key within `window`.
type rateLimiter struct {
	mu     sync.Mutex
	events map[string][]time.Time
	max    int
	window time.Duration
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{events: map[string][]time.Time{}, max: max, window: window}
}

func (l *rateLimiter) prune(key string, now time.Time) []time.Time {
	kept := l.events[key][:0]
	for _, t := range l.events[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.events, key)
	} else {
		l.events[key] = kept
	}
	return kept
}

func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.prune(key, time.Now())) < l.max
}

func (l *rateLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.events) > 10000 { // bound memory under key-spraying
		for k := range l.events {
			l.prune(k, now)
		}
	}
	l.prune(key, now)
	l.events[key] = append(l.events[key], now)
}

func (l *rateLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.events, key)
}
