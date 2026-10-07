package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// mountConsultBilling: Pre-account (services and supplies used) inside a consultation.
// Professionals (adminHistorials) build and send it; the register (pos) reads the sent ones and charges them
// through POST /pos/sales with consult_charge_id. Plans without cobros keep only the unpriced list.
func (s *Server) mountConsultBilling(r chi.Router) {
	pro := require(PermAdminHistorials)
	either := require(PermAdminHistorials, PermPOS)
	r.With(either).Get("/consult-charges", s.cbList)
	r.With(pro, requireCobros).Get("/consult-charges/catalog", s.cbCatalog)
	r.With(either).Get("/consult-charges/{id}", s.cbGet)
	r.With(pro).Post("/consult-charges", s.cbCreate)
	r.With(pro).Put("/consult-charges/{id}", s.cbUpdate)
	r.With(pro).Post("/consult-charges/{id}/send", func(w http.ResponseWriter, r *http.Request) { s.cbTransition(w, r, "sent") })
	r.With(pro).Post("/consult-charges/{id}/cancel", func(w http.ResponseWriter, r *http.Request) { s.cbTransition(w, r, "cancelled") })
}
