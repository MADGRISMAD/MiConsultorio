package api

import "github.com/go-chi/chi/v5"

// mountPortalPublic: Patient portal. Everything here is public: the sign-in routes carry no session and
// the rest need the portal cookie (requirePortal), which is a different credential from a staff session.
func (s *Server) mountPortalPublic(r chi.Router) {
	r.Post("/portal/{slug}/code", s.portalRequestCode)
	r.Post("/portal/{slug}/login", s.portalLogin)
	r.Get("/portal/{slug}/info", s.portalInfo)
	r.Post("/portal/logout", s.portalLogout)

	r.Group(func(r chi.Router) {
		r.Use(s.requirePortal)
		r.Get("/portal/me", s.portalMe)
		r.Get("/portal/appointments", s.portalAppointments)
		r.Post("/portal/appointments/{id}/cancel", s.portalCancelAppointment)
		r.Get("/portal/prescriptions", s.portalPrescriptions)
		r.Get("/portal/vaccinations", s.portalVaccinations)
		r.Get("/portal/nutrition-plans", s.portalNutritionPlans)
		r.Get("/portal/history", s.portalHistory)
	})
}

// mountPortalAdmin: the clinic's own portal settings (inside the staff session group).
func (s *Server) mountPortalAdmin(r chi.Router) {
	r.With(require(PermAdminUsers)).Get("/clinic/portal", s.getPortalSettings)
	r.With(require(PermAdminUsers)).Put("/clinic/portal", s.updatePortalSettings)
}
