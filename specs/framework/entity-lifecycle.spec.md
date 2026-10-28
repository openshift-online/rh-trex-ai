# Entity Lifecycle Specification

**Date:** 2026-10-19
**Status:** Active
**ID:** FW-002
**Related:** [Plugin Architecture](plugin-architecture.spec.md), [DAO Pattern](../data/dao-pattern.spec.md), [REST Conventions](../api/rest-conventions.spec.md)
**Implements:** `pkg/generator/`, `pkg/cmd/generate.go`, `plugins/*/`

---

## Purpose

Define the lifecycle of an entity from code generation through CRUD operations, ensuring consistent layered architecture across all entities.

## Requirements

### Requirement: Layered Architecture

Every entity SHALL follow the four-layer architecture: Handler → Service → DAO → Model.

#### Scenario: Request flow for entity creation
- GIVEN a POST request to `/api/rh-trex-ai/v1/{kinds}`
- WHEN the request is processed
- THEN the handler SHALL parse the request body and check transport-level rules only (for example, that `id` is not set by the client)
- AND the handler SHALL delegate to the service layer
- AND the service SHALL enforce the entity's validation rules
- AND the service SHALL create an event record alongside the entity
- AND the service SHALL delegate persistence to the DAO
- AND the DAO SHALL use GORM to persist the model to PostgreSQL

### Requirement: Model Structure

Every entity model SHALL embed `api.Meta` and define a `PatchRequest` companion struct.

#### Scenario: Model definition
- GIVEN a new entity "Widget"
- WHEN the model is defined
- THEN the `Widget` struct SHALL embed `api.Meta` (providing `ID`, `CreatedAt`, `UpdatedAt`, `DeletedAt`)
- AND a `WidgetPatchRequest` struct SHALL exist with pointer fields for all mutable properties
- AND all fields SHALL use `json:"snake_case"` tags

### Requirement: GORM BeforeCreate Hook

Every entity model SHALL implement a `BeforeCreate` GORM hook that generates a UUID if the ID is empty.

#### Scenario: Auto-generated ID
- GIVEN a new entity instance with an empty ID
- WHEN `BeforeCreate` is triggered by GORM
- THEN a new UUID SHALL be assigned to the entity's ID field

### Requirement: Service Event Creation

Every service create, update, and delete operation SHALL produce a corresponding event record.

#### Scenario: Create operation event
- GIVEN a successful entity creation
- WHEN the service completes the create operation
- THEN an event with `EventType: api.CreateEventType` SHALL be persisted
- AND the event's `Source` SHALL match the entity's kind name
- AND the event's `SourceID` SHALL match the created entity's ID

### Requirement: Business Logic Lives in the Service

Handlers (REST and gRPC) SHALL only parse requests, convert them to model types, call one service method, and present the result. Every rule about what an entity may contain, how a partial update is applied, and how a list is queried SHALL live in the service, so that REST and gRPC behave the same way.

#### Scenario: Same rules on both transports
- GIVEN a Widget with a required string field `name`
- WHEN a client creates a Widget with an empty `name` over REST
- THEN the service SHALL reject it with `errors.Validation` (HTTP 400)
- AND the same request over gRPC SHALL be rejected with `InvalidArgument`

### Requirement: Entity Validation

The service SHALL validate an entity before every create and every update. A required string field SHALL NOT be empty. No string field SHALL be longer than `services.MaxStringFieldLength` (255) characters.

#### Scenario: Overlong field is rejected on update
- GIVEN an existing Widget
- WHEN a client patches `name` to a 300-character string
- THEN the service SHALL return `errors.Validation` with reason `name exceeds maximum length of 255`
- AND the stored Widget SHALL be unchanged

#### Scenario: Required field cannot be cleared by a patch
- GIVEN an existing Widget with `name: "a"`
- WHEN a client patches `name` to `""`
- THEN the service SHALL return `errors.Validation` with reason `name is required`

### Requirement: Atomic Partial Update

The service SHALL expose `Patch(ctx, id, *WidgetPatchRequest)`. It SHALL read the current entity with a row lock (`SELECT ... FOR UPDATE`, via the DAO's `GetForUpdate`), apply only the fields set in the patch, validate, persist, and create an update event. The read and the write SHALL happen in the same request transaction, so two concurrent patches to the same entity SHALL NOT overwrite each other's changes.

#### Scenario: Concurrent patches to different fields
- GIVEN a Widget with `name: "a"` and `color: "red"`
- WHEN one client patches `name` to `"b"` and another client patches `color` to `"blue"` at the same time
- THEN the stored Widget SHALL have `name: "b"` and `color: "blue"`

### Requirement: Service-Owned Listing

The service SHALL expose `List(ctx, *services.ListArguments)` that returns the page of entities and the paging metadata. Handlers SHALL NOT call `GenericService` directly.

#### Scenario: REST and gRPC list through the service
- GIVEN entities exist
- WHEN `GET /widgets` or `ListWidgets` is called
- THEN the handler SHALL call `WidgetService.List`
- AND the handler SHALL only build the transport-specific list envelope

### Requirement: Event Source Constant

Each entity package SHALL define one `{Kind}EventSource` constant (for example `WidgetEventSource = "Widgets"`). The service SHALL use it when creating events, and the gRPC watch handler SHALL use it to filter events.

#### Scenario: Watch filters by the shared constant
- GIVEN a `WatchWidgets` stream
- WHEN an event with `Source: WidgetEventSource` arrives
- THEN it SHALL be sent to the subscriber
- AND events from other sources SHALL be skipped

### Requirement: Presenter Bi-directional Conversion

Each entity SHALL have a presenter that converts between the internal model and the OpenAPI-generated model in both directions.

#### Scenario: Model to API conversion
- GIVEN an internal `Widget` model instance
- WHEN `PresentWidget(widget)` is called
- THEN the returned `openapi.Widget` SHALL contain all mapped fields
- AND the `ObjectReference` fields (id, kind, href) SHALL be populated

### Requirement: Custom Field Support

The entity generator SHALL support custom fields via the `--fields` flag with types: `string`, `int`, `int64`, `bool`, `float`, `time`.

#### Scenario: Generating entity with custom fields
- GIVEN the command `go run ./cmd/trex generate-entity --kind Rocket --fields "name:string:required,fuel_type:string,max_speed:int"`
- WHEN generation completes
- THEN `name` SHALL be `string` (non-pointer, non-nullable)
- AND `fuel_type` SHALL be `*string` (pointer, nullable)
- AND `max_speed` SHALL be `*int` (pointer, nullable)
- AND the PatchRequest SHALL use pointer types for all fields
- AND the OpenAPI spec SHALL list `name` in the `required` array

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Embed `api.Meta` rather than duplicate fields | DRY principle; consistent ID, timestamps, soft delete |
| PatchRequest uses pointer types | Distinguishes "not provided" (`nil`) from "set to zero value" |
| Event creation in service layer (not DAO) | Service layer owns business logic; DAO is pure persistence |
| Validation and patch merging in the service | One copy of the rules for REST and gRPC; handlers cannot drift apart |
| Row lock (`FOR UPDATE`) for patch reads | The advisory lock is released before the request transaction commits, so it cannot by itself stop a lost update; a row lock is held until commit |
| Fields default to nullable | Safer for schema evolution; required fields are opt-in |
| Snake_case JSON tags | API convention consistency; matches OpenAPI and database columns |
