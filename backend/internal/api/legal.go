package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"
)

// Legal is the data of the establishment that Mexican rules ask to see on prescriptions and in the aviso de privacidad.
type Legal struct {
	ResponsibleName        string `json:"responsible_name"`        // responsable sanitario
	ResponsibleLicense     string `json:"responsible_license"`     // su cédula profesional
	ResponsibleInstitution string `json:"responsible_institution"` // institución que expidió el título
	OperatingNotice        string `json:"operating_notice"`        // folio del aviso de funcionamiento o licencia sanitaria
	PrivacyContact         string `json:"privacy_contact"`         // quien atiende los derechos ARCO
	PrivacyEmail           string `json:"privacy_email"`
	PrivacyPhone           string `json:"privacy_phone"`
	PrivacyAddress         string `json:"privacy_address"`
}

func (l *Legal) clean() string {
	for _, f := range []*string{&l.ResponsibleName, &l.ResponsibleLicense, &l.ResponsibleInstitution, &l.OperatingNotice, &l.PrivacyContact, &l.PrivacyEmail, &l.PrivacyPhone, &l.PrivacyAddress} {
		*f = strings.TrimSpace(*f)
		if utf8.RuneCountInString(*f) > 200 {
			return "Uno de los campos es demasiado largo."
		}
	}
	if l.PrivacyEmail != "" && (!strings.Contains(l.PrivacyEmail, "@") || strings.ContainsAny(l.PrivacyEmail, " \r\n")) {
		return "El correo de privacidad no es válido."
	}
	return ""
}

func loadLegal(ctx context.Context, q queryRower, clinicID string) (Legal, error) {
	var raw []byte
	if err := q.QueryRow(ctx, `SELECT legal FROM clinics WHERE id = $1`, clinicID).Scan(&raw); err != nil {
		return Legal{}, err
	}
	var l Legal
	_ = json.Unmarshal(raw, &l)
	return l, nil
}

func (s *Server) getLegal(w http.ResponseWriter, r *http.Request) {
	l, err := loadLegal(r.Context(), s.db, principalFrom(r.Context()).ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"legal": l})
}

func (s *Server) updateLegal(w http.ResponseWriter, r *http.Request) {
	var l Legal
	if !decode(w, r, &l) {
		return
	}
	if msg := l.clean(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	p := principalFrom(r.Context())
	raw, _ := json.Marshal(l)
	if _, err := s.db.Exec(r.Context(), `UPDATE clinics SET legal = $2, updated_at = now() WHERE id = $1`, p.ClinicID, raw); err != nil {
		serverError(w, r, err)
		return
	}
	audit(r.Context(), s.db, p.ClinicID, p, "legal_updated", "Actualizó los datos legales del consultorio", nil)
	writeJSON(w, http.StatusOK, map[string]any{"legal": l})
}

// issuerInfo is the establishment block printed on recetas and expedientes.
type issuer struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Phone   string `json:"phone"`
	Kind    string `json:"kind"`
	Legal   Legal  `json:"legal"`
	// ArcoSlug is the short id of the public ARCO form (/arco/<slug>).
	ArcoSlug string `json:"arco_slug"`
}

func (s *Server) issuerInfo(ctx context.Context, clinicID string) (issuer, error) {
	c, err := loadClinic(ctx, s.db, clinicID)
	if err != nil {
		return issuer{}, err
	}
	l, err := loadLegal(ctx, s.db, clinicID)
	if err != nil {
		return issuer{}, err
	}
	if l.PrivacyAddress == "" {
		l.PrivacyAddress = c.Address
	}
	var arco string
	_ = s.db.QueryRow(ctx, `SELECT arco_code FROM clinics WHERE id = $1`, clinicID).Scan(&arco)
	return issuer{Name: c.Name, Address: c.Address, Phone: c.PhoneNumber, Kind: c.Kind, Legal: l, ArcoSlug: arco}, nil
}

// ---------------------------------------------------------------------------
// Compliance checklist (México): what is in place and what is still missing
// ---------------------------------------------------------------------------

type complianceItem struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"` // ok | todo | info
	Detail string `json:"detail"`
	Link   string `json:"link,omitempty"`
}

func (s *Server) compliance(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	ctx := r.Context()
	kinds, err := s.clinicKindsFor(ctx, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	l, err := loadLegal(ctx, s.db, p.ClinicID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	var pros, withLicense, patients, noNotice, incomplete int
	_ = s.db.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE cedula <> '' AND cedula_institution <> '') FROM users WHERE clinic_id=$1 AND role IN ('doctor','admin') AND NOT disabled`, p.ClinicID).Scan(&pros, &withLicense)
	_ = s.db.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE privacy_notice_at IS NULL), count(*) FILTER (WHERE incomplete) FROM patients WHERE clinic_id=$1 AND archived_at IS NULL`, p.ClinicID).Scan(&patients, &noNotice, &incomplete)

	arco, _ := arcoLoadPending(ctx, s.db, p.ClinicID)
	arcoSum := arcoSummarize(arco)

	st := func(ok bool) string {
		if ok {
			return "ok"
		}
		return "todo"
	}
	items := []complianceItem{
		{Key: "responsible", Label: "Responsable sanitario", Status: st(l.ResponsibleName != "" && l.ResponsibleLicense != ""),
			Detail: "Nombre y cédula profesional de quien responde por el establecimiento (aparece en las recetas).", Link: "/ajustes?s=cumplimiento"},
		{Key: "operating_notice", Label: "Aviso de funcionamiento", Status: st(l.OperatingNotice != ""),
			Detail: "Folio del aviso de funcionamiento o de la licencia sanitaria ante COFEPRIS / la autoridad sanitaria de tu estado.", Link: "/ajustes?s=cumplimiento"},
		{Key: "privacy_contact", Label: "Aviso de privacidad y derechos ARCO", Status: st(l.PrivacyContact != "" && l.PrivacyEmail != ""),
			Detail: "Quién atiende las solicitudes de acceso, rectificación, cancelación y oposición, y a qué correo.", Link: "/ajustes?s=cumplimiento"},
		{Key: "licenses", Label: "Cédula de cada profesional", Status: st(pros > 0 && withLicense == pros),
			Detail: itoa(withLicense) + " de " + itoa(pros) + " profesionales registraron su cédula profesional e institución (sin ella no pueden emitir recetas).", Link: "/cuenta"},
		{Key: "patient_notice", Label: "Aviso de privacidad entregado a cada paciente", Status: st(noNotice == 0),
			Detail: itoa(noNotice) + " de " + itoa(patients) + " pacientes activos no tienen registrado el aviso de privacidad.", Link: "/pacientes?pendientes=1"},
		arcoComplianceItem(arcoSum),
		{Key: "complete_files", Label: "Expedientes completos", Status: st(incomplete == 0),
			Detail: itoa(incomplete) + " pacientes se dieron de alta de forma rápida y les faltan antecedentes.", Link: "/pacientes?pendientes=1"},
		{Key: "retention", Label: "Conservación del expediente (5 años)", Status: "ok",
			Detail: "Caresia no permite borrar pacientes ni editar la bitácora: solo archivar y agregar adendas. La NOM-004-SSA3-2012 pide conservar el expediente al menos 5 años desde el último acto médico."},
		{Key: "access_log", Label: "Bitácora de accesos", Status: "ok",
			Detail: "Se registra quién abre o imprime cada expediente. El administrador puede consultarla desde el expediente."},
	}
	if rxModeFor(kinds) == "instructions" {
		items = append(items, complianceItem{Key: "rx_mode", Label: "Recetas", Status: "info",
			Detail: "Tu giro no receta medicamentos: Caresia emite hojas de indicaciones, no recetas de fármacos."})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// arcoComplianceItem is the checklist line for ARCO requests: overdue ones are a to-do, open ones just information.
func arcoComplianceItem(sum arcoSummary) complianceItem {
	it := complianceItem{Key: "arco_requests", Label: "Solicitudes ARCO", Link: "/arco-solicitudes"}
	switch {
	case sum.Overdue > 0:
		it.Status = "todo"
		it.Detail = itoa(sum.Overdue) + " solicitud(es) ARCO con plazo vencido y " + itoa(sum.Open) + " abierta(s) en total. Verifica los plazos con tu asesor legal."
	case sum.Open > 0:
		it.Status = "info"
		it.Detail = itoa(sum.Open) + " solicitud(es) ARCO abierta(s), " + itoa(sum.Soon) + " por vencer. Verifica los plazos con tu asesor legal."
	default:
		it.Status = "ok"
		it.Detail = "Sin solicitudes ARCO abiertas. Las personas pueden enviarlas desde el formulario público del consultorio."
	}
	return it
}
