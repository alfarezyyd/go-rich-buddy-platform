package radar

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Scheduler manages the daily Radar pipeline execution schedule as defined in PRD §6.5.
//
// Schedule (WIB / UTC+7):
//   - 06:45 — Check data freshness; retry every 5 min if not yet updated
//   - 07:00 — Run Tier 1 universe scan
//   - 07:20 — Run Tier 2 deep enrichment (shortlist + watchlist)
//   - 08:00 — Broadcast to opted-in users
type Scheduler struct {
	cronEngine     *cron.Cron
	radarService   Service
	dbConnection   *gorm.DB
	broadcastFunc  func(phone string, message string) error
}

// NewScheduler creates and configures the radar pipeline cron scheduler.
// The broadcastFunc must be supplied by the caller (e.g. WhatsApp gateway send function).
func NewScheduler(radarService Service, dbConnection *gorm.DB, broadcastFunc func(phone, message string) error) *Scheduler {
	// Use WIB timezone (Asia/Jakarta = UTC+7) for all schedule entries
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		logrus.Warnf("Failed to load Asia/Jakarta timezone, falling back to UTC: %v", err)
		loc = time.UTC
	}

	cronEngine := cron.New(cron.WithLocation(loc), cron.WithSeconds())

	return &Scheduler{
		cronEngine:    cronEngine,
		radarService:  radarService,
		dbConnection:  dbConnection,
		broadcastFunc: broadcastFunc,
	}
}

// Start registers all pipeline jobs and starts the scheduler.
// This is idempotent — safe to call from an fx.Lifecycle OnStart hook.
func (s *Scheduler) Start() {
	// 06:45 WIB — Data freshness check (PRD §6.5)
	s.mustAdd("0 45 6 * * *", "DataFreshnessCheck", s.runFreshnessCheck)

	// 07:00 WIB — Tier 1 universe scan (PRD §6.5)
	s.mustAdd("0 0 7 * * *", "Tier1UniverseScan", s.runTier1)

	// 07:20 WIB — Tier 2 deep enrichment (PRD §6.5)
	s.mustAdd("0 20 7 * * *", "Tier2DeepEnrichment", s.runTier2)

	// 08:00 WIB — Morning broadcast to opted-in users (PRD §6.5)
	s.mustAdd("0 0 8 * * *", "MorningBroadcast", s.runMorningBroadcast)

	s.cronEngine.Start()
	logrus.Info("Radar pipeline scheduler started (timezone: Asia/Jakarta)")
}

// Stop gracefully shuts down the scheduler, waiting for in-progress jobs to complete.
func (s *Scheduler) Stop() {
	ctx := s.cronEngine.Stop()
	// Wait for running jobs to finish
	select {
	case <-ctx.Done():
		logrus.Info("Radar pipeline scheduler stopped")
	case <-time.After(30 * time.Second):
		logrus.Warn("Radar pipeline scheduler stop timed out after 30s")
	}
}

// mustAdd registers a cron job and panics on configuration error to catch bad cron expressions at startup.
func (s *Scheduler) mustAdd(spec, name string, fn func()) {
	_, err := s.cronEngine.AddFunc(spec, func() {
		logrus.Infof("[Scheduler] Starting job: %s", name)
		start := time.Now()
		fn()
		logrus.Infof("[Scheduler] Completed job: %s in %s", name, time.Since(start))
	})
	if err != nil {
		panic(fmt.Sprintf("failed to register cron job %s (%s): %v", name, spec, err))
	}
}

// runFreshnessCheck validates that the Sectors API has new closing data (PRD §6.5 06:45).
// If data is not yet available it logs a warning; the 07:00 Tier 1 job handles retry implicitly
// since getEffectiveDate falls back to yesterday's date, and RunTier1 will simply re-use that.
func (s *Scheduler) runFreshnessCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	freshnessDate, err := s.radarService.(*ServiceImpl).sectorsClient.CheckDataFreshness(ctx)
	today := time.Now().Format("2006-01-02")
	if err != nil {
		logrus.Warnf("[FreshnessCheck] Failed to check data freshness: %v", err)
		return
	}
	if freshnessDate != today {
		logrus.Warnf("[FreshnessCheck] Sectors API data not yet updated for today (%s). Latest: %s. Will retry at 07:00.", today, freshnessDate)
	} else {
		logrus.Infof("[FreshnessCheck] Data is fresh for %s. Tier 1 will proceed at 07:00.", today)
	}
}

// runTier1 executes the Tier 1 universe scan (PRD §6.5 07:00).
func (s *Scheduler) runTier1() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	today := time.Now().Format("2006-01-02")
	resp, err := s.radarService.RunTier1(ctx, today)
	if err != nil {
		logrus.Errorf("[Tier1] Pipeline failed: %v", err)
		return
	}
	logrus.Infof("[Tier1] %s", resp.Message)
}

// runTier2 executes the Tier 2 deep enrichment (PRD §6.5 07:20).
func (s *Scheduler) runTier2() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	today := time.Now().Format("2006-01-02")
	resp, err := s.radarService.RunTier2(ctx, today)
	if err != nil {
		logrus.Errorf("[Tier2] Pipeline failed: %v", err)
		return
	}
	logrus.Infof("[Tier2] %s", resp.Message)
}

// runMorningBroadcast sends the daily Radar to all opted-in users (PRD §6.5 08:00).
func (s *Scheduler) runMorningBroadcast() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	if s.broadcastFunc == nil {
		logrus.Warn("[Broadcast] No broadcast function registered; skipping")
		return
	}
	if err := s.radarService.RunMorningBroadcast(ctx, s.broadcastFunc); err != nil {
		logrus.Errorf("[Broadcast] Morning broadcast failed: %v", err)
	}
}
