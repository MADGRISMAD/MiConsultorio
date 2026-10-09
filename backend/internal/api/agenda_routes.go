package api

import "github.com/go-chi/chi/v5"

// mountAgenda Routes inside /appointments (status, move, confirm...).
func (s *Server) mountAgenda(r chi.Router) {
	r.With(require(PermNavAppointments, PermAdminAppointments)).Post("/{id}/status", s.changeAppointmentStatus)
}

// mountAgendaRoot Routes of /agenda (professionals, availability, blocks, settings).
func (s *Server) mountAgendaRoot(r chi.Router) {
	r.Route("/agenda", func(r chi.Router) {
		view := require(PermNavAppointments, PermAdminAppointments)
		r.With(view).Get("/professionals", s.listProfessionals)
		r.With(require(PermAdminUsers)).Put("/professionals/{id}", s.updateProfessional)
		r.With(view).Get("/services", s.listAgendaServices)
		r.With(view).Get("/blocks", s.listBlocks)
		// a professional marks their own time off (vacation...); only agenda managers block other people or the whole clinic
		r.With(view).Post("/blocks", s.createBlock)
		r.With(view).Delete("/blocks/{id}", s.deleteBlock)
		r.With(view).Get("/settings", s.getAgendaSettings)
		r.With(require(PermAdminUsers)).Put("/settings", s.updateAgendaSettings)
	})
}
