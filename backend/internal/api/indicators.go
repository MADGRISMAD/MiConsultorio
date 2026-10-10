package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Indicators panel: the numbers an owner looks at first, for a range and for the range right before it (same length),
// so each one comes with its change. It reuses the operations report for the agenda numbers and adds patients, income
// (only when the plan includes cobros) and the satisfaction survey.

type indPeriod struct {
	Encounters       int      `json:"encounters"`
	Appointments     int      `json:"appointments"`
	Completed        int      `json:"completed"`
	CancellationRate float64  `json:"cancellation_rate"`
	NoShowRate       float64  `json:"no_show_rate"`
	AvgWaitMin       *float64 `json:"avg_wait_min"`
	AvgAttentionMin  *float64 `json:"avg_attention_min"`
	OccupancyRate    *float64 `json:"occupancy_rate"`
	NewPatients      int      `json:"new_patients"`
	ReturningPatient int      `json:"returning_patients"`
	RevenueCents     int      `json:"revenue_cents"`
	Sales            int      `json:"sales"`
	AvgTicketCents   int      `json:"avg_ticket_cents"`
	SatisfactionAvg  float64  `json:"satisfaction_avg"`
	SatisfactionN    int      `json:"satisfaction_responses"`
}

func (s *Server) indicatorPeriod(ctx context.Context, clinicID string, win rptWindow, cobros bool) (indPeriod, *rptOperationsReport, error) {
	var out indPeriod
	rep, err := s.rptBuildOperations(ctx, clinicID, win, "", "clinic")
	if err != nil {
		return out, nil, err
	}
	out.Encounters = rep.Encounters.Total
	out.Appointments = rep.Appointments.Total
	for _, c := range rep.Appointments.ByStatus {
		if c.Key == "completed" {
			out.Completed = c.Count
		}
	}
	out.CancellationRate = rep.Appointments.CancellationRate
	out.NoShowRate = rep.NoShow.Rate
	out.AvgWaitMin, out.AvgAttentionMin, out.OccupancyRate = rep.Times.AvgWaitMin, rep.Times.AvgAttentionMin, rep.Occupancy.Rate

	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM patients WHERE clinic_id = $1 AND created_at >= $2 AND created_at < $3`, clinicID, win.From, win.To).Scan(&out.NewPatients); err != nil {
		return out, nil, err
	}
	// returning: seen in the range and already seen before it
	if err := s.db.QueryRow(ctx, `
		SELECT count(DISTINCT e.patient_id) FROM encounters e
		WHERE e.clinic_id = $1 AND e.occurred_at >= $2 AND e.occurred_at < $3 AND e.addendum_of IS NULL
		  AND EXISTS (SELECT 1 FROM encounters o WHERE o.clinic_id = $1 AND o.patient_id = e.patient_id AND o.occurred_at < $2 AND o.addendum_of IS NULL)`,
		clinicID, win.From, win.To).Scan(&out.ReturningPatient); err != nil {
		return out, nil, err
	}
	if cobros {
		if err := s.db.QueryRow(ctx, `SELECT count(*), coalesce(sum(total_cents), 0)::int FROM sales WHERE clinic_id = $1 AND status = 'paid' AND created_at >= $2 AND created_at < $3`,
			clinicID, win.From, win.To).Scan(&out.Sales, &out.RevenueCents); err != nil {
			return out, nil, err
		}
		if out.Sales > 0 {
			out.AvgTicketCents = out.RevenueCents / out.Sales
		}
	}
	st, err := s.surveyStatsFor(ctx, clinicID, &win.From, &win.To)
	if err != nil {
		return out, nil, err
	}
	out.SatisfactionAvg, out.SatisfactionN = st.Average, st.Count
	return out, rep, nil
}

func (s *Server) indicators(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	win, msg := rptParseWindow(r.URL.Query().Get)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	cobros := false
	if p.Billing != nil {
		if plan, ok := planByID(p.Billing.Plan); ok {
			cobros = plan.Cobros
		}
	}
	cur, rep, err := s.indicatorPeriod(r.Context(), p.ClinicID, win, cobros)
	if err != nil {
		serverError(w, r, err)
		return
	}
	prevFrom := win.From.AddDate(0, 0, -win.Days)
	prev := rptWindow{From: prevFrom, To: win.From, FromDay: prevFrom.Format("2006-01-02"), ToDay: win.From.AddDate(0, 0, -1).Format("2006-01-02"), Days: win.Days, TZ: win.TZ}
	before, _, err := s.indicatorPeriod(r.Context(), p.ClinicID, prev, cobros)
	if err != nil {
		serverError(w, r, err)
		return
	}
	pros := rep.Professionals
	if pros == nil {
		pros = []rptProfessional{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from": win.FromDay, "to": win.ToDay, "prev_from": prev.FromDay, "prev_to": prev.ToDay, "days": win.Days,
		"current": cur, "previous": before, "cobros": cobros,
		"trend": rep.Encounters.ByPeriod, "granularity": rep.Granularity, "professionals": pros,
		"generated_at": time.Now().UTC(),
	})
}

func (s *Server) mountIndicators(r chi.Router) {
	r.With(require(PermAdminUsers)).Get("/indicators", s.indicators)
}
