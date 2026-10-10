package api

import (
	"context"
	"net/http"
	"time"
)

const (
	TrialDays        = 14
	pastDueGraceDays = 3 // an expired paid period still works this long
)

// Plan limits. nil means unlimited. They mirror the landing page:
// Básico = 2 specialists, 1 front desk, 1 cash account, no AI (agenda and records); Crecimiento = 5 specialists,
// 2 front desk, 2 cash accounts, collections (cobros) and AI; Pro = unlimited accounts for now, 500 AI uses.
type Plan struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceMonth int    `json:"price_month"` // MXN, 0 = custom
	// Seats per kind of account (nil: no limit). Administrators are not counted.
	MaxDoctors   *int   `json:"max_doctors"`   // specialists
	MaxReception *int   `json:"max_reception"` // front desk
	MaxCashiers  *int   `json:"max_cashiers"`  // cash register
	Description  string `json:"description"`
	// Cobros is true when the plan includes the collections section (point of sale, cash register, ...).
	Cobros bool `json:"cobros"`
	// MagicUses is the monthly allowance of AI actions (magic inventory and pricing).
	MagicUses int `json:"magic_uses"`
	// MaxBranches is how many active branches (clinics, the matrix included) one organization may run under the plan.
	MaxBranches int `json:"max_branches"`
	// MaxKinds is how many giros (main one plus extra specialties) a clinic may work in; nil is all of them.
	MaxKinds *int `json:"max_kinds"`
	// StorageGB is the space for attachments (studies, photos, documents).
	StorageGB int `json:"storage_gb"`
	// WhatsApp: appointment reminders by WhatsApp.
	WhatsApp bool `json:"whatsapp"`
	// Permissions: allow or deny single permissions per person, on top of the role.
	Permissions bool `json:"permissions"`
	// Support is the level of attention: "correo", "prioritario" or "dedicado".
	Support string `json:"support"`
}

// StorageBytes is the attachment space of the plan.
func (p Plan) StorageBytes() int64 { return int64(p.StorageGB) << 30 }

func ptr(n int) *int { return &n }

var planCatalog = []Plan{
	{ID: "basico", Name: "Básico", PriceMonth: 499, MaxDoctors: ptr(2), MaxReception: ptr(1), MaxCashiers: ptr(1), Description: "Agenda y expedientes: 2 especialistas, 1 recepción y 1 caja, sin asistente de IA", Cobros: false, MagicUses: 0, MaxBranches: 1,
		MaxKinds: ptr(1), StorageGB: 2, WhatsApp: false, Permissions: false, Support: "correo"},
	{ID: "crecimiento", Name: "Crecimiento", PriceMonth: 1199, MaxDoctors: ptr(5), MaxReception: ptr(2), MaxCashiers: ptr(2), Description: "5 especialistas, 2 recepciones, 2 cajas, cobros y asistente de IA", Cobros: true, MagicUses: 250, MaxBranches: 3,
		MaxKinds: ptr(3), StorageGB: 20, WhatsApp: true, Permissions: true, Support: "prioritario"},
	{ID: "pro", Name: "Pro", PriceMonth: 0, MaxDoctors: nil, MaxReception: nil, MaxCashiers: nil, Description: "Cuentas sin límite por ahora, cobros incluidos y 500 usos de IA, a medida", Cobros: true, MagicUses: 500, MaxBranches: 10,
		MaxKinds: nil, StorageGB: 100, WhatsApp: true, Permissions: true, Support: "dedicado"},
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
	MagicUses        int        `json:"magic_uses"`
}

func (b Billing) info(now time.Time) billingInfo {
	p, _ := planByID(b.Plan)
	return billingInfo{
		Plan: b.Plan, PlanName: p.Name, State: b.State(now), Usable: b.Usable(now),
		TrialEndsAt: b.TrialEndsAt, TrialDaysLeft: b.TrialDaysLeft(now),
		CurrentPeriodEnd: b.CurrentPeriodEnd, SuspendedReason: b.SuspendedReason, Cobros: p.Cobros, MagicUses: p.MagicUses,
	}
}

// kindsError explains that a clinic works in more giros than a plan allows (empty when it fits).
func (p Plan) kindsError(n int) string {
	if p.MaxKinds != nil && n > *p.MaxKinds {
		return "El plan " + p.Name + " permite " + itoa(*p.MaxKinds) + " giro" + map[bool]string{true: "", false: "s"}[*p.MaxKinds == 1] + " y tu consultorio trabaja en " + itoa(n) + "."
	}
	return ""
}

func planRequired(msg string) *httpError {
	e := fail(http.StatusForbidden, msg)
	e.Code = "PLAN_REQUIRED"
	return e
}

// planOf returns the plan of a clinic (the smallest one if it cannot be read).
func planOf(ctx context.Context, q queryRower, clinicID string) Plan {
	var id string
	_ = q.QueryRow(ctx, `SELECT plan FROM clinics WHERE id = $1`, clinicID).Scan(&id)
	if pl, ok := planByID(id); ok {
		return pl
	}
	return planCatalog[0]
}
