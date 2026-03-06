package handlers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bbockelm/fabaid-manager/internal/crypto"
	"github.com/bbockelm/fabaid-manager/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// ListBackups returns all backup records.
func (h *Handler) ListBackups(w http.ResponseWriter, r *http.Request) {
	backups, err := h.queries.ListBackups(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list backups")
		return
	}
	if backups == nil {
		backups = []models.Backup{}
	}
	respondJSON(w, http.StatusOK, backups)
}

// TriggerBackup starts a manual backup.
func (h *Handler) TriggerBackup(w http.ResponseWriter, r *http.Request) {
	if h.backupSvc == nil {
		respondError(w, http.StatusServiceUnavailable, "backup service not configured")
		return
	}

	// Create the backup record synchronously so the UI can see it immediately.
	backup, err := h.backupSvc.StartBackup(r.Context(), "manual")
	if err != nil {
		log.Error().Err(err).Msg("Failed to start backup")
		respondError(w, http.StatusInternalServerError, "failed to start backup: "+err.Error())
		return
	}

	go func() {
		// Use a detached context — the HTTP request context is canceled once the response is sent.
		ctx := context.Background()
		if err := h.backupSvc.RunBackup(ctx, backup); err != nil {
			log.Error().Err(err).Str("backup_id", backup.ID).Msg("Manual backup failed")
		}
	}()

	respondJSON(w, http.StatusAccepted, map[string]string{"status": "backup started", "id": backup.ID})
}

// DownloadBackup streams a backup file to the client.
func (h *Handler) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	backupID := chi.URLParam(r, "backupID")
	if h.backupSvc == nil {
		respondError(w, http.StatusServiceUnavailable, "backup service not configured")
		return
	}

	reader, backup, err := h.backupSvc.DownloadBackup(r.Context(), backupID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, backup.Filename))
	if _, err := io.Copy(w, reader); err != nil {
		log.Error().Err(err).Str("backup_id", backupID).Msg("Error streaming backup download")
	}
}

// RestoreBackup restores from an existing backup in S3.
func (h *Handler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	backupID := chi.URLParam(r, "backupID")
	if h.backupSvc == nil {
		respondError(w, http.StatusServiceUnavailable, "backup service not configured")
		return
	}

	if err := h.backupSvc.RestoreFromBackup(r.Context(), backupID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "restore completed"})
}

// DeleteBackup removes a backup from S3 and the database.
func (h *Handler) DeleteBackup(w http.ResponseWriter, r *http.Request) {
	backupID := chi.URLParam(r, "backupID")
	if h.backupSvc == nil {
		respondError(w, http.StatusServiceUnavailable, "backup service not configured")
		return
	}

	if err := h.backupSvc.DeleteBackup(r.Context(), backupID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}

// UploadRestore accepts a backup file upload and restores from it.
func (h *Handler) UploadRestore(w http.ResponseWriter, r *http.Request) {
	if h.backupSvc == nil {
		respondError(w, http.StatusServiceUnavailable, "backup service not configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 2<<30) // 2GB limit

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing file in upload")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to read uploaded file")
		return
	}

	encrypted := r.FormValue("encrypted") != "false" // default true
	decryptKey := r.FormValue("decrypt_key")          // optional: per-backup or general backup key hex
	filename := fileHeader.Filename                     // use uploaded filename for key derivation

	if err := h.backupSvc.RestoreFromUpload(r.Context(), data, encrypted, filename, decryptKey); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "restore completed"})
}

// GetBackupSettings returns the current backup configuration.
func (h *Handler) GetBackupSettings(w http.ResponseWriter, r *http.Request) {
	if h.backupSvc == nil {
		respondError(w, http.StatusServiceUnavailable, "backup service not configured")
		return
	}
	settings := h.backupSvc.GetSettings(r.Context())
	respondJSON(w, http.StatusOK, settings)
}

// UpdateBackupSettings saves backup configuration.
func (h *Handler) UpdateBackupSettings(w http.ResponseWriter, r *http.Request) {
	if h.backupSvc == nil {
		respondError(w, http.StatusServiceUnavailable, "backup service not configured")
		return
	}

	var settings models.BackupSettings
	if err := decodeJSON(r, &settings); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.backupSvc.SaveSettings(r.Context(), settings); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, settings)
}

// CreateBackupLegacy is the old streaming backup endpoint (download-only, no encryption).
func (h *Handler) CreateBackupLegacy(w http.ResponseWriter, r *http.Request) {
	if h.backupSvc == nil {
		respondError(w, http.StatusServiceUnavailable, "backup service not configured")
		return
	}

	// Trigger a backup synchronously and return it as a download
	backup, err := h.backupSvc.CreateBackup(r.Context(), "manual")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
		return
	}

	// Stream the completed backup
	reader, _, err := h.backupSvc.DownloadBackup(r.Context(), backup.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "download failed: "+err.Error())
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, backup.Filename))
	io.Copy(w, reader)
}

// GetGeneralBackupKey returns the hex-encoded general backup decryption key.
func (h *Handler) GetGeneralBackupKey(w http.ResponseWriter, r *http.Request) {
	if h.backupSvc == nil {
		respondError(w, http.StatusServiceUnavailable, "backup service not configured")
		return
	}
	keyHex, err := h.backupSvc.GeneralBackupKeyHex()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"key": keyHex})
}

// GetPerBackupKey returns the hex-encoded per-backup decryption key for a specific backup.
func (h *Handler) GetPerBackupKey(w http.ResponseWriter, r *http.Request) {
	if h.backupSvc == nil {
		respondError(w, http.StatusServiceUnavailable, "backup service not configured")
		return
	}
	backupID := chi.URLParam(r, "backupID")
	backup, err := h.queries.GetBackup(r.Context(), backupID)
	if err != nil {
		respondError(w, http.StatusNotFound, "backup not found")
		return
	}
	keyHex, err := h.backupSvc.PerBackupKeyHex(backup.Filename)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"key": keyHex, "filename": backup.Filename})
}

// DeriveKeyFromInput derives a per-backup key from a user-provided general backup key and filename.
// This allows decryption without the server's master key.
func (h *Handler) DeriveKeyFromInput(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GeneralKey string `json:"general_key"`
		Filename   string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.GeneralKey == "" || req.Filename == "" {
		respondError(w, http.StatusBadRequest, "general_key and filename are required")
		return
	}

	// This is a pure derivation — no master key needed
	key, err := crypto.DerivePerBackupKeyFromHex(req.GeneralKey, req.Filename)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"key": hex.EncodeToString(key)})
}
