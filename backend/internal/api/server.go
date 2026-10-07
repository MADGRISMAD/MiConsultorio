// Package api implements the Caresia HTTP API.
package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/mail"
)

type Server struct {
	db      *pgxpool.Pool
	cfg     *config.Config
	limiter *rateLimiter // failed logins
	signups *rateLimiter // registrations per IP

	mailer      mail.Sender  // outgoing e-mail; nil or disabled when SMTP is not configured
	forgots     *rateLimiter // recovery requests per IP and per account
	mailLimiter *rateLimiter // tickets e-mailed per user

	portal *portalLimits // patient portal: code requests and failed sign-ins
}

// NewRouter builds the API with the SMTP sender described by cfg.
func NewRouter(db *pgxpool.Pool, cfg *config.Config) http.Handler {
	return NewRouterWithMailer(db, cfg, &mail.SMTP{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.MailFrom, Secure: cfg.SMTPSecure})
}

// NewApp is NewRouter plus the function that starts the background workers (reminders, cleanups).
func NewApp(db *pgxpool.Pool, cfg *config.Config) (http.Handler, func(context.Context)) {
	mailer := &mail.SMTP{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.MailFrom, Secure: cfg.SMTPSecure}
	s := newServer(db, cfg, mailer)
	return s.router(), s.StartBackground
}

// NewRouterWithMailer is NewRouter with an explicit mail sender (tests use a fake).
func NewRouterWithMailer(db *pgxpool.Pool, cfg *config.Config, mailer mail.Sender) http.Handler {
	return newServer(db, cfg, mailer).router()
}

func newServer(db *pgxpool.Pool, cfg *config.Config, mailer mail.Sender) *Server {
	return &Server{db: db, cfg: cfg, limiter: newRateLimiter(8, 15*time.Minute), signups: newRateLimiter(5, time.Hour),
		mailer: mailer, forgots: newRateLimiter(5, time.Hour), mailLimiter: newRateLimiter(30, time.Hour), portal: newPortalLimits()}
}

func (s *Server) router() http.Handler {
	cfg := s.cfg
	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.Recoverer, securityHeaders)

	r.Route("/api", func(r chi.Router) {
		r.Use(s.sameOriginOnly)
		r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		r.Post("/login", s.login)
		r.Post("/register", s.register)
		r.Post("/logout", s.logout)
		r.Post("/forgot", s.forgotPassword)
		r.Post("/reset-password", s.resetPassword)
		// Called by Mercado Pago, not by a browser session: authenticity comes from the signature and a re-fetch.
		r.Post("/webhooks/mercadopago", s.mpWebhook)
		r.Post("/point/webhook", s.mpWebhook) // same handler, the URL MiTiendita uses
		r.Get("/point/oauth/callback", s.pointCallback)
		s.mountPublic(r) // no session: online booking, receta verification, patient portal login

		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/session", s.session)
			r.Put("/me", s.updateProfile)
			r.Put("/me/password", s.changeOwnPassword)
			s.mountSecurity(r) // two-step verification endpoints

			// ---- Clinic accounts: data is blocked while the subscription is not active ----
			r.Group(func(r chi.Router) {
				r.Use(requireClinic)
				r.Get("/clinic", s.clinicInfo)

				// Paying for the plan must work even when the subscription has lapsed.
				r.Route("/billing", func(r chi.Router) {
					r.Use(require(PermAdminUsers))
					r.Get("/", s.billingOverview)
					r.Post("/checkout", s.billingCheckout)
					r.Get("/checkouts/{id}", s.billingCheckoutStatus)
				})

				r.Group(func(r chi.Router) {
					r.Use(s.requireSubscription)

					r.With(require(PermAdminUsers)).Put("/clinic", s.updateOwnClinic)
					r.With(require(PermAdminUsers)).Post("/clinic/setup", s.completeSetup)
					r.Get("/clinic/legal", s.getLegal)
					r.With(require(PermAdminUsers)).Put("/clinic/legal", s.updateLegal)
					r.With(require(PermAdminUsers)).Get("/clinic/compliance", s.compliance)

					r.Route("/team", func(r chi.Router) {
						r.Use(require(PermAdminUsers))
						r.Get("/", s.listTeam)
						r.Post("/", s.createMember)
						r.Patch("/{id}", s.updateMember)
						r.Post("/{id}/password", s.setMemberPassword)
						r.Post("/{id}/deactivate", s.deactivateMember)
						r.Post("/{id}/reactivate", s.reactivateMember)
					})

					r.Route("/pos", func(r chi.Router) {
						r.Use(requireCobros)
						r.With(require(PermPOS)).Get("/settings", s.getPosSettings)
						r.With(require(PermPOSManage)).Put("/settings", s.updatePosSettings)

						r.With(require(PermPOS)).Get("/items", s.listCatalog)
						r.Group(func(r chi.Router) {
							r.Use(require(PermPOSManage))
							r.Post("/items", s.createCatalogItem)
							r.Post("/items/import", s.importCatalog)
							r.Put("/items/{id}", s.updateCatalogItem)
							r.Delete("/items/{id}", s.deleteCatalogItem)
							r.Post("/items/{id}/stock", s.adjustStock)
							r.Get("/items/{id}/movements", s.listStockMovements)
						})

						r.With(require(PermPOS)).Post("/sales", s.createSale)
						r.With(require(PermPOS)).Get("/sales", s.listSales)
						r.With(require(PermPOS)).Get("/sales/{id}", s.getSale)
						r.With(require(PermPOSManage)).Post("/sales/{id}/void", s.voidSale)
						r.With(require(PermPOS)).Post("/sales/{id}/email", s.emailSale)

						r.With(require(PermPOS)).Get("/cash/current", s.currentCash)
						r.With(require(PermPOS)).Post("/cash/open", s.openCash)
						r.With(require(PermPOS)).Post("/cash/movements", s.cashMovementCreate)
						r.With(require(PermPOS)).Post("/cash/close", s.closeCash)
						r.With(require(PermPOSReports)).Get("/cash/sessions", s.listCashSessions)
						r.With(require(PermPOSReports)).Get("/cash/sessions/{id}", s.getCashSession)

						r.With(require(PermPOSReports)).Get("/reports", s.posReport)
						r.With(require(PermPOSReports)).Get("/reports/sales.csv", s.exportSales)

						r.With(require(PermPOSReports)).Get("/invoices", s.listInvoices)
						r.With(require(PermPOS)).Post("/invoices", s.createInvoice)
						r.With(require(PermPOSReports)).Patch("/invoices/{id}", s.updateInvoice)

						r.With(require(PermPOSManage)).Get("/point/connect", s.pointConnect)
						r.With(require(PermPOSManage)).Post("/point/disconnect", s.pointDisconnect)
						r.With(require(PermPOS)).Get("/point/status", s.pointStatus)
						r.With(require(PermPOSManage)).Get("/point/terminals", s.pointTerminals)
						r.With(require(PermPOSManage)).Post("/point/terminal", s.pointRegister)
						r.With(require(PermPOS)).Post("/point/charges", s.pointCharge)
						r.With(require(PermPOS)).Post("/mp/links", s.linkCreate)
						r.With(require(PermPOS)).Get("/charges/{id}", s.chargeStatus)
						r.With(require(PermPOS)).Delete("/charges/{id}", s.chargeCancel)

						r.With(require(PermPOSManage)).Get("/magic", s.magicStatus)
						r.With(require(PermPOSManage)).Post("/magic/inventory", s.magicInventory)
						r.With(require(PermPOSManage)).Post("/magic/price", s.magicPrice)
						s.mountPosRoutes(r) // lots, consumables, commissions, abonos, CFDI stamping
					})

					r.Route("/patients", func(r chi.Router) {
						clinical := require(PermNavHistorials, PermAdminHistorials)
						write := require(PermAdminHistorials)
						front := require(PermAdminAppointments, PermNavHistorials, PermAdminHistorials)
						r.With(front).Get("/lookup", s.lookupPatients)
						r.With(front).Get("/schema", s.patientSchema)
						r.With(require(PermAdminAppointments, PermAdminHistorials)).Post("/quick", s.createPatient(true))
						r.With(clinical).Get("/", s.listPatients)
						r.With(write).Post("/", s.createPatient(false))
						r.With(clinical).Get("/{id}", s.getPatient)
						r.With(write).Put("/{id}", s.updatePatient)
						r.With(require(PermAdminAppointments, PermAdminHistorials)).Post("/{id}/privacy", s.recordPrivacy)
						r.With(write).Post("/{id}/archive", s.archivePatient(true))
						r.With(write).Post("/{id}/unarchive", s.archivePatient(false))
						r.With(require(PermAdminUsers)).Get("/{id}/access", s.patientAccess)
						r.With(clinical).Get("/{id}/encounters", s.listEncounters)
						r.With(write).Post("/{id}/encounters", s.createEncounter)
						r.With(clinical).Get("/{id}/prescriptions", s.listPrescriptions)
						r.With(write).Post("/{id}/prescriptions", s.createPrescription)
						r.With(clinical).Get("/{id}/record", s.patientRecord)
					})
					r.With(require(PermAdminHistorials)).Post("/encounters/{id}/addendum", s.createAddendum)
					r.With(require(PermNavHistorials, PermAdminHistorials)).Get("/prescriptions/{id}", s.prescriptionPrintData)
					r.With(require(PermAdminHistorials)).Post("/prescriptions/{id}/void", s.voidPrescription)

					r.Route("/appointments", func(r chi.Router) {
						r.With(require(PermNavAppointments, PermAdminAppointments)).Get("/", s.listAppointments)
						r.With(require(PermNavAppointments, PermAdminAppointments)).Get("/{id}", s.getAppointment)
						r.With(require(PermAdminAppointments)).Post("/", s.createAppointment)
						r.With(require(PermAdminAppointments)).Put("/{id}", s.updateAppointment)
						r.With(require(PermAdminAppointments)).Delete("/{id}", s.deleteAppointment)
						s.mountAgenda(r) // status changes, availability, blocks, settings
					})
					s.mountAgendaRoot(r)
					s.mountFiles(r)
					s.mountSpecialty(r)
					s.mountRx(r)
					s.mountPosV2(r)
					s.mountReports(r)
					s.mountPortalAdmin(r)
				})
			})

			// ---- Platform staff: administrators change things, support only looks ----
			r.Route("/platform", func(r chi.Router) {
				r.Use(requireRoles(RolePlatformAdmin, RolePlatformSupport))
				r.Get("/overview", s.platformOverview)
				r.Get("/plans", s.platformPlans)
				r.Get("/clinics", s.platformClinics)
				r.Get("/clinics/{id}", s.platformClinic)

				r.Group(func(r chi.Router) {
					r.Use(requireRoles(RolePlatformAdmin))
					r.Patch("/clinics/{id}", s.updateClinic)
					r.Post("/clinics/{id}/suspend", s.suspendClinic)
					r.Post("/clinics/{id}/reactivate", s.reactivateClinic)
					r.Post("/clinics/{id}/payments", s.recordPayment)

					r.Get("/activity", s.platformActivity)
					r.Get("/staff", s.listStaff)
					r.Post("/staff", s.createStaff)
					r.Patch("/staff/{id}", s.updateStaff)
					r.Post("/staff/{id}/password", s.setStaffPassword)
					r.Post("/staff/{id}/deactivate", s.setStaffDisabled(true))
					r.Post("/staff/{id}/reactivate", s.setStaffDisabled(false))
				})
			})
		})

		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusNotFound, "Recurso no encontrado.")
		})
	})

	if cfg.StaticDir != "" {
		r.NotFound(spaHandler(cfg.StaticDir))
	}
	return r
}

// spaHandler serves the built frontend, falling back to index.html so client-side routes work.
func spaHandler(dir string) http.HandlerFunc {
	files := http.FileServer(http.Dir(dir))
	return func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Join(dir, filepath.FromSlash(filepath.Clean("/"+r.URL.Path)))
		if info, err := os.Stat(clean); err == nil && !info.IsDir() {
			if strings.HasPrefix(r.URL.Path, "/_app/immutable/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	}
}
