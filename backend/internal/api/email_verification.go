package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// Verification of the e-mail when signing up: a 6-digit code is mailed and the account is only created with it. It stops
// made-up addresses and mass sign-ups. Without outgoing mail on the server there is no way to send it, so it is not asked for.

const (
	codeTTL         = 10 * time.Minute
	codeCooldown    = time.Minute
	codeMaxPerHour  = 5
	codeMaxTries    = 5
	codeResendLimit = 10 // requests per IP and hour
)

// signupVerifyRequired: the server can send mail and SIGNUP_VERIFY is not "off".
func (s *Server) signupVerifyRequired() bool { return s.cfg.SignupVerify && s.mailEnabled() }

func (s *Server) signupCodeHash(email, code string) string {
	mac := hmac.New(sha256.New, s.cfg.JWTSecret)
	mac.Write([]byte("signup-code|" + email + ":" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

// digitsOnly keeps the digits of what the person typed ("123 456" works).
func digitsOnly(v string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, v)
}

// sendSignupCode is step 1 of the sign-up: POST /register/code mails a new code to the address.
func (s *Server) sendSignupCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if msg := db.ValidateEmail(email); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if !s.signupVerifyRequired() {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "required": false})
		return
	}
	if !limit(w, s.codeIP, "code|"+clientIP(r)) {
		return
	}
	var taken bool
	if err := s.db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM users WHERE lower(email) = $1)`, email).Scan(&taken); err != nil {
		serverError(w, r, err)
		return
	}
	if taken {
		writeError(w, http.StatusConflict, "Ya existe una cuenta con ese correo.")
		return
	}

	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		serverError(w, r, err)
		return
	}
	code := fmt.Sprintf("%06d", n.Int64())
	var wait time.Duration
	err = inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var sentAt time.Time
		var sends []time.Time
		err := tx.QueryRow(r.Context(), `SELECT sent_at, sends FROM email_codes WHERE email = $1 FOR UPDATE`, email).Scan(&sentAt, &sends)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		now := time.Now()
		if err == nil {
			if since := now.Sub(sentAt); since < codeCooldown {
				wait = codeCooldown - since
				return nil
			}
			recent := sends[:0]
			for _, t := range sends {
				if now.Sub(t) < time.Hour {
					recent = append(recent, t)
				}
			}
			sends = recent
			if len(sends) >= codeMaxPerHour {
				wait = -1
				return nil
			}
		}
		sends = append(sends, now)
		_, err = tx.Exec(r.Context(), `
			INSERT INTO email_codes (email, code_hash, tries, sent_at, sends, expires_at) VALUES ($1, $2, 0, $3, $4, $5)
			ON CONFLICT (email) DO UPDATE SET code_hash = $2, tries = 0, sent_at = $3, sends = $4, expires_at = $5`,
			email, s.signupCodeHash(email, code), now, sends, now.Add(codeTTL))
		return err
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	switch {
	case wait > 0:
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("Espera %d s para pedir otro código.", int(wait.Seconds())+1))
		return
	case wait < 0:
		writeError(w, http.StatusTooManyRequests, "Ya pediste varios códigos para este correo. Inténtalo en una hora.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if err := s.mailer.Send(ctx, signupCodeMessage(email, code)); err != nil {
		_, _ = s.db.Exec(context.WithoutCancel(r.Context()), `DELETE FROM email_codes WHERE email = $1`, email) // nothing was sent: no code to wait for
		writeError(w, http.StatusBadGateway, "No pudimos enviar el correo. Revisa que esté bien escrito e inténtalo de nuevo.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "required": true})
}

func signupCodeMessage(to, code string) mail.Message {
	return mail.Message{
		To:      []string{to},
		Subject: code + " es tu código de verificación · Caresia",
		Text:    "Usa este código para terminar de crear tu cuenta en Caresia:\n\n" + code + "\n\nVence en 10 minutos. Si no fuiste tú, ignora este correo: no se creó ninguna cuenta.\n",
		HTML: layout("Verifica tu correo", "<p>Usa este código para terminar de crear tu cuenta en Caresia:</p>"+
			`<p style="margin:18px 0;font-size:34px;font-weight:800;letter-spacing:8px;text-align:center;color:#0b2540">`+esc(code)+`</p>`+
			`<p style="font-size:13px;color:#7a8b9b">El código vence en 10 minutos. Si no fuiste tú, ignora este correo: no se creó ninguna cuenta.</p>`),
	}
}

// checkSignupCode checks the code the person typed against the one mailed to the address, without spending it (the sign-up
// can still fail on a taken username and be retried with the same code). Five wrong tries kill the code.
func (s *Server) checkSignupCode(ctx context.Context, email, typed string) *httpError {
	code := digitsOnly(typed)
	if len(code) != 6 {
		e := fail(http.StatusBadRequest, "Escribe el código de 6 dígitos que te enviamos al correo.")
		e.Code = "EMAIL_CODE_REQUIRED"
		return e
	}
	expired := func() *httpError {
		e := fail(http.StatusBadRequest, "El código venció. Pide uno nuevo.")
		e.Code = "EMAIL_CODE_EXPIRED"
		return e
	}
	var hash string
	var tries int
	var expiresAt time.Time
	err := s.db.QueryRow(ctx, `SELECT code_hash, tries, expires_at FROM email_codes WHERE email = $1`, email).Scan(&hash, &tries, &expiresAt)
	if err != nil || time.Now().After(expiresAt) || tries >= codeMaxTries {
		return expired()
	}
	if subtle.ConstantTimeCompare([]byte(hash), []byte(s.signupCodeHash(email, code))) == 1 {
		return nil
	}
	var now int
	if err := s.db.QueryRow(ctx, `UPDATE email_codes SET tries = tries + 1 WHERE email = $1 RETURNING tries`, email).Scan(&now); err == nil && now >= codeMaxTries {
		_, _ = s.db.Exec(ctx, `DELETE FROM email_codes WHERE email = $1`, email)
		e := fail(http.StatusBadRequest, "Demasiados intentos. Pide un código nuevo.")
		e.Code = "EMAIL_CODE_EXPIRED"
		return e
	}
	e := fail(http.StatusBadRequest, "El código no es correcto.")
	e.Code = "EMAIL_CODE_WRONG"
	return e
}

func (s *Server) spendSignupCode(ctx context.Context, email string) {
	_, _ = s.db.Exec(ctx, `DELETE FROM email_codes WHERE email = $1`, email)
}
