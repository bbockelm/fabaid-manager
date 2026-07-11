// Package backup provides automated and manual backup creation, encryption,
// storage, and restore functionality.
package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/fabaid-manager/internal/config"
	"github.com/bbockelm/fabaid-manager/internal/crypto"
	"github.com/bbockelm/fabaid-manager/internal/db"
	"github.com/bbockelm/fabaid-manager/internal/models"
	"github.com/bbockelm/fabaid-manager/internal/storage"
)

// Service manages backup creation, encryption, and storage.
type Service struct {
	cfg            *config.Config
	queries        *db.Queries
	store          *storage.Store
	enc            *crypto.Encryptor
	masterKeyHex   string   // needed for backup key derivation
	generalBackupKey []byte // cached general backup key
}

// NewService creates a backup service.
func NewService(cfg *config.Config, queries *db.Queries, store *storage.Store, enc *crypto.Encryptor) *Service {
	s := &Service{cfg: cfg, queries: queries, store: store, enc: enc}
	if cfg.InstanceKey != "" {
		s.masterKeyHex = cfg.InstanceKey
		if gbk, err := crypto.DeriveBackupKey(cfg.InstanceKey); err == nil {
			s.generalBackupKey = gbk
		}
	}
	return s
}

// GeneralBackupKeyHex returns the hex-encoded general backup key.
// This key can decrypt any backup. Never expose the master key.
func (s *Service) GeneralBackupKeyHex() (string, error) {
	if len(s.generalBackupKey) == 0 {
		return "", fmt.Errorf("encryption not configured")
	}
	return hex.EncodeToString(s.generalBackupKey), nil
}

// PerBackupKeyHex returns the hex-encoded per-backup decryption key for a specific backup filename.
func (s *Service) PerBackupKeyHex(backupFilename string) (string, error) {
	if len(s.generalBackupKey) == 0 {
		return "", fmt.Errorf("encryption not configured")
	}
	pbk, err := crypto.DerivePerBackupKey(s.generalBackupKey, backupFilename)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(pbk), nil
}

// backupS3Prefix is the key prefix for backup objects.
const backupS3Prefix = "backups/"

// GetSettings reads backup settings from app_config.
func (s *Service) GetSettings(ctx context.Context) models.BackupSettings {
	freq, _ := s.queries.GetAppConfig(ctx, "backup_frequency_hours")
	bucket, _ := s.queries.GetAppConfig(ctx, "backup_bucket")
	endpoint, _ := s.queries.GetAppConfig(ctx, "backup_endpoint")
	accessKey, _ := s.queries.GetAppConfig(ctx, "backup_access_key")
	secretKey, _ := s.queries.GetAppConfig(ctx, "backup_secret_key")
	useSSL, _ := s.queries.GetAppConfig(ctx, "backup_use_ssl")

	// Decrypt credentials if they were stored encrypted
	if s.enc != nil {
		if dec, err := s.enc.DecryptConfigValue(accessKey); err == nil {
			accessKey = dec
		}
		if dec, err := s.enc.DecryptConfigValue(secretKey); err == nil {
			secretKey = dec
		}
	}

	hours := 0
	if freq != "" {
		fmt.Sscanf(freq, "%d", &hours)
	}
	return models.BackupSettings{
		BackupFrequencyHours: hours,
		BackupBucket:         bucket,
		BackupEndpoint:       endpoint,
		BackupAccessKey:      accessKey,
		BackupSecretKey:      secretKey,
		BackupUseSSL:         useSSL == "true",
	}
}

// SaveSettings persists backup settings to app_config.
func (s *Service) SaveSettings(ctx context.Context, settings models.BackupSettings) error {
	// Encrypt credentials if the encryptor is available
	accessKey := settings.BackupAccessKey
	secretKey := settings.BackupSecretKey
	if s.enc != nil {
		if accessKey != "" {
			if enc, err := s.enc.EncryptConfigValue(accessKey); err == nil {
				accessKey = enc
			}
		}
		if secretKey != "" {
			if enc, err := s.enc.EncryptConfigValue(secretKey); err == nil {
				secretKey = enc
			}
		}
	}

	pairs := map[string]string{
		"backup_frequency_hours": fmt.Sprintf("%d", settings.BackupFrequencyHours),
		"backup_bucket":          settings.BackupBucket,
		"backup_endpoint":        settings.BackupEndpoint,
		"backup_access_key":      accessKey,
		"backup_secret_key":      secretKey,
		"backup_use_ssl":         fmt.Sprintf("%t", settings.BackupUseSSL),
	}
	for k, v := range pairs {
		if err := s.queries.SetAppConfig(ctx, k, v); err != nil {
			return fmt.Errorf("setting %s: %w", k, err)
		}
	}
	return nil
}

// backupClient returns the minio client and bucket for storing backups.
// If alternate backup S3 is configured, a separate client is created.
func (s *Service) backupClient(ctx context.Context) (*minio.Client, string, error) {
	settings := s.GetSettings(ctx)

	if settings.BackupEndpoint != "" && settings.BackupBucket != "" {
		endpoint := settings.BackupEndpoint
		endpoint = strings.TrimPrefix(endpoint, "http://")
		endpoint = strings.TrimPrefix(endpoint, "https://")

		accessKey := settings.BackupAccessKey
		secretKey := settings.BackupSecretKey
		if accessKey == "" {
			accessKey = s.cfg.S3AccessKey
		}
		if secretKey == "" {
			secretKey = s.cfg.S3SecretKey
		}

		client, err := minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: settings.BackupUseSSL,
		})
		if err != nil {
			return nil, "", fmt.Errorf("creating backup S3 client: %w", err)
		}

		// Ensure bucket exists
		exists, err := client.BucketExists(ctx, settings.BackupBucket)
		if err != nil {
			return nil, "", fmt.Errorf("checking backup bucket: %w", err)
		}
		if !exists {
			if err := client.MakeBucket(ctx, settings.BackupBucket, minio.MakeBucketOptions{}); err != nil {
				return nil, "", fmt.Errorf("creating backup bucket: %w", err)
			}
		}
		return client, settings.BackupBucket, nil
	}

	// Fall back to default S3
	bucket := settings.BackupBucket
	if bucket == "" {
		bucket = s.store.Bucket()
	}
	return s.store.Client(), bucket, nil
}

// StartBackup creates a backup DB record with status "running" and returns it.
// The caller should then invoke RunBackup in a goroutine.
func (s *Service) StartBackup(ctx context.Context, initiatedBy string) (*models.Backup, error) {
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("fabaid-backup-%s.tar.gz.enc", timestamp)
	s3Key := backupS3Prefix + filename

	_, bucket, err := s.backupClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting backup client: %w", err)
	}

	backup := &models.Backup{
		Filename:     filename,
		S3Key:        s3Key,
		S3Bucket:     bucket,
		Status:       "running",
		StatusDetail: "Initializing",
		InitiatedBy:  initiatedBy,
		Encrypted:    s.enc != nil,
	}
	if err := s.queries.CreateBackup(ctx, backup); err != nil {
		return nil, fmt.Errorf("creating backup record: %w", err)
	}
	return backup, nil
}

// RunBackup executes the actual backup work for an existing backup record.
// It updates the record with progress, and marks it completed or failed.
func (s *Service) RunBackup(ctx context.Context, backup *models.Backup) error {
	client, bucket, err := s.backupClient(ctx)
	if err != nil {
		s.queries.FailBackup(ctx, backup.ID, err.Error())
		return fmt.Errorf("getting backup client: %w", err)
	}
	_ = bucket // already stored in backup record

	// Build tarball in memory
	var tarBuf bytes.Buffer
	gzw := gzip.NewWriter(&tarBuf)
	tw := tar.NewWriter(gzw)

	// 1. Database dump
	s.queries.UpdateBackupProgress(ctx, backup.ID, "Dumping database")
	log.Info().Str("backup_id", backup.ID).Msg("Backup: starting database dump")
	if err := s.addDatabaseDump(tw); err != nil {
		s.queries.FailBackup(ctx, backup.ID, err.Error())
		return fmt.Errorf("database dump: %w", err)
	}

	// 2. All S3 documents
	s.queries.UpdateBackupProgress(ctx, backup.ID, "Copying documents from S3")
	log.Info().Str("backup_id", backup.ID).Msg("Backup: copying S3 documents")
	if err := s.addS3Documents(ctx, tw); err != nil {
		s.queries.FailBackup(ctx, backup.ID, err.Error())
		return fmt.Errorf("S3 documents: %w", err)
	}

	tw.Close()
	gzw.Close()

	// 3. Encrypt the tarball using per-backup key
	s.queries.UpdateBackupProgress(ctx, backup.ID, "Encrypting archive")
	archive := tarBuf.Bytes()
	var finalData []byte
	if s.enc != nil && len(s.generalBackupKey) > 0 {
		// Derive per-backup key from general backup key + filename
		perBackupKey, err := crypto.DerivePerBackupKey(s.generalBackupKey, backup.Filename)
		if err != nil {
			s.queries.FailBackup(ctx, backup.ID, err.Error())
			return fmt.Errorf("deriving per-backup key: %w", err)
		}

		dek, err := crypto.GenerateDEK()
		if err != nil {
			s.queries.FailBackup(ctx, backup.ID, err.Error())
			return fmt.Errorf("generating DEK: %w", err)
		}
		encData, err := crypto.Encrypt(dek, archive)
		if err != nil {
			s.queries.FailBackup(ctx, backup.ID, err.Error())
			return fmt.Errorf("encrypting backup: %w", err)
		}
		wrappedDEK, nonce, err := crypto.WrapDEKWithKey(perBackupKey, dek)
		if err != nil {
			s.queries.FailBackup(ctx, backup.ID, err.Error())
			return fmt.Errorf("wrapping DEK: %w", err)
		}

		// Format: [2-byte nonce len][nonce][2-byte wrapped DEK len][wrapped DEK][encrypted data]
		var buf bytes.Buffer
		nonceLen := uint16(len(nonce))
		dekLen := uint16(len(wrappedDEK))
		buf.WriteByte(byte(nonceLen >> 8))
		buf.WriteByte(byte(nonceLen & 0xff))
		buf.Write(nonce)
		buf.WriteByte(byte(dekLen >> 8))
		buf.WriteByte(byte(dekLen & 0xff))
		buf.Write(wrappedDEK)
		buf.Write(encData)
		finalData = buf.Bytes()
	} else {
		finalData = archive
	}

	// 4. Compute checksum
	hash := sha256.Sum256(finalData)
	checksum := hex.EncodeToString(hash[:])

	// 5. Upload to backup S3
	s.queries.UpdateBackupProgress(ctx, backup.ID, "Uploading to S3")
	log.Info().Str("backup_id", backup.ID).Int("size", len(finalData)).Msg("Backup: uploading to S3")
	_, err = client.PutObject(ctx, backup.S3Bucket, backup.S3Key, bytes.NewReader(finalData), int64(len(finalData)),
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		s.queries.FailBackup(ctx, backup.ID, err.Error())
		return fmt.Errorf("uploading backup: %w", err)
	}

	// 6. Record object hash
	s.queries.UpsertObjectHash(ctx, backup.S3Key, checksum, int64(len(finalData)))

	// 7. Mark complete
	if err := s.queries.CompleteBackup(ctx, backup.ID, int64(len(finalData)), checksum); err != nil {
		return fmt.Errorf("completing backup record: %w", err)
	}

	backup.SizeBytes = int64(len(finalData))
	backup.Checksum = checksum
	backup.Status = "completed"
	log.Info().Str("backup_id", backup.ID).Str("checksum", checksum).Msg("Backup completed")
	return nil
}

// CreateBackup is a convenience wrapper that starts and runs a backup synchronously.
func (s *Service) CreateBackup(ctx context.Context, initiatedBy string) (*models.Backup, error) {
	backup, err := s.StartBackup(ctx, initiatedBy)
	if err != nil {
		return nil, err
	}
	if err := s.RunBackup(ctx, backup); err != nil {
		return backup, err
	}
	return backup, nil
}

// sanitizeDumpSQL removes SET commands for configuration parameters that may not
// exist on the target server, so a dump taken from a newer PostgreSQL can be
// restored into an older one. The notable case is transaction_timeout, which
// pg_dump 17+ emits in the preamble but PostgreSQL 16 and earlier reject with
// "unrecognized configuration parameter". These are per-session timeouts that are
// irrelevant to a restore, so dropping them is safe.
func sanitizeDumpSQL(sql string) string {
	unknownParams := []string{"transaction_timeout"}
	var b strings.Builder
	b.Grow(len(sql))
	for _, line := range strings.Split(sql, "\n") {
		low := strings.ToLower(strings.TrimSpace(line))
		skip := false
		for _, p := range unknownParams {
			if strings.HasPrefix(low, "set "+p) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func (s *Service) addDatabaseDump(tw *tar.Writer) error {
	// Use --inserts (not COPY) so the output is pure SQL compatible with pgx Exec.
	// --clean --if-exists drops existing objects before recreating them so restores
	// work against a database that already has tables (e.g. from migrations).
	// --no-owner and --no-privileges avoid role-specific commands that may not apply on restore.
	cmd := exec.Command("pg_dump",
		"--inserts", "--clean", "--if-exists",
		"--no-owner", "--no-privileges",
		s.cfg.DatabaseURL,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("pg_dump: %w: %s", err, detail)
		}
		return fmt.Errorf("pg_dump: %w", err)
	}

	// Strip psql-specific backslash meta-commands (e.g. \set, \connect) that
	// are not valid SQL and would cause errors when replayed via pgx Exec.
	var filtered bytes.Buffer
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "\\") {
			continue
		}
		filtered.WriteString(line)
		filtered.WriteByte('\n')
	}
	// Also drop version-specific session settings (e.g. transaction_timeout, added
	// in PostgreSQL 17) so backups are portable across major PostgreSQL versions.
	output := []byte(sanitizeDumpSQL(filtered.String()))

	header := &tar.Header{
		Name:    "database/fabaid.sql",
		Mode:    0644,
		Size:    int64(len(output)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("writing tar header: %w", err)
	}
	if _, err := tw.Write(output); err != nil {
		return fmt.Errorf("writing dump: %w", err)
	}
	return nil
}

func (s *Service) addS3Documents(ctx context.Context, tw *tar.Writer) error {
	// 1. Regular documents (stored unencrypted in S3)
	docs, err := s.queries.ListAllDocuments(ctx)
	if err != nil {
		return fmt.Errorf("listing documents: %w", err)
	}

	for _, doc := range docs {
		reader, err := s.store.Download(ctx, doc.S3Key)
		if err != nil {
			log.Warn().Err(err).Str("key", doc.S3Key).Msg("Skipping document in backup")
			continue
		}

		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			log.Warn().Err(err).Str("key", doc.S3Key).Msg("Failed to read document")
			continue
		}

		// Update object hash
		hash := sha256.Sum256(data)
		s.queries.UpsertObjectHash(ctx, doc.S3Key, hex.EncodeToString(hash[:]), int64(len(data)))

		header := &tar.Header{
			Name:    fmt.Sprintf("documents/%s/%s/%s", doc.EntityType, doc.EntityID, doc.Filename),
			Mode:    0644,
			Size:    int64(len(data)),
			ModTime: doc.CreatedAt,
		}
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("writing header for %s: %w", doc.S3Key, err)
		}
		if _, err := tw.Write(data); err != nil {
			return fmt.Errorf("writing document: %w", err)
		}
	}

	// 2. Budget documents (stored encrypted in S3 — decrypt before archiving)
	budgetDocs, err := s.queries.ListAllBudgetDocuments(ctx)
	if err != nil {
		return fmt.Errorf("listing budget documents: %w", err)
	}

	for _, bdoc := range budgetDocs {
		reader, err := s.store.Download(ctx, bdoc.S3Key)
		if err != nil {
			log.Warn().Err(err).Str("key", bdoc.S3Key).Msg("Skipping budget document in backup")
			continue
		}

		ciphertext, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			log.Warn().Err(err).Str("key", bdoc.S3Key).Msg("Failed to read budget document")
			continue
		}

		// Decrypt if we have an encryptor and the doc has encryption metadata
		var plaintext []byte
		if s.enc != nil && len(bdoc.EncryptedDEK) > 0 && len(bdoc.DEKNonce) > 0 {
			dek, err := s.enc.UnwrapDEK(bdoc.EncryptedDEK, bdoc.DEKNonce)
			if err != nil {
				log.Warn().Err(err).Str("key", bdoc.S3Key).Msg("Failed to unwrap DEK for budget document, storing encrypted")
				plaintext = ciphertext
			} else {
				decrypted, err := crypto.Decrypt(dek, ciphertext)
				if err != nil {
					log.Warn().Err(err).Str("key", bdoc.S3Key).Msg("Failed to decrypt budget document, storing encrypted")
					plaintext = ciphertext
				} else {
					plaintext = decrypted
				}
			}
		} else {
			plaintext = ciphertext
		}

		// Update object hash (of the plaintext)
		hash := sha256.Sum256(plaintext)
		s.queries.UpsertObjectHash(ctx, bdoc.S3Key, hex.EncodeToString(hash[:]), int64(len(plaintext)))

		header := &tar.Header{
			Name:    fmt.Sprintf("budget-documents/%s/%s/%s", bdoc.EntityType, bdoc.EntityID, bdoc.Filename),
			Mode:    0644,
			Size:    int64(len(plaintext)),
			ModTime: bdoc.CreatedAt,
		}
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("writing header for %s: %w", bdoc.S3Key, err)
		}
		if _, err := tw.Write(plaintext); err != nil {
			return fmt.Errorf("writing budget document: %w", err)
		}
	}

	return nil
}

// DownloadBackup streams a backup file from the backup S3.
func (s *Service) DownloadBackup(ctx context.Context, backupID string) (io.ReadCloser, *models.Backup, error) {
	b, err := s.queries.GetBackup(ctx, backupID)
	if err != nil {
		return nil, nil, fmt.Errorf("backup not found: %w", err)
	}
	if b.Status != "completed" {
		return nil, nil, fmt.Errorf("backup is not completed (status: %s)", b.Status)
	}

	client, _, err := s.backupClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("getting backup client: %w", err)
	}

	obj, err := client.GetObject(ctx, b.S3Bucket, b.S3Key, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("downloading backup from S3: %w", err)
	}
	return obj, b, nil
}

// DecryptBackup decrypts an encrypted backup archive, returning the raw tar.gz data.
// If decryptKeyHex is provided, it is used as the per-backup decryption key.
// Otherwise, the server derives the key from its master key.
func (s *Service) DecryptBackup(encryptedData []byte, backupFilename string, decryptKeyHex string) ([]byte, error) {
	// Determine the per-backup key to use for unwrapping
	var perBackupKey []byte

	if decryptKeyHex != "" {
		// Caller provided a key — could be per-backup key (32 bytes) or general backup key (32 bytes).
		providedKey, err := hex.DecodeString(decryptKeyHex)
		if err != nil {
			return nil, fmt.Errorf("invalid decryption key hex: %w", err)
		}
		if len(providedKey) != 32 {
			return nil, fmt.Errorf("decryption key must be 32 bytes (64 hex chars), got %d", len(providedKey))
		}
		// Try as per-backup key first; if that fails, try as general backup key
		perBackupKey = providedKey
	} else if len(s.generalBackupKey) > 0 {
		// Derive from server's own general backup key
		key, err := crypto.DerivePerBackupKey(s.generalBackupKey, backupFilename)
		if err != nil {
			return nil, fmt.Errorf("deriving per-backup key: %w", err)
		}
		perBackupKey = key
	} else {
		// Not encrypted or no key available; return as-is
		return encryptedData, nil
	}

	if len(encryptedData) < 4 {
		return nil, fmt.Errorf("encrypted data too short")
	}

	// Parse header: [2-byte nonce len][nonce][2-byte wrapped DEK len][wrapped DEK][encrypted data]
	offset := 0
	nonceLen := int(encryptedData[0])<<8 | int(encryptedData[1])
	offset += 2
	if offset+nonceLen > len(encryptedData) {
		return nil, fmt.Errorf("invalid nonce length")
	}
	nonce := encryptedData[offset : offset+nonceLen]
	offset += nonceLen

	if offset+2 > len(encryptedData) {
		return nil, fmt.Errorf("invalid DEK length header")
	}
	dekLen := int(encryptedData[offset])<<8 | int(encryptedData[offset+1])
	offset += 2
	if offset+dekLen > len(encryptedData) {
		return nil, fmt.Errorf("invalid DEK length")
	}
	wrappedDEK := encryptedData[offset : offset+dekLen]
	offset += dekLen

	// Try unwrapping with the per-backup key
	dek, err := crypto.UnwrapDEKWithKey(perBackupKey, wrappedDEK, nonce)
	if err != nil && decryptKeyHex != "" {
		// If the provided key didn't work as a per-backup key, try it as a general backup key
		derivedKey, deriveErr := crypto.DerivePerBackupKey(perBackupKey, backupFilename)
		if deriveErr == nil {
			dek, err = crypto.UnwrapDEKWithKey(derivedKey, wrappedDEK, nonce)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("unwrapping DEK: %w", err)
	}

	plaintext, err := crypto.Decrypt(dek, encryptedData[offset:])
	if err != nil {
		return nil, fmt.Errorf("decrypting backup: %w", err)
	}

	return plaintext, nil
}

// RestoreFromBackup restores a backup from S3: drops/recreates data and re-imports.
func (s *Service) RestoreFromBackup(ctx context.Context, backupID string) error {
	reader, b, err := s.DownloadBackup(ctx, backupID)
	if err != nil {
		return err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("reading backup: %w", err)
	}

	// Verify checksum
	hash := sha256.Sum256(data)
	if b.Checksum != "" && hex.EncodeToString(hash[:]) != b.Checksum {
		return fmt.Errorf("checksum mismatch: backup may be corrupted")
	}

	return s.restoreFromData(ctx, data, b.Encrypted, b.Filename, "")
}

// RestoreFromUpload restores from uploaded backup data.
// decryptKeyHex is the per-backup key or general backup key provided by the user.
func (s *Service) RestoreFromUpload(ctx context.Context, data []byte, encrypted bool, filename string, decryptKeyHex string) error {
	return s.restoreFromData(ctx, data, encrypted, filename, decryptKeyHex)
}

func (s *Service) restoreFromData(ctx context.Context, data []byte, encrypted bool, filename string, decryptKeyHex string) error {
	archiveData := data
	if encrypted {
		var err error
		archiveData, err = s.DecryptBackup(data, filename, decryptKeyHex)
		if err != nil {
			return fmt.Errorf("decrypting: %w", err)
		}
	}

	gzr, err := gzip.NewReader(bytes.NewReader(archiveData))
	if err != nil {
		return fmt.Errorf("opening gzip: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var sqlDump []byte

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		entryData, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("reading entry %s: %w", header.Name, err)
		}

		if header.Name == "database/fabaid.sql" {
			sqlDump = entryData
		} else if strings.HasPrefix(header.Name, "documents/") {
			// Re-upload document to S3
			parts := strings.SplitN(strings.TrimPrefix(header.Name, "documents/"), "/", 3)
			if len(parts) == 3 {
				s3Key := storage.GenerateKey(parts[0], parts[1], parts[2])
				if uploadErr := s.store.Upload(ctx, s3Key, bytes.NewReader(entryData), int64(len(entryData)), "application/octet-stream"); uploadErr != nil {
					log.Warn().Err(uploadErr).Str("key", s3Key).Msg("Failed to restore document to S3")
				}
			}
		}
	}

	// Apply SQL dump
	if sqlDump != nil {
		log.Info().Msg("Restore: resetting schema and applying database dump")
		// Reset the public schema first so the dump fully defines the target state.
		// Applying a --clean dump on top of an existing (possibly newer) schema fails
		// on drop-dependency ordering — e.g. the dump tries to drop wbs_areas' primary
		// key while a foreign key from a table the older dump doesn't know about (like
		// invoice_expense_wbs from a later migration) still depends on it. Wiping the
		// schema first makes the dump's `DROP ... IF EXISTS` statements no-ops and lets
		// its CREATEs rebuild everything. Prepended in one batch so the restore stays
		// atomic (ExecRaw runs the whole string in a single implicit transaction).
		script := "DROP SCHEMA IF EXISTS public CASCADE;\nCREATE SCHEMA public;\n" + sanitizeDumpSQL(string(sqlDump))
		if err := s.queries.ExecRaw(ctx, script); err != nil {
			return fmt.Errorf("restoring database: %w", err)
		}
		log.Info().Msg("Restore: database dump applied")
	}

	return nil
}

// DeleteBackup removes a backup from S3 and the database.
func (s *Service) DeleteBackup(ctx context.Context, backupID string) error {
	b, err := s.queries.GetBackup(ctx, backupID)
	if err != nil {
		return fmt.Errorf("backup not found: %w", err)
	}

	client, _, err := s.backupClient(ctx)
	if err != nil {
		return fmt.Errorf("getting backup client: %w", err)
	}

	if err := client.RemoveObject(ctx, b.S3Bucket, b.S3Key, minio.RemoveObjectOptions{}); err != nil {
		log.Warn().Err(err).Str("key", b.S3Key).Msg("Failed to delete backup from S3")
	}

	s.queries.DeleteObjectHash(ctx, b.S3Key)
	return s.queries.DeleteBackupRecord(ctx, backupID)
}

// DeleteFailedBackups removes all failed backups from S3 and the database.
func (s *Service) DeleteFailedBackups(ctx context.Context) (int, error) {
	failed, err := s.queries.ListFailedBackups(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing failed backups: %w", err)
	}

	client, _, cErr := s.backupClient(ctx)

	deleted := 0
	for _, b := range failed {
		// Best-effort S3 cleanup
		if cErr == nil && b.S3Key != "" {
			if err := client.RemoveObject(ctx, b.S3Bucket, b.S3Key, minio.RemoveObjectOptions{}); err != nil {
				log.Warn().Err(err).Str("key", b.S3Key).Msg("Failed to delete failed backup from S3")
			}
			s.queries.DeleteObjectHash(ctx, b.S3Key)
		}
		if err := s.queries.DeleteBackupRecord(ctx, b.ID); err != nil {
			log.Warn().Err(err).Str("id", b.ID).Msg("Failed to delete failed backup record")
			continue
		}
		deleted++
	}
	return deleted, nil
}
