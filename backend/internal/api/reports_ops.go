package api

import (
	"context"
	"encoding/csv"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

const rptMaxDays = 731 // two years

// rptWindow is a validated report range: inclusive days plus the equivalent half-open timestamp range.
type rptWindow struct {
	From, To       time.Time // [From, To) in the server zone
	FromDay, ToDay string    // inclusive, YYYY-MM-DD
	Days           int
	TZ             string // IANA name used by SQL "AT TIME ZONE"
}

// rptTZ is the zone used to group by day and hour: TZ when valid, else UTC (the zone time.Local has then).
func rptTZ() string {
	if tz := os.Getenv("TZ"); tz != "" {
		if _, err := time.LoadLocation(tz); err == nil {
			return tz
		}
	}
	return "UTC"
}

// rptParseWindow reads ?from&to (default: the last 30 days) and enforces the 2-year limit.
func rptParseWindow(q func(string) string) (rptWindow, string) {
	loc := time.Local
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	start, end := today.AddDate(0, 0, -29), today.AddDate(0, 0, 1)
	if v := q("from"); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, loc)
		if err != nil {
			return rptWindow{}, "La fecha inicial no es válida."
		}
		start = t
	}
	if v := q("to"); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, loc)
		if err != nil {
			return rptWindow{}, "La fecha final no es válida."
		}
		end = t.AddDate(0, 0, 1)
	}
	if !end.After(start) {
		return rptWindow{}, "El rango de fechas no es válido."
	}
	days := int(end.Sub(start).Hours()/24 + 0.5)
	if days > rptMaxDays {
		return rptWindow{}, "El rango no puede ser mayor a 2 años."
	}
	return rptWindow{From: start, To: end, FromDay: start.Format("2006-01-02"), ToDay: end.AddDate(0, 0, -1).Format("2006-01-02"), Days: days, TZ: rptTZ()}, ""
}

type rptCount struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type rptRate struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Total  int     `json:"total"`
	NoShow int     `json:"no_show"`
	Rate   float64 `json:"rate"` // percent, 0 when Total is 0
}

type rptProfessional struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Encounters   int      `json:"encounters"`
	Appointments int      `json:"appointments"`
	NoShow       int      `json:"no_show"`
	Completed    int      `json:"completed"`
	AvgAttention *float64 `json:"avg_attention_min"`
	AvgWait      *float64 `json:"avg_wait_min"`
}

type rptOperationsReport struct {
	Granularity string `json:"granularity"`
	Scope       string `json:"scope"` // "clinic" or "own"
	Encounters  struct {
		Total    int        `json:"total"`
		ByPeriod []rptCount `json:"by_period"`
	} `json:"encounters"`
	Professionals []rptProfessional `json:"professionals"`
	Appointments  struct {
		Total            int        `json:"total"`
		ByStatus         []rptCount `json:"by_status"`
		BySource         []rptCount `json:"by_source"`
		CancellationRate float64    `json:"cancellation_rate"`
	} `json:"appointments"`
	NoShow struct {
		Eligible       int       `json:"eligible"` // past appointments that were not cancelled
		Count          int       `json:"count"`
		Rate           float64   `json:"rate"`
		ByProfessional []rptRate `json:"by_professional"`
		ByService      []rptRate `json:"by_service"`
		ByWeekday      []rptRate `json:"by_weekday"`
		ByHour         []rptRate `json:"by_hour"`
	} `json:"no_show"`
	Times struct {
		AvgAttentionMin *float64 `json:"avg_attention_min"` // started_at -> finished_at, completed appointments
		AvgWaitMin      *float64 `json:"avg_wait_min"`      // arrived_at -> started_at
		Samples         int      `json:"samples"`
	} `json:"times"`
	// Occupancy is approximate: minutes booked (not cancelled / no show) over minutes available
	// according to each professional's hours (or the clinic's) minus time blocks, for the whole range.
	Occupancy struct {
		BookedMin    int      `json:"booked_min"`
		AvailableMin int      `json:"available_min"`
		Rate         *float64 `json:"rate"`
	} `json:"occupancy"`
	PeakHours struct {
		ByHour  []rptCount `json:"by_hour"`
		Heatmap [7][24]int `json:"heatmap"` // Monday = 0
	} `json:"peak_hours"`
}

var rptStatusLabels = map[string]string{
	"scheduled": "Agendada", "confirmed": "Confirmada", "arrived": "En sala", "in_progress": "En consulta",
	"completed": "Atendida", "no_show": "No asistió", "cancelled": "Cancelada",
}
var rptSourceLabels = map[string]string{"staff": "Recepción", "online": "Reserva en línea", "portal": "Portal del paciente"}
var rptWeekdayLabels = []string{"Lunes", "Martes", "Miércoles", "Jueves", "Viernes", "Sábado", "Domingo"}

func rptPct(n, of int) float64 {
	if of <= 0 {
		return 0
	}
	return float64(int(float64(n)/float64(of)*1000+0.5)) / 10
}

func rptRound1(v float64) *float64 {
	v = float64(int(v*10+0.5)) / 10
	return &v
}

// rptScope returns the professional filter: doctors only ever see their own numbers.
func rptScope(p *Principal, requested string) (pro, scope, msg string) {
	if !hasPermission(p.Permissions, PermAdminUsers) {
		return p.UserID, "own", ""
	}
	if requested != "" && !validUUID(requested) {
		return "", "", "El profesional no es válido."
	}
	return requested, "clinic", ""
}

// rptOperationsParams validates the request shared by the JSON and CSV endpoints.
func (s *Server) rptOperationsParams(w http.ResponseWriter, r *http.Request) (*Principal, rptWindow, string, string, bool) {
	p := principalFrom(r.Context())
	win, msg := rptParseWindow(r.URL.Query().Get)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return nil, win, "", "", false
	}
	pro, scope, msg := rptScope(p, r.URL.Query().Get("professional"))
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return nil, win, "", "", false
	}
	return p, win, pro, scope, true
}

func (s *Server) rptOperations(w http.ResponseWriter, r *http.Request) {
	p, win, pro, scope, ok := s.rptOperationsParams(w, r)
	if !ok {
		return
	}
	rep, err := s.rptBuildOperations(r.Context(), p.ClinicID, win, pro, scope)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"report": rep, "from": win.FromDay, "to": win.ToDay})
}

func (s *Server) rptBuildOperations(ctx context.Context, clinicID string, win rptWindow, pro, scope string) (*rptOperationsReport, error) {
	rep := &rptOperationsReport{Scope: scope}
	rep.Encounters.ByPeriod = []rptCount{}
	rep.Professionals = []rptProfessional{}
	rep.Appointments.ByStatus, rep.Appointments.BySource = []rptCount{}, []rptCount{}
	rep.NoShow.ByProfessional, rep.NoShow.ByService, rep.NoShow.ByWeekday, rep.NoShow.ByHour = []rptRate{}, []rptRate{}, []rptRate{}, []rptRate{}
	rep.PeakHours.ByHour = []rptCount{}

	switch {
	case win.Days <= 31:
		rep.Granularity = "day"
	case win.Days <= 190:
		rep.Granularity = "week"
	default:
		rep.Granularity = "month"
	}
	tz := win.TZ
	// $1 clinic, $2/$3 first and last day, $4 professional filter ('' = everyone); appointments use wall-clock dates.
	const apptWhere = `a.clinic_id = $1 AND a.date >= $2::date AND a.date <= $3::date AND ($4 = '' OR a.professional_id::text = $4)`
	const pastCond = `(a.date + a.start_hour) < (now() AT TIME ZONE $5)`

	// encounters per period
	rows, err := s.db.Query(ctx, `
		SELECT to_char(date_trunc('`+rep.Granularity+`', e.occurred_at AT TIME ZONE $5), 'YYYY-MM-DD'), count(*)
		FROM encounters e
		WHERE e.clinic_id = $1 AND e.occurred_at >= $2 AND e.occurred_at < $3 AND e.addendum_of IS NULL
		  AND e.kind IN ('consulta','seguimiento','procedimiento') AND ($4 = '' OR e.author_id::text = $4)
		GROUP BY 1 ORDER BY 1`, clinicID, win.From, win.To, pro, tz)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var c rptCount
		if err := rows.Scan(&c.Key, &c.Count); err != nil {
			rows.Close()
			return nil, err
		}
		c.Label = c.Key
		rep.Encounters.Total += c.Count
		rep.Encounters.ByPeriod = append(rep.Encounters.ByPeriod, c)
	}
	rows.Close()

	profs := map[string]*rptProfessional{}
	getPro := func(id, name string) *rptProfessional {
		if x, ok := profs[id]; ok {
			if x.Name == "" {
				x.Name = name
			}
			return x
		}
		x := &rptProfessional{ID: id, Name: name}
		profs[id] = x
		return x
	}
	rows, err = s.db.Query(ctx, `
		SELECT coalesce(e.author_id::text, ''), max(e.author_name), count(*)
		FROM encounters e
		WHERE e.clinic_id = $1 AND e.occurred_at >= $2 AND e.occurred_at < $3 AND e.addendum_of IS NULL
		  AND e.kind IN ('consulta','seguimiento','procedimiento') AND ($4 = '' OR e.author_id::text = $4)
		GROUP BY 1`, clinicID, win.From, win.To, pro)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, name string
		var n int
		if err := rows.Scan(&id, &name, &n); err != nil {
			rows.Close()
			return nil, err
		}
		getPro(id, name).Encounters = n
	}
	rows.Close()

	// appointments by status / source / professional, with attention and wait times
	rows, err = s.db.Query(ctx, `
		SELECT a.status, a.source, coalesce(a.professional_id::text, ''), coalesce(max(u.name), ''), count(*),
		       avg(extract(epoch FROM a.finished_at - a.started_at) / 60) FILTER (WHERE a.status = 'completed' AND a.finished_at > a.started_at),
		       count(*) FILTER (WHERE a.status = 'completed' AND a.finished_at > a.started_at),
		       avg(extract(epoch FROM a.started_at - a.arrived_at) / 60) FILTER (WHERE a.started_at >= a.arrived_at),
		       count(*) FILTER (WHERE a.started_at >= a.arrived_at)
		FROM appointments a LEFT JOIN users u ON u.id = a.professional_id
		WHERE `+apptWhere+` GROUP BY 1,2,3`, clinicID, win.FromDay, win.ToDay, pro)
	if err != nil {
		return nil, err
	}
	statusN, sourceN := map[string]int{}, map[string]int{}
	type acc struct {
		attSum, waitSum float64
		attN, waitN     int
	}
	var tot acc
	perPro := map[string]*acc{}
	for rows.Next() {
		var st, src, id, name string
		var n, an, wn int
		var att, wait *float64
		if err := rows.Scan(&st, &src, &id, &name, &n, &att, &an, &wait, &wn); err != nil {
			rows.Close()
			return nil, err
		}
		statusN[st] += n
		sourceN[src] += n
		rep.Appointments.Total += n
		x := getPro(id, name)
		x.Appointments += n
		if st == "no_show" {
			x.NoShow += n
		}
		if st == "completed" {
			x.Completed += n
		}
		a := perPro[id]
		if a == nil {
			a = &acc{}
			perPro[id] = a
		}
		if att != nil {
			tot.attSum += *att * float64(an)
			tot.attN += an
			a.attSum += *att * float64(an)
			a.attN += an
		}
		if wait != nil {
			tot.waitSum += *wait * float64(wn)
			tot.waitN += wn
			a.waitSum += *wait * float64(wn)
			a.waitN += wn
		}
	}
	rows.Close()
	if tot.attN > 0 {
		rep.Times.AvgAttentionMin = rptRound1(tot.attSum / float64(tot.attN))
	}
	if tot.waitN > 0 {
		rep.Times.AvgWaitMin = rptRound1(tot.waitSum / float64(tot.waitN))
	}
	rep.Times.Samples = tot.attN
	for id, a := range perPro {
		if a.attN > 0 {
			profs[id].AvgAttention = rptRound1(a.attSum / float64(a.attN))
		}
		if a.waitN > 0 {
			profs[id].AvgWait = rptRound1(a.waitSum / float64(a.waitN))
		}
	}
	for _, st := range []string{"scheduled", "confirmed", "arrived", "in_progress", "completed", "no_show", "cancelled"} {
		rep.Appointments.ByStatus = append(rep.Appointments.ByStatus, rptCount{Key: st, Label: rptStatusLabels[st], Count: statusN[st]})
	}
	for _, src := range []string{"staff", "online", "portal"} {
		rep.Appointments.BySource = append(rep.Appointments.BySource, rptCount{Key: src, Label: rptSourceLabels[src], Count: sourceN[src]})
	}
	rep.Appointments.CancellationRate = rptPct(statusN["cancelled"], rep.Appointments.Total)
	for id, x := range profs {
		if x.Name == "" {
			x.Name = "Sin asignar"
			if id != "" {
				x.Name = "Profesional"
			}
		}
		rep.Professionals = append(rep.Professionals, *x)
	}
	sort.Slice(rep.Professionals, func(i, j int) bool {
		a, b := rep.Professionals[i], rep.Professionals[j]
		if a.Encounters+a.Appointments != b.Encounters+b.Appointments {
			return a.Encounters+a.Appointments > b.Encounters+b.Appointments
		}
		return a.Name < b.Name
	})

	// no-show breakdowns: appointments already in the past that were not cancelled
	noShow := func(keyExpr, labelExpr, order string) ([]rptRate, error) {
		rows, err := s.db.Query(ctx, `
			SELECT `+keyExpr+`, max(`+labelExpr+`), count(*), count(*) FILTER (WHERE a.status = 'no_show')
			FROM appointments a LEFT JOIN users u ON u.id = a.professional_id LEFT JOIN catalog_items ci ON ci.id = a.service_id
			WHERE `+apptWhere+` AND a.status <> 'cancelled' AND `+pastCond+`
			GROUP BY 1 ORDER BY `+order, clinicID, win.FromDay, win.ToDay, pro, tz)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []rptRate{}
		for rows.Next() {
			var x rptRate
			if err := rows.Scan(&x.Key, &x.Label, &x.Total, &x.NoShow); err != nil {
				return nil, err
			}
			x.Rate = rptPct(x.NoShow, x.Total)
			out = append(out, x)
		}
		return out, rows.Err()
	}
	if rep.NoShow.ByProfessional, err = noShow(`coalesce(a.professional_id::text, '')`, `coalesce(u.name, 'Sin asignar')`, `3 DESC, 1`); err != nil {
		return nil, err
	}
	if rep.NoShow.ByService, err = noShow(`coalesce(a.service_id::text, '')`, `coalesce(ci.name, 'Sin servicio')`, `3 DESC, 1`); err != nil {
		return nil, err
	}
	if rep.NoShow.ByWeekday, err = noShow(`(extract(isodow FROM a.date)::int - 1)::text`, `'x'`, `1`); err != nil {
		return nil, err
	}
	for i := range rep.NoShow.ByWeekday {
		k, _ := strconv.Atoi(rep.NoShow.ByWeekday[i].Key)
		rep.NoShow.ByWeekday[i].Label = rptWeekdayLabels[k%7]
	}
	if rep.NoShow.ByHour, err = noShow(`lpad(extract(hour FROM a.start_hour)::int::text, 2, '0')`, `'x'`, `1`); err != nil {
		return nil, err
	}
	for i := range rep.NoShow.ByHour {
		rep.NoShow.ByHour[i].Label = rep.NoShow.ByHour[i].Key + ":00"
	}
	for _, x := range rep.NoShow.ByWeekday {
		rep.NoShow.Eligible += x.Total
		rep.NoShow.Count += x.NoShow
	}
	rep.NoShow.Rate = rptPct(rep.NoShow.Count, rep.NoShow.Eligible)

	// peak hours: appointments that were not cancelled, by weekday and start hour
	rows, err = s.db.Query(ctx, `
		SELECT extract(isodow FROM a.date)::int - 1, extract(hour FROM a.start_hour)::int, count(*)
		FROM appointments a WHERE `+apptWhere+` AND a.status <> 'cancelled' GROUP BY 1,2`, clinicID, win.FromDay, win.ToDay, pro)
	if err != nil {
		return nil, err
	}
	byHour := [24]int{}
	for rows.Next() {
		var d, h, n int
		if err := rows.Scan(&d, &h, &n); err != nil {
			rows.Close()
			return nil, err
		}
		rep.PeakHours.Heatmap[d][h] += n
		byHour[h] += n
	}
	rows.Close()
	for h, n := range byHour {
		rep.PeakHours.ByHour = append(rep.PeakHours.ByHour, rptCount{Key: strconv.Itoa(h), Label: strconv.Itoa(h) + ":00", Count: n})
	}

	booked, avail, err := s.rptOccupancy(ctx, clinicID, win, pro)
	if err != nil {
		return nil, err
	}
	rep.Occupancy.BookedMin, rep.Occupancy.AvailableMin = booked, avail
	if avail > 0 {
		rep.Occupancy.Rate = rptRound1(float64(booked) / float64(avail) * 100)
	}
	return rep, nil
}

type rptSpan struct{ s, e int } // minutes from midnight

func rptHM(v string) (int, bool) {
	t, err := time.Parse("15:04", v)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

func rptUnion(spans []rptSpan) []rptSpan {
	sort.Slice(spans, func(i, j int) bool { return spans[i].s < spans[j].s })
	var out []rptSpan
	for _, x := range spans {
		if n := len(out); n > 0 && x.s <= out[n-1].e {
			if x.e > out[n-1].e {
				out[n-1].e = x.e
			}
			continue
		}
		out = append(out, x)
	}
	return out
}

func rptMinutes(spans []rptSpan) int {
	t := 0
	for _, x := range spans {
		t += x.e - x.s
	}
	return t
}

// rptSubtract removes blocked spans from open spans.
func rptSubtract(open, blocked []rptSpan) []rptSpan {
	var out []rptSpan
	blocked = rptUnion(blocked)
	for _, o := range rptUnion(open) {
		cur := o.s
		for _, b := range blocked {
			if b.e <= cur || b.s >= o.e {
				continue
			}
			if b.s > cur {
				out = append(out, rptSpan{cur, b.s})
			}
			if b.e > cur {
				cur = b.e
			}
		}
		if cur < o.e {
			out = append(out, rptSpan{cur, o.e})
		}
	}
	return out
}

// rptOccupancy compares booked minutes with the minutes professionals are open, day by day.
// Professionals counted: active doctors (or the filtered one) plus anyone with appointments in the range.
func (s *Server) rptOccupancy(ctx context.Context, clinicID string, win rptWindow, pro string) (int, int, error) {
	var booked int
	if err := s.db.QueryRow(ctx, `
		SELECT coalesce(sum(extract(epoch FROM a.end_hour - a.start_hour) / 60), 0)::int
		FROM appointments a WHERE a.clinic_id = $1 AND a.date >= $2::date AND a.date <= $3::date
		  AND a.status NOT IN ('cancelled', 'no_show') AND ($4 = '' OR a.professional_id::text = $4)`,
		clinicID, win.FromDay, win.ToDay, pro).Scan(&booked); err != nil {
		return 0, 0, err
	}
	var rawSettings []byte
	if err := s.db.QueryRow(ctx, `SELECT coalesce(settings, '{}'::jsonb)::text FROM clinics WHERE id = $1`, clinicID).Scan(&rawSettings); err != nil {
		return 0, 0, err
	}
	clinicHours := rptClinicHours(rawSettings)

	type proInfo struct {
		id    string
		hours map[string][][2]string
	}
	rows, err := s.db.Query(ctx, `
		SELECT u.id::text, coalesce(ps.hours, '{}'::jsonb)::text
		FROM users u LEFT JOIN professional_settings ps ON ps.user_id = u.id
		WHERE u.clinic_id = $1 AND NOT u.disabled AND ($2 = '' OR u.id::text = $2)
		  AND (u.role = 'doctor' OR u.id IN (SELECT professional_id FROM appointments a
		       WHERE a.clinic_id = $1 AND a.date >= $3::date AND a.date <= $4::date AND professional_id IS NOT NULL))`,
		clinicID, pro, win.FromDay, win.ToDay)
	if err != nil {
		return 0, 0, err
	}
	var pros []proInfo
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return 0, 0, err
		}
		pros = append(pros, proInfo{id: id, hours: rptProHours(raw)})
	}
	rows.Close()

	type block struct {
		pro      string
		from, to time.Time
		span     *rptSpan
	}
	brows, err := s.db.Query(ctx, `
		SELECT coalesce(professional_id::text, ''), date_from, date_to, to_char(start_hour, 'HH24:MI'), to_char(end_hour, 'HH24:MI')
		FROM time_blocks WHERE clinic_id = $1 AND date_to >= $2::date AND date_from <= $3::date`, clinicID, win.FromDay, win.ToDay)
	if err != nil {
		return 0, 0, err
	}
	var blocks []block
	for brows.Next() {
		var b block
		var sh, eh *string
		if err := brows.Scan(&b.pro, &b.from, &b.to, &sh, &eh); err != nil {
			brows.Close()
			return 0, 0, err
		}
		if sh != nil && eh != nil {
			a, ok1 := rptHM(*sh)
			z, ok2 := rptHM(*eh)
			if ok1 && ok2 {
				b.span = &rptSpan{a, z}
			}
		}
		blocks = append(blocks, b)
	}
	brows.Close()

	keys := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
	avail := 0
	for d := win.From; d.Before(win.To); d = d.AddDate(0, 0, 1) {
		key := keys[d.Weekday()]
		day := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
		for _, pr := range pros {
			var open []rptSpan
			if len(pr.hours) > 0 {
				for _, iv := range pr.hours[key] {
					a, ok1 := rptHM(iv[0])
					z, ok2 := rptHM(iv[1])
					if ok1 && ok2 && z > a {
						open = append(open, rptSpan{a, z})
					}
				}
			} else if h, ok := clinicHours[key]; ok && h.Open {
				a, ok1 := rptHM(h.Start)
				z, ok2 := rptHM(h.End)
				if ok1 && ok2 && z > a {
					open = append(open, rptSpan{a, z})
				}
			}
			if len(open) == 0 {
				continue
			}
			var blocked []rptSpan
			for _, b := range blocks {
				if (b.pro != "" && b.pro != pr.id) || day.Before(b.from) || day.After(b.to) {
					continue
				}
				if b.span == nil {
					blocked = append(blocked, rptSpan{0, 24 * 60})
				} else {
					blocked = append(blocked, *b.span)
				}
			}
			avail += rptMinutes(rptSubtract(open, blocked))
		}
	}
	return booked, avail, nil
}

func (s *Server) rptOperationsCSV(w http.ResponseWriter, r *http.Request) {
	p, win, pro, scope, ok := s.rptOperationsParams(w, r)
	if !ok {
		return
	}
	rep, err := s.rptBuildOperations(r.Context(), p.ClinicID, win, pro, scope)
	if err != nil {
		serverError(w, r, err)
		return
	}
	cw := rptCSVStart(w, "operacion-"+win.FromDay+"-"+win.ToDay)
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
	opt := func(v *float64) string {
		if v == nil {
			return ""
		}
		return f(*v)
	}
	_ = cw.Write([]string{"Reporte operativo", win.FromDay, win.ToDay})
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Consultas por periodo (" + rep.Granularity + ")", "Total"})
	for _, c := range rep.Encounters.ByPeriod {
		_ = cw.Write([]string{c.Key, strconv.Itoa(c.Count)})
	}
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Profesional", "Consultas", "Citas", "No asistió", "Atendidas", "Min. de atención", "Min. de espera"})
	for _, x := range rep.Professionals {
		_ = cw.Write([]string{csvSafe(x.Name), strconv.Itoa(x.Encounters), strconv.Itoa(x.Appointments), strconv.Itoa(x.NoShow), strconv.Itoa(x.Completed), opt(x.AvgAttention), opt(x.AvgWait)})
	}
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Citas por estado", "Total"})
	for _, c := range rep.Appointments.ByStatus {
		_ = cw.Write([]string{c.Label, strconv.Itoa(c.Count)})
	}
	_ = cw.Write([]string{"Tasa de cancelación %", f(rep.Appointments.CancellationRate)})
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Origen de las citas", "Total"})
	for _, c := range rep.Appointments.BySource {
		_ = cw.Write([]string{c.Label, strconv.Itoa(c.Count)})
	}
	for _, sec := range []struct {
		title string
		rows  []rptRate
	}{{"Ausentismo por profesional", rep.NoShow.ByProfessional}, {"Ausentismo por servicio", rep.NoShow.ByService},
		{"Ausentismo por día de la semana", rep.NoShow.ByWeekday}, {"Ausentismo por hora", rep.NoShow.ByHour}} {
		_ = cw.Write([]string{})
		_ = cw.Write([]string{sec.title, "Citas", "No asistió", "%"})
		for _, x := range sec.rows {
			_ = cw.Write([]string{csvSafe(x.Label), strconv.Itoa(x.Total), strconv.Itoa(x.NoShow), f(x.Rate)})
		}
	}
	_ = cw.Write([]string{"Ausentismo global %", f(rep.NoShow.Rate)})
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Tiempo promedio de atención (min)", opt(rep.Times.AvgAttentionMin)})
	_ = cw.Write([]string{"Tiempo promedio de espera (min)", opt(rep.Times.AvgWaitMin)})
	_ = cw.Write([]string{"Minutos agendados", strconv.Itoa(rep.Occupancy.BookedMin)})
	_ = cw.Write([]string{"Minutos disponibles (aprox.)", strconv.Itoa(rep.Occupancy.AvailableMin)})
	_ = cw.Write([]string{"Ocupación %", opt(rep.Occupancy.Rate)})
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Horas pico", "Citas"})
	for _, c := range rep.PeakHours.ByHour {
		_ = cw.Write([]string{c.Label, strconv.Itoa(c.Count)})
	}
	cw.Flush()
}

// rptCSVStart writes CSV headers (with the BOM Excel needs for UTF-8) and returns the writer.
func rptCSVStart(w http.ResponseWriter, name string) *csv.Writer {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.csv"`)
	_, _ = w.Write([]byte("\xEF\xBB\xBF"))
	return csv.NewWriter(w)
}
