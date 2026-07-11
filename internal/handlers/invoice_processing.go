package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/fabaid-manager/internal/docextract"
	"github.com/bbockelm/fabaid-manager/internal/llm"
	"github.com/bbockelm/fabaid-manager/internal/models"
)

// ProcessInvoiceCoding triggers AI coding of an uploaded invoice document. The
// agent proposes a DRAFT coding (expense lines + WBS splits); it never finalizes.
// POST /institution-rates/{entityType}/{entityID}/invoices/{invoiceID}/code
func (h *Handler) ProcessInvoiceCoding(w http.ResponseWriter, r *http.Request) {
	if h.cfg.LLMAPIKey == "" {
		respondError(w, http.StatusServiceUnavailable, "LLM not configured (set LLM_API_KEY or LLM_API_KEY_FILE)")
		return
	}
	invoiceID := chi.URLParam(r, "invoiceID")

	inv, err := h.queries.GetInvoice(r.Context(), invoiceID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Invoice not found")
		return
	}
	// Never let the AI overwrite a human-finalized coding.
	if inv.CodingStatus == "final" {
		respondError(w, http.StatusConflict, "Invoice coding is finalized; re-open it before re-coding")
		return
	}
	if inv.DocumentID == nil {
		respondError(w, http.StatusBadRequest, "Upload an invoice document before running AI coding")
		return
	}

	doc, err := h.queries.GetDocument(r.Context(), *inv.DocumentID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Invoice document not found")
		return
	}

	var reqBody struct {
		UserPrompt string `json:"user_prompt"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
	}

	// Create the processing run (reuses the shared runs table via run_type).
	run := &models.DocumentProcessingRun{
		DocumentID:   inv.DocumentID,
		InvoiceID:    &invoiceID,
		RunType:      "invoice_coding",
		EntityType:   inv.EntityType,
		EntityID:     inv.EntityID,
		Status:       "pending",
		StatusDetail: "Initializing...",
		LLMModel:     h.cfg.LLMModel,
	}
	if err := h.queries.CreateDocumentProcessingRun(r.Context(), run); err != nil {
		log.Error().Err(err).Msg("Failed to create invoice coding run")
		respondError(w, http.StatusInternalServerError, "Failed to create coding run")
		return
	}

	failRun := func(msg string) {
		now := time.Now()
		run.Status, run.StatusDetail, run.ErrorMsg, run.CompletedAt = "failed", msg, msg, &now
		_ = h.queries.UpdateDocumentProcessingRun(r.Context(), run)
	}

	// Download the (unencrypted) invoice document and extract its text.
	reader, err := h.store.Download(r.Context(), doc.S3Key)
	if err != nil {
		failRun("Failed to download invoice document")
		respondError(w, http.StatusInternalServerError, "Failed to download document")
		return
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		failRun("Failed to read invoice document")
		respondError(w, http.StatusInternalServerError, "Failed to read document")
		return
	}
	result, err := docextract.Extract(doc.Filename, data)
	if err != nil {
		failRun(fmt.Sprintf("Failed to extract invoice: %s", err))
		respondError(w, http.StatusUnprocessableEntity, fmt.Sprintf("Failed to extract invoice: %s", err))
		return
	}
	extractedMD := result.ToMarkdown()

	userPrompt := reqBody.UserPrompt
	go func() {
		client := llm.NewClient(h.cfg.LLMAPIKey, h.cfg.LLMAPIURL, h.cfg.LLMModel)
		ic := llm.NewInvoiceCodingContext(h.queries, client, run, inv)
		ic.UserPrompt = userPrompt
		if err := ic.Process(context.Background(), extractedMD); err != nil {
			log.Error().Err(err).Str("run_id", run.ID).Msg("Invoice coding failed")
		}
	}()

	respondJSON(w, http.StatusAccepted, map[string]string{"run_id": run.ID, "status": "pending"})
}
