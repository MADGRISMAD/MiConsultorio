package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"html"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

const resetTTL = time.Hour

// sendMail delivers in the background: a slow or failing mail server must never hold up a request.
func (s *Server) sendMail(m mail.Message) {
	if s.mailer == nil || !s.mailer.Enabled() || len(m.To) == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := s.mailer.Send(ctx, m); err != nil {
			log.Printf("mail to %d recipient(s) (%q): %v", len(m.To), m.Subject, err)
		}
	}()
}

func (s *Server) mailEnabled() bool { return s.mailer != nil && s.mailer.Enabled() }

// appLink builds an absolute link into the web app.
func (s *Server) appLink(path string) string {
	base := s.cfg.AppURL
	if base == "" {
		base = s.cfg.APIPublicURL
	}
	return base + path
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

func esc(s string) string { return html.EscapeString(s) }

// layout wraps content in a simple, mail-client-safe frame (tables and inline styles only).
func layout(title, bodyHTML string) string {
	return `<!doctype html><html lang="es"><body style="margin:0;background:#f4f8fb;font-family:Segoe UI,Helvetica,Arial,sans-serif;color:#0b2540">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f8fb;padding:24px 12px"><tr><td align="center">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:520px;background:#ffffff;border-radius:16px;padding:32px">
<tr><td style="font-size:20px;font-weight:700;letter-spacing:-0.01em;color:#1673d1;padding-bottom:18px">Caresia</td></tr>
<tr><td style="font-size:22px;font-weight:600;padding-bottom:12px">` + esc(title) + `</td></tr>
<tr><td style="font-size:15px;line-height:1.55;color:#33475b">` + bodyHTML + `</td></tr>
</table>
<p style="font-size:12px;color:#7a8b9b;max-width:520px">Recibes este correo porque se usó tu dirección en Caresia. Si no fuiste tú, puedes ignorarlo.</p>
</td></tr></table></body></html>`
}

func button(href, label string) string {
	return `<p style="margin:22px 0"><a href="` + esc(href) + `" style="background:#0b2540;color:#ffffff;text-decoration:none;padding:12px 22px;border-radius:999px;font-weight:600;display:inline-block">` + esc(label) + `</a></p>` +
		`<p style="font-size:13px;color:#7a8b9b">Si el botón no abre, copia este enlace en tu navegador:<br><span style="word-break:break-all">` + esc(href) + `</span></p>`
}

// ---------------------------------------------------------------------------
// Password recovery
// ---------------------------------------------------------------------------

func hashToken(t string) []byte {
	h := sha256.Sum256([]byte(t))
	return h[:]
}

// forgotPassword always answers the same way, so it can't be used to discover which accounts exist.
func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identifier string `json:"identifier"`
	}
	if !decode(w, r, &req) {
		return
	}
	ident := strings.ToLower(strings.TrimSpace(req.Identifier))
	if ident == "" || len(ident) > 200 {
		writeError(w, http.StatusBadRequest, "Escribe tu correo o tu usuario.")
		return
	}
	if !s.mailEnabled() {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "El envío de correos no está configurado. Pídele a un administrador que cambie tu contraseña."})
		return
	}
	ipKey, idKey := "forgot|"+clientIP(r), "forgot|"+ident
	if !s.forgots.allow(ipKey) || !s.forgots.allow(idKey) {
		writeError(w, http.StatusTooManyRequests, "Demasiados intentos. Inténtalo de nuevo en un rato.")
		return
	}
	s.forgots.fail(ipKey)
	s.forgots.fail(idKey)

	var userID, name, email string
	var disabled bool
	err := s.db.QueryRow(r.Context(),
		`SELECT id, name, coalesce(email, ''), disabled FROM users WHERE lower(email) = $1 OR lower(username) = $1`, ident).Scan(&userID, &name, &email, &disabled)
	reply := func() { writeJSON(w, http.StatusOK, map[string]any{"ok": true}) }
	if errors.Is(err, pgx.ErrNoRows) {
		reply()
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	// Permanent platform admins only get a new password from another permanent admin (see permanent.go).
	if disabled || email == "" || isPermanentAdmin(email) {
		reply()
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		serverError(w, r, err)
		return
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `DELETE FROM password_resets WHERE user_id = $1 AND (used_at IS NOT NULL OR expires_at < now())`, userID); err != nil {
			return err
		}
		_, err := tx.Exec(r.Context(), `INSERT INTO password_resets (user_id, token_hash, expires_at) VALUES ($1,$2,$3)`, userID, hashToken(token), time.Now().Add(resetTTL))
		return err
	}); err != nil {
		serverError(w, r, err)
		return
	}
	link := s.appLink("/restablecer?token=" + url.QueryEscape(token))
	hello := "Hola"
	if name != "" {
		hello = "Hola " + name
	}
	s.sendMail(mail.Message{
		To:      []string{email},
		Subject: "Restablece tu contraseña de Caresia",
		Text:    hello + ",\n\nPara elegir una contraseña nueva abre este enlace (vale 1 hora y solo sirve una vez):\n" + link + "\n\nSi no lo pediste tú, ignora este correo: tu contraseña no cambia.\n",
		HTML: layout("Restablece tu contraseña", "<p>"+esc(hello)+",</p><p>Recibimos una solicitud para cambiar tu contraseña. El enlace vale <strong>1 hora</strong> y solo sirve una vez.</p>"+
			button(link, "Elegir contraseña nueva")+"<p>Si no lo pediste tú, ignora este correo: tu contraseña no cambia.</p>"),
	})
	reply()
}

// resetPassword sets a new password from a recovery link and ends every session of that account.
func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if msg := db.ValidatePassword(req.Password); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	ipKey := "reset|" + clientIP(r)
	if !s.limiter.allow(ipKey) {
		writeError(w, http.StatusTooManyRequests, "Demasiados intentos. Inténtalo de nuevo en unos minutos.")
		return
	}
	hash, err := db.HashPassword(req.Password)
	if err != nil {
		serverError(w, r, err)
		return
	}
	errInvalid := fail(http.StatusBadRequest, "El enlace no es válido o ya venció. Pide uno nuevo.")
	var clinicID, who string
	err = inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var id, userID string
		var expires time.Time
		var used *time.Time
		err := tx.QueryRow(r.Context(), `SELECT id, user_id, expires_at, used_at FROM password_resets WHERE token_hash = $1 FOR UPDATE`, hashToken(strings.TrimSpace(req.Token))).
			Scan(&id, &userID, &expires, &used)
		if errors.Is(err, pgx.ErrNoRows) {
			return errInvalid
		}
		if err != nil {
			return err
		}
		if used != nil || time.Now().After(expires) {
			return errInvalid
		}
		var disabled bool
		var email string
		if err := tx.QueryRow(r.Context(), `SELECT coalesce(clinic_id::text, ''), name, disabled, coalesce(email, '') FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&clinicID, &who, &disabled, &email); err != nil {
			return err
		}
		if disabled || isPermanentAdmin(email) {
			return errInvalid
		}
		if _, err := tx.Exec(r.Context(), `UPDATE users SET password_hash = $2, token_version = token_version + 1 WHERE id = $1`, userID, hash); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE password_resets SET used_at = now() WHERE user_id = $1 AND used_at IS NULL`, userID); err != nil {
			return err
		}
		audit(r.Context(), tx, clinicID, nil, "password_reset", who+" restableció su contraseña con un enlace de correo", nil)
		return nil
	})
	if err != nil {
		s.limiter.fail(ipKey)
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------------------------------------------------------------------
// Notices
// ---------------------------------------------------------------------------

func (s *Server) welcomeEmail(to, name, clinicName string) {
	hello := "Hola"
	if name != "" {
		hello += " " + name
	}
	link := s.appLink("/login")
	s.sendMail(mail.Message{
		To:      []string{to},
		Subject: "Bienvenido a Caresia",
		Text:    hello + ",\n\nTu consultorio «" + clinicName + "» ya está creado y tienes 14 días de prueba.\nEntra aquí: " + link + "\n",
		HTML: layout("Bienvenido a Caresia", "<p>"+esc(hello)+",</p><p>Tu consultorio <strong>"+esc(clinicName)+"</strong> ya está creado y tienes <strong>14 días de prueba</strong>. "+
			"Te ayudamos a configurarlo en unos minutos: giro, horario y equipo.</p>"+button(link, "Entrar a Caresia")),
	})
}

// clinicAdminEmails are the addresses of a clinic's active administrators.
func clinicAdminEmails(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, clinicID string) []string {
	rows, err := q.Query(ctx, `SELECT email FROM users WHERE clinic_id = $1 AND role = 'admin' AND NOT disabled AND coalesce(email, '') <> ''`, clinicID)
	if err != nil {
		return nil
	}
	out, _ := pgx.CollectRows(rows, pgx.RowTo[string])
	return out
}

func (s *Server) paymentReceivedEmail(ctx context.Context, clinicID, planName string, months, amountCents int, periodEnd time.Time) {
	to := clinicAdminEmails(ctx, s.db, clinicID)
	if len(to) == 0 {
		return
	}
	end := periodEnd.Format("02/01/2006")
	s.sendMail(mail.Message{
		To:      to,
		Subject: "Recibimos tu pago en Caresia",
		Text:    "Recibimos tu pago de $" + cents(amountCents) + " por " + itoa(months) + " mes(es) del plan " + planName + ".\nTu plan está activo hasta el " + end + ".\n",
		HTML: layout("Pago recibido", "<p>Recibimos tu pago de <strong>$"+esc(cents(amountCents))+"</strong> por <strong>"+itoa(months)+" mes(es)</strong> del plan <strong>"+esc(planName)+"</strong>.</p>"+
			"<p>Tu plan está activo hasta el <strong>"+end+"</strong>. ¡Gracias por confiar en Caresia!</p>"),
	})
}

// ---------------------------------------------------------------------------
// Ticket by e-mail
// ---------------------------------------------------------------------------

var methodNames = map[string]string{"cash": "Efectivo", "card": "Tarjeta", "transfer": "Transferencia", "mp_point": "Terminal Mercado Pago", "mp_link": "Liga de Mercado Pago", "other": "Otro"}

func qtyStr(q float64) string { return strconv.FormatFloat(q, 'f', -1, 64) }

func (s *Server) emailSale(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Venta no encontrada.")
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if !strings.Contains(req.Email, "@") || len(req.Email) > 160 || strings.ContainsAny(req.Email, "\r\n<>,; ") {
		writeError(w, http.StatusBadRequest, "Escribe un correo válido.")
		return
	}
	if !s.mailEnabled() {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Code: "NOT_CONFIGURED", Message: "El envío de correos no está configurado en el servidor."})
		return
	}
	p := principalFrom(r.Context())
	if !s.mailLimiter.allow("mail|" + p.UserID) {
		writeError(w, http.StatusTooManyRequests, "Has enviado muchos correos. Inténtalo más tarde.")
		return
	}
	sale, err := loadSale(r.Context(), s.db, p.ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Venta no encontrada.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	cfg, err := loadPosSettings(r.Context(), s.db, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var clinicName string
	_ = s.db.QueryRow(r.Context(), `SELECT name FROM clinics WHERE id = $1`, p.ClinicID).Scan(&clinicName)
	biz := cfg.BusinessName
	if biz == "" {
		biz = clinicName
	}
	s.mailLimiter.fail("mail|" + p.UserID)

	var text, rows strings.Builder
	text.WriteString(biz + "\nTicket #" + itoa(sale.Folio) + " · " + sale.CreatedAt.Local().Format("02/01/2006 15:04") + "\n\n")
	for _, l := range sale.Lines {
		text.WriteString(qtyStr(l.Qty) + " x " + l.Name + "  $" + cents(l.TotalCents+l.DiscountCents) + "\n")
		rows.WriteString(`<tr><td style="padding:6px 0">` + esc(qtyStr(l.Qty)) + ` × ` + esc(l.Name) + `</td><td align="right" style="padding:6px 0;white-space:nowrap">$` + cents(l.TotalCents+l.DiscountCents) + `</td></tr>`)
	}
	if extra := sale.DiscountCents; extra > 0 {
		text.WriteString("Descuento  -$" + cents(extra) + "\n")
		rows.WriteString(`<tr><td style="padding:6px 0">Descuento</td><td align="right">-$` + cents(extra) + `</td></tr>`)
	}
	text.WriteString("\nTOTAL  $" + cents(sale.TotalCents) + "\n")
	rows.WriteString(`<tr><td style="padding:10px 0;border-top:1px solid #dde6ee;font-weight:700">Total</td><td align="right" style="padding:10px 0;border-top:1px solid #dde6ee;font-weight:700">$` + cents(sale.TotalCents) + `</td></tr>`)
	if cfg.ShowTaxLine && sale.TaxCents > 0 {
		text.WriteString("IVA incluido  $" + cents(sale.TaxCents) + "\n")
	}
	for _, pay := range sale.Payments {
		name := methodNames[pay.Method]
		text.WriteString(name + "  $" + cents(pay.AmountCents) + "\n")
		rows.WriteString(`<tr><td style="padding:3px 0;color:#7a8b9b">` + esc(name) + `</td><td align="right" style="color:#7a8b9b">$` + cents(pay.AmountCents) + `</td></tr>`)
	}
	if cfg.TicketFooter != "" {
		text.WriteString("\n" + cfg.TicketFooter + "\n")
	}
	fiscal := ""
	if cfg.RFC != "" {
		fiscal = `<p style="font-size:12px;color:#7a8b9b">` + esc(cfg.LegalName) + ` · RFC ` + esc(cfg.RFC) + `</p>`
	}
	s.sendMail(mail.Message{
		To:      []string{req.Email},
		Subject: "Tu ticket de " + biz + " (#" + itoa(sale.Folio) + ")",
		Text:    text.String(),
		HTML: layout("Gracias por tu visita", `<p><strong>`+esc(biz)+`</strong><br>Ticket #`+itoa(sale.Folio)+` · `+sale.CreatedAt.Local().Format("02/01/2006 15:04")+`</p>`+
			`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="font-size:14px">`+rows.String()+`</table>`+fiscal+
			`<p>`+esc(cfg.TicketFooter)+`</p>`),
	})
	audit(r.Context(), s.db, p.ClinicID, p, "ticket_emailed", "Envió por correo el ticket #"+itoa(sale.Folio), nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
