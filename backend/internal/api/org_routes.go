package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// mountOrg: Branches and consolidated reports.
func (s *Server) mountOrg(r chi.Router) {
	r.Route("/org", func(r chi.Router) {
		r.Get("/", s.orgOverview)
		r.With(require(PermAdminUsers)).Put("/", s.orgRename)
		r.With(require(PermAdminUsers)).Post("/branches", s.orgBranchCreate)
		r.With(require(PermAdminUsers)).Post("/branches/{id}/suspend", s.orgBranchSuspend(true))
		r.With(require(PermAdminUsers)).Post("/branches/{id}/reactivate", s.orgBranchSuspend(false))
		r.With(require(PermAdminUsers)).Post("/switch", s.orgSwitch)
		r.With(require(PermAdminUsers)).Get("/reports/summary", s.orgReportSummary)
		r.With(require(PermAdminUsers)).Get("/reports/summary.csv", s.orgReportCSV)
	})
}

// orgOwnerOnly resolves the organization of an owner, answering 403 NOT_ORG_OWNER to anyone else.
func (s *Server) orgOwnerOnly(w http.ResponseWriter, r *http.Request) (*Principal, *orgInfo, bool) {
	p := principalFrom(r.Context())
	org, err := orgOf(r.Context(), s.db, p)
	if err != nil {
		serverError(w, r, err)
		return nil, nil, false
	}
	if org == nil {
		writeFailure(w, r, errNotOrgOwner())
		return nil, nil, false
	}
	return p, org, true
}

// orgOverview tells the UI what to show: the organization and its branches for the owner, and whether this
// person could start one. Everyone else gets organization: null (their clinic is a clinic like any other).
func (s *Server) orgOverview(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	org, err := orgOf(r.Context(), s.db, p)
	if err != nil {
		serverError(w, r, err)
		return
	}
	plan, _ := planByID(p.Billing.Plan)
	out := map[string]any{
		"organization": nil, "is_owner": false, "can_create": false, "current_clinic_id": p.ClinicID,
		"branch_limit": plan.MaxBranches, "plan": plan.ID, "plan_name": plan.Name, "branches": []branchRow{},
	}
	if org == nil {
		// Only the first administrator of a clinic outside any organization may start one (and a plan with more than one branch).
		var first string
		var inOrg bool
		if p.Role == RoleAdmin && p.LinkedOwnerID == "" && p.ClinicID != "" {
			if err := s.db.QueryRow(r.Context(), `
				SELECT coalesce((SELECT id::text FROM users WHERE clinic_id = $1 AND role = 'admin' AND linked_owner_id IS NULL ORDER BY created_at, id LIMIT 1), ''),
				       (SELECT organization_id IS NOT NULL FROM clinics WHERE id = $1)`, p.ClinicID).Scan(&first, &inOrg); err != nil {
				serverError(w, r, err)
				return
			}
			out["can_create"] = first == p.UserID && !inOrg
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	list, err := loadBranches(r.Context(), s.db, org, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	active := 0
	for _, b := range list {
		if !b.Suspended {
			active++
		}
	}
	out["organization"] = map[string]any{"id": org.ID, "name": org.Name}
	out["is_owner"], out["can_create"] = true, true
	out["branches"], out["active_branches"] = list, active
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) orgRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	p, org, ok := s.orgOwnerOnly(w, r)
	if !ok {
		return
	}
	b := branchRequest{Name: req.Name}
	if msg := b.clean(); msg != "" {
		writeError(w, http.StatusBadRequest, "El nombre de la organización debe tener entre 2 y 120 caracteres.")
		return
	}
	if _, err := s.db.Exec(r.Context(), `UPDATE organizations SET name = $2 WHERE id = $1`, org.ID, b.Name); err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "org_renamed", "Renombró la organización a "+b.Name, nil)
	writeJSON(w, http.StatusOK, map[string]any{"organization": map[string]any{"id": org.ID, "name": b.Name}})
}

func (s *Server) orgBranchCreate(w http.ResponseWriter, r *http.Request) {
	var req branchRequest
	if !decode(w, r, &req) {
		return
	}
	if msg := req.clean(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	b, err := s.orgCreateBranch(r.Context(), principalFrom(r.Context()), req)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"branch": b})
}

// orgBranchSuspend takes a branch out of service (or back in). Clinical data is never deleted; the
// matrix and the clinic the owner is currently working in cannot be suspended.
func (s *Server) orgBranchSuspend(off bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		p, org, ok := s.orgOwnerOnly(w, r)
		if !ok {
			return
		}
		if !validUUID(id) {
			writeError(w, http.StatusNotFound, "Sucursal no encontrada.")
			return
		}
		err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
			if err := lockClinic(r.Context(), tx, org.MatrixID); err != nil {
				return err
			}
			var name string
			var suspended bool
			err := tx.QueryRow(r.Context(), `SELECT name, branch_suspended_at IS NOT NULL FROM clinics WHERE id = $1 AND organization_id = $2 FOR UPDATE`, id, org.ID).Scan(&name, &suspended)
			if errors.Is(err, pgx.ErrNoRows) {
				return fail(http.StatusNotFound, "Sucursal no encontrada.")
			}
			if err != nil {
				return err
			}
			if off {
				if id == org.MatrixID {
					return fail(http.StatusConflict, "La sucursal matriz no se puede dar de baja: lleva la suscripción de la organización.")
				}
				if id == p.ClinicID {
					return fail(http.StatusConflict, "Estás dentro de esta sucursal. Cambia a otra antes de darla de baja.")
				}
				if _, err := tx.Exec(r.Context(), `UPDATE clinics SET branch_suspended_at = coalesce(branch_suspended_at, now()), updated_at = now() WHERE id = $1`, id); err != nil {
					return err
				}
				audit(r.Context(), tx, org.MatrixID, p, "branch_suspended", "Dio de baja la sucursal "+name, map[string]any{"branch_id": id})
				audit(r.Context(), tx, id, p, "branch_suspended", "Sucursal dada de baja por el dueño de la organización", nil)
				return nil
			}
			if suspended {
				n, err := orgActiveBranches(r.Context(), tx, org.ID)
				if err != nil {
					return err
				}
				var planID string
				if err := tx.QueryRow(r.Context(), `SELECT plan FROM clinics WHERE id = $1`, org.MatrixID).Scan(&planID); err != nil {
					return err
				}
				if pl, _ := planByID(planID); n >= pl.MaxBranches {
					return branchLimitError(pl.Name, pl.MaxBranches)
				}
			}
			if _, err := tx.Exec(r.Context(), `UPDATE clinics SET branch_suspended_at = NULL, updated_at = now() WHERE id = $1`, id); err != nil {
				return err
			}
			audit(r.Context(), tx, org.MatrixID, p, "branch_reactivated", "Reactivó la sucursal "+name, map[string]any{"branch_id": id})
			audit(r.Context(), tx, id, p, "branch_reactivated", "Sucursal reactivada por el dueño de la organización", nil)
			return nil
		})
		if err != nil {
			writeFailure(w, r, err)
			return
		}
		list, err := loadBranches(r.Context(), s.db, org, p.ClinicID)
		if err != nil {
			serverError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"branches": list})
	}
}

// orgSwitch moves the owner into another branch. Ownership and membership are re-verified in the database on
// this very request (never taken from the session alone), then a fresh session is signed as the branch's
// linked administrator, or as the owner themself when the target is the matrix.
func (s *Server) orgSwitch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BranchID string `json:"branch_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	p, org, ok := s.orgOwnerOnly(w, r)
	if !ok {
		return
	}
	if !validUUID(req.BranchID) {
		writeError(w, http.StatusNotFound, "Sucursal no encontrada.")
		return
	}
	var name string
	var suspended bool
	err := s.db.QueryRow(r.Context(), `SELECT name, branch_suspended_at IS NOT NULL FROM clinics WHERE id = $1 AND organization_id = $2`, req.BranchID, org.ID).Scan(&name, &suspended)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Sucursal no encontrada.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if suspended {
		e := fail(http.StatusConflict, "Esta sucursal está dada de baja. Reactívala para entrar.")
		e.Code = "BRANCH_SUSPENDED"
		writeFailure(w, r, e)
		return
	}
	target := org.OwnerID
	if req.BranchID != org.MatrixID {
		err := s.db.QueryRow(r.Context(), `SELECT id FROM users WHERE clinic_id = $1 AND linked_owner_id = $2 AND role = 'admin' AND NOT disabled`, req.BranchID, org.OwnerID).Scan(&target)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusConflict, "La sucursal no tiene su cuenta de acceso del dueño. Contacta a soporte.")
			return
		}
		if err != nil {
			serverError(w, r, err)
			return
		}
	}
	np, err := loadPrincipal(r.Context(), s.db, target)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if np.Disabled || np.ClinicID != req.BranchID {
		writeError(w, http.StatusConflict, "No se pudo entrar a la sucursal.")
		return
	}
	if req.BranchID != p.ClinicID {
		audit(r.Context(), s.db, req.BranchID, p, "branch_switch", "El dueño entró a la sucursal "+name, nil)
	}
	s.startSession(w, r, np)
}
