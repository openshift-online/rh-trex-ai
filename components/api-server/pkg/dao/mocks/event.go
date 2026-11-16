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
	for _, e := range d.events {
		if e.ID == id {
			return copyEvent(e), nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// The mock stores and returns copies, like a real database. A change the caller
// makes to an event is visible only after it is saved with Replace.
func (d *eventDaoMock) Create(ctx context.Context, event *api.Event) (*api.Event, error) {
	d.events = append(d.events, copyEvent(event))
	return event, nil
}

func (d *eventDaoMock) Replace(ctx context.Context, event *api.Event) (*api.Event, error) {
	for i, e := range d.events {
		if e.ID == event.ID {
			d.events[i] = copyEvent(event)
			return event, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func copyEvent(e *api.Event) *api.Event {
	c := *e
	if e.ReconciledDate != nil {
		t := *e.ReconciledDate
		c.ReconciledDate = &t
	}
	return &c
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

func (d *eventDaoMock) DeleteReconciledBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return d.deleteMatching(limit, func(e *api.Event) bool {
		return e.ReconciledDate != nil && e.ReconciledDate.Before(cutoff)
	}), nil
}

func (d *eventDaoMock) DeleteCreatedBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return d.deleteMatching(limit, func(e *api.Event) bool {
		return e.CreatedAt.Before(cutoff)
	}), nil
}

// deleteMatching removes up to limit events that match, in insertion order.
func (d *eventDaoMock) deleteMatching(limit int, match func(*api.Event) bool) int64 {
	kept := api.EventList{}
	var deleted int64
	for _, e := range d.events {
		if deleted < int64(limit) && match(e) {
			deleted++
			continue
		}
		kept = append(kept, e)
	}
	d.events = kept
	return deleted
}
