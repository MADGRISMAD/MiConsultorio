package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

// One person, one record. Someone with the same e-mail, name and surname is the same patient: they are registered
// once, in every giro of the clinic. Two records of the same person (made while the clinic changed giro) are joined
// into the more complete one: its rows (consultations, recetas, charts, files, appointments, sales...) move over and
// the data that only the other one had fills what the kept one lacked.

// patientTables are the tables that point at a patient.
var patientTables = []string{"encounters", "prescriptions", "record_access", "appointments", "attachments", "vaccinations", "patient_charts", "treatment_plans",
	"consent_signatures", "encounter_charges", "waitlist_entries", "lab_orders", "lab_results", "sales", "arco_requests", "chronic_medications", "satisfaction_surveys", "certificates"}

// samePerson is the SQL condition that two patient rows ("a" and "b") are the same person.
const samePerson = `a.clinic_id = b.clinic_id AND a.subject = 'person' AND b.subject = 'person' AND a.email <> '' AND lower(a.email) = lower(b.email)
	AND lower(trim(a.names)) = lower(trim(b.names)) AND lower(trim(a.last_names)) = lower(trim(b.last_names)) AND trim(a.last_names) <> ''`

func filled(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(x) != ""
	case []any:
		return len(x) > 0
	case bool:
		return x
	}
	return true
}

// completeness counts what a record knows: its filled fields and answers, then how much clinical history hangs from it.
func (s *Server) completeness(ctx context.Context, q rowsQuerier, p patient) (int, error) {
	n := 0
	for _, v := range []string{p.Sex, p.CURP, p.Phone, p.Email, p.Address, p.GuardianName, p.GuardianPhone, p.GuardianEmail} {
		if v != "" {
			n++
		}
	}
	if p.BirthDate != nil {
		n++
	}
	if p.PrivacyNoticeAt != nil {
		n++
	}
	for _, v := range p.Profile {
		if filled(v) {
			n++
		}
	}
	var history int
	if err := q.QueryRow(ctx, `SELECT (SELECT count(*) FROM encounters WHERE patient_id = $1) + (SELECT count(*) FROM prescriptions WHERE patient_id = $1)
		+ (SELECT count(*) FROM patient_charts WHERE patient_id = $1)`, p.ID).Scan(&history); err != nil {
		return 0, err
	}
	return n*1000 + history, nil
}

// mergePatients moves everything of drop to keep and removes drop. Must run inside a transaction.
func (s *Server) mergePatients(ctx context.Context, tx pgx.Tx, clinicID string, actor *Principal, keepID, dropID string) error {
	if _, err := tx.Exec(ctx, `SELECT 1 FROM patients WHERE clinic_id = $1 AND id IN ($2::uuid, $3::uuid) ORDER BY id FOR UPDATE`, clinicID, keepID, dropID); err != nil {
		return err
	}
	keep, err := loadPatient(ctx, tx, clinicID, keepID)
	if err != nil {
		return err
	}
	drop, err := loadPatient(ctx, tx, clinicID, dropID)
	if err != nil {
		return err
	}
	pick := func(a, b string) string {
		if strings.TrimSpace(a) != "" {
			return a
		}
		return b
	}
	profile := map[string]any{}
	for k, v := range drop.Profile {
		if filled(v) {
			profile[k] = v
		}
	}
	for k, v := range keep.Profile {
		if filled(v) {
			profile[k] = v // the kept record's own answers win
		}
	}
	profileJSON, err := encProfile(keepID, profile)
	if err != nil {
		return err
	}
	birth := keep.BirthDate
	if birth == nil {
		birth = drop.BirthDate
	}
	ack, by := keep.PrivacyNoticeAt, keep.PrivacyNoticeBy
	if ack == nil && drop.PrivacyNoticeAt != nil {
		ack, by = drop.PrivacyNoticeAt, drop.PrivacyNoticeBy
	}
	owner := keep.OwnerID
	if owner == nil {
		owner = drop.OwnerID
	}
	curp := keep.CURP
	if curp == "" && drop.CURP != "" {
		if _, err := tx.Exec(ctx, `UPDATE patients SET curp = '' WHERE id = $1`, dropID); err != nil { // the unique CURP moves over
			return err
		}
		curp = drop.CURP
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.patient_merge', 'on', true)`); err != nil {
		return err
	}
	for _, t := range patientTables {
		if _, err := tx.Exec(ctx, `UPDATE `+t+` SET patient_id = $1 WHERE patient_id = $2`, keepID, dropID); err != nil {
			return err
		}
	}
	// giros: a record shown in every giro (no tags) stays so; otherwise the union
	if _, err := tx.Exec(ctx, `
		UPDATE patients k SET
			kinds = CASE WHEN cardinality(k.kinds) = 0 OR cardinality(d.kinds) = 0 THEN '{}'::text[]
			             ELSE (SELECT array_agg(DISTINCT x) FROM unnest(k.kinds || d.kinds) x) END,
			last_encounter_at = greatest(k.last_encounter_at, d.last_encounter_at),
			incomplete = k.incomplete AND d.incomplete, reminders_ok = k.reminders_ok OR d.reminders_ok,
			archived_at = CASE WHEN k.archived_at IS NOT NULL AND d.archived_at IS NULL THEN NULL ELSE k.archived_at END
		FROM patients d WHERE k.id = $1 AND d.id = $2`, keepID, dropID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE patients SET sex = $3, birth_date = nullif($4, '')::date, curp = $5, phone = $6, email = $7, address = $8,
			guardian_name = $9, guardian_relation = $10, guardian_phone = $11, guardian_email = $12, profile = $13,
			privacy_notice_at = $14, privacy_notice_by = $15, owner_id = $16::uuid, updated_at = now()
		WHERE clinic_id = $1 AND id = $2`,
		clinicID, keepID, pick(keep.Sex, drop.Sex), deref(birth), curp, pick(keep.Phone, drop.Phone), pick(keep.Email, drop.Email), pick(keep.Address, drop.Address),
		pick(keep.GuardianName, drop.GuardianName), pick(keep.GuardianRelation, drop.GuardianRelation), pick(keep.GuardianPhone, drop.GuardianPhone),
		pick(keep.GuardianEmail, drop.GuardianEmail), profileJSON, ack, by, owner); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM patients WHERE clinic_id = $1 AND id = $2`, clinicID, dropID); err != nil {
		return err
	}
	audit(ctx, tx, clinicID, actor, "patient_merged", "Unió dos registros de "+keep.Names+" "+keep.LastNames+": el expediente #"+itoa(drop.FileNumber)+" pasó al #"+itoa(keep.FileNumber),
		map[string]any{"kept": keepID, "dropped": dropID, "dropped_file_number": drop.FileNumber})
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// mergeDuplicates joins every group of records of the same person in a clinic and returns how many were joined.
func (s *Server) mergeDuplicates(ctx context.Context, clinicID string, actor *Principal) (int, error) {
	merged := 0
	for {
		var a, b string
		err := s.db.QueryRow(ctx, `SELECT a.id::text, b.id::text FROM patients a JOIN patients b ON a.id < b.id AND `+samePerson+` WHERE a.clinic_id = $1 LIMIT 1`, clinicID).Scan(&a, &b)
		if err == pgx.ErrNoRows {
			return merged, nil
		}
		if err != nil {
			return merged, err
		}
		pa, err := loadPatient(ctx, s.db, clinicID, a)
		if err != nil {
			return merged, err
		}
		pb, err := loadPatient(ctx, s.db, clinicID, b)
		if err != nil {
			return merged, err
		}
		sa, err := s.completeness(ctx, s.db, pa)
		if err != nil {
			return merged, err
		}
		sb, err := s.completeness(ctx, s.db, pb)
		if err != nil {
			return merged, err
		}
		keep, drop := a, b // on a tie the older file stays
		if sb > sa {
			keep, drop = b, a
		}
		if err := inTx(ctx, s.db, func(tx pgx.Tx) error { return s.mergePatients(ctx, tx, clinicID, actor, keep, drop) }); err != nil {
			return merged, err
		}
		merged++
	}
}

// mergeDuplicatesRoute lets an administrator join the duplicated records right away.
func (s *Server) mergeDuplicatesRoute(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	n, err := s.mergeDuplicates(r.Context(), p.ClinicID, p)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"merged": n})
}

// runPatientMerge joins the duplicated records of every clinic now and then, so none lasts.
func (s *Server) mergePass(ctx context.Context) {
	rows, err := s.db.Query(ctx, `SELECT DISTINCT a.clinic_id::text FROM patients a JOIN patients b ON a.id < b.id AND `+samePerson)
	if err != nil {
		log.Printf("patient merge: %v", err)
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if n, err := s.mergeDuplicates(ctx, id, nil); err != nil && ctx.Err() == nil {
			log.Printf("patient merge: clinic %s: %v", id, err)
		} else if n > 0 {
			log.Printf("patient merge: clinic %s: %d records joined", id, n)
		}
	}
}

// RunPatientMergeOnce is one pass over every clinic (used by tests and tooling).
func RunPatientMergeOnce(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, mailer mail.Sender) {
	newServer(pool, cfg, mailer).mergePass(ctx)
}

// findSamePerson looks for the record of a person already registered with that e-mail, name and surname.
func findSamePerson(ctx context.Context, q rowsQuerier, clinicID string, in *patientIn) (patient, bool, error) {
	if in.Subject != "person" || strings.TrimSpace(in.Email) == "" || strings.TrimSpace(in.LastNames) == "" {
		return patient{}, false, nil
	}
	var id string
	err := q.QueryRow(ctx, `SELECT id::text FROM patients WHERE clinic_id = $1 AND subject = 'person' AND lower(email) = lower($2)
		AND lower(trim(names)) = lower(trim($3)) AND lower(trim(last_names)) = lower(trim($4)) ORDER BY created_at LIMIT 1`,
		clinicID, strings.TrimSpace(in.Email), in.Names, in.LastNames).Scan(&id)
	if err == pgx.ErrNoRows {
		return patient{}, false, nil
	}
	if err != nil {
		return patient{}, false, err
	}
	x, err := loadPatient(ctx, q, clinicID, id)
	return x, err == nil, err
}

// reusePatient adds the giro to an existing record and fills what it lacked with what the new form brought.
func (s *Server) reusePatient(ctx context.Context, tx pgx.Tx, actor *Principal, cur patient, in *patientIn, profile map[string]any, tags []string) (patient, error) {
	pick := func(a, b string) string {
		if strings.TrimSpace(a) != "" {
			return a
		}
		return b
	}
	merged := map[string]any{}
	for k, v := range profile {
		if filled(v) {
			merged[k] = v
		}
	}
	for k, v := range cur.Profile {
		if filled(v) {
			merged[k] = v
		}
	}
	profileJSON, err := encProfile(cur.ID, merged)
	if err != nil {
		return patient{}, err
	}
	birth := deref(cur.BirthDate)
	if birth == "" {
		birth = in.BirthDate
	}
	var ack any = cur.PrivacyNoticeAt
	who := cur.PrivacyNoticeBy
	if cur.PrivacyNoticeAt == nil && in.PrivacyAck {
		ack, who = time.Now(), actor.actorName()
	}
	row := tx.QueryRow(ctx, `
		UPDATE patients SET sex = $3, birth_date = nullif($4, '')::date, curp = $5, phone = $6, address = $7,
			guardian_name = $8, guardian_relation = $9, guardian_phone = $10, guardian_email = $11, profile = $12,
			privacy_notice_at = $13, privacy_notice_by = $14,
			kinds = CASE WHEN cardinality(kinds) = 0 THEN '{}'::text[] ELSE (SELECT array_agg(DISTINCT x) FROM unnest(kinds || $15::text[]) x) END,
			archived_at = NULL, updated_at = now()
		WHERE clinic_id = $1 AND id = $2 RETURNING `+patientCols,
		actor.ClinicID, cur.ID, pick(cur.Sex, in.Sex), birth, pick(cur.CURP, in.CURP), pick(cur.Phone, in.Phone), pick(cur.Address, in.Address),
		pick(cur.GuardianName, in.GuardianName), pick(cur.GuardianRelation, in.GuardianRelation), pick(cur.GuardianPhone, in.GuardianPhone),
		pick(cur.GuardianEmail, in.GuardianEmail), profileJSON, ack, who, tags)
	out, err := scanPatient(row)
	if err != nil {
		return patient{}, err
	}
	audit(ctx, tx, actor.ClinicID, actor, "patient_reused", "Ya existía el paciente #"+itoa(out.FileNumber)+" "+out.Names+": se agregó el giro en lugar de duplicarlo", map[string]any{"patient": out.ID})
	return out, nil
}
