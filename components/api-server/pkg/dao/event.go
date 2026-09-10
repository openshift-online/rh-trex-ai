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
