package dao

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
)

type EventDao interface {
	Get(ctx context.Context, id string) (*api.Event, error)
	Create(ctx context.Context, event *api.Event) (*api.Event, error)
	Replace(ctx context.Context, event *api.Event) (*api.Event, error)
	Delete(ctx context.Context, id string) error
	FindByIDs(ctx context.Context, ids []string) (api.EventList, error)
	All(ctx context.Context) (api.EventList, error)

	// Sync-the-world methods for missed event recovery
	FindUnreconciled(ctx context.Context, olderThan time.Duration) (api.EventList, error)
	FindBySourceAndType(ctx context.Context, source string, eventType api.EventType) (api.EventList, error)

	// Retention methods. Each call hard-deletes at most limit rows, oldest first,
	// and returns how many rows it deleted.
	DeleteReconciledBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error)
	DeleteCreatedBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error)
}

var _ EventDao = &sqlEventDao{}

type sqlEventDao struct {
	sessionFactory *db.SessionFactory
}

func NewEventDao(sessionFactory *db.SessionFactory) EventDao {
	return &sqlEventDao{sessionFactory: sessionFactory}
}

func (d *sqlEventDao) Get(ctx context.Context, id string) (*api.Event, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var event api.Event
	if err := g2.Take(&event, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (d *sqlEventDao) Create(ctx context.Context, event *api.Event) (*api.Event, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(event).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}

	notify := fmt.Sprintf("select pg_notify('%s', '%s')", "events", event.ID)

	err := g2.Exec(notify).Error
	if err != nil {
		return nil, err
	}

	return event, nil
}

func (d *sqlEventDao) Replace(ctx context.Context, event *api.Event) (*api.Event, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Save(event).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return event, nil
}

func (d *sqlEventDao) Delete(ctx context.Context, id string) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Unscoped().Omit(clause.Associations).Delete(&api.Event{Meta: api.Meta{ID: id}}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlEventDao) FindByIDs(ctx context.Context, ids []string) (api.EventList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	events := api.EventList{}
	if err := g2.Where("id in (?)", ids).Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (d *sqlEventDao) All(ctx context.Context) (api.EventList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	events := api.EventList{}
	if err := g2.Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (d *sqlEventDao) FindUnreconciled(ctx context.Context, olderThan time.Duration) (api.EventList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	events := api.EventList{}
	cutoff := time.Now().Add(-olderThan)

	// Find events where ReconciledDate is NULL and created_at is older than the cutoff
	if err := g2.Where("reconciled_date IS NULL AND created_at < ?", cutoff).
		Order("created_at ASC").
		Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (d *sqlEventDao) FindBySourceAndType(ctx context.Context, source string, eventType api.EventType) (api.EventList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	events := api.EventList{}

	if err := g2.Where("source = ? AND event_type = ?", source, eventType).
		Order("created_at ASC").
		Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// DeleteReconciledBefore deletes up to limit events whose reconciled_date is
// older than cutoff. Unreconciled events are never matched.
func (d *sqlEventDao) DeleteReconciledBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return d.deleteBatch(ctx, "reconciled_date IS NOT NULL AND reconciled_date < ?", "reconciled_date", cutoff, limit)
}

// DeleteCreatedBefore deletes up to limit events whose created_at is older
// than cutoff, reconciled or not.
func (d *sqlEventDao) DeleteCreatedBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return d.deleteBatch(ctx, "created_at < ?", "created_at", cutoff, limit)
}

// deleteBatch runs one bounded DELETE. The inner SELECT picks the oldest rows
// first and skips rows another session holds a row lock on, so a run never
// waits on an event that is being updated. The where and order arguments are
// fixed strings from this file, never user input.
func (d *sqlEventDao) deleteBatch(ctx context.Context, where, order string, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("event delete batch limit must be positive, got %d", limit)
	}
	g2 := (*d.sessionFactory).New(ctx)
	query := fmt.Sprintf(
		"DELETE FROM events WHERE id IN (SELECT id FROM events WHERE %s ORDER BY %s LIMIT ? FOR UPDATE SKIP LOCKED)",
		where, order)
	result := g2.Exec(query, cutoff, limit)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
