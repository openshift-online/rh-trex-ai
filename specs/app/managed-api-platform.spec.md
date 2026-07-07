# Managed API Platform Specification

**Date:** 2026-07-07
**Status:** Draft
**ID:** APP-001
**Related:** [Entity Generator](../codegen/entity-generator.spec.md), [CLI Generator](../codegen/cli-generator.spec.md), [SDK Generator](../codegen/sdk-generator.spec.md), [Console Plugin Generator](../codegen/console-plugin-generator.spec.md), [Entity Lifecycle](../framework/entity-lifecycle.spec.md), [Event-Driven Controllers](../framework/event-driven-controllers.spec.md), [Plugin Architecture](../framework/plugin-architecture.spec.md)
**Implements:** `plugins/projects/`, `plugins/entity_definitions/`, `plugins/field_definitions/`, `plugins/relationships/`, `plugins/builds/`, `plugins/deployments/`

---

## Purpose

Transform TRex from a demonstrative "dinosaurs" API into a managed API platform that orchestrates the full TRex generation pipeline. Users define their entity-relationship diagrams (ERD) through the API, and the platform generates complete API services (REST, gRPC, CLI, SDK, Console Plugin) from those definitions. This dogfoods every TRex generator (CG-001 through CG-004) and stress-tests the entity system with a non-trivial 6-entity relational data model featuring parent-child hierarchies, state machines, and cross-entity referential integrity.

## Data Model

### Entity-Relationship Diagram

```
Project (1) ──────< (N) EntityDefinition
    │                      │
    │                      ├──< (N) FieldDefinition
    │                      │
    │                      └──< (N) Relationship (source + target)
    │
    ├──< (N) Build
    │         │
    │         └──< (N) Deployment
```

### Entity Summary

| Entity | Plugin Directory | Parent | State Machine |
|--------|-----------------|--------|---------------|
| Project | `plugins/projects/` | — | draft → active → archived |
| EntityDefinition | `plugins/entity_definitions/` | Project | — |
| FieldDefinition | `plugins/field_definitions/` | EntityDefinition | — |
| Relationship | `plugins/relationships/` | Project (+ source/target EntityDefinition) | — |
| Build | `plugins/builds/` | Project | pending → building → succeeded / failed |
| Deployment | `plugins/deployments/` | Build + Project | provisioning → running → stopped / failed |

## Requirements

### Requirement: Project Lifecycle

A Project SHALL represent a managed API service with a name, description, repository URL, and lifecycle status.

#### Scenario: Create a new project
- GIVEN a POST request to `/api/rh-trex-ai/v1/projects`
- WHEN the request body contains `{"name": "pet-store", "description": "Pet management API"}`
- THEN a Project SHALL be created with `status: "draft"`
- AND an event with `EventType: CreateEventType` and `Source: "Projects"` SHALL be emitted

#### Scenario: Project status transitions
- GIVEN a Project in status "draft"
- WHEN the Project is patched with `{"status": "active"}`
- THEN the status SHALL change to "active"
- AND a Project in status "active" MAY transition to "archived"
- AND a Project in status "archived" SHALL NOT transition to any other status

#### Scenario: Project deletion cascades
- GIVEN a Project with associated EntityDefinitions, FieldDefinitions, Relationships, Builds, and Deployments
- WHEN the Project is deleted
- THEN all child EntityDefinitions SHALL be soft-deleted
- AND all child FieldDefinitions SHALL be soft-deleted (via EntityDefinition cascade)
- AND all child Relationships SHALL be soft-deleted
- AND all child Builds SHALL be soft-deleted
- AND all child Deployments SHALL be soft-deleted (via Build cascade)

**Fields:**

| Field | Go Type | JSON | Required | Description |
|-------|---------|------|----------|-------------|
| name | `string` | `name` | yes | Unique project identifier |
| description | `*string` | `description` | no | Human-readable description |
| repository_url | `*string` | `repository_url` | no | Git repository URL for generated code |
| status | `string` | `status` | yes | Lifecycle state: draft, active, archived |

### Requirement: Entity Definition Management

An EntityDefinition SHALL represent a single entity kind within a Project, mapping directly to the `--kind` argument of the entity generator.

#### Scenario: Define an entity
- GIVEN a Project with ID "proj-123"
- WHEN a POST request is made to `/api/rh-trex-ai/v1/entity_definitions` with `{"project_id": "proj-123", "kind_name": "Customer", "description": "Manages customer records"}`
- THEN an EntityDefinition SHALL be created linking to Project "proj-123"
- AND `kind_name` SHALL be validated as a valid Go PascalCase identifier

#### Scenario: Prevent duplicate kind names within a project
- GIVEN a Project with an existing EntityDefinition where `kind_name` is "Customer"
- WHEN a new EntityDefinition with the same `kind_name` and `project_id` is submitted
- THEN the request SHALL be rejected with a 409 Conflict error

#### Scenario: List entities for a project
- GIVEN a Project with 3 EntityDefinitions
- WHEN a GET request is made to `/api/rh-trex-ai/v1/entity_definitions?project_id=proj-123`
- THEN all 3 EntityDefinitions SHALL be returned

**Fields:**

| Field | Go Type | JSON | Required | Description |
|-------|---------|------|----------|-------------|
| project_id | `string` | `project_id` | yes | Parent Project reference |
| kind_name | `string` | `kind_name` | yes | PascalCase entity name (e.g., "Customer") |
| plural_override | `*string` | `plural_override` | no | Custom plural form (e.g., "People" for "Person") |
| description | `*string` | `description` | no | Human-readable description |

### Requirement: Field Definition Management

A FieldDefinition SHALL represent a single field on an EntityDefinition, mapping to individual entries in the `--fields` argument of the entity generator.

#### Scenario: Define a field
- GIVEN an EntityDefinition with ID "ent-456"
- WHEN a POST request is made to `/api/rh-trex-ai/v1/field_definitions` with `{"entity_definition_id": "ent-456", "field_name": "email", "field_type": "string", "is_required": true}`
- THEN a FieldDefinition SHALL be created with `nullable: false` (since required)

#### Scenario: Validate field type
- GIVEN a POST request with `field_type: "invalid_type"`
- WHEN the handler validates the request
- THEN the request SHALL be rejected with a 400 Bad Request
- AND the error message SHALL list valid types: string, int, int64, bool, float, time

#### Scenario: Field name uniqueness per entity
- GIVEN an EntityDefinition with a field named "email"
- WHEN a new FieldDefinition with the same `field_name` and `entity_definition_id` is submitted
- THEN the request SHALL be rejected with a 409 Conflict error

**Fields:**

| Field | Go Type | JSON | Required | Description |
|-------|---------|------|----------|-------------|
| entity_definition_id | `string` | `entity_definition_id` | yes | Parent EntityDefinition reference |
| field_name | `string` | `field_name` | yes | snake_case field name |
| field_type | `string` | `field_type` | yes | One of: string, int, int64, bool, float, time |
| nullable | `*bool` | `nullable` | no | Whether the field accepts null (default: true) |
| is_required | `*bool` | `is_required` | no | Whether the field is required in API requests (default: false) |

### Requirement: Relationship Management

A Relationship SHALL define a foreign-key association between two EntityDefinitions within the same Project.

#### Scenario: Define a has_many relationship
- GIVEN EntityDefinitions "Customer" (ID: ent-1) and "Order" (ID: ent-2) in the same Project
- WHEN a POST request is made with `{"project_id": "proj-123", "source_entity_id": "ent-1", "target_entity_id": "ent-2", "relationship_type": "has_many"}`
- THEN a Relationship SHALL be created representing "Customer has_many Orders"

#### Scenario: Validate same-project constraint
- GIVEN source EntityDefinition in Project "proj-A" and target EntityDefinition in Project "proj-B"
- WHEN a Relationship is submitted
- THEN the request SHALL be rejected with a 400 Bad Request error
- AND the error message SHALL indicate that both entities must belong to the same project

#### Scenario: Validate relationship type
- GIVEN a POST request with `relationship_type: "invalid"`
- WHEN the handler validates the request
- THEN the request SHALL be rejected with a 400 Bad Request
- AND valid types SHALL be: has_one, has_many, belongs_to, many_to_many

#### Scenario: Prevent self-referencing relationships
- GIVEN a source_entity_id equal to target_entity_id
- WHEN the Relationship is submitted
- THEN the request SHALL be rejected with a 400 Bad Request error

**Fields:**

| Field | Go Type | JSON | Required | Description |
|-------|---------|------|----------|-------------|
| project_id | `string` | `project_id` | yes | Parent Project reference |
| source_entity_id | `string` | `source_entity_id` | yes | Source EntityDefinition reference |
| target_entity_id | `string` | `target_entity_id` | yes | Target EntityDefinition reference |
| relationship_type | `string` | `relationship_type` | yes | One of: has_one, has_many, belongs_to, many_to_many |
| foreign_key | `*string` | `foreign_key` | no | Custom FK column name (auto-derived if omitted) |

### Requirement: Build Orchestration

A Build SHALL represent a single execution of the TRex generation pipeline for a Project, capturing inputs and tracking status through a state machine.

#### Scenario: Trigger a build
- GIVEN a Project "proj-123" in status "active" with at least one EntityDefinition
- WHEN a POST request is made to `/api/rh-trex-ai/v1/builds` with `{"project_id": "proj-123", "triggered_by": "user@example.com"}`
- THEN a Build SHALL be created with `status: "pending"`
- AND a CreateEvent SHALL be emitted with Source "Builds"

#### Scenario: Build controller processes pending build
- GIVEN a Build in status "pending"
- WHEN the Build's OnUpsert controller handler fires
- THEN the handler SHALL:
  1. Transition the Build to status "building"
  2. Resolve all EntityDefinitions, FieldDefinitions, and Relationships for the Project
  3. Invoke the entity generator (CG-001) for each EntityDefinition with its resolved fields
  4. Invoke the CLI generator (CG-002) against the generated OpenAPI spec
  5. Invoke the SDK generator (CG-003) against the generated OpenAPI spec
  6. On success: set status to "succeeded" and record `completed_at`
  7. On failure: set status to "failed" and capture error details in `build_log`

#### Scenario: Prevent build for draft project
- GIVEN a Project in status "draft"
- WHEN a Build is requested
- THEN the request SHALL be rejected with a 400 Bad Request
- AND the error SHALL indicate the project must be "active" to build

#### Scenario: Build idempotency
- GIVEN a Build that has already reached status "succeeded"
- WHEN the OnUpsert handler fires again (event replay)
- THEN the handler SHALL detect the terminal status and skip processing

**Fields:**

| Field | Go Type | JSON | Required | Description |
|-------|---------|------|----------|-------------|
| project_id | `string` | `project_id` | yes | Parent Project reference |
| status | `string` | `status` | yes | State: pending, building, succeeded, failed |
| build_log | `*string` | `build_log` | no | Generation output and error details |
| triggered_by | `*string` | `triggered_by` | no | Identity of the user or system that triggered the build |
| completed_at | `*time.Time` | `completed_at` | no | Timestamp when the build reached a terminal state |

### Requirement: Deployment Lifecycle

A Deployment SHALL represent a running instance of a generated API service from a successful Build.

#### Scenario: Create a deployment
- GIVEN a Build "build-789" with status "succeeded"
- WHEN a POST request is made to `/api/rh-trex-ai/v1/deployments` with `{"build_id": "build-789", "project_id": "proj-123", "target_environment": "staging"}`
- THEN a Deployment SHALL be created with `status: "provisioning"`

#### Scenario: Deployment controller provisions instance
- GIVEN a Deployment in status "provisioning"
- WHEN the Deployment's OnUpsert controller handler fires
- THEN the handler SHALL:
  1. Allocate resources for the generated API service
  2. Deploy the built artifacts to the target environment
  3. On success: set status to "running" and record the `endpoint_url`
  4. On failure: set status to "failed" and capture error in Build's log

#### Scenario: Prevent deployment of failed build
- GIVEN a Build with status "failed"
- WHEN a Deployment referencing that Build is submitted
- THEN the request SHALL be rejected with a 400 Bad Request

#### Scenario: Stop a deployment
- GIVEN a Deployment in status "running"
- WHEN a PATCH request sets `{"status": "stopped"}`
- THEN the Deployment SHALL transition to "stopped"
- AND resources SHALL be deallocated by the OnUpsert controller handler

**Fields:**

| Field | Go Type | JSON | Required | Description |
|-------|---------|------|----------|-------------|
| build_id | `string` | `build_id` | yes | Parent Build reference |
| project_id | `string` | `project_id` | yes | Parent Project reference (denormalized for query efficiency) |
| status | `string` | `status` | yes | State: provisioning, running, stopped, failed |
| endpoint_url | `*string` | `endpoint_url` | no | Live API endpoint URL when running |
| target_environment | `string` | `target_environment` | yes | Target: development, staging, production |

### Requirement: Generator Flag Mapping

The platform SHALL translate EntityDefinitions and FieldDefinitions into generator CLI arguments.

#### Scenario: Single entity with fields
- GIVEN an EntityDefinition `kind_name: "Customer"` with FieldDefinitions `[{field_name: "email", field_type: "string", is_required: true}, {field_name: "age", field_type: "int", nullable: true}]`
- WHEN the Build controller resolves generator arguments
- THEN the equivalent command SHALL be: `go run ./scripts/generator.go --kind Customer --fields "email:string:required,age:int"`

#### Scenario: Entity with plural override
- GIVEN an EntityDefinition with `kind_name: "Person"` and `plural_override: "People"`
- WHEN the Build controller resolves generator arguments
- THEN the `--plural People` flag SHALL be included

#### Scenario: Relationship to generator mapping
- GIVEN a `has_many` relationship from "Customer" to "Order"
- WHEN the Build controller processes relationships
- THEN a `customer_id:string:required` field SHALL be auto-injected into the "Order" EntityDefinition
- AND the foreign key column SHALL follow the pattern `{source_kind_snake_case}_id`

### Requirement: API Path Structure

All platform entities SHALL follow the TRex REST conventions under the `/api/rh-trex-ai/v1/` prefix.

#### Scenario: Resource paths
- GIVEN the managed API platform is running
- THEN the following paths SHALL be available:
  - `GET/POST /api/rh-trex-ai/v1/projects`
  - `GET/PATCH/DELETE /api/rh-trex-ai/v1/projects/{id}`
  - `GET/POST /api/rh-trex-ai/v1/entity_definitions`
  - `GET/PATCH/DELETE /api/rh-trex-ai/v1/entity_definitions/{id}`
  - `GET/POST /api/rh-trex-ai/v1/field_definitions`
  - `GET/PATCH/DELETE /api/rh-trex-ai/v1/field_definitions/{id}`
  - `GET/POST /api/rh-trex-ai/v1/relationships`
  - `GET/PATCH/DELETE /api/rh-trex-ai/v1/relationships/{id}`
  - `GET/POST /api/rh-trex-ai/v1/builds`
  - `GET/DELETE /api/rh-trex-ai/v1/builds/{id}`
  - `GET/POST /api/rh-trex-ai/v1/deployments`
  - `GET/PATCH/DELETE /api/rh-trex-ai/v1/deployments/{id}`

### Requirement: Parent-Child Query Filtering

List endpoints for child entities SHALL support filtering by parent ID.

#### Scenario: Filter entity definitions by project
- GIVEN 5 EntityDefinitions across 2 Projects
- WHEN `GET /api/rh-trex-ai/v1/entity_definitions?project_id=proj-123`
- THEN only EntityDefinitions belonging to "proj-123" SHALL be returned

#### Scenario: Filter field definitions by entity
- GIVEN 10 FieldDefinitions across 3 EntityDefinitions
- WHEN `GET /api/rh-trex-ai/v1/field_definitions?entity_definition_id=ent-456`
- THEN only FieldDefinitions belonging to "ent-456" SHALL be returned

#### Scenario: Filter builds by project
- GIVEN 4 Builds across 2 Projects
- WHEN `GET /api/rh-trex-ai/v1/builds?project_id=proj-123`
- THEN only Builds belonging to "proj-123" SHALL be returned

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| 6 entities (Project, EntityDefinition, FieldDefinition, Relationship, Build, Deployment) | 3x current entity count; exercises parent-child hierarchies, state machines, and cross-entity validation that the toy "dinosaurs" model doesn't test |
| Separate FieldDefinition entity (not embedded JSON) | Enables individual CRUD, audit trail per field, and filtering — proves the generator handles high entity counts |
| Build as immutable snapshot | Generation is auditable and reproducible; failed builds are preserved for debugging |
| Deployment separate from Build | One Build can have multiple Deployments (staging, production); decouples generation from operational lifecycle |
| New `app/` spec domain | Cleanly separates "what TRex does as a product" from "how TRex works as a framework" |
| Denormalized `project_id` on Deployment | Avoids joining through Build for common project-scoped queries |
| Relationship auto-injects FK fields | Mirrors real ORM behavior; validates that the generator can handle derived fields |
| Status field as string enum (not Go enum) | Matches existing TRex pattern; validated in handler layer |
| No self-referencing relationships | Simplifies initial implementation; MAY be added in a future spec revision |
