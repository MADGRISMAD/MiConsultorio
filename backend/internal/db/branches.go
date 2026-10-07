package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// InsertBranchAdmin creates the administrator account of a branch, linked to the organization owner.
// It has no e-mail and a random password that is never shown or stored anywhere else, so nobody can sign
// in to it directly; sessions for it are only issued by the owner-checked branch switch.
func InsertBranchAdmin(ctx context.Context, tx pgx.Tx, clinicID, ownerID, ownerName string) (string, error) {
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	hash, err := HashPassword(hex.EncodeToString(secret))
	if err != nil {
		return "", err
	}
	username := "sucursal-" + strings.ReplaceAll(clinicID, "-", "")[:16]
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (clinic_id, name, email, username, password_hash, role, linked_owner_id)
		VALUES ($1, $2, NULL, $3, $4, 'admin', $5) RETURNING id`,
		clinicID, strings.TrimSpace(ownerName), username, hash, ownerID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create branch admin: %w", err)
	}
	return id, nil
}
