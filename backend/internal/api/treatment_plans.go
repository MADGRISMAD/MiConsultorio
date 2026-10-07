package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Treatment plans by phases. Draft and proposed plans are freely editable; once accepted (signed) the history is
// kept: items are only added (a new version that asks for a new signature), cancelled or marked done.
// Amounts are in cents and, like the POS catalog, already include tax.

type planItem struct {
	ID              string     `json:"id"`
	Phase           int        `json:"phase"`
	Position        int        `json:"position"`
	Description     string     `json:"description"`
	Tooth           string     `json:"tooth"`
	CatalogItemID   *string    `json:"catalog_item_id"`
	Qty             float64    `json:"qty"`
	UnitPriceCents  int        `json:"unit_price_cents"`
	TaxRate         float64    `json:"tax_rate"`
	Status          string     `json:"status"`
	VersionAdded    int        `json:"version_added"`
	TotalCents      int        `json:"total_cents"`
	DoneAt          *time.Time `json:"done_at"`
	DoneByName      string     `json:"done_by_name"`
	DoneEncounterID *string    `json:"done_encounter_id"`
	CancelReason    string     `json:"cancel_reason"`
	SaleID          *string    `json:"sale_id"`
}

type planEvent struct {
	Version   int       `json:"version"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail"`
	ActorName string    `json:"actor_name"`
	CreatedAt time.Time `json:"created_at"`
}

type treatmentPlan struct {
	ID              string      `json:"id"`
	PatientID       string      `json:"patient_id"`
	PatientName     string      `json:"patient_name"`
	Title           string      `json:"title"`
	Status          string      `json:"status"`
	Notes           string      `json:"notes"`
	ProfessionalID  *string     `json:"professional_id"`
	Version         int         `json:"version"`
	AcceptedVersion int         `json:"accepted_version"`
	AcceptedAt      *time.Time  `json:"accepted_at"`
	AcceptedByName  string      `json:"accepted_by_name"`
	CancelReason    string      `json:"cancel_reason"`
	CreatedByName   string      `json:"created_by_name"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
	TotalCents      int         `json:"total_cents"`   // without cancelled items
	DoneCents       int         `json:"done_cents"`    // items already performed
	PendingCents    int         `json:"pending_cents"` // still to perform
	Items           []planItem  `json:"items"`
	Events          []planEvent `json:"events,omitempty"`
}

type rowsQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func lineTotal(qty float64, unit int) int { return int(math.Round(qty * float64(unit))) }

const planCols = `p.id::text, p.patient_id::text, trim(pt.names || ' ' || pt.last_names), p.title, p.status, p.notes, p.professional_id::text,
	p.version, p.accepted_version, p.accepted_at, p.accepted_by_name, p.cancel_reason, p.created_by_name, p.created_at, p.updated_at`

func scanPlan(row pgx.Row) (treatmentPlan, error) {
	var t treatmentPlan
	err := row.Scan(&t.ID, &t.PatientID, &t.PatientName, &t.Title, &t.Status, &t.Notes, &t.ProfessionalID, &t.Version, &t.AcceptedVersion,
		&t.AcceptedAt, &t.AcceptedByName, &t.CancelReason, &t.CreatedByName, &t.CreatedAt, &t.UpdatedAt)
	t.Items = []planItem{}
	return t, err
}

func (t *treatmentPlan) sum() {
	t.TotalCents, t.DoneCents, t.PendingCents = 0, 0, 0
	for i := range t.Items {
		it := &t.Items[i]
		it.TotalCents = lineTotal(it.Qty, it.UnitPriceCents)
		switch it.Status {
		case "done":
			t.DoneCents += it.TotalCents
			t.TotalCents += it.TotalCents
		case "pending":
			t.PendingCents += it.TotalCents
			t.TotalCents += it.TotalCents
		}
	}
}

func loadPlanItems(ctx context.Context, q rowsQuerier, planIDs []string) (map[string][]planItem, error) {
	out := map[string][]planItem{}
	if len(planIDs) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `SELECT plan_id::text, id::text, phase, position, description, tooth, catalog_item_id::text, qty::float8, unit_price_cents,
		tax_rate::float8, status, version_added, done_at, done_by_name, done_encounter_id::text, cancel_reason, sale_id::text
		FROM treatment_plan_items WHERE plan_id = ANY($1::uuid[]) ORDER BY phase, position, id`, planIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pid string
		var it planItem
		if err := rows.Scan(&pid, &it.ID, &it.Phase, &it.Position, &it.Description, &it.Tooth, &it.CatalogItemID, &it.Qty, &it.UnitPriceCents,
			&it.TaxRate, &it.Status, &it.VersionAdded, &it.DoneAt, &it.DoneByName, &it.DoneEncounterID, &it.CancelReason, &it.SaleID); err != nil {
			return nil, err
		}
		out[pid] = append(out[pid], it)
	}
	return out, rows.Err()
}

func loadPlan(ctx context.Context, q rowsQuerier, clinicID, id string, withEvents bool) (treatmentPlan, error) {
	t, err := scanPlan(q.QueryRow(ctx, `SELECT `+planCols+` FROM treatment_plans p JOIN patients pt ON pt.id = p.patient_id WHERE p.clinic_id=$1 AND p.id=$2`, clinicID, id))
	if err != nil {
		return t, err
	}
	items, err := loadPlanItems(ctx, q, []string{t.ID})
	if err != nil {
		return t, err
	}
	if items[t.ID] != nil {
		t.Items = items[t.ID]
	}
	t.sum()
	if withEvents {
		rows, err := q.Query(ctx, `SELECT version, action, detail, actor_name, created_at FROM treatment_plan_events WHERE plan_id=$1 ORDER BY id`, t.ID)
		if err != nil {
			return t, err
		}
		defer rows.Close()
		t.Events = []planEvent{}
		for rows.Next() {
			var e planEvent
			if err := rows.Scan(&e.Version, &e.Action, &e.Detail, &e.ActorName, &e.CreatedAt); err != nil {
				return t, err
			}
			t.Events = append(t.Events, e)
		}
		return t, rows.Err()
	}
	return t, nil
}

func addPlanEvent(ctx context.Context, tx pgx.Tx, clinicID, planID string, version int, action, detail string, p *Principal) error {
	_, err := tx.Exec(ctx, `INSERT INTO treatment_plan_events (plan_id, clinic_id, version, action, detail, actor_name) VALUES ($1,$2,$3,$4,$5,$6)`,
		planID, clinicID, version, action, detail, p.actorName())
	return err
}

func (s *Server) listPlans(w http.ResponseWriter, r *http.Request) {
	id, _, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `SELECT `+planCols+` FROM treatment_plans p JOIN patients pt ON pt.id = p.patient_id
		WHERE p.clinic_id=$1 AND p.patient_id=$2 ORDER BY p.created_at DESC LIMIT 200`, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	plans := []treatmentPlan{}
	ids := []string{}
	for rows.Next() {
		t, err := scanPlan(rows)
		if err != nil {
			rows.Close()
			serverError(w, r, err)
			return
		}
		plans = append(plans, t)
		ids = append(ids, t.ID)
	}
	rows.Close()
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	items, err := loadPlanItems(r.Context(), s.db, ids)
	if err != nil {
		serverError(w, r, err)
		return
	}
	for i := range plans {
		if items[plans[i].ID] != nil {
			plans[i].Items = items[plans[i].ID]
		}
		plans[i].sum()
	}
	s.logAccess(r.Context(), p.ClinicID, id, p, "view")
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans})
}

func (s *Server) getPlan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Plan no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	t, err := loadPlan(r.Context(), s.db, p.ClinicID, id, true)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Plan no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.logAccess(r.Context(), p.ClinicID, t.PatientID, p, "view")
	writeJSON(w, http.StatusOK, map[string]any{"plan": t})
}

type planItemIn struct {
	Phase          int     `json:"phase"`
	Description    string  `json:"description"`
	Tooth          string  `json:"tooth"`
	CatalogItemID  string  `json:"catalog_item_id"`
	Qty            float64 `json:"qty"`
	UnitPriceCents int     `json:"unit_price_cents"`
	TaxRate        float64 `json:"tax_rate"`
}

type planIn struct {
	Title          string       `json:"title"`
	Notes          string       `json:"notes"`
	ProfessionalID string       `json:"professional_id"`
	Items          []planItemIn `json:"items"`
}

// clean validates the items of a plan and returns an error message when something is wrong.
func (s *Server) cleanPlanItems(ctx context.Context, clinicID string, items []planItemIn) string {
	if len(items) > 200 {
		return "El plan tiene demasiados conceptos."
	}
	for i := range items {
		it := &items[i]
		it.Description, it.Tooth = strings.TrimSpace(it.Description), strings.TrimSpace(it.Tooth)
		if it.Phase == 0 {
			it.Phase = 1
		}
		if it.Qty == 0 {
			it.Qty = 1
		}
		switch {
		case it.Description == "" || utf8.RuneCountInString(it.Description) > 300:
			return "Cada concepto necesita una descripción (máximo 300 caracteres)."
		case utf8.RuneCountInString(it.Tooth) > 40:
			return "La pieza o zona de un concepto es demasiado larga."
		case it.Phase < 1 || it.Phase > 50:
			return "La fase debe estar entre 1 y 50."
		case it.Qty <= 0 || it.Qty > 100000 || math.IsNaN(it.Qty):
			return "La cantidad de un concepto no es válida."
		case it.UnitPriceCents < 0 || it.UnitPriceCents > 1_000_000_000:
			return "El precio de un concepto no es válido."
		case it.TaxRate < 0 || it.TaxRate > 100:
			return "El IVA de un concepto no es válido."
		}
		if it.CatalogItemID != "" {
			var found bool
			if !validUUID(it.CatalogItemID) {
				return "Un producto del catálogo no es válido."
			}
			if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM catalog_items WHERE clinic_id=$1 AND id=$2)`, clinicID, it.CatalogItemID).Scan(&found); err != nil || !found {
				return "Un producto del catálogo no existe."
			}
		}
	}
	return ""
}

func (s *Server) cleanPlanHead(ctx context.Context, clinicID string, in *planIn) string {
	in.Title, in.Notes = strings.TrimSpace(in.Title), strings.TrimSpace(in.Notes)
	switch {
	case in.Title == "" || utf8.RuneCountInString(in.Title) > 150:
		return "Escribe el título del plan (máximo 150 caracteres)."
	case utf8.RuneCountInString(in.Notes) > maxLong:
		return "Las notas son demasiado largas."
	}
	if in.ProfessionalID != "" {
		var found bool
		if !validUUID(in.ProfessionalID) {
			return "El profesional no es válido."
		}
		if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE clinic_id=$1 AND id=$2)`, clinicID, in.ProfessionalID).Scan(&found); err != nil || !found {
			return "El profesional no existe."
		}
	}
	return ""
}

func insertPlanItems(ctx context.Context, tx pgx.Tx, clinicID, planID string, version, startPos int, items []planItemIn) error {
	for i, it := range items {
		var cat any
		if it.CatalogItemID != "" {
			cat = it.CatalogItemID
		}
		if _, err := tx.Exec(ctx, `INSERT INTO treatment_plan_items (plan_id, clinic_id, phase, position, description, tooth, catalog_item_id, qty, unit_price_cents, tax_rate, version_added)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, planID, clinicID, it.Phase, startPos+i, it.Description, it.Tooth, cat, it.Qty, it.UnitPriceCents, it.TaxRate, version); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) createPlan(w http.ResponseWriter, r *http.Request) {
	id, _, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	var in planIn
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	if msg := s.cleanPlanHead(r.Context(), p.ClinicID, &in); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if msg := s.cleanPlanItems(r.Context(), p.ClinicID, in.Items); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	var prof any
	if in.ProfessionalID != "" {
		prof = in.ProfessionalID
	} else {
		prof = p.UserID
	}
	var planID string
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if err := tx.QueryRow(r.Context(), `INSERT INTO treatment_plans (clinic_id, patient_id, title, notes, professional_id, created_by_name)
			VALUES ($1,$2,$3,$4,$5,$6) RETURNING id::text`, p.ClinicID, id, in.Title, in.Notes, prof, p.actorName()).Scan(&planID); err != nil {
			return err
		}
		if err := insertPlanItems(r.Context(), tx, p.ClinicID, planID, 1, 0, in.Items); err != nil {
			return err
		}
		return addPlanEvent(r.Context(), tx, p.ClinicID, planID, 1, "created", "", p)
	})
	if err != nil {
		serverError(w, r, err)
		return
	}
	t, err := loadPlan(r.Context(), s.db, p.ClinicID, planID, true)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"plan": t})
}

// lockPlan reads a plan of this clinic inside tx with a row lock.
func lockPlan(ctx context.Context, tx pgx.Tx, clinicID, id string) (status string, version, acceptedVersion int, patientID string, err error) {
	err = tx.QueryRow(ctx, `SELECT status, version, accepted_version, patient_id::text FROM treatment_plans WHERE clinic_id=$1 AND id=$2 FOR UPDATE`, clinicID, id).
		Scan(&status, &version, &acceptedVersion, &patientID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fail(http.StatusNotFound, "Plan no encontrado.")
	}
	return
}

// respondPlan answers with the plan as it stands now.
func (s *Server) respondPlan(w http.ResponseWriter, r *http.Request, status int, id string) {
	p := principalFrom(r.Context())
	t, err := loadPlan(r.Context(), s.db, p.ClinicID, id, true)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, status, map[string]any{"plan": t})
}

func planID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Plan no encontrado.")
		return "", false
	}
	return id, true
}

// updatePlan rewrites a draft or proposed plan, items included.
func (s *Server) updatePlan(w http.ResponseWriter, r *http.Request) {
	id, ok := planID(w, r)
	if !ok {
		return
	}
	var in planIn
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	if msg := s.cleanPlanHead(r.Context(), p.ClinicID, &in); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if msg := s.cleanPlanItems(r.Context(), p.ClinicID, in.Items); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	var prof any
	if in.ProfessionalID != "" {
		prof = in.ProfessionalID
	}
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		status, version, _, _, err := lockPlan(r.Context(), tx, p.ClinicID, id)
		if err != nil {
			return err
		}
		if status != "draft" && status != "proposed" {
			return fail(http.StatusConflict, "El plan ya fue aceptado: solo se pueden agregar conceptos o cancelar los pendientes.")
		}
		if _, err := tx.Exec(r.Context(), `UPDATE treatment_plans SET title=$3, notes=$4, professional_id=$5, updated_at=now() WHERE clinic_id=$1 AND id=$2`,
			p.ClinicID, id, in.Title, in.Notes, prof); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `DELETE FROM treatment_plan_items WHERE plan_id=$1`, id); err != nil {
			return err
		}
		if err := insertPlanItems(r.Context(), tx, p.ClinicID, id, version, 0, in.Items); err != nil {
			return err
		}
		return addPlanEvent(r.Context(), tx, p.ClinicID, id, version, "edited", "", p)
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	s.respondPlan(w, r, http.StatusOK, id)
}

func (s *Server) proposePlan(w http.ResponseWriter, r *http.Request) {
	id, ok := planID(w, r)
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		status, version, _, _, err := lockPlan(r.Context(), tx, p.ClinicID, id)
		if err != nil {
			return err
		}
		if status != "draft" {
			return fail(http.StatusConflict, "Solo un borrador puede proponerse al paciente.")
		}
		var n int
		if err := tx.QueryRow(r.Context(), `SELECT count(*) FROM treatment_plan_items WHERE plan_id=$1`, id).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return fail(http.StatusBadRequest, "Agrega al menos un concepto antes de proponer el plan.")
		}
		if _, err := tx.Exec(r.Context(), `UPDATE treatment_plans SET status='proposed', updated_at=now() WHERE id=$1`, id); err != nil {
			return err
		}
		return addPlanEvent(r.Context(), tx, p.ClinicID, id, version, "proposed", "", p)
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	s.respondPlan(w, r, http.StatusOK, id)
}

// acceptPlan records the patient's signature. It is also how an added version is signed again.
func (s *Server) acceptPlan(w http.ResponseWriter, r *http.Request) {
	id, ok := planID(w, r)
	if !ok {
		return
	}
	var req struct {
		SignerName   string `json:"signer_name"`
		SignerRole   string `json:"signer_role"`
		SignaturePNG string `json:"signature_png"`
		Witness1     string `json:"witness1"`
		Witness2     string `json:"witness2"`
		Text         string `json:"text"` // the text shown when signing; defaults to a summary built here
	}
	if !decode(w, r, &req) {
		return
	}
	if req.SignerRole == "" {
		req.SignerRole = "paciente"
	}
	p := principalFrom(r.Context())
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		status, version, accepted, patientID, err := lockPlan(r.Context(), tx, p.ClinicID, id)
		if err != nil {
			return err
		}
		fresh := status == "draft" || status == "proposed"
		if !fresh && !((status == "accepted" || status == "in_progress") && version > accepted) {
			return fail(http.StatusConflict, "Este plan no está pendiente de aceptación.")
		}
		plan, err := loadPlan(r.Context(), tx, p.ClinicID, id, false)
		if err != nil {
			return err
		}
		if len(plan.Items) == 0 {
			return fail(http.StatusBadRequest, "El plan no tiene conceptos.")
		}
		text := strings.TrimSpace(req.Text)
		if text == "" {
			text = planConsentText(plan)
		}
		in := consentIn{PlanID: id, Kind: "plan_tratamiento", TextSnapshot: text, SignerName: req.SignerName, SignerRole: req.SignerRole,
			SignaturePNG: req.SignaturePNG, Witness1: req.Witness1, Witness2: req.Witness2}
		sig, msg := in.validate()
		if msg != "" {
			return fail(http.StatusBadRequest, msg)
		}
		if _, err := insertConsent(r.Context(), tx, p, patientID, in, sig, clientIP(r), r.UserAgent()); err != nil {
			return err
		}
		newStatus := status
		if fresh {
			newStatus = "accepted"
		}
		if _, err := tx.Exec(r.Context(), `UPDATE treatment_plans SET status=$3, accepted_version=version, accepted_at=now(), accepted_by_name=$4, updated_at=now() WHERE clinic_id=$1 AND id=$2`,
			p.ClinicID, id, newStatus, in.SignerName); err != nil {
			return err
		}
		return addPlanEvent(r.Context(), tx, p.ClinicID, id, version, "accepted", "Firmó "+in.SignerName+" ("+in.SignerRole+")", p)
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "plan_accepted", "Registró la aceptación firmada de un plan de tratamiento", map[string]any{"plan": id})
	s.respondPlan(w, r, http.StatusOK, id)
}

func planConsentText(t treatmentPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Plan de tratamiento: %s (versión %d)\n", t.Title, t.Version)
	for _, it := range t.Items {
		if it.Status == "cancelled" {
			continue
		}
		tooth := ""
		if it.Tooth != "" {
			tooth = " [" + it.Tooth + "]"
		}
		fmt.Fprintf(&b, "Fase %d: %s%s x%g = $%.2f\n", it.Phase, it.Description, tooth, it.Qty, float64(it.TotalCents)/100)
	}
	fmt.Fprintf(&b, "Total: $%.2f MXN\n", float64(t.TotalCents)/100)
	b.WriteString("Acepto el plan de tratamiento, sus costos y las fases descritas, y fui informado de que puede modificarse con mi autorización.")
	return b.String()
}

// addPlanItems appends concepts to an accepted plan: a new version that needs a new signature.
func (s *Server) addPlanItems(w http.ResponseWriter, r *http.Request) {
	id, ok := planID(w, r)
	if !ok {
		return
	}
	var in struct {
		Reason string       `json:"reason"`
		Items  []planItemIn `json:"items"`
	}
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	in.Reason = strings.TrimSpace(in.Reason)
	if len(in.Items) == 0 {
		writeError(w, http.StatusBadRequest, "Agrega al menos un concepto.")
		return
	}
	if utf8.RuneCountInString(in.Reason) > 300 {
		writeError(w, http.StatusBadRequest, "El motivo es demasiado largo.")
		return
	}
	if msg := s.cleanPlanItems(r.Context(), p.ClinicID, in.Items); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		status, version, _, _, err := lockPlan(r.Context(), tx, p.ClinicID, id)
		if err != nil {
			return err
		}
		if status == "draft" || status == "proposed" {
			return fail(http.StatusConflict, "El plan aún es editable: modifícalo directamente.")
		}
		if status == "cancelled" {
			return fail(http.StatusConflict, "El plan está cancelado.")
		}
		var pos int
		if err := tx.QueryRow(r.Context(), `SELECT coalesce(max(position),-1)+1 FROM treatment_plan_items WHERE plan_id=$1`, id).Scan(&pos); err != nil {
			return err
		}
		version++
		if err := insertPlanItems(r.Context(), tx, p.ClinicID, id, version, pos, in.Items); err != nil {
			return err
		}
		next := status
		if status == "completed" {
			next = "in_progress"
		}
		if _, err := tx.Exec(r.Context(), `UPDATE treatment_plans SET version=$3, status=$4, updated_at=now() WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id, version, next); err != nil {
			return err
		}
		return addPlanEvent(r.Context(), tx, p.ClinicID, id, version, "items_added", in.Reason, p)
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	s.respondPlan(w, r, http.StatusCreated, id)
}

// refreshPlanStatus derives accepted / in_progress / completed from the items.
func refreshPlanStatus(ctx context.Context, tx pgx.Tx, clinicID, id string) error {
	var done, pending int
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='done'), count(*) FILTER (WHERE status='pending') FROM treatment_plan_items WHERE plan_id=$1`, id).Scan(&done, &pending); err != nil {
		return err
	}
	next := ""
	switch {
	case done > 0 && pending == 0:
		next = "completed"
	case done > 0:
		next = "in_progress"
	}
	if next == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE treatment_plans SET status=$3, updated_at=now() WHERE clinic_id=$1 AND id=$2 AND status IN ('accepted','in_progress','completed')`, clinicID, id, next)
	return err
}

func (s *Server) markItem(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := planID(w, r)
		if !ok {
			return
		}
		itemID := chi.URLParam(r, "itemId")
		if !validUUID(itemID) {
			writeError(w, http.StatusNotFound, "Concepto no encontrado.")
			return
		}
		var req struct {
			EncounterID string `json:"encounter_id"`
			Reason      string `json:"reason"`
		}
		if !decode(w, r, &req) {
			return
		}
		req.Reason = strings.TrimSpace(req.Reason)
		p := principalFrom(r.Context())
		if action == "cancel" && (req.Reason == "" || utf8.RuneCountInString(req.Reason) > 300) {
			writeError(w, http.StatusBadRequest, "Escribe el motivo de la cancelación.")
			return
		}
		err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
			status, version, _, patientID, err := lockPlan(r.Context(), tx, p.ClinicID, id)
			if err != nil {
				return err
			}
			if status != "accepted" && status != "in_progress" && status != "completed" {
				return fail(http.StatusConflict, "El plan debe estar aceptado para registrar avances.")
			}
			var cur string
			var desc string
			err = tx.QueryRow(r.Context(), `SELECT status, description FROM treatment_plan_items WHERE plan_id=$1 AND id=$2 FOR UPDATE`, id, itemID).Scan(&cur, &desc)
			if errors.Is(err, pgx.ErrNoRows) {
				return fail(http.StatusNotFound, "Concepto no encontrado.")
			}
			if err != nil {
				return err
			}
			if cur != "pending" {
				return fail(http.StatusConflict, "El concepto ya no está pendiente.")
			}
			if action == "done" {
				var enc any
				if req.EncounterID != "" {
					var found bool
					if !validUUID(req.EncounterID) {
						return fail(http.StatusBadRequest, "El registro de la bitácora no es válido.")
					}
					if err := tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM encounters WHERE clinic_id=$1 AND patient_id=$2 AND id=$3)`, p.ClinicID, patientID, req.EncounterID).Scan(&found); err != nil {
						return err
					}
					if !found {
						return fail(http.StatusBadRequest, "El registro de la bitácora no existe.")
					}
					enc = req.EncounterID
				}
				if _, err := tx.Exec(r.Context(), `UPDATE treatment_plan_items SET status='done', done_at=now(), done_by_name=$3, done_encounter_id=$4 WHERE plan_id=$1 AND id=$2`,
					id, itemID, p.actorName(), enc); err != nil {
					return err
				}
			} else if _, err := tx.Exec(r.Context(), `UPDATE treatment_plan_items SET status='cancelled', cancel_reason=$3 WHERE plan_id=$1 AND id=$2`, id, itemID, req.Reason); err != nil {
				return err
			}
			if err := addPlanEvent(r.Context(), tx, p.ClinicID, id, version, "item_"+action, desc, p); err != nil {
				return err
			}
			return refreshPlanStatus(r.Context(), tx, p.ClinicID, id)
		})
		if err != nil {
			writeFailure(w, r, err)
			return
		}
		s.respondPlan(w, r, http.StatusOK, id)
	}
}

func (s *Server) cancelPlan(w http.ResponseWriter, r *http.Request) {
	id, ok := planID(w, r)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" || utf8.RuneCountInString(req.Reason) > 300 {
		writeError(w, http.StatusBadRequest, "Escribe el motivo de la cancelación.")
		return
	}
	p := principalFrom(r.Context())
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		status, version, _, _, err := lockPlan(r.Context(), tx, p.ClinicID, id)
		if err != nil {
			return err
		}
		if status == "completed" || status == "cancelled" {
			return fail(http.StatusConflict, "Este plan ya no puede cancelarse.")
		}
		if _, err := tx.Exec(r.Context(), `UPDATE treatment_plans SET status='cancelled', cancel_reason=$3, updated_at=now() WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id, req.Reason); err != nil {
			return err
		}
		return addPlanEvent(r.Context(), tx, p.ClinicID, id, version, "cancelled", req.Reason, p)
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	s.respondPlan(w, r, http.StatusOK, id)
}

// linkPlanSale ties plan items to the sale that charged them (called by the POS flow).
func (s *Server) linkPlanSale(w http.ResponseWriter, r *http.Request) {
	id, ok := planID(w, r)
	if !ok {
		return
	}
	var req struct {
		SaleID  string   `json:"sale_id"`
		ItemIDs []string `json:"item_ids"`
	}
	if !decode(w, r, &req) {
		return
	}
	p := principalFrom(r.Context())
	if !validUUID(req.SaleID) || len(req.ItemIDs) == 0 || len(req.ItemIDs) > 200 {
		writeError(w, http.StatusBadRequest, "Indica la venta y los conceptos.")
		return
	}
	for _, x := range req.ItemIDs {
		if !validUUID(x) {
			writeError(w, http.StatusBadRequest, "Un concepto no es válido.")
			return
		}
	}
	var found bool
	if err := s.db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM sales WHERE clinic_id=$1 AND id=$2)`, p.ClinicID, req.SaleID).Scan(&found); err != nil {
		serverError(w, r, err)
		return
	}
	if !found {
		writeError(w, http.StatusBadRequest, "La venta no existe.")
		return
	}
	tag, err := s.db.Exec(r.Context(), `UPDATE treatment_plan_items SET sale_id=$3 WHERE clinic_id=$1 AND plan_id=$2 AND id = ANY($4::uuid[]) AND status <> 'cancelled'`,
		p.ClinicID, id, req.SaleID, req.ItemIDs)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) {
			writeError(w, http.StatusBadRequest, "No se pudo vincular la venta.")
			return
		}
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Concepto no encontrado.")
		return
	}
	s.respondPlan(w, r, http.StatusOK, id)
}
