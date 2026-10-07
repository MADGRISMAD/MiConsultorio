package api

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// Consolidated reports for the organization owner. Every query filters by the clinic ids read from the
// organization on this request (orgClinicIDs): no id ever comes from the client.

type orgSales struct {
	Enabled        bool `json:"enabled"` // false when the plan has no cobros: the branch shows no sales
	Count          int  `json:"count"`
	TotalCents     int  `json:"total_cents"`
	AvgTicketCents int  `json:"avg_ticket_cents"`
	VoidCount      int  `json:"void_count"`
}

type orgAppointments struct {
	Total            int            `json:"total"`
	ByStatus         map[string]int `json:"by_status"`
	Eligible         int            `json:"eligible"` // past appointments that were not cancelled
	NoShow           int            `json:"no_show"`
	NoShowRate       float64        `json:"no_show_rate"` // percent of eligible
	CancellationRate float64        `json:"cancellation_rate"`
}

type orgBranchReport struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	IsMatrix      bool            `json:"is_matrix"`
	Suspended     bool            `json:"suspended"`
	Sales         orgSales        `json:"sales"`
	Appointments  orgAppointments `json:"appointments"`
	Consultations int             `json:"consultations"`
	NewPatients   int             `json:"new_patients"`
}

type orgDay struct {
	Day           string `json:"day"`
	SalesCents    int    `json:"sales_cents"`
	Appointments  int    `json:"appointments"`
	Consultations int    `json:"consultations"`
	NewPatients   int    `json:"new_patients"`
}

type orgSummary struct {
	Organization string            `json:"organization"`
	From         string            `json:"from"`
	To           string            `json:"to"`
	Branches     []orgBranchReport `json:"branches"`
	Total        orgBranchReport   `json:"total"`
	Daily        []orgDay          `json:"daily"`
}

func orgFinish(a *orgAppointments, s *orgSales) {
	if s.Count > 0 {
		s.AvgTicketCents = s.TotalCents / s.Count
	}
	a.NoShowRate, a.CancellationRate = rptPct(a.NoShow, a.Eligible), rptPct(a.ByStatus["cancelled"], a.Total)
}

func (s *Server) orgBuildSummary(ctx context.Context, org *orgInfo, currentClinic string, win rptWindow) (*orgSummary, error) {
	ids, err := orgClinicIDs(ctx, s.db, org.ID)
	if err != nil {
		return nil, err
	}
	branches, err := loadBranches(ctx, s.db, org, currentClinic)
	if err != nil {
		return nil, err
	}
	var planID string
	if err := s.db.QueryRow(ctx, `SELECT plan FROM clinics WHERE id = $1`, org.MatrixID).Scan(&planID); err != nil {
		return nil, err
	}
	pl, _ := planByID(planID)

	rep := &orgSummary{Organization: org.Name, From: win.FromDay, To: win.ToDay, Branches: []orgBranchReport{}, Daily: []orgDay{}}
	idx := map[string]*orgBranchReport{}
	for _, b := range branches {
		rep.Branches = append(rep.Branches, orgBranchReport{ID: b.ID, Name: b.Name, IsMatrix: b.IsMatrix, Suspended: b.Suspended,
			Sales: orgSales{Enabled: pl.Cobros}, Appointments: orgAppointments{ByStatus: map[string]int{}}})
	}
	for i := range rep.Branches {
		idx[rep.Branches[i].ID] = &rep.Branches[i]
	}
	rep.Total = orgBranchReport{Name: "Total", Sales: orgSales{Enabled: pl.Cobros}, Appointments: orgAppointments{ByStatus: map[string]int{}}}

	// sales
	if pl.Cobros {
		rows, err := s.db.Query(ctx, `
			SELECT clinic_id::text, count(*) FILTER (WHERE status = 'paid'), coalesce(sum(total_cents) FILTER (WHERE status = 'paid'), 0), count(*) FILTER (WHERE status = 'void')
			FROM sales WHERE clinic_id = ANY($1::uuid[]) AND created_at >= $2 AND created_at < $3 GROUP BY clinic_id`, ids, win.From, win.To)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var n, total, voids int
			if err := rows.Scan(&id, &n, &total, &voids); err != nil {
				rows.Close()
				return nil, err
			}
			if b := idx[id]; b != nil {
				b.Sales.Count, b.Sales.TotalCents, b.Sales.VoidCount = n, total, voids
			}
		}
		rows.Close()
		if rows.Err() != nil {
			return nil, rows.Err()
		}
	}
	// appointments by status
	rows, err := s.db.Query(ctx, `
		SELECT clinic_id::text, status, count(*), count(*) FILTER (WHERE status <> 'cancelled' AND date < current_date)
		FROM appointments WHERE clinic_id = ANY($1::uuid[]) AND date >= $2::date AND date <= $3::date GROUP BY clinic_id, status`, ids, win.FromDay, win.ToDay)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, status string
		var n, eligible int
		if err := rows.Scan(&id, &status, &n, &eligible); err != nil {
			rows.Close()
			return nil, err
		}
		if b := idx[id]; b != nil {
			b.Appointments.ByStatus[status] += n
			b.Appointments.Total += n
			b.Appointments.Eligible += eligible
			if status == "no_show" {
				b.Appointments.NoShow += n
			}
		}
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	// consultations and new patients
	for _, q := range []struct {
		sql string
		set func(b *orgBranchReport, n int)
	}{
		{`SELECT clinic_id::text, count(*) FROM encounters WHERE clinic_id = ANY($1::uuid[]) AND kind = 'consulta' AND occurred_at >= $2 AND occurred_at < $3 GROUP BY clinic_id`,
			func(b *orgBranchReport, n int) { b.Consultations = n }},
		{`SELECT clinic_id::text, count(*) FROM patients WHERE clinic_id = ANY($1::uuid[]) AND created_at >= $2 AND created_at < $3 GROUP BY clinic_id`,
			func(b *orgBranchReport, n int) { b.NewPatients = n }},
	} {
		rows, err := s.db.Query(ctx, q.sql, ids, win.From, win.To)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var n int
			if err := rows.Scan(&id, &n); err != nil {
				rows.Close()
				return nil, err
			}
			if b := idx[id]; b != nil {
				q.set(b, n)
			}
		}
		rows.Close()
		if rows.Err() != nil {
			return nil, rows.Err()
		}
	}

	// totals
	t := &rep.Total
	for i := range rep.Branches {
		b := &rep.Branches[i]
		orgFinish(&b.Appointments, &b.Sales)
		t.Sales.Count += b.Sales.Count
		t.Sales.TotalCents += b.Sales.TotalCents
		t.Sales.VoidCount += b.Sales.VoidCount
		t.Appointments.Total += b.Appointments.Total
		t.Appointments.Eligible += b.Appointments.Eligible
		t.Appointments.NoShow += b.Appointments.NoShow
		for k, v := range b.Appointments.ByStatus {
			t.Appointments.ByStatus[k] += v
		}
		t.Consultations += b.Consultations
		t.NewPatients += b.NewPatients
	}
	orgFinish(&t.Appointments, &t.Sales)

	// daily series for the whole organization
	days := map[string]*orgDay{}
	for d := win.From; d.Before(win.To); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		days[key] = &orgDay{Day: key}
	}
	add := func(sql string, args []any, set func(d *orgDay, n int)) error {
		rows, err := s.db.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var day time.Time
			var n int
			if err := rows.Scan(&day, &n); err != nil {
				return err
			}
			if d := days[day.Format("2006-01-02")]; d != nil {
				set(d, n)
			}
		}
		return rows.Err()
	}
	if pl.Cobros {
		if err := add(`SELECT (created_at AT TIME ZONE $4)::date, coalesce(sum(total_cents), 0)::int FROM sales WHERE clinic_id = ANY($1::uuid[]) AND status = 'paid' AND created_at >= $2 AND created_at < $3 GROUP BY 1`,
			[]any{ids, win.From, win.To, win.TZ}, func(d *orgDay, n int) { d.SalesCents = n }); err != nil {
			return nil, err
		}
	}
	if err := add(`SELECT date, count(*)::int FROM appointments WHERE clinic_id = ANY($1::uuid[]) AND date >= $2::date AND date <= $3::date GROUP BY 1`,
		[]any{ids, win.FromDay, win.ToDay}, func(d *orgDay, n int) { d.Appointments = n }); err != nil {
		return nil, err
	}
	if err := add(`SELECT (occurred_at AT TIME ZONE $4)::date, count(*)::int FROM encounters WHERE clinic_id = ANY($1::uuid[]) AND kind = 'consulta' AND occurred_at >= $2 AND occurred_at < $3 GROUP BY 1`,
		[]any{ids, win.From, win.To, win.TZ}, func(d *orgDay, n int) { d.Consultations = n }); err != nil {
		return nil, err
	}
	if err := add(`SELECT (created_at AT TIME ZONE $4)::date, count(*)::int FROM patients WHERE clinic_id = ANY($1::uuid[]) AND created_at >= $2 AND created_at < $3 GROUP BY 1`,
		[]any{ids, win.From, win.To, win.TZ}, func(d *orgDay, n int) { d.NewPatients = n }); err != nil {
		return nil, err
	}
	for d := win.From; d.Before(win.To); d = d.AddDate(0, 0, 1) {
		rep.Daily = append(rep.Daily, *days[d.Format("2006-01-02")])
	}
	return rep, nil
}

func (s *Server) orgReportParams(w http.ResponseWriter, r *http.Request) (*Principal, *orgInfo, rptWindow, bool) {
	p, org, ok := s.orgOwnerOnly(w, r)
	if !ok {
		return nil, nil, rptWindow{}, false
	}
	win, msg := rptParseWindow(r.URL.Query().Get)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return nil, nil, rptWindow{}, false
	}
	return p, org, win, true
}

func (s *Server) orgReportSummary(w http.ResponseWriter, r *http.Request) {
	p, org, win, ok := s.orgReportParams(w, r)
	if !ok {
		return
	}
	rep, err := s.orgBuildSummary(r.Context(), org, p.ClinicID, win)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (s *Server) orgReportCSV(w http.ResponseWriter, r *http.Request) {
	p, org, win, ok := s.orgReportParams(w, r)
	if !ok {
		return
	}
	rep, err := s.orgBuildSummary(r.Context(), org, p.ClinicID, win)
	if err != nil {
		serverError(w, r, err)
		return
	}
	cw := rptCSVStart(w, "organizacion-"+win.FromDay+"-"+win.ToDay)
	_ = cw.Write([]string{"Reporte consolidado", csvSafe(rep.Organization), win.FromDay, win.ToDay})
	_ = cw.Write([]string{})
	_ = cw.Write([]string{"Sucursal", "Ventas (MXN)", "Cobros", "Ticket promedio (MXN)", "Citas", "Atendidas", "No asistió", "Ausentismo %", "Canceladas", "Consultas", "Pacientes nuevos", "Estado"})
	money := func(c int) string { return strconv.FormatFloat(float64(c)/100, 'f', 2, 64) }
	row := func(b orgBranchReport) {
		sales, count, avg := "", "", ""
		if b.Sales.Enabled {
			sales, count, avg = money(b.Sales.TotalCents), strconv.Itoa(b.Sales.Count), money(b.Sales.AvgTicketCents)
		}
		state := "En servicio"
		if b.Suspended {
			state = "De baja"
		}
		if b.ID == "" {
			state = ""
		}
		_ = cw.Write([]string{csvSafe(b.Name), sales, count, avg, strconv.Itoa(b.Appointments.Total), strconv.Itoa(b.Appointments.ByStatus["completed"]),
			strconv.Itoa(b.Appointments.NoShow), strconv.FormatFloat(b.Appointments.NoShowRate, 'f', 1, 64), strconv.Itoa(b.Appointments.ByStatus["cancelled"]),
			strconv.Itoa(b.Consultations), strconv.Itoa(b.NewPatients), state})
	}
	for _, b := range rep.Branches {
		row(b)
	}
	row(rep.Total)
	cw.Flush()
}
