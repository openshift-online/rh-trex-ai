package integration

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	"gorm.io/gorm"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/dao"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
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
