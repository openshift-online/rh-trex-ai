package controllers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/dao/mocks"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	svcerrors "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/services"
)

// fakeRetentionService records delete calls and returns scripted batch sizes.
// It embeds services.EventService so only the delete methods are implemented.
type fakeRetentionService struct {
	services.EventService

	mu               sync.Mutex
	reconciledBatch  []int64 // returned in order; 0 once exhausted
	createdBatch     []int64
	reconciledCalls  int
	createdCalls     int
	reconciledCutoff time.Time
	createdCutoff    time.Time
	limits           []int
	reconciledErr    *svcerrors.ServiceError
	called           chan struct{}
	onReconciled     func(call int)
}

func (f *fakeRetentionService) DeleteReconciledBefore(ctx context.Context, cutoff time.Time, limit int) (int64, *svcerrors.ServiceError) {
	f.mu.Lock()
	f.reconciledCalls++
	call := f.reconciledCalls
	f.reconciledCutoff = cutoff
	f.limits = append(f.limits, limit)
	var n int64
	if call <= len(f.reconciledBatch) {
		n = f.reconciledBatch[call-1]
	}
	cb := f.onReconciled
	f.mu.Unlock()
	if f.called != nil {
		select {
		case f.called <- struct{}{}:
		default:
		}
	}
	if cb != nil {
		cb(call)
	}
	if f.reconciledErr != nil {
		return 0, f.reconciledErr
	}
	return n, nil
}

func (f *fakeRetentionService) DeleteCreatedBefore(ctx context.Context, cutoff time.Time, limit int) (int64, *svcerrors.ServiceError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createdCalls++
	f.createdCutoff = cutoff
	if f.createdCalls <= len(f.createdBatch) {
		return f.createdBatch[f.createdCalls-1], nil
	}
	return 0, nil
}

func (f *fakeRetentionService) calls() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reconciledCalls, f.createdCalls
}

// scriptedLockFactory returns a fixed lock outcome and records unlocks.
type scriptedLockFactory struct {
	acquired bool
	err      error

	mu       sync.Mutex
	lockIDs  []string
	types    []db.LockType
	unlocked []string
}

func (l *scriptedLockFactory) NewAdvisoryLock(ctx context.Context, id string, lockType db.LockType) (string, error) {
	return "", errors.New("not used")
}

func (l *scriptedLockFactory) NewNonBlockingLock(ctx context.Context, id string, lockType db.LockType) (string, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lockIDs = append(l.lockIDs, id)
	l.types = append(l.types, lockType)
	return "owner-" + id, l.acquired, l.err
}

func (l *scriptedLockFactory) Unlock(ctx context.Context, owner string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.unlocked = append(l.unlocked, owner)
}

func retentionConfig() EventRetentionConfig {
	return EventRetentionConfig{
		Interval:     time.Hour,
		Retention:    7 * 24 * time.Hour,
		BatchSize:    5,
		BatchPause:   0,
		InitialDelay: 0,
	}
}

func TestEventRetention_TurnedOffDoesNothing(t *testing.T) {
	RegisterTestingT(t)
	for _, interval := range []time.Duration{0, -time.Minute} {
		svc := &fakeRetentionService{}
		cfg := retentionConfig()
		cfg.Interval = interval
		c := NewEventRetentionControllerForTesting(svc, &scriptedLockFactory{acquired: true}, cfg)

		Expect(c.Enabled()).To(BeFalse())
		c.Start()
		Expect(c.Stop()).To(Succeed()) // must not block
		r, m := svc.calls()
		Expect(r).To(Equal(0))
		Expect(m).To(Equal(0))
	}
}

func TestEventRetention_StopWithoutStart(t *testing.T) {
	RegisterTestingT(t)
	c := NewEventRetentionControllerForTesting(&fakeRetentionService{}, &scriptedLockFactory{acquired: true}, retentionConfig())
	Expect(c.Stop()).To(Succeed())
}

func TestEventRetention_StartRunsAfterInitialDelayAndStops(t *testing.T) {
	RegisterTestingT(t)
	svc := &fakeRetentionService{called: make(chan struct{}, 1)}
	cfg := retentionConfig()
	cfg.InitialDelay = 10 * time.Millisecond
	c := NewEventRetentionControllerForTesting(svc, &scriptedLockFactory{acquired: true}, cfg)

	c.Start()
	c.Start() // second call is a no-op

	select {
	case <-svc.called:
	case <-time.After(5 * time.Second):
		t.Fatal("retention run did not start")
	}

	stopped := make(chan struct{})
	go func() { _ = c.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return")
	}
	r, _ := svc.calls()
	Expect(r).To(Equal(1))
}

func TestEventRetention_StopDuringInitialDelay(t *testing.T) {
	RegisterTestingT(t)
	svc := &fakeRetentionService{}
	cfg := retentionConfig()
	cfg.InitialDelay = time.Hour
	c := NewEventRetentionControllerForTesting(svc, &scriptedLockFactory{acquired: true}, cfg)

	c.Start()
	start := time.Now()
	Expect(c.Stop()).To(Succeed())
	Expect(time.Since(start)).To(BeNumerically("<", time.Second))
	r, _ := svc.calls()
	Expect(r).To(Equal(0))
}

func TestEventRetention_SkipsWhenLockHeld(t *testing.T) {
	RegisterTestingT(t)
	svc := &fakeRetentionService{}
	locks := &scriptedLockFactory{acquired: false}
	cfg := retentionConfig()
	cfg.MaxAge = 30 * 24 * time.Hour
	c := NewEventRetentionControllerForTesting(svc, locks, cfg)

	res := c.RunOnce(context.Background())

	Expect(res.Skipped).To(BeTrue())
	r, m := svc.calls()
	Expect(r).To(Equal(0))
	Expect(m).To(Equal(0))
	Expect(locks.lockIDs).To(Equal([]string{eventRetentionLockID}))
	Expect(locks.types).To(Equal([]db.LockType{db.EventRetention}))
	Expect(locks.unlocked).To(Equal([]string{"owner-" + eventRetentionLockID}))
}

func TestEventRetention_LockErrorSkipsDeletes(t *testing.T) {
	RegisterTestingT(t)
	svc := &fakeRetentionService{}
	c := NewEventRetentionControllerForTesting(svc, &scriptedLockFactory{err: errors.New("db down")}, retentionConfig())

	res := c.RunOnce(context.Background())

	Expect(res.Skipped).To(BeFalse())
	r, _ := svc.calls()
	Expect(r).To(Equal(0))
}

func TestEventRetention_BatchLoopStopsOnShortBatch(t *testing.T) {
	RegisterTestingT(t)
	svc := &fakeRetentionService{reconciledBatch: []int64{5, 5, 2, 5}}
	c := NewEventRetentionControllerForTesting(svc, &scriptedLockFactory{acquired: true}, retentionConfig())

	res := c.RunOnce(context.Background())

	Expect(res.DeletedReconciled).To(Equal(int64(12)))
	r, m := svc.calls()
	Expect(r).To(Equal(3), "the 2-row batch is short, so the 4th batch must not run")
	Expect(m).To(Equal(0), "max age is off")
	Expect(svc.limits).To(Equal([]int{5, 5, 5}))
}

func TestEventRetention_BatchLoopStopsOnEmptyBatch(t *testing.T) {
	RegisterTestingT(t)
	svc := &fakeRetentionService{reconciledBatch: []int64{5, 5}} // third call returns 0
	c := NewEventRetentionControllerForTesting(svc, &scriptedLockFactory{acquired: true}, retentionConfig())

	res := c.RunOnce(context.Background())

	Expect(res.DeletedReconciled).To(Equal(int64(10)))
	r, _ := svc.calls()
	Expect(r).To(Equal(3))
}

func TestEventRetention_BatchLoopStopsOnCancel(t *testing.T) {
	RegisterTestingT(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	full := make([]int64, 100)
	for i := range full {
		full[i] = 5
	}
	svc := &fakeRetentionService{reconciledBatch: full}
	svc.onReconciled = func(call int) {
		if call == 1 {
			cancel()
		}
	}
	cfg := retentionConfig()
	cfg.MaxAge = 30 * 24 * time.Hour
	cfg.BatchPause = time.Hour // cancel must also interrupt the pause
	c := NewEventRetentionControllerForTesting(svc, &scriptedLockFactory{acquired: true}, cfg)

	done := make(chan RetentionResult)
	go func() { done <- c.RunOnce(ctx) }()

	var res RetentionResult
	select {
	case res = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunOnce did not stop after cancel")
	}
	r, m := svc.calls()
	Expect(r).To(Equal(1), "cancel during the pause must end the loop")
	Expect(m).To(Equal(0), "max-age delete must not start after cancel")
	Expect(res.DeletedReconciled).To(Equal(int64(5)))
}

func TestEventRetention_ErrorStopsRun(t *testing.T) {
	RegisterTestingT(t)
	svc := &fakeRetentionService{reconciledErr: svcerrors.GeneralError("boom")}
	cfg := retentionConfig()
	cfg.MaxAge = 30 * 24 * time.Hour
	c := NewEventRetentionControllerForTesting(svc, &scriptedLockFactory{acquired: true}, cfg)

	res := c.RunOnce(context.Background())

	Expect(res.Skipped).To(BeFalse())
	r, m := svc.calls()
	Expect(r).To(Equal(1))
	Expect(m).To(Equal(0))
}

func TestEventRetention_Cutoffs(t *testing.T) {
	RegisterTestingT(t)
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	svc := &fakeRetentionService{createdBatch: []int64{3}}
	cfg := retentionConfig()
	cfg.MaxAge = 30 * 24 * time.Hour
	c := NewEventRetentionControllerForTesting(svc, &scriptedLockFactory{acquired: true}, cfg)
	c.now = func() time.Time { return now }

	res := c.RunOnce(context.Background())

	Expect(svc.reconciledCutoff).To(Equal(now.Add(-7 * 24 * time.Hour)))
	Expect(svc.createdCutoff).To(Equal(now.Add(-30 * 24 * time.Hour)))
	Expect(res.DeletedMaxAge).To(Equal(int64(3)))
}

func TestEventRetention_WithMockDao(t *testing.T) {
	RegisterTestingT(t)
	now := time.Now()
	ago := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	day := 24 * time.Hour

	newEvents := func() *services.EventService {
		dao := mocks.NewEventDao()
		for _, e := range []*api.Event{
			{Meta: api.Meta{ID: "reconciled-old", CreatedAt: *ago(10 * day)}, ReconciledDate: ago(9 * day)},
			{Meta: api.Meta{ID: "reconciled-recent", CreatedAt: *ago(10 * day)}, ReconciledDate: ago(day)},
			{Meta: api.Meta{ID: "unreconciled-old", CreatedAt: *ago(10 * day)}},
			{Meta: api.Meta{ID: "unreconciled-very-old", CreatedAt: *ago(40 * day)}},
		} {
			_, _ = dao.Create(context.Background(), e)
		}
		svc := services.NewEventService(dao)
		return &svc
	}
	remaining := func(svc services.EventService) []string {
		all, _ := svc.All(context.Background())
		ids := []string{}
		for _, e := range all {
			ids = append(ids, e.ID)
		}
		return ids
	}

	cfg := retentionConfig()
	cfg.BatchSize = 1 // forces several batches

	svc := *newEvents()
	NewEventRetentionControllerForTesting(svc, &scriptedLockFactory{acquired: true}, cfg).RunOnce(context.Background())
	Expect(remaining(svc)).To(ConsistOf("reconciled-recent", "unreconciled-old", "unreconciled-very-old"))

	cfg.MaxAge = 30 * day
	svc = *newEvents()
	res := NewEventRetentionControllerForTesting(svc, &scriptedLockFactory{acquired: true}, cfg).RunOnce(context.Background())
	Expect(remaining(svc)).To(ConsistOf("reconciled-recent", "unreconciled-old"))
	Expect(res.DeletedReconciled).To(Equal(int64(1)))
	Expect(res.DeletedMaxAge).To(Equal(int64(1)))
}
