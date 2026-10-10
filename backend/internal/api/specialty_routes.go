package api

import "github.com/go-chi/chi/v5"

// mountSpecialty Vaccination, odontogram, body map, treatment plans, consents.
func (s *Server) mountSpecialty(r chi.Router) {
	clinical := require(PermNavHistorials, PermAdminHistorials)
	write := require(PermAdminHistorials)

	r.With(clinical).Get("/patients/{id}/vaccinations", s.listVaccinations)
	r.With(write).Post("/patients/{id}/vaccinations", s.createVaccination)
	r.With(clinical).Get("/patients/{id}/weights", s.patientWeights)
	r.With(write).Post("/vaccinations/{id}/void", s.voidVaccination)
	r.With(require(PermNavHistorials, PermAdminHistorials, PermAdminAppointments)).Get("/vaccinations/due", s.dueVaccinations)

	r.With(clinical).Get("/patients/{id}/chronic-meds", s.listChronicMeds)
	r.With(write).Post("/patients/{id}/chronic-meds", s.createChronicMed)
	r.With(write).Patch("/patients/{id}/chronic-meds/{mid}", s.updateChronicMed)
	r.With(clinical).Post("/patients/{id}/rx-check", s.rxCheck)
	r.With(clinical).Post("/patients/{id}/ai-summary", s.consultSummaryAI)

	r.With(clinical).Get("/patients/{id}/charts", s.listCharts)
	r.With(write).Post("/patients/{id}/charts", s.createChart)
	r.With(write).Post("/patients/{id}/nutrition-plan/ai", s.nutritionPlanAI)
	r.With(write).Post("/patients/{id}/nutrition-plan/ai/fragment", s.nutritionFragmentAI)
	r.With(clinical).Post("/patients/{id}/nutrition-plan/calc", s.nutritionCalc)
	// any professional who writes clinical notes (or manages the agenda) can recommend the next visit
	r.With(require(PermAdminHistorials, PermAdminAppointments)).Post("/patients/{id}/follow-up", s.recommendFollowUp)

	r.With(clinical).Get("/patients/{id}/plans", s.listPlans)
	r.With(write).Post("/patients/{id}/plans", s.createPlan)
	// The POS reads a plan to charge it, so cashiers can see it too.
	r.With(require(PermNavHistorials, PermAdminHistorials, PermPOS)).Get("/plans/{id}", s.getPlan)
	r.With(write).Put("/plans/{id}", s.updatePlan)
	r.With(write).Post("/plans/{id}/propose", s.proposePlan)
	r.With(write).Post("/plans/{id}/accept", s.acceptPlan)
	r.With(write).Post("/plans/{id}/cancel", s.cancelPlan)
	r.With(write).Post("/plans/{id}/items", s.addPlanItems)
	r.With(write).Post("/plans/{id}/items/{itemId}/done", s.markItem("done"))
	r.With(write).Post("/plans/{id}/items/{itemId}/cancel", s.markItem("cancel"))
	r.With(require(PermPOS)).Post("/plans/{id}/link-sale", s.linkPlanSale)

	r.With(clinical).Get("/patients/{id}/consents", s.listConsents)
	r.With(write).Post("/patients/{id}/consents", s.createConsent)
	r.With(clinical).Get("/consents/{id}", s.getConsent)
}
