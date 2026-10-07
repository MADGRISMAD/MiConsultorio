package api

import "github.com/go-chi/chi/v5"

// mountPublic Unauthenticated routes (booking, verification, portal login).
func (s *Server) mountPublic(r chi.Router) {}
