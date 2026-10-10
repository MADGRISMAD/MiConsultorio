package api

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

// The platform panel: the people who run Caresia itself. Support can look at everything;
// only platform administrators change plans, subscriptions, payments and staff.

// ---------------------------------------------------------------------------
// Clinics
// ---------------------------------------------------------------------------

type clinicRow struct {
	ID               string     `json:"id" db:"id"`
	Name             string     `json:"name" db:"name"`
	Kind             string     `json:"kind" db:"kind"`
	Email            string     `json:"email" db:"email"`
	PhoneNumber      string     `json:"phone_number" db:"phone_number"`
	Address          string     `json:"address" db:"address"`
	Plan             string     `json:"plan" db:"plan"`
	BillingStatus    string     `json:"billing_status" db:"billing_status"`
	TrialEndsAt      *time.Time `json:"trial_ends_at" db:"trial_ends_at"`
	CurrentPeriodEnd *time.Time `json:"current_period_end" db:"current_period_end"`
	SuspendedReason  string     `json:"suspended_reason" db:"suspended_reason"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	Users            int        `json:"users" db:"users"`
	OwnerName        string     `json:"owner_name" db:"owner_name"`
	OwnerEmail       string     `json:"owner_email" db:"owner_email"`
	LastSeen         *time.Time `json:"last_seen" db:"last_seen"`

	PlanName      string `json:"plan_name" db:"-"`
	State         string `json:"state" db:"-"` // trialing | trial_expired | active | past_due | suspended
	TrialDaysLeft *int   `json:"trial_days_left" db:"-"`
}

const clinicCols = `c.id, c.name, c.kind, c.email, c.phone_number, c.address, c.plan, c.billing_status, c.trial_ends_at,
	c.current_period_end, c.suspended_reason, c.created_at,
	(SELECT count(*) FROM users u WHERE u.clinic_id = c.id AND NOT u.disabled) AS users,
	coalesce((SELECT u.name FROM users u WHERE u.clinic_id = c.id AND u.role = 'admin' ORDER BY u.created_at LIMIT 1), '') AS owner_name,
	coalesce((SELECT u.email FROM users u WHERE u.clinic_id = c.id AND u.role = 'admin' ORDER BY u.created_at LIMIT 1), '') AS owner_email,
	(SELECT max(u.last_login_at) FROM users u WHERE u.clinic_id = c.id) AS last_seen`

func (c *clinicRow) billing() Billing {
	return Billing{Plan: c.Plan, Status: c.BillingStatus, TrialEndsAt: c.TrialEndsAt, CurrentPeriodEnd: c.CurrentPeriodEnd, SuspendedReason: c.SuspendedReason}
}

func (c *clinicRow) finish(now time.Time) {
	b := c.billing()
	plan, _ := planByID(c.Plan)
	c.PlanName, c.State, c.TrialDaysLeft = plan.Name, b.State(now), b.TrialDaysLeft(now)
}

func loadClinics(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, where string, args ...any) ([]clinicRow, error) {
	rows, err := q.Query(ctx, `SELECT `+clinicCols+` FROM clinics c `+where+` ORDER BY c.created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[clinicRow])
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for i := range list {
		list[i].finish(now)
	}
	return list, nil
}

func (s *Server) platformPlans(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"plans": planCatalog})
}

func (s *Server) platformClinics(w http.ResponseWriter, r *http.Request) {
	list, err := loadClinics(r.Context(), s.db, "")
	if err != nil {
		serverError(w, r, err)
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	state := r.URL.Query().Get("state")
	out := list[:0]
	for _, c := range list {
		if state != "" && state != "all" && c.State != state {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(c.Name+" "+c.OwnerName+" "+c.OwnerEmail+" "+c.Email), q) {
			continue
		}
		out = append(out, c)
	}
	counts := map[string]int{"all": len(list)}
	for _, c := range list {
		counts[c.State]++
	}
	writeJSON(w, http.StatusOK, map[string]any{"clinics": out, "counts": counts})
}

type activityItem struct {
	ID         int64           `json:"id"`
	ClinicID   *string         `json:"clinic_id"`
	ClinicName string          `json:"clinic_name"`
	ActorName  string          `json:"actor_name"`
	Type       string          `json:"type"`
	Message    string          `json:"message"`
	Meta       json.RawMessage `json:"meta"`
	CreatedAt  time.Time       `json:"created_at"`
}

func loadActivity(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, clinicID string, limit int) ([]activityItem, error) {
	where, args := "", []any{limit}
	if clinicID != "" {
		where, args = "WHERE a.clinic_id = $2", []any{limit, clinicID}
	}
	rows, err := q.Query(ctx, `
		SELECT a.id, a.clinic_id, coalesce(c.name, ''), a.actor_name, a.type, a.message, a.meta, a.created_at
		FROM activity_log a LEFT JOIN clinics c ON c.id = a.clinic_id `+where+`
		ORDER BY a.created_at DESC, a.id DESC LIMIT $1`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []activityItem{}
	for rows.Next() {
		var it activityItem
		if err := rows.Scan(&it.ID, &it.ClinicID, &it.ClinicName, &it.ActorName, &it.Type, &it.Message, &it.Meta, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Server) platformClinic(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Negocio no encontrado.")
		return
	}
	list, err := loadClinics(r.Context(), s.db, "WHERE c.id = $1", id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if len(list) == 0 {
		writeError(w, http.StatusNotFound, "Negocio no encontrado.")
		return
	}
	prows, err := s.db.Query(r.Context(), `SELECT `+personCols+` FROM users WHERE clinic_id = $1 ORDER BY disabled, role <> 'admin', lower(name)`, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	people, err := pgx.CollectRows(prows, pgx.RowToStructByName[person])
	if err != nil {
		serverError(w, r, err)
		return
	}
	st, err := seatUsage(r.Context(), s.db, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	activity, err := loadActivity(r.Context(), s.db, id, 40)
	if err != nil {
		serverError(w, r, err)
		return
	}
	pays, err := loadPayments(r.Context(), s.db, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"clinic": list[0], "people": withLabels(people), "seats": st, "activity": activity, "payments": pays,
	})
}

type payment struct {
	ID          string    `json:"id"`
	AmountCents int       `json:"amount_cents"`
	Plan        string    `json:"plan"`
	Months      int       `json:"months"`
	Note        string    `json:"note"`
	PeriodEnd   time.Time `json:"period_end"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

func loadPayments(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, clinicID string) ([]payment, error) {
	rows, err := q.Query(ctx, `
		SELECT p.id, p.amount_cents, p.plan, p.months, p.note, p.period_end, coalesce(u.name, ''), p.created_at
		FROM payments p LEFT JOIN users u ON u.id = p.created_by
		WHERE p.clinic_id = $1 ORDER BY p.created_at DESC LIMIT 12`, clinicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []payment{}
	for rows.Next() {
		var p payment
		if err := rows.Scan(&p.ID, &p.AmountCents, &p.Plan, &p.Months, &p.Note, &p.PeriodEnd, &p.CreatedBy, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type clinicPatch struct {
	Name            *string `json:"name"`
	Kind            *string `json:"kind"`
	PhoneNumber     *string `json:"phone_number"`
	Address         *string `json:"address"`
	Plan            *string `json:"plan"`
	BillingStatus   *string `json:"billing_status"`
	TrialEndsOn     *string `json:"trial_ends_on"` // YYYY-MM-DD
	SuspendedReason *string `json:"suspended_reason"`
}

// applyClinicChange edits a clinic inside a transaction and writes the audit line.
func (s *Server) applyClinicChange(ctx context.Context, actor *Principal, id string, patch clinicPatch) (clinicRow, error) {
	var result clinicRow
	err := inTx(ctx, s.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+clinicCols+` FROM clinics c WHERE c.id = $1 FOR UPDATE OF c`, id)
		if err != nil {
			return err
		}
		cur, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[clinicRow])
		if err == pgx.ErrNoRows {
			return fail(http.StatusNotFound, "Negocio no encontrado.")
		}
		if err != nil {
			return err
		}
		cur.finish(time.Now()) // PlanName etc. for readable audit lines
		next := cur
		var changes []string
		note := func(label, from, to string) {
			if from != to {
				changes = append(changes, label+": "+from+" → "+to)
			}
		}

		if patch.Name != nil {
			n := strings.TrimSpace(*patch.Name)
			if l := utf8.RuneCountInString(n); l < 2 || l > 120 {
				return fail(http.StatusBadRequest, "El nombre debe tener entre 2 y 120 caracteres.")
			}
			note("nombre", cur.Name, n)
			next.Name = n
		}
		if patch.Kind != nil {
			if !clinicKinds[*patch.Kind] {
				return fail(http.StatusBadRequest, "Giro inválido.")
			}
			note("giro", cur.Kind, *patch.Kind)
			next.Kind = *patch.Kind
		}
		if patch.PhoneNumber != nil {
			if len(*patch.PhoneNumber) > 30 {
				return fail(http.StatusBadRequest, "El teléfono es demasiado largo.")
			}
			next.PhoneNumber = strings.TrimSpace(*patch.PhoneNumber)
		}
		if patch.Address != nil {
			if utf8.RuneCountInString(*patch.Address) > 250 {
				return fail(http.StatusBadRequest, "La dirección es demasiado larga.")
			}
			next.Address = strings.TrimSpace(*patch.Address)
		}
		if patch.Plan != nil && *patch.Plan != cur.Plan {
			plan, ok := planByID(*patch.Plan)
			if !ok {
				return fail(http.StatusBadRequest, "Plan inválido.")
			}
			st, err := seatUsage(ctx, tx, id)
			if err != nil {
				return err
			}
			if plan.MaxUsers != nil && st.UsedUsers > *plan.MaxUsers {
				return fail(http.StatusConflict, "El consultorio tiene "+itoa(st.UsedUsers)+" cuentas activas y el plan "+plan.Name+" permite "+itoa(*plan.MaxUsers)+". Que desactive cuentas primero.")
			}
			if plan.MaxDoctors != nil && st.UsedDoctors > *plan.MaxDoctors {
				return fail(http.StatusConflict, "El consultorio tiene "+itoa(st.UsedDoctors)+" médicos activos y el plan "+plan.Name+" permite "+itoa(*plan.MaxDoctors)+". Que desactive cuentas primero.")
			}
			var kinds int
			if err := tx.QueryRow(ctx, `SELECT 1 + cardinality(specialties) FROM clinics WHERE id = $1`, id).Scan(&kinds); err != nil {
				return err
			}
			if msg := plan.kindsError(kinds); msg != "" {
				return fail(http.StatusConflict, "El consultorio trabaja en "+itoa(kinds)+" giros y el plan "+plan.Name+" permite menos. Que quite giros primero.")
			}
			note("plan", cur.PlanName, plan.Name)
			next.Plan = plan.ID
		}
		if patch.TrialEndsOn != nil {
			if *patch.TrialEndsOn == "" {
				next.TrialEndsAt = nil
			} else {
				d, err := time.Parse("2006-01-02", *patch.TrialEndsOn)
				if err != nil {
					return fail(http.StatusBadRequest, "La fecha de fin de prueba no es válida.")
				}
				end := d.Add(23*time.Hour + 59*time.Minute)
				next.TrialEndsAt = &end
				changes = append(changes, "fin de prueba: "+*patch.TrialEndsOn)
			}
		}
		if patch.BillingStatus != nil && *patch.BillingStatus != cur.BillingStatus {
			switch *patch.BillingStatus {
			case "trialing", "active", "past_due", "suspended":
			default:
				return fail(http.StatusBadRequest, "Estado inválido.")
			}
			note("estado", cur.BillingStatus, *patch.BillingStatus)
			next.BillingStatus = *patch.BillingStatus
		}
		if next.BillingStatus == "trialing" && next.TrialEndsAt == nil {
			end := time.Now().Add(TrialDays * 24 * time.Hour)
			next.TrialEndsAt = &end
		}
		var suspendedAt any
		if next.BillingStatus == "suspended" {
			if patch.SuspendedReason != nil {
				next.SuspendedReason = strings.TrimSpace(*patch.SuspendedReason)
				if utf8.RuneCountInString(next.SuspendedReason) > 200 {
					return fail(http.StatusBadRequest, "El motivo es demasiado largo.")
				}
			}
			if cur.BillingStatus == "suspended" {
				suspendedAt = nil // keep the stored date (see COALESCE below)
			} else {
				suspendedAt = time.Now()
			}
		} else {
			next.SuspendedReason = ""
		}

		if _, err := tx.Exec(ctx, `
			UPDATE clinics SET name=$2, kind=$3, phone_number=$4, address=$5, plan=$6, billing_status=$7,
			       trial_ends_at=$8, suspended_reason=$9, updated_at=now(),
			       suspended_at = CASE WHEN $7 = 'suspended' THEN coalesce($10::timestamptz, suspended_at) ELSE NULL END
			WHERE id = $1`,
			id, next.Name, next.Kind, next.PhoneNumber, next.Address, next.Plan, next.BillingStatus, next.TrialEndsAt, next.SuspendedReason, suspendedAt); err != nil {
			return err
		}
		if len(changes) > 0 {
			audit(ctx, tx, id, actor, "clinic_updated", "Actualizó "+cur.Name+" ("+strings.Join(changes, "; ")+")", map[string]any{"changes": changes})
		}
		rows, err = tx.Query(ctx, `SELECT `+clinicCols+` FROM clinics c WHERE c.id = $1`, id)
		if err != nil {
			return err
		}
		if result, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[clinicRow]); err != nil {
			return err
		}
		result.finish(time.Now())
		return nil
	})
	return result, err
}

func (s *Server) updateClinic(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Negocio no encontrado.")
		return
	}
	var patch clinicPatch
	if !decode(w, r, &patch) {
		return
	}
	c, err := s.applyClinicChange(r.Context(), principalFrom(r.Context()), id, patch)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clinic": c})
}

func (s *Server) suspendClinic(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Negocio no encontrado.")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &req) {
		return
	}
	status := "suspended"
	c, err := s.applyClinicChange(r.Context(), principalFrom(r.Context()), id, clinicPatch{BillingStatus: &status, SuspendedReason: &req.Reason})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clinic": c})
}

// reactivateClinic lifts a suspension: back to the trial if one is still running, else active.
func (s *Server) reactivateClinic(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Negocio no encontrado.")
		return
	}
	list, err := loadClinics(r.Context(), s.db, "WHERE c.id = $1", id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if len(list) == 0 {
		writeError(w, http.StatusNotFound, "Negocio no encontrado.")
		return
	}
	status := "active"
	if cur := list[0]; cur.CurrentPeriodEnd == nil && cur.TrialEndsAt != nil && cur.TrialEndsAt.After(time.Now()) {
		status = "trialing"
	}
	c, err := s.applyClinicChange(r.Context(), principalFrom(r.Context()), id, clinicPatch{BillingStatus: &status})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clinic": c})
}

// recordPayment logs a payment received outside the app, extends the paid period and activates the clinic.
func (s *Server) recordPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Negocio no encontrado.")
		return
	}
	var req struct {
		Amount float64 `json:"amount"` // MXN
		Months int     `json:"months"`
		Note   string  `json:"note"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Months == 0 {
		req.Months = 1
	}
	if req.Months < 1 || req.Months > 36 {
		writeError(w, http.StatusBadRequest, "Los meses deben estar entre 1 y 36.")
		return
	}
	if math.IsNaN(req.Amount) || req.Amount < 0 || req.Amount > 10_000_000 {
		writeError(w, http.StatusBadRequest, "El monto no es válido.")
		return
	}
	if utf8.RuneCountInString(req.Note) > 200 {
		writeError(w, http.StatusBadRequest, "La nota es demasiado larga.")
		return
	}
	actor := principalFrom(r.Context())
	var name string
	var periodEnd time.Time
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var plan string
		var cur *time.Time
		err := tx.QueryRow(r.Context(), `SELECT name, plan, current_period_end FROM clinics WHERE id = $1 FOR UPDATE`, id).Scan(&name, &plan, &cur)
		if err == pgx.ErrNoRows {
			return fail(http.StatusNotFound, "Negocio no encontrado.")
		}
		if err != nil {
			return err
		}
		base := time.Now()
		if cur != nil && cur.After(base) {
			base = *cur
		}
		periodEnd = base.AddDate(0, req.Months, 0)
		cents := int(math.Round(req.Amount * 100))
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO payments (clinic_id, amount_cents, plan, months, note, period_end, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			id, cents, plan, req.Months, strings.TrimSpace(req.Note), periodEnd, actor.UserID); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `
			UPDATE clinics SET billing_status = 'active', current_period_end = $2, suspended_at = NULL, suspended_reason = '', updated_at = now()
			WHERE id = $1`, id, periodEnd); err != nil {
			return err
		}
		audit(r.Context(), tx, id, actor, "payment_recorded",
			"Registró un pago de $"+strconv.FormatFloat(req.Amount, 'f', 2, 64)+" por "+itoa(req.Months)+" mes(es) de "+name,
			map[string]any{"amount": req.Amount, "months": req.Months})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"period_end": periodEnd})
}

// ---------------------------------------------------------------------------
// Overview
// ---------------------------------------------------------------------------

type attentionItem struct {
	Kind       string     `json:"kind"` // past_due | trial_expired | trial_ending
	ClinicID   string     `json:"clinic_id"`
	ClinicName string     `json:"clinic_name"`
	Title      string     `json:"title"`
	Detail     string     `json:"detail"`
	Since      *time.Time `json:"since"`
}

func (s *Server) platformOverview(w http.ResponseWriter, r *http.Request) {
	list, err := loadClinics(r.Context(), s.db, "")
	if err != nil {
		serverError(w, r, err)
		return
	}
	now := time.Now()
	counts := map[string]int{"total": len(list), "trialing": 0, "trial_expired": 0, "active": 0, "past_due": 0, "suspended": 0}
	mrr := 0
	var attention []attentionItem
	for _, c := range list {
		counts[c.State]++
		plan, _ := planByID(c.Plan)
		switch c.State {
		case "active":
			mrr += plan.PriceMonth
		case "past_due":
			attention = append(attention, attentionItem{"past_due", c.ID, c.Name, "Pago atrasado", "Plan " + c.PlanName, c.CurrentPeriodEnd})
		case "trial_expired":
			attention = append(attention, attentionItem{"trial_expired", c.ID, c.Name, "La prueba terminó", "Sin suscripción activa", c.TrialEndsAt})
		case "trialing":
			if c.TrialDaysLeft != nil && *c.TrialDaysLeft <= 3 {
				attention = append(attention, attentionItem{"trial_ending", c.ID, c.Name, "La prueba termina pronto", itoa(*c.TrialDaysLeft) + " día(s) restantes", c.TrialEndsAt})
			}
		}
	}
	order := map[string]int{"past_due": 0, "trial_expired": 1, "trial_ending": 2}
	sort.SliceStable(attention, func(i, j int) bool { return order[attention[i].Kind] < order[attention[j].Kind] })
	if attention == nil {
		attention = []attentionItem{}
	}

	recent := list
	if len(recent) > 5 {
		recent = recent[:5]
	}
	signups30 := 0
	for _, c := range list {
		if now.Sub(c.CreatedAt) < 30*24*time.Hour {
			signups30++
		}
	}
	out := map[string]any{"counts": counts, "attention": attention, "recent": recent, "signups_30d": signups30}

	// Money is for administrators only.
	if principalFrom(r.Context()).Role == RolePlatformAdmin {
		var paid int
		if err := s.db.QueryRow(r.Context(),
			`SELECT coalesce(sum(amount_cents), 0) FROM payments WHERE created_at >= date_trunc('month', now())`).Scan(&paid); err != nil {
			serverError(w, r, err)
			return
		}
		out["mrr"] = mrr
		out["paid_this_month_cents"] = paid
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) platformActivity(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 500 {
		limit = n
	}
	clinicID := r.URL.Query().Get("clinic_id")
	if clinicID != "" && !validUUID(clinicID) {
		writeError(w, http.StatusBadRequest, "Negocio inválido.")
		return
	}
	items, err := loadActivity(r.Context(), s.db, clinicID, limit)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"activity": items})
}

// ---------------------------------------------------------------------------
// Platform staff
// ---------------------------------------------------------------------------

func (s *Server) listStaff(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `SELECT `+personCols+` FROM users WHERE clinic_id IS NULL ORDER BY disabled, role <> 'platform_admin', lower(name)`)
	if err != nil {
		serverError(w, r, err)
		return
	}
	people, err := pgx.CollectRows(rows, pgx.RowToStructByName[person])
	if err != nil {
		serverError(w, r, err)
		return
	}
	for i := range people {
		people[i].Permanent = isPermanentAdmin(people[i].Email)
	}
	roles := []map[string]string{}
	for _, role := range platformRoles {
		roles = append(roles, map[string]string{"id": role, "label": roleLabels[role]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"people": withLabels(people), "roles": roles})
}

// lockStaff serializes platform staff changes so "at least one administrator" cannot race.
func lockStaff(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7319002)`)
	return err
}

func requirePlatformAdminLeft(ctx context.Context, tx pgx.Tx) error {
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'platform_admin' AND NOT disabled`).Scan(&n); err != nil {
		return err
	}
	if n < 1 {
		return fail(http.StatusBadRequest, "Debe quedar al menos un administrador de plataforma activo.")
	}
	return nil
}

func loadStaff(ctx context.Context, tx pgx.Tx, id string) (person, error) {
	if !validUUID(id) {
		return person{}, fail(http.StatusNotFound, "Cuenta no encontrada.")
	}
	rows, err := tx.Query(ctx, `SELECT `+personCols+` FROM users WHERE id = $1 AND clinic_id IS NULL FOR UPDATE`, id)
	if err != nil {
		return person{}, err
	}
	m, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[person])
	if err == pgx.ErrNoRows {
		return person{}, fail(http.StatusNotFound, "Cuenta no encontrada.")
	}
	m.RoleLabel = roleLabels[m.Role]
	m.Permanent = isPermanentAdmin(m.Email)
	return m, err
}

func (s *Server) createStaff(w http.ResponseWriter, r *http.Request) {
	var req memberRequest
	if !decode(w, r, &req) {
		return
	}
	for _, msg := range []string{db.ValidateName(req.Name), db.ValidateEmail(req.Email), db.ValidateUsername(req.Username), db.ValidatePassword(req.Password)} {
		if msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
	}
	if !isPlatformRole(req.Role) {
		writeError(w, http.StatusBadRequest, "Elige un rol válido.")
		return
	}
	actor := principalFrom(r.Context())
	var created person
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		id, err := db.InsertUser(r.Context(), tx, db.UserParams{Name: req.Name, Email: req.Email, Username: req.Username, Phone: req.Phone, Password: req.Password, Role: req.Role})
		if err != nil {
			if msg := uniqueMessage(err); msg != "" {
				return fail(http.StatusConflict, msg)
			}
			return err
		}
		rows, err := tx.Query(r.Context(), `SELECT `+personCols+` FROM users WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if created, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[person]); err != nil {
			return err
		}
		created.RoleLabel = roleLabels[created.Role]
		audit(r.Context(), tx, "", actor, "staff_created", "Agregó a "+created.Name+" al equipo de plataforma como "+created.RoleLabel, map[string]any{"userId": id, "role": created.Role})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"person": created})
}

func (s *Server) updateStaff(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Role *string `json:"role"`
		Name *string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Role == nil && req.Name == nil {
		writeError(w, http.StatusBadRequest, "No hay nada que actualizar.")
		return
	}
	actor := principalFrom(r.Context())
	var result person
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if err := lockStaff(r.Context(), tx); err != nil {
			return err
		}
		m, err := loadStaff(r.Context(), tx, chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		if req.Role != nil && *req.Role != m.Role {
			if !isPlatformRole(*req.Role) {
				return fail(http.StatusBadRequest, "Elige un rol válido.")
			}
			if m.ID == actor.UserID {
				return fail(http.StatusBadRequest, "No puedes cambiar tu propio rol.")
			}
			if m.Permanent {
				return fail(http.StatusForbidden, "Es un administrador permanente: no se le puede cambiar el rol.")
			}
			if _, err := tx.Exec(r.Context(), `UPDATE users SET role = $2, token_version = token_version + 1 WHERE id = $1`, m.ID, *req.Role); err != nil {
				return err
			}
			if err := requirePlatformAdminLeft(r.Context(), tx); err != nil {
				return err
			}
			audit(r.Context(), tx, "", actor, "staff_role_changed", "Cambió a "+m.Name+" de "+roleLabels[m.Role]+" a "+roleLabels[*req.Role], map[string]any{"userId": m.ID, "from": m.Role, "to": *req.Role})
		}
		if req.Name != nil {
			if msg := db.ValidateName(*req.Name); msg != "" {
				return fail(http.StatusBadRequest, msg)
			}
			if _, err := tx.Exec(r.Context(), `UPDATE users SET name = $2 WHERE id = $1`, m.ID, strings.TrimSpace(*req.Name)); err != nil {
				return err
			}
		}
		result, err = loadStaff(r.Context(), tx, m.ID)
		return err
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"person": result})
}

func (s *Server) setStaffPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if msg := db.ValidatePassword(req.Password); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	hash, err := db.HashPassword(req.Password)
	if err != nil {
		serverError(w, r, err)
		return
	}
	actor := principalFrom(r.Context())
	err = inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		m, err := loadStaff(r.Context(), tx, chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		if m.ID == actor.UserID {
			return fail(http.StatusBadRequest, "Para cambiar tu propia contraseña usa Mi cuenta.")
		}
		// Si cualquiera pudiera cambiarla, podría quedarse con la cuenta de un administrador permanente.
		if m.Permanent && !isPermanentAdmin(actor.Email) {
			return fail(http.StatusForbidden, "Solo otro administrador permanente puede restablecer esta contraseña.")
		}
		if _, err := tx.Exec(r.Context(), `UPDATE users SET password_hash = $2, token_version = token_version + 1 WHERE id = $1`, m.ID, hash); err != nil {
			return err
		}
		audit(r.Context(), tx, "", actor, "staff_password_reset", "Restableció la contraseña de "+m.Name, map[string]any{"userId": m.ID})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setStaffDisabled(disable bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := principalFrom(r.Context())
		err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
			if err := lockStaff(r.Context(), tx); err != nil {
				return err
			}
			m, err := loadStaff(r.Context(), tx, chi.URLParam(r, "id"))
			if err != nil {
				return err
			}
			if disable && m.ID == actor.UserID {
				return fail(http.StatusBadRequest, "No puedes desactivar tu propia cuenta.")
			}
			if disable && m.Permanent {
				return fail(http.StatusForbidden, "Es un administrador permanente: no se puede desactivar.")
			}
			if m.Disabled == disable {
				return nil
			}
			if _, err := tx.Exec(r.Context(),
				`UPDATE users SET disabled = $2, disabled_at = CASE WHEN $2 THEN now() END, token_version = token_version + 1 WHERE id = $1`, m.ID, disable); err != nil {
				return err
			}
			verb, typ := "Reactivó", "staff_reactivated"
			if disable {
				if err := requirePlatformAdminLeft(r.Context(), tx); err != nil {
					return err
				}
				verb, typ = "Desactivó", "staff_deactivated"
			}
			audit(r.Context(), tx, "", actor, typ, verb+" la cuenta de "+m.Name, map[string]any{"userId": m.ID})
			return nil
		})
		if err != nil {
			writeFailure(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
