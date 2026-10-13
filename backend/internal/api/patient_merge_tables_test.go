package api_test

import (
	"slices"
	"testing"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
	"github.com/madgrismad/miconsultorio/backend/internal/fieldcrypt"
)

// Merging two patients re-points every table that holds a patient_id before the duplicate is deleted. A table missing from the
// list would be wiped by the cascade, so the list is checked against the foreign keys of the real schema.
func TestPatientMergeCoversEveryTableWithAPatient(t *testing.T) {
	e := setup(t)
	rows, err := e.pool.Query(t.Context(), `
		SELECT DISTINCT c.conrelid::regclass::text FROM pg_constraint c
		WHERE c.contype = 'f' AND c.confrelid = 'patients'::regclass AND c.conrelid <> 'patients'::regclass`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	have := api.PatientMergeTables()
	// derived or per-person data the merge deliberately leaves alone
	skip := []string{"billing_checkouts"}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(have, table) && !slices.Contains(skip, table) {
			t.Errorf("table %q points at patients but patient_merge.go does not move it: its rows would be deleted with the duplicate", table)
		}
	}
}

// Every column the code seals must be listed for key rotation, or it becomes unreadable once an old key is retired.
func TestSealedColumnsAreInKeyRotation(t *testing.T) {
	for _, col := range api.SealedColumns() {
		found := false
		for _, tg := range fieldcrypt.Targets {
			if tg.Table+"."+tg.Column == col {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is sealed by the API but missing from fieldcrypt.Targets", col)
		}
	}
}
