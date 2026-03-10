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

	"github.com/bbockelm/fabaid-manager/internal/crypto"
	"github.com/bbockelm/fabaid-manager/internal/docextract"
	"github.com/bbockelm/fabaid-manager/internal/llm"
	"github.com/bbockelm/fabaid-manager/internal/models"
)

// ProcessBudgetDocument triggers AI processing of an uploaded budget document.
// It validates inputs, extracts the document, and launches processing in a
// background goroutine. Returns the run ID immediately so the frontend can poll.
// POST /api/v1/institution-rates/{entityType}/{entityID}/budget-documents/{docID}/process
func (h *Handler) ProcessBudgetDocument(w http.ResponseWriter, r *http.Request) {
	if h.cfg.LLMAPIKey == "" {
		log.Warn().Msg("ProcessBudgetDocument called but LLM_API_KEY is not configured")
		respondError(w, http.StatusServiceUnavailable, "LLM not configured (set LLM_API_KEY or LLM_API_KEY_FILE)")
		return
	}
	if h.encryptor == nil {
		log.Warn().Msg("ProcessBudgetDocument called but document encryption is not configured")
		respondError(w, http.StatusServiceUnavailable, "Document encryption not configured")
		return
	}

	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityID")
	docID := chi.URLParam(r, "docID")

	// Parse optional user prompt from request body
	var reqBody struct {
		UserPrompt string `json:"user_prompt"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
	}

	// Validate entity type
	if entityType != "grant" && entityType != "subaward" {
		respondError(w, http.StatusBadRequest, "entityType must be 'grant' or 'subaward'")
		return
	}

	// Fetch the document record
	doc, err := h.queries.GetBudgetDocument(r.Context(), docID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Document not found")
		return
	}
	if doc.DeletedAt != nil {
		respondError(w, http.StatusGone, "Document has been deleted")
		return
	}

	// Create the processing run record
	run := &models.DocumentProcessingRun{
		DocumentID:   docID,
		EntityType:   entityType,
		EntityID:     entityID,
		Status:       "pending",
		StatusDetail: "Initializing...",
		LLMModel:     h.cfg.LLMModel,
	}
	if err := h.queries.CreateDocumentProcessingRun(r.Context(), run); err != nil {
		log.Error().Err(err).Msg("Failed to create processing run")
		respondError(w, http.StatusInternalServerError, "Failed to create processing run")
		return
	}

	// failRun marks the processing run as failed in the database.
	failRun := func(errMsg string) {
		now := time.Now()
		run.Status = "failed"
		run.StatusDetail = errMsg
		run.ErrorMsg = errMsg
		run.CompletedAt = &now
		if uerr := h.queries.UpdateDocumentProcessingRun(r.Context(), run); uerr != nil {
			log.Error().Err(uerr).Str("run_id", run.ID).Msg("Failed to mark processing run as failed")
		}
	}

	// Download + decrypt the document synchronously (uses request context).
	reader, err := h.store.Download(r.Context(), doc.S3Key)
	if err != nil {
		log.Error().Err(err).Str("s3_key", doc.S3Key).Msg("Failed to download document from S3")
		failRun("Failed to download document from storage")
		respondError(w, http.StatusInternalServerError, "Failed to download document")
		return
	}
	defer reader.Close()

	ciphertext, err := io.ReadAll(reader)
	if err != nil {
		log.Error().Err(err).Str("doc_id", docID).Msg("Failed to read document from S3")
		failRun("Failed to read document from storage")
		respondError(w, http.StatusInternalServerError, "Failed to read document")
		return
	}

	dek, err := h.encryptor.UnwrapDEK(doc.EncryptedDEK, doc.DEKNonce)
	if err != nil {
		log.Error().Err(err).Str("doc_id", docID).Msg("Failed to unwrap document encryption key")
		failRun("Decryption error — wrong master key?")
		respondError(w, http.StatusInternalServerError, "Decryption error — wrong master key?")
		return
	}
	plaintext, err := crypto.Decrypt(dek, ciphertext)
	if err != nil {
		log.Error().Err(err).Str("doc_id", docID).Msg("Failed to decrypt document")
		failRun("Failed to decrypt document")
		respondError(w, http.StatusInternalServerError, "Decryption error")
		return
	}

	// Extract document content synchronously so extraction errors are reported to the client.
	result, err := docextract.Extract(doc.Filename, plaintext)
	if err != nil {
		log.Error().Err(err).Str("filename", doc.Filename).Msg("Failed to extract document")
		failRun(fmt.Sprintf("Failed to extract document: %s", err))
		respondError(w, http.StatusUnprocessableEntity, fmt.Sprintf("Failed to extract document: %s", err))
		return
	}
	extractedMD := result.ToMarkdown()

	// Launch processing in a background goroutine.
	// The processor uses context.Background() so it is independent of this request.
	userPrompt := reqBody.UserPrompt
	go func() {
		client := llm.NewClient(h.cfg.LLMAPIKey, h.cfg.LLMAPIURL, h.cfg.LLMModel)
		proc := llm.NewProcessorContext(h.queries, client, run)
		proc.UserPrompt = userPrompt

		if err := proc.Process(context.Background(), extractedMD); err != nil {
			log.Error().Err(err).Str("run_id", run.ID).Msg("Document processing failed")
		}
	}()

	// Return the run ID immediately. The frontend will poll for status updates.
	respondJSON(w, http.StatusAccepted, map[string]string{
		"run_id": run.ID,
		"status": "pending",
	})
}

// GetProcessingRun returns details of a specific processing run.
// GET /api/v1/institution-rates/{entityType}/{entityID}/budget-documents/processing-runs/{runID}
func (h *Handler) GetProcessingRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")
	run, err := h.queries.GetDocumentProcessingRun(r.Context(), runID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Processing run not found")
		return
	}
	respondJSON(w, http.StatusOK, run)
}

// ListProcessingRuns returns all processing runs for a document.
// GET /api/v1/institution-rates/{entityType}/{entityID}/budget-documents/{docID}/processing-runs
func (h *Handler) ListProcessingRuns(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docID")
	runs, err := h.queries.ListDocumentProcessingRuns(r.Context(), docID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list processing runs")
		respondError(w, http.StatusInternalServerError, "Failed to list processing runs")
		return
	}
	if runs == nil {
		runs = []models.DocumentProcessingRun{}
	}
	respondJSON(w, http.StatusOK, runs)
}

// ListEntityProcessingRuns returns all processing runs for an institution entity.
// GET /api/v1/institution-rates/{entityType}/{entityID}/processing-runs
func (h *Handler) ListEntityProcessingRuns(w http.ResponseWriter, r *http.Request) {
	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityID")
	runs, err := h.queries.ListDocumentProcessingRunsByEntity(r.Context(), entityType, entityID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list processing runs")
		respondError(w, http.StatusInternalServerError, "Failed to list processing runs")
		return
	}
	if runs == nil {
		runs = []models.DocumentProcessingRun{}
	}
	respondJSON(w, http.StatusOK, runs)
}

// decryptDocument is a helper that downloads, decrypts, and returns plaintext for a budget document.
func (h *Handler) decryptDocument(r *http.Request, doc *models.BudgetDocument) ([]byte, error) {
	if h.encryptor == nil {
		return nil, fmt.Errorf("document encryption not configured")
	}

	reader, err := h.store.Download(r.Context(), doc.S3Key)
	if err != nil {
		return nil, fmt.Errorf("downloading from S3: %w", err)
	}
	defer reader.Close()

	ciphertext, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("reading encrypted content: %w", err)
	}

	dek, err := h.encryptor.UnwrapDEK(doc.EncryptedDEK, doc.DEKNonce)
	if err != nil {
		return nil, fmt.Errorf("unwrapping DEK: %w", err)
	}

	plaintext, err := crypto.Decrypt(dek, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return plaintext, nil
}

// PreviewDocumentExtraction extracts and returns the markdown without running the LLM.
// POST /api/v1/institution-rates/{entityType}/{entityID}/budget-documents/{docID}/preview-extract
func (h *Handler) PreviewDocumentExtraction(w http.ResponseWriter, r *http.Request) {
	if h.encryptor == nil {
		respondError(w, http.StatusServiceUnavailable, "Document encryption not configured")
		return
	}

	docID := chi.URLParam(r, "docID")
	doc, err := h.queries.GetBudgetDocument(r.Context(), docID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Document not found")
		return
	}
	if doc.DeletedAt != nil {
		respondError(w, http.StatusGone, "Document has been deleted")
		return
	}

	// Download + decrypt
	dlReader, err := h.store.Download(r.Context(), doc.S3Key)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to download document")
		return
	}
	defer dlReader.Close()

	ct, err := io.ReadAll(dlReader)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to read document")
		return
	}

	dek, err := h.encryptor.UnwrapDEK(doc.EncryptedDEK, doc.DEKNonce)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Decryption error")
		return
	}
	pt, err := crypto.Decrypt(dek, ct)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Decryption error")
		return
	}

	result, err := docextract.Extract(doc.Filename, pt)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, fmt.Sprintf("Failed to extract: %s", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"filename":  doc.Filename,
		"tables":    len(result.Tables),
		"has_text":  result.RawText != "",
		"markdown":  result.ToMarkdown(),
		"text_size": len(result.ToMarkdown()),
	})
}
