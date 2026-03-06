package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

// Config holds all application configuration, populated from environment variables.
type Config struct {
	// App settings
	AppEnv  string `envconfig:"APP_ENV" default:"development"`
	AppPort string `envconfig:"APP_PORT" default:"8080"`
	BaseURL string `envconfig:"BASE_URL" default:"http://localhost:3000"`

	// Database
	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`

	// S3 / MinIO
	S3Endpoint     string `envconfig:"S3_ENDPOINT" required:"true"`
	S3Bucket       string `envconfig:"S3_BUCKET" default:"fabaid-documents"`
	S3AccessKey    string `envconfig:"S3_ACCESS_KEY" required:"true"`
	S3SecretKey    string `envconfig:"S3_SECRET_KEY" required:"true"`
	S3UsePathStyle bool   `envconfig:"S3_USE_PATH_STYLE" default:"true"`
	S3UseSSL       bool   `envconfig:"S3_USE_SSL" default:"false"`

	// SSO / Auth (for production)
	OIDCIssuer       string `envconfig:"OIDC_ISSUER" default:""`
	OIDCClientID     string `envconfig:"OIDC_CLIENT_ID" default:""`
	OIDCClientSecret string `envconfig:"OIDC_CLIENT_SECRET" default:""`

	// Session
	SessionSecret string `envconfig:"SESSION_SECRET" default:"dev-secret-change-me-in-prod"`

	// Backup
	BackupDir string `envconfig:"BACKUP_DIR" default:"/tmp/fabaid-backup"`

	// Document encryption
	DocumentMasterKey string `envconfig:"DOCUMENT_MASTER_KEY" default:""`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return &cfg, nil
}

// masterKeyFile is the filename used to persist an auto-generated master key.
const masterKeyFile = ".fabaid-master.key"

// EnsureMasterKey checks whether DocumentMasterKey is set. If it is empty
// it tries to load a previously-generated key from disk; failing that it
// generates a new random 32-byte key, saves it to disk, and populates the
// config field. The key file is meant to be git-ignored.
func (c *Config) EnsureMasterKey() error {
	if c.DocumentMasterKey != "" {
		return nil
	}

	// Try to read an existing key file.
	data, err := os.ReadFile(masterKeyFile)
	if err == nil {
		key := strings.TrimSpace(string(data))
		if len(key) == 64 {
			c.DocumentMasterKey = key
			log.Info().Str("file", masterKeyFile).Msg("Loaded document master key from disk")
			return nil
		}
		log.Warn().Str("file", masterKeyFile).Msg("Key file exists but content is invalid; generating a new key")
	}

	// Generate a new 32-byte random key.
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("generating master key: %w", err)
	}
	keyHex := hex.EncodeToString(buf)

	// Write 0600 so only the owner can read it.
	if err := os.WriteFile(masterKeyFile, []byte(keyHex+"\n"), 0600); err != nil {
		return fmt.Errorf("saving master key to %s: %w", masterKeyFile, err)
	}

	c.DocumentMasterKey = keyHex
	log.Info().Str("file", masterKeyFile).Msg("Generated and saved new document master key")
	return nil
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}
