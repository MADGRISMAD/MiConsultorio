package api

import (
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
)

var (
	slugRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)
	colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	dayKeys = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
)

// ---------------------------------------------------------------------------
// Professionals
// ---------------------------------------------------------------------------

type professional struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Role        string                 `json:"role"`
	Specialty   string                 `json:"specialty"`
	Bookable    bool                   `json:"bookable"`
	Consults    bool                   `json:"consults"` // sees patients: false for an owner who only runs the clinic
	SlotMinutes int                    `json:"slot_minutes"`
	Hours       map[string][][2]string `json:"hours"`
	Color       string                 `json:"color"`
}

func (s *Server) listProfessionals(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	list, err := s.loadProfessionals(r, p.ClinicID, "")
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"professionals": list})
}

func (s *Server) loadProfessionals(r *http.Request, clinicID, onlyID string) ([]professional, error) {
	rows, err := s.db.Query(r.Context(), `
		SELECT u.id::text, u.name, u.role, u.specialty_title, coalesce(ps.bookable, false), coalesce(ps.consults, true),
		       coalesce(ps.slot_minutes, ags.slot_minutes, 30), coalesce(ps.hours, '{}'::jsonb), coalesce(ps.color, '')
		FROM users u
		LEFT JOIN professional_settings ps ON ps.user_id = u.id
		LEFT JOIN agenda_settings ags ON ags.clinic_id = u.clinic_id
		WHERE u.clinic_id = $1 AND NOT u.disabled AND u.role IN ('admin', 'doctor') AND ($2 = '' OR u.id::text = $2)
		ORDER BY u.name, u.id`, clinicID, onlyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []professional{}
	for rows.Next() {
		var pr professional
		var raw []byte
		if err := rows.Scan(&pr.ID, &pr.Name, &pr.Role, &pr.Specialty, &pr.Bookable, &pr.Consults, &pr.SlotMinutes, &raw, &pr.Color); err != nil {
			return nil, err
		}
		pr.Hours = map[string][][2]string{}
		_ = json.Unmarshal(raw, &pr.Hours)
		out = append(out, pr)
	}
	return out, rows.Err()
}

type professionalIn struct {
	Bookable    bool                   `json:"bookable"`
	Consults    *bool                  `json:"consults"` // omitted: unchanged
	SlotMinutes int                    `json:"slot_minutes"`
	Hours       map[string][][2]string `json:"hours"`
	Color       string                 `json:"color"`
}

func (in *professionalIn) validate() string {
	if in.SlotMinutes < 5 || in.SlotMinutes > 480 {
		return "El intervalo de citas debe estar entre 5 y 480 minutos."
	}
	in.Color = strings.TrimSpace(in.Color)
	if in.Color != "" && !colorRe.MatchString(in.Color) {
		return "El color debe tener el formato #RRGGBB."
	}
	if in.Hours == nil {
		in.Hours = map[string][][2]string{}
	}
	for day, ranges := range in.Hours {
		if !slices.Contains(dayKeys, day) {
			return "Día de la semana no válido."
		}
		if len(ranges) > 4 {
			return "Máximo 4 tramos de horario por día."
		}
		sort.Slice(ranges, func(i, j int) bool { return ranges[i][0] < ranges[j][0] })
		prevEnd := ""
		for _, rg := range ranges {
			st, err1 := time.Parse("15:04", rg[0])
			en, err2 := time.Parse("15:04", rg[1])
			if err1 != nil || err2 != nil || !en.After(st) {
				return "Los horarios deben ser HH:MM y la hora de fin posterior a la de inicio."
			}
			if prevEnd != "" && rg[0] < prevEnd {
				return "Los tramos de un mismo día no pueden traslaparse."
			}
			prevEnd = rg[1]
		}
		if len(ranges) == 0 {
			delete(in.Hours, day)
		}
	}
	return ""
}

func (s *Server) updateProfessional(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p := principalFrom(r.Context())
	if !validUUID(id) || !s.isProfessional(r.Context(), s.db, p.ClinicID, id) {
		writeError(w, http.StatusNotFound, "Profesional no encontrado.")
		return
	}
	var in professionalIn
	if !decode(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	raw, _ := json.Marshal(in.Hours)
	if _, err := s.db.Exec(r.Context(), `
		INSERT INTO professional_settings (user_id, clinic_id, bookable, consults, slot_minutes, hours, color)
		VALUES ($1, $2, $3 AND coalesce($7, true), coalesce($7, true), $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE SET consults = coalesce($7, professional_settings.consults),
			bookable = $3 AND coalesce($7, professional_settings.consults), slot_minutes = $4, hours = $5, color = $6, updated_at = now()`,
		id, p.ClinicID, in.Bookable, in.SlotMinutes, raw, in.Color, in.Consults); err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "agenda_professional", "Cambió la configuración de agenda de un profesional", map[string]any{"professional": id, "bookable": in.Bookable})
	list, err := s.loadProfessionals(r, p.ClinicID, id)
	if err != nil || len(list) == 0 {
		serverError(w, r, errors.Join(err, pgx.ErrNoRows))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"professional": list[0]})
}

// listAgendaServices is the catalog of services an appointment can be for (works on every plan).
func (s *Server) listAgendaServices(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `SELECT id::text, name, duration_minutes FROM catalog_items WHERE clinic_id = $1 AND kind = 'service' AND active ORDER BY name LIMIT 500`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	type svc struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		DurationMinutes *int   `json:"duration_minutes"`
	}
	out := []svc{}
	for rows.Next() {
		var v svc
		if err := rows.Scan(&v.ID, &v.Name, &v.DurationMinutes); err != nil {
			serverError(w, r, err)
			return
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": out})
}

// ---------------------------------------------------------------------------
// Time blocks
// ---------------------------------------------------------------------------

type timeBlock struct {
	ID             string  `json:"id"`
	ProfessionalID *string `json:"professional_id"`
	DateFrom       string  `json:"date_from"`
	DateTo         string  `json:"date_to"`
	StartHour      *string `json:"startHour"`
	EndHour        *string `json:"endHour"`
	Reason         string  `json:"reason"`
	CreatedByName  string  `json:"created_by_name"`
}

const blockCols = `id::text, professional_id::text, to_char(date_from, 'YYYY-MM-DD'), to_char(date_to, 'YYYY-MM-DD'),
	to_char(start_hour, 'HH24:MI'), to_char(end_hour, 'HH24:MI'), reason, created_by_name`

func scanBlock(row pgx.Row) (timeBlock, error) {
	var b timeBlock
	err := row.Scan(&b.ID, &b.ProfessionalID, &b.DateFrom, &b.DateTo, &b.StartHour, &b.EndHour, &b.Reason, &b.CreatedByName)
	return b, err
}

func (s *Server) listBlocks(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	for _, d := range []string{from, to} {
		if d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				writeError(w, http.StatusBadRequest, "La fecha no es válida.")
				return
			}
		}
	}
	rows, err := s.db.Query(r.Context(), `SELECT `+blockCols+` FROM time_blocks
		WHERE clinic_id = $1 AND ($2 = '' OR date_to >= $2::date) AND ($3 = '' OR date_from <= $3::date)
		ORDER BY date_from, start_hour NULLS FIRST LIMIT 1000`, p.ClinicID, from, to)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []timeBlock{}
	for rows.Next() {
		b, err := scanBlock(rows)
		if err != nil {
			serverError(w, r, err)
			return
		}
		out = append(out, b)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"blocks": out})
}

type blockIn struct {
	ProfessionalID *string `json:"professional_id"`
	DateFrom       string  `json:"date_from"`
	DateTo         string  `json:"date_to"`
	StartHour      string  `json:"startHour"`
	EndHour        string  `json:"endHour"`
	Reason         string  `json:"reason"`
}

func (s *Server) createBlock(w http.ResponseWriter, r *http.Request) {
	var in blockIn
	if !decode(w, r, &in) {
		return
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if in.ProfessionalID != nil && *in.ProfessionalID == "" {
		in.ProfessionalID = nil
	}
	from, err1 := time.Parse("2006-01-02", in.DateFrom)
	to, err2 := time.Parse("2006-01-02", in.DateTo)
	if in.DateTo == "" && err1 == nil {
		to, err2, in.DateTo = from, nil, in.DateFrom
	}
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusBadRequest, "Las fechas del bloqueo no son válidas.")
		return
	}
	today := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -1)
	switch {
	case to.Before(from):
		writeError(w, http.StatusBadRequest, "La fecha final no puede ser anterior a la inicial.")
		return
	case to.Before(today):
		writeError(w, http.StatusBadRequest, "No tiene sentido bloquear fechas que ya pasaron.")
		return
	case to.Sub(from) > 366*24*time.Hour:
		writeError(w, http.StatusBadRequest, "Un bloqueo no puede durar más de un año.")
		return
	case from.After(time.Now().AddDate(5, 0, 0)):
		writeError(w, http.StatusBadRequest, "La fecha del bloqueo está demasiado lejos.")
		return
	case utf8.RuneCountInString(in.Reason) > 200:
		writeError(w, http.StatusBadRequest, "El motivo es demasiado largo.")
		return
	}
	if (in.StartHour == "") != (in.EndHour == "") {
		writeError(w, http.StatusBadRequest, "Indica la hora de inicio y la de fin, o deja ambas vacías para bloquear el día completo.")
		return
	}
	if in.StartHour != "" {
		st, e1 := time.Parse("15:04", in.StartHour)
		en, e2 := time.Parse("15:04", in.EndHour)
		if e1 != nil || e2 != nil || !en.After(st) {
			writeError(w, http.StatusBadRequest, "El horario del bloqueo no es válido.")
			return
		}
	}
	p := principalFrom(r.Context())
	if !hasAnyPermission(p.Permissions, PermAdminAppointments) {
		own := p.UserID
		if in.ProfessionalID == nil || *in.ProfessionalID != own {
			writeError(w, http.StatusForbidden, "Solo puedes bloquear tu propio horario.")
			return
		}
	}
	if in.ProfessionalID != nil && (!validUUID(*in.ProfessionalID) || !s.isConsulting(r.Context(), s.db, p.ClinicID, *in.ProfessionalID)) {
		writeError(w, http.StatusBadRequest, "El profesional no existe o no está activo en este consultorio.")
		return
	}
	var block timeBlock
	var affected []appointment
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var err error
		block, err = scanBlock(tx.QueryRow(r.Context(), `
			INSERT INTO time_blocks (clinic_id, professional_id, date_from, date_to, start_hour, end_hour, reason, created_by_name)
			VALUES ($1, $2::uuid, $3::date, $4::date, NULLIF($5, '')::time, NULLIF($6, '')::time, $7, $8) RETURNING `+blockCols,
			p.ClinicID, in.ProfessionalID, in.DateFrom, in.DateTo, in.StartHour, in.EndHour, in.Reason, p.actorName()))
		if err != nil {
			return err
		}
		// Existing appointments under the block are reported, never cancelled: someone must decide what to do with them.
		rows, err := tx.Query(r.Context(), appointmentSelect+`
			WHERE a.clinic_id = $1 AND a.date BETWEEN $2::date AND $3::date AND a.status NOT IN ('cancelled', 'no_show', 'completed')
			  AND ($4::uuid IS NULL OR a.professional_id = $4::uuid)
			  AND (NULLIF($5, '') IS NULL OR (a.start_hour < NULLIF($6, '')::time AND a.end_hour > NULLIF($5, '')::time))
			ORDER BY a.date, a.start_hour`, p.ClinicID, in.DateFrom, in.DateTo, in.ProfessionalID, in.StartHour, in.EndHour)
		if err != nil {
			return err
		}
		if affected, err = pgx.CollectRows(rows, pgx.RowToStructByName[appointment]); err != nil {
			return err
		}
		if err = openAppointmentDetails(affected); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "agenda_block_create", "Bloqueó un horario de la agenda",
			map[string]any{"block": block.ID, "from": in.DateFrom, "to": in.DateTo, "affected": len(affected)})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	if affected == nil {
		affected = []appointment{}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"block": block, "affected": affected})
}

func (s *Server) deleteBlock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Bloqueo no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	manager := hasAnyPermission(p.Permissions, PermAdminAppointments)
	tag, err := s.db.Exec(r.Context(), `DELETE FROM time_blocks WHERE clinic_id = $1 AND id = $2 AND ($3 OR (professional_id = $4::uuid AND created_by_name = $5))`, p.ClinicID, id, manager, p.UserID, p.actorName())
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Bloqueo no encontrado.")
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "agenda_block_delete", "Quitó un bloqueo de la agenda", map[string]any{"block": id})
	waitlistWake()
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Clinic agenda settings
// ---------------------------------------------------------------------------

type agendaSettings struct {
	SlotMinutes                 int      `json:"slot_minutes"`
	Rooms                       []string `json:"rooms"`
	BookingEnabled              bool     `json:"booking_enabled"`
	BookingSlug                 string   `json:"booking_slug"`
	BookingLeadHours            int      `json:"booking_lead_hours"`
	BookingHorizonDays          int      `json:"booking_horizon_days"`
	BookingMessage              string   `json:"booking_message"`
	BookingRequiresConfirmation bool     `json:"booking_requires_confirmation"`
	BookingShowPrices           bool     `json:"booking_show_prices"`
	CancelMinHours              int      `json:"cancel_min_hours"`
	RemindEmail                 bool     `json:"remind_email"`
	RemindWhatsapp              bool     `json:"remind_whatsapp"`
	RemindHours                 []int    `json:"remind_hours"`
	ReminderTemplate            string   `json:"reminder_template"`
}

func defaultAgendaSettings() agendaSettings {
	return agendaSettings{SlotMinutes: 30, Rooms: []string{}, BookingLeadHours: 2, BookingHorizonDays: 30, CancelMinHours: 2, RemindEmail: true, RemindHours: []int{24, 2}}
}

func loadAgendaSettings(r *http.Request, q queryRower, clinicID string) (agendaSettings, error) {
	st := defaultAgendaSettings()
	err := q.QueryRow(r.Context(), `
		SELECT slot_minutes, rooms, booking_enabled, coalesce(booking_slug, ''), booking_lead_hours, booking_horizon_days, booking_message,
		       booking_requires_confirmation, remind_email, remind_whatsapp, remind_hours, reminder_template, booking_show_prices, cancel_min_hours
		FROM agenda_settings WHERE clinic_id = $1`, clinicID).
		Scan(&st.SlotMinutes, &st.Rooms, &st.BookingEnabled, &st.BookingSlug, &st.BookingLeadHours, &st.BookingHorizonDays, &st.BookingMessage,
			&st.BookingRequiresConfirmation, &st.RemindEmail, &st.RemindWhatsapp, &st.RemindHours, &st.ReminderTemplate, &st.BookingShowPrices, &st.CancelMinHours)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, nil
	}
	if st.Rooms == nil {
		st.Rooms = []string{}
	}
	if st.RemindHours == nil {
		st.RemindHours = []int{}
	}
	return st, err
}

func (s *Server) getAgendaSettings(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	st, err := loadAgendaSettings(r, s.db, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": st})
}

func (in *agendaSettings) validate() string {
	if in.SlotMinutes < 5 || in.SlotMinutes > 480 {
		return "El intervalo de citas debe estar entre 5 y 480 minutos."
	}
	rooms := []string{}
	for _, name := range in.Rooms {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if utf8.RuneCountInString(name) > 40 {
			return "El nombre de una sala es demasiado largo."
		}
		if !slices.Contains(rooms, name) {
			rooms = append(rooms, name)
		}
	}
	if len(rooms) > 20 {
		return "Máximo 20 salas."
	}
	in.Rooms = rooms
	in.BookingSlug = strings.ToLower(strings.TrimSpace(in.BookingSlug))
	if in.BookingSlug != "" && !slugRe.MatchString(in.BookingSlug) {
		return "La dirección de reservas debe tener de 2 a 63 caracteres: letras minúsculas, números y guiones, y empezar con letra o número."
	}
	if in.BookingEnabled && in.BookingSlug == "" {
		return "Elige una dirección para activar las reservas en línea."
	}
	if in.BookingLeadHours < 0 || in.BookingLeadHours > 720 {
		return "La anticipación mínima debe estar entre 0 y 720 horas."
	}
	if in.BookingHorizonDays < 1 || in.BookingHorizonDays > 365 {
		return "El horizonte de reservas debe estar entre 1 y 365 días."
	}
	if in.CancelMinHours < 0 || in.CancelMinHours > 720 {
		return "El tiempo mínimo para cancelar debe estar entre 0 y 720 horas."
	}
	in.BookingMessage = strings.TrimSpace(in.BookingMessage)
	in.ReminderTemplate = strings.TrimSpace(in.ReminderTemplate)
	if utf8.RuneCountInString(in.BookingMessage) > 500 || utf8.RuneCountInString(in.ReminderTemplate) > 1000 {
		return "Uno de los textos es demasiado largo."
	}
	hours := []int{}
	for _, h := range in.RemindHours {
		if h < 1 || h > 168 {
			return "Los recordatorios deben enviarse entre 1 y 168 horas antes."
		}
		if !slices.Contains(hours, h) {
			hours = append(hours, h)
		}
	}
	if len(hours) > 3 {
		return "Máximo 3 recordatorios por cita."
	}
	sort.Sort(sort.Reverse(sort.IntSlice(hours)))
	in.RemindHours = hours
	return ""
}

func (s *Server) updateAgendaSettings(w http.ResponseWriter, r *http.Request) {
	var in agendaSettings
	if !decode(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	var slug any
	if in.BookingSlug != "" {
		slug = in.BookingSlug
		var taken bool
		if err := s.db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM agenda_settings WHERE lower(booking_slug) = $1 AND clinic_id <> $2)`, in.BookingSlug, p.ClinicID).Scan(&taken); err != nil {
			serverError(w, r, err)
			return
		}
		if taken {
			writeJSON(w, http.StatusConflict, errorBody{Code: "SLUG_TAKEN", Message: "Esa dirección de reservas ya la usa otro consultorio."})
			return
		}
	}
	_, err := s.db.Exec(r.Context(), `
		INSERT INTO agenda_settings (clinic_id, slot_minutes, rooms, booking_enabled, booking_slug, booking_lead_hours, booking_horizon_days,
			booking_message, booking_requires_confirmation, remind_email, remind_whatsapp, remind_hours, reminder_template, booking_show_prices, cancel_min_hours)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (clinic_id) DO UPDATE SET slot_minutes = $2, rooms = $3, booking_enabled = $4, booking_slug = $5, booking_lead_hours = $6,
			booking_horizon_days = $7, booking_message = $8, booking_requires_confirmation = $9, remind_email = $10, remind_whatsapp = $11,
			remind_hours = $12, reminder_template = $13, booking_show_prices = $14, cancel_min_hours = $15, updated_at = now()`,
		p.ClinicID, in.SlotMinutes, in.Rooms, in.BookingEnabled, slug, in.BookingLeadHours, in.BookingHorizonDays, in.BookingMessage,
		in.BookingRequiresConfirmation, in.RemindEmail, in.RemindWhatsapp, in.RemindHours, in.ReminderTemplate, in.BookingShowPrices, in.CancelMinHours)
	if isUniqueViolation(err) {
		writeJSON(w, http.StatusConflict, errorBody{Code: "SLUG_TAKEN", Message: "Esa dirección de reservas ya la usa otro consultorio."})
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "agenda_settings", "Cambió la configuración de la agenda", map[string]any{"booking_enabled": in.BookingEnabled})
	writeJSON(w, http.StatusOK, map[string]any{"settings": in})
}
