package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

// Branches (sucursales): a branch is an ordinary clinic, isolated by clinic_id like any other. An
// organization only links clinics under one owner. The owner moves between them with POST /org/switch,
// which signs a new session as the branch's own linked administrator account (users.linked_owner_id)
// after re-checking ownership; nobody else's access changes. See docs/SUCURSALES.md.

type orgInfo struct {
	ID       string
	Name     string
	OwnerID  string
	MatrixID string
}

// orgOf returns the organization the principal OWNS (directly, or acting from one of its branches
// through a linked administrator account), or nil. The owner must still be an active administrator
// of the matrix clinic, and the principal's clinic must belong to the organization.
func orgOf(ctx context.Context, q queryRower, p *Principal) (*orgInfo, error) {
	if p == nil || p.ClinicID == "" || p.Role != RoleAdmin {
		return nil, nil
	}
	ownerID := p.UserID
	if p.LinkedOwnerID != "" {
		ownerID = p.LinkedOwnerID
	}
	var o orgInfo
	err := q.QueryRow(ctx, `
		SELECT o.id, o.name, o.owner_user_id, o.matrix_clinic_id
		FROM organizations o
		JOIN users ow ON ow.id = o.owner_user_id AND ow.role = 'admin' AND NOT ow.disabled AND ow.clinic_id = o.matrix_clinic_id
		JOIN clinics c ON c.id = $2 AND c.organization_id = o.id
		WHERE o.owner_user_id = $1`, ownerID, p.ClinicID).Scan(&o.ID, &o.Name, &o.OwnerID, &o.MatrixID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func errNotOrgOwner() *httpError {
	e := fail(http.StatusForbidden, "Solo el dueño de la organización puede hacer esto.")
	e.Code = "NOT_ORG_OWNER"
	return e
}

// orgClinicIDs lists every clinic of the organization: the only ids the consolidated reports may touch.
func orgClinicIDs(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, orgID string) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT id FROM clinics WHERE organization_id = $1 ORDER BY (id::text = (SELECT matrix_clinic_id::text FROM organizations WHERE id = $1)) DESC, created_at, id`, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

type branchRow struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Kind        string     `json:"kind"`
	Phone       string     `json:"phone_number"`
	Address     string     `json:"address"`
	IsMatrix    bool       `json:"is_matrix"`
	Suspended   bool       `json:"suspended"`
	SuspendedAt *time.Time `json:"suspended_at"`
	Current     bool       `json:"current"`
	People      int        `json:"people"`
	CreatedAt   time.Time  `json:"created_at"`
}

func loadBranches(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, org *orgInfo, currentClinic string) ([]branchRow, error) {
	rows, err := q.Query(ctx, `
		SELECT c.id, c.name, c.kind, c.phone_number, c.address, c.id = $2::uuid, c.branch_suspended_at, c.id = $3::uuid, c.created_at,
		       (SELECT count(*) FROM users u WHERE u.clinic_id = c.id AND NOT u.disabled AND u.linked_owner_id IS NULL)
		FROM clinics c WHERE c.organization_id = $1
		ORDER BY c.id = $2::uuid DESC, c.created_at, c.id`, org.ID, org.MatrixID, currentClinic)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []branchRow{}
	for rows.Next() {
		var b branchRow
		if err := rows.Scan(&b.ID, &b.Name, &b.Kind, &b.Phone, &b.Address, &b.IsMatrix, &b.SuspendedAt, &b.Current, &b.CreatedAt, &b.People); err != nil {
			return nil, err
		}
		b.Suspended = b.SuspendedAt != nil
		out = append(out, b)
	}
	return out, rows.Err()
}

// orgActiveBranches counts the branches in service (the matrix included) of an organization.
func orgActiveBranches(ctx context.Context, q queryRower, orgID string) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM clinics WHERE organization_id = $1 AND branch_suspended_at IS NULL`, orgID).Scan(&n)
	return n, err
}

func branchLimitError(planName string, max int) *httpError {
	e := fail(http.StatusConflict, "Tu plan "+planName+" permite "+itoa(max)+" "+pluralSucursal(max)+" (la matriz cuenta como una). Cambia de plan o da de baja una sucursal.")
	e.Code = "BRANCH_LIMIT"
	return e
}

func pluralSucursal(n int) string {
	if n == 1 {
		return "sucursal"
	}
	return "sucursales"
}

// orgBillingBlock stops payments that would not do what the buyer expects: branches share the matrix's
// subscription (they are paid from the matrix), and a plan cannot be lowered below the branches in service.
func orgBillingBlock(ctx context.Context, q queryRower, clinicID, newPlan string) *httpError {
	var orgID, matrixID string
	err := q.QueryRow(ctx, `SELECT o.id, o.matrix_clinic_id FROM clinics c JOIN organizations o ON o.id = c.organization_id WHERE c.id = $1`, clinicID).Scan(&orgID, &matrixID)
	if err != nil {
		return nil // not in an organization (or a read failure that the caller's own queries will surface)
	}
	if matrixID != clinicID {
		return fail(http.StatusConflict, "Las sucursales comparten la suscripción de la matriz. Cambia o paga el plan desde la sucursal matriz.")
	}
	n, err := orgActiveBranches(ctx, q, orgID)
	if err != nil {
		return nil
	}
	if pl, ok := planByID(newPlan); ok && n > pl.MaxBranches {
		e := fail(http.StatusConflict, "Tienes "+itoa(n)+" sucursales en servicio y el plan "+pl.Name+" permite "+itoa(pl.MaxBranches)+". Da de baja sucursales primero.")
		e.Code = "BRANCH_LIMIT"
		return e
	}
	return nil
}

type branchRequest struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Phone   string `json:"phone_number"`
	Address string `json:"address"`
}

func (b *branchRequest) clean() string {
	b.Name = strings.TrimSpace(b.Name)
	b.Phone = strings.TrimSpace(b.Phone)
	b.Address = strings.TrimSpace(b.Address)
	if n := utf8.RuneCountInString(b.Name); n < 2 || n > 120 {
		return "El nombre de la sucursal debe tener entre 2 y 120 caracteres."
	}
	if b.Kind == "" {
		b.Kind = "GENERAL_MEDICAL"
	}
	if !clinicKinds[b.Kind] {
		return "El giro de la sucursal no es válido."
	}
	if utf8.RuneCountInString(b.Phone) > 30 {
		return "El teléfono es demasiado largo."
	}
	if utf8.RuneCountInString(b.Address) > 250 {
		return "La dirección es demasiado larga."
	}
	return ""
}

// orgCreateBranch adds a branch to the principal's organization, founding the organization first when the
// principal is the clinic's owner (its oldest administrator) and none exists yet. Everything runs in one
// transaction under a lock on the matrix clinic, so concurrent requests cannot exceed the plan.
func (s *Server) orgCreateBranch(ctx context.Context, p *Principal, req branchRequest) (branchRow, error) {
	var out branchRow
	err := inTx(ctx, s.db, func(tx pgx.Tx) error {
		org, err := orgOf(ctx, tx, p)
		if err != nil {
			return err
		}
		matrixID := p.ClinicID
		if org != nil {
			matrixID = org.MatrixID
		} else if p.LinkedOwnerID != "" {
			return errNotOrgOwner()
		}
		if err := lockClinic(ctx, tx, matrixID); err != nil {
			return err
		}
		var plan, matrixName, ownerName, ownerEmail string
		var matrixOrg *string
		if err := tx.QueryRow(ctx, `SELECT plan, name, organization_id::text FROM clinics WHERE id = $1`, matrixID).Scan(&plan, &matrixName, &matrixOrg); err != nil {
			return err
		}
		if org == nil { // founding: only the clinic's first administrator becomes the owner
			var first string
			if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE clinic_id = $1 AND role = 'admin' AND linked_owner_id IS NULL ORDER BY created_at, id LIMIT 1`, matrixID).Scan(&first); err != nil {
				return err
			}
			if first != p.UserID || matrixOrg != nil {
				return errNotOrgOwner()
			}
		}
		owner := p.UserID
		if org != nil {
			owner = org.OwnerID
		}
		if err := tx.QueryRow(ctx, `SELECT name, coalesce(email, '') FROM users WHERE id = $1`, owner).Scan(&ownerName, &ownerEmail); err != nil {
			return err
		}
		pl, _ := planByID(plan)
		inService := 1
		if org != nil {
			if inService, err = orgActiveBranches(ctx, tx, org.ID); err != nil {
				return err
			}
		}
		if inService >= pl.MaxBranches {
			return branchLimitError(pl.Name, pl.MaxBranches)
		}
		if org == nil {
			org = &orgInfo{Name: matrixName, OwnerID: owner, MatrixID: matrixID}
			if err := tx.QueryRow(ctx, `INSERT INTO organizations (name, owner_user_id, matrix_clinic_id) VALUES ($1, $2, $3) RETURNING id`, org.Name, owner, matrixID).Scan(&org.ID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE clinics SET organization_id = $2 WHERE id = $1`, matrixID, org.ID); err != nil {
				return err
			}
			audit(ctx, tx, matrixID, p, "org_created", "Creó la organización "+org.Name, nil)
		}

		// A branch starts like any clinic (db.InsertClinic) with its setup wizard skipped; plan and
		// subscription are copied from the matrix by the database trigger once it joins the organization.
		id, err := db.InsertClinic(ctx, tx, db.ClinicParams{
			Name: req.Name, Kind: req.Kind, Phone: req.Phone, Address: req.Address,
			Plan: plan, Status: "active", AdminEmail: ownerEmail, SetupDone: true,
		})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE clinics b SET organization_id = $2, settings = m.settings, legal = m.legal
			FROM clinics m WHERE b.id = $1 AND m.id = $3`, id, org.ID, matrixID); err != nil {
			return err
		}
		if _, err := db.InsertBranchAdmin(ctx, tx, id, owner, ownerName); err != nil {
			return err
		}
		audit(ctx, tx, matrixID, p, "branch_created", "Creó la sucursal "+req.Name, map[string]any{"branch_id": id})
		audit(ctx, tx, id, p, "branch_created", "Sucursal creada por el dueño de la organización", map[string]any{"organization": org.Name})
		list, err := loadBranches(ctx, tx, org, p.ClinicID)
		if err != nil {
			return err
		}
		for _, b := range list {
			if b.ID == id {
				out = b
			}
		}
		return nil
	})
	return out, err
}
