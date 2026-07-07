# Entity Lifecycle Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** FW-002
**Related:** [Plugin Architecture](plugin-architecture.spec.md), [DAO Pattern](../data/dao-pattern.spec.md), [REST Conventions](../api/rest-conventions.spec.md)
**Implements:** `scripts/generator.go`, `plugins/*/`

---

## Purpose

Define the lifecycle of an entity from code generation through CRUD operations, ensuring consistent layered architecture across all entities.

## Requirements

### Requirement: Layered Architecture

Every entity SHALL follow the four-layer architecture: Handler → Service → DAO → Model.

#### Scenario: Request flow for entity creation
- GIVEN a POST request to `/api/rh-trex/v1/{kinds}`
- WHEN the request is processed
- THEN the handler SHALL validate and parse the request body
- AND the handler SHALL delegate to the service layer
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
- GIVEN the command `go run ./scripts/generator.go --kind Rocket --fields "name:string:required,fuel_type:string,max_speed:int"`
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
| Fields default to nullable | Safer for schema evolution; required fields are opt-in |
| Snake_case JSON tags | API convention consistency; matches OpenAPI and database columns |
