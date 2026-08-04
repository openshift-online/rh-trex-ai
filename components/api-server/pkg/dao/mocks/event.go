package mocks

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/dao"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/errors"
)

var _ dao.EventDao = &eventDaoMock{}

type eventDaoMock struct {
	events api.EventList
}

func NewEventDao() *eventDaoMock {
	return &eventDaoMock{}
}

func (d *eventDaoMock) Get(ctx context.Context, id string) (*api.Event, error) {
	for _, dino := range d.events {
		if dino.ID == id {
			return dino, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *eventDaoMock) Create(ctx context.Context, event *api.Event) (*api.Event, error) {
	d.events = append(d.events, event)
	return event, nil
}

func (d *eventDaoMock) Replace(ctx context.Context, event *api.Event) (*api.Event, error) {
	return nil, errors.NotImplemented("Event").AsError()
}

func (d *eventDaoMock) Delete(ctx context.Context, id string) error {
	newEvents := api.EventList{}
	for _, e := range d.events {
		if e.ID == id {
			// deleting this one
			// do not include in the new list
		} else {
			newEvents = append(newEvents, e)
		}
	}
	d.events = newEvents
	return nil
}

func (d *eventDaoMock) FindByIDs(ctx context.Context, ids []string) (api.EventList, error) {
	return nil, errors.NotImplemented("Event").AsError()
}

func (d *eventDaoMock) All(ctx context.Context) (api.EventList, error) {
	return d.events, nil
}

func (d *eventDaoMock) FindUnreconciled(ctx context.Context, olderThan time.Duration) (api.EventList, error) {
	cutoff := time.Now().Add(-olderThan)
	result := api.EventList{}

	for _, event := range d.events {
		if event.ReconciledDate == nil && event.CreatedAt.Before(cutoff) {
			result = append(result, event)
		}
	}
	return result, nil
}

func (d *eventDaoMock) FindBySourceAndType(ctx context.Context, source string, eventType api.EventType) (api.EventList, error) {
	result := api.EventList{}

	for _, event := range d.events {
		if event.Source == source && event.EventType == eventType {
			result = append(result, event)
		}
	}
	return result, nil
}
