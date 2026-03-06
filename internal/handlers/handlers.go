package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/fabaid-manager/internal/backup"
	"github.com/bbockelm/fabaid-manager/internal/config"
	"github.com/bbockelm/fabaid-manager/internal/crypto"
	"github.com/bbockelm/fabaid-manager/internal/db"
	"github.com/bbockelm/fabaid-manager/internal/models"
	"github.com/bbockelm/fabaid-manager/internal/storage"
)

// Handler contains dependencies for HTTP handlers.
type Handler struct {
	cfg       *config.Config
	queries   *db.Queries
	store     *storage.Store
	encryptor *crypto.Encryptor // nil if master key not configured
	backupSvc *backup.Service   // nil if not initialized
}

// New creates a new Handler.
func New(cfg *config.Config, queries *db.Queries, store *storage.Store, enc *crypto.Encryptor) *Handler {
	return &Handler{cfg: cfg, queries: queries, store: store, encryptor: enc}
}

// SetBackupService sets the backup service on the handler.
func (h *Handler) SetBackupService(svc *backup.Service) {
	h.backupSvc = svc
}

// --- Helpers ---

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// --- Health Check ---

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Grants ---

func (h *Handler) ListGrants(w http.ResponseWriter, r *http.Request) {
	grants, err := h.queries.ListGrants(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list grants")
		respondError(w, http.StatusInternalServerError, "Failed to list grants")
		return
	}
	if grants == nil {
		grants = []models.Grant{}
	}
	respondJSON(w, http.StatusOK, grants)
}

func (h *Handler) GetGrant(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "grantID")
	grant, err := h.queries.GetGrant(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Grant not found")
		return
	}
	respondJSON(w, http.StatusOK, grant)
}

func (h *Handler) CreateGrant(w http.ResponseWriter, r *http.Request) {
	var g models.Grant
	if err := decodeJSON(r, &g); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if g.Status == "" {
		g.Status = "active"
	}
	if err := h.queries.CreateGrant(r.Context(), &g); err != nil {
		log.Error().Err(err).Msg("Failed to create grant")
		respondError(w, http.StatusInternalServerError, "Failed to create grant")
		return
	}
	respondJSON(w, http.StatusCreated, g)
}

func (h *Handler) UpdateGrant(w http.ResponseWriter, r *http.Request) {
	var g models.Grant
	if err := decodeJSON(r, &g); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	g.ID = chi.URLParam(r, "grantID")
	if err := h.queries.UpdateGrant(r.Context(), &g); err != nil {
		log.Error().Err(err).Msg("Failed to update grant")
		respondError(w, http.StatusInternalServerError, "Failed to update grant")
		return
	}
	respondJSON(w, http.StatusOK, g)
}

func (h *Handler) DeleteGrant(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "grantID")
	if err := h.queries.DeleteGrant(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete grant")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- WBS Areas ---

func (h *Handler) ListWBSAreas(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantID")
	areas, err := h.queries.ListWBSAreas(r.Context(), grantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list WBS areas")
		return
	}
	if areas == nil {
		areas = []models.WBSArea{}
	}
	respondJSON(w, http.StatusOK, areas)
}

func (h *Handler) CreateWBSArea(w http.ResponseWriter, r *http.Request) {
	var a models.WBSArea
	if err := decodeJSON(r, &a); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	a.GrantID = chi.URLParam(r, "grantID")
	if err := h.queries.CreateWBSArea(r.Context(), &a); err != nil {
		log.Error().Err(err).Msg("Failed to create WBS area")
		respondError(w, http.StatusInternalServerError, "Failed to create WBS area")
		return
	}
	respondJSON(w, http.StatusCreated, a)
}

func (h *Handler) UpdateWBSArea(w http.ResponseWriter, r *http.Request) {
	var a models.WBSArea
	if err := decodeJSON(r, &a); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	a.ID = chi.URLParam(r, "wbsID")
	a.GrantID = chi.URLParam(r, "grantID")
	if err := h.queries.UpdateWBSArea(r.Context(), &a); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update WBS area")
		return
	}
	respondJSON(w, http.StatusOK, a)
}

func (h *Handler) DeleteWBSArea(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "wbsID")
	if err := h.queries.DeleteWBSArea(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete WBS area")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetWBSArea(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "wbsID")
	area, err := h.queries.GetWBSArea(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "WBS area not found")
		return
	}
	respondJSON(w, http.StatusOK, area)
}

func (h *Handler) WBSEffortSummary(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantID")
	summaries, err := h.queries.WBSEffortSummary(r.Context(), grantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get WBS effort summary")
		respondError(w, http.StatusInternalServerError, "Failed to get WBS effort summary")
		return
	}
	if summaries == nil {
		summaries = []models.WBSEffortSummary{}
	}
	respondJSON(w, http.StatusOK, summaries)
}

func (h *Handler) ListPersonnelDefaultWBS(w http.ResponseWriter, r *http.Request) {
	personnelID := chi.URLParam(r, "personnelID")
	items, err := h.queries.ListPersonnelDefaultWBS(r.Context(), personnelID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list default WBS")
		respondError(w, http.StatusInternalServerError, "Failed to list default WBS")
		return
	}
	if items == nil {
		items = []models.PersonnelDefaultWBS{}
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *Handler) SetPersonnelDefaultWBS(w http.ResponseWriter, r *http.Request) {
	personnelID := chi.URLParam(r, "personnelID")
	var items []models.PersonnelDefaultWBS
	if err := decodeJSON(r, &items); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	result, err := h.queries.SetPersonnelDefaultWBS(r.Context(), personnelID, items)
	if err != nil {
		log.Error().Err(err).Msg("Failed to set default WBS")
		respondError(w, http.StatusInternalServerError, "Failed to set default WBS")
		return
	}
	if result == nil {
		result = []models.PersonnelDefaultWBS{}
	}
	respondJSON(w, http.StatusOK, result)
}

// --- Personnel ---

func (h *Handler) ListPersonnel(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantID")
	people, err := h.queries.ListPersonnel(r.Context(), grantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list personnel")
		return
	}
	if people == nil {
		people = []models.Personnel{}
	}
	respondJSON(w, http.StatusOK, people)
}

func (h *Handler) CreatePersonnel(w http.ResponseWriter, r *http.Request) {
	var p models.Personnel
	if err := decodeJSON(r, &p); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	p.GrantID = chi.URLParam(r, "grantID")
	if err := h.queries.CreatePersonnel(r.Context(), &p); err != nil {
		log.Error().Err(err).Msg("Failed to create personnel")
		respondError(w, http.StatusInternalServerError, "Failed to create personnel")
		return
	}
	respondJSON(w, http.StatusCreated, p)
}

func (h *Handler) UpdatePersonnel(w http.ResponseWriter, r *http.Request) {
	var p models.Personnel
	if err := decodeJSON(r, &p); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	p.ID = chi.URLParam(r, "personnelID")
	p.GrantID = chi.URLParam(r, "grantID")
	if err := h.queries.UpdatePersonnel(r.Context(), &p); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update personnel")
		return
	}
	respondJSON(w, http.StatusOK, p)
}

func (h *Handler) DeletePersonnel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "personnelID")
	if err := h.queries.DeletePersonnel(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete personnel")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListPersonnelTitles(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantID")
	titles, err := h.queries.ListPersonnelTitles(r.Context(), grantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list titles")
		return
	}
	if titles == nil {
		titles = []string{}
	}
	respondJSON(w, http.StatusOK, titles)
}

func (h *Handler) PersonnelBudgetSummary(w http.ResponseWriter, r *http.Request) {
	personnelID := chi.URLParam(r, "personnelID")
	entries, err := h.queries.PersonnelBudgetSummary(r.Context(), personnelID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get budget summary")
		return
	}
	if entries == nil {
		entries = []models.PersonnelBudgetEntry{}
	}
	respondJSON(w, http.StatusOK, entries)
}

// --- Budget Line Items & Overhead Rates ---
// (Old grant-level budget items / year budgets replaced by institution-scoped line items)

func (h *Handler) ListOverheadRates(w http.ResponseWriter, r *http.Request) {
	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityID")
	items, err := h.queries.ListOverheadRates(r.Context(), entityType, entityID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list overhead rates")
		return
	}
	if items == nil {
		items = []models.OverheadRate{}
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *Handler) CreateOverheadRate(w http.ResponseWriter, r *http.Request) {
	var rate models.OverheadRate
	if err := decodeJSON(r, &rate); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	rate.EntityType = chi.URLParam(r, "entityType")
	rate.EntityID = chi.URLParam(r, "entityID")
	if err := h.queries.CreateOverheadRate(r.Context(), &rate); err != nil {
		log.Error().Err(err).Msg("Failed to create overhead rate")
		respondError(w, http.StatusInternalServerError, "Failed to create overhead rate")
		return
	}
	respondJSON(w, http.StatusCreated, rate)
}

func (h *Handler) UpdateOverheadRate(w http.ResponseWriter, r *http.Request) {
	var rate models.OverheadRate
	if err := decodeJSON(r, &rate); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	rate.ID = chi.URLParam(r, "overheadRateID")
	rate.EntityType = chi.URLParam(r, "entityType")
	rate.EntityID = chi.URLParam(r, "entityID")
	if err := h.queries.UpdateOverheadRate(r.Context(), &rate); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update overhead rate")
		return
	}
	respondJSON(w, http.StatusOK, rate)
}

func (h *Handler) DeleteOverheadRate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "overheadRateID")
	if err := h.queries.DeleteOverheadRate(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete overhead rate")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListBudgetLineItems(w http.ResponseWriter, r *http.Request) {
	budgetID := chi.URLParam(r, "budgetID")
	items, err := h.queries.ListBudgetLineItems(r.Context(), budgetID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list budget line items")
		return
	}
	if items == nil {
		items = []models.BudgetLineItem{}
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *Handler) CreateBudgetLineItem(w http.ResponseWriter, r *http.Request) {
	var b models.BudgetLineItem
	if err := decodeJSON(r, &b); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	b.InstitutionBudgetID = chi.URLParam(r, "budgetID")
	if err := h.queries.CreateBudgetLineItem(r.Context(), &b); err != nil {
		log.Error().Err(err).Msg("Failed to create budget line item")
		respondError(w, http.StatusInternalServerError, "Failed to create budget line item")
		return
	}
	respondJSON(w, http.StatusCreated, b)
}

func (h *Handler) UpdateBudgetLineItem(w http.ResponseWriter, r *http.Request) {
	var b models.BudgetLineItem
	if err := decodeJSON(r, &b); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	b.ID = chi.URLParam(r, "lineItemID")
	b.InstitutionBudgetID = chi.URLParam(r, "budgetID")
	if err := h.queries.UpdateBudgetLineItem(r.Context(), &b); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update budget line item")
		return
	}
	respondJSON(w, http.StatusOK, b)
}

func (h *Handler) DeleteBudgetLineItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "lineItemID")
	if err := h.queries.DeleteBudgetLineItem(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete budget line item")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListLineItemWBS(w http.ResponseWriter, r *http.Request) {
	lineItemID := chi.URLParam(r, "lineItemID")
	items, err := h.queries.ListLineItemWBS(r.Context(), lineItemID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list line item WBS allocations")
		return
	}
	if items == nil {
		items = []models.BudgetLineItemWBS{}
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *Handler) SetLineItemWBS(w http.ResponseWriter, r *http.Request) {
	lineItemID := chi.URLParam(r, "lineItemID")
	var allocations []models.BudgetLineItemWBS
	if err := decodeJSON(r, &allocations); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if err := h.queries.SetLineItemWBS(r.Context(), lineItemID, allocations); err != nil {
		log.Error().Err(err).Msg("Failed to set line item WBS allocations")
		respondError(w, http.StatusInternalServerError, "Failed to update WBS allocations")
		return
	}
	// Return the updated allocations
	items, err := h.queries.ListLineItemWBS(r.Context(), lineItemID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list updated WBS allocations")
		return
	}
	respondJSON(w, http.StatusOK, items)
}

// --- Subawards ---

func (h *Handler) ListSubawards(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantID")
	subs, err := h.queries.ListSubawards(r.Context(), grantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list subawards")
		return
	}
	if subs == nil {
		subs = []models.Subaward{}
	}
	respondJSON(w, http.StatusOK, subs)
}

func (h *Handler) CreateSubaward(w http.ResponseWriter, r *http.Request) {
	var s models.Subaward
	if err := decodeJSON(r, &s); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	s.GrantID = chi.URLParam(r, "grantID")
	if s.Status == "" {
		s.Status = "active"
	}
	if err := h.queries.CreateSubaward(r.Context(), &s); err != nil {
		log.Error().Err(err).Msg("Failed to create subaward")
		respondError(w, http.StatusInternalServerError, "Failed to create subaward")
		return
	}
	respondJSON(w, http.StatusCreated, s)
}

func (h *Handler) UpdateSubaward(w http.ResponseWriter, r *http.Request) {
	var s models.Subaward
	if err := decodeJSON(r, &s); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	s.ID = chi.URLParam(r, "subawardID")
	s.GrantID = chi.URLParam(r, "grantID")
	if err := h.queries.UpdateSubaward(r.Context(), &s); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update subaward")
		return
	}
	respondJSON(w, http.StatusOK, s)
}

func (h *Handler) DeleteSubaward(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "subawardID")
	if err := h.queries.DeleteSubaward(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete subaward")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Invoices ---

func (h *Handler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	subawardID := chi.URLParam(r, "subawardID")
	invoices, err := h.queries.ListInvoices(r.Context(), subawardID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list invoices")
		return
	}
	if invoices == nil {
		invoices = []models.Invoice{}
	}
	respondJSON(w, http.StatusOK, invoices)
}

func (h *Handler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	var inv models.Invoice
	if err := decodeJSON(r, &inv); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	inv.SubawardID = chi.URLParam(r, "subawardID")
	if inv.Status == "" {
		inv.Status = "pending"
	}
	if err := h.queries.CreateInvoice(r.Context(), &inv); err != nil {
		log.Error().Err(err).Msg("Failed to create invoice")
		respondError(w, http.StatusInternalServerError, "Failed to create invoice")
		return
	}
	respondJSON(w, http.StatusCreated, inv)
}

func (h *Handler) UpdateInvoiceStatus(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoiceID")
	var body struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if err := h.queries.UpdateInvoiceStatus(r.Context(), invoiceID, body.Status); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update invoice status")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": body.Status})
}

// --- Document Upload/Download ---

func (h *Handler) UploadInvoiceDocument(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoiceID")
	h.uploadDocument(w, r, "invoice", invoiceID)
}

func (h *Handler) UploadSignedSOW(w http.ResponseWriter, r *http.Request) {
	sowID := chi.URLParam(r, "sowID")
	h.uploadDocument(w, r, "statement_of_work", sowID)
}

func (h *Handler) uploadDocument(w http.ResponseWriter, r *http.Request, entityType, entityID string) {
	// Max 50MB
	r.ParseMultipartForm(50 << 20)

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "No file provided")
		return
	}
	defer file.Close()

	s3Key := storage.GenerateKey(entityType, entityID, header.Filename)

	if err := h.store.Upload(r.Context(), s3Key, file, header.Size, header.Header.Get("Content-Type")); err != nil {
		log.Error().Err(err).Msg("Failed to upload file")
		respondError(w, http.StatusInternalServerError, "Failed to upload file")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/pdf"
	}

	doc := models.Document{
		EntityType:  entityType,
		EntityID:    entityID,
		Filename:    header.Filename,
		ContentType: contentType,
		S3Key:       s3Key,
		FileSize:    header.Size,
	}

	if err := h.queries.CreateDocument(r.Context(), &doc); err != nil {
		log.Error().Err(err).Msg("Failed to save document record")
		respondError(w, http.StatusInternalServerError, "Failed to save document record")
		return
	}

	respondJSON(w, http.StatusCreated, doc)
}

func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docID")
	doc, err := h.queries.GetDocument(r.Context(), docID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Document not found")
		return
	}
	respondJSON(w, http.StatusOK, doc)
}

func (h *Handler) DownloadDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docID")
	doc, err := h.queries.GetDocument(r.Context(), docID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Document not found")
		return
	}

	reader, err := h.store.Download(r.Context(), doc.S3Key)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to download file")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", doc.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, doc.Filename))
	io.Copy(w, reader)
}

// --- Statements of Work ---

func (h *Handler) ListStatementsOfWork(w http.ResponseWriter, r *http.Request) {
	subawardID := chi.URLParam(r, "subawardID")
	sows, err := h.queries.ListStatementsOfWork(r.Context(), subawardID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list statements of work")
		return
	}
	if sows == nil {
		sows = []models.StatementOfWork{}
	}
	respondJSON(w, http.StatusOK, sows)
}

func (h *Handler) CreateStatementOfWork(w http.ResponseWriter, r *http.Request) {
	var s models.StatementOfWork
	if err := decodeJSON(r, &s); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	s.SubawardID = chi.URLParam(r, "subawardID")
	if s.Status == "" {
		s.Status = "draft"
	}
	if err := h.queries.CreateStatementOfWork(r.Context(), &s); err != nil {
		log.Error().Err(err).Msg("Failed to create SOW")
		respondError(w, http.StatusInternalServerError, "Failed to create statement of work")
		return
	}
	respondJSON(w, http.StatusCreated, s)
}

func (h *Handler) UpdateStatementOfWork(w http.ResponseWriter, r *http.Request) {
	var s models.StatementOfWork
	if err := decodeJSON(r, &s); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	s.ID = chi.URLParam(r, "sowID")
	s.SubawardID = chi.URLParam(r, "subawardID")
	if err := h.queries.UpdateStatementOfWork(r.Context(), &s); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update statement of work")
		return
	}
	respondJSON(w, http.StatusOK, s)
}

// --- Institution Fringe Rates ---

func (h *Handler) ListFringeRates(w http.ResponseWriter, r *http.Request) {
	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityID")
	items, err := h.queries.ListFringeRates(r.Context(), entityType, entityID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list fringe rates")
		return
	}
	if items == nil {
		items = []models.InstitutionFringeRate{}
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *Handler) UpsertFringeRate(w http.ResponseWriter, r *http.Request) {
	var fr models.InstitutionFringeRate
	if err := decodeJSON(r, &fr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	fr.EntityType = chi.URLParam(r, "entityType")
	fr.EntityID = chi.URLParam(r, "entityID")
	if fr.RateName == "" {
		fr.RateName = "default"
	}
	if err := h.queries.UpsertFringeRate(r.Context(), &fr); err != nil {
		log.Error().Err(err).Msg("Failed to upsert fringe rate")
		respondError(w, http.StatusInternalServerError, "Failed to save fringe rate")
		return
	}
	respondJSON(w, http.StatusOK, fr)
}

func (h *Handler) DeleteFringeRate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "fringeRateID")
	if err := h.queries.DeleteFringeRate(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete fringe rate")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Institution Budgets (versioned) ---

func (h *Handler) ListInstitutionBudgets(w http.ResponseWriter, r *http.Request) {
	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityID")
	latestOnly := r.URL.Query().Get("latest") == "true"

	var items []models.InstitutionBudget
	var err error
	if latestOnly {
		items, err = h.queries.ListLatestInstitutionBudgets(r.Context(), entityType, entityID)
	} else {
		items, err = h.queries.ListInstitutionBudgets(r.Context(), entityType, entityID)
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list institution budgets")
		return
	}
	if items == nil {
		items = []models.InstitutionBudget{}
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *Handler) CreateInstitutionBudget(w http.ResponseWriter, r *http.Request) {
	var b models.InstitutionBudget
	if err := decodeJSON(r, &b); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	b.EntityType = chi.URLParam(r, "entityType")
	b.EntityID = chi.URLParam(r, "entityID")
	if b.Status == "" {
		b.Status = "draft"
	}
	if err := h.queries.CreateInstitutionBudget(r.Context(), &b); err != nil {
		log.Error().Err(err).Msg("Failed to create institution budget")
		respondError(w, http.StatusInternalServerError, "Failed to create institution budget")
		return
	}
	respondJSON(w, http.StatusCreated, b)
}

func (h *Handler) FinalizeBudget(w http.ResponseWriter, r *http.Request) {
	budgetID := chi.URLParam(r, "budgetID")

	// Validate before finalizing
	validationErrors, err := h.queries.ValidateBudgetForFinalize(r.Context(), budgetID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to validate budget for finalization")
		respondError(w, http.StatusInternalServerError, "Failed to validate budget")
		return
	}
	if len(validationErrors) > 0 {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{
			"error":             "Budget cannot be finalized",
			"validation_errors": validationErrors,
		})
		return
	}

	if err := h.queries.FinalizeBudget(r.Context(), budgetID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to finalize budget")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteInstitutionBudget(w http.ResponseWriter, r *http.Request) {
	budgetID := chi.URLParam(r, "budgetID")
	if err := h.queries.DeleteBudget(r.Context(), budgetID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete budget")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DuplicateBudget(w http.ResponseWriter, r *http.Request) {
	budgetID := chi.URLParam(r, "budgetID")
	newBudget, err := h.queries.DuplicateBudget(r.Context(), budgetID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to duplicate budget")
		respondError(w, http.StatusInternalServerError, "Failed to duplicate budget")
		return
	}
	respondJSON(w, http.StatusCreated, newBudget)
}
