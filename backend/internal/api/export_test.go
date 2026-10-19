package api

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// AITaskKeys lists the keys of the AI task registry for the external tests.
func AITaskKeys() []string {
	keys := make([]string, 0, len(aiTasks))
	for _, t := range aiTasks {
		keys = append(keys, t.Key)
	}
	return keys
}

// PatientMergeTables lists the tables patient_merge.go moves from one record to the other.
func PatientMergeTables() []string { return patientTables }

// SealedColumns is every column the code seals with encFields, as "table.column".
func SealedColumns() []string {
	var out []string
	for _, c := range encounterSealed {
		out = append(out, "encounters."+c)
	}
	for _, c := range rxSealed {
		out = append(out, "prescriptions."+c)
	}
	for _, c := range certSealed {
		out = append(out, "certificates."+c)
	}
	return out
}

// TenureLabel is how long someone has been a client ("1 año y 2 meses") for the external tests.
func TenureLabel(since, now time.Time) string { return tenureLabel(since, now) }

// RunBillingNotices runs, on its own server, the pass that mails the team about payments that did not arrive.
func RunBillingNotices(ctx context.Context, db *pgxpool.Pool, cfg *config.Config, mailer mail.Sender) {
	newServer(db, cfg, mailer).billingNoticePass(ctx)
}
