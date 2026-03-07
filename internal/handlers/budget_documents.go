package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/fabaid-manager/internal/crypto"
	"github.com/bbockelm/fabaid-manager/internal/models"
	"github.com/bbockelm/fabaid-manager/internal/storage"
)

// UploadBudgetDocument handles multipart upload of an encrypted budget document.
func (h *Handler) UploadBudgetDocument(w http.ResponseWriter, r *http.Request) {
	if h.encryptor == nil {
		respondError(w, http.StatusServiceUnavailable, "Document encryption not configured (INSTANCE_KEY missing)")
		return
	}

	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityID")

	// Max 50MB
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "No file provided")
		return
	}
	defer file.Close()

	// Optional form fields
	docType := r.FormValue("doc_type")
	if docType == "" {
		docType = "budget"
	}
	if docType != "budget" && docType != "budget_justification" {
		respondError(w, http.StatusBadRequest, "doc_type must be 'budget' or 'budget_justification'")
		return
	}
	budgetID := r.FormValue("budget_id")
	notes := r.FormValue("notes")

	// Read file into memory for encryption
	plaintext, err := io.ReadAll(file)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read uploaded file")
		respondError(w, http.StatusInternalServerError, "Failed to read uploaded file")
		return
	}

	// Generate per-document DEK and encrypt the file
	dek, err := crypto.GenerateDEK()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate DEK")
		respondError(w, http.StatusInternalServerError, "Encryption error")
		return
	}

	ciphertext, err := crypto.Encrypt(dek, plaintext)
	if err != nil {
		log.Error().Err(err).Msg("Failed to encrypt document")
		respondError(w, http.StatusInternalServerError, "Encryption error")
		return
	}

	// Wrap (encrypt) the DEK with the master-derived KEK
	encryptedDEK, dekNonce, err := h.encryptor.WrapDEK(dek)
	if err != nil {
		log.Error().Err(err).Msg("Failed to wrap DEK")
		respondError(w, http.StatusInternalServerError, "Encryption error")
		return
	}

	// Upload encrypted content to S3
	s3Key := storage.GenerateKey("budget-docs/"+entityType, entityID, header.Filename)
	if err := h.store.Upload(r.Context(), s3Key, bytes.NewReader(ciphertext), int64(len(ciphertext)), "application/octet-stream"); err != nil {
		log.Error().Err(err).Msg("Failed to upload encrypted document to S3")
		respondError(w, http.StatusInternalServerError, "Failed to upload file")
		return
	}

	// Determine who uploaded
	var uploadedBy *string
	user := GetUserFromContext(r.Context())
	if user != nil {
		uploadedBy = &user.ID
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/pdf"
	}

	var budgetIDPtr *string
	if budgetID != "" {
		budgetIDPtr = &budgetID
	}

	doc := models.BudgetDocument{
		EntityType:   entityType,
		EntityID:     entityID,
		BudgetID:     budgetIDPtr,
		DocType:      docType,
		Filename:     header.Filename,
		ContentType:  contentType,
		S3Key:        s3Key,
		FileSize:     header.Size,
		EncryptedDEK: encryptedDEK,
		DEKNonce:     dekNonce,
		UploadedBy:   uploadedBy,
		Notes:        notes,
	}

	if err := h.queries.CreateBudgetDocument(r.Context(), &doc); err != nil {
		log.Error().Err(err).Msg("Failed to save budget document record")
		respondError(w, http.StatusInternalServerError, "Failed to save document record")
		return
	}

	// Fill in the uploader name for the response
	if user != nil {
		doc.UploadedName = user.DisplayName
	}

	respondJSON(w, http.StatusCreated, doc)
}

// ListBudgetDocuments returns all budget documents for an institution entity.
func (h *Handler) ListBudgetDocuments(w http.ResponseWriter, r *http.Request) {
	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityID")

	includeDeleted := r.URL.Query().Get("include_deleted") == "true"
	docs, err := h.queries.ListBudgetDocuments(r.Context(), entityType, entityID, includeDeleted)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list budget documents")
		respondError(w, http.StatusInternalServerError, "Failed to list documents")
		return
	}
	if docs == nil {
		docs = []models.BudgetDocument{}
	}
	respondJSON(w, http.StatusOK, docs)
}

// GetBudgetDocumentInfo returns metadata for a single budget document.
func (h *Handler) GetBudgetDocumentInfo(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docID")
	doc, err := h.queries.GetBudgetDocument(r.Context(), docID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Document not found")
		return
	}
	respondJSON(w, http.StatusOK, doc)
}

// DownloadBudgetDocument decrypts and streams a budget document.
func (h *Handler) DownloadBudgetDocument(w http.ResponseWriter, r *http.Request) {
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

	// Download encrypted content from S3
	reader, err := h.store.Download(r.Context(), doc.S3Key)
	if err != nil {
		log.Error().Err(err).Str("s3_key", doc.S3Key).Msg("Failed to download from S3")
		respondError(w, http.StatusInternalServerError, "Failed to download file")
		return
	}
	defer reader.Close()

	ciphertext, err := io.ReadAll(reader)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read encrypted content")
		respondError(w, http.StatusInternalServerError, "Failed to read file")
		return
	}

	// Unwrap the DEK
	dek, err := h.encryptor.UnwrapDEK(doc.EncryptedDEK, doc.DEKNonce)
	if err != nil {
		log.Error().Err(err).Msg("Failed to unwrap DEK")
		respondError(w, http.StatusInternalServerError, "Decryption error — wrong master key?")
		return
	}

	// Decrypt the document
	plaintext, err := crypto.Decrypt(dek, ciphertext)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decrypt document")
		respondError(w, http.StatusInternalServerError, "Decryption error")
		return
	}

	w.Header().Set("Content-Type", doc.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, doc.Filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(plaintext)))
	w.WriteHeader(http.StatusOK)
	w.Write(plaintext)
}

// SoftDeleteBudgetDocument marks a budget document as deleted without removing data.
func (h *Handler) SoftDeleteBudgetDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docID")

	user := GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	if err := h.queries.SoftDeleteBudgetDocument(r.Context(), docID, user.ID); err != nil {
		log.Error().Err(err).Msg("Failed to soft-delete budget document")
		respondError(w, http.StatusInternalServerError, "Failed to delete document")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
