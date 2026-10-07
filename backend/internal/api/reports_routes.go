package api

import "github.com/go-chi/chi/v5"

// mountReports Clinical and operational reports (the financial ones live under /pos/reports).
func (s *Server) mountReports(r chi.Router) {
	r.Route("/reports", func(r chi.Router) {
		r.With(require(PermAdminUsers, PermNavHistorials)).Get("/operations", s.rptOperations)
		r.With(require(PermAdminUsers, PermNavHistorials)).Get("/operations.csv", s.rptOperationsCSV)
		r.With(require(PermAdminUsers)).Get("/patients", s.rptPatients)
		r.With(require(PermAdminUsers)).Get("/patients.csv", s.rptPatientsCSV)
		r.With(require(PermAdminUsers)).Get("/patients/inactive.csv", s.rptInactiveCSV)
	})
}
