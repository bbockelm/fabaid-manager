package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/fabaid-manager/internal/backup"
	"github.com/bbockelm/fabaid-manager/internal/config"
	"github.com/bbockelm/fabaid-manager/internal/crypto"
	"github.com/bbockelm/fabaid-manager/internal/db"
	"github.com/bbockelm/fabaid-manager/internal/router"
	"github.com/bbockelm/fabaid-manager/internal/storage"
)

func main() {
	// Logger setup
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	if cfg.IsDevelopment() {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Info().Msg("Running in development mode")
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Ensure a document master key exists (generates one on first run).
	if err := cfg.EnsureMasterKey(); err != nil {
		log.Fatal().Err(err).Msg("Failed to ensure document master key")
	}
	// Derive the session HMAC secret from the master key.
	if err := cfg.DeriveSessionSecret(); err != nil {
		log.Fatal().Err(err).Msg("Failed to derive session secret")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database connection
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer pool.Close()

	// Run migrations
	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatal().Err(err).Msg("Failed to run migrations")
	}

	// S3 storage
	store, err := storage.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize storage")
	}

	// Router
	r, h := router.New(cfg, pool, store)

	// Initialize document encryption + backup service
	var enc *crypto.Encryptor
	if cfg.InstanceKey != "" {
		var err error
		enc, err = crypto.NewEncryptor(cfg.InstanceKey)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize document encryption")
		}
	}

	queries := db.NewQueries(pool)

	// Clean up any backups left in 'running' state from a previous crash/restart
	if n, err := queries.FailStaleBackups(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to clean up stale backups")
	} else if n > 0 {
		log.Info().Int64("count", n).Msg("Marked stale running backups as failed")
	}

	backupSvc := backup.NewService(cfg, queries, store, enc)
	h.SetBackupService(backupSvc)

	// Start backup scheduler
	scheduler := backup.NewScheduler(backupSvc)
	scheduler.Start(ctx)
	defer scheduler.Stop()

	addr := fmt.Sprintf(":%s", cfg.AppPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info().Msg("Shutting down server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Fatal().Err(err).Msg("Server shutdown failed")
		}
	}()

	log.Info().Str("addr", addr).Msg("Starting FabAID Manager")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Server failed")
	}
	log.Info().Msg("Server stopped")
}
