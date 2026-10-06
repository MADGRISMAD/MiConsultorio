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

func (s *Server) issueToken(userID string) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.SessionTTL)),
	}).SignedString(s.cfg.JWTSecret)
}

func (s *Server) parseToken(raw string) (string, error) {
	var claims jwt.RegisteredClaims
	tok, err := jwt.ParseWithClaims(raw, &claims, func(*jwt.Token) (any, error) { return s.cfg.JWTSecret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !tok.Valid || claims.Subject == "" {
		return "", errors.New("invalid token")
	}
	return claims.Subject, nil
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		MaxAge:   int(s.cfg.SessionTTL.Seconds()),
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

type sessionResponse struct {
	Session sessionInfo `json:"session"`
}

type sessionInfo struct {
	ClinicID    string   `json:"clinicId"`
	Username    string   `json:"username"`
	Permissions []string `json:"permissions"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decode(w, r, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	username := strings.TrimSpace(req.Username)
	if email == "" || username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "Correo de la clínica, usuario y contraseña son obligatorios.")
		return
	}
	key := clientIP(r) + "|" + email + "|" + strings.ToLower(username)
	if !s.limiter.allow(key) {
		writeError(w, http.StatusTooManyRequests, "Demasiados intentos. Inténtalo de nuevo en unos minutos.")
		return
	}

	var id, hash string
	err := s.db.QueryRow(r.Context(), `
		SELECT u.id, u.password_hash FROM users u JOIN clinics c ON c.id = u.clinic_id
		WHERE lower(c.email) = $1 AND u.username = $2`, email, username).Scan(&id, &hash)
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

	token, err := s.issueToken(id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.setSessionCookie(w, token)
	s.writeSession(w, r, id)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	writeJSON(w, http.StatusOK, sessionResponse{Session: sessionInfo{ClinicID: p.ClinicID, Username: p.Username, Permissions: p.Permissions}})
}

func (s *Server) writeSession(w http.ResponseWriter, r *http.Request, userID string) {
	var info sessionInfo
	err := s.db.QueryRow(r.Context(), `SELECT clinic_id, username, permissions FROM users WHERE id = $1`, userID).
		Scan(&info.ClinicID, &info.Username, &info.Permissions)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionResponse{Session: info})
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
