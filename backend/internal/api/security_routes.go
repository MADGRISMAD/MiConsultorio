package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// Two-step verification (TOTP). Setup and management live under /me/2fa; the second step of the
// sign-in is POST /login/2fa (mounted next to /login). Clinic policy and the administrator's reset
// live under /security.

const (
	challengeTTL      = 5 * time.Minute
	recoveryCodeCount = 10
	errCodeBad        = "El código no es correcto o ya se usó. Revisa la hora de tu teléfono e inténtalo con el siguiente."
)

// mountSecurity Two-step verification.
func (s *Server) mountSecurity(r chi.Router) {
	r.Post("/me/2fa/setup", s.twoFactorSetup)
	r.Post("/me/2fa/enable", s.twoFactorEnable)
	r.Post("/me/2fa/disable", s.twoFactorDisable)
	r.Post("/me/2fa/recovery-codes", s.twoFactorRegenerate)

	r.Route("/security", func(r chi.Router) {
		r.Use(requireClinic, s.requireSubscription, require(PermAdminUsers))
		r.Get("/policy", s.getSecurityPolicy)
		r.Put("/policy", s.updateSecurityPolicy)
		r.Get("/members", s.securityMembers)
		r.Post("/members/{id}/2fa/reset", s.resetMemberTwoFactor)
	})
}

// ---- policy ------------------------------------------------------------------------------------------------

var twoFactorPolicies = []string{"none", "admins", "clinical", "all"}

// twoFactorRequired says whether a clinic policy asks this role to use two-step verification.
func twoFactorRequired(policy, role string) bool {
	switch policy {
	case "admins":
		return role == RoleAdmin
	case "clinical":
		return role == RoleAdmin || role == RoleDoctor
	case "all":
		return isClinicRole(role)
	}
	return false
}

// clinicalPrefixes are the routes closed to someone who must set up two-step verification and has not.
var clinicalPrefixes = []string{"/api/patients", "/api/encounters", "/api/prescriptions", "/api/files"}

// requireTwoFactorSetup answers 403 SETUP_2FA on clinical data until the person turns two-step verification on.
func requireTwoFactorSetup(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := principalFrom(r.Context()); p != nil && p.MustSetup2FA {
			for _, pre := range clinicalPrefixes {
				if r.URL.Path == pre || strings.HasPrefix(r.URL.Path, pre+"/") {
					writeJSON(w, http.StatusForbidden, errorBody{
						Code:    "SETUP_2FA",
						Message: "Tu consultorio pide verificación en dos pasos. Actívala en Mi cuenta para ver expedientes.",
					})
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) getSecurityPolicy(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	var policy string
	if err := s.db.QueryRow(r.Context(), `SELECT require_2fa FROM clinics WHERE id = $1`, p.ClinicID).Scan(&policy); err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"require_2fa": policy})
}

func (s *Server) updateSecurityPolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Require2FA string `json:"require_2fa"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !hasPermission(twoFactorPolicies, req.Require2FA) {
		writeError(w, http.StatusBadRequest, "Elige una opción válida.")
		return
	}
	p := principalFrom(r.Context())
	if _, err := s.db.Exec(r.Context(), `UPDATE clinics SET require_2fa = $2 WHERE id = $1`, p.ClinicID, req.Require2FA); err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "security_policy", p.actorName()+" cambió la política de verificación en dos pasos", map[string]any{"require_2fa": req.Require2FA})
	writeJSON(w, http.StatusOK, map[string]any{"require_2fa": req.Require2FA})
}

func (s *Server) securityMembers(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `
		SELECT id, name, username, role, disabled, totp_enabled FROM users WHERE clinic_id = $1 ORDER BY lower(name)`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	type member struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		Username         string `json:"username"`
		Role             string `json:"role"`
		RoleLabel        string `json:"role_label"`
		Disabled         bool   `json:"disabled"`
		TwoFactorEnabled bool   `json:"two_factor_enabled"`
	}
	out := []member{}
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.ID, &m.Name, &m.Username, &m.Role, &m.Disabled, &m.TwoFactorEnabled); err != nil {
			serverError(w, r, err)
			return
		}
		m.RoleLabel = roleLabels[m.Role]
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": out})
}

// resetMemberTwoFactor lets an administrator clear a teammate's authenticator (lost phone). Their
// sessions end, and the person must set it up again if the policy asks for it.
func (s *Server) resetMemberTwoFactor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p := principalFrom(r.Context())
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Persona no encontrada.")
		return
	}
	if id == p.UserID {
		writeError(w, http.StatusBadRequest, "Para tu propia cuenta usa Mi cuenta: ahí se pide tu contraseña y un código.")
		return
	}
	var name string
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		err := tx.QueryRow(r.Context(), `
			UPDATE users SET totp_secret_enc = NULL, totp_enabled = false, totp_confirmed_at = NULL, totp_last_step = 0, token_version = token_version + 1
			WHERE id = $1 AND clinic_id = $2 RETURNING name`, id, p.ClinicID).Scan(&name)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `DELETE FROM totp_recovery_codes WHERE user_id = $1`, id); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "2fa_reset", p.actorName()+" restableció la verificación en dos pasos de "+name, map[string]any{"userId": id})
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Persona no encontrada.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---- own account -------------------------------------------------------------------------------------------

type totpState struct {
	enc     []byte
	enabled bool
	last    int64
	hash    string
}

func (s *Server) loadTOTP(ctx context.Context, userID string) (totpState, error) {
	var t totpState
	err := s.db.QueryRow(ctx, `SELECT totp_secret_enc, totp_enabled, totp_last_step, password_hash FROM users WHERE id = $1`, userID).
		Scan(&t.enc, &t.enabled, &t.last, &t.hash)
	return t, err
}

// verifyFactor accepts an authenticator code (6 digits) or a recovery code, and consumes what it accepts.
// usedRecovery tells which one it was.
func (s *Server) verifyFactor(ctx context.Context, userID, input string) (ok, usedRecovery bool, err error) {
	t, err := s.loadTOTP(ctx, userID)
	if err != nil || !t.enabled || len(t.enc) == 0 {
		return false, false, err
	}
	if digits := normalizeCode(input); len(digits) == totpDigits && !strings.ContainsAny(input, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		secret, err := s.openTOTP(userID, t.enc)
		if err != nil {
			return false, false, err
		}
		step, match := totpMatch(secret, digits, time.Now(), t.last)
		if !match {
			return false, false, nil
		}
		// Advance atomically: of two concurrent requests with the same code, only one wins.
		tag, err := s.db.Exec(ctx, `UPDATE users SET totp_last_step = $2 WHERE id = $1 AND totp_last_step < $2`, userID, step)
		return err == nil && tag.RowsAffected() == 1, false, err
	}
	rec := normalizeRecovery(input)
	if len(rec) != 10 {
		return false, false, nil
	}
	rows, err := s.db.Query(ctx, `SELECT id, salt, code_hash FROM totp_recovery_codes WHERE user_id = $1 AND used_at IS NULL`, userID)
	if err != nil {
		return false, false, err
	}
	var hitID string
	for rows.Next() {
		var id, salt, hash string
		if err := rows.Scan(&id, &salt, &hash); err != nil {
			rows.Close()
			return false, false, err
		}
		if hmacEqual(recoveryHash(salt, rec), hash) {
			hitID = id
		}
	}
	rows.Close()
	if hitID == "" {
		return false, false, nil
	}
	tag, err := s.db.Exec(ctx, `UPDATE totp_recovery_codes SET used_at = now() WHERE id = $1 AND used_at IS NULL`, hitID)
	return err == nil && tag.RowsAffected() == 1, true, err
}

func hmacEqual(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

func (s *Server) recoveryLeft(ctx context.Context, userID string) int {
	var n int
	_ = s.db.QueryRow(ctx, `SELECT count(*) FROM totp_recovery_codes WHERE user_id = $1 AND used_at IS NULL`, userID).Scan(&n)
	return n
}

func (s *Server) twoFactorSetup(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	if p.TwoFactorEnabled {
		writeError(w, http.StatusConflict, "La verificación en dos pasos ya está activa. Desactívala primero si quieres configurar otro teléfono.")
		return
	}
	secret, err := newTOTPSecret()
	if err != nil {
		serverError(w, r, err)
		return
	}
	enc, err := s.sealTOTP(p.UserID, secret)
	if err != nil {
		serverError(w, r, err)
		return
	}
	// Not enabled yet: the secret only counts once a code from the app proves it was scanned.
	if _, err := s.db.Exec(r.Context(), `UPDATE users SET totp_secret_enc = $2, totp_enabled = false, totp_confirmed_at = NULL WHERE id = $1`, p.UserID, enc); err != nil {
		serverError(w, r, err)
		return
	}
	account := p.Email
	if account == "" {
		account = p.Username
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"secret": totpB32.EncodeToString(secret), "otpauth_uri": totpURI(secret, account), "issuer": totpIssuer, "account": account,
	})
}

func (s *Server) twoFactorEnable(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &req) {
		return
	}
	p := principalFrom(r.Context())
	if p.TwoFactorEnabled {
		writeError(w, http.StatusConflict, "La verificación en dos pasos ya está activa.")
		return
	}
	key := "2fa|" + p.UserID
	if !s.limiter.allow(key) {
		writeError(w, http.StatusTooManyRequests, "Demasiados intentos. Inténtalo de nuevo en unos minutos.")
		return
	}
	t, err := s.loadTOTP(r.Context(), p.UserID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if len(t.enc) == 0 {
		writeError(w, http.StatusBadRequest, "Primero genera el código QR.")
		return
	}
	secret, err := s.openTOTP(p.UserID, t.enc)
	if err != nil {
		serverError(w, r, err)
		return
	}
	step, ok := totpMatch(secret, normalizeCode(req.Code), time.Now(), 0)
	if !ok {
		s.limiter.fail(key)
		writeError(w, http.StatusBadRequest, errCodeBad)
		return
	}
	s.limiter.reset(key)
	var codes []string
	err = inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `UPDATE users SET totp_enabled = true, totp_confirmed_at = now(), totp_last_step = $2 WHERE id = $1`, p.UserID, step); err != nil {
			return err
		}
		var err error
		codes, err = s.replaceRecoveryCodes(r.Context(), tx, p)
		if err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "2fa_enabled", p.actorName()+" activó la verificación en dos pasos", nil)
		return nil
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.writeFactorResult(w, r, p.UserID, codes)
}

func (s *Server) twoFactorDisable(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decode(w, r, &req) {
		return
	}
	p := principalFrom(r.Context())
	if !s.confirmSensitive(w, r, p, req.Password, req.Code) {
		return
	}
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `UPDATE users SET totp_secret_enc = NULL, totp_enabled = false, totp_confirmed_at = NULL, totp_last_step = 0 WHERE id = $1`, p.UserID); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `DELETE FROM totp_recovery_codes WHERE user_id = $1`, p.UserID); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "2fa_disabled", p.actorName()+" desactivó la verificación en dos pasos", nil)
		return nil
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.writeFactorResult(w, r, p.UserID, nil)
}

func (s *Server) twoFactorRegenerate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decode(w, r, &req) {
		return
	}
	p := principalFrom(r.Context())
	if !s.confirmSensitive(w, r, p, req.Password, req.Code) {
		return
	}
	var codes []string
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var err error
		if codes, err = s.replaceRecoveryCodes(r.Context(), tx, p); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "2fa_recovery_codes", p.actorName()+" generó códigos de recuperación nuevos", nil)
		return nil
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.writeFactorResult(w, r, p.UserID, codes)
}

// confirmSensitive asks for the password and a current code before turning protection off or reissuing codes.
// It writes the error response itself and reports whether to go on.
func (s *Server) confirmSensitive(w http.ResponseWriter, r *http.Request, p *Principal, password, code string) bool {
	if !p.TwoFactorEnabled {
		writeError(w, http.StatusConflict, "La verificación en dos pasos no está activa.")
		return false
	}
	key := "2fa|" + p.UserID
	if !s.limiter.allow(key) {
		writeError(w, http.StatusTooManyRequests, "Demasiados intentos. Inténtalo de nuevo en unos minutos.")
		return false
	}
	t, err := s.loadTOTP(r.Context(), p.UserID)
	if err != nil {
		serverError(w, r, err)
		return false
	}
	if bcrypt.CompareHashAndPassword([]byte(t.hash), []byte(password)) != nil {
		s.limiter.fail(key)
		writeError(w, http.StatusBadRequest, "La contraseña no es correcta.")
		return false
	}
	ok, _, err := s.verifyFactor(r.Context(), p.UserID, code)
	if err != nil {
		serverError(w, r, err)
		return false
	}
	if !ok {
		s.limiter.fail(key)
		writeError(w, http.StatusBadRequest, errCodeBad)
		return false
	}
	s.limiter.reset(key)
	return true
}

func (s *Server) writeFactorResult(w http.ResponseWriter, r *http.Request, userID string, codes []string) {
	fresh, err := loadPrincipal(r.Context(), s.db, userID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	out := map[string]any{"session": sessionOf(fresh)}
	if codes != nil {
		out["recovery_codes"] = codes
	}
	writeJSON(w, http.StatusOK, out)
}

// replaceRecoveryCodes drops the old codes and stores ten new ones, returning them in clear this one time.
func (s *Server) replaceRecoveryCodes(ctx context.Context, tx pgx.Tx, p *Principal) ([]string, error) {
	if _, err := tx.Exec(ctx, `DELETE FROM totp_recovery_codes WHERE user_id = $1`, p.UserID); err != nil {
		return nil, err
	}
	var clinic any
	if p.ClinicID != "" {
		clinic = p.ClinicID
	}
	codes := make([]string, 0, recoveryCodeCount)
	for i := 0; i < recoveryCodeCount; i++ {
		code, err := newRecoveryCode()
		if err != nil {
			return nil, err
		}
		salt, err := newSalt()
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO totp_recovery_codes (user_id, clinic_id, salt, code_hash) VALUES ($1,$2,$3,$4)`,
			p.UserID, clinic, salt, recoveryHash(salt, code)); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}

// ---- sign-in, second step ----------------------------------------------------------------------------------

// The challenge is a JWT signed with a key derived from the session secret under its own label, with its own
// audience: it can never be read as a session cookie, and a session token can never be read as a challenge.
func (s *Server) challengeKey() []byte {
	mac := hmac.New(sha256.New, s.cfg.JWTSecret)
	mac.Write([]byte("2fa-challenge-v1"))
	return mac.Sum(nil)
}

const challengeAud = "2fa-challenge"

func (s *Server) issueChallenge(userID string, tokenVersion int) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID, Audience: jwt.ClaimStrings{challengeAud},
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(challengeTTL)),
		},
		TV: tokenVersion,
	}).SignedString(s.challengeKey())
}

func (s *Server) parseChallenge(raw string) (userID string, tokenVersion int, err error) {
	var c claims
	tok, err := jwt.ParseWithClaims(raw, &c, func(*jwt.Token) (any, error) { return s.challengeKey(), nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithAudience(challengeAud))
	if err != nil || !tok.Valid || c.Subject == "" {
		return "", 0, errors.New("invalid challenge")
	}
	return c.Subject, c.TV, nil
}

// beginTwoFactor answers a correct password with a challenge instead of a session.
func (s *Server) beginTwoFactor(w http.ResponseWriter, r *http.Request, p *Principal) {
	ch, err := s.issueChallenge(p.UserID, p.TokenVersion)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"needs_2fa": true, "challenge": ch})
}

func (s *Server) loginTwoFactor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Challenge    string `json:"challenge"`
		Code         string `json:"code"`
		RecoveryCode string `json:"recovery_code"`
	}
	if !decode(w, r, &req) {
		return
	}
	const generic = "El código no es correcto o el acceso expiró. Vuelve a empezar."
	input := req.Code
	if input == "" {
		input = req.RecoveryCode
	}
	uid, tv, err := s.parseChallenge(req.Challenge)
	if err != nil || strings.TrimSpace(input) == "" {
		writeError(w, http.StatusUnauthorized, generic)
		return
	}
	ipKey, userKey := "2fa-ip|"+clientIP(r), "2fa|"+uid
	if !s.limiter.allow(ipKey) || !s.limiter.allow(userKey) {
		writeError(w, http.StatusTooManyRequests, "Demasiados intentos. Inténtalo de nuevo en unos minutos.")
		return
	}
	p, err := loadPrincipal(r.Context(), s.db, uid)
	if errors.Is(err, errNoUser) || (err == nil && (p.TokenVersion != tv || !p.TwoFactorEnabled)) {
		s.limiter.fail(ipKey)
		writeError(w, http.StatusUnauthorized, generic)
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if p.Disabled {
		writeError(w, http.StatusForbidden, "Esta cuenta está desactivada. Contacta al administrador de tu consultorio.")
		return
	}
	ok, usedRecovery, err := s.verifyFactor(r.Context(), uid, input)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if !ok {
		s.limiter.fail(ipKey)
		s.limiter.fail(userKey)
		writeError(w, http.StatusUnauthorized, generic)
		return
	}
	s.limiter.reset(userKey)
	_, _ = s.db.Exec(r.Context(), `UPDATE users SET last_login_at = now() WHERE id = $1`, uid)
	token, err := s.issueToken(p.UserID, p.TokenVersion)
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.setSessionCookie(w, token)
	out := map[string]any{"session": sessionOf(p)}
	if usedRecovery {
		out["recovery_codes_left"] = s.recoveryLeft(r.Context(), uid)
		audit(r.Context(), s.db, p.ClinicID, p, "2fa_recovery_used", p.actorName()+" entró con un código de recuperación", nil)
	}
	writeJSON(w, http.StatusOK, out)
}
