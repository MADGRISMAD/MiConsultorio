package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// the kinds of pet offered on the public page
var bookingSpecies = []string{"Perro", "Gato", "Ave", "Conejo", "Roedor", "Reptil", "Otro"}

var slugShape = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

// bookingClinic is a clinic with online booking switched on, as the public pages see it.
type bookingClinic struct {
	ID, Name, Kind, Address, Phone, Message string
	RequiresConfirmation, ShowPrices        bool
	LeadHours, HorizonDays, SlotMinutes     int
	Animals, People                         bool // the clinic sees pets and/or people
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
		  AND c.billing_status IN ('active', 'trialing', 'past_due') AND c.branch_suspended_at IS NULL`, slug).
		Scan(&c.ID, &c.Name, &c.Kind, &c.Address, &c.Phone, &settings, &tz,
			&c.Message, &c.RequiresConfirmation, &c.ShowPrices, &c.LeadHours, &c.HorizonDays, &c.SlotMinutes)
	if err != nil {
		return nil, err
	}
	c.Loc = locationOrDefault(tz)
	kinds, err := s.clinicKindsFor(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	for _, k := range kinds {
		if k == "VETERINARY" {
			c.Animals = true
		} else {
			c.People = true
		}
	}
	if !c.Animals {
		c.People = true
	}
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
		WHERE ps.clinic_id = $1 AND u.clinic_id = $1 AND ps.bookable AND ps.consults AND NOT u.disabled
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
	rows, err := b.db.Query(ctx, `SELECT id, name, category, price_cents, duration_minutes FROM catalog_items
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
		var minutes *int
		if err := rows.Scan(&id, &name, &cat, &price, &minutes); err != nil {
			serverError(w, r, err)
			return
		}
		svc := map[string]any{"id": id, "name": name, "category": cat}
		if minutes != nil {
			svc["duration_minutes"] = *minutes
		}
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
		"animals":               c.Animals,
		"people":                c.People,
		"species":               bookingSpecies,
		"today":                 now.Format("2006-01-02"),
		"lead_hours":            c.LeadHours,
		"horizon_days":          c.HorizonDays,
	}})
}

var weekdayKeys = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// bookingWindows are the working ranges ("HH:MM") of a professional on a weekday key: their own hours for that
// day or, on a day where they have none, the clinic's. (The settings screen says exactly that: a day left empty
// uses the clinic's hours, so setting Monday does not close the rest of the week.)
func (c *bookingClinic) bookingWindows(p bookable, key string) [][2]string {
	var windows [][2]string
	for _, w := range p.Hours[key] {
		if len(w) == 2 {
			windows = append(windows, [2]string{w[0], w[1]})
		}
	}
	if len(windows) == 0 {
		if h := c.Hours.Hours[key]; h.Open {
			windows = append(windows, [2]string{h.Start, h.End})
		}
	}
	return windows
}

// step is the grid, in minutes, on which the professional's appointments start.
func (p bookable) step(c *bookingClinic) int {
	if p.Slot < 5 {
		return c.SlotMinutes
	}
	return p.Slot
}

// minutes is how long an appointment takes: the service's duration when it has one (dur > 0),
// otherwise the professional's slot.
func (p bookable) minutes(c *bookingClinic, dur int) int {
	if dur > 0 {
		return dur
	}
	return p.step(c)
}

// candidateSlots lists the start times ("HH:MM") a professional works on date, one per grid step, that
// respect the lead time and the horizon and leave room for an appointment of dur minutes (0 = the
// professional's slot). Existing appointments and blocks are not considered.
func (c *bookingClinic) candidateSlots(p bookable, date string, now time.Time, dur int) []string {
	day, err := time.ParseInLocation("2006-01-02", date, c.Loc)
	if err != nil {
		return nil
	}
	last := now.In(c.Loc).AddDate(0, 0, c.HorizonDays).Format("2006-01-02")
	if date > last || date < now.In(c.Loc).Format("2006-01-02") {
		return nil
	}
	step := time.Duration(p.step(c)) * time.Minute
	length := time.Duration(p.minutes(c, dur)) * time.Minute
	earliest := now.Add(time.Duration(c.LeadHours) * time.Hour)
	var out []string
	for _, w := range c.bookingWindows(p, weekdayKeys[day.Weekday()]) {
		from, err1 := localTime(c.Loc, date, w[0])
		to, err2 := localTime(c.Loc, date, w[1])
		if err1 != nil || err2 != nil {
			continue
		}
		for t := from; !t.Add(length).After(to); t = t.Add(step) {
			if !t.Before(earliest) {
				out = append(out, t.Format("15:04"))
			}
		}
	}
	sort.Strings(out)
	return out
}

// end is when an appointment of dur minutes (0 = the professional's slot) starting at start finishes.
func (p bookable) end(c *bookingClinic, start string, dur int) string {
	t, _ := time.Parse("15:04", start)
	return t.Add(time.Duration(p.minutes(c, dur)) * time.Minute).Format("15:04")
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
	svc := r.URL.Query().Get("service")
	if svc != "" && !b.serviceOK(ctx, c.ID, svc) {
		writeError(w, http.StatusBadRequest, "El servicio no es válido.")
		return
	}
	dur := b.slotServiceMinutes(ctx, b.db, c.ID, svc)
	pros, err := b.listBookable(ctx, c, profID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	slots := []map[string]string{}
	holder := r.URL.Query().Get("holder")
	if len(pros) == 1 {
		pid := pros[0].ID
		held, err := b.slotHeldSpans(ctx, b.db, c.ID, pid, date, holder)
		if err != nil {
			serverError(w, r, err)
			return
		}
		for _, st := range c.candidateSlots(pros[0], date, time.Now(), dur) {
			end := pros[0].end(c, st, dur)
			code, err := b.slotConflict(ctx, b.db, c.ID, &pid, "", date, st, end, "")
			if err != nil {
				serverError(w, r, err)
				return
			}
			if code == SlotFree && !held.overlaps(st, end) {
				slots = append(slots, map[string]string{"start": st, "end": end})
			}
		}
	}
	// "unavailable" lets the page grey out a specialist for that day (vacation, full day) without saying why
	writeJSON(w, http.StatusOK, map[string]any{"date": date, "slots": slots, "unavailable": len(slots) == 0})
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
	Website         string `json:"website"`    // honeypot: real people never fill it
	Holder          string `json:"holder"`     // the visitor's key for the time held while the form is filled in
	Registered      bool   `json:"registered"` // already a patient: the record is found by phone
	PatientID       string `json:"patient_id"` // registered pets: the one chosen from the names the lookup returned
	Animal          bool   `json:"animal"`     // the visit is for a pet
	Species         string `json:"species"`
	PetName         string `json:"pet_name"`
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
	case req.Animal && !c.Animals, !req.Animal && !c.People:
		writeError(w, http.StatusBadRequest, "Este consultorio no atiende esa opción.")
		return
	case !req.Registered && (req.Names == "" || utf8.RuneCountInString(req.Names) > 100 || utf8.RuneCountInString(req.LastNames) > 100):
		writeError(w, http.StatusBadRequest, "Escribe tu nombre.")
		return
	case req.Registered && req.Phone == "":
		writeError(w, http.StatusBadRequest, "Escribe el teléfono con el que te registraste.")
		return
	case !req.Registered && req.Animal && !slices.Contains(bookingSpecies, req.Species):
		writeError(w, http.StatusBadRequest, "Elige qué mascota vas a consultar.")
		return
	case utf8.RuneCountInString(req.PetName) > 80:
		writeError(w, http.StatusBadRequest, "El nombre de la mascota es demasiado largo.")
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
	var linked *regPatient
	if req.Registered {
		var status int
		var msg string
		if linked, status, msg = b.findRegistered(ctx, c.ID, phone, req); linked == nil {
			writeError(w, status, msg)
			return
		}
		req.Names, req.LastNames = linked.Names, linked.LastNames
		if want := b.followUpProfessional(ctx, c, linked.ID); want != "" && want != req.ProfessionalID {
			name := "quien te ha atendido"
			if ps, err := b.listBookable(ctx, c, want); err == nil && len(ps) == 1 {
				name = ps[0].Name
			}
			writeError(w, http.StatusConflict, "Para tu seguimiento, agenda con "+name+".")
			return
		}
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
	dur := b.slotServiceMinutes(ctx, b.db, c.ID, req.ServiceID)
	valid := false
	for _, st := range c.candidateSlots(pro, req.Date, time.Now(), dur) {
		valid = valid || st == req.Start
	}
	if !valid {
		writeError(w, http.StatusConflict, "Ese horario ya no está disponible. Elige otro.")
		return
	}
	end := pro.end(c, req.Start, dur)
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
		if code == SlotFree {
			held, err := b.slotHeldSpans(ctx, tx, c.ID, pro.ID, req.Date, req.Holder)
			if err != nil {
				return err
			}
			if held.overlaps(req.Start, end) {
				code = SlotTaken // the slot is on offer to someone on the waitlist
			}
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
		reason := req.Reason
		if req.Animal && !req.Registered {
			pet := "Mascota: " + req.Species
			if req.PetName != "" {
				pet += " (" + req.PetName + ")"
			}
			reason = strings.TrimSpace(pet + ". " + reason)
		}
		var patientID any
		if linked != nil {
			patientID = linked.ID
		}
		sealedReason, err := encField("appointments", "details", newID, reason)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO appointments (clinic_id, curp, names, last_names, date, start_hour, end_hour, details, professional_id, status, source,
			                          service_id, phone, email, confirm_token, reminders_consent, privacy_accepted_at, id, patient_id)
			VALUES ($1, '', $2, $3, $4, $5, $6, $7, $8, 'scheduled', 'online', $9, $10, $11, $12, $13, now(), $14::uuid, $15::uuid)
			RETURNING id`,
			c.ID, req.Names, req.LastNames, req.Date, req.Start, end, sealedReason, pro.ID, svc, phone, req.Email, token, req.AcceptReminders, newID, patientID).Scan(&apptID); err != nil {
			return err
		}
		if req.Holder != "" { // the time is theirs now
			if _, err := tx.Exec(ctx, `DELETE FROM booking_holds WHERE clinic_id = $1 AND holder = $2`, c.ID, req.Holder); err != nil {
				return err
			}
		}
		audit(ctx, tx, c.ID, nil, "appointment_booked_online", "Cita reservada en línea para el "+req.Date+" a las "+req.Start,
			map[string]any{"appointment_id": apptID, "professional_id": pro.ID, "requires_confirmation": c.RequiresConfirmation})
		b.ntfAppointment(ctx, tx, c.ID, pro.ID, "booking_new", "Nueva cita por reserva en línea",
			bookingWho(req)+" · "+req.Date+" "+req.Start+" con "+pro.Name, "/admin/navegar-citas")
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
	b.mailSpecialist(ctx, c, pro, req, end)
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

// bookingWho names the person (and pet) of a booking for the clinic's notification.
func bookingWho(req bookRequest) string {
	who := strings.TrimSpace(req.Names + " " + req.LastNames)
	switch {
	case req.Registered && req.Animal:
		return who + " (paciente registrado, mascota)"
	case req.Registered:
		return who + " (paciente registrado)"
	case req.Animal:
		return who + " (mascota: " + req.Species + ")"
	}
	return who
}

// regPatient is the record a registered patient's booking attaches to. Nothing of it is sent back to the visitor.
type regPatient struct{ ID, Names, LastNames, Subject string }

// registeredMatches are the active records of a clinic whose own phone, or whose owner's / guardian's phone, is this one.
func (s *Server) registeredMatches(ctx context.Context, clinicID, phone string) ([]regPatient, error) {
	digits := phone
	if len(digits) > 10 {
		digits = digits[len(digits)-10:]
	}
	rows, err := s.db.Query(ctx, `
		SELECT id::text, names, last_names, subject FROM patients
		WHERE clinic_id = $1 AND archived_at IS NULL
		  AND (right(regexp_replace(phone, '\D', '', 'g'), 10) = $2 OR right(regexp_replace(guardian_phone, '\D', '', 'g'), 10) = $2)
		ORDER BY file_number LIMIT 30`, clinicID, digits)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []regPatient
	for rows.Next() {
		var x regPatient
		if err := rows.Scan(&x.ID, &x.Names, &x.LastNames, &x.Subject); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// findRegistered picks the record a registered visitor means from their phone: for a pet the one chosen from the
// lookup; for a person the only match, or, with several people on one phone, the one whose typed name matches.
func (b *bookingAPI) findRegistered(ctx context.Context, clinicID, phone string, req bookRequest) (*regPatient, int, string) {
	all, err := b.registeredMatches(ctx, clinicID, phone)
	if err != nil {
		return nil, http.StatusInternalServerError, "No se pudo buscar tu expediente. Intenta de nuevo."
	}
	var pick []regPatient
	for _, x := range all {
		switch {
		case req.Animal && x.Subject == "animal" && (req.PatientID == "" || x.ID == req.PatientID):
			pick = append(pick, x)
		case !req.Animal && x.Subject == "person":
			pick = append(pick, x)
		}
	}
	if req.Animal {
		if len(pick) == 0 {
			return nil, http.StatusConflict, "No encontramos mascotas registradas con ese teléfono. Agenda como cliente nuevo."
		}
		if len(pick) > 1 {
			return nil, http.StatusBadRequest, "Elige tu mascota."
		}
		return &pick[0], 0, ""
	}
	if len(pick) > 1 && strings.TrimSpace(req.Names) != "" {
		want := foldText(strings.TrimSpace(req.Names + " " + req.LastNames))
		var named []regPatient
		for _, x := range pick {
			if strings.HasPrefix(foldText(strings.TrimSpace(x.Names+" "+x.LastNames)), want) {
				named = append(named, x)
			}
		}
		pick = named
	}
	switch len(pick) {
	case 0:
		return nil, http.StatusConflict, "No encontramos un expediente con ese teléfono. Agenda como paciente nuevo."
	case 1:
		return &pick[0], 0, ""
	}
	return nil, http.StatusConflict, "Hay varias personas con ese teléfono: escribe tu nombre completo para identificarte."
}

// bookingLookup is POST /public/booking/{slug}/lookup: does this phone belong to a registered patient, and which pets
// (names only) are registered under it. Nothing else of the record is ever returned.
func (b *bookingAPI) bookingLookup(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !limit(w, b.lookups, "look|"+ip) {
		return
	}
	var req struct {
		Phone string `json:"phone"`
	}
	if !decode(w, r, &req) {
		return
	}
	c, err := b.loadBookingClinic(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Esta página de citas no está disponible.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	phone := normalizePhoneMX(req.Phone)
	if phone == "" {
		writeError(w, http.StatusBadRequest, "El teléfono debe tener 10 dígitos.")
		return
	}
	if !b.lookups.allow("lookp|" + c.ID + "|" + phone) {
		tooMany(w)
		return
	}
	b.lookups.fail("lookp|" + c.ID + "|" + phone)
	all, err := b.registeredMatches(r.Context(), c.ID, phone)
	if err != nil {
		serverError(w, r, err)
		return
	}
	people, pets := 0, []map[string]string{}
	var onlyPerson string
	for _, x := range all {
		if x.Subject == "animal" {
			if c.Animals {
				pets = append(pets, map[string]string{"id": x.ID, "name": x.Names, "professional_id": b.followUpProfessional(r.Context(), c, x.ID)})
			}
		} else if c.People {
			people++
			onlyPerson = x.ID
		}
	}
	// a patient already in treatment books with whoever has been seeing them, for continuity
	pro := ""
	if people == 1 {
		pro = b.followUpProfessional(r.Context(), c, onlyPerson)
	}
	writeJSON(w, http.StatusOK, map[string]any{"person": people > 0, "several": people > 1, "professional_id": pro, "pets": pets})
}

// bookingHold is POST /public/booking/{slug}/hold: the time a visitor picked is kept for them for a few minutes.
func (b *bookingAPI) bookingHold(w http.ResponseWriter, r *http.Request) {
	if !limit(w, b.holds, "hold|"+clientIP(r)) {
		return
	}
	var req struct {
		ProfessionalID string `json:"professional_id"`
		ServiceID      string `json:"service_id"`
		Date           string `json:"date"`
		Start          string `json:"start"`
		Holder         string `json:"holder"`
	}
	if !decode(w, r, &req) {
		return
	}
	ctx := r.Context()
	c, err := b.loadBookingClinic(ctx, chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Esta página de citas no está disponible.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if !holderShape.MatchString(req.Holder) || !validUUID(req.ProfessionalID) {
		writeError(w, http.StatusBadRequest, "Solicitud inválida.")
		return
	}
	if req.ServiceID != "" && !b.serviceOK(ctx, c.ID, req.ServiceID) {
		writeError(w, http.StatusBadRequest, "El servicio no es válido.")
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
	dur := b.slotServiceMinutes(ctx, b.db, c.ID, req.ServiceID)
	if !slices.Contains(c.candidateSlots(pro, req.Date, time.Now(), dur), req.Start) {
		writeError(w, http.StatusConflict, "Ese horario ya no está disponible. Elige otro.")
		return
	}
	end := pro.end(c, req.Start, dur)
	taken := false
	var expires time.Time
	err = inTx(ctx, b.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "booking|"+c.ID+"|"+pro.ID+"|"+req.Date); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM booking_holds WHERE expires_at < now() OR (clinic_id = $1 AND holder = $2)`, c.ID, req.Holder); err != nil {
			return err
		}
		code, err := b.slotConflict(ctx, tx, c.ID, &pro.ID, "", req.Date, req.Start, end, "")
		if err != nil {
			return err
		}
		if code == SlotFree {
			held, err := b.slotHeldSpans(ctx, tx, c.ID, pro.ID, req.Date, req.Holder)
			if err != nil {
				return err
			}
			if held.overlaps(req.Start, end) {
				code = SlotTaken
			}
		}
		if code != SlotFree {
			taken = true
			return nil
		}
		return tx.QueryRow(ctx, `
			INSERT INTO booking_holds (clinic_id, professional_id, date, start_hour, end_hour, holder, expires_at)
			VALUES ($1, $2, $3::date, $4::time, $5::time, $6, now() + $7::float8 * interval '1 second') RETURNING expires_at`,
			c.ID, pro.ID, req.Date, req.Start, end, req.Holder, bookingHoldTTL.Seconds()).Scan(&expires)
	})
	switch {
	case err != nil:
		serverError(w, r, err)
	case taken:
		writeError(w, http.StatusConflict, "Alguien más acaba de elegir ese horario. Elige otro.")
	default:
		writeJSON(w, http.StatusOK, map[string]any{"expires_at": expires})
	}
}

const bookingHoldTTL = 10 * time.Minute

var holderShape = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

// mailSpecialist tells the professional by e-mail, besides the bell, that a patient booked with them. The reason of the
// visit stays out of the message: it is read in the agenda.
func (b *bookingAPI) mailSpecialist(ctx context.Context, c *bookingClinic, pro bookable, req bookRequest, end string) {
	if !b.mailEnabled() {
		return
	}
	var to string
	if b.db.QueryRow(ctx, `SELECT coalesce(email, '') FROM users WHERE id = $1 AND clinic_id = $2 AND NOT disabled`, pro.ID, c.ID).Scan(&to) != nil || to == "" {
		return
	}
	day, err := localTime(c.Loc, req.Date, req.Start)
	if err != nil {
		return
	}
	who := bookingWho(req)
	link := b.appLink("/admin/navegar-citas")
	title := "Nueva cita agendada en línea"
	lead := who + " agendó una cita contigo desde el enlace de " + c.Name + "."
	when := longDateES(day) + " a las " + clockES(day) + " (hasta las " + end + ")"
	if c.RequiresConfirmation {
		lead += " Está pendiente de que el consultorio la confirme."
	}
	text := "Hola " + firstName(pro.Name) + ",\n\n" + lead + "\n\nCuándo: " + when + "\n\nVer la agenda: " + link + "\n"
	body := `<p style="margin:0 0 8px">Hola ` + esc(firstName(pro.Name)) + `,</p><p style="margin:0 0 8px">` + esc(lead) + `</p>` +
		`<p style="margin:0 0 8px"><strong>Cuándo:</strong> ` + esc(when) + `</p>` + button(link, "Ver la agenda")
	b.sendMail(mail.Message{To: []string{to}, Subject: title + " · " + req.Date + " " + req.Start, Text: text, HTML: layout(title, body)})
}

// followUpProfessional is the professional who last attended the patient (a note they wrote or a visit they held),
// as long as they are still bookable; empty when the patient has no history or that person no longer takes bookings.
func (s *Server) followUpProfessional(ctx context.Context, c *bookingClinic, patientID string) string {
	var id string
	err := s.db.QueryRow(ctx, `
		SELECT pid FROM (
			SELECT author_id::text AS pid, occurred_at AS at FROM encounters
			WHERE clinic_id = $1 AND patient_id = $2 AND author_id IS NOT NULL AND addendum_of IS NULL
			UNION ALL
			SELECT professional_id::text, date::timestamptz FROM appointments
			WHERE clinic_id = $1 AND patient_id = $2 AND professional_id IS NOT NULL AND status IN ('completed', 'arrived', 'in_progress')
		) h ORDER BY at DESC LIMIT 1`, c.ID, patientID).Scan(&id)
	if err != nil || id == "" {
		return ""
	}
	if ps, err := s.listBookable(ctx, c, id); err != nil || len(ps) != 1 {
		return ""
	}
	return id
}
