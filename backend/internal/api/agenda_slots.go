package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// ---------------------------------------------------------------------------
// Duration per service
// ---------------------------------------------------------------------------

// slotServiceMinutes is the duration in minutes of a service of the clinic, or 0 when it has none
// (or serviceID is empty or unknown): the professional's slot applies then.
func (s *Server) slotServiceMinutes(ctx context.Context, q queryRower, clinicID, serviceID string) int {
	if !validUUID(serviceID) {
		return 0
	}
	var m *int
	if q.QueryRow(ctx, `SELECT duration_minutes FROM catalog_items WHERE clinic_id = $1 AND id = $2 AND kind = 'service'`, clinicID, serviceID).Scan(&m) != nil || m == nil {
		return 0
	}
	return *m
}

// slotFillEnd gives an internal appointment its end time when the client sent a service and no end:
// the service's duration, or the professional's slot (the clinic's when there is none).
func (s *Server) slotFillEnd(ctx context.Context, q queryRower, clinicID string, in *appointmentIn) {
	if in.EndHour != "" || in.StartHour == "" {
		return
	}
	start, err := time.Parse("15:04", in.StartHour)
	if err != nil {
		return
	}
	mins := 0
	if in.ServiceID != nil {
		mins = s.slotServiceMinutes(ctx, q, clinicID, *in.ServiceID)
	}
	if mins == 0 && in.ProfessionalID != nil {
		_ = q.QueryRow(ctx, `
			SELECT coalesce(ps.slot_minutes, ags.slot_minutes, 30)
			FROM users u LEFT JOIN professional_settings ps ON ps.user_id = u.id LEFT JOIN agenda_settings ags ON ags.clinic_id = u.clinic_id
			WHERE u.clinic_id = $1 AND u.id = $2`, clinicID, *in.ProfessionalID).Scan(&mins)
	}
	if mins == 0 {
		_ = q.QueryRow(ctx, `SELECT coalesce((SELECT slot_minutes FROM agenda_settings WHERE clinic_id = $1), 30)`, clinicID).Scan(&mins)
	}
	if mins > 0 {
		in.EndHour = start.Add(time.Duration(mins) * time.Minute).Format("15:04")
	}
}

// setServiceDuration is PUT /agenda/services/{id}/duration. minutes null clears it.
func (s *Server) setServiceDuration(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var in struct {
		Minutes *int `json:"duration_minutes"`
	}
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Servicio no encontrado.")
		return
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Minutes != nil && (*in.Minutes < 5 || *in.Minutes > 480) {
		writeError(w, http.StatusBadRequest, "La duración debe estar entre 5 y 480 minutos.")
		return
	}
	p := principalFrom(r.Context())
	tag, err := s.db.Exec(r.Context(), `UPDATE catalog_items SET duration_minutes = $3, updated_at = now() WHERE clinic_id = $1 AND id = $2 AND kind = 'service'`, p.ClinicID, id, in.Minutes)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Servicio no encontrado.")
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "service_duration", "Cambió la duración de un servicio", map[string]any{"service": id, "minutes": in.Minutes})
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "duration_minutes": in.Minutes})
}

// ---------------------------------------------------------------------------
// Time spans
// ---------------------------------------------------------------------------

// slotSpan is a range of the day in minutes since midnight.
type slotSpan struct{ from, to int }

type slotSpans []slotSpan

func slotClock(hhmm string) int {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0
	}
	return t.Hour()*60 + t.Minute()
}

func (l slotSpans) overlapsMin(from, to int) bool {
	for _, x := range l {
		if x.from < to && x.to > from {
			return true
		}
	}
	return false
}

func (l slotSpans) overlaps(start, end string) bool {
	return l.overlapsMin(slotClock(start), slotClock(end))
}

// slotHeldSpans are the slots of a professional on a date that are on offer to someone on the waitlist or that
// another visitor of the booking page is filling in (holder is the caller's own key, whose holds do not count).
func (s *Server) slotHeldSpans(ctx context.Context, q rowsQuerier, clinicID, professionalID, date, holder string) (slotSpans, error) {
	rows, err := q.Query(ctx, `
		SELECT to_char(start_hour, 'HH24:MI'), to_char(end_hour, 'HH24:MI') FROM waitlist_offers
		WHERE clinic_id = $1 AND professional_id = $2 AND date = $3::date AND status = 'offered' AND expires_at > now()
		UNION ALL
		SELECT to_char(start_hour, 'HH24:MI'), to_char(end_hour, 'HH24:MI') FROM booking_holds
		WHERE clinic_id = $1 AND professional_id = $2 AND date = $3::date AND expires_at > now() AND holder <> $4`, clinicID, professionalID, date, holder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out slotSpans
	for rows.Next() {
		var a, b string
		if err := rows.Scan(&a, &b); err != nil {
			return nil, err
		}
		out = append(out, slotSpan{slotClock(a), slotClock(b)})
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Monthly availability (public)
// ---------------------------------------------------------------------------

// slotBusy holds what makes a professional busy over a range of dates, loaded in three queries.
type slotBusy struct {
	appts  map[string]slotSpans // professional|date
	holds  map[string]slotSpans
	blocks []slotBlock
}

type slotBlock struct {
	pro        *string
	from, to   string
	whole      bool
	start, end int
}

func slotKey(pro, date string) string { return pro + "|" + date }

// loadSlotBusy reads appointments, offers on hold and blocks between from and to (dates) for the professionals.
func (s *Server) loadSlotBusy(ctx context.Context, clinicID string, pros []string, from, to string) (*slotBusy, error) {
	b := &slotBusy{appts: map[string]slotSpans{}, holds: map[string]slotSpans{}}
	scan := func(sql string, into map[string]slotSpans) error {
		rows, err := s.db.Query(ctx, sql, clinicID, pros, from, to)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var pro, date, a, e string
			if err := rows.Scan(&pro, &date, &a, &e); err != nil {
				return err
			}
			k := slotKey(pro, date)
			into[k] = append(into[k], slotSpan{slotClock(a), slotClock(e)})
		}
		return rows.Err()
	}
	if err := scan(`
		SELECT professional_id::text, to_char(date, 'YYYY-MM-DD'), to_char(start_hour, 'HH24:MI'), to_char(end_hour, 'HH24:MI')
		FROM appointments WHERE clinic_id = $1 AND professional_id = ANY($2::uuid[]) AND date BETWEEN $3::date AND $4::date
		  AND status NOT IN ('cancelled', 'no_show')`, b.appts); err != nil {
		return nil, err
	}
	if err := scan(`
		SELECT professional_id::text, to_char(date, 'YYYY-MM-DD'), to_char(start_hour, 'HH24:MI'), to_char(end_hour, 'HH24:MI')
		FROM waitlist_offers WHERE clinic_id = $1 AND professional_id = ANY($2::uuid[]) AND date BETWEEN $3::date AND $4::date
		  AND status = 'offered' AND expires_at > now()`, b.holds); err != nil {
		return nil, err
	}
	if err := scan(`
		SELECT professional_id::text, to_char(date, 'YYYY-MM-DD'), to_char(start_hour, 'HH24:MI'), to_char(end_hour, 'HH24:MI')
		FROM booking_holds WHERE clinic_id = $1 AND professional_id = ANY($2::uuid[]) AND date BETWEEN $3::date AND $4::date
		  AND expires_at > now()`, b.holds); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT professional_id::text, to_char(date_from, 'YYYY-MM-DD'), to_char(date_to, 'YYYY-MM-DD'),
		       start_hour IS NULL, coalesce(to_char(start_hour, 'HH24:MI'), '00:00'), coalesce(to_char(end_hour, 'HH24:MI'), '00:00')
		FROM time_blocks WHERE clinic_id = $1 AND date_to >= $2::date AND date_from <= $3::date`, clinicID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var x slotBlock
		var a, e string
		if err := rows.Scan(&x.pro, &x.from, &x.to, &x.whole, &a, &e); err != nil {
			return nil, err
		}
		x.start, x.end = slotClock(a), slotClock(e)
		b.blocks = append(b.blocks, x)
	}
	return b, rows.Err()
}

// free says whether the professional can take [start, end) on date; it mirrors slotConflict plus the
// offers on hold, but answers from memory.
func (b *slotBusy) free(pro, date string, start, end int) bool {
	for _, x := range b.blocks {
		if date < x.from || date > x.to || (x.pro != nil && *x.pro != pro) {
			continue
		}
		if x.whole || (x.start < end && x.end > start) {
			return false
		}
	}
	k := slotKey(pro, date)
	return !b.appts[k].overlapsMin(start, end) && !b.holds[k].overlapsMin(start, end)
}

// monthRange parses "YYYY-MM" into its first and last day.
func monthRange(month string, loc *time.Location) (first, last time.Time, ok bool) {
	t, err := time.ParseInLocation("2006-01", month, loc)
	if err != nil {
		return first, last, false
	}
	return t, t.AddDate(0, 1, -1), true
}

type monthDay struct {
	Date  string `json:"date"`
	Slots int    `json:"slots"`
}

// bookingMonth is GET /public/booking/{slug}/month: the days of a month with at least one free slot,
// computed in one pass over the month's appointments and blocks.
func (b *bookingAPI) bookingMonth(w http.ResponseWriter, r *http.Request) {
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
	qs := r.URL.Query()
	first, last, ok := monthRange(qs.Get("month"), c.Loc)
	if !ok {
		writeError(w, http.StatusBadRequest, "El mes no es válido.")
		return
	}
	profID, svc := qs.Get("professional"), qs.Get("service")
	if profID != "" && !validUUID(profID) {
		writeError(w, http.StatusBadRequest, "Elige un profesional.")
		return
	}
	if svc != "" && !b.serviceOK(ctx, c.ID, svc) {
		writeError(w, http.StatusBadRequest, "El servicio no es válido.")
		return
	}
	pros, err := b.listBookable(ctx, c, profID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	days := []monthDay{}
	if len(pros) > 0 {
		ids := make([]string, len(pros))
		for i, p := range pros {
			ids[i] = p.ID
		}
		busy, err := b.loadSlotBusy(ctx, c.ID, ids, first.Format("2006-01-02"), last.Format("2006-01-02"))
		if err != nil {
			serverError(w, r, err)
			return
		}
		dur := b.slotServiceMinutes(ctx, b.db, c.ID, svc)
		now := time.Now()
		for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
			date := d.Format("2006-01-02")
			n := 0
			for _, p := range pros {
				for _, st := range c.candidateSlots(p, date, now, dur) {
					if busy.free(p.ID, date, slotClock(st), slotClock(p.end(c, st, dur))) {
						n++
					}
				}
			}
			if n > 0 {
				days = append(days, monthDay{Date: date, Slots: n})
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"month": first.Format("2006-01"), "days": days})
}

// ---------------------------------------------------------------------------
// Monthly load (staff)
// ---------------------------------------------------------------------------

type loadDay struct {
	Date            string  `json:"date"`
	Appointments    int     `json:"appointments"`
	BookedMinutes   int     `json:"booked_minutes"`
	CapacityMinutes int     `json:"capacity_minutes"`
	Load            float64 `json:"load"`    // booked / capacity, 0 when there is no capacity
	Blocked         bool    `json:"blocked"` // the whole clinic is blocked that day
}

// agendaMonthLoad is GET /agenda/month-load?month=YYYY-MM[&professional=]: how full each day of the
// month is, to paint the month view. Capacity is the working hours of the professionals who have agenda
// settings (every active professional when none has), minus blocks.
func (s *Server) agendaMonthLoad(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	ctx := r.Context()
	loc := clinicLocation(ctx, s.db, p.ClinicID)
	first, last, ok := monthRange(r.URL.Query().Get("month"), loc)
	if !ok {
		writeError(w, http.StatusBadRequest, "El mes no es válido.")
		return
	}
	onlyPro := r.URL.Query().Get("professional")
	if onlyPro != "" && !validUUID(onlyPro) {
		writeError(w, http.StatusBadRequest, "El filtro no es válido.")
		return
	}
	// Working hours of each professional (their own, else the clinic's).
	var clinicRaw []byte
	if err := s.db.QueryRow(ctx, `SELECT settings FROM clinics WHERE id = $1`, p.ClinicID).Scan(&clinicRaw); err != nil {
		serverError(w, r, err)
		return
	}
	var clinicHours Settings
	_ = json.Unmarshal(clinicRaw, &clinicHours)
	clinicHours = clinicHours.normalized()
	rows, err := s.db.Query(ctx, `
		SELECT u.id::text, coalesce(ps.hours, '{}'::jsonb), ps.user_id IS NOT NULL
		FROM users u LEFT JOIN professional_settings ps ON ps.user_id = u.id
		WHERE u.clinic_id = $1 AND NOT u.disabled AND u.role IN ('admin', 'doctor') AND ($2 = '' OR u.id::text = $2)`, p.ClinicID, onlyPro)
	if err != nil {
		serverError(w, r, err)
		return
	}
	type proHours struct {
		id         string
		hours      map[string][][]string
		configured bool
	}
	var all []proHours
	anyConfigured := false
	for rows.Next() {
		var x proHours
		var raw []byte
		if err := rows.Scan(&x.id, &raw, &x.configured); err != nil {
			rows.Close()
			serverError(w, r, err)
			return
		}
		_ = json.Unmarshal(raw, &x.hours)
		anyConfigured = anyConfigured || x.configured
		all = append(all, x)
	}
	rows.Close()
	var pros []proHours
	for _, x := range all {
		if x.configured || !anyConfigured {
			pros = append(pros, x)
		}
	}
	ids := make([]string, len(pros))
	for i, x := range pros {
		ids[i] = x.id
	}
	from, to := first.Format("2006-01-02"), last.Format("2006-01-02")
	busy, err := s.loadSlotBusy(ctx, p.ClinicID, ids, from, to)
	if err != nil {
		serverError(w, r, err)
		return
	}
	// Appointments per day: every professional (and unassigned ones when not filtering).
	count := map[string][2]int{}
	arows, err := s.db.Query(ctx, `
		SELECT to_char(date, 'YYYY-MM-DD'), count(*)::int, coalesce(sum(extract(epoch FROM end_hour - start_hour) / 60), 0)::int
		FROM appointments WHERE clinic_id = $1 AND date BETWEEN $2::date AND $3::date AND status NOT IN ('cancelled', 'no_show')
		  AND ($4 = '' OR professional_id::text = $4)
		GROUP BY 1`, p.ClinicID, from, to, onlyPro)
	if err != nil {
		serverError(w, r, err)
		return
	}
	for arows.Next() {
		var d string
		var n, m int
		if err := arows.Scan(&d, &n, &m); err != nil {
			arows.Close()
			serverError(w, r, err)
			return
		}
		count[d] = [2]int{n, m}
	}
	arows.Close()
	days := []loadDay{}
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		key := weekdayKeys[d.Weekday()]
		day := loadDay{Date: date, Appointments: count[date][0], BookedMinutes: count[date][1]}
		for _, x := range busy.blocks {
			if x.pro == nil && x.whole && date >= x.from && date <= x.to {
				day.Blocked = true
			}
		}
		for _, pr := range pros {
			var wins []slotSpan
			if len(pr.hours) > 0 {
				for _, w := range pr.hours[key] {
					if len(w) == 2 {
						wins = append(wins, slotSpan{slotClock(w[0]), slotClock(w[1])})
					}
				}
			} else if h := clinicHours.Hours[key]; h.Open {
				wins = append(wins, slotSpan{slotClock(h.Start), slotClock(h.End)})
			}
			for _, wn := range wins {
				cap := wn.to - wn.from
				for m := wn.from; m < wn.to; m += 5 { // minutes lost to blocks, in 5-minute steps
					if !busy.freeOfBlocks(pr.id, date, m, m+5) {
						cap -= 5
					}
				}
				if cap > 0 {
					day.CapacityMinutes += cap
				}
			}
		}
		if day.CapacityMinutes > 0 {
			day.Load = float64(day.BookedMinutes) / float64(day.CapacityMinutes)
		}
		days = append(days, day)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Date < days[j].Date })
	writeJSON(w, http.StatusOK, map[string]any{"month": first.Format("2006-01"), "days": days})
}

// freeOfBlocks is free ignoring appointments and offers.
func (b *slotBusy) freeOfBlocks(pro, date string, start, end int) bool {
	for _, x := range b.blocks {
		if date < x.from || date > x.to || (x.pro != nil && *x.pro != pro) {
			continue
		}
		if x.whole || (x.start < end && x.end > start) {
			return false
		}
	}
	return true
}
