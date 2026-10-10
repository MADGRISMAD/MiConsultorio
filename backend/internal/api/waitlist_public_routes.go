package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// wlPublic holds the public (no session) handlers of the waitlist, with their own rate limiters.
type wlPublic struct {
	*Server
	reads   *rateLimiter // any public read, per IP
	badTok  *rateLimiter // unknown tokens, per IP
	writes  *rateLimiter // accept / decline / leave, per IP
	joinIP  *rateLimiter // sign-ups, per IP
	joinKey *rateLimiter // sign-ups, per contact
}

// Active sign-ups one e-mail may hold in a clinic at once.
const wlMaxPerEmail = 5

// mountPublicWaitlist: Public waitlist sign-up and offers.
func (s *Server) mountPublicWaitlist(r chi.Router) {
	p := &wlPublic{
		Server:  s,
		reads:   newRateLimiter(300, 10*time.Minute),
		badTok:  newRateLimiter(15, 15*time.Minute),
		writes:  newRateLimiter(30, 10*time.Minute),
		joinIP:  newRateLimiter(6, time.Hour),
		joinKey: newRateLimiter(3, time.Hour),
	}
	r.Post("/public/booking/{slug}/waitlist", p.join)
	r.Get("/public/waitlist/{token}", p.view)
	r.Post("/public/waitlist/{token}", p.act)
}

type wlJoinRequest struct {
	Names          string `json:"names"`
	LastNames      string `json:"last_names"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	ProfessionalID string `json:"professional_id"`
	ServiceID      string `json:"service_id"`
	Days           []int  `json:"days"`
	FromTime       string `json:"from_time"`
	ToTime         string `json:"to_time"`
	Notes          string `json:"notes"`
	AcceptPrivacy  bool   `json:"accept_privacy"`
	AcceptNotices  bool   `json:"accept_notices"`
	Website        string `json:"website"` // honeypot: real people never fill it
}

// join is POST /public/booking/{slug}/waitlist: "avísame si se libera un lugar".
func (p *wlPublic) join(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !p.joinIP.allow("wl|" + ip) {
		tooMany(w)
		return
	}
	var req wlJoinRequest
	if !decode(w, r, &req) {
		return
	}
	ctx := r.Context()
	c, err := p.loadBookingClinic(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		p.badTok.fail("tok|" + ip)
		writeError(w, http.StatusNotFound, "Esta página de citas no está disponible.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if req.Website != "" { // a bot: pretend it worked
		p.joinIP.fail("wl|" + ip)
		writeJSON(w, http.StatusCreated, map[string]any{"waitlist": map[string]any{"status": "waiting"}})
		return
	}
	in := wlIn{
		Name: strings.TrimSpace(req.Names + " " + req.LastNames), Phone: req.Phone, Email: req.Email, Days: req.Days,
		FromTime: req.FromTime, ToTime: req.ToTime, Notes: req.Notes, Consent: req.AcceptNotices,
	}
	if req.ProfessionalID != "" {
		in.ProfessionalID = &req.ProfessionalID
	}
	if req.ServiceID != "" {
		in.ServiceID = &req.ServiceID
	}
	switch {
	case strings.TrimSpace(req.Names) == "" || strings.TrimSpace(req.LastNames) == "":
		writeError(w, http.StatusBadRequest, "Escribe tu nombre y apellidos.")
		return
	case strings.TrimSpace(req.Email) == "":
		writeError(w, http.StatusBadRequest, "Escribe tu correo: ahí te avisaremos si se libera un lugar.")
		return
	case utf8.RuneCountInString(req.Names) > 100 || utf8.RuneCountInString(req.LastNames) > 100:
		writeError(w, http.StatusBadRequest, "Escribe tu nombre y apellidos.")
		return
	case !req.AcceptPrivacy:
		writeError(w, http.StatusBadRequest, "Debes aceptar el aviso de privacidad para anotarte.")
		return
	case !req.AcceptNotices:
		writeError(w, http.StatusBadRequest, "Necesitamos tu permiso para avisarte por correo cuando se libere un lugar.")
		return
	}
	if msg := in.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	// Only the people and services the public page offers.
	if in.ProfessionalID != nil {
		pros, err := p.listBookable(ctx, c, *in.ProfessionalID)
		if err != nil {
			serverError(w, r, err)
			return
		}
		if !validUUID(*in.ProfessionalID) || len(pros) != 1 {
			writeError(w, http.StatusBadRequest, "Elige un profesional.")
			return
		}
	}
	if in.ServiceID != nil && !p.serviceOK(ctx, c.ID, *in.ServiceID) {
		writeError(w, http.StatusBadRequest, "El servicio no es válido.")
		return
	}
	if !p.joinKey.allow("wlk|" + c.ID + "|" + in.Email) {
		tooMany(w)
		return
	}
	token, err := newToken()
	if err != nil {
		serverError(w, r, err)
		return
	}
	var created bool
	err = inTx(ctx, p.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "waitlist-join|"+c.ID+"|"+in.Email); err != nil {
			return err
		}
		var open, same int
		if err := tx.QueryRow(ctx, `
			SELECT count(*), count(*) FILTER (WHERE professional_id IS NOT DISTINCT FROM $3::uuid AND service_id IS NOT DISTINCT FROM $4::uuid)
			FROM waitlist_entries WHERE clinic_id = $1 AND lower(email) = $2 AND status IN ('waiting', 'offered')`,
			c.ID, in.Email, in.ProfessionalID, in.ServiceID).Scan(&open, &same); err != nil {
			return err
		}
		if same > 0 || open >= wlMaxPerEmail {
			return nil // already on the list (or too many): answer as if it worked, without telling who is on it
		}
		created = true
		if _, err := tx.Exec(ctx, `
			INSERT INTO waitlist_entries (clinic_id, name, phone, email, professional_id, service_id, days, from_time, to_time, notes, consent, token, created_via)
			VALUES ($1, $2, $3, $4, $5::uuid, $6::uuid, $7::smallint[], NULLIF($8, '')::time, NULLIF($9, '')::time, $10, true, $11, 'online')`,
			c.ID, in.Name, in.Phone, in.Email, in.ProfessionalID, in.ServiceID, in.Days, in.FromTime, in.ToTime, in.Notes, token); err != nil {
			return err
		}
		audit(ctx, tx, c.ID, nil, "waitlist_join", "Alguien se anotó en la lista de espera desde la reserva en línea", map[string]any{"via": "online"})
		return nil
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	p.joinIP.fail("wl|" + ip)
	p.joinKey.fail("wlk|" + c.ID + "|" + in.Email)
	if created {
		waitlistWake()
		p.wlJoinMail(c, in, token)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"waitlist": map[string]any{"status": "waiting"}})
}

func (s *Server) wlJoinMail(c *bookingClinic, in wlIn, token string) {
	link := s.appLink("/espera/" + token)
	hello := "Hola " + firstName(in.Name)
	subject := "Estás en la lista de espera de " + c.Name
	text := hello + ", te anotamos en la lista de espera de " + c.Name + ". Si se libera un lugar que te acomode, te avisaremos por este medio.\n\n" +
		"Si ya no lo necesitas, puedes salir de la lista aquí:\n" + link + "\n"
	body := `<p style="margin:0 0 8px">` + esc(hello) + `, te anotamos en la lista de espera de <strong>` + esc(c.Name) + `</strong>.</p>` +
		`<p style="margin:0 0 12px">Si se libera un lugar que te acomode, te avisaremos por este medio.</p>` + button(link, "Administrar mi lugar en la lista")
	s.sendMail(mail.Message{To: []string{in.Email}, Subject: subject, Text: text, HTML: layout("Lista de espera", body)})
}

// wlView is what the public page of a token shows.
type wlView struct {
	Status string
	Name   string
	Clinic struct{ Name, Address, Phone string }
	Offer  *struct {
		ID, Professional, Date, Start, End string
		ExpiresAt                          time.Time
	}
	Expired bool
}

func (p *wlPublic) load(ctx context.Context, q queryRower, token string) (*wlView, string, error) {
	var v wlView
	var clinicID string
	var oid, opro, odate, ostart, oend *string
	var oexp *time.Time
	err := q.QueryRow(ctx, `
		SELECT e.status, e.name, c.id::text, c.name, c.address, c.phone_number,
		       o.id::text, coalesce(u.name, ''), to_char(o.date, 'YYYY-MM-DD'), to_char(o.start_hour, 'HH24:MI'), to_char(o.end_hour, 'HH24:MI'), o.expires_at
		FROM waitlist_entries e
		JOIN clinics c ON c.id = e.clinic_id
		LEFT JOIN waitlist_offers o ON o.entry_id = e.id AND o.status = 'offered'
		LEFT JOIN users u ON u.id = o.professional_id
		WHERE e.token = $1`, token).
		Scan(&v.Status, &v.Name, &clinicID, &v.Clinic.Name, &v.Clinic.Address, &v.Clinic.Phone, &oid, &opro, &odate, &ostart, &oend, &oexp)
	if err != nil {
		return nil, "", err
	}
	v.Name = firstName(v.Name)
	if oid != nil {
		if oexp.After(time.Now()) {
			v.Offer = &struct {
				ID, Professional, Date, Start, End string
				ExpiresAt                          time.Time
			}{*oid, *opro, *odate, *ostart, *oend, *oexp}
		} else {
			v.Expired = true
		}
	}
	return &v, clinicID, nil
}

func (v *wlView) json() map[string]any {
	out := map[string]any{
		"status":  v.Status,
		"name":    v.Name,
		"clinic":  map[string]string{"name": v.Clinic.Name, "address": v.Clinic.Address, "phone": v.Clinic.Phone},
		"expired": v.Expired,
		"offer":   nil,
	}
	if v.Offer != nil {
		out["offer"] = map[string]any{
			"professional": v.Offer.Professional, "date": v.Offer.Date, "start": v.Offer.Start, "end": v.Offer.End, "expires_at": v.Offer.ExpiresAt,
		}
	}
	return out
}

func (p *wlPublic) view(w http.ResponseWriter, r *http.Request) {
	if !limit(w, p.reads, "read|"+clientIP(r)) {
		return
	}
	token := chi.URLParam(r, "token")
	if !p.badTok.allow("tok|" + clientIP(r)) {
		tooMany(w)
		return
	}
	if !validToken(token) {
		p.badTok.fail("tok|" + clientIP(r))
		writeError(w, http.StatusNotFound, "No encontramos esa solicitud.")
		return
	}
	v, _, err := p.load(r.Context(), p.db, token)
	if errors.Is(err, pgx.ErrNoRows) {
		p.badTok.fail("tok|" + clientIP(r))
		writeError(w, http.StatusNotFound, "No encontramos esa solicitud.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"waitlist": v.json()})
}

// act is POST /public/waitlist/{token} with {"action": "accept" | "decline" | "leave"}.
func (p *wlPublic) act(w http.ResponseWriter, r *http.Request) {
	if !limit(w, p.writes, "act|"+clientIP(r)) {
		return
	}
	var in struct {
		Action string `json:"action"`
	}
	if !decode(w, r, &in) {
		return
	}
	token := chi.URLParam(r, "token")
	ctx := r.Context()
	if !p.badTok.allow("tok|" + clientIP(r)) {
		tooMany(w)
		return
	}
	notFound := func() {
		p.badTok.fail("tok|" + clientIP(r))
		writeError(w, http.StatusNotFound, "No encontramos esa solicitud.")
	}
	if !validToken(token) {
		notFound()
		return
	}
	if in.Action != "accept" && in.Action != "decline" && in.Action != "leave" {
		writeError(w, http.StatusBadRequest, "La acción no es válida.")
		return
	}
	v, clinicID, err := p.load(ctx, p.db, token)
	if errors.Is(err, pgx.ErrNoRows) {
		notFound()
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	var refused string
	switch in.Action {
	case "accept":
		refused, err = p.wlAccept(ctx, token, v, clinicID)
	default:
		err = p.wlRespond(ctx, token, clinicID, in.Action)
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if refused != "" {
		writeJSON(w, http.StatusConflict, errorBody{Code: "NOT_ALLOWED", Message: refused})
		return
	}
	v, _, err = p.load(ctx, p.db, token)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"waitlist": v.json()})
}

// wlRespond declines the live offer (the person goes back to waiting) or takes the person off the list.
func (s *Server) wlRespond(ctx context.Context, token, clinicID, action string) error {
	changed := false
	err := inTx(ctx, s.db, func(tx pgx.Tx) error {
		var id, status string
		if err := tx.QueryRow(ctx, `SELECT id::text, status FROM waitlist_entries WHERE token = $1 FOR UPDATE`, token).Scan(&id, &status); err != nil {
			return err
		}
		switch {
		case action == "leave" && (status == "waiting" || status == "offered"):
			if _, err := tx.Exec(ctx, `UPDATE waitlist_offers SET status = 'cancelled', responded_at = now() WHERE entry_id = $1 AND status = 'offered'`, id); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE waitlist_entries SET status = 'cancelled', updated_at = now() WHERE id = $1`, id); err != nil {
				return err
			}
			audit(ctx, tx, clinicID, nil, "waitlist_leave", "La persona salió de la lista de espera desde su enlace", map[string]any{"entry": id})
			changed = true
		case action == "decline" && status == "offered":
			if _, err := tx.Exec(ctx, `UPDATE waitlist_offers SET status = 'declined', responded_at = now() WHERE entry_id = $1 AND status = 'offered'`, id); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE waitlist_entries SET status = 'waiting', updated_at = now() WHERE id = $1`, id); err != nil {
				return err
			}
			audit(ctx, tx, clinicID, nil, "waitlist_decline", "La persona rechazó el lugar ofrecido", map[string]any{"entry": id})
			changed = true
		}
		return nil
	})
	if changed {
		waitlistWake() // the slot is free again for the next person
	}
	return err
}

// wlAccept turns the live offer of a token into an appointment. refused is a message when it no longer
// can be (expired, taken meanwhile, nothing on offer). The slot is re-checked under the same locks the
// online booking and the offering take, so it can never be double booked.
func (s *Server) wlAccept(ctx context.Context, token string, v *wlView, clinicID string) (refused string, err error) {
	if v.Status != "offered" || (v.Offer == nil && !v.Expired) {
		if v.Status == "booked" {
			return "Ya aceptaste este lugar: tu cita está agendada.", nil
		}
		return "Este lugar ya no está disponible.", nil
	}
	var proID, date string
	if err := s.db.QueryRow(ctx, `
		SELECT o.professional_id::text, to_char(o.date, 'YYYY-MM-DD') FROM waitlist_offers o JOIN waitlist_entries e ON e.id = o.entry_id
		WHERE e.token = $1 AND o.status = 'offered'`, token).Scan(&proID, &date); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "Este lugar ya no está disponible.", nil
		}
		return "", err
	}
	var apptID string
	err = inTx(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "booking|"+clinicID+"|"+proID+"|"+date); err != nil {
			return err
		}
		if err := lockAgendaDay(ctx, tx, clinicID, date); err != nil {
			return err
		}
		var entryID, offerID, name, phone, email, via string
		var patient, service *string
		var consent bool
		var start, end string
		var expires time.Time
		err := tx.QueryRow(ctx, `
			SELECT e.id::text, o.id::text, e.name, e.phone, e.email, e.patient_id::text, o.service_id::text, e.consent, e.created_via,
			       to_char(o.start_hour, 'HH24:MI'), to_char(o.end_hour, 'HH24:MI'), o.expires_at
			FROM waitlist_entries e JOIN waitlist_offers o ON o.entry_id = e.id AND o.status = 'offered'
			WHERE e.token = $1 AND e.status = 'offered' FOR UPDATE OF e, o`, token).
			Scan(&entryID, &offerID, &name, &phone, &email, &patient, &service, &consent, &via, &start, &end, &expires)
		if errors.Is(err, pgx.ErrNoRows) {
			refused = "Este lugar ya no está disponible."
			return nil
		}
		if err != nil {
			return err
		}
		release := func(offerStatus, msg string) error {
			refused = msg
			if _, err := tx.Exec(ctx, `UPDATE waitlist_offers SET status = $2, responded_at = now() WHERE id = $1`, offerID, offerStatus); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE waitlist_entries SET status = 'waiting', updated_at = now() WHERE id = $1`, entryID)
			return err
		}
		if !expires.After(time.Now()) {
			return release("expired", "El tiempo para aceptar este lugar terminó. Seguirás en la lista por si se libera otro.")
		}
		code, err := s.slotConflict(ctx, tx, clinicID, &proID, "", date, start, end, "")
		if err != nil {
			return err
		}
		if code != SlotFree {
			return release("cancelled", "Ese horario ya no está disponible. Seguirás en la lista por si se libera otro.")
		}
		if err := s.openInSameArea(ctx, tx, clinicID, deref(patient), email, phone, &proID, ""); err != nil {
			return release("cancelled", "Ya tienes una cita pendiente en esta especialidad. Reprograma o cancela esa para tomar este lugar.")
		}
		names, last := name, ""
		if i := strings.Index(name, " "); i > 0 {
			names, last = name[:i], strings.TrimSpace(name[i+1:])
		}
		confirm, err := newToken()
		if err != nil {
			return err
		}
		newID := newRowID()
		sealed, err := encField("appointments", "details", newID, "Lista de espera")
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO appointments (id, clinic_id, curp, names, last_names, date, start_hour, end_hour, details, patient_id, professional_id, status, source,
			                          service_id, phone, email, confirm_token, reminders_consent, privacy_accepted_at)
			VALUES ($15::uuid, $1, '', $2, $3, $4::date, $5::time, $6::time, $16, $7::uuid, $8::uuid, 'scheduled', 'online',
			        $9::uuid, $10, $11, $12, $13, CASE WHEN $14 = 'online' THEN now() END)
			RETURNING id::text`,
			clinicID, names, last, date, start, end, patient, proID, service, phone, email, confirm, consent, via, newID, sealed).Scan(&apptID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE waitlist_offers SET status = 'accepted', responded_at = now(), appointment_id = $2 WHERE id = $1`, offerID, apptID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE waitlist_entries SET status = 'booked', updated_at = now() WHERE id = $1`, entryID); err != nil {
			return err
		}
		audit(ctx, tx, clinicID, nil, "waitlist_accept", "Se agendó una cita desde la lista de espera", map[string]any{"entry": entryID, "appointment_id": apptID})
		s.ntfAppointment(ctx, tx, clinicID, proID, "waitlist_accepted", "Lugar aceptado desde la lista de espera", name+" · "+date+" "+start, "/admin/navegar-citas")
		return s.scheduleReminders(ctx, tx, clinicID, apptID)
	})
	if err != nil || refused != "" {
		return refused, err
	}
	if info, ok, err := s.loadApptInfo(ctx, s.db, clinicID, apptID); err == nil && ok {
		var email string
		_ = s.db.QueryRow(ctx, `SELECT email FROM appointments WHERE id = $1`, apptID).Scan(&email)
		if email != "" {
			subject, text, html := s.bookingMail(info, false)
			s.sendMail(mail.Message{To: []string{email}, Subject: subject, Text: text, HTML: html, Attachments: info.calendarAttachments(false, s.manageLink(info.Token, ""))})
		}
	}
	return "", nil
}
