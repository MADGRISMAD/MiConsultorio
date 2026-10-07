package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"image/png"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Digital informed consent. Each row keeps the exact text that was shown, the drawn signature and a SHA-256 that
// ties them to the patient and the moment of signing. Append-only.

var (
	consentKinds = []string{"procedimiento", "plan_tratamiento", "privacidad", "telemedicina", "animal", "psicologia"}
	signerRoles  = []string{"paciente", "tutor", "propietario"}
)

const (
	maxSignatureBytes = 200 << 10
	pngDataPrefix     = "data:image/png;base64,"
	pngMagic          = "\x89PNG\r\n\x1a\n"
)

type consent struct {
	ID               string    `json:"id"`
	PatientID        string    `json:"patient_id"`
	PlanID           *string   `json:"plan_id"`
	Kind             string    `json:"kind"`
	TextSnapshot     string    `json:"text_snapshot,omitempty"`
	SignerName       string    `json:"signer_name"`
	SignerRole       string    `json:"signer_role"`
	SignaturePNG     string    `json:"signature_png,omitempty"`
	Witness1         string    `json:"witness1"`
	Witness2         string    `json:"witness2"`
	ContentSHA256    string    `json:"content_sha256"`
	SignedAt         time.Time `json:"signed_at"`
	IP               string    `json:"ip"`
	UserAgent        string    `json:"user_agent"`
	RegisteredByName string    `json:"registered_by_name"`
}

const consentCols = `id, patient_id::text, plan_id::text, kind, text_snapshot, signer_name, signer_role, signature_png, witness1, witness2,
	content_sha256, signed_at, ip, user_agent, registered_by_name`

func scanConsent(row pgx.Row) (consent, error) {
	var c consent
	err := row.Scan(&c.ID, &c.PatientID, &c.PlanID, &c.Kind, &c.TextSnapshot, &c.SignerName, &c.SignerRole, &c.SignaturePNG, &c.Witness1, &c.Witness2,
		&c.ContentSHA256, &c.SignedAt, &c.IP, &c.UserAgent, &c.RegisteredByName)
	return c, err
}

// checkSignaturePNG validates a data URL holding a real, reasonably small PNG and returns its bytes.
func checkSignaturePNG(s string) ([]byte, string) {
	if !strings.HasPrefix(s, pngDataPrefix) {
		return nil, "La firma debe ser una imagen PNG."
	}
	enc := s[len(pngDataPrefix):]
	if base64.StdEncoding.DecodedLen(len(enc)) > maxSignatureBytes+3 {
		return nil, "La firma pesa demasiado (máximo 200 KB)."
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil || len(raw) == 0 {
		return nil, "La firma no es una imagen válida."
	}
	if len(raw) > maxSignatureBytes {
		return nil, "La firma pesa demasiado (máximo 200 KB)."
	}
	if !bytes.HasPrefix(raw, []byte(pngMagic)) {
		return nil, "La firma no es una imagen PNG válida."
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width < 20 || cfg.Height < 10 || cfg.Width > 4000 || cfg.Height > 4000 {
		return nil, "La firma no es una imagen PNG válida."
	}
	return raw, ""
}

type consentIn struct {
	PlanID       string `json:"plan_id"`
	Kind         string `json:"kind"`
	TextSnapshot string `json:"text_snapshot"`
	SignerName   string `json:"signer_name"`
	SignerRole   string `json:"signer_role"`
	SignaturePNG string `json:"signature_png"`
	Witness1     string `json:"witness1"`
	Witness2     string `json:"witness2"`
}

// validate cleans the input in place and returns a message when it is not acceptable.
func (in *consentIn) validate() (sig []byte, msg string) {
	in.TextSnapshot, in.SignerName = strings.TrimSpace(in.TextSnapshot), strings.TrimSpace(in.SignerName)
	in.Witness1, in.Witness2 = strings.TrimSpace(in.Witness1), strings.TrimSpace(in.Witness2)
	switch {
	case !slices.Contains(consentKinds, in.Kind):
		return nil, "Tipo de consentimiento inválido."
	case in.TextSnapshot == "" || utf8.RuneCountInString(in.TextSnapshot) > 20000:
		return nil, "El texto del consentimiento está vacío o es demasiado largo."
	case in.SignerName == "" || utf8.RuneCountInString(in.SignerName) > 150:
		return nil, "Escribe el nombre de quien firma."
	case !slices.Contains(signerRoles, in.SignerRole):
		return nil, "Indica si firma el paciente, el tutor o el propietario."
	case utf8.RuneCountInString(in.Witness1) > 150 || utf8.RuneCountInString(in.Witness2) > 150:
		return nil, "El nombre de un testigo es demasiado largo."
	}
	return checkSignaturePNG(in.SignaturePNG)
}

func consentHash(patientID string, in consentIn, sig []byte, signedAt time.Time) string {
	h := sha256.New()
	for _, part := range []string{patientID, in.Kind, in.TextSnapshot, in.SignerName, in.SignerRole, signedAt.UTC().Format(time.RFC3339Nano)} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	h.Write(sig)
	return hex.EncodeToString(h.Sum(nil))
}

type consentWriter interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// insertConsent stores an already validated consent.
func insertConsent(ctx context.Context, q consentWriter, p *Principal, patientID string, in consentIn, sig []byte, ip, ua string) (consent, error) {
	signedAt := time.Now().UTC().Truncate(time.Microsecond)
	var plan any
	if in.PlanID != "" {
		plan = in.PlanID
	}
	if len(ua) > 300 {
		ua = ua[:300]
	}
	return scanConsent(q.QueryRow(ctx, `INSERT INTO consent_signatures (clinic_id, patient_id, plan_id, kind, text_snapshot, signer_name, signer_role,
		signature_png, witness1, witness2, content_sha256, signed_at, ip, user_agent, registered_by_name)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING `+consentCols,
		p.ClinicID, patientID, plan, in.Kind, in.TextSnapshot, in.SignerName, in.SignerRole, in.SignaturePNG, in.Witness1, in.Witness2,
		consentHash(patientID, in, sig, signedAt), signedAt, ip, ua, p.actorName()))
}

func (s *Server) createConsent(w http.ResponseWriter, r *http.Request) {
	id, _, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	var in consentIn
	if !decode(w, r, &in) {
		return
	}
	sig, msg := in.validate()
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	if in.PlanID != "" {
		if !validUUID(in.PlanID) {
			writeError(w, http.StatusBadRequest, "El plan no es válido.")
			return
		}
		var found bool
		if err := s.db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM treatment_plans WHERE clinic_id=$1 AND patient_id=$2 AND id=$3)`, p.ClinicID, id, in.PlanID).Scan(&found); err != nil {
			serverError(w, r, err)
			return
		}
		if !found {
			writeError(w, http.StatusBadRequest, "El plan no existe.")
			return
		}
	}
	c, err := insertConsent(r.Context(), s.db, p, id, in, sig, clientIP(r), r.UserAgent())
	if err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "consent_signed", "Registró un consentimiento informado firmado", map[string]any{"patient": id, "kind": in.Kind})
	c.SignaturePNG = ""
	writeJSON(w, http.StatusCreated, map[string]any{"consent": c})
}

func (s *Server) listConsents(w http.ResponseWriter, r *http.Request) {
	id, _, _, ok := s.patientForSpecialty(w, r)
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	rows, err := s.db.Query(r.Context(), `SELECT `+consentCols+` FROM consent_signatures WHERE clinic_id=$1 AND patient_id=$2 ORDER BY signed_at DESC LIMIT 500`, p.ClinicID, id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []consent{}
	for rows.Next() {
		c, err := scanConsent(rows)
		if err != nil {
			serverError(w, r, err)
			return
		}
		c.SignaturePNG, c.TextSnapshot = "", "" // heavy: fetched one by one to print
		out = append(out, c)
	}
	if rows.Err() != nil {
		serverError(w, r, rows.Err())
		return
	}
	s.logAccess(r.Context(), p.ClinicID, id, p, "view")
	writeJSON(w, http.StatusOK, map[string]any{"consents": out})
}

// getConsent returns everything needed to print a signed consent.
func (s *Server) getConsent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validUUID(id) {
		writeError(w, http.StatusNotFound, "Consentimiento no encontrado.")
		return
	}
	p := principalFrom(r.Context())
	c, err := scanConsent(s.db.QueryRow(r.Context(), `SELECT `+consentCols+` FROM consent_signatures WHERE clinic_id=$1 AND id=$2`, p.ClinicID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Consentimiento no encontrado.")
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	var patientName string
	_ = s.db.QueryRow(r.Context(), `SELECT trim(names || ' ' || last_names) FROM patients WHERE clinic_id=$1 AND id=$2`, p.ClinicID, c.PatientID).Scan(&patientName)
	s.logAccess(r.Context(), p.ClinicID, c.PatientID, p, "view")
	writeJSON(w, http.StatusOK, map[string]any{"consent": c, "patient_name": patientName})
}
