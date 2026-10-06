package db

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Role names stored in users.role (the API package defines the same constants).
const (
	RoleAdmin         = "admin"
	RolePlatformAdmin = "platform_admin"
)

// ---- validation: each returns a user-facing message, or "" when the value is fine ----

func ValidateName(n string) string {
	if l := utf8.RuneCountInString(strings.TrimSpace(n)); l < 2 || l > 120 {
		return "El nombre debe tener entre 2 y 120 caracteres."
	}
	return ""
}

func ValidateEmail(e string) string {
	e = strings.TrimSpace(e)
	a, err := mail.ParseAddress(e)
	if err != nil || a.Address != e || len(e) > 254 || !strings.Contains(e[strings.LastIndex(e, "@"):], ".") {
		return "El correo electrónico no es válido."
	}
	return ""
}

// ValidateUsername allows letters, digits and . _ - ; '@' is reserved so an e-mail and a
// username can never be confused when signing in.
func ValidateUsername(u string) string {
	u = strings.TrimSpace(u)
	if l := utf8.RuneCountInString(u); l < 3 || l > 64 {
		return "El usuario debe tener entre 3 y 64 caracteres."
	}
	for _, r := range u {
		ok := r == '.' || r == '_' || r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if !ok {
			return "El usuario solo puede llevar letras, números, punto, guion y guion bajo."
		}
	}
	return ""
}

func ValidatePassword(p string) string {
	switch {
	case utf8.RuneCountInString(p) < 8:
		return "La contraseña debe tener al menos 8 caracteres."
	case len(p) > 72: // bcrypt ignores everything past 72 bytes
		return "La contraseña es demasiado larga (máximo 72 bytes)."
	}
	return ""
}

func HashPassword(p string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(h), err
}

// ---- users ----

// UserParams describes a new account. ClinicID is empty for platform staff.
type UserParams struct {
	ClinicID string
	Name     string
	Email    string
	Username string
	Phone    string
	Password string
	Role     string
}

// InsertUser validates and inserts an account inside tx, returning its id.
func InsertUser(ctx context.Context, tx pgx.Tx, u UserParams) (string, error) {
	for _, msg := range []string{ValidateName(u.Name), ValidateEmail(u.Email), ValidateUsername(u.Username), ValidatePassword(u.Password)} {
		if msg != "" {
			return "", errors.New(msg)
		}
	}
	hash, err := HashPassword(u.Password)
	if err != nil {
		return "", err
	}
	var clinic any
	if u.ClinicID != "" {
		clinic = u.ClinicID
	}
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (clinic_id, name, email, username, phone, password_hash, role)
		VALUES ($1, $2, lower($3), $4, $5, $6, $7) RETURNING id`,
		clinic, strings.TrimSpace(u.Name), strings.TrimSpace(u.Email), strings.TrimSpace(u.Username), strings.TrimSpace(u.Phone), hash, u.Role).Scan(&id)
	return id, err
}

// ---- clinics ----

const TrialDays = 14

// ClinicParams describes a new clinic and its first administrator.
type ClinicParams struct {
	Name, Phone, Address, ImageURL string
	Kind                           string // GENERAL_MEDICAL when empty
	Plan                           string // consultorio when empty
	Status                         string // "trialing" (default) or "active"
	AdminName, AdminEmail          string
	AdminUsername, AdminPassword   string
}

// CreateClinic inserts a clinic and its administrator in one transaction.
func CreateClinic(ctx context.Context, pool *pgxpool.Pool, p ClinicParams) (clinicID string, err error) {
	if p.Kind == "" {
		p.Kind = "GENERAL_MEDICAL"
	}
	if p.Plan == "" {
		p.Plan = "consultorio"
	}
	var trialEnds any
	switch p.Status {
	case "", "trialing":
		p.Status = "trialing"
		trialEnds = time.Now().Add(TrialDays * 24 * time.Hour)
	case "active":
	default:
		return "", fmt.Errorf("estado de suscripción inválido: %q", p.Status)
	}
	if strings.TrimSpace(p.AdminName) == "" {
		p.AdminName = p.AdminUsername
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err := tx.QueryRow(ctx, `
		INSERT INTO clinics (name, email, phone_number, address, image_url, kind, plan, billing_status, trial_ends_at)
		VALUES ($1, lower($2), $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		p.Name, p.AdminEmail, p.Phone, p.Address, p.ImageURL, p.Kind, p.Plan, p.Status, trialEnds).Scan(&clinicID); err != nil {
		return "", fmt.Errorf("create clinic: %w", err)
	}
	if _, err := InsertUser(ctx, tx, UserParams{
		ClinicID: clinicID, Name: p.AdminName, Email: p.AdminEmail, Username: p.AdminUsername, Password: p.AdminPassword, Role: RoleAdmin,
	}); err != nil {
		return "", fmt.Errorf("create admin: %w", err)
	}
	return clinicID, tx.Commit(ctx)
}

// EnsureFirstClinic creates the clinic described by p when the database has none,
// so a fresh deployment is usable without any manual step. It is a no-op otherwise.
func EnsureFirstClinic(ctx context.Context, pool *pgxpool.Pool, p ClinicParams) (created bool, err error) {
	var any bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM clinics)`).Scan(&any); err != nil {
		return false, err
	}
	if any {
		return false, nil
	}
	_, err = CreateClinic(ctx, pool, p)
	return err == nil, err
}

// ---- platform staff ----

// CreatePlatformUser creates a platform account (platform_admin or platform_support).
func CreatePlatformUser(ctx context.Context, pool *pgxpool.Pool, u UserParams) (string, error) {
	if u.Role == "" {
		u.Role = RolePlatformAdmin
	}
	if u.Role != RolePlatformAdmin && u.Role != "platform_support" {
		return "", fmt.Errorf("rol de plataforma inválido: %q", u.Role)
	}
	u.ClinicID = ""
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	id, err := InsertUser(ctx, tx, u)
	if err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

// EnsureFirstPlatformAdmin creates the platform administrator when none exists yet.
func EnsureFirstPlatformAdmin(ctx context.Context, pool *pgxpool.Pool, u UserParams) (created bool, err error) {
	var any bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE role = 'platform_admin')`).Scan(&any); err != nil {
		return false, err
	}
	if any {
		return false, nil
	}
	u.Role = RolePlatformAdmin
	_, err = CreatePlatformUser(ctx, pool, u)
	return err == nil, err
}

// ResetPassword sets a new password for the account whose e-mail or username is identifier,
// ending all of that account's sessions. It returns the account's name.
func ResetPassword(ctx context.Context, pool *pgxpool.Pool, identifier, password string) (string, error) {
	if msg := ValidatePassword(password); msg != "" {
		return "", errors.New(msg)
	}
	hash, err := HashPassword(password)
	if err != nil {
		return "", err
	}
	var name string
	err = pool.QueryRow(ctx, `
		UPDATE users SET password_hash = $2, token_version = token_version + 1
		WHERE lower(email) = lower($1) OR lower(username) = lower($1)
		RETURNING name`, strings.TrimSpace(identifier), hash).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("no existe ninguna cuenta con el correo o usuario %q", identifier)
	}
	return name, err
}
