package backup

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Scheduler runs periodic backups based on the configured frequency.
type Scheduler struct {
	svc    *Service
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewScheduler creates a new backup scheduler.
func NewScheduler(svc *Service) *Scheduler {
	return &Scheduler{svc: svc}
}

// Start begins the periodic backup loop. It reads the frequency from settings
// and re-checks every minute so config changes are picked up dynamically.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return // already running
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	s.wg.Add(1)
	go s.loop(ctx)
	log.Info().Msg("Backup scheduler started")
}

// Stop cancels the scheduler and waits for the goroutine to exit.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.mu.Unlock()
	s.wg.Wait()
	log.Info().Msg("Backup scheduler stopped")
}

func (s *Scheduler) loop(ctx context.Context) {
	defer s.wg.Done()

	// Check every 60 seconds whether we need to run a backup.
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	// Also run an immediate check on startup.
	s.checkAndRun(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAndRun(ctx)
		}
	}
}

func (s *Scheduler) checkAndRun(ctx context.Context) {
	settings := s.svc.GetSettings(ctx)
	if settings.BackupFrequencyHours <= 0 {
		return // backups disabled
	}

	freq := time.Duration(settings.BackupFrequencyHours) * time.Hour

	// Find the most recent completed backup.
	backups, err := s.svc.queries.ListBackups(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Backup scheduler: failed to list backups")
		return
	}

	var lastCompleted *time.Time
	for _, b := range backups {
		if b.Status == "completed" && b.CompletedAt != nil {
			lastCompleted = b.CompletedAt
			break // list is sorted DESC by started_at
		}
	}

	if lastCompleted != nil && time.Since(*lastCompleted) < freq {
		return // not yet time
	}

	log.Info().Int("frequency_hours", settings.BackupFrequencyHours).Msg("Backup scheduler: starting automated backup")
	if _, err := s.svc.CreateBackup(ctx, "scheduler"); err != nil {
		log.Error().Err(err).Msg("Backup scheduler: backup failed")
	}
}
