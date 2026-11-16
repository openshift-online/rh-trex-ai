package controllers

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	svcerrors "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/services"
)

// eventRetentionLockID is the fixed id of the advisory lock that lets one
// replica at a time run the retention job. Together with the
// db.EventRetention lock type it can never match a per-event lock, whose
// type is db.Events.
const eventRetentionLockID = "event-retention"

// Deletion reasons, used as the "reason" label of event_retention_deleted_total.
const (
	retentionReasonReconciled = "reconciled"
	retentionReasonMaxAge     = "max_age"
)

// EventRetentionConfig configures an EventRetentionController.
// Unlike SyncControllerConfig, zero values are meaningful and are not
// replaced with defaults: Interval <= 0 turns the job off and MaxAge <= 0
// turns the age cap off. Defaults live in config.NewEventRetentionConfig.
type EventRetentionConfig struct {
	// Interval between runs. Zero or less turns the controller off.
	Interval time.Duration
	// Retention is how long a reconciled event is kept after reconciled_date.
	Retention time.Duration
	// MaxAge deletes any event older than this, reconciled or not. Zero or less is off.
	MaxAge time.Duration
	// BatchSize is the most rows one DELETE removes. Values below 1 are treated as 1.
	BatchSize int
	// BatchPause is the wait between two DELETE statements.
	BatchPause time.Duration
	// InitialDelay is the wait before the first run.
	InitialDelay time.Duration
}

// RetentionResult is what one retention run did.
type RetentionResult struct {
	// Skipped is true when another replica held the retention lock.
	Skipped           bool
	DeletedReconciled int64
	DeletedMaxAge     int64
	Duration          time.Duration
}

// EventRetentionController periodically deletes old rows from the events
// table. Reconciled events are deleted once they are older than Retention.
// When MaxAge is set, any event older than MaxAge is deleted too.
type EventRetentionController struct {
	eventService services.EventService
	lockFactory  db.LockFactory
	config       EventRetentionConfig
	now          func() time.Time
	cancel       context.CancelFunc
	done         chan struct{}
	startOnce    sync.Once
	metrics      *retentionMetrics
}

type retentionMetrics struct {
	deleted     *prometheus.CounterVec
	runs        prometheus.Counter
	skippedRuns prometheus.Counter
	errors      prometheus.Counter
	duration    prometheus.Histogram
}

func newRetentionMetrics() *retentionMetrics {
	return &retentionMetrics{
		deleted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "event_retention_deleted_total",
			Help: "Total number of events deleted by the retention job, by reason",
		}, []string{"reason"}),
		runs: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "event_retention_runs_total",
			Help: "Total number of event retention runs that took the lock",
		}),
		skippedRuns: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "event_retention_runs_skipped_total",
			Help: "Total number of event retention runs skipped because another replica held the lock",
		}),
		errors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "event_retention_errors_total",
			Help: "Total number of event retention runs that ended with an error",
		}),
		duration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "event_retention_run_duration_seconds",
			Help:    "Duration of event retention runs",
			Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 300, 900, 1800},
		}),
	}
}

// register registers the metrics. If a collector with the same name is
// already registered (for example a second controllers server in one test
// process), the existing one is reused, so this never panics.
func (m *retentionMetrics) register() {
	m.deleted = registerOrReuse(m.deleted)
	m.runs = registerOrReuse(m.runs)
	m.skippedRuns = registerOrReuse(m.skippedRuns)
	m.errors = registerOrReuse(m.errors)
	m.duration = registerOrReuse(m.duration)
}

func registerOrReuse[T prometheus.Collector](c T) T {
	if err := prometheus.Register(c); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			if existing, ok := are.ExistingCollector.(T); ok {
				return existing
			}
		}
		slog.Error("Failed to register event retention metric", "error", err)
	}
	return c
}

// NewEventRetentionController creates a retention controller and registers its metrics.
func NewEventRetentionController(eventService services.EventService, lockFactory db.LockFactory, config EventRetentionConfig) *EventRetentionController {
	return newEventRetentionController(eventService, lockFactory, config, true)
}

// NewEventRetentionControllerForTesting creates a retention controller without registering metrics.
func NewEventRetentionControllerForTesting(eventService services.EventService, lockFactory db.LockFactory, config EventRetentionConfig) *EventRetentionController {
	return newEventRetentionController(eventService, lockFactory, config, false)
}

func newEventRetentionController(eventService services.EventService, lockFactory db.LockFactory, config EventRetentionConfig, registerMetrics bool) *EventRetentionController {
	if config.BatchSize < 1 {
		config.BatchSize = 1
	}
	metrics := newRetentionMetrics()
	if registerMetrics {
		metrics.register()
	}
	return &EventRetentionController{
		eventService: eventService,
		lockFactory:  lockFactory,
		config:       config,
		now:          time.Now,
		done:         make(chan struct{}),
		metrics:      metrics,
	}
}

// Enabled reports whether Start will run the job.
func (c *EventRetentionController) Enabled() bool {
	return c.config.Interval > 0
}

// Start begins the periodic retention loop. It does nothing when the
// controller is turned off, and only the first call has an effect.
func (c *EventRetentionController) Start() {
	c.startOnce.Do(func() {
		if !c.Enabled() {
			slog.Info("Event retention controller is turned off", "interval", c.config.Interval)
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		c.cancel = cancel

		slog.InfoContext(ctx, "Starting event retention controller",
			"interval", c.config.Interval, "retention", c.config.Retention, "max_age", c.config.MaxAge,
			"batch_size", c.config.BatchSize, "batch_pause", c.config.BatchPause, "initial_delay", c.config.InitialDelay)

		go c.loop(ctx)
	})
}

// Stop cancels the loop, including a run in progress, and waits for it to return.
func (c *EventRetentionController) Stop() error {
	if c.cancel != nil {
		c.cancel()
		<-c.done
	}
	return nil
}

func (c *EventRetentionController) loop(ctx context.Context) {
	defer close(c.done)

	if !sleepCtx(ctx, c.config.InitialDelay) {
		return
	}
	c.RunOnce(ctx)

	ticker := time.NewTicker(c.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Event retention controller shutting down")
			return
		case <-ticker.C:
			c.RunOnce(ctx)
		}
	}
}

// RunOnce runs one retention pass: it takes the lock, deletes reconciled
// events past the retention period, then, if MaxAge is set, any event past
// MaxAge. Errors are logged and counted, never returned to the loop.
func (c *EventRetentionController) RunOnce(ctx context.Context) RetentionResult {
	start := time.Now()
	result := RetentionResult{}

	// The lock lives in its own transaction. It must outlive ctx being
	// cancelled, so its commit at Unlock still succeeds on shutdown.
	lockCtx := context.WithoutCancel(ctx)
	owner, acquired, err := c.lockFactory.NewNonBlockingLock(lockCtx, eventRetentionLockID, db.EventRetention)
	defer c.lockFactory.Unlock(lockCtx, owner)
	if err != nil {
		c.metrics.errors.Inc()
		slog.ErrorContext(ctx, "Event retention could not take its lock", "error", err)
		return result
	}
	if !acquired {
		c.metrics.skippedRuns.Inc()
		result.Skipped = true
		slog.InfoContext(ctx, "Event retention run skipped, another replica holds the lock")
		return result
	}
	c.metrics.runs.Inc()

	now := c.now()
	var runErr error

	reconciledCutoff := now.Add(-c.config.Retention)
	result.DeletedReconciled, runErr = c.deleteInBatches(ctx, retentionReasonReconciled, reconciledCutoff,
		c.eventService.DeleteReconciledBefore)

	if runErr == nil && c.config.MaxAge > 0 {
		maxAgeCutoff := now.Add(-c.config.MaxAge)
		result.DeletedMaxAge, runErr = c.deleteInBatches(ctx, retentionReasonMaxAge, maxAgeCutoff,
			c.eventService.DeleteCreatedBefore)
	}

	result.Duration = time.Since(start)
	c.metrics.duration.Observe(result.Duration.Seconds())

	attrs := []any{
		"deleted_reconciled", result.DeletedReconciled,
		"deleted_max_age", result.DeletedMaxAge,
		"duration", result.Duration,
	}
	switch {
	case runErr == nil:
		slog.InfoContext(ctx, "Event retention run completed", attrs...)
	case errors.Is(runErr, context.Canceled):
		slog.InfoContext(ctx, "Event retention run stopped by shutdown", attrs...)
	default:
		c.metrics.errors.Inc()
		slog.ErrorContext(ctx, "Event retention run failed", append(attrs, "error", runErr)...)
	}
	return result
}

type deleteBatchFunc func(ctx context.Context, cutoff time.Time, limit int) (int64, *svcerrors.ServiceError)

// deleteInBatches calls del until a batch deletes fewer than BatchSize rows,
// pausing BatchPause between batches. It stops early when ctx is cancelled.
func (c *EventRetentionController) deleteInBatches(ctx context.Context, reason string, cutoff time.Time, del deleteBatchFunc) (int64, error) {
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, svcErr := del(ctx, cutoff, c.config.BatchSize)
		total += n
		c.metrics.deleted.WithLabelValues(reason).Add(float64(n))
		if svcErr != nil {
			if ctx.Err() != nil {
				return total, ctx.Err()
			}
			return total, svcErr.AsError()
		}
		if n < int64(c.config.BatchSize) {
			return total, nil
		}
		if !sleepCtx(ctx, c.config.BatchPause) {
			return total, ctx.Err()
		}
	}
}

// sleepCtx waits for d or until ctx is done. It returns false if ctx is done.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
