package api

import "github.com/go-chi/chi/v5"

// mountLab: laboratory orders/results and growth charts (see lab.go, growth.go).
func (s *Server) mountLab(r chi.Router) {
	clinical := require(PermNavHistorials, PermAdminHistorials)
	write := require(PermAdminHistorials)

	r.With(clinical).Get("/lab/catalog", s.labCatalog)
	r.With(clinical).Get("/patients/{id}/lab/orders", s.listLabOrders)
	r.With(write).Post("/patients/{id}/lab/orders", s.createLabOrder)
	r.With(clinical).Get("/patients/{id}/lab-trends", s.labTrends)
	r.With(clinical).Get("/lab/orders/{id}", s.getLabOrder)
	r.With(write).Put("/lab/orders/{id}", s.updateLabOrder)
	r.With(write).Post("/lab/orders/{id}/status", s.setLabOrderStatus)
	r.With(write).Post("/lab/orders/{id}/results", s.addLabResults)

	r.With(clinical).Get("/patients/{id}/growth", s.patientGrowth)
	// Reference tables are loaded by the administrator only.
	r.With(require(PermAdminUsers)).Get("/growth/references", s.listGrowthImports)
	r.With(require(PermAdminUsers)).Post("/growth/references/import", s.importGrowthReferences)
}
