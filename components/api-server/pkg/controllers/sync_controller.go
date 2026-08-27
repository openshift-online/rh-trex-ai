package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/services"
)

// SyncController implements periodic "sync-the-world" functionality to handle missed events.
// This addresses scenarios where:
// - PostgreSQL LISTEN/NOTIFY messages are lost due to connection issues
// - Worker processes crash before completing event processing
// - Network partitions cause temporary isolation from database
// - Queue drops cause events to be missed
type SyncController struct {
	manager          *KindControllerManager
	eventService     services.EventService
	interval         time.Duration
	maxAge           time.Duration
	maxEventsPerSync int
	cancel           context.CancelFunc
	done             chan struct{}
	startOnce        sync.Once
	metrics          *syncMetrics
}

type SyncControllerConfig struct {
	// Interval between sync runs (default: 5 minutes)
	Interval time.Duration
	// MaxAge of unreconciled events to process (default: 1 hour)
	MaxAge time.Duration
	// MaxEventsPerSync limits events processed in one sync cycle (default: 1000)
	MaxEventsPerSync int
}

type syncMetrics struct {
	syncRuns          prometheus.Counter
	eventsFound       prometheus.Counter
	eventsRequeued    prometheus.Counter
	eventsFailed      prometheus.Counter
	syncDuration      prometheus.Histogram
	syncErrors        prometheus.Counter
	oldestUnprocessed prometheus.Gauge
}

func newSyncMetrics() *syncMetrics {
	return &syncMetrics{
		syncRuns: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "controller_sync_runs_total",
			Help: "Total number of sync-the-world runs",
		}),
		eventsFound: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "controller_sync_events_found_total",
			Help: "Total number of unreconciled events found during sync",
		}),
		eventsRequeued: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "controller_sync_events_requeued_total",
			Help: "Total number of events successfully requeued during sync",
		}),
		eventsFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "controller_sync_events_failed_total",
			Help: "Total number of events that failed during sync reprocessing",
		}),
		syncDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "controller_sync_duration_seconds",
			Help:    "Duration of sync-the-world operations",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120},
		}),
		syncErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "controller_sync_errors_total",
			Help: "Total number of sync-the-world errors",
		}),
		oldestUnprocessed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "controller_oldest_unprocessed_event_age_seconds",
			Help: "Age in seconds of the oldest unprocessed event",
		}),
	}
}

func (m *syncMetrics) Register() {
	prometheus.MustRegister(m.syncRuns)
	prometheus.MustRegister(m.eventsFound)
	prometheus.MustRegister(m.eventsRequeued)
	prometheus.MustRegister(m.eventsFailed)
	prometheus.MustRegister(m.syncDuration)
	prometheus.MustRegister(m.syncErrors)
	prometheus.MustRegister(m.oldestUnprocessed)
}

// NewSyncController creates a new sync controller with the given configuration
func NewSyncController(manager *KindControllerManager, eventService services.EventService, config SyncControllerConfig) *SyncController {
	return newSyncController(manager, eventService, config, true)
}

// NewSyncControllerForTesting creates a sync controller without registering Prometheus metrics
func NewSyncControllerForTesting(manager *KindControllerManager, eventService services.EventService, config SyncControllerConfig) *SyncController {
	return newSyncController(manager, eventService, config, false)
}

func newSyncController(manager *KindControllerManager, eventService services.EventService, config SyncControllerConfig, registerMetrics bool) *SyncController {
	// Set defaults
	if config.Interval == 0 {
		config.Interval = 5 * time.Minute
	}
	if config.MaxAge == 0 {
		config.MaxAge = 1 * time.Hour
	}
	if config.MaxEventsPerSync == 0 {
		config.MaxEventsPerSync = 1000
	}

	metrics := newSyncMetrics()
	if registerMetrics {
		metrics.Register()
	}

	return &SyncController{
		manager:          manager,
		eventService:     eventService,
		interval:         config.Interval,
		maxAge:           config.MaxAge,
		maxEventsPerSync: config.MaxEventsPerSync,
		done:             make(chan struct{}),
		metrics:          metrics,
	}
}

// Start begins the periodic sync process
func (sc *SyncController) Start() {
	sc.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		sc.cancel = cancel

		log := logger.NewLogger(ctx)
		log.Infof("Starting sync controller with interval=%v, maxAge=%v, maxEvents=%d",
			sc.interval, sc.maxAge, sc.maxEventsPerSync)

		go sc.syncLoop(ctx)
	})
}

// Stop gracefully shuts down the sync controller
func (sc *SyncController) Stop() error {
	if sc.cancel != nil {
		sc.cancel()
		<-sc.done
	}
	return nil
}

// syncLoop runs the periodic sync process
func (sc *SyncController) syncLoop(ctx context.Context) {
	defer close(sc.done)

	log := logger.NewLogger(ctx)
	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	// Run initial sync immediately
	sc.performSync(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info("Sync controller shutting down")
			return
		case <-ticker.C:
			sc.performSync(ctx)
		}
	}
}

// performSync executes one sync-the-world cycle
func (sc *SyncController) performSync(ctx context.Context) {
	start := time.Now()
	log := logger.NewLogger(ctx)

	defer func() {
		duration := time.Since(start)
		sc.metrics.syncDuration.Observe(duration.Seconds())
		log.Infof("Sync cycle completed in %v", duration)
	}()

	sc.metrics.syncRuns.Inc()
	log.Info("Starting sync-the-world cycle")

	// Find unreconciled events older than maxAge
	events, svcErr := sc.eventService.FindUnreconciled(ctx, sc.maxAge)
	if svcErr != nil {
		sc.metrics.syncErrors.Inc()
		log.Error(fmt.Sprintf("Failed to find unreconciled events: %v", svcErr))
		return
	}

	eventCount := len(events)
	sc.metrics.eventsFound.Add(float64(eventCount))

	if eventCount == 0 {
		log.V(1).Info("No unreconciled events found")
		sc.metrics.oldestUnprocessed.Set(0)
		return
	}

	// Update oldest unprocessed event metric
	if eventCount > 0 {
		oldestAge := time.Since(events[0].CreatedAt).Seconds()
		sc.metrics.oldestUnprocessed.Set(oldestAge)
	}

	// Limit events per sync cycle to prevent overwhelming the system
	if eventCount > sc.maxEventsPerSync {
		log.Warning(fmt.Sprintf("Found %d unreconciled events, limiting to %d per sync cycle",
			eventCount, sc.maxEventsPerSync))
		events = events[:sc.maxEventsPerSync]
		eventCount = sc.maxEventsPerSync
	}

	log.Infof("Found %d unreconciled events to reprocess", eventCount)

	// Requeue events for processing by triggering the normal event pipeline
	// This uses the same advisory locking mechanism as real-time events
	requeued := 0
	failed := 0

	for _, event := range events {
		// Use the existing Handle method which includes advisory locking
		// This ensures proper concurrency control and prevents race conditions
		sc.manager.Handle(event.ID)

		// Check if the event was successfully processed by trying to get it again
		// If ReconciledDate is set, it was processed successfully
		checkEvent, checkErr := sc.eventService.Get(ctx, event.ID)
		if checkErr != nil {
			log.Warning(fmt.Sprintf("Failed to verify event %s processing: %v", event.ID, checkErr))
			failed++
			continue
		}

		if checkEvent.ReconciledDate != nil {
			requeued++
		} else {
			// Event still unreconciled - might be being processed by another worker
			// or handler failed. This is expected and will be retried next cycle.
			log.V(2).Infof("Event %s still unreconciled after requeue attempt", event.ID)
		}
	}

	sc.metrics.eventsRequeued.Add(float64(requeued))
	sc.metrics.eventsFailed.Add(float64(failed))

	log.Infof("Sync cycle results: %d events requeued, %d failed verification",
		requeued, failed)
}
