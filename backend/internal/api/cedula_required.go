package api

import "net/http"

// requireCedula stops a professional who has not registered their cédula profesional from issuing a document that carries
// their name (a meal plan, like a receta, is signed by whoever writes it). It answers and returns false when it is missing.
func (s *Server) requireCedula(w http.ResponseWriter, r *http.Request, what string) bool {
	p := principalFrom(r.Context())
	if p.Cedula != "" {
		return true
	}
	var ced string
	if err := s.db.QueryRow(r.Context(), `SELECT cedula FROM users WHERE id = $1`, p.UserID).Scan(&ced); err != nil {
		serverError(w, r, err)
		return false
	}
	if ced != "" {
		return true
	}
	writeJSON(w, http.StatusConflict, errorBody{Code: "CEDULA_REQUIRED", Message: "Para " + what + " registra tu cédula profesional en Mi cuenta."})
	return false
}

// credentials are the data that identify who signs a receta or a certificate.
type credentials struct{ Title, License, Institution, SpecLicense string }

// requireSignerCredentials loads the cédula, the school and the title of the person signing and answers 409 CEDULA_REQUIRED
// when the cédula or the institution is missing. what is the document ("recetas", "certificados").
func (s *Server) requireSignerCredentials(w http.ResponseWriter, r *http.Request, what string) (credentials, bool) {
	var c credentials
	if err := s.db.QueryRow(r.Context(), `SELECT specialty_title, cedula, cedula_institution, cedula_specialty FROM users WHERE id = $1`, principalFrom(r.Context()).UserID).
		Scan(&c.Title, &c.License, &c.Institution, &c.SpecLicense); err != nil {
		serverError(w, r, err)
		return c, false
	}
	if c.License == "" || c.Institution == "" {
		writeJSON(w, http.StatusConflict, errorBody{Code: "CEDULA_REQUIRED", Message: "Para emitir " + what + " registra tu cédula profesional y la institución que expidió tu título en Mi cuenta."})
		return c, false
	}
	return c, true
}
