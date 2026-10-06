package api

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 72 // bcrypt ignores everything past 72 bytes
)

type userOut struct {
	Username    string   `json:"username"`
	Permissions []string `json:"permissions"`
}

func validPermissions(perms []string) ([]string, bool) {
	seen := map[string]bool{}
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		if !hasPermission(allPermissions, p) {
			return nil, false
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out, true
}

func validPassword(pw string) string {
	switch {
	case utf8.RuneCountInString(pw) < minPasswordLen:
		return "La contraseña debe tener al menos 8 caracteres."
	case len(pw) > maxPasswordLen:
		return "La contraseña es demasiado larga (máximo 72 bytes)."
	}
	return ""
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(),
		`SELECT username, permissions FROM users WHERE clinic_id = $1 ORDER BY username`, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	users, err := pgx.CollectRows(rows, pgx.RowToStructByPos[userOut])
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

type createUserRequest struct {
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	Permissions []string `json:"permissions"`
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if !decode(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "Los campos de usuario y contraseña son obligatorios.")
		return
	}
	if utf8.RuneCountInString(req.Username) > 64 {
		writeError(w, http.StatusBadRequest, "El nombre de usuario es demasiado largo.")
		return
	}
	if msg := validPassword(req.Password); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	perms, ok := validPermissions(req.Permissions)
	if !ok {
		writeError(w, http.StatusBadRequest, "Permisos inválidos.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		serverError(w, r, err)
		return
	}
	p := principalFrom(r.Context())
	_, err = s.db.Exec(r.Context(),
		`INSERT INTO users (clinic_id, username, password_hash, permissions) VALUES ($1, $2, $3, $4)`,
		p.ClinicID, req.Username, string(hash), perms)
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "Ya existe un usuario con ese nombre.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": userOut{Username: req.Username, Permissions: perms}})
}

type updateUserRequest struct {
	Password    *string   `json:"password"`
	Permissions *[]string `json:"permissions"`
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	var req updateUserRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Password == nil && req.Permissions == nil {
		writeError(w, http.StatusBadRequest, "No hay nada que actualizar.")
		return
	}
	p := principalFrom(r.Context())
	username := chi.URLParam(r, "username")

	tx, err := s.db.Begin(r.Context())
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())

	// Lock the clinic's users so the "at least one admin" check can't race.
	if _, err := tx.Exec(r.Context(), `SELECT 1 FROM users WHERE clinic_id = $1 FOR UPDATE`, p.ClinicID); err != nil {
		serverError(w, r, err)
		return
	}
	var exists bool
	if err := tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM users WHERE clinic_id = $1 AND username = $2)`, p.ClinicID, username).Scan(&exists); err != nil {
		serverError(w, r, err)
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "Usuario no encontrado.")
		return
	}

	if req.Password != nil {
		if msg := validPassword(*req.Password); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			serverError(w, r, err)
			return
		}
		if _, err := tx.Exec(r.Context(), `UPDATE users SET password_hash = $3 WHERE clinic_id = $1 AND username = $2`, p.ClinicID, username, string(hash)); err != nil {
			serverError(w, r, err)
			return
		}
	}
	if req.Permissions != nil {
		perms, ok := validPermissions(*req.Permissions)
		if !ok {
			writeError(w, http.StatusBadRequest, "Permisos inválidos.")
			return
		}
		if _, err := tx.Exec(r.Context(), `UPDATE users SET permissions = $3 WHERE clinic_id = $1 AND username = $2`, p.ClinicID, username, perms); err != nil {
			serverError(w, r, err)
			return
		}
		if ok, err := clinicHasUserAdmin(r, tx, p.ClinicID); err != nil {
			serverError(w, r, err)
			return
		} else if !ok {
			writeError(w, http.StatusBadRequest, "Por lo menos un usuario debe tener permisos de admin. de usuarios.")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type updatePermissionsRequest struct {
	Users map[string][]string `json:"users"`
}

// updatePermissions applies permission changes for several users atomically.
func (s *Server) updatePermissions(w http.ResponseWriter, r *http.Request) {
	var req updatePermissionsRequest
	if !decode(w, r, &req) {
		return
	}
	if len(req.Users) == 0 {
		writeError(w, http.StatusBadRequest, "No hay nada que actualizar.")
		return
	}
	p := principalFrom(r.Context())

	tx, err := s.db.Begin(r.Context())
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err := tx.Exec(r.Context(), `SELECT 1 FROM users WHERE clinic_id = $1 FOR UPDATE`, p.ClinicID); err != nil {
		serverError(w, r, err)
		return
	}
	for username, raw := range req.Users {
		perms, ok := validPermissions(raw)
		if !ok {
			writeError(w, http.StatusBadRequest, "Permisos inválidos.")
			return
		}
		tag, err := tx.Exec(r.Context(), `UPDATE users SET permissions = $3 WHERE clinic_id = $1 AND username = $2`, p.ClinicID, username, perms)
		if err != nil {
			serverError(w, r, err)
			return
		}
		if tag.RowsAffected() == 0 {
			writeError(w, http.StatusNotFound, "Usuario no encontrado: "+username)
			return
		}
	}
	if ok, err := clinicHasUserAdmin(r, tx, p.ClinicID); err != nil {
		serverError(w, r, err)
		return
	} else if !ok {
		writeError(w, http.StatusBadRequest, "Por lo menos un usuario debe tener permisos de admin. de usuarios.")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func clinicHasUserAdmin(r *http.Request, tx pgx.Tx, clinicID string) (bool, error) {
	var ok bool
	err := tx.QueryRow(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM users WHERE clinic_id = $1 AND $2 = ANY (permissions))`, clinicID, PermAdminUsers).Scan(&ok)
	return ok, err
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	username := chi.URLParam(r, "username")
	if username == p.Username {
		writeError(w, http.StatusBadRequest, "No puedes eliminar tu propio usuario.")
		return
	}
	// Admin-users accounts are protected: revoke the permission first.
	tag, err := s.db.Exec(r.Context(),
		`DELETE FROM users WHERE clinic_id = $1 AND username = $2 AND NOT ($3 = ANY (permissions))`,
		p.ClinicID, username, PermAdminUsers)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		var isAdmin bool
		err := s.db.QueryRow(r.Context(), `SELECT $3 = ANY (permissions) FROM users WHERE clinic_id = $1 AND username = $2`,
			p.ClinicID, username, PermAdminUsers).Scan(&isAdmin)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "Usuario no encontrado.")
			return
		}
		if err != nil {
			serverError(w, r, err)
			return
		}
		writeError(w, http.StatusBadRequest, "No puedes eliminar un usuario con permisos de admin. de usuarios.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
