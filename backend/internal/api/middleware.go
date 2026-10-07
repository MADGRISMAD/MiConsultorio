package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Capability names, shared with the frontend. Roles grant them (see roles.go).
const (
	PermAdminUsers        = "adminUsers"
	PermAdminAppointments = "adminAppointments"
	PermAdminHistorials   = "adminHistorials"
	PermNavHistorials     = "navHistorials"
	PermNavAppointments   = "navAppointments"
)

type ctxKey struct{}

// Principal is the authenticated user, loaded fresh from the database on every
// request so deactivated accounts, role changes and subscription changes apply immediately.
type Principal struct {
	UserID       string
	ClinicID     string // empty for platform staff
	Username     string
	Name         string
	Email        string
	Role         string
	Permissions  []string
	TokenVersion int
	Disabled     bool
	Billing      *Billing // nil for platform staff
	// Professional data printed on recetas and notes.
	Cedula, CedulaInstitution, CedulaSpecialty, SpecialtyTitle string
	// SetupPending: a clinic administrator whose clinic has not finished the setup wizard.
	SetupPending bool
	// TwoFactorEnabled: the person confirmed an authenticator app. MustSetup2FA: the clinic's policy
	// asks for it and they have not set it up yet (clinical routes answer SETUP_2FA until they do).
	TwoFactorEnabled, MustSetup2FA bool
}

func (p *Principal) isPlatform() bool { return p.ClinicID == "" }

// actorName is how the person appears in the activity log.
func (p *Principal) actorName() string {
	if p.Name != "" {
		return p.Name
	}
	return p.Username
}

func principalFrom(ctx context.Context) *Principal {
	p, _ := ctx.Value(ctxKey{}).(*Principal)
	return p
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var errNoUser = errors.New("user not found")

// loadPrincipal reads a user with their clinic's subscription state.
func loadPrincipal(ctx context.Context, q queryRower, id string) (*Principal, error) {
	var p Principal
	var plan, status, reason string
	var setupOpen bool
	var policy string
	var trialEnds, periodEnd *time.Time
	err := q.QueryRow(ctx, `
		SELECT u.id, coalesce(u.clinic_id::text, ''), u.username, u.name, coalesce(u.email, ''), u.role, u.disabled, u.token_version, u.cedula, u.cedula_institution, u.cedula_specialty, u.specialty_title,
		       coalesce(c.plan, ''), coalesce(c.billing_status, ''), c.trial_ends_at, c.current_period_end, coalesce(c.suspended_reason, ''), coalesce(c.setup_completed_at IS NULL, false), u.totp_enabled, coalesce(c.require_2fa, 'none')
		FROM users u LEFT JOIN clinics c ON c.id = u.clinic_id
		WHERE u.id = $1`, id).
		Scan(&p.UserID, &p.ClinicID, &p.Username, &p.Name, &p.Email, &p.Role, &p.Disabled, &p.TokenVersion, &p.Cedula, &p.CedulaInstitution, &p.CedulaSpecialty, &p.SpecialtyTitle,
			&plan, &status, &trialEnds, &periodEnd, &reason, &setupOpen, &p.TwoFactorEnabled, &policy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNoUser
	}
	if err != nil {
		return nil, err
	}
	p.Permissions = permissionsFor(p.Role)
	p.SetupPending = setupOpen && p.Role == RoleAdmin
	p.MustSetup2FA = p.ClinicID != "" && !p.TwoFactorEnabled && twoFactorRequired(policy, p.Role)
	if p.ClinicID != "" {
		p.Billing = &Billing{Plan: plan, Status: status, TrialEndsAt: trialEnds, CurrentPeriodEnd: periodEnd, SuspendedReason: reason}
		if pl, ok := planByID(plan); !ok || !pl.Cobros { // plans without cobros never get the POS capabilities
			p.Permissions = withoutPOS(p.Permissions)
		}
	}
	return &p, nil
}

// requireAuth resolves the session cookie into a Principal. A session dies as soon as
// the account is deactivated, deleted, or its token version moves (password or role change).
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "No has iniciado sesión.")
			return
		}
		userID, tv, err := s.parseToken(c.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Tu sesión no es válida o expiró.")
			return
		}
		p, err := loadPrincipal(r.Context(), s.db, userID)
		if errors.Is(err, errNoUser) || (err == nil && (p.Disabled || p.TokenVersion != tv)) {
			writeError(w, http.StatusUnauthorized, "Tu sesión terminó. Vuelve a entrar.")
			return
		}
		if err != nil {
			serverError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, p)))
	})
}

// requireClinic keeps platform staff out of clinic data.
func requireClinic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := principalFrom(r.Context()); p == nil || p.isPlatform() {
			writeError(w, http.StatusForbidden, "Esta sección es solo para cuentas de un consultorio.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireSubscription blocks clinic data while the clinic's trial or subscription is not active.
func (s *Server) requireSubscription(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := principalFrom(r.Context())
		if p != nil && p.Billing != nil && !p.Billing.Usable(time.Now()) {
			writeJSON(w, http.StatusForbidden, errorBody{
				Code:    "SUBSCRIPTION_REQUIRED",
				Message: "La prueba o suscripción de este consultorio no está activa. Contacta a tu administrador.",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// require restricts a route to users holding at least one of the given capabilities.
func require(perms ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := principalFrom(r.Context())
			if p == nil || !hasAnyPermission(p.Permissions, perms...) {
				writeError(w, http.StatusForbidden, "No tienes permiso para realizar esta acción.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requireRoles restricts a route to the listed roles.
func requireRoles(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := principalFrom(r.Context())
			if p == nil || !hasPermission(roles, p.Role) {
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

func withoutPOS(perms []string) []string {
	out := make([]string, 0, len(perms))
	for _, x := range perms {
		if x != PermPOS && x != PermPOSReports && x != PermPOSManage {
			out = append(out, x)
		}
	}
	return out
}
