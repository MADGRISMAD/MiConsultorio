package api

import "github.com/go-chi/chi/v5"

// mountPublic Unauthenticated routes (booking, verification, portal login).
func (s *Server) mountPublic(r chi.Router) {
	s.mountPublicBooking(r)  // booking page data, confirm/cancel links
	s.mountPublicRx(r)       // receta verification by QR
	s.mountPortalPublic(r)   // patient portal: login and session routes
	s.mountPublicArco(r)     // ARCO request form
	s.mountPublicWaitlist(r) // waitlist sign-up and offers
}
