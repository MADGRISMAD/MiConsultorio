package api

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

var curpRe = regexp.MustCompile(`^[A-Z0-9]{18}$`)

func normalizeCURP(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// patient is the registry card: who they are, how to reach them and the kind-specific antecedentes.
type patient struct {
	ID               string         `json:"id"`
	FileNumber       int            `json:"file_number"`
	Subject          string         `json:"subject"`
	Names            string         `json:"names"`
	LastNames        string         `json:"last_names"`
	Sex              string         `json:"sex"`
	BirthDate        *string        `json:"birth_date"`
	Age              *int           `json:"age"`
	CURP             string         `json:"curp"`
	Phone            string         `json:"phone"`
	Email            string         `json:"email"`
	Address          string         `json:"address"`
	GuardianName     string         `json:"guardian_name"`
	GuardianRelation string         `json:"guardian_relation"`
	GuardianPhone    string         `json:"guardian_phone"`
	GuardianEmail    string         `json:"guardian_email"`
	OwnerID          *string        `json:"owner_id"` // animals: the owner record the guardian_* columns copy
	Profile          map[string]any `json:"profile"`
	Incomplete       bool           `json:"incomplete"`
	PrivacyNoticeAt  *time.Time     `json:"privacy_notice_at"`
	PrivacyNoticeBy  string         `json:"privacy_notice_by"`
	LastEncounterAt  *time.Time     `json:"last_encounter_at"`
	ArchivedAt       *time.Time     `json:"archived_at"`
	ArchiveReason    string         `json:"archive_reason"`
	CreatedAt        time.Time      `json:"created_at"`
}

const patientCols = `id, file_number, subject, names, last_names, sex, to_char(birth_date, 'YYYY-MM-DD'),
	CASE WHEN birth_date IS NULL THEN NULL ELSE date_part('year', age(birth_date))::int END,
	curp, phone, email, address, guardian_name, guardian_relation, guardian_phone, guardian_email, owner_id::text, profile,
	incomplete, privacy_notice_at, privacy_notice_by, last_encounter_at, archived_at, archive_reason, created_at`

func scanPatient(row pgx.Row) (patient, error) {
	var p patient
	var raw []byte
	err := row.Scan(&p.ID, &p.FileNumber, &p.Subject, &p.Names, &p.LastNames, &p.Sex, &p.BirthDate, &p.Age, &p.CURP, &p.Phone, &p.Email, &p.Address,
		&p.GuardianName, &p.GuardianRelation, &p.GuardianPhone, &p.GuardianEmail, &p.OwnerID, &raw, &p.Incomplete, &p.PrivacyNoticeAt, &p.PrivacyNoticeBy,
		&p.LastEncounterAt, &p.ArchivedAt, &p.ArchiveReason, &p.CreatedAt)
	if err != nil {
		return p, err
	}
	p.Profile, err = decProfile(p.ID, raw)
	if err != nil {
		return patient{}, err
	}
	return p, nil
}

func loadPatient(ctx context.Context, q queryRower, clinicID, id string) (patient, error) {
	return scanPatient(q.QueryRow(ctx, `SELECT `+patientCols+` FROM patients WHERE clinic_id = $1 AND id = $2`, clinicID, id))
}

type patientIn struct {
	Subject          string         `json:"subject"`
	Names            string         `json:"names"`
	LastNames        string         `json:"last_names"`
	Sex              string         `json:"sex"`
	BirthDate        string         `json:"birth_date"`
	CURP             string         `json:"curp"`
	Phone            string         `json:"phone"`
	Email            string         `json:"email"`
	Address          string         `json:"address"`
	GuardianName     string         `json:"guardian_name"`
	GuardianRelation string         `json:"guardian_relation"`
	GuardianPhone    string         `json:"guardian_phone"`
	GuardianEmail    string         `json:"guardian_email"`
	OwnerBirthDate   string         `json:"owner_birth_date"` // animals: the owner's birth date (informative)
	OwnerID          *string        `json:"owner_id"`         // animals: an existing owner of the clinic (otherwise the typed data finds or creates one)
	Profile          map[string]any `json:"profile"`
	PrivacyAck       bool           `json:"privacy_ack"` // the patient received and accepted the aviso de privacidad
}

// clinicKindsFor loads the giros of the signed-in person's clinic.
func (s *Server) clinicKindsFor(ctx context.Context, clinicID string) ([]string, error) {
	c, err := loadClinic(ctx, s.db, clinicID)
	if err != nil {
		return nil, err
	}
	return clinicKindsOf(c.Kind, c.Specialties), nil
}

// validateCore trims and checks the fixed part of a patient. quick skips the clinical profile.
func (in *patientIn) validateCore(kinds []string, quick bool) string {
	in.Names, in.LastNames, in.Sex = strings.TrimSpace(in.Names), strings.TrimSpace(in.LastNames), strings.TrimSpace(in.Sex)
	in.Phone, in.Email, in.Address = strings.TrimSpace(in.Phone), strings.TrimSpace(in.Email), strings.TrimSpace(in.Address)
	in.GuardianName, in.GuardianRelation = strings.TrimSpace(in.GuardianName), strings.TrimSpace(in.GuardianRelation)
	in.GuardianPhone, in.GuardianEmail = strings.TrimSpace(in.GuardianPhone), strings.TrimSpace(in.GuardianEmail)
	in.CURP = normalizeCURP(in.CURP)
	if in.Subject == "" {
		in.Subject = subjectsFor(kinds)[0]
	}
	if !slices.Contains(subjectsFor(kinds), in.Subject) {
		return "Este consultorio no atiende ese tipo de paciente."
	}
	for _, v := range []struct {
		s    string
		max  int
		name string
	}{{in.Names, 120, "El nombre"}, {in.LastNames, 120, "Los apellidos"}, {in.Phone, 30, "El teléfono"}, {in.Email, 160, "El correo"}, {in.Address, 300, "El domicilio"},
		{in.GuardianName, 160, "El nombre del responsable"}, {in.GuardianRelation, 40, "El parentesco"}, {in.GuardianPhone, 30, "El teléfono del responsable"}, {in.GuardianEmail, 160, "El correo del responsable"}} {
		if utf8.RuneCountInString(v.s) > v.max {
			return v.name + " es demasiado largo."
		}
	}
	if in.Names == "" {
		if in.Subject == "animal" {
			return "Escribe el nombre del animal."
		}
		return "Escribe el nombre."
	}
	for _, e := range []string{in.Email, in.GuardianEmail} {
		if e != "" && (!strings.Contains(e, "@") || strings.ContainsAny(e, " \r\n")) {
			return "El correo no es válido."
		}
	}
	if in.BirthDate != "" {
		t, err := time.Parse("2006-01-02", in.BirthDate)
		if err != nil || t.After(time.Now()) || t.Year() < 1900 {
			return "La fecha de nacimiento no es válida."
		}
	}
	in.OwnerBirthDate = strings.TrimSpace(in.OwnerBirthDate)
	if in.OwnerBirthDate != "" {
		t, err := time.Parse("2006-01-02", in.OwnerBirthDate)
		if err != nil || t.After(time.Now()) || t.Year() < 1900 {
			return "La fecha de nacimiento del propietario no es válida."
		}
	}
	if in.Subject == "animal" {
		if !slices.Contains([]string{"", "Macho", "Hembra"}, in.Sex) {
			return "El sexo del animal no es válido."
		}
		if in.CURP != "" {
			return "Los animales no llevan CURP."
		}
		// An owner picked from the clinic's list brings their own name and phone.
		pickedOwner := in.OwnerID != nil && strings.TrimSpace(*in.OwnerID) != ""
		if !pickedOwner && (in.GuardianName == "" || in.GuardianPhone == "") {
			return "Escribe el nombre y el teléfono del propietario."
		}
		return ""
	}
	if !slices.Contains([]string{"", "Mujer", "Hombre", "Otro"}, in.Sex) {
		return "El sexo no es válido."
	}
	if in.LastNames == "" {
		return "Escribe los apellidos."
	}
	if in.CURP != "" && !curpRe.MatchString(in.CURP) {
		return "La CURP debe tener 18 caracteres alfanuméricos."
	}
	// A minor is attended with a responsible adult.
	if in.BirthDate != "" {
		if t, _ := time.Parse("2006-01-02", in.BirthDate); t.AddDate(18, 0, 0).After(time.Now()) && (in.GuardianName == "" || in.GuardianPhone == "") {
			return "Es menor de edad: escribe el nombre y el teléfono de su madre, padre o tutor."
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Schema (what the forms ask for)
// ---------------------------------------------------------------------------

func (s *Server) patientSchema(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	kinds, err := s.clinicKindsFor(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	subjects := subjectsFor(kinds)
	prof, meas, measAll := map[string][]Field{}, map[string][]Field{}, map[string][]Field{}
	for _, sub := range subjects {
		prof[sub], meas[sub], measAll[sub] = profileFields(sub, kinds), measureFields(sub, kinds), allMeasureFields(sub, kinds)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"subjects": subjects, "profile": prof, "measures": meas, "measures_all": measAll, "rx_mode": rxModeFor(kinds), "kinds": kinds,
		"encounter_kinds": encounterKindsFor("person", kinds), "encounter_kinds_animal": encounterKindsFor("animal", kinds),
		"routes": rxRoutes,
	})
}

// ---------------------------------------------------------------------------
// List, lookup, read
// ---------------------------------------------------------------------------

type patientRow struct {
	ID              string     `json:"id"`
	FileNumber      int        `json:"file_number"`
	Subject         string     `json:"subject"`
	Names           string     `json:"names"`
	LastNames       string     `json:"last_names"`
	Age             *int       `json:"age"`
	Phone           string     `json:"phone"`
	GuardianName    string     `json:"guardian_name"`
	GuardianPhone   string     `json:"guardian_phone"`
	OwnerID         *string    `json:"owner_id"`
	Species         string     `json:"species,omitempty"`
	Incomplete      bool       `json:"incomplete"`
	NoPrivacyNotice bool       `json:"no_privacy_notice"`
	LastEncounterAt *time.Time `json:"last_encounter_at"`
	ArchivedAt      *time.Time `json:"archived_at"`
}

func (s *Server) patientQuery(w http.ResponseWriter, r *http.Request, where string, args []any, limit int) []patientRow {
	rows, err := s.db.Query(r.Context(), `
		SELECT id, file_number, subject, names, last_names,
		       CASE WHEN birth_date IS NULL THEN NULL ELSE date_part('year', age(birth_date))::int END,
		       phone, guardian_name, guardian_phone, owner_id::text, coalesce(profile->>'species', ''), incomplete, privacy_notice_at IS NULL, last_encounter_at, archived_at
		FROM patients WHERE `+where+` ORDER BY lower(names), lower(last_names) LIMIT `+strconv.Itoa(limit), args...)
	if err != nil {
		serverError(w, r, err)
		return nil
	}
	defer rows.Close()
	out := []patientRow{}
	for rows.Next() {
		var x patientRow
		if err := rows.Scan(&x.ID, &x.FileNumber, &x.Subject, &x.Names, &x.LastNames, &x.Age, &x.Phone, &x.GuardianName, &x.GuardianPhone, &x.OwnerID, &x.Species, &x.Incomplete, &x.NoPrivacyNotice, &x.LastEncounterAt, &x.ArchivedAt); err != nil {
			serverError(w, r, err)
			return nil
		}
		out = append(out, x)
	}
	return out
}

// patientKindsFor are the tags a new patient gets: animals belong to veterinary, people to the other giros of the clinic.
func patientKindsFor(subject string, clinicKinds []string) []string {
	if subject == "animal" {
		return []string{"VETERINARY"}
	}
	out := []string{}
	for _, k := range clinicKinds {
		if k != "VETERINARY" {
			out = append(out, k)
		}
	}
	if len(out) == 0 {
		return slices.Clone(clinicKinds)
	}
	return out
}

// onlyActiveGiros keeps the patients of the giros the clinic works with now (untagged ones are always shown).
func onlyActiveGiros(where string, args []any, kinds []string) (string, []any) {
	args = append(args, kinds)
	return where + " AND (cardinality(kinds) = 0 OR kinds && $" + itoa(len(args)) + "::text[])", args
}

func searchWhere(clinicID, q string) (string, []any) {
	where, args := "clinic_id = $1", []any{clinicID}
	if q = strings.TrimSpace(q); q != "" {
		args = append(args, "%"+escapeLike(q)+"%", q)
		where += " AND (names ILIKE $2 OR last_names ILIKE $2 OR (names || ' ' || last_names) ILIKE $2 OR guardian_name ILIKE $2 OR guardian_phone ILIKE $2 OR curp = upper($3) OR phone ILIKE $2 OR file_number::text = $3)"
	}
	return where, args
}

func (s *Server) listPatients(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	kinds, err := s.clinicKindsFor(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	where, args := searchWhere(p.ClinicID, r.URL.Query().Get("q"))
	where, args = onlyActiveGiros(where, args, kinds)
	if r.URL.Query().Get("archived") == "1" {
		where += " AND archived_at IS NOT NULL"
	} else {
		where += " AND archived_at IS NULL"
	}
	if r.URL.Query().Get("pending") == "1" {
		where += " AND (incomplete OR privacy_notice_at IS NULL)"
	}
	if list := s.patientQuery(w, r, where, args, 500); list != nil {
		writeJSON(w, http.StatusOK, map[string]any{"patients": list})
	}
}

// lookupPatients is the minimum the front desk needs to book a visit: name and contact, no clinical data.
func (s *Server) lookupPatients(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	kinds, err := s.clinicKindsFor(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	where, args := searchWhere(p.ClinicID, r.URL.Query().Get("q"))
	where, args = onlyActiveGiros(where, args, kinds)
	where += " AND archived_at IS NULL"
	list := s.patientQuery(w, r, where, args, 12)
	if list == nil {
		return
	}
	for i := range list {
		list[i].LastEncounterAt = nil // the visit history is not the front desk's business (the species helps tell pets apart)
	}
	writeJSON(w, http.StatusOK, map[string]any{"patients": list})
}

func (s *Server) logAccess(ctx context.Context, clinicID, patientID string, p *Principal, action string) {
	_, _ = s.db.Exec(ctx, `INSERT INTO record_access (clinic_id, patient_id, user_id, user_name, action) VALUES ($1,$2,$3,$4,$5)`,
		clinicID, patientID, p.UserID, p.actorName(), action)
}

func (s *Server) getPatient(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	x, err := loadPatient(r.Context(), s.db, p.ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.logAccess(r.Context(), p.ClinicID, id, p, "view")
	writeJSON(w, http.StatusOK, map[string]any{"patient": x})
}

// ---------------------------------------------------------------------------
// Create / update / archive
// ---------------------------------------------------------------------------

func (s *Server) createPatient(quick bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in patientIn
		if !decode(w, r, &in) {
			return
		}
		p := principalFrom(r.Context())
		kinds, err := s.clinicKindsFor(r.Context(), p.ClinicID)
		if err != nil {
			serverError(w, r, err)
			return
		}
		if msg := in.validateCore(kinds, quick); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		profile := map[string]any{}
		if !quick {
			var msg string
			if profile, msg = cleanValues(profileFields(in.Subject, kinds), in.Profile, true); msg != "" {
				writeError(w, http.StatusBadRequest, msg)
				return
			}
			if !in.PrivacyAck {
				writeError(w, http.StatusBadRequest, "Confirma que el paciente recibió el aviso de privacidad y aceptó el tratamiento de sus datos.")
				return
			}
		}
		patID := newRowID()
		profileJSON, err := encProfile(patID, profile)
		if err != nil {
			serverError(w, r, err)
			return
		}
		var created patient
		err = inTx(r.Context(), s.db, func(tx pgx.Tx) error {
			var n int
			if err := tx.QueryRow(r.Context(), `UPDATE clinics SET patient_seq = patient_seq + 1 WHERE id = $1 RETURNING patient_seq`, p.ClinicID).Scan(&n); err != nil {
				return err
			}
			var ack any
			who := ""
			if in.PrivacyAck {
				ack, who = time.Now(), p.actorName()
			}
			ownerID, err := resolveOwner(r.Context(), tx, p.ClinicID, &in)
			if err != nil {
				return err
			}
			row := tx.QueryRow(r.Context(), `
				INSERT INTO patients (clinic_id, file_number, subject, names, last_names, sex, birth_date, curp, phone, email, address,
					guardian_name, guardian_relation, guardian_phone, guardian_email, profile, incomplete, privacy_notice_at, privacy_notice_by, created_by, id, owner_id, kinds)
				VALUES ($1,$2,$3,$4,$5,$6,nullif($7,'')::date,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21::uuid,$22::uuid,$23)
				RETURNING `+patientCols,
				p.ClinicID, n, in.Subject, in.Names, in.LastNames, in.Sex, in.BirthDate, in.CURP, in.Phone, in.Email, in.Address,
				in.GuardianName, in.GuardianRelation, in.GuardianPhone, in.GuardianEmail, profileJSON, quick, ack, who, p.actorName(), patID, ownerID, patientKindsFor(in.Subject, kinds))
			if created, err = scanPatient(row); err != nil {
				return err
			}
			audit(r.Context(), tx, p.ClinicID, p, "patient_created", "Registró al paciente #"+itoa(n)+" "+in.Names, map[string]any{"patient": created.ID})
			return nil
		})
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "Ya existe un paciente con esa CURP.")
			return
		}
		if err != nil {
			writeFailure(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"patient": created})
	}
}

func (s *Server) updatePatient(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	var in patientIn
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	cur, err := loadPatient(r.Context(), s.db, p.ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	in.Subject = cur.Subject // a person never becomes an animal
	kinds, err := s.clinicKindsFor(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if msg := in.validateCore(kinds, false); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	profile, msg := cleanValues(profileFields(in.Subject, kinds), in.Profile, true)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	profileJSON, err := encProfile(id, profile)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var ack any
	who := cur.PrivacyNoticeBy
	if cur.PrivacyNoticeAt != nil {
		ack = *cur.PrivacyNoticeAt
	} else if in.PrivacyAck {
		ack, who = time.Now(), p.actorName()
	}
	ownerID, err := resolveOwner(r.Context(), s.db, p.ClinicID, &in)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	row := s.db.QueryRow(r.Context(), `
		UPDATE patients SET names=$3, last_names=$4, sex=$5, birth_date=nullif($6,'')::date, curp=$7, phone=$8, email=$9, address=$10,
			guardian_name=$11, guardian_relation=$12, guardian_phone=$13, guardian_email=$14, profile=$15, incomplete=false,
			privacy_notice_at=$16, privacy_notice_by=$17, owner_id=$18::uuid, updated_at=now()
		WHERE clinic_id=$1 AND id=$2 RETURNING `+patientCols,
		p.ClinicID, id, in.Names, in.LastNames, in.Sex, in.BirthDate, in.CURP, in.Phone, in.Email, in.Address,
		in.GuardianName, in.GuardianRelation, in.GuardianPhone, in.GuardianEmail, profileJSON, ack, who, ownerID)
	out, err := scanPatient(row)
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "Ya existe un paciente con esa CURP.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "patient_updated", "Actualizó los datos del paciente #"+itoa(out.FileNumber), map[string]any{"patient": id})
	writeJSON(w, http.StatusOK, map[string]any{"patient": out})
}

// recordPrivacy notes that the patient has now received the aviso de privacidad.
func (s *Server) recordPrivacy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	tag, err := s.db.Exec(r.Context(), `UPDATE patients SET privacy_notice_at = now(), privacy_notice_by = $3, updated_at = now() WHERE clinic_id=$1 AND id=$2 AND privacy_notice_at IS NULL`, p.ClinicID, id, p.actorName())
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusConflict, "El paciente no existe o ya tenía el aviso de privacidad registrado.")
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "patient_privacy", "Registró el aviso de privacidad de un paciente", map[string]any{"patient": id})
	x, _ := loadPatient(r.Context(), s.db, p.ClinicID, id)
	writeJSON(w, http.StatusOK, map[string]any{"patient": x})
}

// archivePatient takes someone off the active list. Clinical records are kept (NOM-004-SSA3-2012: at least
// five years from the last medical act), so there is deliberately no way to delete a patient.
func (s *Server) archivePatient(archive bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if !validUUID(id) {
			writeError(w, http.StatusNotFound, "Paciente no encontrado.")
			return
		}
		var req struct {
			Reason string `json:"reason"`
		}
		if archive && !decode(w, r, &req) {
			return
		}
		req.Reason = strings.TrimSpace(req.Reason)
		if archive && (req.Reason == "" || utf8.RuneCountInString(req.Reason) > 200) {
			writeError(w, http.StatusBadRequest, "Escribe el motivo (máximo 200 caracteres).")
			return
		}
		p := principalFrom(r.Context())
		var tag int64
		var err error
		if archive {
			t, e := s.db.Exec(r.Context(), `UPDATE patients SET archived_at=now(), archived_by=$3, archive_reason=$4, updated_at=now() WHERE clinic_id=$1 AND id=$2 AND archived_at IS NULL`, p.ClinicID, id, p.actorName(), req.Reason)
			tag, err = t.RowsAffected(), e
		} else {
			t, e := s.db.Exec(r.Context(), `UPDATE patients SET archived_at=NULL, archived_by='', archive_reason='', updated_at=now() WHERE clinic_id=$1 AND id=$2 AND archived_at IS NOT NULL`, p.ClinicID, id)
			tag, err = t.RowsAffected(), e
		}
		if err != nil {
			serverError(w, r, err)
			return
		}
		if tag == 0 {
			writeError(w, http.StatusConflict, "El paciente no existe o ya estaba en ese estado.")
			return
		}
		msg := map[bool]string{true: "Archivó un expediente: " + req.Reason, false: "Reactivó un expediente"}[archive]
		audit(r.Context(), s.db, p.ClinicID, p, "patient_archived", msg, map[string]any{"patient": id})
		x, _ := loadPatient(r.Context(), s.db, p.ClinicID, id)
		writeJSON(w, http.StatusOK, map[string]any{"patient": x})
	}
}

// patientAccess lists who opened or printed a record (traceability, NOM-024-SSA3-2012 and ARCO requests).
func (s *Server) patientAccess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `SELECT user_name, action, created_at FROM record_access WHERE clinic_id=$1 AND patient_id=$2 ORDER BY created_at DESC LIMIT 200`, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	type row struct {
		User   string    `json:"user"`
		Action string    `json:"action"`
		At     time.Time `json:"at"`
	}
	out := []row{}
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.User, &x.Action, &x.At); err != nil {
			serverError(w, r, err)
			return
		}
		out = append(out, x)
	}
	writeJSON(w, http.StatusOK, map[string]any{"access": out})
}

// patientRecord is the whole expediente for printing or for an ARCO access request. Private notes of
// other people are left out, and the access is logged.
func (s *Server) patientRecord(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	pat, err := loadPatient(r.Context(), s.db, p.ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	encs, err := s.visibleEncounters(r.Context(), p, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	kept := encs[:0]
	for _, e := range encs {
		if !e.Hidden {
			kept = append(kept, e)
		}
	}
	rxs, err := s.prescriptionsOf(r.Context(), p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	iss, err := s.issuerInfo(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.logAccess(r.Context(), p.ClinicID, id, p, "print")
	writeJSON(w, http.StatusOK, map[string]any{
		"patient": pat, "encounters": kept, "prescriptions": rxs, "clinic": iss,
		"printed_by": p.actorName(), "generated_at": time.Now(),
	})
}
