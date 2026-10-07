package api

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// ScheduleRemindersFor rebuilds the pending reminders of one appointment (used by tests and tooling;
// the API calls scheduleReminders itself).
func ScheduleRemindersFor(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, mailer mail.Sender, clinicID, appointmentID string) error {
	return newServer(pool, cfg, mailer).scheduleReminders(ctx, pool, clinicID, appointmentID)
}
