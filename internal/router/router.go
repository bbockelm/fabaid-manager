package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/fabaid-manager/internal/config"
	"github.com/bbockelm/fabaid-manager/internal/crypto"
	"github.com/bbockelm/fabaid-manager/internal/db"
	"github.com/bbockelm/fabaid-manager/internal/frontend"
	"github.com/bbockelm/fabaid-manager/internal/handlers"
	"github.com/bbockelm/fabaid-manager/internal/openapi"
	"github.com/bbockelm/fabaid-manager/internal/storage"
)

// New creates the application HTTP router with all routes.
// Returns the mux and the handler (so callers can set the backup service).
func New(cfg *config.Config, pool *pgxpool.Pool, store *storage.Store) (*chi.Mux, *handlers.Handler) {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	queries := db.NewQueries(pool)

	// Initialize document encryption (optional in dev)
	var enc *crypto.Encryptor
	if cfg.InstanceKey != "" {
		var err error
		enc, err = crypto.NewEncryptor(cfg.InstanceKey)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize document encryption")
		}
		log.Info().Msg("Document encryption enabled")
	} else {
		log.Warn().Msg("INSTANCE_KEY not set — encrypted document upload disabled")
	}

	h := handlers.New(cfg, queries, store, enc)

	// Serve OpenAPI spec
	r.Get("/api/v1/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(openapi.Spec)
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {

		// --- Public auth endpoints (no session required) ---
		r.Route("/auth", func(r chi.Router) {
			r.Get("/mode", h.GetAuthMode)
			r.Get("/me", h.GetCurrentSession)
			r.Post("/logout", h.Logout)
			r.Post("/dev-login", h.DevLogin)
			r.Get("/oidc/login", h.OIDCLogin)
			r.Get("/oidc/callback", h.OIDCCallback)
		})

		// Profile update: requires auth but NOT write access (read-only users can update their name)
		r.Group(func(r chi.Router) {
			r.Use(h.RequireAuthOrAPIKey)
			r.Put("/auth/profile", h.UpdateMyProfile)
		})

		// --- All other API routes require authentication + write access ---
		r.Group(func(r chi.Router) {
			r.Use(h.RequireAuthOrAPIKey)
			r.Use(handlers.RequireWriteAccess)

			// Grants
			r.Route("/grants", func(r chi.Router) {
				r.Get("/", h.ListGrants)
				// Only admin/grant_admin can create grants
				r.Group(func(r chi.Router) {
					r.Use(handlers.RequireRole(handlers.RoleAdmin, handlers.RoleGrantAdmin))
					r.Post("/", h.CreateGrant)
				})
				r.Route("/{grantID}", func(r chi.Router) {
					r.Get("/", h.GetGrant)
					// Only admin/grant_admin can modify/delete grants
					r.Group(func(r chi.Router) {
						r.Use(handlers.RequireRole(handlers.RoleAdmin, handlers.RoleGrantAdmin))
						r.Put("/", h.UpdateGrant)
						r.Delete("/", h.DeleteGrant)
					})

					// Budget overview (project-wide summary)
					r.Get("/budget-overview", h.BudgetOverview)

					// SOW Config (per-grant)
					r.Get("/sow-config", h.GetSOWConfig)
					// Only admin/grant_admin can modify SOW config
					r.Group(func(r chi.Router) {
						r.Use(handlers.RequireRole(handlers.RoleAdmin, handlers.RoleGrantAdmin))
						r.Put("/sow-config", h.UpsertSOWConfig)
					})

					// WBS Areas
					r.Route("/wbs", func(r chi.Router) {
						r.Get("/", h.ListWBSAreas)
						r.Post("/", h.CreateWBSArea)
						r.Get("/effort-summary", h.WBSEffortSummary)
						r.Get("/{wbsID}", h.GetWBSArea)
						r.Put("/{wbsID}", h.UpdateWBSArea)
						r.Delete("/{wbsID}", h.DeleteWBSArea)
					})

					// Personnel
					r.Route("/personnel", func(r chi.Router) {
						r.Get("/", h.ListPersonnel)
						r.Post("/", h.CreatePersonnel)
						r.Get("/titles", h.ListPersonnelTitles)
						r.Route("/{personnelID}", func(r chi.Router) {
							r.Put("/", h.UpdatePersonnel)
							r.Delete("/", h.DeletePersonnel)
							r.Get("/budget-summary", h.PersonnelBudgetSummary)
							r.Get("/default-wbs", h.ListPersonnelDefaultWBS)
							r.Put("/default-wbs", h.SetPersonnelDefaultWBS)
						})
					})

					// Subawards
					r.Route("/subawards", func(r chi.Router) {
						r.Get("/", h.ListSubawards)
						r.Post("/", h.CreateSubaward)
						r.Route("/{subawardID}", func(r chi.Router) {
							r.Put("/", h.UpdateSubaward)
							r.Delete("/", h.DeleteSubaward)

							// Invoices
							r.Route("/invoices", func(r chi.Router) {
								r.Get("/", h.ListInvoices)
								r.Post("/", h.CreateInvoice)
								r.Patch("/{invoiceID}/status", h.UpdateInvoiceStatus)
							})

							// Upload invoice PDF
							r.Post("/invoices/{invoiceID}/upload", h.UploadInvoiceDocument)

							// Statements of Work
							r.Route("/sow", func(r chi.Router) {
								r.Get("/", h.ListStatementsOfWork)
								r.Post("/", h.CreateStatementOfWork)
								r.Route("/{sowID}", func(r chi.Router) {
									r.Put("/", h.UpdateStatementOfWork)
									r.Delete("/", h.DeleteStatementOfWork)
									r.Post("/upload-signed", h.UploadSignedSOW)
									r.Get("/render", h.RenderSOW)

									// Personnel descriptions
									r.Get("/personnel-descriptions", h.ListSOWPersonnelDescriptions)
									r.Put("/personnel-descriptions", h.UpsertSOWPersonnelDescription)
									r.Delete("/personnel-descriptions/{descID}", h.DeleteSOWPersonnelDescription)

									// Line item descriptions
									r.Get("/line-item-descriptions", h.ListSOWLineItemDescriptions)
									r.Put("/line-item-descriptions", h.UpsertSOWLineItemDescription)
									r.Delete("/line-item-descriptions/{descID}", h.DeleteSOWLineItemDescription)
								})
							})
						})
					})
				})
			})

			// Documents
			r.Get("/documents/{docID}", h.GetDocument)
			r.Get("/documents/{docID}/download", h.DownloadDocument)

			// Institution-scoped endpoints
			r.Route("/institution-rates/{entityType}/{entityID}", func(r chi.Router) {
				// Overhead (F&A) rates
				r.Route("/overhead-rates", func(r chi.Router) {
					r.Get("/", h.ListOverheadRates)
					r.Post("/", h.CreateOverheadRate)
					r.Put("/{overheadRateID}", h.UpdateOverheadRate)
					r.Delete("/{overheadRateID}", h.DeleteOverheadRate)
				})

				// Fringe rates
				r.Get("/fringe-rates", h.ListFringeRates)
				r.Put("/fringe-rates", h.UpsertFringeRate)
				r.Delete("/fringe-rates/{fringeRateID}", h.DeleteFringeRate)

				// Versioned institution budgets
				r.Get("/budgets", h.ListInstitutionBudgets)
				r.Post("/budgets", h.CreateInstitutionBudget)
				r.Post("/budgets/{budgetID}/finalize", h.FinalizeBudget)
				r.Post("/budgets/{budgetID}/duplicate", h.DuplicateBudget)
				r.Delete("/budgets/{budgetID}", h.DeleteInstitutionBudget)

				// Budget line items
				r.Route("/budgets/{budgetID}/line-items", func(r chi.Router) {
					r.Get("/", h.ListBudgetLineItems)
					r.Post("/", h.CreateBudgetLineItem)
					r.Put("/{lineItemID}", h.UpdateBudgetLineItem)
					r.Delete("/{lineItemID}", h.DeleteBudgetLineItem)

					// WBS allocations
					r.Get("/{lineItemID}/wbs", h.ListLineItemWBS)
					r.Put("/{lineItemID}/wbs", h.SetLineItemWBS)
				})

				// NSF Form 1030 budget render
				r.Get("/nsf1030", h.RenderNSF1030)

				// Budget documents (encrypted)
				r.Route("/budget-documents", func(r chi.Router) {
					r.Get("/", h.ListBudgetDocuments)
					r.Post("/", h.UploadBudgetDocument)
					r.Get("/{docID}", h.GetBudgetDocumentInfo)
					r.Get("/{docID}/download", h.DownloadBudgetDocument)
					r.Delete("/{docID}", h.SoftDeleteBudgetDocument)
				})
			})

			// Legacy download endpoint (creates + downloads in one request)
			r.Group(func(r chi.Router) {
				r.Use(handlers.RequireRole(handlers.RoleAdmin, handlers.RoleGrantAdmin))
				r.Get("/backup", h.CreateBackupLegacy)
			})

			// Backup routes — require admin or grant_admin.
			r.Route("/backups", func(r chi.Router) {
				r.Use(handlers.RequireRole(handlers.RoleAdmin, handlers.RoleGrantAdmin))
				r.Get("/", h.ListBackups)

				r.Group(func(r chi.Router) {
					r.Use(handlers.RequireRole(handlers.RoleAdmin))
					r.Post("/trigger", h.TriggerBackup)
					r.Post("/upload-restore", h.UploadRestore)
					r.Delete("/failed", h.DeleteFailedBackups)
					r.Get("/settings", h.GetBackupSettings)
					r.Put("/settings", h.UpdateBackupSettings)
					r.Get("/general-key", h.GetGeneralBackupKey)
					r.Post("/derive-key", h.DeriveKeyFromInput)
					r.Route("/{backupID}", func(r chi.Router) {
						r.Get("/download", h.DownloadBackup)
						r.Get("/key", h.GetPerBackupKey)
						r.Post("/restore", h.RestoreBackup)
						r.Delete("/", h.DeleteBackup)
					})
				})
			})

			// --- Admin / Grant-Admin routes ---
			r.Group(func(r chi.Router) {
				r.Use(handlers.RequireRole(handlers.RoleAdmin, handlers.RoleGrantAdmin))

				// User management
				r.Route("/admin/users", func(r chi.Router) {
					r.Get("/", h.ListUsers)
					r.Post("/", h.CreateUserAccount)
					r.Route("/{userID}", func(r chi.Router) {
						r.Put("/", h.UpdateUserAccount)
						r.Delete("/", h.DeleteUserAccount)
						r.Post("/roles", h.AddUserRoleHandler)
						r.Delete("/roles/{role}", h.RemoveUserRoleHandler)
						r.Delete("/identities/{identityID}", h.RemoveUserIdentityHandler)
						r.Post("/invites", h.CreateInviteHandler)
						r.Get("/invites", h.ListInvitesHandler)
						r.Delete("/invites/{inviteID}", h.DeleteInviteHandler)
						r.Get("/institutions", h.ListUserInstitutionsHandler)
						r.Post("/institutions", h.AddUserInstitutionHandler)
						r.Delete("/institutions/{institution}", h.RemoveUserInstitutionHandler)
					})
				})

				// OIDC config
				r.Get("/admin/oidc-config", h.GetOIDCConfig)
				r.Put("/admin/oidc-config", h.UpdateOIDCConfig)

				// API key management
				r.Route("/admin/api-keys", func(r chi.Router) {
					r.Get("/", h.ListAPIKeys)
					r.Post("/", h.CreateAPIKey)
					r.Post("/{keyID}/revoke", h.RevokeAPIKey)
					r.Delete("/{keyID}", h.DeleteAPIKey)
				})
			})
		})
	})

	// Health check
	r.Get("/healthz", h.HealthCheck)

	// Serve embedded frontend for all non-API routes (production builds only).
	// In development, the separate Next.js dev server handles the frontend.
	if frontend.IsEmbedded() {
		distFS, err := frontend.DistFS()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to access embedded frontend")
		}
		r.NotFound(frontend.NewSPAHandler(distFS))
		log.Info().Msg("Serving embedded frontend from Go binary")
	}

	return r, h
}
