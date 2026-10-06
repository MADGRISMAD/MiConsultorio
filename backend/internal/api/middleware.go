package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Permission names, shared with the frontend.
const (
	PermAdminUsers        = "adminUsers"
	PermAdminAppointments = "adminAppointments"
	PermAdminHistorials   = "adminHistorials"
	PermNavHistorials     = "navHistorials"
	PermNavAppointments   = "navAppointments"
)

var allPermissions = []string{PermAdminUsers, PermAdminAppointments, PermAdminHistorials, PermNavHistorials, PermNavAppointments}

type ctxKey struct{}

// Principal is the authenticated user, loaded fresh from the database on every
// request so deleted users and permission changes take effect immediately.
type Principal struct {
	UserID      string
	ClinicID    string
	Username    string
	Permissions []string
}

func principalFrom(ctx context.Context) *Principal {
	p, _ := ctx.Value(ctxKey{}).(*Principal)
	return p
}

// requireAuth resolves the session cookie into a Principal.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "No has iniciado sesión.")
			return
		}
		userID, err := s.parseToken(c.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Tu sesión no es válida o expiró.")
			return
		}
		var p Principal
		err = s.db.QueryRow(r.Context(),
			`SELECT id, clinic_id, username, permissions FROM users WHERE id = $1`, userID,
		).Scan(&p.UserID, &p.ClinicID, &p.Username, &p.Permissions)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "Tu sesión no es válida o expiró.")
			return
		}
		if err != nil {
			serverError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, &p)))
	})
}

// require restricts a route to users holding at least one of the given permissions.
func require(perms ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := principalFrom(r.Context())
			if p == nil || !hasPermission(p.Permissions, perms...) {
				writeError(w, http.StatusForbidden, "No tienes permiso para realizar esta acción.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// sameOriginOnly blocks cross-site state-changing requests (defence in depth on
// top of the SameSite=Lax session cookie).
func (s *Server) sameOriginOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if origin := r.Header.Get("Origin"); origin != "" && !s.originAllowed(origin, r.Host) {
				writeError(w, http.StatusForbidden, "Origen no permitido.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin, host string) bool {
	if sameOrigin(origin, host) {
		return true
	}
	origin = strings.TrimRight(origin, "/")
	for _, o := range s.cfg.AllowedOrigins {
		if o == origin {
			return true
		}
	}
	return false
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
