package api

import "github.com/go-chi/chi/v5"

// mountNotifications: In-app notifications and waitlist (staff side).
func (s *Server) mountNotifications(r chi.Router) {
	s.mountNotificationsInbox(r)
	s.mountWaitlistStaff(r)

	// Agenda extras that belong to this package (the agenda's own router is mounted elsewhere).
	r.With(require(PermNavAppointments, PermAdminAppointments)).Get("/agenda/month-load", s.agendaMonthLoad)
	r.With(require(PermAdminUsers)).Get("/agenda/booking-check", s.bookingCheck)
	r.With(require(PermAdminUsers)).Put("/agenda/services/{id}/duration", s.setServiceDuration)
}
