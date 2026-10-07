package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// arcoPublicAPI holds the public (no session) handlers of the ARCO form with their own rate limiters.
type arcoPublicAPI struct {
	*Server
	reads  *rateLimiter // clinic info and status reads, per IP
	badKey *rateLimiter // unknown slugs and tokens, per IP
	subIP  *rateLimiter // submissions, per IP
	subKey *rateLimiter // submissions, per clinic and e-mail
	subAll *rateLimiter // submissions, per clinic (flood guard)
}

// mountPublicArco: Public ARCO request form.
func (s *Server) mountPublicArco(r chi.Router) {
	a := &arcoPublicAPI{
		Server: s,
		reads:  newRateLimiter(120, 10*time.Minute),
		badKey: newRateLimiter(15, 15*time.Minute),
		subIP:  newRateLimiter(5, time.Hour),
		subKey: newRateLimiter(3, 24*time.Hour),
		subAll: newRateLimiter(100, 24*time.Hour),
	}
	r.Get("/public/arco/status/{token}", a.status)
	r.Get("/public/arco/{slug}", a.info)
	r.Post("/public/arco/{slug}", a.submit)
}

var arcoSlugShape = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type arcoPublicClinic struct {
	ID, Name, TZ string
}

// resolve finds the clinic by its booking slug or its short ARCO code. Every miss is the same 404.
func (a *arcoPublicAPI) resolve(w http.ResponseWriter, r *http.Request) (arcoPublicClinic, bool) {
	var c arcoPublicClinic
	slug := chi.URLParam(r, "slug")
	miss := func() (arcoPublicClinic, bool) {
		a.badKey.fail("slug|" + clientIP(r))
		writeError(w, http.StatusNotFound, "Esta página no está disponible.")
		return c, false
	}
	if !a.badKey.allow("slug|" + clientIP(r)) {
		tooMany(w)
		return c, false
	}
	if !arcoSlugShape.MatchString(slug) {
		return miss()
	}
	err := a.db.QueryRow(r.Context(), `
		SELECT c.id, c.name, coalesce(c.settings->>'timezone', '') FROM clinics c
		WHERE lower(c.arco_code) = lower($1)
		   OR c.id = (SELECT clinic_id FROM agenda_settings WHERE lower(booking_slug) = lower($1))`, slug).Scan(&c.ID, &c.Name, &c.TZ)
	if errors.Is(err, pgx.ErrNoRows) {
		return miss()
	}
	if err != nil {
		serverError(w, r, err)
		return c, false
	}
	return c, true
}

func (a *arcoPublicAPI) info(w http.ResponseWriter, r *http.Request) {
	if !limit(w, a.reads, "read|"+clientIP(r)) {
		return
	}
	c, ok := a.resolve(w, r)
	if !ok {
		return
	}
	l, err := loadLegal(r.Context(), a.db, c.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	kinds := []map[string]string{}
	for _, k := range []string{"acceso", "rectificacion", "cancelacion", "oposicion", "revocacion"} {
		kinds = append(kinds, map[string]string{"value": k, "label": arcoKindLabels[k]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"clinic": map[string]any{
		"name": c.Name, "privacy_contact": l.PrivacyContact, "privacy_email": l.PrivacyEmail, "privacy_phone": l.PrivacyPhone,
	}, "kinds": kinds})
}

func (a *arcoPublicAPI) submit(w http.ResponseWriter, r *http.Request) {
	if !limit(w, a.subIP, "sub|"+clientIP(r)) {
		return
	}
	c, ok := a.resolve(w, r)
	if !ok {
		return
	}
	var req struct {
		Kind         string `json:"kind"`
		Name         string `json:"requester_name"`
		Email        string `json:"requester_email"`
		Phone        string `json:"requester_phone"`
		Description  string `json:"description"`
		Acknowledged bool   `json:"acknowledged"`
		Website      string `json:"website"` // honeypot: people leave it empty
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Website != "" {
		// A bot: answer like a success and store nothing.
		writeJSON(w, http.StatusCreated, map[string]any{"folio": "ARCO-0000-0000", "token": "", "status_path": ""})
		return
	}
	name, okName := arcoCleanText(arcoOneLine(req.Name), 3, 120)
	desc, okDesc := arcoCleanText(req.Description, 10, 3000)
	email := strings.TrimSpace(req.Email)
	phone := arcoOneLine(req.Phone)
	var msg string
	switch {
	case !arcoValidKind(req.Kind):
		msg = "Elige el tipo de solicitud."
	case !okName:
		msg = "Escribe tu nombre completo."
	case !validEmail(email):
		msg = "Escribe un correo válido: es donde te responderemos."
	case len(phone) > 30:
		msg = "El teléfono es demasiado largo."
	case !okDesc:
		msg = "Describe tu solicitud (entre 10 y 3000 caracteres)."
	case !req.Acknowledged:
		msg = "Confirma que leíste el aviso de privacidad."
	}
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if !limit(w, a.subKey, "sub|"+c.ID+"|"+strings.ToLower(email)) || !limit(w, a.subAll, "sub|"+c.ID) {
		return
	}

	ctx := r.Context()
	loc := locationOrDefault(c.TZ)
	var id, folio, token string
	err := inTx(ctx, a.db, func(tx pgx.Tx) error {
		var err error
		id, folio, token, err = arcoCreate(ctx, tx, c.ID, arcoNew{Kind: req.Kind, Name: name, Email: email, Phone: phone, Description: desc,
			Via: "public", Actor: "Formulario público"}, loc)
		if err != nil {
			return err
		}
		audit(ctx, tx, c.ID, nil, "arco_received", "Nueva solicitud ARCO "+folio+" ("+arcoKindLabels[req.Kind]+") desde el formulario público", map[string]any{"arco_id": id, "folio": folio})
		return nil
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	a.notify(ctx, c, folio, req.Kind, email, token, loc)
	writeJSON(w, http.StatusCreated, map[string]any{"folio": folio, "token": token, "status_path": "/arco/estado?t=" + url.QueryEscape(token)})
}

// notify sends the acknowledgement to the requester and a short notice to the clinic's privacy e-mail.
// Both are optional: without SMTP nothing is sent and the request is stored all the same.
func (a *arcoPublicAPI) notify(ctx context.Context, c arcoPublicClinic, folio, kind, email, token string, loc *time.Location) {
	if !a.mailEnabled() {
		return
	}
	link := a.appLink("/arco/estado?t=" + url.QueryEscape(token))
	body := "Recibimos tu solicitud de " + strings.ToLower(arcoKindLabels[kind]) + " con folio " + folio + ".\n" +
		"El consultorio revisará tu solicitud y te comunicará la determinación por este medio dentro de los plazos que marca la ley. " +
		"Es posible que te pidamos verificar tu identidad.\n\nPuedes consultar el estado con tu folio en este enlace (guárdalo, es personal):\n" + link
	a.sendMail(mail.Message{
		To:      []string{email},
		Subject: "Recibimos tu solicitud " + folio,
		Text:    body + "\n\n" + c.Name + "\n",
		HTML: layout("Recibimos tu solicitud "+folio,
			"<p>Recibimos tu solicitud de <strong>"+esc(strings.ToLower(arcoKindLabels[kind]))+"</strong> con folio <strong>"+esc(folio)+"</strong>.</p>"+
				"<p>El consultorio revisará tu solicitud y te comunicará la determinación por este medio dentro de los plazos que marca la ley. Es posible que te pidamos verificar tu identidad.</p>"+
				button(link, "Consultar el estado")+"<p><strong>"+esc(c.Name)+"</strong></p>"),
	})
	if l, err := loadLegal(ctx, a.db, c.ID); err == nil && l.PrivacyEmail != "" && validEmail(l.PrivacyEmail) {
		a.sendMail(mail.Message{
			To:      []string{l.PrivacyEmail},
			Subject: "Nueva solicitud ARCO " + folio,
			Text:    "Llegó una solicitud ARCO (" + arcoKindLabels[kind] + ") con folio " + folio + ". Revísala en Caresia: " + a.appLink("/arco-solicitudes") + "\n",
			HTML:    layout("Nueva solicitud ARCO", "<p>Llegó una solicitud ARCO (<strong>"+esc(arcoKindLabels[kind])+"</strong>) con folio <strong>"+esc(folio)+"</strong>.</p>"+button(a.appLink("/arco-solicitudes"), "Abrir solicitudes ARCO")),
		})
	}
}

var arcoPublicStatus = map[string]string{
	"recibida":      "Recibida: el consultorio la revisará.",
	"en_revision":   "En revisión.",
	"requiere_info": "El consultorio necesita más información o verificar tu identidad. Revisa tu correo o comunícate con el consultorio.",
	"atendida":      "Atendida: el consultorio ya comunicó su determinación por el medio que registraste.",
	"negada":        "El consultorio ya comunicó su determinación por el medio que registraste.",
}

// status answers with the minimum the requester needs: no name, contact, description, notes or response text.
func (a *arcoPublicAPI) status(w http.ResponseWriter, r *http.Request) {
	if !limit(w, a.reads, "read|"+clientIP(r)) {
		return
	}
	token := chi.URLParam(r, "token")
	miss := func() {
		a.badKey.fail("tok|" + clientIP(r))
		writeError(w, http.StatusNotFound, "No encontramos esa solicitud.")
	}
	if !a.badKey.allow("tok|" + clientIP(r)) {
		tooMany(w)
		return
	}
	if !validToken(token) {
		miss()
		return
	}
	var folio, kind, status, clinic string
	var received time.Time
	var answered bool
	err := a.db.QueryRow(r.Context(), `
		SELECT r.folio, r.kind, r.status, c.name, r.received_at, r.answered_at IS NOT NULL
		FROM arco_requests r JOIN clinics c ON c.id = r.clinic_id WHERE r.token_hash = $1`, hashToken(token)).Scan(&folio, &kind, &status, &clinic, &received, &answered)
	if errors.Is(err, pgx.ErrNoRows) {
		miss()
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if status == "vencida" {
		status = "en_revision"
	}
	writeJSON(w, http.StatusOK, map[string]any{"request": map[string]any{
		"folio": folio, "kind": kind, "kind_label": arcoKindLabels[kind], "status": status, "status_label": arcoPublicStatus[status],
		"clinic": clinic, "received_on": received.Format("2006-01-02"), "answered": answered,
	}})
}
