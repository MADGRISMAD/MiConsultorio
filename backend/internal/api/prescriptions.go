package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Recetas. What a Mexican prescription needs (Ley General de Salud y su reglamento de insumos): the
// prescriber's name, title, cédula profesional and the school that issued it, the establishment's address,
// the date, the patient, the medicine by its generic name with dose, route, frequency and duration, and
// the prescriber's own signature. We print all of it and leave the signature line; Caresia does not sign.

var rxRoutes = []string{"Oral", "Sublingual", "Tópica", "Oftálmica", "Ótica", "Nasal", "Inhalada", "Rectal", "Vaginal", "Intramuscular", "Intravenosa", "Subcutánea", "Otra"}

// Control category of a medicine. Fracción I and II (estupefacientes, psicotrópicos) need the special
// prescription with a COFEPRIS barcode, which cannot be issued from here.
var rxControl = []string{"No", "Antibiótico", "Fracción III", "Fracción I o II"}

type rxItem struct {
	Medicine     string `json:"medicine"` // denominación genérica
	Brand        string `json:"brand"`
	Presentation string `json:"presentation"`
	Dose         string `json:"dose"`
	Route        string `json:"route"`
	Frequency    string `json:"frequency"`
	Duration     string `json:"duration"`
	Quantity     string `json:"quantity"`
	Notes        string `json:"notes"`
	Control      string `json:"control"`
	// Optional, for the weight-based dose check: the catalog entry and the numbers behind Dose.
	CatalogID   string  `json:"catalog_id,omitempty"`
	DoseMg      float64 `json:"dose_mg,omitempty"`       // mg per intake
	DosesPerDay float64 `json:"doses_per_day,omitempty"` // intakes per day
}

type prescription struct {
	ID                     string     `json:"id"`
	PatientID              string     `json:"patient_id"`
	EncounterID            *string    `json:"encounter_id"`
	Folio                  int        `json:"folio"`
	Mode                   string     `json:"mode"`
	IssuedAt               time.Time  `json:"issued_at"`
	ValidUntil             *string    `json:"valid_until"`
	Diagnosis              string     `json:"diagnosis"`
	Items                  []rxItem   `json:"items"`
	Instructions           string     `json:"instructions"`
	NextVisit              *string    `json:"next_visit"`
	AuthorName             string     `json:"author_name"`
	AuthorTitle            string     `json:"author_title"`
	AuthorLicense          string     `json:"author_license"`
	AuthorInstitution      string     `json:"author_institution"`
	AuthorSpecialtyLicense string     `json:"author_specialty_license"`
	VoidedAt               *time.Time `json:"voided_at"`
	VoidedBy               string     `json:"voided_by"`
	VoidReason             string     `json:"void_reason"`
	WeightKg               *float64   `json:"weight_kg"`
	AllergyOverrideReason  string     `json:"allergy_override_reason,omitempty"`
	DoseOverrideReason     string     `json:"dose_override_reason,omitempty"`
}

const rxCols = `id, patient_id::text, encounter_id::text, folio, mode, issued_at, to_char(valid_until,'YYYY-MM-DD'), diagnosis, items, instructions,
	to_char(next_visit,'YYYY-MM-DD'), author_name, author_title, author_license, author_institution, author_specialty_license, voided_at, voided_by, void_reason,
	weight_kg::float8, allergy_override_reason, dose_override_reason`

func scanRx(row pgx.Row) (prescription, error) {
	var x prescription
	var raw []byte
	err := row.Scan(&x.ID, &x.PatientID, &x.EncounterID, &x.Folio, &x.Mode, &x.IssuedAt, &x.ValidUntil, &x.Diagnosis, &raw, &x.Instructions,
		&x.NextVisit, &x.AuthorName, &x.AuthorTitle, &x.AuthorLicense, &x.AuthorInstitution, &x.AuthorSpecialtyLicense, &x.VoidedAt, &x.VoidedBy, &x.VoidReason,
		&x.WeightKg, &x.AllergyOverrideReason, &x.DoseOverrideReason)
	if err != nil {
		return x, err
	}
	x.Items = []rxItem{}
	_ = json.Unmarshal(raw, &x.Items)
	if err := decFields("prescriptions", x.ID, rxSealed, &x.Diagnosis, &x.Instructions); err != nil {
		return prescription{}, err
	}
	return x, nil
}

// rxSealed are the encrypted columns of prescriptions (items stay readable: verification and allergy checks use them).
var rxSealed = []string{"diagnosis", "instructions"}

func (s *Server) listPrescriptions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	clinicID := principalFrom(r.Context()).ClinicID
	if _, err := s.patientExists(r.Context(), clinicID, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		} else {
			serverError(w, r, err)
		}
		return
	}
	list, err := s.prescriptionsOf(r.Context(), clinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"prescriptions": list})
}

func (s *Server) prescriptionsOf(ctx context.Context, clinicID, patientID string) ([]prescription, error) {
	rows, err := s.db.Query(ctx, `SELECT `+rxCols+` FROM prescriptions WHERE clinic_id=$1 AND patient_id=$2 ORDER BY issued_at DESC LIMIT 500`, clinicID, patientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []prescription{}
	for rows.Next() {
		x, err := scanRx(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

type rxIn struct {
	EncounterID  string   `json:"encounter_id"`
	Diagnosis    string   `json:"diagnosis"`
	Items        []rxItem `json:"items"`
	Instructions string   `json:"instructions"`
	NextVisit    string   `json:"next_visit"`
	ValidDays    int      `json:"valid_days"`
	// Patient weight (kg) for the dose check, and the reasons to go on despite an allergy or dose warning.
	WeightKg              float64 `json:"weight_kg"`
	AllergyOverrideReason string  `json:"allergy_override_reason"`
	DoseOverrideReason    string  `json:"dose_override_reason"`
}

func (s *Server) createPrescription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	var in rxIn
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	subject, err := s.patientExists(r.Context(), p.ClinicID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	kinds, err := s.clinicKindsFor(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	mode := rxModeFor(kinds)
	if subject == "animal" {
		mode = "medication"
	}

	// The prescriber must be identifiable: name, title, cédula and school.
	var title, license, institution, specLicense string
	if err := s.db.QueryRow(r.Context(), `SELECT specialty_title, cedula, cedula_institution, cedula_specialty FROM users WHERE id = $1`, p.UserID).Scan(&title, &license, &institution, &specLicense); err != nil {
		serverError(w, r, err)
		return
	}
	if license == "" || institution == "" {
		writeJSON(w, http.StatusConflict, errorBody{Code: "CEDULA_REQUIRED", Message: "Para emitir recetas registra tu cédula profesional y la institución que expidió tu título en Mi cuenta."})
		return
	}
	if title == "" {
		title = map[string]string{"medication": "Médico", "instructions": "Profesional de la salud"}[mode]
	}

	in.Diagnosis, in.Instructions = strings.TrimSpace(in.Diagnosis), strings.TrimSpace(in.Instructions)
	if utf8.RuneCountInString(in.Diagnosis) > 300 || utf8.RuneCountInString(in.Instructions) > 3000 {
		writeError(w, http.StatusBadRequest, "Uno de los textos es demasiado largo.")
		return
	}
	var next any
	if in.NextVisit != "" {
		if _, err := time.Parse("2006-01-02", in.NextVisit); err != nil {
			writeError(w, http.StatusBadRequest, "La fecha de la próxima cita no es válida.")
			return
		}
		next = in.NextVisit
	}
	days := in.ValidDays
	if days == 0 {
		days = 30
	}
	if days < 1 || days > 30 {
		writeError(w, http.StatusBadRequest, "La vigencia máxima de una receta es de 30 días.")
		return
	}
	valid := time.Now().AddDate(0, 0, days).Format("2006-01-02")

	items := []rxItem{}
	if mode == "medication" {
		if len(in.Items) == 0 || len(in.Items) > 15 {
			writeError(w, http.StatusBadRequest, "Agrega de 1 a 15 medicamentos.")
			return
		}
		for i, it := range in.Items {
			for _, f := range []*string{&it.Medicine, &it.Brand, &it.Presentation, &it.Dose, &it.Route, &it.Frequency, &it.Duration, &it.Quantity, &it.Notes, &it.Control} {
				*f = strings.TrimSpace(*f)
			}
			if it.Control == "" {
				it.Control = "No"
			}
			n := "Medicamento " + itoa(i+1) + ": "
			switch {
			case it.Medicine == "" || utf8.RuneCountInString(it.Medicine) > 120:
				writeError(w, http.StatusBadRequest, n+"escribe la denominación genérica (el nombre de la sustancia).")
				return
			case it.Dose == "" || it.Frequency == "" || it.Duration == "":
				writeError(w, http.StatusBadRequest, n+"indica dosis, frecuencia y duración del tratamiento.")
				return
			case !slices.Contains(rxRoutes, it.Route):
				writeError(w, http.StatusBadRequest, n+"elige la vía de administración.")
				return
			case !slices.Contains(rxControl, it.Control):
				writeError(w, http.StatusBadRequest, n+"categoría de control no válida.")
				return
			case it.Control == "Fracción I o II":
				writeJSON(w, http.StatusUnprocessableEntity, errorBody{Code: "CONTROLLED", Message: n + "los estupefacientes y psicotrópicos de fracción I y II requieren la receta especial con código de barras de COFEPRIS. Caresia no la emite: haz esa receta en el formato oficial."})
				return
			case it.DoseMg < 0 || it.DoseMg > 100000 || it.DosesPerDay < 0 || it.DosesPerDay > 48:
				writeError(w, http.StatusBadRequest, n+"la dosis en mg o las tomas al día no son válidas.")
				return
			case utf8.RuneCountInString(it.CatalogID) > 60:
				writeError(w, http.StatusBadRequest, n+"medicamento de catálogo no válido.")
				return
			case utf8.RuneCountInString(it.Brand) > 120 || utf8.RuneCountInString(it.Presentation) > 100 || utf8.RuneCountInString(it.Dose) > 120 ||
				utf8.RuneCountInString(it.Frequency) > 100 || utf8.RuneCountInString(it.Duration) > 100 || utf8.RuneCountInString(it.Quantity) > 60 || utf8.RuneCountInString(it.Notes) > 300:
				writeError(w, http.StatusBadRequest, n+"uno de los campos es demasiado largo.")
				return
			}
			items = append(items, it)
		}
	} else if in.Instructions == "" || len(in.Items) > 0 {
		writeError(w, http.StatusBadRequest, "Escribe las indicaciones para el paciente. Este giro no receta medicamentos.")
		return
	}
	in.AllergyOverrideReason, in.DoseOverrideReason = strings.TrimSpace(in.AllergyOverrideReason), strings.TrimSpace(in.DoseOverrideReason)
	if in.WeightKg < 0 || in.WeightKg > 2000 || utf8.RuneCountInString(in.AllergyOverrideReason) > 300 || utf8.RuneCountInString(in.DoseOverrideReason) > 300 {
		writeError(w, http.StatusBadRequest, "El peso o el motivo no son válidos.")
		return
	}
	if len(items) > 0 {
		pat, err := loadPatient(r.Context(), s.db, p.ClinicID, id)
		if err != nil {
			serverError(w, r, err)
			return
		}
		if conflicts := allergyConflicts(patientAllergies(pat.Profile), items); len(conflicts) > 0 && in.AllergyOverrideReason == "" {
			writeJSON(w, http.StatusConflict, map[string]any{"code": "ALLERGY_CONFLICT", "message": "El paciente tiene alergias registradas que coinciden con la receta. Revisa o escribe el motivo para continuar.", "conflicts": conflicts})
			return
		}
		var warns []doseWarning
		for _, it := range items {
			if it.CatalogID == "" {
				continue
			}
			if m, ok := s.lookupMed(r.Context(), p.ClinicID, it.CatalogID); ok {
				if wn, over := doseExceeds(m, in.WeightKg, it.DoseMg, it.DosesPerDay); over {
					warns = append(warns, wn)
				}
			}
		}
		if len(warns) > 0 && in.DoseOverrideReason == "" {
			writeJSON(w, http.StatusConflict, map[string]any{"code": "DOSE_WARNING", "message": "La dosis diaria supera el máximo de referencia por kilo. Revisa o escribe el motivo para continuar.", "warnings": warns})
			return
		}
	}
	var weight any
	if in.WeightKg > 0 {
		weight = in.WeightKg
	}
	var enc any
	if in.EncounterID != "" {
		if !validUUID(in.EncounterID) {
			writeError(w, http.StatusBadRequest, "La consulta no es válida.")
			return
		}
		enc = in.EncounterID
	}
	raw, _ := json.Marshal(items)
	rxID, sealedDx, sealedIns := newRowID(), in.Diagnosis, in.Instructions
	if err := encFields("prescriptions", rxID, rxSealed, &sealedDx, &sealedIns); err != nil {
		serverError(w, r, err)
		return
	}
	var out prescription
	err = inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var folio int
		if err := tx.QueryRow(r.Context(), `UPDATE clinics SET rx_seq = rx_seq + 1 WHERE id = $1 RETURNING rx_seq`, p.ClinicID).Scan(&folio); err != nil {
			return err
		}
		row := tx.QueryRow(r.Context(), `
			INSERT INTO prescriptions (clinic_id, patient_id, encounter_id, folio, mode, valid_until, diagnosis, items, instructions, next_visit,
				author_id, author_name, author_title, author_license, author_institution, author_specialty_license,
				verify_token, weight_kg, allergy_override_reason, dose_override_reason, id)
			VALUES ($1,$2,$3,$4,$5,$6::date,$7,$8,$9,$10::date,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21::uuid) RETURNING `+rxCols,
			p.ClinicID, id, enc, folio, mode, valid, sealedDx, raw, sealedIns, next, p.UserID, p.actorName(), title, license, institution, specLicense,
			newVerifyToken(), weight, in.AllergyOverrideReason, in.DoseOverrideReason, rxID)
		var err error
		if out, err = scanRx(row); err != nil {
			return err
		}
		meta := map[string]any{"patient": id}
		if in.AllergyOverrideReason != "" {
			meta["allergy_override_reason"] = in.AllergyOverrideReason
		}
		if in.DoseOverrideReason != "" {
			meta["dose_override_reason"] = in.DoseOverrideReason
		}
		audit(r.Context(), tx, p.ClinicID, p, "prescription", "Emitió la receta #"+itoa(folio), meta)
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"prescription": out})
}

func (s *Server) voidPrescription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Receta no encontrada.")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" || utf8.RuneCountInString(req.Reason) > 200 {
		writeError(w, http.StatusBadRequest, "Escribe el motivo de la cancelación (máximo 200 caracteres).")
		return
	}
	p := principalFrom(r.Context())
	// Only the prescriber or an administrator can void; the record stays.
	tag, err := s.db.Exec(r.Context(), `
		UPDATE prescriptions SET voided_at = now(), voided_by = $3, void_reason = $4
		WHERE clinic_id = $1 AND id = $2 AND voided_at IS NULL AND (author_id = $5 OR $6)`,
		p.ClinicID, id, p.actorName(), req.Reason, p.UserID, p.Role == RoleAdmin)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusConflict, "La receta no existe, ya estaba cancelada o no la emitiste tú.")
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "prescription_void", "Canceló una receta: "+req.Reason, nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// printData is everything the receta sheet needs in one call.
func (s *Server) prescriptionPrintData(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Receta no encontrada.")
		return
	}
	p := principalFrom(r.Context())
	rx, err := scanRx(s.db.QueryRow(r.Context(), `SELECT `+rxCols+` FROM prescriptions WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Receta no encontrada.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	pat, err := loadPatient(r.Context(), s.db, p.ClinicID, rx.PatientID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	issuer, err := s.issuerInfo(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var token string
	if err := s.db.QueryRow(r.Context(), `SELECT verify_token FROM prescriptions WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id).Scan(&token); err != nil {
		serverError(w, r, err)
		return
	}
	s.logAccess(r.Context(), p.ClinicID, pat.ID, p, "print")
	writeJSON(w, http.StatusOK, map[string]any{"prescription": rx, "patient": pat, "clinic": issuer, "verify_url": s.verifyURL(token)})
}
