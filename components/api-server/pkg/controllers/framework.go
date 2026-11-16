package controllers

import (
	"context"
	"log/slog"
	"time"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/services"
)

/*
This controller pattern mimics upstream Kube-style controllers with Add/Update/Delete events with periodic
sync-the-world for any messages missed.

The implementation is specific to the Event table in this service and leverages features of PostresDB:

	1. pg_notify(channel, msg) is used for real time notification to listeners
	2. advisory locks are used for concurrency when doing background work

DAOs decorated similarly to the DinosaurDAO will persist Events to the database and listeners are notified of the changed.
A worker attemping to process the Event will first obtain a fail-fast adivosry lock. Of many competing workers, only
one would first successfully obtain the lock. All other workers will *not* wait to obtain the lock.

Any successful processing of an Event will remove it from the Events table permanently.

A periodic process reads from the Events table and calls pg_notify, ensuring any failed Events are re-processed. Competing
consumers for the lock will fail fast on redundant messages.

*/

type contextKey string

const contextKeyEvent contextKey = "event"

type ControllerHandlerFunc func(ctx context.Context, id string) error

type ControllerConfig struct {
	Source   string
	Handlers map[api.EventType][]ControllerHandlerFunc
}

type KindControllerManager struct {
	controllers map[string]map[api.EventType][]ControllerHandlerFunc
	lockFactory db.LockFactory
	events      services.EventService
}

func NewKindControllerManager(lockFactory db.LockFactory, events services.EventService) *KindControllerManager {
	return &KindControllerManager{
		controllers: map[string]map[api.EventType][]ControllerHandlerFunc{},
		lockFactory: lockFactory,
		events:      events,
	}
}

func (km *KindControllerManager) Add(config *ControllerConfig) {
	for ev, fn := range config.Handlers {
		km.add(config.Source, ev, fn)
	}
}

func (km *KindControllerManager) add(source string, ev api.EventType, fns []ControllerHandlerFunc) {
	if _, exists := km.controllers[source]; !exists {
		km.controllers[source] = map[api.EventType][]ControllerHandlerFunc{}
	}

	if _, exists := km.controllers[source][ev]; !exists {
		km.controllers[source][ev] = []ControllerHandlerFunc{}
	}

	km.controllers[source][ev] = append(km.controllers[source][ev], fns...)
}

func (km *KindControllerManager) Handle(id string) {

	ctx := context.Background()

	// lock the Event with a fail-fast advisory lock context.
	// this allows concurrent processing of many events by one or many controller managers.
	// allow the lock to be released by the handler goroutine and allow this function to continue.
	// subsequent events will be locked by their own distinct IDs.
	lockOwnerID, acquired, err := km.lockFactory.NewNonBlockingLock(ctx, id, db.Events)
	defer km.lockFactory.Unlock(ctx, lockOwnerID)
	if err != nil {
		slog.ErrorContext(ctx, "Error obtaining the event lock", "event_id", id, "error", err)
		return
	}
	if !acquired {
		slog.InfoContext(ctx, "Event is processed by another worker, continue to process the next", "event_id", id)
		return
	}
	threadContext := context.WithValue(ctx, contextKeyEvent, id)

	km.handle(threadContext, id)
}

func (km *KindControllerManager) handle(ctx context.Context, id string) {

	event, err := km.events.Get(ctx, id)

	if err != nil {
		slog.ErrorContext(ctx, "Failed to get event", "event_id", id, "error", err)
		return
	}

	source, found := km.controllers[event.Source]
	if !found {
		// Nothing is registered to react to this source, so there is nothing to retry.
		// Mark the event handled; otherwise SyncController re-handles it forever.
		slog.DebugContext(ctx, "No controllers found", "source", event.Source, "event_id", id)
		km.markReconciled(ctx, event)
		return
	}

	handlerFns, found := source[event.EventType]
	if !found {
		// The source has a controller, but not for this event type. Same as above.
		slog.DebugContext(ctx, "No handler functions found", "source", event.Source, "event_type", event.EventType, "event_id", id)
		km.markReconciled(ctx, event)
		return
	}

	for _, fn := range handlerFns {
		err := fn(ctx, event.SourceID)
		if err != nil {
			slog.ErrorContext(ctx, "Error handling event", "source", event.Source, "event_type", event.EventType, "event_id", id, "error", err)
			return
		}
	}

	// all handlers successfully executed
	km.markReconciled(ctx, event)
}

// markReconciled records that the event needs no more work, so SyncController
// stops picking it up. A failed save is logged; the event is then retried on the
// next sync cycle.
func (km *KindControllerManager) markReconciled(ctx context.Context, event *api.Event) {
	now := time.Now()
	event.ReconciledDate = &now
	if _, err := km.events.Replace(ctx, event); err != nil {
		slog.ErrorContext(ctx, "Failed to mark event as reconciled", "event_id", event.ID, "error", err)
	}
}
