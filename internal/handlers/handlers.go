package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"

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
	grantID := chi.URLParam(r, "grantID")
	existing, err := h.queries.GetGrant(r.Context(), grantID)
	if err != nil {
		log.Error().Err(err).Str("grant_id", grantID).Msg("Failed to fetch grant for update")
		respondError(w, http.StatusNotFound, "Grant not found")
		return
	}
	if err := decodeJSON(r, existing); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	existing.ID = grantID
	if err := h.queries.UpdateGrant(r.Context(), existing); err != nil {
		log.Error().Err(err).Str("grant_id", grantID).Msg("Failed to update grant")
		respondError(w, http.StatusInternalServerError, "Failed to update grant")
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

func (h *Handler) DeleteGrant(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "grantID")
	if err := h.queries.DeleteGrant(r.Context(), id); err != nil {
		log.Error().Err(err).Msg("Failed to delete grant")
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
		log.Error().Err(err).Msg("Failed to list WBS areas")
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
	wbsID := chi.URLParam(r, "wbsID")
	grantID := chi.URLParam(r, "grantID")
	existing, err := h.queries.GetWBSArea(r.Context(), wbsID)
	if err != nil {
		log.Error().Err(err).Str("wbs_id", wbsID).Msg("Failed to fetch WBS area for update")
		respondError(w, http.StatusNotFound, "WBS area not found")
		return
	}
	if err := decodeJSON(r, existing); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	existing.ID = wbsID
	existing.GrantID = grantID
	if err := h.queries.UpdateWBSArea(r.Context(), existing); err != nil {
		log.Error().Err(err).Str("wbs_id", wbsID).Msg("Failed to update WBS area")
		respondError(w, http.StatusInternalServerError, "Failed to update WBS area")
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

func (h *Handler) DeleteWBSArea(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "wbsID")
	if err := h.queries.DeleteWBSArea(r.Context(), id); err != nil {
		log.Error().Err(err).Msg("Failed to delete WBS area")
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

	// Optional institution filter (comma-separated or repeated query param)
	institutions := parseInstitutionFilter(r)

	// Subaward admins are automatically restricted to their permitted institutions
	session := GetSessionFromContext(r.Context())
	if session != nil && session.Role == RoleSubawardAdmin {
		user := GetUserFromContext(r.Context())
		if user != nil {
			permitted, _ := h.queries.ListUserInstitutionNames(r.Context(), user.ID)
			institutions = intersectInstitutions(institutions, permitted)
		}
	}

	summaries, err := h.queries.WBSEffortSummaryFiltered(r.Context(), grantID, institutions)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get WBS effort summary")
		respondError(w, http.StatusInternalServerError, "Failed to get WBS effort summary")
		return
	}
	if summaries == nil {
		summaries = []models.WBSEffortSummary{}
	}

	// Optional export formats for embedding the breakdown in other documents.
	switch r.URL.Query().Get("format") {
	case "csv":
		writeWBSSummaryCSV(w, summaries)
		return
	case "md", "markdown":
		writeWBSSummaryMarkdown(w, summaries)
		return
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
		log.Error().Err(err).Msg("Failed to list personnel")
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
	personnelID := chi.URLParam(r, "personnelID")
	grantID := chi.URLParam(r, "grantID")
	existing, err := h.queries.GetPersonnel(r.Context(), personnelID)
	if err != nil {
		log.Error().Err(err).Str("personnel_id", personnelID).Msg("Failed to fetch personnel for update")
		respondError(w, http.StatusNotFound, "Personnel not found")
		return
	}
	if err := decodeJSON(r, existing); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	existing.ID = personnelID
	existing.GrantID = grantID
	if err := h.queries.UpdatePersonnel(r.Context(), existing); err != nil {
		log.Error().Err(err).Str("personnel_id", personnelID).Msg("Failed to update personnel")
		respondError(w, http.StatusInternalServerError, "Failed to update personnel")
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

func (h *Handler) DeletePersonnel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "personnelID")
	if err := h.queries.DeletePersonnel(r.Context(), id); err != nil {
		log.Error().Err(err).Msg("Failed to delete personnel")
		respondError(w, http.StatusInternalServerError, "Failed to delete personnel")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListPersonnelTitles(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantID")
	titles, err := h.queries.ListPersonnelTitles(r.Context(), grantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list titles")
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
		log.Error().Err(err).Msg("Failed to get budget summary")
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
		log.Error().Err(err).Msg("Failed to list overhead rates")
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
	rateID := chi.URLParam(r, "overheadRateID")
	existing, err := h.queries.GetOverheadRate(r.Context(), rateID)
	if err != nil {
		log.Error().Err(err).Str("overhead_rate_id", rateID).Msg("Failed to fetch overhead rate for update")
		respondError(w, http.StatusNotFound, "Overhead rate not found")
		return
	}
	if err := decodeJSON(r, existing); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	existing.ID = rateID
	existing.EntityType = chi.URLParam(r, "entityType")
	existing.EntityID = chi.URLParam(r, "entityID")
	if err := h.queries.UpdateOverheadRate(r.Context(), existing); err != nil {
		log.Error().Err(err).Str("overhead_rate_id", rateID).Msg("Failed to update overhead rate")
		respondError(w, http.StatusInternalServerError, "Failed to update overhead rate")
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

func (h *Handler) DeleteOverheadRate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "overheadRateID")
	if err := h.queries.DeleteOverheadRate(r.Context(), id); err != nil {
		log.Error().Err(err).Msg("Failed to delete overhead rate")
		respondError(w, http.StatusInternalServerError, "Failed to delete overhead rate")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListBudgetLineItems(w http.ResponseWriter, r *http.Request) {
	budgetID := chi.URLParam(r, "budgetID")
	items, err := h.queries.ListBudgetLineItems(r.Context(), budgetID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list budget line items")
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
	lineItemID := chi.URLParam(r, "lineItemID")
	budgetID := chi.URLParam(r, "budgetID")

	// Load existing item so partial JSON doesn't zero out fields
	existing, err := h.queries.GetBudgetLineItem(r.Context(), lineItemID)
	if err != nil {
		log.Error().Err(err).Str("line_item_id", lineItemID).Msg("Failed to fetch budget line item for update")
		respondError(w, http.StatusNotFound, "Budget line item not found")
		return
	}

	// Decode partial update on top of existing
	if err := decodeJSON(r, existing); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	existing.ID = lineItemID
	existing.InstitutionBudgetID = budgetID

	if err := h.queries.UpdateBudgetLineItem(r.Context(), existing); err != nil {
		log.Error().Err(err).Str("line_item_id", lineItemID).Str("budget_id", budgetID).Msg("Failed to update budget line item")
		respondError(w, http.StatusInternalServerError, "Failed to update budget line item")
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

func (h *Handler) DeleteBudgetLineItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "lineItemID")
	if err := h.queries.DeleteBudgetLineItem(r.Context(), id); err != nil {
		log.Error().Err(err).Msg("Failed to delete budget line item")
		respondError(w, http.StatusInternalServerError, "Failed to delete budget line item")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListLineItemWBS(w http.ResponseWriter, r *http.Request) {
	lineItemID := chi.URLParam(r, "lineItemID")
	items, err := h.queries.ListLineItemWBS(r.Context(), lineItemID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list line item WBS allocations")
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
		log.Error().Err(err).Msg("Failed to list updated WBS allocations")
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
		log.Error().Err(err).Msg("Failed to list subawards")
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
	subawardID := chi.URLParam(r, "subawardID")
	grantID := chi.URLParam(r, "grantID")
	existing, err := h.queries.GetSubaward(r.Context(), subawardID)
	if err != nil {
		log.Error().Err(err).Str("subaward_id", subawardID).Msg("Failed to fetch subaward for update")
		respondError(w, http.StatusNotFound, "Subaward not found")
		return
	}
	if err := decodeJSON(r, existing); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	existing.ID = subawardID
	existing.GrantID = grantID
	if err := h.queries.UpdateSubaward(r.Context(), existing); err != nil {
		log.Error().Err(err).Str("subaward_id", subawardID).Msg("Failed to update subaward")
		respondError(w, http.StatusInternalServerError, "Failed to update subaward")
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

func (h *Handler) DeleteSubaward(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "subawardID")
	if err := h.queries.DeleteSubaward(r.Context(), id); err != nil {
		log.Error().Err(err).Msg("Failed to delete subaward")
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
		log.Error().Err(err).Msg("Failed to list invoices")
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
	subawardID := chi.URLParam(r, "subawardID")
	inv.EntityType = "subaward"
	inv.EntityID = subawardID
	inv.SubawardID = &subawardID
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
		log.Error().Err(err).Msg("Failed to update invoice status")
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

// UploadSignedSOW stores the signed SOW document and associates it with the SOW
// (sets signed_doc_id and marks the SOW 'signed').
func (h *Handler) UploadSignedSOW(w http.ResponseWriter, r *http.Request) {
	sowID := chi.URLParam(r, "sowID")
	doc, code, msg := h.storeUploadedDocument(r, "statement_of_work", sowID)
	if code != 0 {
		respondError(w, code, msg)
		return
	}
	if err := h.queries.SetSOWSignedDoc(r.Context(), sowID, doc.ID); err != nil {
		log.Error().Err(err).Msg("Failed to associate signed SOW document")
		respondError(w, http.StatusInternalServerError, "Uploaded but failed to associate with the SOW")
		return
	}
	respondJSON(w, http.StatusCreated, doc)
}

func (h *Handler) uploadDocument(w http.ResponseWriter, r *http.Request, entityType, entityID string) {
	doc, code, msg := h.storeUploadedDocument(r, entityType, entityID)
	if code != 0 {
		respondError(w, code, msg)
		return
	}
	respondJSON(w, http.StatusCreated, doc)
}

// storeUploadedDocument reads the multipart "file", uploads it to S3, and records
// it in the documents table. Returns (doc, 0, "") on success or (nil, httpCode, msg).
func (h *Handler) storeUploadedDocument(r *http.Request, entityType, entityID string) (*models.Document, int, string) {
	// Max 50MB
	r.ParseMultipartForm(50 << 20)

	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, http.StatusBadRequest, "No file provided"
	}
	defer file.Close()

	s3Key := storage.GenerateKey(entityType, entityID, header.Filename)
	if err := h.store.Upload(r.Context(), s3Key, file, header.Size, header.Header.Get("Content-Type")); err != nil {
		log.Error().Err(err).Msg("Failed to upload file")
		return nil, http.StatusInternalServerError, "Failed to upload file"
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/pdf"
	}

	doc := &models.Document{
		EntityType:  entityType,
		EntityID:    entityID,
		Filename:    header.Filename,
		ContentType: contentType,
		S3Key:       s3Key,
		FileSize:    header.Size,
	}
	if err := h.queries.CreateDocument(r.Context(), doc); err != nil {
		log.Error().Err(err).Msg("Failed to save document record")
		return nil, http.StatusInternalServerError, "Failed to save document record"
	}
	return doc, 0, ""
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
		log.Error().Err(err).Msg("Failed to download file")
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
		log.Error().Err(err).Msg("Failed to list statements of work")
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
	sowID := chi.URLParam(r, "sowID")
	subawardID := chi.URLParam(r, "subawardID")
	existing, err := h.queries.GetStatementOfWork(r.Context(), sowID)
	if err != nil {
		log.Error().Err(err).Str("sow_id", sowID).Msg("Failed to fetch SOW for update")
		respondError(w, http.StatusNotFound, "Statement of work not found")
		return
	}
	if err := decodeJSON(r, existing); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	existing.ID = sowID
	existing.SubawardID = subawardID
	if err := h.queries.UpdateStatementOfWork(r.Context(), existing); err != nil {
		log.Error().Err(err).Str("sow_id", sowID).Msg("Failed to update SOW")
		respondError(w, http.StatusInternalServerError, "Failed to update statement of work")
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

func (h *Handler) DeleteStatementOfWork(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sowID")
	if err := h.queries.DeleteStatementOfWork(r.Context(), id); err != nil {
		log.Error().Err(err).Msg("Failed to delete SOW")
		respondError(w, http.StatusInternalServerError, "Failed to delete statement of work")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Institution Fringe Rates ---

func (h *Handler) ListFringeRates(w http.ResponseWriter, r *http.Request) {
	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityID")
	items, err := h.queries.ListFringeRates(r.Context(), entityType, entityID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list fringe rates")
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
		log.Error().Err(err).Msg("Failed to delete fringe rate")
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
		log.Error().Err(err).Msg("Failed to list institution budgets")
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

	// Validate before finalizing. Errors block finalization; warnings are advisory.
	validationErrors, validationWarnings, err := h.queries.ValidateBudgetForFinalize(r.Context(), budgetID)
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
		log.Error().Err(err).Msg("Failed to finalize budget")
		respondError(w, http.StatusInternalServerError, "Failed to finalize budget")
		return
	}
	// Success — surface any non-blocking warnings so the UI can display them.
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"warnings": validationWarnings,
	})
}

func (h *Handler) DeleteInstitutionBudget(w http.ResponseWriter, r *http.Request) {
	budgetID := chi.URLParam(r, "budgetID")
	if err := h.queries.DeleteBudget(r.Context(), budgetID); err != nil {
		log.Error().Err(err).Msg("Failed to delete budget")
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

// BudgetOverview returns the overall project budget broken down by institution and WBS area.
func (h *Handler) BudgetOverview(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantID")
	ctx := r.Context()

	// Optional institution filter
	institutions := parseInstitutionFilter(r)

	// Subaward admins are automatically restricted to their permitted institutions
	session := GetSessionFromContext(ctx)
	if session != nil && session.Role == RoleSubawardAdmin {
		user := GetUserFromContext(ctx)
		if user != nil {
			permitted, _ := h.queries.ListUserInstitutionNames(ctx, user.ID)
			institutions = intersectInstitutions(institutions, permitted)
		}
	}

	grant, err := h.queries.GetGrant(ctx, grantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Grant not found")
		return
	}

	subawards, err := h.queries.ListSubawards(ctx, grantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list subawards")
		respondError(w, http.StatusInternalServerError, "Failed to list subawards")
		return
	}

	wbsAreas, err := h.queries.ListWBSAreas(ctx, grantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list WBS areas")
		respondError(w, http.StatusInternalServerError, "Failed to list WBS areas")
		return
	}

	instRows, err := h.queries.BudgetOverviewByInstitutionFiltered(ctx, grantID, institutions)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get budget overview by institution")
		respondError(w, http.StatusInternalServerError, "Failed to get budget overview")
		return
	}

	wbsRows, err := h.queries.BudgetOverviewByWBSFiltered(ctx, grantID, institutions)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get budget overview by WBS")
		respondError(w, http.StatusInternalServerError, "Failed to get budget overview")
		return
	}

	// Build institution name map (filtered if applicable)
	type instInfo struct {
		name   string
		isLead bool
	}
	instNames := make(map[string]instInfo)
	instSet := make(map[string]bool)
	for _, name := range institutions {
		instSet[name] = true
	}

	if len(instSet) == 0 || instSet[grant.Institution] {
		instNames["grant:"+grant.ID] = instInfo{name: grant.Institution, isLead: true}
	}
	for _, s := range subawards {
		if len(instSet) == 0 || instSet[s.Institution] {
			instNames["subaward:"+s.ID] = instInfo{name: s.Institution, isLead: false}
		}
	}

	// Build WBS area name/code map
	wbsMap := make(map[string]models.WBSArea)
	for _, a := range wbsAreas {
		wbsMap[a.ID] = a
	}

	// Aggregate institution data
	instMap := make(map[string]*models.BudgetOverviewInstitution)
	// Pre-populate all institutions (even those with no budgets)
	for key, info := range instNames {
		parts := splitEntityKey(key)
		instMap[key] = &models.BudgetOverviewInstitution{
			EntityType: parts[0],
			EntityID:   parts[1],
			Name:       info.name,
			IsLead:     info.isLead,
			Years:      make(map[int]*models.BudgetOverviewYear),
		}
	}

	for _, row := range instRows {
		key := row.EntityType + ":" + row.EntityID
		inst := instMap[key]
		if inst == nil {
			continue
		}
		yr := inst.Years[row.FiscalYear]
		if yr == nil {
			yr = &models.BudgetOverviewYear{
				BudgetID:   row.BudgetID,
				Status:     row.Status,
				ByCategory: make(map[string]float64),
			}
			inst.Years[row.FiscalYear] = yr
		}
		yr.ByCategory[row.LineType] += row.Amount
		yr.DirectCosts += row.Amount
	}

	// --- Compute indirect (F&A) costs ---

	// 1. Build a map of all overhead rates (rateID -> OverheadRate)
	rateMap := make(map[string]*models.OverheadRate)
	for key := range instNames {
		parts := splitEntityKey(key)
		rates, err := h.queries.ListOverheadRates(ctx, parts[0], parts[1])
		if err != nil {
			log.Error().Err(err).Str("entity", key).Msg("Failed to list overhead rates")
			continue
		}
		for i := range rates {
			rateMap[rates[i].ID] = &rates[i]
		}
	}

	// 2. Fetch overhead bases (F&A base per entity/year/rate)
	overheadBaseRows, err := h.queries.BudgetOverheadBasesFiltered(ctx, grantID, institutions)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get overhead bases")
		respondError(w, http.StatusInternalServerError, "Failed to get budget overview")
		return
	}

	// Organize overhead bases: entityKey -> year -> rateID -> base
	type entityYearBases = map[int]map[string]float64
	overheadBases := make(map[string]entityYearBases)
	for _, row := range overheadBaseRows {
		key := row.EntityType + ":" + row.EntityID
		if overheadBases[key] == nil {
			overheadBases[key] = make(entityYearBases)
		}
		if overheadBases[key][row.FiscalYear] == nil {
			overheadBases[key][row.FiscalYear] = make(map[string]float64)
		}
		overheadBases[key][row.FiscalYear][row.OverheadRateID] += row.BaseAmount
	}

	// 3. For the lead institution, add subaward MTDC (first $25K per subaward)
	// Only applies when the lead institution is included in the results
	leadKey := "grant:" + grant.ID
	const mtdcCap = 25000.0
	if _, leadIncluded := instNames[leadKey]; leadIncluded {
	for _, sub := range subawards {
		subBudgets, err := h.queries.ListLatestInstitutionBudgets(ctx, "subaward", sub.ID)
		if err != nil {
			continue
		}
		sort.Slice(subBudgets, func(i, j int) bool {
			return subBudgets[i].FiscalYear < subBudgets[j].FiscalYear
		})
		cumulative := 0.0
		for _, sb := range subBudgets {
			if cumulative >= mtdcCap {
				break
			}
			// Use actual line item total instead of the header budget field,
			// which may be stale or zero.
			subTotal := sb.Budget
			if subLineItems, liErr := h.queries.ListBudgetLineItems(ctx, sb.ID); liErr == nil && len(subLineItems) > 0 {
				subTotal = 0
				for _, li := range subLineItems {
					subTotal += li.Amount
				}
			}
			contribution := math.Min(subTotal, mtdcCap-cumulative)
			cumulative += subTotal
			if contribution <= 0 {
				continue
			}
			// Add MTDC contribution to lead institution's overhead base for this year.
			// Assign to the rate with the largest base, matching NSF 1030 logic.
			if overheadBases[leadKey] == nil {
				overheadBases[leadKey] = make(entityYearBases)
			}
			if overheadBases[leadKey][sb.FiscalYear] == nil {
				overheadBases[leadKey][sb.FiscalYear] = make(map[string]float64)
			}
			yearBases := overheadBases[leadKey][sb.FiscalYear]
			bestID := ""
			bestBase := -1.0
			for id, b := range yearBases {
				if b > bestBase {
					bestBase = b
					bestID = id
				}
			}
			if bestID == "" {
				// No line-item bases yet; pick the first defined rate for the lead entity
				leadRates, _ := h.queries.ListOverheadRates(ctx, "grant", grant.ID)
				if len(leadRates) > 0 {
					bestID = leadRates[0].ID
					if rateMap[bestID] == nil {
						rateMap[bestID] = &leadRates[0]
					}
				}
			}
			if bestID != "" {
				yearBases[bestID] += contribution
			}
		}
	}
	} // end if leadIncluded

	// 4. Compute indirect costs per institution-year from bases × rates
	for key, yearBases := range overheadBases {
		inst := instMap[key]
		if inst == nil {
			continue
		}
		for year, bases := range yearBases {
			var yearIndirect float64
			for rateID, base := range bases {
				rate, ok := rateMap[rateID]
				if !ok {
					continue
				}
				yearIndirect += base * rate.Rate
			}
			yearIndirect = math.Round(yearIndirect*100) / 100

			yr := inst.Years[year]
			if yr == nil {
				yr = &models.BudgetOverviewYear{
					ByCategory: make(map[string]float64),
				}
				inst.Years[year] = yr
			}
			yr.IndirectCosts = yearIndirect
			yr.Total = yr.DirectCosts + yr.IndirectCosts
			inst.DirectTotal += yr.DirectCosts
			inst.IndirectTotal += yearIndirect
		}
	}

	// Also set Total for years that might only have direct costs (no overhead bases)
	for _, inst := range instMap {
		inst.Total = 0
		inst.DirectTotal = 0
		inst.IndirectTotal = 0
		for _, yr := range inst.Years {
			yr.Total = yr.DirectCosts + yr.IndirectCosts
			inst.DirectTotal += yr.DirectCosts
			inst.IndirectTotal += yr.IndirectCosts
		}
		inst.Total = inst.DirectTotal + inst.IndirectTotal
	}

	// Build sorted institutions list (lead first)
	var instList []*models.BudgetOverviewInstitution
	// Lead first
	for _, inst := range instMap {
		if inst.IsLead {
			instList = append([]*models.BudgetOverviewInstitution{inst}, instList...)
		} else {
			instList = append(instList, inst)
		}
	}

	// Aggregate WBS data
	wbsOverview := make(map[string]*models.BudgetOverviewWBS) // key: wbs_area_id or "unassigned"
	for _, row := range wbsRows {
		var key string
		if row.WBSAreaID != nil {
			key = *row.WBSAreaID
		} else {
			key = "unassigned"
		}
		entry := wbsOverview[key]
		if entry == nil {
			if row.WBSAreaID != nil {
				area := wbsMap[*row.WBSAreaID]
				entry = &models.BudgetOverviewWBS{
					WBSAreaID: row.WBSAreaID,
					Code:      area.Code,
					Name:      area.Name,
					Years:     make(map[int]float64),
				}
			} else {
				entry = &models.BudgetOverviewWBS{
					Code:  "",
					Name:  "Unassigned",
					Years: make(map[int]float64),
				}
			}
			wbsOverview[key] = entry
		}
		entry.Years[row.FiscalYear] += row.Amount
		entry.Total += row.Amount
	}

	// Build sorted WBS list (by code, unassigned last)
	var wbsList []*models.BudgetOverviewWBS
	for _, area := range wbsAreas {
		if entry, ok := wbsOverview[area.ID]; ok {
			wbsList = append(wbsList, entry)
		}
	}
	if entry, ok := wbsOverview["unassigned"]; ok {
		wbsList = append(wbsList, entry)
	}

	// Compute yearly totals
	yearlyTotals := make(map[int]float64)
	yearlyDirect := make(map[int]float64)
	yearlyIndirect := make(map[int]float64)
	var grandTotal, grandDirect, grandIndirect float64
	for _, inst := range instList {
		for yr, data := range inst.Years {
			yearlyTotals[yr] += data.Total
			yearlyDirect[yr] += data.DirectCosts
			yearlyIndirect[yr] += data.IndirectCosts
		}
		grandTotal += inst.Total
		grandDirect += inst.DirectTotal
		grandIndirect += inst.IndirectTotal
	}

	resp := models.BudgetOverviewResponse{
		Institutions:   instList,
		WBSAreas:       wbsList,
		YearlyTotals:   yearlyTotals,
		YearlyDirect:   yearlyDirect,
		YearlyIndirect: yearlyIndirect,
		GrandTotal:     grandTotal,
		GrandDirect:    grandDirect,
		GrandIndirect:  grandIndirect,
		AwardTotal:     grant.TotalBudget,
	}
	if resp.Institutions == nil {
		resp.Institutions = []*models.BudgetOverviewInstitution{}
	}
	if resp.WBSAreas == nil {
		resp.WBSAreas = []*models.BudgetOverviewWBS{}
	}

	respondJSON(w, http.StatusOK, resp)
}

// splitEntityKey splits "grant:uuid" into ["grant", "uuid"].
func splitEntityKey(key string) [2]string {
	for i, c := range key {
		if c == ':' {
			return [2]string{key[:i], key[i+1:]}
		}
	}
	return [2]string{key, ""}
}

// parseInstitutionFilter reads ?institutions= from the URL query.
// Accepts comma-separated values or repeated params.
func parseInstitutionFilter(r *http.Request) []string {
	vals := r.URL.Query()["institutions"]
	if len(vals) == 0 {
		return nil
	}
	var result []string
	for _, v := range vals {
		for _, name := range strings.Split(v, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				result = append(result, name)
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// intersectInstitutions returns only institutions that appear in both lists.
// If the user-provided filter is nil/empty, returns the permitted list.
// If the permitted list is nil/empty, returns the user filter (no enforcement).
func intersectInstitutions(filter, permitted []string) []string {
	if len(permitted) == 0 {
		return filter
	}
	if len(filter) == 0 {
		return permitted
	}
	pset := make(map[string]bool, len(permitted))
	for _, p := range permitted {
		pset[p] = true
	}
	var result []string
	for _, f := range filter {
		if pset[f] {
			result = append(result, f)
		}
	}
	if len(result) == 0 {
		// No overlap: return the permitted list (don't allow seeing nothing)
		return permitted
	}
	return result
}
