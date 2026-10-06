// Package db opens the PostgreSQL pool and applies embedded SQL migrations.
package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := open(ctx, url)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "3D000" { // database does not exist yet
		if cerr := createDatabase(ctx, url); cerr != nil {
			return nil, fmt.Errorf("la base de datos no existe y no se pudo crear: %w", cerr)
		}
		pool, err = open(ctx, url)
	}
	return pool, err
}

func open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// createDatabase creates the database named in url by connecting to the maintenance
// database "postgres" with the same credentials.
func createDatabase(ctx context.Context, url string) error {
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return err
	}
	name := cfg.Database
	cfg.Database = "postgres"
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	return err
}

// Migrate applies every migration that has not run yet, in filename order.
// A transaction-scoped advisory lock keeps concurrent instances from racing.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7319001)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var done bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ClinicParams describes a clinic and its first administrator.
type ClinicParams struct {
	Name, Email, Phone, Address, ImageURL string
	Username, Password                    string
}

// AllPermissions is the full permission set granted to a clinic's first administrator.
var AllPermissions = []string{"adminUsers", "adminAppointments", "adminHistorials", "navHistorials", "navAppointments"}

// CreateClinic inserts a clinic and its administrator in one transaction.
func CreateClinic(ctx context.Context, pool *pgxpool.Pool, p ClinicParams) (string, error) {
	if n := utf8.RuneCountInString(p.Password); n < 8 || len(p.Password) > 72 {
		return "", errors.New("la contraseña del administrador debe tener entre 8 y 72 caracteres")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var id string
	if err := tx.QueryRow(ctx,
		`INSERT INTO clinics (name, email, phone_number, address, image_url) VALUES ($1, lower($2), $3, $4, $5) RETURNING id`,
		p.Name, p.Email, p.Phone, p.Address, p.ImageURL).Scan(&id); err != nil {
		return "", fmt.Errorf("create clinic: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO users (clinic_id, username, password_hash, permissions) VALUES ($1, $2, $3, $4)`,
		id, p.Username, string(hash), AllPermissions); err != nil {
		return "", fmt.Errorf("create admin: %w", err)
	}
	return id, tx.Commit(ctx)
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
