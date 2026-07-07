# Event-Driven Controllers Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** FW-003
**Related:** [Plugin Architecture](plugin-architecture.spec.md), [gRPC Conventions](../api/grpc-conventions.spec.md)
**Implements:** `pkg/controllers/framework.go`, `pkg/server/controllers.go`, `pkg/server/event_broker.go`, `pkg/db/session_factory.go`

---

## Purpose

Define the event-driven controller framework that uses PostgreSQL LISTEN/NOTIFY and advisory locks to process entity lifecycle events with guaranteed at-least-once delivery and concurrent worker safety.

## Requirements

### Requirement: PostgreSQL LISTEN/NOTIFY Integration

The controller server SHALL use PostgreSQL's `pg_notify` channel to receive real-time event notifications.

#### Scenario: Event notification flow
- GIVEN a running `ControllersServer` with a listener on the "events" channel
- WHEN a new event row is inserted and `pg_notify('events', event_id)` fires
- THEN the `KindControllerManager.Handle(id)` SHALL be invoked with the event ID
- AND the `EventBroker.Publish(id)` SHALL be invoked for gRPC stream fan-out

### Requirement: Advisory Lock Concurrency

The controller framework SHALL use PostgreSQL advisory locks for fail-fast, non-blocking event processing.

#### Scenario: Concurrent event processing
- GIVEN two controller workers receive the same event ID
- WHEN both attempt to acquire an advisory lock for that event ID
- THEN exactly one worker SHALL acquire the lock and process the event
- AND the other worker SHALL skip processing without waiting (non-blocking)
- AND the lock SHALL be released after processing completes (via defer)

### Requirement: Event Dispatch by Source and Type

The `KindControllerManager` SHALL dispatch events to registered handlers based on `event.Source` and `event.EventType`.

#### Scenario: Dispatching a create event
- GIVEN a controller registered for source "Dinosaurs" with handlers for `CreateEventType`
- WHEN an event with `Source: "Dinosaurs"` and `EventType: CreateEventType` is handled
- THEN all registered `OnUpsert` handler functions SHALL be invoked in order
- AND if all handlers succeed, `event.ReconciledDate` SHALL be set to the current time

#### Scenario: Unknown source
- GIVEN no controllers registered for source "UnknownKind"
- WHEN an event with `Source: "UnknownKind"` arrives
- THEN the event SHALL be logged and skipped without error

### Requirement: Idempotent Event Handlers

All event handler functions (OnUpsert, OnDelete) SHALL be idempotent — safe to invoke multiple times for the same event.

#### Scenario: Duplicate event processing
- GIVEN an `OnUpsert` handler that was already executed for entity ID "abc-123"
- WHEN the same event is replayed (e.g., by the sync controller)
- THEN the handler SHALL produce the same result without side effects

### Requirement: Sync Controller for Missed Events

A `SyncController` SHALL periodically scan for unprocessed events to recover from missed NOTIFY messages.

#### Scenario: Missed event recovery
- GIVEN an event was inserted but the NOTIFY was lost (e.g., during a connection reset)
- WHEN the `SyncController` runs its periodic scan (default: every 5 minutes)
- THEN unprocessed events up to `MaxAge` (default: 1 hour) SHALL be re-dispatched
- AND at most `MaxEventsPerSync` (default: 1000) events SHALL be processed per cycle

### Requirement: Event Broker Fan-Out

The `EventBroker` SHALL implement pub/sub fan-out from PostgreSQL events to gRPC stream subscribers.

#### Scenario: Multiple gRPC stream subscribers
- GIVEN three active gRPC Watch stream subscribers
- WHEN a new event is published to the broker
- THEN all three subscribers SHALL receive a `BrokerEvent` on their channels

#### Scenario: Slow subscriber handling
- GIVEN a subscriber whose channel buffer (default: 256) is full
- WHEN a new event is published
- THEN the event SHALL be dropped for that subscriber (non-blocking send)
- AND the `grpc_stream_events_dropped_total` metric SHALL be incremented

#### Scenario: Context-based auto-unsubscription
- GIVEN a subscriber whose gRPC stream context is cancelled
- WHEN the context's `Done()` channel fires
- THEN the subscriber SHALL be automatically unsubscribed
- AND the subscriber's channel SHALL be closed

### Requirement: Event Broker Metrics

The `EventBroker` SHALL expose Prometheus metrics for observability.

#### Scenario: Metrics registration
- GIVEN the event broker is initialized
- THEN the following Prometheus metrics SHALL be registered:
  - `grpc_stream_subscribers_active` (gauge)
  - `grpc_stream_events_sent_total` (counter)
  - `grpc_stream_events_dropped_total` (counter)

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| PostgreSQL LISTEN/NOTIFY over message queues | Zero operational overhead; leverages existing database infrastructure |
| Non-blocking advisory locks | Fail-fast prevents worker pile-up on contested events |
| Sync controller for missed events | NOTIFY is fire-and-forget; periodic scan ensures at-least-once delivery |
| Buffered channels for broker subscribers | Prevents blocking the publish path; drops events for slow consumers |
| Event reconciliation date marking | Prevents re-processing of successfully handled events |
| Kubernetes-style controller pattern | Familiar to Go developers; proven at scale in upstream projects |
