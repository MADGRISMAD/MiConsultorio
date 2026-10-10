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

// Certificados. The medical one (people) and the veterinary one (animals) state what the professional found on
// examining the patient and what it is for. Like the receta, it carries the professional's name, title and cédula,
// the establishment, a folio and a QR that proves it is real without showing its content. The professional signs it
// by hand. It is not a sick-leave note (incapacidad) and it is not an official health certificate for moving animals
// (SENASICA): the printed sheet says so.

var certPurposes = map[string][]string{
	"medical":    {"Escolar", "Laboral", "Deportivo", "Viaje", "Trámite", "Buen estado de salud", "Otro"},
	"veterinary": {"Salud general", "Viaje nacional", "Viaje internacional", "Vacunación y desparasitación", "Esterilización", "Pensión o guardería", "Exposición o concurso", "Otro"},
}

var certAptitudes = []string{"", "apto", "no_apto", "restricciones"}

type certificate struct {
	ID          string         `json:"id"`
	PatientID   string         `json:"patient_id"`
	Kind        string         `json:"kind"`
	Folio       int            `json:"folio"`
	IssuedAt    time.Time      `json:"issued_at"`
	ValidUntil  *string        `json:"valid_until"`
	Purpose     string         `json:"purpose"`
	Statement   string         `json:"statement"`
	Findings    string         `json:"findings"`
	Extra       map[string]any `json:"extra"`
	AuthorName  string         `json:"author_name"`
	AuthorTitle string         `json:"author_title"`
	AuthorLic   string         `json:"author_license"`
	AuthorInst  string         `json:"author_institution"`
	AuthorSpec  string         `json:"author_specialty_license"`
	VoidedAt    *time.Time     `json:"voided_at"`
	VoidedBy    string         `json:"voided_by"`
	VoidReason  string         `json:"void_reason"`
}

const certCols = `id::text, patient_id::text, kind, folio, issued_at, to_char(valid_until,'YYYY-MM-DD'), purpose, statement, findings, extra, author_name, author_title,
	author_license, author_institution, author_specialty_license, voided_at, voided_by, void_reason`

var certSealed = []string{"statement", "findings"}

func scanCert(row pgx.Row) (certificate, error) {
	var c certificate
	var raw []byte
	if err := row.Scan(&c.ID, &c.PatientID, &c.Kind, &c.Folio, &c.IssuedAt, &c.ValidUntil, &c.Purpose, &c.Statement, &c.Findings, &raw, &c.AuthorName, &c.AuthorTitle,
		&c.AuthorLic, &c.AuthorInst, &c.AuthorSpec, &c.VoidedAt, &c.VoidedBy, &c.VoidReason); err != nil {
		return c, err
	}
	c.Extra = map[string]any{}
	_ = json.Unmarshal(raw, &c.Extra)
	if err := decFields("certificates", c.ID, certSealed, &c.Statement, &c.Findings); err != nil {
		return certificate{}, err
	}
	return c, nil
}

func (s *Server) listCertificates(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	if _, err := s.patientExists(r.Context(), p.ClinicID, id); err != nil {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	out, err := s.certificatesOf(r.Context(), p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"certificates": out, "purposes": certPurposes})
}

type certIn struct {
	Purpose      string `json:"purpose"`
	Statement    string `json:"statement"`
	Findings     string `json:"findings"`
	Aptitude     string `json:"aptitude"`
	Restrictions string `json:"restrictions"`
	ValidDays    int    `json:"valid_days"`  // 0: no expiry
	Destination  string `json:"destination"` // veterinary: where the animal goes
	Microchip    string `json:"microchip"`   // veterinary
	Vaccines     string `json:"vaccines"`    // veterinary: vaccines and deworming up to date, as text
}

func (s *Server) createCertificate(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	if !validUUID(pid) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	var in certIn
	if !decode(w, r, &in) {
		return
	}
	p := principalFrom(r.Context())
	subject, err := s.patientExists(r.Context(), p.ClinicID, pid)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Paciente no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	kind := "medical"
	if subject == "animal" {
		kind = "veterinary"
	}
	for _, f := range []*string{&in.Purpose, &in.Statement, &in.Findings, &in.Restrictions, &in.Destination, &in.Microchip, &in.Vaccines} {
		*f = strings.TrimSpace(*f)
	}
	switch {
	case !slices.Contains(certPurposes[kind], in.Purpose):
		writeError(w, http.StatusBadRequest, "Elige para qué es el certificado.")
		return
	case in.Statement == "":
		writeError(w, http.StatusBadRequest, "Escribe el texto del certificado.")
		return
	case utf8.RuneCountInString(in.Statement) > 3000 || utf8.RuneCountInString(in.Findings) > 1500 || utf8.RuneCountInString(in.Restrictions) > 500 ||
		utf8.RuneCountInString(in.Destination) > 200 || utf8.RuneCountInString(in.Microchip) > 40 || utf8.RuneCountInString(in.Vaccines) > 1000:
		writeError(w, http.StatusBadRequest, "Uno de los textos es demasiado largo.")
		return
	case !slices.Contains(certAptitudes, in.Aptitude) || (kind == "veterinary" && in.Aptitude != ""):
		writeError(w, http.StatusBadRequest, "La aptitud no es válida.")
		return
	case in.ValidDays < 0 || in.ValidDays > 365:
		writeError(w, http.StatusBadRequest, "La vigencia máxima es de 365 días.")
		return
	}
	var title, license, institution, specLicense string
	if err := s.db.QueryRow(r.Context(), `SELECT specialty_title, cedula, cedula_institution, cedula_specialty FROM users WHERE id = $1`, p.UserID).Scan(&title, &license, &institution, &specLicense); err != nil {
		serverError(w, r, err)
		return
	}
	if license == "" || institution == "" {
		writeJSON(w, http.StatusConflict, errorBody{Code: "CEDULA_REQUIRED", Message: "Para emitir certificados registra tu cédula profesional y la institución que expidió tu título en Mi cuenta."})
		return
	}
	if title == "" {
		title = map[string]string{"medical": "Médico", "veterinary": "Médico veterinario zootecnista"}[kind]
	}
	extra := map[string]any{}
	set := func(k, v string) {
		if v != "" {
			extra[k] = v
		}
	}
	set("aptitude", in.Aptitude)
	set("restrictions", in.Restrictions)
	set("destination", in.Destination)
	set("microchip", in.Microchip)
	set("vaccines", in.Vaccines)
	rawExtra, _ := json.Marshal(extra)
	var valid any
	if in.ValidDays > 0 {
		valid = time.Now().AddDate(0, 0, in.ValidDays).Format("2006-01-02")
	}
	certID, sealedStatement, sealedFindings := newRowID(), in.Statement, in.Findings
	if err := encFields("certificates", certID, certSealed, &sealedStatement, &sealedFindings); err != nil {
		serverError(w, r, err)
		return
	}
	var out certificate
	err = inTx(r.Context(), s.db, func(tx pgx.Tx) error {
		var folio int
		if err := tx.QueryRow(r.Context(), `UPDATE clinics SET cert_seq = cert_seq + 1 WHERE id = $1 RETURNING cert_seq`, p.ClinicID).Scan(&folio); err != nil {
			return err
		}
		var err error
		out, err = scanCert(tx.QueryRow(r.Context(), `
			INSERT INTO certificates (id, clinic_id, patient_id, kind, folio, valid_until, purpose, statement, findings, extra, author_id, author_name, author_title,
				author_license, author_institution, author_specialty_license, verify_token)
			VALUES ($1::uuid,$2,$3,$4,$5,$6::date,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) RETURNING `+certCols,
			certID, p.ClinicID, pid, kind, folio, valid, in.Purpose, sealedStatement, sealedFindings, rawExtra, p.UserID, p.actorName(), title, license, institution, specLicense, newVerifyToken()))
		if err != nil {
			return err
		}
		audit(r.Context(), tx, p.ClinicID, p, "certificate", "Emitió el certificado #"+itoa(folio), map[string]any{"patient": pid, "kind": kind, "purpose": in.Purpose})
		return nil
	})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"certificate": out})
}

func (s *Server) certificatePrintData(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Certificado no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	var token string
	c, err := scanCert(s.db.QueryRow(r.Context(), `SELECT `+certCols+` FROM certificates WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Certificado no encontrado.")
		return
	}
	if err == nil {
		err = s.db.QueryRow(r.Context(), `SELECT verify_token FROM certificates WHERE clinic_id = $1 AND id = $2`, p.ClinicID, id).Scan(&token)
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	pat, err := loadPatient(r.Context(), s.db, p.ClinicID, c.PatientID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	issuer, err := s.issuerInfo(r.Context(), p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.logAccess(r.Context(), p.ClinicID, pat.ID, p, "print")
	writeJSON(w, http.StatusOK, map[string]any{"certificate": c, "patient": pat, "clinic": issuer, "verify_url": s.verifyURL(token)})
}

func (s *Server) voidCertificate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Certificado no encontrado.")
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
	tag, err := s.db.Exec(r.Context(), `
		UPDATE certificates SET voided_at = now(), voided_by = $3, void_reason = $4
		WHERE clinic_id = $1 AND id = $2 AND voided_at IS NULL AND (author_id = $5 OR $6)`,
		p.ClinicID, id, p.actorName(), req.Reason, p.UserID, p.Role == RoleAdmin)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusConflict, "El certificado no existe, ya estaba cancelado o no lo emitiste tú.")
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "certificate_void", "Canceló un certificado: "+req.Reason, nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) mountCertificates(r chi.Router) {
	clinical := require(PermNavHistorials, PermAdminHistorials)
	write := require(PermAdminHistorials)
	r.With(clinical).Get("/patients/{id}/certificates", s.listCertificates)
	r.With(write).Post("/patients/{id}/certificates", s.createCertificate)
	r.With(clinical).Get("/certificates/{id}", s.certificatePrintData)
	r.With(write).Post("/certificates/{id}/void", s.voidCertificate)
}

// certificatesOf lists the certificates of a patient, newest first.
func (s *Server) certificatesOf(ctx context.Context, clinicID, patientID string) ([]certificate, error) {
	rows, err := s.db.Query(ctx, `SELECT `+certCols+` FROM certificates WHERE clinic_id = $1 AND patient_id = $2 ORDER BY issued_at DESC LIMIT 500`, clinicID, patientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []certificate{}
	for rows.Next() {
		c, err := scanCert(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
