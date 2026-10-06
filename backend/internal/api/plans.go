package api

import "time"

const (
	TrialDays        = 14
	pastDueGraceDays = 3 // an expired paid period still works this long
)

// Plan limits. nil means unlimited. They mirror the landing page:
// Básico = 1 professional + 1 front desk (agenda and records); Crecimiento = up to 5 professionals
// and collections (cobros); Pro = no limits.
type Plan struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	PriceMonth  int    `json:"price_month"` // MXN, 0 = custom
	MaxUsers    *int   `json:"max_users"`
	MaxDoctors  *int   `json:"max_doctors"`
	Description string `json:"description"`
	// Cobros is true when the plan includes the collections section (point of sale, cash register, ...).
	Cobros bool `json:"cobros"`
}

func ptr(n int) *int { return &n }

var planCatalog = []Plan{
	{ID: "basico", Name: "Básico", PriceMonth: 499, MaxUsers: ptr(3), MaxDoctors: ptr(1), Description: "Agenda, expedientes y equipo: 1 profesional, 1 recepción y el administrador", Cobros: false},
	{ID: "crecimiento", Name: "Crecimiento", PriceMonth: 1199, MaxUsers: nil, MaxDoctors: ptr(5), Description: "Hasta 5 profesionales, recepción ilimitada y sección de cobros", Cobros: true},
	{ID: "pro", Name: "Pro", PriceMonth: 0, MaxUsers: nil, MaxDoctors: nil, Description: "Sin límites, cobros incluidos, a medida", Cobros: true},
}

func planByID(id string) (Plan, bool) {
	for _, p := range planCatalog {
		if p.ID == id {
			return p, true
		}
	}
	return Plan{}, false
}

// Billing is the subscription state of a clinic.
type Billing struct {
	Plan             string
	Status           string // trialing | active | past_due | suspended (as stored)
	TrialEndsAt      *time.Time
	CurrentPeriodEnd *time.Time
	SuspendedReason  string
}

// State is the effective state, which also accounts for dates passing:
// trialing, trial_expired, active, past_due or suspended.
func (b Billing) State(now time.Time) string {
	switch b.Status {
	case "suspended":
		return "suspended"
	case "past_due":
		return "past_due"
	case "trialing":
		if b.TrialEndsAt != nil && b.TrialEndsAt.After(now) {
			return "trialing"
		}
		return "trial_expired"
	default: // active
		if b.CurrentPeriodEnd != nil && b.CurrentPeriodEnd.Add(pastDueGraceDays*24*time.Hour).Before(now) {
			return "past_due"
		}
		return "active"
	}
}

// Usable reports whether the clinic may use the product right now.
func (b Billing) Usable(now time.Time) bool {
	s := b.State(now)
	return s == "trialing" || s == "active"
}

// TrialDaysLeft is whole days remaining in the trial (0 once over), or nil outside a trial.
func (b Billing) TrialDaysLeft(now time.Time) *int {
	if b.Status != "trialing" || b.TrialEndsAt == nil {
		return nil
	}
	d := int(b.TrialEndsAt.Sub(now).Hours()/24 + 0.999)
	if d < 0 {
		d = 0
	}
	return &d
}

// billingInfo is the subscription summary sent to the frontend.
type billingInfo struct {
	Plan             string     `json:"plan"`
	PlanName         string     `json:"plan_name"`
	State            string     `json:"state"`
	Usable           bool       `json:"usable"`
	TrialEndsAt      *time.Time `json:"trial_ends_at"`
	TrialDaysLeft    *int       `json:"trial_days_left"`
	CurrentPeriodEnd *time.Time `json:"current_period_end"`
	SuspendedReason  string     `json:"suspended_reason"`
	Cobros           bool       `json:"cobros"`
}

func (b Billing) info(now time.Time) billingInfo {
	p, _ := planByID(b.Plan)
	return billingInfo{
		Plan: b.Plan, PlanName: p.Name, State: b.State(now), Usable: b.Usable(now),
		TrialEndsAt: b.TrialEndsAt, TrialDaysLeft: b.TrialDaysLeft(now),
		CurrentPeriodEnd: b.CurrentPeriodEnd, SuspendedReason: b.SuspendedReason, Cobros: p.Cobros,
	}
}
