# Event-Driven Controllers Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** FW-003
**Related:** [Plugin Architecture](plugin-architecture.spec.md), [gRPC Conventions](../api/grpc-conventions.spec.md)
**Implements:** `pkg/controllers/framework.go`, `pkg/controllers/event_retention_controller.go`, `pkg/server/controllers.go`, `pkg/server/event_broker.go`, `pkg/db/session_factory.go`

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
- AND `event.ReconciledDate` SHALL be set to the current time, because there is nothing to retry

#### Scenario: No handler for the event type
- GIVEN a controller registered for source "Dinosaurs" with handlers only for `CreateEventType`
- WHEN an event with `Source: "Dinosaurs"` and `EventType: DeleteEventType` arrives
- THEN no handler function SHALL be invoked
- AND `event.ReconciledDate` SHALL be set to the current time, because there is nothing to retry

#### Scenario: Handler failure
- GIVEN a controller registered for source "Dinosaurs" whose `CreateEventType` handler returns an error
- WHEN an event with `Source: "Dinosaurs"` and `EventType: CreateEventType` is handled
- THEN `event.ReconciledDate` SHALL remain unset
- AND the event SHALL be retried by the `SyncController`

#### Scenario: Event cannot be loaded
- GIVEN an event ID that cannot be loaded from the events table
- WHEN `Handle(id)` is invoked
- THEN no handler SHALL be invoked and no event SHALL be marked reconciled

#### Scenario: Failure to save the reconciled date
- GIVEN an event that was handled, or had nothing to run
- WHEN saving `event.ReconciledDate` fails
- THEN the error SHALL be logged
- AND the event SHALL remain unreconciled and be retried by the `SyncController`

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

#### Scenario: Events with no handler stop being re-dispatched
- GIVEN an unreconciled event older than `MaxAge` whose source or event type has no registered handler
- WHEN the `SyncController` completes one cycle that includes it
- THEN the event SHALL be marked reconciled
- AND later cycles SHALL NOT return it as unprocessed

### Requirement: Event Retention

An `EventRetentionController` SHALL periodically delete old rows from the `events` table, so the table does not grow without bound.

#### Scenario: Reconciled events past the retention period
- GIVEN the retention period is `--event-retention-period` (default: 168h)
- WHEN a retention run starts
- THEN every event with `reconciled_date` older than now minus the retention period SHALL be deleted
- AND events with a more recent `reconciled_date` SHALL be kept
- AND unreconciled events SHALL be kept, so the `SyncController` can still retry them

#### Scenario: Hard cap on event age
- GIVEN `--event-retention-max-age` is greater than zero
- WHEN a retention run starts
- THEN every event with `created_at` older than now minus the max age SHALL be deleted, reconciled or not
- AND when `--event-retention-max-age` is zero or less, this cap SHALL NOT apply (default: off)
- AND a non-zero max age MUST NOT be shorter than the retention period

#### Scenario: Batched deletes
- GIVEN more matching rows than `--event-retention-batch-size` (default: 5000)
- WHEN a retention run deletes them
- THEN it SHALL delete at most one batch per statement, oldest first
- AND it SHALL pause `--event-retention-batch-pause` (default: 100ms) between batches
- AND it SHALL stop when a batch deletes fewer rows than the batch size, or when the server shuts down

#### Scenario: One replica at a time
- GIVEN several API server replicas share one database
- WHEN their retention runs overlap
- THEN each run SHALL first try a non-blocking advisory lock of type `event_retention`
- AND a run that does not get the lock SHALL be skipped and counted in `event_retention_runs_skipped_total`
- AND this lock type MUST differ from the `events` lock type used per event ID

#### Scenario: Scheduling and off switch
- GIVEN `--event-retention-interval` (default: 1h) is greater than zero
- WHEN the controllers server starts
- THEN the first run SHALL start after `--event-retention-initial-delay` (default: 5m), and then once per interval
- AND when the interval is zero or less, the controller SHALL NOT run
- AND `Stop` SHALL cancel a run in progress and wait for it to return

#### Scenario: Retention observability
- GIVEN the retention controller is created
- THEN it SHALL register these Prometheus metrics:
  - `event_retention_deleted_total` (counter, label `reason`: `reconciled` | `max_age`)
  - `event_retention_run_duration_seconds` (histogram)
  - `event_retention_runs_total` (counter)
  - `event_retention_runs_skipped_total` (counter)
  - `event_retention_errors_total` (counter)
- AND each run SHALL log one `slog` line with the deleted counts and the duration
- AND errors SHALL be logged and counted, and SHALL NOT stop the process

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
| Event reconciliation date marking | Prevents re-processing of successfully handled events. An event with no registered controller or handler counts as handled: no retry can change the outcome, and leaving it unreconciled makes every sync cycle re-handle it |
| Retention keeps unreconciled events by default | The sync controller still needs them; the max-age cap is opt-in |
| Batched, oldest-first deletes with a pause | Short statements keep row locks and WAL bursts small on a busy table |
| Kubernetes-style controller pattern | Familiar to Go developers; proven at scale in upstream projects |
