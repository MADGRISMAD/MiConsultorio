package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

// Team management for a clinic, modeled on MiTiendita's "Empleados":
//   - a clinic never ends up without an active administrator (checked inside the transaction);
//   - nobody changes their own role or deactivates themselves;
//   - deactivating, changing a role or resetting a password ends that person's sessions at once;
//   - seats follow the clinic's plan;
//   - every change is written to the activity log.

type person struct {
	ID          string     `json:"id" db:"id"`
	Name        string     `json:"name" db:"name"`
	Email       string     `json:"email" db:"email"`
	Username    string     `json:"username" db:"username"`
	Phone       string     `json:"phone" db:"phone"`
	Role        string     `json:"role" db:"role"`
	RoleLabel   string     `json:"role_label" db:"-"`
	Permanent   bool       `json:"permanent" db:"-"` // solo para el equipo de plataforma
	Disabled    bool       `json:"disabled" db:"disabled"`
	LastLoginAt *time.Time `json:"last_login_at" db:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	// capabilities added to / taken from this person on top of the role
	Extra  []string `json:"permissions_extra" db:"permissions_extra"`
	Denied []string `json:"permissions_denied" db:"permissions_denied"`
	// giros the person works in (empty: all those of the clinic)
	Areas []string `json:"areas" db:"areas"`
	// what the person can do in the end
	Effective []string `json:"permissions" db:"-"`
}

const personCols = `id, name, coalesce(email, '') AS email, username, phone, role, disabled, last_login_at, created_at, permissions_extra, permissions_denied, areas`

func withLabels(list []person) []person {
	for i := range list {
		list[i].RoleLabel = roleLabels[list[i].Role]
		list[i].Effective = permissionsWith(list[i].Role, list[i].Extra, list[i].Denied)
	}
	return list
}

type seats struct {
	Plan        string `json:"plan"`
	PlanName    string `json:"plan_name"`
	MaxUsers    *int   `json:"max_users"`
	MaxDoctors  *int   `json:"max_doctors"`
	UsedUsers   int    `json:"used_users"`
	UsedDoctors int    `json:"used_doctors"`
}

func seatUsage(ctx context.Context, q queryRower, clinicID string) (seats, error) {
	var s seats
	err := q.QueryRow(ctx, `
		SELECT c.plan,
		       count(u.id) FILTER (WHERE NOT u.disabled AND u.linked_owner_id IS NULL),
		       count(u.id) FILTER (WHERE NOT u.disabled AND u.role = 'doctor' AND u.linked_owner_id IS NULL)
		FROM clinics c LEFT JOIN users u ON u.clinic_id = c.id
		WHERE c.id = $1 GROUP BY c.plan`, clinicID).Scan(&s.Plan, &s.UsedUsers, &s.UsedDoctors)
	if err != nil {
		return s, err
	}
	plan, _ := planByID(s.Plan)
	s.PlanName, s.MaxUsers, s.MaxDoctors = plan.Name, plan.MaxUsers, plan.MaxDoctors
	return s, nil
}

// room returns an error when adding the given people would exceed the plan.
func (s seats) room(addUsers, addDoctors int) *httpError {
	if s.MaxUsers != nil && s.UsedUsers+addUsers > *s.MaxUsers {
		e := fail(http.StatusConflict, "Tu plan "+s.PlanName+" ya usa todos sus lugares. Cambia de plan o desactiva a alguien.")
		e.Code = "SEAT_LIMIT"
		return e
	}
	if s.MaxDoctors != nil && s.UsedDoctors+addDoctors > *s.MaxDoctors {
		e := fail(http.StatusConflict, "Tu plan "+s.PlanName+" permite hasta "+itoa(*s.MaxDoctors)+" médicos o especialistas. Cambia de plan o desactiva a alguien.")
		e.Code = "SEAT_LIMIT"
		return e
	}
	return nil
}

func itoa(n int) string { return strconv.Itoa(n) }

// lockClinic serializes team changes of one clinic, so the "at least one admin" and seat
// checks cannot race with a concurrent request.
func lockClinic(ctx context.Context, tx pgx.Tx, clinicID string) error {
	var x int
	return tx.QueryRow(ctx, `SELECT 1 FROM clinics WHERE id = $1 FOR UPDATE`, clinicID).Scan(&x)
}

func requireAdminLeft(ctx context.Context, tx pgx.Tx, clinicID string) error {
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE clinic_id = $1 AND role = 'admin' AND NOT disabled`, clinicID).Scan(&n); err != nil {
		return err
	}
	if n < 1 {
		return fail(http.StatusBadRequest, "Debe quedar al menos un administrador activo en el consultorio.")
	}
	return nil
}

// loadMember locks and returns a person of this clinic.
func loadMember(ctx context.Context, tx pgx.Tx, clinicID, id string) (person, error) {
	if !validUUID(id) {
		return person{}, fail(http.StatusNotFound, "Cuenta no encontrada.")
	}
	var linked bool
	if err := tx.QueryRow(ctx, `SELECT linked_owner_id IS NOT NULL FROM users WHERE id = $1 AND clinic_id = $2`, id, clinicID).Scan(&linked); err == nil && linked {
		return person{}, fail(http.StatusForbidden, "Esta cuenta la administra el dueño de la organización y no se puede modificar desde la sucursal.")
	}
	rows, err := tx.Query(ctx, `SELECT `+personCols+` FROM users WHERE id = $1 AND clinic_id = $2 FOR UPDATE`, id, clinicID)
	if err != nil {
		return person{}, err
	}
	m, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[person])
	if err == pgx.ErrNoRows {
		return person{}, fail(http.StatusNotFound, "Cuenta no encontrada.")
	}
	m.RoleLabel = roleLabels[m.Role]
	m.Effective = permissionsWith(m.Role, m.Extra, m.Denied)
	return m, err
}

func (s *Server) listTeam(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `SELECT `+personCols+` FROM users WHERE clinic_id = $1 ORDER BY disabled, role <> 'admin', lower(name)`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	people, err := pgx.CollectRows(rows, pgx.RowToStructByName[person])
	if err != nil {
		serverError(w, r, err)
		return
	}
	st, err := seatUsage(r.Context(), s.db, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	roles := make([]map[string]any, 0, len(clinicRoles))
	for _, role := range clinicRoles {
		roles = append(roles, map[string]any{"id": role, "label": roleLabels[role], "permissions": permissionsFor(role)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"people": withLabels(people), "seats": st, "roles": roles})
}

type memberRequest struct {
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Username string   `json:"username"`
	Phone    string   `json:"phone"`
	Password string   `json:"password"`
	Role     string   `json:"role"`
	Areas    []string `json:"areas"`
}

func (s *Server) createMember(w http.ResponseWriter, r *http.Request) {
	var req memberRequest
	if !decode(w, r, &req) {
		return
	}
	p := principalFrom(r.Context())
	for _, msg := range []string{db.ValidateName(req.Name), db.ValidateEmail(req.Email), db.ValidateUsername(req.Username), db.ValidatePassword(req.Password)} {
		if msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
	}
	if !isClinicRole(req.Role) {
		writeError(w, http.StatusBadRequest, "Elige un rol válido.")
		return
	}

	areas, msg := s.cleanAreas(r.Context(), p.ClinicID, req.Areas)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	var created person
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if err := lockClinic(r.Context(), tx, p.ClinicID); err != nil {
			return err
		}
		st, err := seatUsage(r.Context(), tx, p.ClinicID)
		if err != nil {
			return err
		}
		doctors := 0
		if req.Role == RoleDoctor {
			doctors = 1
		}
		if he := st.room(1, doctors); he != nil {
			return he
		}
		id, err := db.InsertUser(r.Context(), tx, db.UserParams{
			ClinicID: p.ClinicID, Name: req.Name, Email: req.Email, Username: req.Username, Phone: req.Phone, Password: req.Password, Role: req.Role,
		})
		if err != nil {
			if msg := uniqueMessage(err); msg != "" {
				return fail(http.StatusConflict, msg)
			}
			return err
		}
		if len(areas) > 0 {
			if _, err := tx.Exec(r.Context(), `UPDATE users SET areas = $2 WHERE id = $1`, id, areas); err != nil {
				return err
			}
		}
		rows, err := tx.Query(r.Context(), `SELECT `+personCols+` FROM users WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if created, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[person]); err != nil {
			return err
		}
		created.RoleLabel = roleLabels[created.Role]
		audit(r.Context(), tx, p.ClinicID, p, "user_created", "Agregó a "+created.Name+" como "+roleLabels[created.Role], map[string]any{"userId": id, "role": created.Role})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"person": created})
}

type memberPatch struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
	Phone *string `json:"phone"`
	Role  *string `json:"role"`
	// capabilities for this person only (null leaves them as they are)
	PermissionsExtra  *[]string `json:"permissions_extra"`
	PermissionsDenied *[]string `json:"permissions_denied"`
	Areas             *[]string `json:"areas"`
}

func (s *Server) updateMember(w http.ResponseWriter, r *http.Request) {
	var req memberPatch
	if !decode(w, r, &req) {
		return
	}
	if req.Name == nil && req.Email == nil && req.Phone == nil && req.Role == nil && req.PermissionsExtra == nil && req.PermissionsDenied == nil && req.Areas == nil {
		writeError(w, http.StatusBadRequest, "No hay nada que actualizar.")
		return
	}
	p := principalFrom(r.Context())
	var result person
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if err := lockClinic(r.Context(), tx, p.ClinicID); err != nil {
			return err
		}
		m, err := loadMember(r.Context(), tx, p.ClinicID, chi.URLParam(r, "id"))
		if err != nil {
			return err
		}

		if req.Role != nil && *req.Role != m.Role {
			if !isClinicRole(*req.Role) {
				return fail(http.StatusBadRequest, "Elige un rol válido.")
			}
			if m.ID == p.UserID {
				return fail(http.StatusBadRequest, "No puedes cambiar tu propio rol.")
			}
			if *req.Role == RoleDoctor && !m.Disabled {
				st, err := seatUsage(r.Context(), tx, p.ClinicID)
				if err != nil {
					return err
				}
				if he := st.room(0, 1); he != nil {
					return he
				}
			}
			// A new role means new permissions: end their session so they sign in again with it.
			if _, err := tx.Exec(r.Context(), `UPDATE users SET role = $2, permissions_extra = '{}', permissions_denied = '{}', token_version = token_version + 1 WHERE id = $1`, m.ID, *req.Role); err != nil {
				return err
			}
			if m.Role == RoleAdmin {
				if err := requireAdminLeft(r.Context(), tx, p.ClinicID); err != nil {
					return err
				}
			}
			audit(r.Context(), tx, p.ClinicID, p, "user_role_changed",
				"Cambió a "+m.Name+" de "+roleLabels[m.Role]+" a "+roleLabels[*req.Role], map[string]any{"userId": m.ID, "from": m.Role, "to": *req.Role})
		}
		if req.PermissionsExtra != nil || req.PermissionsDenied != nil {
			if m.Role == RoleAdmin {
				return fail(http.StatusBadRequest, "Los administradores siempre tienen todos los permisos.")
			}
			extra, denied := m.Extra, m.Denied
			if req.PermissionsExtra != nil {
				extra = *req.PermissionsExtra
			}
			if req.PermissionsDenied != nil {
				denied = *req.PermissionsDenied
			}
			for _, list := range [][]string{extra, denied} {
				for _, perm := range list {
					if !hasPermission(grantablePerms, perm) {
						return fail(http.StatusBadRequest, "Ese permiso no se puede asignar.")
					}
				}
			}
			for _, perm := range extra {
				if hasPermission(denied, perm) || hasPermission(rolePermissions[m.Role], perm) {
					return fail(http.StatusBadRequest, "Un permiso no puede estar a la vez agregado y quitado, ni agregarse si el rol ya lo tiene.")
				}
			}
			for _, perm := range denied {
				if !hasPermission(rolePermissions[m.Role], perm) {
					return fail(http.StatusBadRequest, "Solo se pueden quitar permisos que el rol tiene.")
				}
			}
			if extra == nil {
				extra = []string{}
			}
			if denied == nil {
				denied = []string{}
			}
			if _, err := tx.Exec(r.Context(), `UPDATE users SET permissions_extra = $2, permissions_denied = $3 WHERE id = $1`, m.ID, extra, denied); err != nil {
				return err
			}
			audit(r.Context(), tx, p.ClinicID, p, "user_permissions_changed", "Cambió los permisos de "+m.Name, map[string]any{"userId": m.ID, "extra": extra, "denied": denied})
		}
		if req.Areas != nil {
			areas, msg := s.cleanAreas(r.Context(), p.ClinicID, *req.Areas)
			if msg != "" {
				return fail(http.StatusBadRequest, msg)
			}
			if _, err := tx.Exec(r.Context(), `UPDATE users SET areas = $2 WHERE id = $1`, m.ID, areas); err != nil {
				return err
			}
			audit(r.Context(), tx, p.ClinicID, p, "user_areas_changed", "Cambió las áreas de "+m.Name, map[string]any{"userId": m.ID, "areas": areas})
		}
		if req.Name != nil {
			if msg := db.ValidateName(*req.Name); msg != "" {
				return fail(http.StatusBadRequest, msg)
			}
			if _, err := tx.Exec(r.Context(), `UPDATE users SET name = $2 WHERE id = $1`, m.ID, strings.TrimSpace(*req.Name)); err != nil {
				return err
			}
		}
		if req.Phone != nil {
			if len(*req.Phone) > 30 {
				return fail(http.StatusBadRequest, "El teléfono es demasiado largo.")
			}
			if _, err := tx.Exec(r.Context(), `UPDATE users SET phone = $2 WHERE id = $1`, m.ID, strings.TrimSpace(*req.Phone)); err != nil {
				return err
			}
		}
		if req.Email != nil {
			if msg := db.ValidateEmail(*req.Email); msg != "" {
				return fail(http.StatusBadRequest, msg)
			}
			if _, err := tx.Exec(r.Context(), `UPDATE users SET email = lower($2) WHERE id = $1`, m.ID, strings.TrimSpace(*req.Email)); err != nil {
				if msg := uniqueMessage(err); msg != "" {
					return fail(http.StatusConflict, msg)
				}
				return err
			}
		}
		result, err = loadMember(r.Context(), tx, p.ClinicID, m.ID)
		return err
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"person": result})
}

func (s *Server) setMemberPassword(w http.ResponseWriter, r *http.Request) {
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
	p := principalFrom(r.Context())
	err = inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		m, err := loadMember(r.Context(), tx, p.ClinicID, chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		if m.ID == p.UserID {
			return fail(http.StatusBadRequest, "Para cambiar tu propia contraseña usa Mi cuenta.")
		}
		if _, err := tx.Exec(r.Context(), `UPDATE users SET password_hash = $2, token_version = token_version + 1 WHERE id = $1`, m.ID, hash); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "user_password_reset", "Restableció la contraseña de "+m.Name, map[string]any{"userId": m.ID})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deactivateMember(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if err := lockClinic(r.Context(), tx, p.ClinicID); err != nil {
			return err
		}
		m, err := loadMember(r.Context(), tx, p.ClinicID, chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		if m.ID == p.UserID {
			return fail(http.StatusBadRequest, "No puedes desactivar tu propia cuenta.")
		}
		if m.Disabled {
			return nil
		}
		if _, err := tx.Exec(r.Context(), `UPDATE users SET disabled = true, disabled_at = now(), token_version = token_version + 1 WHERE id = $1`, m.ID); err != nil {
			return err
		}
		if m.Role == RoleAdmin {
			if err := requireAdminLeft(r.Context(), tx, p.ClinicID); err != nil {
				return err
			}
		}
		audit(r.Context(), tx, p.ClinicID, p, "user_deactivated", "Desactivó la cuenta de "+m.Name, map[string]any{"userId": m.ID})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) reactivateMember(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	err := inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		if err := lockClinic(r.Context(), tx, p.ClinicID); err != nil {
			return err
		}
		m, err := loadMember(r.Context(), tx, p.ClinicID, chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		if !m.Disabled {
			return nil
		}
		st, err := seatUsage(r.Context(), tx, p.ClinicID)
		if err != nil {
			return err
		}
		doctors := 0
		if m.Role == RoleDoctor {
			doctors = 1
		}
		if he := st.room(1, doctors); he != nil {
			return he
		}
		if _, err := tx.Exec(r.Context(), `UPDATE users SET disabled = false, disabled_at = NULL, token_version = token_version + 1 WHERE id = $1`, m.ID); err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "user_reactivated", "Reactivó la cuenta de "+m.Name, map[string]any{"userId": m.ID})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
