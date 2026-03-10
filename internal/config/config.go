package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/hkdf"
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

	// SessionSecret is derived from the master key at startup (not user-configurable).
	SessionSecret string `envconfig:"-"`

	// Backup
	BackupDir string `envconfig:"BACKUP_DIR" default:"/tmp/fabaid-backup"`

	// Instance encryption key
	InstanceKey string `envconfig:"INSTANCE_KEY" default:""`

	// LLM / AI processing
	LLMAPIKey     string `envconfig:"LLM_API_KEY" default:""`
	LLMAPIKeyFile string `envconfig:"LLM_API_KEY_FILE" default:""`
	LLMAPIURL     string `envconfig:"LLM_API_URL" default:"https://api.openai.com/v1"`
	LLMModel      string `envconfig:"LLM_MODEL" default:"gpt-5.4"`
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

// EnsureMasterKey checks whether InstanceKey is set. If it is empty
// it tries to load a previously-generated key from disk; failing that it
// generates a new random 32-byte key, saves it to disk, and populates the
// config field. The key file is meant to be git-ignored.
func (c *Config) EnsureMasterKey() error {
	if c.InstanceKey != "" {
		return nil
	}

	// Try to read an existing key file.
	data, err := os.ReadFile(masterKeyFile)
	if err == nil {
		key := strings.TrimSpace(string(data))
		if len(key) == 64 {
			c.InstanceKey = key
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

	c.InstanceKey = keyHex
	log.Info().Str("file", masterKeyFile).Msg("Generated and saved new instance key")
	return nil
}

// DeriveSessionSecret derives the session HMAC secret from the master key
// via HKDF-SHA256. Must be called after EnsureMasterKey.
func (c *Config) DeriveSessionSecret() error {
	if c.InstanceKey == "" {
		return fmt.Errorf("instance key must be set before deriving session secret")
	}
	masterKey, err := hex.DecodeString(c.InstanceKey)
	if err != nil {
		return fmt.Errorf("decoding master key: %w", err)
	}
	r := hkdf.New(sha256.New, masterKey, nil, []byte("fabaid-session-secret"))
	buf := make([]byte, 32)
	if _, err := io.ReadFull(r, buf); err != nil {
		return fmt.Errorf("deriving session secret: %w", err)
	}
	c.SessionSecret = hex.EncodeToString(buf)
	return nil
}

// LoadLLMKeyFile reads the LLM API key from the file specified by
// LLM_API_KEY_FILE, if LLM_API_KEY is not already set. In development
// mode it also checks the well-known path ".fabaid-openai.key".
func (c *Config) LoadLLMKeyFile() error {
	if c.LLMAPIKey != "" {
		log.Info().Msg("LLM API key configured via environment")
		return nil
	}

	file := c.LLMAPIKeyFile
	if file == "" && c.IsDevelopment() {
		// Try well-known dev key file
		if _, err := os.Stat(".fabaid-openai.key"); err == nil {
			file = ".fabaid-openai.key"
		}
	}
	if file == "" {
		log.Warn().Msg("No LLM API key configured (set LLM_API_KEY or LLM_API_KEY_FILE); AI document processing will be unavailable")
		return nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading LLM API key file %s: %w", file, err)
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return fmt.Errorf("LLM API key file %s is empty", file)
	}
	c.LLMAPIKey = key
	log.Info().Str("file", file).Str("model", c.LLMModel).Msg("Loaded LLM API key from file")
	return nil
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}
