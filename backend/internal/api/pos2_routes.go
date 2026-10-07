package api

import "github.com/go-chi/chi/v5"

// mountPosV2 mounts the Cobros v2 routes that live outside /pos: the supplies used in an encounter.
// Both need a plan with cobros (stock belongs to it).
func (s *Server) mountPosV2(r chi.Router) {
	r.With(requireCobros, require(PermAdminHistorials)).Post("/encounters/{id}/consumables", s.recordEncounterConsumables)
	r.With(requireCobros, require(PermAdminHistorials, PermNavHistorials)).Get("/encounters/{id}/consumables", s.listEncounterConsumables)
}

// mountPosRoutes adds Cobros v2 to the /pos subtree (already behind requireCobros).
func (s *Server) mountPosRoutes(r chi.Router) {
	// lots, alerts, consumables
	r.With(require(PermPOSManage)).Get("/items/{id}/lots", s.listItemLots)
	r.With(require(PermPOS)).Get("/alerts", s.posAlerts)
	r.With(require(PermPOS)).Get("/items/{id}/consumables", s.getConsumables)
	r.With(require(PermPOSManage)).Put("/items/{id}/consumables", s.setConsumables)

	// professionals and commissions
	r.With(require(PermPOS)).Get("/professionals", s.posProfessionals)
	r.With(require(PermPOSManage, PermPOSReports)).Get("/commission-rules", s.listCommissionRules)
	r.With(require(PermPOSManage)).Post("/commission-rules", s.saveCommissionRule(true))
	r.With(require(PermPOSManage)).Put("/commission-rules/{id}", s.saveCommissionRule(false))
	r.With(require(PermPOSManage)).Delete("/commission-rules/{id}", s.deleteCommissionRule)
	r.With(require(PermPOSReports)).Get("/reports/commissions", s.commissionReport)
	r.With(require(PermPOSReports)).Get("/reports/commissions.csv", s.commissionCSV)

	// sales on account
	r.With(require(PermPOS)).Post("/sales/{id}/payments", s.addSalePayments)
	r.With(require(PermPOS)).Get("/receivables", s.listReceivables)

	// CFDI stamping
	r.With(require(PermPOSReports, PermPOSManage)).Post("/invoices/{id}/stamp", s.stampInvoice)
	r.With(require(PermPOSManage)).Post("/invoices/{id}/cancel-cfdi", s.cancelCFDI)
	r.With(require(PermPOSReports, PermPOSManage)).Get("/invoices/{id}/xml", s.downloadCFDI("xml"))
	r.With(require(PermPOSReports, PermPOSManage)).Get("/invoices/{id}/pdf", s.downloadCFDI("pdf"))
}
