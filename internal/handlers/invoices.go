package handlers

import (
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/fabaid-manager/internal/models"
	"github.com/bbockelm/fabaid-manager/internal/storage"
)

// validEntityType reports whether t is a recognized billing entity type.
func validEntityType(t string) bool { return t == "grant" || t == "subaward" }

// --- Entity-scoped invoice CRUD ---

// ListEntityInvoices lists invoices for one billing entity.
// GET /institution-rates/{entityType}/{entityID}/invoices
func (h *Handler) ListEntityInvoices(w http.ResponseWriter, r *http.Request) {
	et, eid := chi.URLParam(r, "entityType"), chi.URLParam(r, "entityID")
	if !validEntityType(et) {
		respondError(w, http.StatusBadRequest, "entityType must be 'grant' or 'subaward'")
		return
	}
	invoices, err := h.queries.ListInvoicesByEntity(r.Context(), et, eid)
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

// CreateEntityInvoice creates an invoice for a billing entity.
// POST /institution-rates/{entityType}/{entityID}/invoices
func (h *Handler) CreateEntityInvoice(w http.ResponseWriter, r *http.Request) {
	et, eid := chi.URLParam(r, "entityType"), chi.URLParam(r, "entityID")
	if !validEntityType(et) {
		respondError(w, http.StatusBadRequest, "entityType must be 'grant' or 'subaward'")
		return
	}
	var inv models.Invoice
	if err := decodeJSON(r, &inv); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	inv.EntityType, inv.EntityID = et, eid
	if err := h.queries.CreateInvoice(r.Context(), &inv); err != nil {
		log.Error().Err(err).Msg("Failed to create invoice")
		respondError(w, http.StatusInternalServerError, "Failed to create invoice")
		return
	}
	respondJSON(w, http.StatusCreated, inv)
}

// invoiceDetail bundles an invoice with its coded expense lines and WBS splits.
type invoiceDetail struct {
	models.Invoice
	Expenses []expenseWithWBS `json:"expenses"`
}

type expenseWithWBS struct {
	models.InvoiceExpense
	WBS []models.InvoiceExpenseWBS `json:"wbs"`
}

// GetInvoiceDetail returns an invoice plus its expenses and their WBS allocations.
// GET /institution-rates/{entityType}/{entityID}/invoices/{invoiceID}
func (h *Handler) GetInvoiceDetail(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoiceID")
	inv, err := h.queries.GetInvoice(r.Context(), invoiceID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Invoice not found")
		return
	}
	expenses, err := h.queries.ListInvoiceExpenses(r.Context(), invoiceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load expenses")
		return
	}
	detail := invoiceDetail{Invoice: *inv, Expenses: []expenseWithWBS{}}
	for _, e := range expenses {
		wbs, err := h.queries.ListInvoiceExpenseWBS(r.Context(), e.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to load expense WBS")
			return
		}
		if wbs == nil {
			wbs = []models.InvoiceExpenseWBS{}
		}
		detail.Expenses = append(detail.Expenses, expenseWithWBS{InvoiceExpense: e, WBS: wbs})
	}
	respondJSON(w, http.StatusOK, detail)
}

// UpdateInvoice updates an invoice's header fields.
// PUT /institution-rates/{entityType}/{entityID}/invoices/{invoiceID}
func (h *Handler) UpdateInvoice(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoiceID")
	existing, err := h.queries.GetInvoice(r.Context(), invoiceID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Invoice not found")
		return
	}
	// Payment status is only changed via the approve endpoint (admin/grant_admin),
	// never through a general edit — preserve it across a PUT.
	origStatus := existing.Status
	if err := decodeJSON(r, existing); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	existing.ID = invoiceID
	existing.Status = origStatus
	if err := h.queries.UpdateInvoice(r.Context(), existing); err != nil {
		log.Error().Err(err).Msg("Failed to update invoice")
		respondError(w, http.StatusInternalServerError, "Failed to update invoice")
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

// DeleteInvoice deletes an invoice (expenses cascade).
// DELETE /institution-rates/{entityType}/{entityID}/invoices/{invoiceID}
func (h *Handler) DeleteInvoice(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoiceID")
	if err := h.queries.DeleteInvoice(r.Context(), invoiceID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete invoice")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UploadEntityInvoiceDoc uploads an invoice PDF and links it to the invoice.
// POST /institution-rates/{entityType}/{entityID}/invoices/{invoiceID}/upload
func (h *Handler) UploadEntityInvoiceDoc(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoiceID")
	r.ParseMultipartForm(50 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "No file provided")
		return
	}
	defer file.Close()

	s3Key := storage.GenerateKey("invoice", invoiceID, header.Filename)
	if err := h.store.Upload(r.Context(), s3Key, file, header.Size, header.Header.Get("Content-Type")); err != nil {
		log.Error().Err(err).Msg("Failed to upload invoice file")
		respondError(w, http.StatusInternalServerError, "Failed to upload file")
		return
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/pdf"
	}
	doc := models.Document{
		EntityType: "invoice", EntityID: invoiceID, Filename: header.Filename,
		ContentType: contentType, S3Key: s3Key, FileSize: header.Size,
	}
	if err := h.queries.CreateDocument(r.Context(), &doc); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save document record")
		return
	}
	if err := h.queries.SetInvoiceDocument(r.Context(), invoiceID, doc.ID); err != nil {
		log.Error().Err(err).Msg("Failed to link invoice document")
	}
	respondJSON(w, http.StatusCreated, doc)
}

// FinalizeInvoiceCoding marks an invoice's coding as final. Human-only: the AI
// coding agent only ever writes 'draft' codings; finalizing is a deliberate human
// action (this endpoint requires write access and is never called by the agent).
// POST /institution-rates/{entityType}/{entityID}/invoices/{invoiceID}/finalize-coding
func (h *Handler) FinalizeInvoiceCoding(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoiceID")
	if err := h.queries.SetInvoiceCodingStatus(r.Context(), invoiceID, "final"); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to finalize coding")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"coding_status": "final"})
}

// SetInvoiceCoding sets an invoice's coding status to draft or uncoded (human edits).
// PATCH /institution-rates/{entityType}/{entityID}/invoices/{invoiceID}/coding-status
func (h *Handler) SetInvoiceCoding(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoiceID")
	var body struct {
		CodingStatus string `json:"coding_status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if body.CodingStatus != "draft" && body.CodingStatus != "uncoded" && body.CodingStatus != "final" {
		respondError(w, http.StatusBadRequest, "coding_status must be uncoded, draft, or final")
		return
	}
	if err := h.queries.SetInvoiceCodingStatus(r.Context(), invoiceID, body.CodingStatus); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to set coding status")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"coding_status": body.CodingStatus})
}

// SetInvoicePaymentStatus approves/updates an invoice's payment status. Gated to
// admin/grant_admin at the router — subaward admins can code but not approve.
// PATCH /institution-rates/{entityType}/{entityID}/invoices/{invoiceID}/status
func (h *Handler) SetInvoicePaymentStatus(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoiceID")
	var body struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	switch body.Status {
	case "pending", "approved", "rejected", "paid":
	default:
		respondError(w, http.StatusBadRequest, "status must be pending, approved, rejected, or paid")
		return
	}
	if err := h.queries.UpdateInvoiceStatus(r.Context(), invoiceID, body.Status); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update payment status")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": body.Status})
}

// entityInstitution resolves an entity to its institution name.
func (h *Handler) entityInstitution(r *http.Request, entityType, entityID string) string {
	switch entityType {
	case "grant":
		if g, err := h.queries.GetGrant(r.Context(), entityID); err == nil {
			return g.Institution
		}
	case "subaward":
		if s, err := h.queries.GetSubaward(r.Context(), entityID); err == nil {
			return s.Institution
		}
	}
	return ""
}

// RequireInvoiceWriteScope restricts invoice mutations for subaward admins to
// invoices belonging to their permitted institution(s). Reads pass through;
// admin/grant_admin are unrestricted. (read_only is already blocked upstream.)
func (h *Handler) RequireInvoiceWriteScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		session := GetSessionFromContext(r.Context())
		if session == nil {
			respondError(w, http.StatusUnauthorized, "Not authenticated")
			return
		}
		if session.Role == RoleAdmin || session.Role == RoleGrantAdmin {
			next.ServeHTTP(w, r)
			return
		}
		if session.Role == RoleSubawardAdmin {
			inst := h.entityInstitution(r, chi.URLParam(r, "entityType"), chi.URLParam(r, "entityID"))
			user := GetUserFromContext(r.Context())
			if user != nil && inst != "" {
				permitted, _ := h.queries.ListUserInstitutionNames(r.Context(), user.ID)
				for _, p := range permitted {
					if p == inst {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			respondError(w, http.StatusForbidden, "You can only manage invoices for your own institution")
			return
		}
		respondError(w, http.StatusForbidden, "Insufficient permissions")
	})
}

// --- Invoice expense CRUD ---

// ListInvoiceExpensesHandler lists an invoice's expense lines.
func (h *Handler) ListInvoiceExpensesHandler(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoiceID")
	expenses, err := h.queries.ListInvoiceExpenses(r.Context(), invoiceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list expenses")
		return
	}
	if expenses == nil {
		expenses = []models.InvoiceExpense{}
	}
	respondJSON(w, http.StatusOK, expenses)
}

// CreateInvoiceExpenseHandler adds an expense line to an invoice.
func (h *Handler) CreateInvoiceExpenseHandler(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoiceID")
	var e models.InvoiceExpense
	if err := decodeJSON(r, &e); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	e.InvoiceID = invoiceID
	if err := h.queries.CreateInvoiceExpense(r.Context(), &e); err != nil {
		log.Error().Err(err).Msg("Failed to create invoice expense")
		respondError(w, http.StatusInternalServerError, "Failed to create expense")
		return
	}
	respondJSON(w, http.StatusCreated, e)
}

// UpdateInvoiceExpenseHandler updates an expense line (partial merge on existing).
func (h *Handler) UpdateInvoiceExpenseHandler(w http.ResponseWriter, r *http.Request) {
	expenseID := chi.URLParam(r, "expenseID")
	existing, err := h.queries.GetInvoiceExpense(r.Context(), expenseID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Expense not found")
		return
	}
	if err := decodeJSON(r, existing); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	existing.ID = expenseID
	if err := h.queries.UpdateInvoiceExpense(r.Context(), existing); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update expense")
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

// DeleteInvoiceExpenseHandler removes an expense line.
func (h *Handler) DeleteInvoiceExpenseHandler(w http.ResponseWriter, r *http.Request) {
	expenseID := chi.URLParam(r, "expenseID")
	if err := h.queries.DeleteInvoiceExpense(r.Context(), expenseID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete expense")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetInvoiceExpenseWBSHandler lists an expense line's WBS allocations.
func (h *Handler) GetInvoiceExpenseWBSHandler(w http.ResponseWriter, r *http.Request) {
	expenseID := chi.URLParam(r, "expenseID")
	wbs, err := h.queries.ListInvoiceExpenseWBS(r.Context(), expenseID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list expense WBS")
		return
	}
	if wbs == nil {
		wbs = []models.InvoiceExpenseWBS{}
	}
	respondJSON(w, http.StatusOK, wbs)
}

// SetInvoiceExpenseWBSHandler replaces an expense line's WBS allocations.
func (h *Handler) SetInvoiceExpenseWBSHandler(w http.ResponseWriter, r *http.Request) {
	expenseID := chi.URLParam(r, "expenseID")
	var allocations []models.InvoiceExpenseWBS
	if err := decodeJSON(r, &allocations); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if err := h.queries.SetInvoiceExpenseWBS(r.Context(), expenseID, allocations); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to set expense WBS")
		return
	}
	respondJSON(w, http.StatusOK, allocations)
}

// --- Grant-level invoice views + analytics ---

// ListGrantInvoices lists all invoices across a grant and its subawards.
// GET /grants/{grantID}/invoices
func (h *Handler) ListGrantInvoices(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantID")
	invoices, err := h.queries.ListInvoicesForGrant(r.Context(), grantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list invoices")
		return
	}
	if invoices == nil {
		invoices = []models.Invoice{}
	}
	respondJSON(w, http.StatusOK, invoices)
}

// behindThresholdMonths: an institution billing less recently than this (since its
// last billed period end) is flagged as behind on invoicing.
const behindThresholdMonths = 2.0

// InvoiceAnalytics computes actual expenditure rollups and burn-rate projections.
// GET /grants/{grantID}/invoice-analytics
func (h *Handler) InvoiceAnalytics(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantID")
	now := time.Now()

	entities, err := h.queries.ListBillingEntities(r.Context(), grantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load billing entities")
		return
	}
	invoices, err := h.queries.ListInvoicesForGrant(r.Context(), grantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load invoices")
		return
	}
	expenses, err := h.queries.ListFinalizedExpensesForGrant(r.Context(), grantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load expenses")
		return
	}
	wbsAreas, err := h.queries.ListWBSAreas(r.Context(), grantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load WBS areas")
		return
	}
	wbsName := map[string]string{}
	for _, a := range wbsAreas {
		wbsName[a.ID] = a.Name
	}

	// --- Actuals rollups (finalized expenses only) ---
	byWBS := map[string]float64{}
	byCategory := map[string]float64{}
	byEntity := map[string]float64{}
	var uncategorizedWBS, uncategorizedCategory, totalActual float64

	for _, e := range expenses {
		totalActual += e.Amount
		byEntity[e.EntityType+":"+e.EntityID] += e.Amount
		if e.LineType == "uncategorized" || e.LineType == "" {
			uncategorizedCategory += e.Amount
		}
		byCategory[nonEmpty(e.LineType, "uncategorized")] += e.Amount

		var allocated float64
		for _, w := range e.WBS {
			share := e.Amount * w.AllocationPercent / 100
			byWBS[w.WBSAreaID] += share
			allocated += share
		}
		if rem := e.Amount - allocated; rem > 0.005 {
			uncategorizedWBS += rem
		}
	}

	// --- Per-institution burn analysis ---
	type burnRow struct {
		EntityType         string  `json:"entity_type"`
		EntityID           string  `json:"entity_id"`
		Institution        string  `json:"institution"`
		Budget             float64 `json:"budget"`
		ActualTotal        float64 `json:"actual_total"`
		ActualNonCapital   float64 `json:"actual_non_capital"`
		LastPeriodEnd      string  `json:"last_period_end,omitempty"`
		MonthsSinceLast    float64 `json:"months_since_last"`
		Behind             bool    `json:"behind"`
		EstimatedMonthly   float64 `json:"estimated_monthly"`   // recurring non-capital $/month to date
		ProjectedSinceLast float64 `json:"projected_since_last"` // extrapolated unbilled to today
		ProjectedToDate    float64 `json:"projected_to_date"`   // actual + projected since last
		ExpectedRemaining  float64 `json:"expected_remaining"`  // recurring spend to entity end
		ExpectedYearEndFunds float64 `json:"expected_year_end_funds"`
	}

	// Non-capital actuals per entity (for burn). Capital == equipment.
	nonCapByEntity := map[string]float64{}
	for _, e := range expenses {
		if e.LineType != "equipment" {
			nonCapByEntity[e.EntityType+":"+e.EntityID] += e.Amount
		}
	}

	burn := []burnRow{}
	behind := []map[string]any{}
	for _, ent := range entities {
		key := ent.EntityType + ":" + ent.EntityID
		row := burnRow{
			EntityType: ent.EntityType, EntityID: ent.EntityID, Institution: ent.Institution,
			Budget: ent.TotalBudget, ActualTotal: byEntity[key], ActualNonCapital: nonCapByEntity[key],
		}

		// Gather this entity's invoices (most recent first) for last-billed + trailing avg.
		var entInv []models.Invoice
		for _, inv := range invoices {
			if inv.EntityType == ent.EntityType && inv.EntityID == ent.EntityID {
				entInv = append(entInv, inv)
			}
		}
		lastEnd, hasLast := latestPeriodEnd(entInv)
		startT, hasStart := parseDate(ent.StartDate)

		if hasLast {
			row.LastPeriodEnd = lastEnd.Format("2006-01-02")
			row.MonthsSinceLast = monthsBetween(lastEnd, now)
			row.Behind = row.MonthsSinceLast >= behindThresholdMonths
			// Estimated recurring burn: non-capital actuals over elapsed project months.
			if hasStart {
				elapsed := monthsBetween(startT, lastEnd)
				if elapsed > 0.5 {
					row.EstimatedMonthly = row.ActualNonCapital / elapsed
				}
			}
			// Projected unbilled since last invoice, using trailing 3-invoice monthly avg.
			trailingMonthly := trailingMonthlyAverage(entInv, 3)
			if trailingMonthly == 0 {
				trailingMonthly = row.EstimatedMonthly
			}
			if row.MonthsSinceLast > 0 {
				row.ProjectedSinceLast = trailingMonthly * row.MonthsSinceLast
			}
			row.ProjectedToDate = row.ActualTotal + row.ProjectedSinceLast
			// Expected recurring spend from today to the entity's end date.
			if endT, ok := parseDate(ent.EndDate); ok {
				if m := monthsBetween(now, endT); m > 0 {
					row.ExpectedRemaining = trailingMonthly * m
				}
			}
			row.ExpectedYearEndFunds = row.Budget - (row.ProjectedToDate + row.ExpectedRemaining)

			if row.Behind {
				behind = append(behind, map[string]any{
					"entity_type": ent.EntityType, "entity_id": ent.EntityID,
					"institution": ent.Institution, "last_period_end": row.LastPeriodEnd,
					"months_since_last": round2(row.MonthsSinceLast),
				})
			}
		} else {
			// No invoices yet: entire budget is unspent; flag as behind if the project has started.
			row.ExpectedYearEndFunds = row.Budget
			if hasStart && monthsBetween(startT, now) >= behindThresholdMonths {
				row.Behind = true
				behind = append(behind, map[string]any{
					"entity_type": ent.EntityType, "entity_id": ent.EntityID,
					"institution": ent.Institution, "last_period_end": nil,
					"months_since_last": nil,
				})
			}
		}
		// Round monetary/months fields for presentation.
		row.EstimatedMonthly = round2(row.EstimatedMonthly)
		row.ProjectedSinceLast = round2(row.ProjectedSinceLast)
		row.ProjectedToDate = round2(row.ProjectedToDate)
		row.ExpectedRemaining = round2(row.ExpectedRemaining)
		row.ExpectedYearEndFunds = round2(row.ExpectedYearEndFunds)
		row.MonthsSinceLast = round2(row.MonthsSinceLast)
		burn = append(burn, row)
	}

	// Shape actuals-by-* as sorted-ish slices with names.
	wbsRows := []map[string]any{}
	for id, amt := range byWBS {
		wbsRows = append(wbsRows, map[string]any{"wbs_area_id": id, "name": wbsName[id], "amount": round2(amt)})
	}
	if uncategorizedWBS > 0.005 {
		wbsRows = append(wbsRows, map[string]any{"wbs_area_id": nil, "name": "Uncategorized", "amount": round2(uncategorizedWBS), "uncategorized": true})
	}
	catRows := []map[string]any{}
	for lt, amt := range byCategory {
		catRows = append(catRows, map[string]any{"line_type": lt, "amount": round2(amt), "uncategorized": lt == "uncategorized"})
	}
	instRows := []map[string]any{}
	for _, ent := range entities {
		instRows = append(instRows, map[string]any{
			"entity_type": ent.EntityType, "entity_id": ent.EntityID,
			"institution": ent.Institution, "amount": round2(byEntity[ent.EntityType+":"+ent.EntityID]),
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"total_actual":   round2(totalActual),
		"by_wbs":         wbsRows,
		"by_category":    catRows,
		"by_institution": instRows,
		"uncategorized": map[string]any{
			"category": round2(uncategorizedCategory),
			"wbs":      round2(uncategorizedWBS),
		},
		"behind": behind,
		"burn":   burn,
	})
}

// --- small helpers ---

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

func parseDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// monthsBetween returns the fractional number of 30.44-day months from a to b.
func monthsBetween(a, b time.Time) float64 {
	if b.Before(a) {
		return 0
	}
	return b.Sub(a).Hours() / 24 / 30.44
}

// latestPeriodEnd returns the most recent period_end (falling back to invoice_date).
func latestPeriodEnd(invs []models.Invoice) (time.Time, bool) {
	var latest time.Time
	found := false
	for _, inv := range invs {
		var t time.Time
		var ok bool
		if inv.PeriodEnd != nil {
			t, ok = parseDate(*inv.PeriodEnd)
		}
		if !ok {
			t, ok = parseDate(inv.InvoiceDate)
		}
		if ok && (!found || t.After(latest)) {
			latest, found = t, true
		}
	}
	return latest, found
}

// trailingMonthlyAverage estimates $/month from the most recent n invoices,
// dividing total billed by the total months those invoices cover.
func trailingMonthlyAverage(invs []models.Invoice, n int) float64 {
	// invs is already sorted by invoice_date DESC from the query.
	var amt, months float64
	count := 0
	for _, inv := range invs {
		if count >= n {
			break
		}
		start, sok := time.Time{}, false
		if inv.PeriodStart != nil {
			start, sok = parseDate(*inv.PeriodStart)
		}
		end, eok := time.Time{}, false
		if inv.PeriodEnd != nil {
			end, eok = parseDate(*inv.PeriodEnd)
		}
		span := 1.0 // assume ~1 month if no period given
		if sok && eok {
			if m := monthsBetween(start, end); m > 0 {
				span = m
			}
		}
		amt += inv.Amount
		months += span
		count++
	}
	if months <= 0 {
		return 0
	}
	return amt / months
}
