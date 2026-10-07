package api

import "github.com/go-chi/chi/v5"

// mountAgenda Routes inside /appointments (status, move, confirm...).
func (s *Server) mountAgenda(r chi.Router) {}

// mountAgendaRoot Routes of /agenda (professionals, availability, blocks, settings).
func (s *Server) mountAgendaRoot(r chi.Router) {}
