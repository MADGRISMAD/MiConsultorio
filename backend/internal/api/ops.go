package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

// backupStaleAfter is how old the last successful backup may be before health calls it stale.
const backupStaleAfter = 36 * time.Hour

// health is public: it only says whether the service is up and how current it is, never anything about people.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	out := map[string]any{"status": "ok", "version": s.cfg.BuildCommit}
	code := http.StatusOK

	start := time.Now()
	if err := s.db.Ping(ctx); err != nil {
		out["status"], code = "down", http.StatusServiceUnavailable
		out["db"] = map[string]any{"ok": false}
	} else {
		out["db"] = map[string]any{"ok": true, "latency_ms": time.Since(start).Milliseconds()}
		applied, pending, latest, err := db.MigrationStatus(ctx, s.db)
		if err != nil {
			out["migrations"] = map[string]any{"ok": false}
			out["status"] = "degraded"
		} else {
			out["migrations"] = map[string]any{"ok": pending == 0, "applied": applied, "pending": pending, "latest": latest}
			if pending > 0 {
				out["status"] = "degraded"
			}
		}
	}
	if b := s.backupStatus(); b != nil {
		out["backup"] = b
		if b["ok"] != true && out["status"] == "ok" {
			out["status"] = "degraded"
		}
	}
	writeJSON(w, code, out)
}

// backupStatus reads the file scripts/backup.sh writes after each run. nil when no file is configured or present.
func (s *Server) backupStatus() map[string]any {
	if s.cfg.BackupStatusFile == "" {
		return nil
	}
	raw, err := os.ReadFile(s.cfg.BackupStatusFile)
	if err != nil {
		return nil
	}
	var f struct {
		Status     string `json:"status"`
		FinishedAt string `json:"finished_at"`
		SizeBytes  int64  `json:"size_bytes"`
	}
	if json.Unmarshal(raw, &f) != nil {
		return map[string]any{"ok": false, "status": "unreadable"}
	}
	out := map[string]any{"status": f.Status, "finished_at": f.FinishedAt, "size_bytes": f.SizeBytes}
	ok := f.Status == "ok"
	if t, err := time.Parse(time.RFC3339, f.FinishedAt); err == nil {
		age := time.Since(t)
		out["age_hours"] = int(age.Hours())
		if age > backupStaleAfter {
			ok, out["stale"] = false, true
		}
	} else {
		ok = false
	}
	out["ok"] = ok
	return out
}

// platformHealth is for the people who run Caresia: how much is in use and what needs attention.
func (s *Server) platformHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var clinics, failed, overdue int
	var dbBytes int64
	err := s.db.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM clinics WHERE billing_status IN ('active', 'trialing')),
		       (SELECT count(*) FROM appointment_reminders WHERE status = 'failed'),
		       (SELECT count(*) FROM appointment_reminders WHERE status = 'pending' AND send_at < now() - interval '1 hour'),
		       pg_database_size(current_database())`).Scan(&clinics, &failed, &overdue, &dbBytes)
	if err != nil {
		serverError(w, r, err)
		return
	}
	applied, pending, latest, err := db.MigrationStatus(ctx, s.db)
	if err != nil {
		serverError(w, r, err)
		return
	}
	out := map[string]any{
		"version": s.cfg.BuildCommit, "active_clinics": clinics, "failed_reminders": failed, "overdue_reminders": overdue,
		"db_size_bytes": dbBytes, "migrations": map[string]any{"applied": applied, "pending": pending, "latest": latest},
	}
	if b := s.backupStatus(); b != nil {
		out["backup"] = b
	}
	writeJSON(w, http.StatusOK, out)
}

// requestLog adds an id to every request (header X-Request-Id) and logs server errors with it. It logs the route
// pattern, never the real path, query or body, so nothing about a person reaches the logs.
func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := middleware.GetReqID(r.Context())
		w.Header().Set("X-Request-Id", id)
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		if ww.Status() >= 500 {
			pattern := "?"
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				pattern = rc.RoutePattern()
			}
			log.Printf("[%s] %d %s %s", id, ww.Status(), r.Method, pattern)
		}
	})
}

// logSafeErr keeps what helps to debug and drops what may carry personal data (PostgreSQL error details quote values).
func logSafeErr(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return "postgres " + pg.Code + " " + pg.Message + " constraint=" + pg.ConstraintName
	}
	return err.Error()
}
