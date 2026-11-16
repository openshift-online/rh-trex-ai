package config

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
)

// EventRetentionConfig controls the background job that deletes old rows from
// the events table. See specs/framework/event-driven-controllers.spec.md.
type EventRetentionConfig struct {
	// Interval between runs. A value of zero or less turns the job off.
	Interval time.Duration `json:"interval"`
	// Period is how long a reconciled event is kept after its reconciled_date.
	Period time.Duration `json:"period"`
	// MaxAge deletes any event older than this, reconciled or not.
	// A value of zero or less turns this cap off.
	MaxAge time.Duration `json:"max_age"`
	// BatchSize is the most rows one DELETE statement removes.
	BatchSize int `json:"batch_size"`
	// BatchPause is the wait between two DELETE statements.
	BatchPause time.Duration `json:"batch_pause"`
	// InitialDelay is the wait before the first run after startup.
	InitialDelay time.Duration `json:"initial_delay"`
}

func NewEventRetentionConfig() *EventRetentionConfig {
	return &EventRetentionConfig{
		Interval:     time.Hour,
		Period:       7 * 24 * time.Hour,
		MaxAge:       0,
		BatchSize:    5000,
		BatchPause:   100 * time.Millisecond,
		InitialDelay: 5 * time.Minute,
	}
}

func (c *EventRetentionConfig) AddFlags(fs *pflag.FlagSet) {
	fs.DurationVar(&c.Interval, "event-retention-interval", c.Interval,
		"Time between event retention runs; 0 or less turns the job off")
	fs.DurationVar(&c.Period, "event-retention-period", c.Period,
		"How long reconciled events are kept after they are reconciled")
	fs.DurationVar(&c.MaxAge, "event-retention-max-age", c.MaxAge,
		"Delete any event older than this, reconciled or not; 0 or less turns it off")
	fs.IntVar(&c.BatchSize, "event-retention-batch-size", c.BatchSize,
		"Most events deleted by one statement during a retention run")
	fs.DurationVar(&c.BatchPause, "event-retention-batch-pause", c.BatchPause,
		"Pause between delete batches during a retention run")
	fs.DurationVar(&c.InitialDelay, "event-retention-initial-delay", c.InitialDelay,
		"Wait before the first retention run after startup")
}

// Enabled reports whether the retention job should run.
func (c *EventRetentionConfig) Enabled() bool {
	return c.Interval > 0
}

// Validate checks the settings only when the job is on, so turning it off with
// --event-retention-interval=0 never fails startup.
func (c *EventRetentionConfig) Validate() error {
	if !c.Enabled() {
		return nil
	}
	if c.Period <= 0 {
		return &ConfigValidationError{Field: "event-retention-period",
			Message: fmt.Sprintf("must be greater than 0, got %s", c.Period)}
	}
	if c.BatchSize <= 0 {
		return &ConfigValidationError{Field: "event-retention-batch-size",
			Message: fmt.Sprintf("must be greater than 0, got %d", c.BatchSize)}
	}
	if c.BatchPause < 0 {
		return &ConfigValidationError{Field: "event-retention-batch-pause",
			Message: fmt.Sprintf("must not be negative, got %s", c.BatchPause)}
	}
	if c.InitialDelay < 0 {
		return &ConfigValidationError{Field: "event-retention-initial-delay",
			Message: fmt.Sprintf("must not be negative, got %s", c.InitialDelay)}
	}
	if c.MaxAge > 0 && c.MaxAge < c.Period {
		return &ConfigValidationError{Field: "event-retention-max-age",
			Message: fmt.Sprintf("must not be shorter than --event-retention-period (%s), got %s", c.Period, c.MaxAge)}
	}
	return nil
}
