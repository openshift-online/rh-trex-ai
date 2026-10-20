package integration

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/dao"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/server"
	"github.com/openshift-online/rh-trex-ai/components/api-server/test"
)

// TestRequestTransactionCommit checks that DAO writes made with a request
// context stay invisible to other sessions until db.Resolve commits them.
func TestRequestTransactionCommit(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sessionFactory := h.Env().Database.SessionFactory
	eventDao := dao.NewEventDao(&sessionFactory)

	ctx, err := db.NewContext(context.Background(), sessionFactory)
	Expect(err).NotTo(HaveOccurred())

	first, err := eventDao.Create(ctx, &api.Event{Source: "Dinosaurs", SourceID: "commit-1", EventType: api.CreateEventType})
	Expect(err).NotTo(HaveOccurred())
	second, err := eventDao.Create(ctx, &api.Event{Source: "Dinosaurs", SourceID: "commit-2", EventType: api.CreateEventType})
	Expect(err).NotTo(HaveOccurred())

	// The request sees its own writes.
	_, err = eventDao.Get(ctx, first.ID)
	Expect(err).NotTo(HaveOccurred())

	// Other sessions do not see them before commit.
	_, err = eventDao.Get(context.Background(), first.ID)
	Expect(errors.Is(err, gorm.ErrRecordNotFound)).To(BeTrue(), "write must not be visible before commit, got %v", err)

	db.Resolve(ctx)

	for _, id := range []string{first.ID, second.ID} {
		_, err = eventDao.Get(context.Background(), id)
		Expect(err).NotTo(HaveOccurred())
	}
}

// TestRequestTransactionRollback checks that MarkForRollback undoes every DAO
// write made with the request context, not only the failing one.
func TestRequestTransactionRollback(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sessionFactory := h.Env().Database.SessionFactory
	eventDao := dao.NewEventDao(&sessionFactory)

	ctx, err := db.NewContext(context.Background(), sessionFactory)
	Expect(err).NotTo(HaveOccurred())

	first, err := eventDao.Create(ctx, &api.Event{Source: "Dinosaurs", SourceID: "rollback-1", EventType: api.CreateEventType})
	Expect(err).NotTo(HaveOccurred())
	second, err := eventDao.Create(ctx, &api.Event{Source: "Dinosaurs", SourceID: "rollback-2", EventType: api.CreateEventType})
	Expect(err).NotTo(HaveOccurred())

	db.MarkForRollback(ctx, errors.New("test rollback"))
	db.Resolve(ctx)

	for _, id := range []string{first.ID, second.ID} {
		_, err = eventDao.Get(context.Background(), id)
		Expect(errors.Is(err, gorm.ErrRecordNotFound)).To(BeTrue(), "write must be rolled back, got %v", err)
	}
}

// TestAdvisoryLockInRequestTransaction checks that an advisory lock taken with a
// request context works, and is held until the request transaction ends.
func TestAdvisoryLockInRequestTransaction(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sessionFactory := h.Env().Database.SessionFactory
	locks := db.NewAdvisoryLockFactory(sessionFactory)
	const lockType db.LockType = "transaction-test"

	ctx, err := db.NewContext(context.Background(), sessionFactory)
	Expect(err).NotTo(HaveOccurred())

	owner, err := locks.NewAdvisoryLock(ctx, "lock-1", lockType)
	Expect(err).NotTo(HaveOccurred())
	locks.Unlock(ctx, owner)

	// The lock covers the request's writes until they commit.
	other, acquired, err := locks.NewNonBlockingLock(context.Background(), "lock-1", lockType)
	Expect(err).NotTo(HaveOccurred())
	Expect(acquired).To(BeFalse(), "lock must be held until the request transaction ends")
	locks.Unlock(context.Background(), other)

	db.Resolve(ctx)

	other, acquired, err = locks.NewNonBlockingLock(context.Background(), "lock-1", lockType)
	Expect(err).NotTo(HaveOccurred())
	Expect(acquired).To(BeTrue(), "lock must be released when the request transaction ends")
	locks.Unlock(context.Background(), other)
}

// expectEvent checks, from outside any request, whether the event was persisted.
func expectEvent(eventDao dao.EventDao, id string, persisted bool) {
	_, err := eventDao.Get(context.Background(), id)
	if persisted {
		Expect(err).NotTo(HaveOccurred(), "write must be committed")
		return
	}
	Expect(errors.Is(err, gorm.ErrRecordNotFound)).To(BeTrue(), "write must be rolled back, got %v", err)
}

// TestTransactionMiddlewareResolvesByStatus checks that the HTTP middleware
// commits on success and rolls back when the handler responds with an error
// status, without anyone calling MarkForRollback.
func TestTransactionMiddlewareResolvesByStatus(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sessionFactory := h.Env().Database.SessionFactory
	eventDao := dao.NewEventDao(&sessionFactory)

	cases := []struct {
		name      string
		status    int // 0 means write a body without calling WriteHeader
		persisted bool
	}{
		{"implicit 200", 0, true},
		{"201 created", http.StatusCreated, true},
		{"400 bad request", http.StatusBadRequest, false},
		{"422 unprocessable", http.StatusUnprocessableEntity, false},
		{"500 internal error", http.StatusInternalServerError, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var id string
			handler := db.TransactionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ev, err := eventDao.Create(r.Context(), &api.Event{Source: "Dinosaurs", SourceID: "http-" + tc.name, EventType: api.CreateEventType})
				Expect(err).NotTo(HaveOccurred())
				id = ev.ID
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				_, _ = w.Write([]byte("{}"))
			}), sessionFactory)

			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
			expectEvent(eventDao, id, tc.persisted)
		})
	}
}

// TestTransactionMiddlewarePanicRollsBack checks that a panic in the handler
// rolls back the request's writes and still reaches outer recovery code.
func TestTransactionMiddlewarePanicRollsBack(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sessionFactory := h.Env().Database.SessionFactory
	eventDao := dao.NewEventDao(&sessionFactory)

	var id string
	handler := db.TransactionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ev, err := eventDao.Create(r.Context(), &api.Event{Source: "Dinosaurs", SourceID: "http-panic", EventType: api.CreateEventType})
		Expect(err).NotTo(HaveOccurred())
		id = ev.ID
		panic("boom")
	}), sessionFactory)

	Expect(func() {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
	}).To(PanicWith("boom"))
	expectEvent(eventDao, id, false)
}

// TestTransactionUnaryInterceptorResolvesByError checks that the gRPC
// interceptor commits on success, rolls back when the handler returns an
// error, and rolls back (then re-panics) when the handler panics.
func TestTransactionUnaryInterceptorResolvesByError(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sessionFactory := h.Env().Database.SessionFactory
	eventDao := dao.NewEventDao(&sessionFactory)
	interceptor := server.TransactionUnaryInterceptor(sessionFactory)
	info := &grpc.UnaryServerInfo{FullMethod: "/test/Method"}

	// run writes the event ID to *id before the outcome, so it is set even on panic.
	run := func(id *string, sourceID string, outcome func() error) error {
		_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, _ interface{}) (interface{}, error) {
			ev, err := eventDao.Create(ctx, &api.Event{Source: "Dinosaurs", SourceID: sourceID, EventType: api.CreateEventType})
			Expect(err).NotTo(HaveOccurred())
			*id = ev.ID
			return nil, outcome()
		})
		return err
	}

	var id string
	err := run(&id, "grpc-ok", func() error { return nil })
	Expect(err).NotTo(HaveOccurred())
	expectEvent(eventDao, id, true)

	err = run(&id, "grpc-error", func() error { return errors.New("handler failed") })
	Expect(err).To(HaveOccurred())
	expectEvent(eventDao, id, false)

	Expect(func() {
		_ = run(&id, "grpc-panic", func() error { panic("boom") })
	}).To(PanicWith("boom"))
	expectEvent(eventDao, id, false)
}
