package db

import (
	"context"
	"io/fs"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MigrationStatus compares the embedded migrations with what schema_migrations records.
func MigrationStatus(ctx context.Context, pool *pgxpool.Pool) (applied, pending int, latest string, err error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return 0, 0, "", err
	}
	known := map[string]bool{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			known[e.Name()] = true
		}
	}
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return 0, 0, "", err
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return 0, 0, "", err
		}
		applied++
		delete(known, v)
		if v > latest {
			latest = v
		}
	}
	return applied, len(known), latest, rows.Err()
}
