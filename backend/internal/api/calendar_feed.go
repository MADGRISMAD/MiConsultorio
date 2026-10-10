package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Private calendar feed. A professional turns it on and gets an address that Google Calendar (or Apple/Outlook) can
// subscribe to; their appointments then show in their own calendar. It is one-way: the feed is only published by
// Caresia, and nothing of the person's calendar is ever read. The address holds a secret token (anyone with it can read
// the feed) that can be replaced at any time, which cuts off the old one. By default the events do not carry the
// patient's name, so no health-related data lands in a third-party calendar unless the professional chooses it.

const (
	feedPast   = 30 * 24 * time.Hour
	feedFuture = 365 * 24 * time.Hour
)

func (s *Server) feedState(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	var tok *string
	var names bool
	if err := s.db.QueryRow(r.Context(), `SELECT calendar_token, calendar_show_names FROM users WHERE id = $1 AND clinic_id = $2`, p.UserID, p.ClinicID).Scan(&tok, &names); err != nil {
		serverError(w, r, err)
		return
	}
	out := map[string]any{"enabled": tok != nil, "show_names": names, "path": ""}
	if tok != nil {
		out["path"] = "/api/public/calendar/" + *tok + ".ics"
	}
	writeJSON(w, http.StatusOK, out)
}

// feedUpdate: {"action": "enable" | "rotate" | "disable", "show_names": bool}
func (s *Server) feedUpdate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Action    string `json:"action"`
		ShowNames bool   `json:"show_names"`
	}
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	switch in.Action {
	case "enable", "rotate":
		tok, err := newToken()
		if err != nil {
			serverError(w, r, err)
			return
		}
		// "enable" keeps the current address when there is one
		q := `UPDATE users SET calendar_token = coalesce(calendar_token, $3), calendar_show_names = $4 WHERE id = $1 AND clinic_id = $2`
		if in.Action == "rotate" {
			q = `UPDATE users SET calendar_token = $3, calendar_show_names = $4 WHERE id = $1 AND clinic_id = $2`
		}
		if _, err := s.db.Exec(r.Context(), q, p.UserID, p.ClinicID, tok, in.ShowNames); err != nil {
			serverError(w, r, err)
			return
		}
	case "disable":
		if _, err := s.db.Exec(r.Context(), `UPDATE users SET calendar_token = NULL WHERE id = $1 AND clinic_id = $2`, p.UserID, p.ClinicID); err != nil {
			serverError(w, r, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "Acción no válida.")
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "calendar_feed", "Cambió su calendario de citas para Google Calendar ("+in.Action+")", nil)
	s.feedState(w, r)
}

func (s *Server) mountCalendarFeed(r chi.Router) {
	r.Get("/me/calendar", s.feedState)
	r.Post("/me/calendar", s.feedUpdate)
}

type feedAPI struct {
	*Server
	reads *rateLimiter
	bad   *rateLimiter
}

func (s *Server) mountPublicCalendar(r chi.Router) {
	f := &feedAPI{Server: s, reads: newRateLimiter(120, 10*time.Minute), bad: newRateLimiter(15, 15*time.Minute)}
	r.Get("/public/calendar/{file}", f.serve)
}

func (f *feedAPI) serve(w http.ResponseWriter, r *http.Request) {
	if !limit(w, f.reads, "cal|"+clientIP(r)) {
		return
	}
	tok, ok := strings.CutSuffix(chi.URLParam(r, "file"), ".ics")
	notFound := func() {
		f.bad.fail("cal|" + clientIP(r))
		writeError(w, http.StatusNotFound, "Este calendario no existe.")
	}
	if !ok || len(tok) < 20 || len(tok) > 100 || !f.bad.allow("cal|"+clientIP(r)) {
		notFound()
		return
	}
	ctx := r.Context()
	var userID, clinicID, proName, clinicName, address, tz string
	var names bool
	err := f.db.QueryRow(ctx, `
		SELECT u.id::text, u.clinic_id::text, u.name, u.calendar_show_names, c.name, c.address, coalesce(c.settings->>'timezone', '')
		FROM users u JOIN clinics c ON c.id = u.clinic_id
		WHERE u.calendar_token = $1 AND NOT u.disabled AND c.billing_status IN ('active', 'trialing', 'past_due')`, tok).
		Scan(&userID, &clinicID, &proName, &names, &clinicName, &address, &tz)
	if err != nil {
		notFound()
		return
	}
	loc := locationOrDefault(tz)
	now := time.Now()
	rows, err := f.db.Query(ctx, `
		SELECT a.id::text, to_char(a.date, 'YYYY-MM-DD'), to_char(a.start_hour, 'HH24:MI'), to_char(a.end_hour, 'HH24:MI'), a.names, a.last_names, a.status,
		       coalesce(ci.name, ''), a.room, a.updated_at
		FROM appointments a LEFT JOIN catalog_items ci ON ci.id = a.service_id
		WHERE a.clinic_id = $1 AND a.professional_id = $2 AND a.status NOT IN ('cancelled', 'no_show')
		  AND a.date BETWEEN $3::date AND $4::date ORDER BY a.date, a.start_hour LIMIT 3000`,
		clinicID, userID, now.Add(-feedPast).In(loc).Format("2006-01-02"), now.Add(feedFuture).In(loc).Format("2006-01-02"))
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Caresia//Agenda//ES", "CALSCALE:GREGORIAN", "METHOD:PUBLISH",
		"X-WR-CALNAME:" + icsText("Citas · "+clinicName), "X-WR-CALDESC:" + icsText("Tus citas de "+clinicName+" en Caresia"), "REFRESH-INTERVAL;VALUE=DURATION:PT1H", "X-PUBLISHED-TTL:PT1H"}
	stamp := now.UTC().Format(icsStamp)
	for rows.Next() {
		var id, date, start, end, first, last, status, service, room string
		var upd time.Time
		if err := rows.Scan(&id, &date, &start, &end, &first, &last, &status, &service, &room, &upd); err != nil {
			serverError(w, r, err)
			return
		}
		st, e1 := time.ParseInLocation("2006-01-02 15:04", date+" "+start, loc)
		en, e2 := time.ParseInLocation("2006-01-02 15:04", date+" "+end, loc)
		if e1 != nil || e2 != nil {
			continue
		}
		if !en.After(st) {
			en = st.Add(calendarDefaultMinutes * time.Minute)
		}
		title := "Cita"
		if names {
			title = "Cita · " + strings.TrimSpace(first+" "+last)
		}
		if service != "" {
			title += " · " + service
		}
		var desc []string
		if room != "" {
			desc = append(desc, "Sala: "+room)
		}
		desc = append(desc, "Abre Caresia para ver el detalle.")
		evStatus := "CONFIRMED"
		if status == "scheduled" {
			evStatus = "TENTATIVE"
		}
		ev := []string{"BEGIN:VEVENT", "UID:cita-" + id + "@caresia", "DTSTAMP:" + stamp, "LAST-MODIFIED:" + upd.UTC().Format(icsStamp),
			"SEQUENCE:" + itoa(int(max(upd.Unix(), 0)%2000000000)), "DTSTART:" + st.UTC().Format(icsStamp), "DTEND:" + en.UTC().Format(icsStamp),
			"SUMMARY:" + icsText(title), "LOCATION:" + icsText(address), "DESCRIPTION:" + icsText(strings.Join(desc, "\n")), "STATUS:" + evStatus, "TRANSP:OPAQUE", "END:VEVENT"}
		lines = append(lines, ev...)
	}
	if err := rows.Err(); err != nil {
		serverError(w, r, err)
		return
	}
	lines = append(lines, "END:VCALENDAR")
	for i, l := range lines {
		lines[i] = icsFold(l)
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("Content-Disposition", `inline; filename="citas.ics"`)
	_, _ = w.Write([]byte(strings.Join(lines, "\r\n") + "\r\n"))
}
