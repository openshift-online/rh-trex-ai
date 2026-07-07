# Managed API Platform Specification

**Date:** 2026-07-07
**Status:** Draft
**ID:** APP-001
**Related:** [Entity Generator](../codegen/entity-generator.spec.md), [CLI Generator](../codegen/cli-generator.spec.md), [SDK Generator](../codegen/sdk-generator.spec.md), [Console Plugin Generator](../codegen/console-plugin-generator.spec.md), [Entity Lifecycle](../framework/entity-lifecycle.spec.md), [Event-Driven Controllers](../framework/event-driven-controllers.spec.md), [Plugin Architecture](../framework/plugin-architecture.spec.md)
**Implements:** `plugins/projects/`, `plugins/entity_definitions/`, `plugins/field_definitions/`, `plugins/relationships/`, `plugins/builds/`

---

## Purpose

Transform TRex from a demonstrative "dinosaurs" API into a managed API platform that orchestrates the full TRex generation pipeline. Users define their entity-relationship diagrams (ERD) through the API, and the platform generates complete API services (REST, gRPC, CLI, SDK, Console Plugin) from those definitions. This dogfoods every TRex generator (CG-001 through CG-004) and stress-tests the entity system with a non-trivial relational data model featuring parent-child hierarchies, state machines, and cross-entity referential integrity.

## Phasing

### Phase 1 (this spec)

5 entities: Project, EntityDefinition, FieldDefinition, Relationship, Build. These cover the full ERD-definition and generation workflow.

### Phase 2 (future spec)

Deployment entity — requires defining the deployment target (containers, Kubernetes, process-per-project) and resource lifecycle management. Deferred until the Build pipeline is proven.

## Prerequisites — Generator Enhancements Required

The entity generator (CG-001, `scripts/generator.go`) produces ~40% of the code needed for this spec's entities. The following capabilities are **missing** and MUST be added to CG-001 (or a follow-up spec extending it) before APP-001 can be fully implemented:

| Capability | Current State | Required Enhancement |
|------------|--------------|---------------------|
| Foreign key fields | No `--parent` or `--references` flag | Add `--parent ParentKind` flag that generates a `parent_kind_id` field, DB index, and `FindByParentKindID` DAO method |
| Parent-child cascade delete | Generated delete is simple, no cascade | Generate cascade soft-delete in parent's `OnDelete` handler when `--parent` is used |
| State machine validation | Status is a plain string field | Add `--status "draft,active,archived"` flag that generates a `ValidateStatusTransition()` method and handler-layer validation |
| Filtered DAO queries | Only `Get`, `Create`, `Replace`, `Delete`, `All` | Generate `FindBy{Field}` methods for fields marked with a `:indexed` modifier |
| Uniqueness constraints | No unique constraint support | Add `:unique` field modifier that generates a DB unique index and 409 Conflict error mapping |
| Migration ordering for FK dependencies | SHA256 hash-based IDs are non-deterministic | Add `--migration-order N` flag or use topological sort based on `--parent` graph |

Until these enhancements land, the gap between generated code and the spec's requirements MUST be bridged by hand-coded post-generation customization (see [Post-Generation Customization](#post-generation-customization)).

## Data Model

### Entity-Relationship Diagram

```
Project (1) ──────< (N) EntityDefinition
    │                      │
    │                      ├──< (N) FieldDefinition
    │                      │
    │                      └──< (N) Relationship (source + target)
    │
    └──< (N) Build
```

### Entity Summary

| Entity | Plugin Directory | Parent | State Machine | Phase |
|--------|-----------------|--------|---------------|-------|
| Project | `plugins/projects/` | — | draft → active → archived | 1 |
| EntityDefinition | `plugins/entity_definitions/` | Project | — | 1 |
| FieldDefinition | `plugins/field_definitions/` | EntityDefinition | — | 1 |
| Relationship | `plugins/relationships/` | Project (+ source/target EntityDefinition) | — | 1 |
| Build | `plugins/builds/` | Project | pending → building → succeeded / failed | 1 |
| Deployment | `plugins/deployments/` | Build + Project | provisioning → running → stopped / failed | 2 (future) |

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
- GIVEN a Project with associated EntityDefinitions, FieldDefinitions, Relationships, and Builds
- WHEN the Project is deleted
- THEN all child EntityDefinitions SHALL be soft-deleted
- AND all child FieldDefinitions SHALL be soft-deleted (via EntityDefinition cascade)
- AND all child Relationships SHALL be soft-deleted
- AND all child Builds SHALL be soft-deleted

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
- AND uniqueness SHALL be enforced via a composite DB unique index on `(project_id, kind_name)` with GORM error mapping to 409

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
- AND uniqueness SHALL be enforced via a composite DB unique index on `(entity_definition_id, field_name)` with GORM error mapping to 409

**Fields:**

| Field | Go Type | JSON | Required | Description |
|-------|---------|------|----------|-------------|
| entity_definition_id | `string` | `entity_definition_id` | yes | Parent EntityDefinition reference |
| field_name | `string` | `field_name` | yes | snake_case field name |
| field_type | `string` | `field_type` | yes | One of: string, int, int64, bool, float, time |
| is_required | `*bool` | `is_required` | no | Whether the field is required in API requests and non-nullable in Go (default: false). When `true`, the field is non-nullable (`string` not `*string`) and listed in the OpenAPI `required` array. When `false` or omitted, the field is nullable (pointer type). |

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

A Build SHALL represent a single execution of the TRex generation pipeline for a Project, capturing inputs and tracking status through a state machine. Builds are **immutable after creation** — only the Build controller (via event handlers) MAY update the status and build_log fields. No PATCH endpoint is exposed.

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
  3. Create an isolated workspace directory under a configured `build_workspace_root` path (default: `/tmp/trex-builds/{build_id}/`)
  4. For each EntityDefinition, invoke the entity generator as a subprocess: `go run ./scripts/generator.go --kind {KindName} --fields {resolved_fields} --repo {project.repository_url} --project {project.name}` within the workspace
  5. Invoke the CLI generator (CG-002) as a subprocess against the workspace's generated OpenAPI spec
  6. Invoke the SDK generator (CG-003) as a subprocess against the workspace's generated OpenAPI spec
  7. On success: set status to "succeeded", record `completed_at`, and store the workspace path or artifact reference in `build_log`
  8. On failure: set status to "failed" and capture stderr/stdout in `build_log`

#### Scenario: Prevent build for draft project
- GIVEN a Project in status "draft"
- WHEN a Build is requested
- THEN the request SHALL be rejected with a 400 Bad Request
- AND the error SHALL indicate the project must be "active" to build

#### Scenario: Build idempotency
- GIVEN a Build that has already reached status "succeeded" or "failed"
- WHEN the OnUpsert handler fires again (event replay)
- THEN the handler SHALL detect the terminal status and skip processing

**Fields:**

| Field | Go Type | JSON | Required | Description |
|-------|---------|------|----------|-------------|
| project_id | `string` | `project_id` | yes | Parent Project reference |
| status | `string` | `status` | yes | State: pending, building, succeeded, failed |
| build_log | `*string` | `build_log` | no | Generation output, error details, and workspace path |
| triggered_by | `*string` | `triggered_by` | no | Identity of the user or system that triggered the build |
| completed_at | `*time.Time` | `completed_at` | no | Timestamp when the build reached a terminal state |

### Requirement: Deployment Lifecycle (Phase 2 — Future)

A Deployment SHALL represent a running instance of a generated API service from a successful Build. This requirement is deferred to Phase 2 pending resolution of deployment target architecture (containers, Kubernetes pods, process-per-project, etc.).

**Status:** Future — not implemented in Phase 1.

**Fields (preliminary):**

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
- GIVEN an EntityDefinition `kind_name: "Customer"` with FieldDefinitions `[{field_name: "email", field_type: "string", is_required: true}, {field_name: "age", field_type: "int", is_required: false}]`
- WHEN the Build controller resolves generator arguments
- THEN the equivalent command SHALL be: `go run ./scripts/generator.go --kind Customer --fields "email:string:required,age:int"`

#### Scenario: Entity with plural override
- GIVEN an EntityDefinition with `kind_name: "Person"` and `plural_override: "People"`
- WHEN the Build controller resolves generator arguments
- THEN the `--plural People` flag SHALL be included

#### Scenario: Library mode for downstream projects
- GIVEN a Build for a Project with `repository_url: "github.com/myorg/my-service"`
- WHEN the Build controller resolves generator arguments
- THEN the `--library github.com/openshift-online/rh-trex-ai` flag SHALL be included
- AND `--repo github.com/myorg/my-service` and `--project my-service` SHALL be included

#### Scenario: Relationship to generator mapping
- GIVEN a `has_many` relationship from "Customer" to "Order"
- WHEN the Build controller processes relationships
- THEN a `customer_id:string:required` field SHALL be auto-injected into the "Order" EntityDefinition's resolved field list
- AND the foreign key column SHALL follow the pattern `{source_kind_snake_case}_id`
- AND this auto-injection requires the CG-001 `--parent` enhancement (see [Prerequisites](#prerequisites--generator-enhancements-required)); until then, FK fields MUST be manually added as FieldDefinitions

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

## Post-Generation Customization

The entity generator (CG-001) produces scaffolding for each entity. The following customizations MUST be applied manually after generation until the corresponding generator enhancements are implemented:

| Entity | Post-Generation Work | Generator Enhancement That Eliminates It |
|--------|---------------------|------------------------------------------|
| **All entities with parents** | Add `{parent}_id` foreign key field to model, DB index in migration, and `FindBy{Parent}ID()` DAO method | `--parent` flag (CG-001) |
| **Project** | Add `ValidateStatusTransition()` to service layer; reject invalid transitions in handler | `--status` flag (CG-001) |
| **Build** | Add `ValidateStatusTransition()` to service layer; reject invalid transitions in handler | `--status` flag (CG-001) |
| **Project** | Add cascade soft-delete of children in `OnDelete` handler | `--parent` cascade support (CG-001) |
| **EntityDefinition** | Add composite unique index `(project_id, kind_name)` to migration; map GORM duplicate key error to 409 | `:unique` field modifier (CG-001) |
| **FieldDefinition** | Add composite unique index `(entity_definition_id, field_name)` to migration; map GORM duplicate key error to 409 | `:unique` field modifier (CG-001) |
| **Relationship** | Add same-project validation and self-reference check in handler | Custom validation (likely always hand-coded) |
| **Build** | Implement subprocess-based generation orchestration in `OnUpsert` handler | N/A (application-specific logic) |
| **All child entities** | Add `FindBy{Parent}ID()` filtered query to DAO and expose via query parameter in handler | `:indexed` field modifier (CG-001) |

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| 5 entities in Phase 1, Deployment deferred to Phase 2 | Deployment requires defining infrastructure targets (K8s, containers, processes) which is orthogonal to the ERD-to-generation pipeline. Ship the core loop first. |
| Build is immutable (no PATCH endpoint) | Only the controller changes Build state, ensuring a clean audit trail. Users can only trigger (POST) or cancel (DELETE) builds. |
| Build controller uses subprocess execution | The entity generator is a file-system tool (`go run ./scripts/generator.go`), not a library. Subprocess invocation in an isolated workspace directory is the simplest correct approach. The Build controller creates a temp directory, shells out, and captures stdout/stderr. |
| Separate FieldDefinition entity (not embedded JSON) | Enables individual CRUD, audit trail per field, and filtering — proves the generator handles high entity counts |
| `is_required` subsumes `nullable` | A single `is_required` boolean controls both Go type (pointer vs value) and OpenAPI `required` array membership. No separate `nullable` field — `is_required: true` means non-nullable, `is_required: false` or omitted means nullable. Eliminates ambiguity. |
| Uniqueness enforced at DB level with error mapping | Composite unique indexes on `(project_id, kind_name)` and `(entity_definition_id, field_name)` prevent races. GORM duplicate key errors are mapped to 409 Conflict in the service layer. |
| New `app/` spec domain | Cleanly separates "what TRex does as a product" from "how TRex works as a framework" |
| Relationship auto-injection is aspirational | FK field auto-injection from Relationships requires CG-001 `--parent` support. Until then, FK fields are added as explicit FieldDefinitions. The spec documents both the target state and the interim workaround. |
| Status field as string enum (not Go enum) | Matches existing TRex pattern; validated in handler layer |
| No self-referencing relationships | Simplifies initial implementation; MAY be added in a future spec revision |
