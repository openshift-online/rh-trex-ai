# Error Handling Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** STD-002
**Related:** [REST Conventions](../api/rest-conventions.spec.md)
**Implements:** `pkg/errors/errors.go`, `pkg/handlers/`

---

## Purpose

Define the standardized error handling system including error types, HTTP status code mapping, and structured error responses.

## Requirements

### Requirement: ServiceError Type System

All API errors SHALL be represented as `ServiceError` with a typed error code, human-readable reason, and HTTP status code.

#### Scenario: Error code to HTTP status mapping
- THEN the following mappings SHALL be enforced:
  - `ErrorNotFound` (7) → 404 Not Found
  - `ErrorBadRequest` (21) → 400 Bad Request
  - `ErrorValidation` (8) → 400 Bad Request
  - `ErrorMalformedRequest` (17) → 400 Bad Request
  - `ErrorUnauthenticated` (15) → 401 Unauthorized
  - `ErrorUnauthorized` (11) → 403 Forbidden
  - `ErrorForbidden` (4) → 403 Forbidden
  - `ErrorConflict` (6) → 409 Conflict
  - `ErrorGeneral` (9) → 500 Internal Server Error
  - `ErrorNotImplemented` (10) → 405 Method Not Allowed

### Requirement: Error Response Format

All API error responses SHALL conform to the OpenAPI `Error` schema.

#### Scenario: Error response body
- GIVEN a request that produces `ErrorNotFound`
- THEN the response body SHALL contain:
  ```json
  {
    "kind": "Error",
    "id": "7",
    "href": "/api/rh-trex-ai/v1/errors/7",
    "code": "rh-trex-ai-7",
    "reason": "Resource not found",
    "operation_id": "{operation_id}"
  }
  ```

### Requirement: Configurable Error Prefix

The error code prefix and href base SHALL be configurable via `SetErrorCodePrefix` and `SetErrorHref`.

#### Scenario: Downstream project error codes
- GIVEN a downstream project "my-service"
- WHEN `errors.SetErrorCodePrefix("my-service")` is called
- THEN error codes SHALL be formatted as `my-service-{code}` instead of `rh-trex-ai-{code}`

### Requirement: Constructor Functions

Named constructor functions SHALL exist for all standard error types.

#### Scenario: Error construction
- GIVEN code needs to return a "not found" error
- WHEN `errors.NotFound("dinosaur with id %s not found", id)` is called
- THEN the returned `*ServiceError` SHALL have code 7, the formatted reason, and HTTP 404

### Requirement: Error Introspection

`ServiceError` SHALL support type checking via `Is404()`, `IsConflict()`, and `IsForbidden()` methods.

#### Scenario: Error type checking
- GIVEN a service error returned from a function
- WHEN `err.Is404()` is called
- THEN it SHALL return `true` if the error code is `ErrorNotFound`

### Requirement: No Internal Details in Error Responses

Error reasons returned to clients SHALL NOT expose internal implementation details such as SQL queries, stack traces, or file paths.

#### Scenario: Database error translation
- GIVEN a GORM unique constraint violation
- WHEN the error is translated to a `ServiceError`
- THEN the reason SHALL be "An entity with the specified unique values already exists"
- AND NOT the raw PostgreSQL error message

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Numeric error codes with string prefix | Machine-parseable; prefix identifies the service source |
| Configurable prefix/href | Enables reuse across downstream projects without code changes |
| Separate from Go `error` interface | Rich error type with HTTP code; implements `Error()` for compatibility |
| Constructor functions over struct literals | Enforces correct code-to-status mapping; prevents misconfiguration |
| `AsOpenapiError` conversion | Clean integration with OpenAPI-generated response models |
