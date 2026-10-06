package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// httpError is a failure that should reach the user with a specific status and message.
type httpError struct {
	Status int
	Msg    string
	Code   string
}

func (e *httpError) Error() string { return e.Msg }

func fail(status int, msg string) *httpError { return &httpError{Status: status, Msg: msg} }

// writeFailure maps an error from a handler step to a response.
func writeFailure(w http.ResponseWriter, r *http.Request, err error) {
	var he *httpError
	if errors.As(err, &he) {
		writeJSON(w, he.Status, errorBody{Message: he.Msg, Code: he.Code})
		return
	}
	serverError(w, r, err)
}

// inTx runs fn in a transaction, committing only when fn succeeds.
func inTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// audit writes one line to the activity log. clinicID may be empty for platform-level events.
// A failure to log never blocks the action itself.
func audit(ctx context.Context, q execer, clinicID string, actor *Principal, typ, message string, meta map[string]any) {
	var clinic, actorID any
	if clinicID != "" {
		clinic = clinicID
	}
	name := ""
	if actor != nil {
		actorID, name = actor.UserID, actor.actorName()
	}
	if meta == nil {
		meta = map[string]any{}
	}
	raw, _ := json.Marshal(meta)
	if _, err := q.Exec(ctx,
		`INSERT INTO activity_log (clinic_id, actor_id, actor_name, type, message, meta) VALUES ($1, $2, $3, $4, $5, $6)`,
		clinic, actorID, name, typ, message, raw); err != nil {
		log.Printf("activity log: %v", err)
	}
}

// uniqueMessage turns a unique-constraint violation on accounts into a readable message.
func uniqueMessage(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_username_key":
			return "Ya existe una cuenta con ese usuario."
		case "users_email_key":
			return "Ya existe una cuenta con ese correo."
		}
		return "Ya existe un registro con esos datos."
	}
	return ""
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
