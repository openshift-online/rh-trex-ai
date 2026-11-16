package controllers

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/dao/mocks"
	dbmocks "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db/mocks"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/services"
)

func newExampleControllerConfig(ctrl *exampleController) *ControllerConfig {
	return &ControllerConfig{
		Source: "my-event-source",
		Handlers: map[api.EventType][]ControllerHandlerFunc{
			api.CreateEventType: {ctrl.OnAdd},
			api.UpdateEventType: {ctrl.OnUpdate},
			api.DeleteEventType: {ctrl.OnDelete},
		},
	}
}

type exampleController struct {
	addCounter    int
	updateCounter int
	deleteCounter int
}

func (d *exampleController) OnAdd(ctx context.Context, id string) error {
	d.addCounter++
	return nil
}

func (d *exampleController) OnUpdate(ctx context.Context, id string) error {
	d.updateCounter++
	return nil
}

func (d *exampleController) OnDelete(ctx context.Context, id string) error {
	d.deleteCounter++
	return nil
}

func TestControllerFramework(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	eventsDao := mocks.NewEventDao()
	events := services.NewEventService(eventsDao)
	mgr := NewKindControllerManager(dbmocks.NewMockAdvisoryLockFactory(), events)

	ctrl := &exampleController{}
	config := newExampleControllerConfig(ctrl)
	mgr.Add(config)

	_, _ = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "1"},
		Source:    config.Source,
		SourceID:  "any id",
		EventType: api.CreateEventType,
	})

	_, _ = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "2"},
		Source:    config.Source,
		SourceID:  "any id",
		EventType: api.UpdateEventType,
	})

	_, _ = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "3"},
		Source:    config.Source,
		SourceID:  "any id",
		EventType: api.DeleteEventType,
	})

	mgr.Handle("1")
	mgr.Handle("2")
	mgr.Handle("3")

	Expect(ctrl.addCounter).To(Equal(1))
	Expect(ctrl.updateCounter).To(Equal(1))
	Expect(ctrl.deleteCounter).To(Equal(1))

	for _, id := range []string{"1", "2", "3"} {
		eve, _ := eventsDao.Get(ctx, id)
		Expect(eve.ReconciledDate).ToNot(BeNil(), "event %s reconcile date should be set", id)
	}
}

func newTestManager() (*KindControllerManager, services.EventService, context.Context) {
	eventsDao := mocks.NewEventDao()
	events := services.NewEventService(eventsDao)
	mgr := NewKindControllerManager(dbmocks.NewMockAdvisoryLockFactory(), events)
	return mgr, events, context.Background()
}

func TestControllerFramework_NoControllerForSource_MarksReconciled(t *testing.T) {
	RegisterTestingT(t)
	mgr, events, ctx := newTestManager()
	mgr.Add(newExampleControllerConfig(&exampleController{}))

	_, _ = events.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "no-source"},
		Source:    "kind-without-controller",
		SourceID:  "any id",
		EventType: api.CreateEventType,
	})

	mgr.Handle("no-source")

	eve, err := events.Get(ctx, "no-source")
	Expect(err).To(BeNil())
	Expect(eve.ReconciledDate).ToNot(BeNil(), "an event with no controller for its source should be reconciled")
}

func TestControllerFramework_NoHandlerForEventType_MarksReconciled(t *testing.T) {
	RegisterTestingT(t)
	mgr, events, ctx := newTestManager()
	ctrl := &exampleController{}
	mgr.Add(&ControllerConfig{
		Source: "my-event-source",
		Handlers: map[api.EventType][]ControllerHandlerFunc{
			api.CreateEventType: {ctrl.OnAdd},
		},
	})

	_, _ = events.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "no-handler"},
		Source:    "my-event-source",
		SourceID:  "any id",
		EventType: api.DeleteEventType,
	})

	mgr.Handle("no-handler")

	Expect(ctrl.addCounter).To(Equal(0))
	eve, err := events.Get(ctx, "no-handler")
	Expect(err).To(BeNil())
	Expect(eve.ReconciledDate).ToNot(BeNil(), "an event with no handler for its type should be reconciled")
}

func TestControllerFramework_HandlerError_LeavesUnreconciled(t *testing.T) {
	RegisterTestingT(t)
	mgr, events, ctx := newTestManager()
	calls := 0
	mgr.Add(&ControllerConfig{
		Source: "my-event-source",
		Handlers: map[api.EventType][]ControllerHandlerFunc{
			api.CreateEventType: {func(ctx context.Context, id string) error {
				calls++
				return fmt.Errorf("handler failed")
			}},
		},
	})

	_, _ = events.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "fails"},
		Source:    "my-event-source",
		SourceID:  "any id",
		EventType: api.CreateEventType,
	})

	mgr.Handle("fails")

	Expect(calls).To(Equal(1))
	eve, err := events.Get(ctx, "fails")
	Expect(err).To(BeNil())
	Expect(eve.ReconciledDate).To(BeNil(), "a failed handler must leave the event for SyncController to retry")
}
