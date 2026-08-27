# REST Conventions Specification

**Date:** 2026-08-03
**Status:** Active
**ID:** API-001
**Related:** [Entity Lifecycle](../framework/entity-lifecycle.spec.md), [Error Handling](../standards/error-handling.spec.md), [Testing Standards](../standards/testing.spec.md)
**Implements:** `pkg/handlers/`, `pkg/server/route_builder.go`, `openapi/`

---

## Purpose

Define the REST API conventions governing endpoint design, request/response format, pagination, and OpenAPI specification compliance.

## Requirements

### Requirement: URL Path Convention

All unscoped entity API paths SHALL follow the pattern `/api/{project}/v1/{kind_snake_case_plural}`. A scoped resource view MAY add alternating collection and identifier segments beneath the API version prefix, and every identifier segment SHALL have a declared OpenAPI path parameter.

#### Scenario: Dinosaur entity paths
- GIVEN the project "rh-trex" and entity "Dinosaur"
- THEN the API paths SHALL be:
  - `GET /api/rh-trex/v1/dinosaurs` (list)
  - `POST /api/rh-trex/v1/dinosaurs` (create)
  - `GET /api/rh-trex/v1/dinosaurs/{id}` (get)
  - `PATCH /api/rh-trex/v1/dinosaurs/{id}` (update)
  - `DELETE /api/rh-trex/v1/dinosaurs/{id}` (delete)

#### Scenario: Parent-scoped collection
- GIVEN an Inbox resource view scoped to an Agent
- THEN its collection path MAY be `/api/rh-trex/v1/agents/{agent_id}/inbox`
- AND `agent_id` SHALL be declared as a required path parameter

### Requirement: HTTP Method Semantics

API endpoints SHALL use correct HTTP methods and status codes.

#### Scenario: Successful CRUD responses
- GIVEN a valid authenticated request
- WHEN a resource is created via POST → THEN response SHALL be `201 Created`
- WHEN a resource is retrieved via GET → THEN response SHALL be `200 OK`
- WHEN a resource is updated via PATCH → THEN response SHALL be `200 OK`
- WHEN a resource is deleted via DELETE → THEN response SHALL be `204 No Content`
- WHEN a list is retrieved via GET → THEN response SHALL be `200 OK`

### Requirement: List Response Format

List endpoints SHALL return a paginated response with `kind`, `page`, `size`, `total`, and `items` fields.

#### Scenario: Paginated list response
- GIVEN 50 dinosaurs in the database
- WHEN `GET /api/rh-trex/v1/dinosaurs?page=2&size=10` is requested
- THEN the response SHALL contain:
  - `kind: "DinosaurList"`
  - `page: 2`
  - `size: 10`
  - `total: 50`
  - `items: [...]` (10 dinosaur objects)

### Requirement: ObjectReference Fields

Every API resource response SHALL include `id`, `kind`, and `href` fields.

#### Scenario: Resource identification
- GIVEN a Dinosaur with ID "abc-123"
- WHEN the resource is returned in an API response
- THEN the response SHALL include:
  - `id: "abc-123"`
  - `kind: "Dinosaur"`
  - `href: "/api/rh-trex/v1/dinosaurs/abc-123"`

### Requirement: PATCH Partial Updates

PATCH endpoints SHALL support partial updates where only provided fields are modified.

#### Scenario: Partial update
- GIVEN a Dinosaur with `species: "T-Rex"` and `name: "Rex"`
- WHEN `PATCH /api/rh-trex/v1/dinosaurs/{id}` with body `{"species": "Velociraptor"}` is sent
- THEN `species` SHALL be updated to "Velociraptor"
- AND `name` SHALL remain "Rex" (unchanged)

### Requirement: OpenAPI Specification Compliance

Each entity SHALL have a companion OpenAPI YAML file at `openapi/openapi.{kindLowerPlural}.yaml`.

#### Scenario: OpenAPI file structure
- GIVEN a "Dinosaur" entity
- THEN `openapi/openapi.dinosaurs.yaml` SHALL define:
  - Path definitions for all CRUD endpoints
  - Schema definitions for `Dinosaur`, `DinosaurList`, `DinosaurPatchRequest`
  - The main `openapi/openapi.yaml` SHALL reference these via `$ref`

### Requirement: Stable Operation Identity

Every public OpenAPI operation SHALL declare a globally unique, stable `operationId`. An operation ID SHALL describe the operation's API meaning and SHALL remain unchanged when the specification is split into different files.

#### Scenario: Generated CRUD operation IDs
- GIVEN the Dinosaur entity exposes complete CRUD operations
- THEN its operation IDs SHALL include `listDinosaurs`, `createDinosaur`, `getDinosaur`, `updateDinosaur`, and `deleteDinosaur`
- AND no other operation in the root OpenAPI document SHALL reuse those IDs

#### Scenario: Operation ID migration remains buildable
- GIVEN an existing operation ID is replaced with its semantic operation ID
- WHEN the OpenAPI client is regenerated
- THEN all in-repository consumers SHALL be updated atomically to use the regenerated client method
- AND the repository SHALL build without retaining a second compatibility operation for the obsolete operation ID

### Requirement: Navigable Operation Relationships

A relationship intended for generated client navigation SHALL be declared with an OpenAPI Link Object when it cannot be represented unambiguously by a resource view's own path and parameters. The Link Object SHALL target a stable `operationId` and SHALL map every target parameter supplied by the source operation.

#### Scenario: Agent to scoped inbox
- GIVEN `getAgent` returns an Agent whose `id` scopes `listAgentInbox`
- WHEN the API author declares generated navigation from the Agent to its inbox
- THEN the `getAgent` response SHALL contain a Link Object targeting `listAgentInbox`
- AND the link SHALL map `agent_id` from the source response

### Requirement: Canonical OpenAPI Completeness

The root `openapi/openapi.yaml` document SHALL include or reference every registered public application operation intended for API clients, including CRUD, action, and streaming operations. Each operation SHALL fully declare its parameters, request body when applicable, responses, content types, and security requirements.

#### Scenario: Router and specification agree
- GIVEN all plugins and server routes are registered
- WHEN the public router is compared with the resolved root OpenAPI document
- THEN every public method and path SHALL have a corresponding OpenAPI operation
- AND every documented public method and path SHALL resolve to a registered route

### Requirement: Automated Route-Spec Parity

An automated test SHALL compare every registered plugin application route intended for API clients with the fully resolved root OpenAPI document. The comparison SHALL normalize route templates and HTTP methods, SHALL fail for undocumented registered operations, and SHALL fail for documented operations that have no registered route.

#### Scenario: Undocumented DELETE route
- GIVEN a plugin registers `DELETE /api/rh-trex/v1/dinosaurs/{id}`
- AND the resolved root OpenAPI document omits that operation
- WHEN the route-spec parity test runs in the standard test suite
- THEN the test SHALL fail and identify the unmatched method and normalized path

### Requirement: Content-Type Handling

All API endpoints SHALL accept and return `application/json`.

#### Scenario: Request content type
- GIVEN a POST request without `Content-Type: application/json`
- WHEN the request is processed
- THEN the server SHALL return `400 Bad Request`

### Requirement: CORS Configuration

The API server SHALL support configurable CORS with allowed origins, methods, and headers.

#### Scenario: CORS preflight
- GIVEN CORS is configured with allowed origin "https://console.example.com"
- WHEN an OPTIONS preflight request arrives from that origin
- THEN the response SHALL include `Access-Control-Allow-Origin: https://console.example.com`
- AND allowed methods SHALL include GET, POST, PATCH, DELETE

### Requirement: Trailing Slash Normalization

The API server SHALL strip trailing slashes from request paths before routing.

#### Scenario: Trailing slash redirect
- GIVEN a request to `GET /api/rh-trex/v1/dinosaurs/`
- WHEN the request is processed
- THEN it SHALL be handled identically to `GET /api/rh-trex/v1/dinosaurs`

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| PATCH over PUT for updates | Supports partial updates; less error-prone for clients |
| Snake_case in URLs and JSON | Consistent with OpenAPI and PostgreSQL column naming |
| Separate OpenAPI files per entity | Prevents monolithic spec file; enables independent generation |
| `$ref` composition in main openapi.yaml | Auto-composable; generator appends references without conflicts |
| Stable `operationId` values | Gives generators and OpenAPI Links an unambiguous cross-file operation key |
| Atomic client regeneration for operation ID changes | Generated method names derive from operation IDs; updating the document, client, and consumers together avoids a mixed contract |
| Link Objects for semantic navigation | Standard OpenAPI relationships can disambiguate hierarchy without imposing one schema tree |
| Root document is canonical | All generated clients must see the same complete public API surface |
| 204 for DELETE (no body) | RESTful convention; nothing to return after deletion |
