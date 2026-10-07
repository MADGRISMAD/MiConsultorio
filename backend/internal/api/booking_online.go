package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

var slugShape = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

// bookingClinic is a clinic with online booking switched on, as the public pages see it.
type bookingClinic struct {
	ID, Name, Kind, Address, Phone, Message string
	RequiresConfirmation, ShowPrices        bool
	LeadHours, HorizonDays, SlotMinutes     int
	Loc                                     *time.Location
	Hours                                   Settings
}

// loadBookingClinic finds the clinic behind a public slug. A disabled or unknown slug is the same miss.
func (s *Server) loadBookingClinic(ctx context.Context, slug string) (*bookingClinic, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if !slugShape.MatchString(slug) {
		return nil, pgx.ErrNoRows
	}
	var c bookingClinic
	var settings []byte
	var tz string
	err := s.db.QueryRow(ctx, `
		SELECT c.id, c.name, c.kind, c.address, c.phone_number, c.settings, coalesce(c.settings->>'timezone', ''),
		       a.booking_message, a.booking_requires_confirmation, a.booking_show_prices, a.booking_lead_hours, a.booking_horizon_days, a.slot_minutes
		FROM agenda_settings a JOIN clinics c ON c.id = a.clinic_id
		WHERE lower(a.booking_slug) = $1 AND a.booking_enabled
		  AND c.billing_status IN ('active', 'trialing', 'past_due')`, slug).
		Scan(&c.ID, &c.Name, &c.Kind, &c.Address, &c.Phone, &settings, &tz,
			&c.Message, &c.RequiresConfirmation, &c.ShowPrices, &c.LeadHours, &c.HorizonDays, &c.SlotMinutes)
	if err != nil {
		return nil, err
	}
	c.Loc = locationOrDefault(tz)
	_ = json.Unmarshal(settings, &c.Hours)
	c.Hours = c.Hours.normalized()
	return &c, nil
}

// bookable is a professional offered on the public page.
type bookable struct {
	ID, Name string
	Slot     int
	Hours    map[string][][]string
}

func (s *Server) listBookable(ctx context.Context, c *bookingClinic, onlyID string) ([]bookable, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.name, ps.slot_minutes, ps.hours
		FROM professional_settings ps JOIN users u ON u.id = ps.user_id
		WHERE ps.clinic_id = $1 AND u.clinic_id = $1 AND ps.bookable AND NOT u.disabled
		  AND ($2 = '' OR u.id = NULLIF($2, '')::uuid)
		ORDER BY u.name`, c.ID, onlyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []bookable
	for rows.Next() {
		var p bookable
		var raw []byte
		if err := rows.Scan(&p.ID, &p.Name, &p.Slot, &raw); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &p.Hours)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (b *bookingAPI) bookingInfo(w http.ResponseWriter, r *http.Request) {
	if !limit(w, b.reads, "read|"+clientIP(r)) {
		return
	}
	ctx := r.Context()
	c, err := b.loadBookingClinic(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		b.badTok.fail("tok|" + clientIP(r))
		writeError(w, http.StatusNotFound, "Esta página de citas no está disponible.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	pros, err := b.listBookable(ctx, c, "")
	if err != nil {
		serverError(w, r, err)
		return
	}
	profs := make([]map[string]any, 0, len(pros))
	for _, p := range pros {
		profs = append(profs, map[string]any{"id": p.ID, "name": p.Name})
	}
	rows, err := b.db.Query(ctx, `SELECT id, name, category, price_cents FROM catalog_items
		WHERE clinic_id = $1 AND kind = 'service' AND active ORDER BY category, name LIMIT 200`, c.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	services := []map[string]any{}
	for rows.Next() {
		var id, name, cat string
		var price int
		if err := rows.Scan(&id, &name, &cat, &price); err != nil {
			serverError(w, r, err)
			return
		}
		svc := map[string]any{"id": id, "name": name, "category": cat}
		if c.ShowPrices {
			svc["price_cents"] = price
		}
		services = append(services, svc)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	now := time.Now().In(c.Loc)
	writeJSON(w, http.StatusOK, map[string]any{"booking": map[string]any{
		"clinic":                map[string]string{"name": c.Name, "kind": c.Kind, "address": c.Address, "phone": c.Phone},
		"message":               c.Message,
		"requires_confirmation": c.RequiresConfirmation,
		"services":              services,
		"professionals":         profs,
		"today":                 now.Format("2006-01-02"),
		"lead_hours":            c.LeadHours,
		"horizon_days":          c.HorizonDays,
	}})
}

var weekdayKeys = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// candidateSlots lists the start times ("HH:MM") a professional works on date, one per slot, that
// respect the lead time and the horizon. Existing appointments and blocks are not considered.
func (c *bookingClinic) candidateSlots(p bookable, date string, now time.Time) []string {
	day, err := time.ParseInLocation("2006-01-02", date, c.Loc)
	if err != nil {
		return nil
	}
	last := now.In(c.Loc).AddDate(0, 0, c.HorizonDays).Format("2006-01-02")
	if date > last || date < now.In(c.Loc).Format("2006-01-02") {
		return nil
	}
	key := weekdayKeys[day.Weekday()]
	var windows [][2]string
	if len(p.Hours) > 0 {
		for _, w := range p.Hours[key] {
			if len(w) == 2 {
				windows = append(windows, [2]string{w[0], w[1]})
			}
		}
	} else if h := c.Hours.Hours[key]; h.Open {
		windows = append(windows, [2]string{h.Start, h.End})
	}
	step := p.Slot
	if step < 5 {
		step = c.SlotMinutes
	}
	earliest := now.Add(time.Duration(c.LeadHours) * time.Hour)
	var out []string
	for _, w := range windows {
		from, err1 := localTime(c.Loc, date, w[0])
		to, err2 := localTime(c.Loc, date, w[1])
		if err1 != nil || err2 != nil {
			continue
		}
		for t := from; !t.Add(time.Duration(step) * time.Minute).After(to); t = t.Add(time.Duration(step) * time.Minute) {
			if !t.Before(earliest) {
				out = append(out, t.Format("15:04"))
			}
		}
	}
	sort.Strings(out)
	return out
}

func (p bookable) end(c *bookingClinic, start string) string {
	step := p.Slot
	if step < 5 {
		step = c.SlotMinutes
	}
	t, _ := time.Parse("15:04", start)
	return t.Add(time.Duration(step) * time.Minute).Format("15:04")
}

func (b *bookingAPI) bookingAvailability(w http.ResponseWriter, r *http.Request) {
	if !limit(w, b.reads, "read|"+clientIP(r)) {
		return
	}
	ctx := r.Context()
	c, err := b.loadBookingClinic(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		b.badTok.fail("tok|" + clientIP(r))
		writeError(w, http.StatusNotFound, "Esta página de citas no está disponible.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	profID, date := r.URL.Query().Get("professional"), r.URL.Query().Get("date")
	if !validUUID(profID) {
		writeError(w, http.StatusBadRequest, "Elige un profesional.")
		return
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		writeError(w, http.StatusBadRequest, "La fecha no es válida.")
		return
	}
	if svc := r.URL.Query().Get("service"); svc != "" && !b.serviceOK(ctx, c.ID, svc) {
		writeError(w, http.StatusBadRequest, "El servicio no es válido.")
		return
	}
	pros, err := b.listBookable(ctx, c, profID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	slots := []map[string]string{}
	if len(pros) == 1 {
		pid := pros[0].ID
		for _, st := range c.candidateSlots(pros[0], date, time.Now()) {
			end := pros[0].end(c, st)
			code, err := b.slotConflict(ctx, b.db, c.ID, &pid, "", date, st, end, "")
			if err != nil {
				serverError(w, r, err)
				return
			}
			if code == SlotFree {
				slots = append(slots, map[string]string{"start": st, "end": end})
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": date, "slots": slots})
}

func (s *Server) serviceOK(ctx context.Context, clinicID, id string) bool {
	if !validUUID(id) {
		return false
	}
	var ok bool
	_ = s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM catalog_items WHERE id = $1 AND clinic_id = $2 AND kind = 'service' AND active)`, id, clinicID).Scan(&ok)
	return ok
}

type bookRequest struct {
	ProfessionalID  string `json:"professional_id"`
	ServiceID       string `json:"service_id"`
	Date            string `json:"date"`
	Start           string `json:"start"`
	Names           string `json:"names"`
	LastNames       string `json:"last_names"`
	Phone           string `json:"phone"`
	Email           string `json:"email"`
	Reason          string `json:"reason"`
	AcceptPrivacy   bool   `json:"accept_privacy"`
	AcceptReminders bool   `json:"accept_reminders"`
	Website         string `json:"website"` // honeypot: real people never fill it
}

func (b *bookingAPI) bookingCreate(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !b.bookIP.allow("book|" + ip) {
		tooMany(w)
		return
	}
	var req bookRequest
	if !decode(w, r, &req) {
		return
	}
	ctx := r.Context()
	c, err := b.loadBookingClinic(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		b.badTok.fail("tok|" + ip)
		writeError(w, http.StatusNotFound, "Esta página de citas no está disponible.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if req.Website != "" { // a bot: pretend it worked
		b.bookIP.fail("book|" + ip)
		writeJSON(w, http.StatusCreated, map[string]any{"appointment": map[string]any{"status": "scheduled"}})
		return
	}
	for _, f := range []*string{&req.Names, &req.LastNames, &req.Phone, &req.Email, &req.Reason, &req.Date, &req.Start} {
		*f = strings.TrimSpace(*f)
	}
	req.Email = strings.ToLower(req.Email)
	switch {
	case !validUUID(req.ProfessionalID):
		writeError(w, http.StatusBadRequest, "Elige un profesional.")
		return
	case req.Names == "" || req.LastNames == "" || utf8.RuneCountInString(req.Names) > 100 || utf8.RuneCountInString(req.LastNames) > 100:
		writeError(w, http.StatusBadRequest, "Escribe tu nombre y apellidos.")
		return
	case req.Phone == "" && req.Email == "":
		writeError(w, http.StatusBadRequest, "Escribe un teléfono o un correo para poder contactarte.")
		return
	case utf8.RuneCountInString(req.Reason) > 300:
		writeError(w, http.StatusBadRequest, "El motivo es demasiado largo (máximo 300 caracteres).")
		return
	case !req.AcceptPrivacy:
		writeError(w, http.StatusBadRequest, "Debes aceptar el aviso de privacidad para agendar.")
		return
	}
	phone := ""
	if req.Phone != "" {
		if phone = normalizePhoneMX(req.Phone); phone == "" {
			writeError(w, http.StatusBadRequest, "El teléfono debe tener 10 dígitos.")
			return
		}
	}
	if req.Email != "" && !validEmail(req.Email) {
		writeError(w, http.StatusBadRequest, "El correo no es válido.")
		return
	}
	if req.ServiceID != "" && !b.serviceOK(ctx, c.ID, req.ServiceID) {
		writeError(w, http.StatusBadRequest, "El servicio no es válido.")
		return
	}
	for _, k := range []string{req.Email, phone} {
		if k != "" && !b.bookKey.allow("bookk|"+c.ID+"|"+k) {
			tooMany(w)
			return
		}
	}
	if _, err := time.Parse("2006-01-02", req.Date); err != nil {
		writeError(w, http.StatusBadRequest, "La fecha no es válida.")
		return
	}
	pros, err := b.listBookable(ctx, c, req.ProfessionalID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if len(pros) != 1 {
		writeError(w, http.StatusBadRequest, "Elige un profesional.")
		return
	}
	pro := pros[0]
	valid := false
	for _, st := range c.candidateSlots(pro, req.Date, time.Now()) {
		valid = valid || st == req.Start
	}
	if !valid {
		writeError(w, http.StatusConflict, "Ese horario ya no está disponible. Elige otro.")
		return
	}
	end := pro.end(c, req.Start)
	token, err := newToken()
	if err != nil {
		serverError(w, r, err)
		return
	}

	var apptID string
	taken, tooManyOpen := false, false
	err = inTx(ctx, b.db, func(tx pgx.Tx) error {
		// One booking at a time per professional and day: the check and the insert are atomic.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "booking|"+c.ID+"|"+pro.ID+"|"+req.Date); err != nil {
			return err
		}
		code, err := b.slotConflict(ctx, tx, c.ID, &pro.ID, "", req.Date, req.Start, end, "")
		if err != nil {
			return err
		}
		if code != SlotFree {
			taken = true
			return nil
		}
		var open int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM appointments
			WHERE clinic_id = $1 AND source = 'online' AND status IN ('scheduled', 'confirmed') AND date >= current_date - 1
			  AND (($2 <> '' AND lower(email) = $2) OR ($3 <> '' AND phone = $3))`, c.ID, req.Email, phone).Scan(&open); err != nil {
			return err
		}
		if open >= 3 {
			tooManyOpen = true
			return nil
		}
		var svc any
		if req.ServiceID != "" {
			svc = req.ServiceID
		}
		newID := newRowID()
		sealedReason, err := encField("appointments", "details", newID, req.Reason)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, details, professional_id, status, source,
			                          service_id, phone, email, confirm_token, reminders_consent, privacy_accepted_at, id)
			VALUES ($1, '', $2, $3, $4, $5, $6, $7, $8, 'scheduled', 'online', $9, $10, $11, $12, $13, now(), $14::uuid)
			RETURNING id`,
			c.ID, req.Names, req.LastNames, req.Date, req.Start, end, sealedReason, pro.ID, svc, phone, req.Email, token, req.AcceptReminders, newID).Scan(&apptID); err != nil {
			return err
		}
		audit(ctx, tx, c.ID, nil, "appointment_booked_online", "Cita reservada en línea para el "+req.Date+" a las "+req.Start,
			map[string]any{"appointment_id": apptID, "professional_id": pro.ID, "requires_confirmation": c.RequiresConfirmation})
		return b.scheduleReminders(ctx, tx, c.ID, apptID)
	})
	switch {
	case err != nil:
		serverError(w, r, err)
		return
	case taken:
		writeError(w, http.StatusConflict, "Ese horario ya no está disponible. Elige otro.")
		return
	case tooManyOpen:
		writeError(w, http.StatusConflict, "Ya tienes varias citas próximas con estos datos. Para agendar otra, comunícate con el consultorio.")
		return
	}
	b.bookIP.fail("book|" + ip)
	for _, k := range []string{req.Email, phone} {
		if k != "" {
			b.bookKey.fail("bookk|" + c.ID + "|" + k)
		}
	}

	info, ok, err := b.loadApptInfo(ctx, b.db, c.ID, apptID)
	if err == nil && ok && req.Email != "" {
		subject, text, html := b.bookingMail(info, c.RequiresConfirmation)
		b.sendMail(mail.Message{To: []string{req.Email}, Subject: subject, Text: text, HTML: html})
	}
	writeJSON(w, http.StatusCreated, map[string]any{"appointment": map[string]any{
		"token":                token,
		"date":                 req.Date,
		"start":                req.Start,
		"end":                  end,
		"professional":         pro.Name,
		"clinic":               c.Name,
		"status":               "scheduled",
		"pending_confirmation": c.RequiresConfirmation,
		"reminders":            req.AcceptReminders,
	}})
}
