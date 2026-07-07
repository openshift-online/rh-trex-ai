# REST Conventions Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** API-001
**Related:** [Entity Lifecycle](../framework/entity-lifecycle.spec.md), [Error Handling](../standards/error-handling.spec.md)
**Implements:** `pkg/handlers/`, `pkg/server/route_builder.go`, `openapi/`

---

## Purpose

Define the REST API conventions governing endpoint design, request/response format, pagination, and OpenAPI specification compliance.

## Requirements

### Requirement: URL Path Convention

All entity API paths SHALL follow the pattern `/api/{project}/v1/{kind_snake_case_plural}`.

#### Scenario: Dinosaur entity paths
- GIVEN the project "rh-trex" and entity "Dinosaur"
- THEN the API paths SHALL be:
  - `GET /api/rh-trex/v1/dinosaurs` (list)
  - `POST /api/rh-trex/v1/dinosaurs` (create)
  - `GET /api/rh-trex/v1/dinosaurs/{id}` (get)
  - `PATCH /api/rh-trex/v1/dinosaurs/{id}` (update)
  - `DELETE /api/rh-trex/v1/dinosaurs/{id}` (delete)

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
| 204 for DELETE (no body) | RESTful convention; nothing to return after deletion |
