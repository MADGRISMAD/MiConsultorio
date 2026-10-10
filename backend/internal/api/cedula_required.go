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
