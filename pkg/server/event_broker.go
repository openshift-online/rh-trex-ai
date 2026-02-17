package server

import (
	"context"
	"sync"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/segmentio/ksuid"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
)

var (
	brokerSubscribersActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "grpc_stream_subscribers_active",
			Help: "Current number of active watch stream subscribers",
		},
	)
	brokerEventsSent = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "grpc_stream_events_sent_total",
			Help: "Total events successfully sent to stream subscribers",
		},
	)
	brokerEventsDropped = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "grpc_stream_events_dropped_total",
			Help: "Total events dropped due to slow stream subscribers",
		},
	)
)

func init() {
	prometheus.MustRegister(brokerSubscribersActive)
	prometheus.MustRegister(brokerEventsSent)
	prometheus.MustRegister(brokerEventsDropped)
}

type BrokerEvent struct {
	EventID    string
	Source     string
	SourceID   string
	EventType  api.EventType
}

type Subscription struct {
	ID     string
	Events <-chan *BrokerEvent
}

type EventBroker struct {
	mu          sync.RWMutex
	subscribers map[string]chan *BrokerEvent
	bufferSize  int
	events      services.EventService
	closed      bool
}

func NewEventBroker(bufferSize int, events services.EventService) *EventBroker {
	return &EventBroker{
		subscribers: make(map[string]chan *BrokerEvent),
		bufferSize:  bufferSize,
		events:      events,
	}
}

func (b *EventBroker) Subscribe(ctx context.Context) *Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := ksuid.New().String()
	ch := make(chan *BrokerEvent, b.bufferSize)
	b.subscribers[id] = ch
	brokerSubscribersActive.Inc()

	go func() {
		<-ctx.Done()
		b.Unsubscribe(id)
	}()

	return &Subscription{
		ID:     id,
		Events: ch,
	}
}

func (b *EventBroker) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, ok := b.subscribers[id]; ok {
		delete(b.subscribers, id)
		close(ch)
		brokerSubscribersActive.Dec()
	}
}

func (b *EventBroker) Publish(eventID string) {
	ctx := context.Background()
	event, svcErr := b.events.Get(ctx, eventID)
	if svcErr != nil {
		glog.Warningf("EventBroker: failed to load event %s: %v", eventID, svcErr)
		return
	}

	brokerEvent := &BrokerEvent{
		EventID:   event.ID,
		Source:    event.Source,
		SourceID:  event.SourceID,
		EventType: event.EventType,
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for subID, ch := range b.subscribers {
		select {
		case ch <- brokerEvent:
			brokerEventsSent.Inc()
		default:
			brokerEventsDropped.Inc()
			glog.V(5).Infof("EventBroker: dropped event %s for slow subscriber %s", eventID, subID)
		}
	}
}

func (b *EventBroker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	for id, ch := range b.subscribers {
		delete(b.subscribers, id)
		close(ch)
	}
	brokerSubscribersActive.Set(0)
}
