# Authorization Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** SEC-002
**Related:** [Authentication](authentication.spec.md)
**Implements:** `pkg/auth/authz_middleware.go`, `pkg/client/apiclient/`

---

## Purpose

Define the authorization middleware chain that controls access to API endpoints based on authenticated identity, roles, and resource-level policies.

## Requirements

### Requirement: Authorization Middleware Interface

The `AuthorizationMiddleware` SHALL implement an `AuthorizeApi` method that wraps HTTP handlers.

#### Scenario: Authorized request
- GIVEN an authenticated user with appropriate permissions
- WHEN the request passes through `AuthorizeApi` middleware
- THEN the request SHALL be forwarded to the next handler

#### Scenario: Unauthorized request
- GIVEN an authenticated user without appropriate permissions
- WHEN the request passes through `AuthorizeApi` middleware
- THEN the response SHALL be `403 Forbidden`

### Requirement: Configurable Authorization

Authorization SHALL be toggleable via `--enable-authz` flag.

#### Scenario: Authorization disabled
- GIVEN `--enable-authz=false` in the server configuration
- WHEN a request arrives
- THEN the `AuthorizeApi` middleware SHALL pass all requests through without checking permissions

### Requirement: Route-Level Authorization

Authorization middleware SHALL be applied at the subrouter level for each entity.

#### Scenario: Entity route protection
- GIVEN a dinosaurs subrouter at `/api/rh-trex/v1/dinosaurs`
- WHEN routes are registered in the plugin's `init()` function
- THEN `dinosaursRouter.Use(authzMiddleware.AuthorizeApi)` SHALL be applied
- AND all CRUD endpoints under that router SHALL require authorization

### Requirement: Mock Authorization for Testing

A mock `AuthorizationMiddleware` SHALL be available for integration testing.

#### Scenario: Test with mock authorization
- GIVEN an integration test environment
- WHEN mock authorization is configured
- THEN all requests SHALL be authorized without external service calls

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Middleware-based authorization | Consistent with gorilla/mux patterns; composable with auth middleware |
| Subrouter-level application | Groups authorization by entity; different entities can have different policies |
| Toggle via configuration | Simplifies local development; mock mode for testing |
| Interface-based AuthorizationMiddleware | Enables mock and real implementations without code changes |
