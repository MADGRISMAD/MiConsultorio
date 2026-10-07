package api

import "github.com/go-chi/chi/v5"

// mountFiles Attachment routes (see files.go).
func (s *Server) mountFiles(r chi.Router) { s.mountFilesRoutes(r) }
