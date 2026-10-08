package api

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// ScheduleRemindersFor rebuilds the pending reminders of one appointment (used by tests and tooling;
// the API calls scheduleReminders itself).
func ScheduleRemindersFor(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, mailer mail.Sender, clinicID, appointmentID string) error {
	return newServer(pool, cfg, mailer).scheduleReminders(ctx, pool, clinicID, appointmentID)
}

// SetApptClock replaces the clock the appointment rules use (arrival window, completion, no-show) and returns
// a function that restores it. Tests only.
func SetApptClock(f func() time.Time) (restore func()) {
	prev := apptNow
	apptNow = f
	return func() { apptNow = prev }
}
