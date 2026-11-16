package integration

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/controllers"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/plugins/events"
	"github.com/openshift-online/rh-trex-ai/components/api-server/test"
)

type retentionFixture struct {
	reconciledOld       string // reconciled 9 days ago
	reconciledRecent    string // reconciled 1 day ago
	unreconciledOld     string // created 10 days ago, never reconciled
	unreconciledVeryOld string // created 40 days ago, never reconciled
}

// insertRetentionEvents writes events with chosen timestamps straight into
// the table. It skips pg_notify, so no controller picks them up.
func insertRetentionEvents(t *testing.T, h *test.Helper) retentionFixture {
	t.Helper()
	g2 := h.Env().Database.SessionFactory.New(context.Background())
	now := time.Now()
	day := 24 * time.Hour
	f := retentionFixture{
		reconciledOld:       h.NewID(),
		reconciledRecent:    h.NewID(),
		unreconciledOld:     h.NewID(),
		unreconciledVeryOld: h.NewID(),
	}
	rows := []struct {
		id         string
		created    time.Time
		reconciled *time.Time
	}{
		{f.reconciledOld, now.Add(-10 * day), ptrTime(now.Add(-9 * day))},
		{f.reconciledRecent, now.Add(-10 * day), ptrTime(now.Add(-1 * day))},
		{f.unreconciledOld, now.Add(-10 * day), nil},
		{f.unreconciledVeryOld, now.Add(-40 * day), nil},
	}
	for _, r := range rows {
		err := g2.Exec(`INSERT INTO events (id, created_at, updated_at, source, source_id, event_type, reconciled_date)
			VALUES (?, ?, ?, 'RetentionTest', ?, 'Create', ?)`,
			r.id, r.created, r.created, r.id, r.reconciled).Error
		Expect(err).NotTo(HaveOccurred())
	}
	return f
}

func ptrTime(t time.Time) *time.Time { return &t }

func remainingEventIDs(t *testing.T, h *test.Helper, ids ...string) []string {
	t.Helper()
	var found []string
	err := h.Env().Database.SessionFactory.New(context.Background()).
		Raw("SELECT id FROM events WHERE id IN (?)", ids).Scan(&found).Error
	Expect(err).NotTo(HaveOccurred())
	return found
}

func newRetentionController(h *test.Helper, maxAge time.Duration) *controllers.EventRetentionController {
	return controllers.NewEventRetentionControllerForTesting(
		events.Service(&h.Env().Services),
		db.NewAdvisoryLockFactory(h.Env().Database.SessionFactory),
		controllers.EventRetentionConfig{
			Interval:  time.Hour,
			Retention: 7 * 24 * time.Hour,
			MaxAge:    maxAge,
			BatchSize: 1, // forces the batch loop to run several times
		},
	)
}

func TestEventRetention_ReconciledOnly(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	f := insertRetentionEvents(t, h)

	res := newRetentionController(h, 0).RunOnce(context.Background())

	Expect(res.Skipped).To(BeFalse())
	Expect(res.DeletedReconciled).To(Equal(int64(1)))
	Expect(res.DeletedMaxAge).To(Equal(int64(0)))
	Expect(remainingEventIDs(t, h, f.reconciledOld, f.reconciledRecent, f.unreconciledOld, f.unreconciledVeryOld)).
		To(ConsistOf(f.reconciledRecent, f.unreconciledOld, f.unreconciledVeryOld))
}

func TestEventRetention_WithMaxAge(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	f := insertRetentionEvents(t, h)

	res := newRetentionController(h, 30*24*time.Hour).RunOnce(context.Background())

	Expect(res.Skipped).To(BeFalse())
	Expect(res.DeletedReconciled).To(Equal(int64(1)))
	Expect(res.DeletedMaxAge).To(Equal(int64(1)))
	Expect(remainingEventIDs(t, h, f.reconciledOld, f.reconciledRecent, f.unreconciledOld, f.unreconciledVeryOld)).
		To(ConsistOf(f.reconciledRecent, f.unreconciledOld))
}

func TestEventRetention_SkipsWhenAnotherReplicaHoldsLock(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	f := insertRetentionEvents(t, h)
	ctx := context.Background()

	// Another "replica" holds the retention lock in its own session.
	other := db.NewAdvisoryLockFactory(h.Env().Database.SessionFactory)
	owner, acquired, err := other.NewNonBlockingLock(ctx, "event-retention", db.EventRetention)
	Expect(err).NotTo(HaveOccurred())
	Expect(acquired).To(BeTrue())

	res := newRetentionController(h, 0).RunOnce(ctx)
	Expect(res.Skipped).To(BeTrue())
	Expect(remainingEventIDs(t, h, f.reconciledOld)).To(ConsistOf(f.reconciledOld))

	// Once released, the next run proceeds.
	other.Unlock(ctx, owner)
	res = newRetentionController(h, 0).RunOnce(ctx)
	Expect(res.Skipped).To(BeFalse())
	Expect(remainingEventIDs(t, h, f.reconciledOld)).To(BeEmpty())
}

func TestEventRetention_CreatedAtIndexExists(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	var indexes []string
	err := h.Env().Database.SessionFactory.New(context.Background()).
		Raw("SELECT indexname FROM pg_indexes WHERE tablename = 'events'").Scan(&indexes).Error
	Expect(err).NotTo(HaveOccurred())
	Expect(indexes).To(ContainElements("idx_events_created_at", "idx_events_reconciled_date"))
}
