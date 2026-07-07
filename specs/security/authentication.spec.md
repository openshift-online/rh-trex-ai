# Authentication Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** SEC-001
**Related:** [Authorization](authorization.spec.md), [REST Conventions](../api/rest-conventions.spec.md), [gRPC Conventions](../api/grpc-conventions.spec.md)
**Implements:** `pkg/auth/middleware.go`, `pkg/auth/context.go`, `pkg/auth/auth_middleware.go`, `pkg/server/grpcutil/`, `pkg/server/apiserver.go`

---

## Purpose

Define the JWT-based authentication system supporting configurable OIDC providers with JWK key management, token validation, and identity extraction for both HTTP and gRPC protocols.

## Requirements

### Requirement: JWT Token Validation

The `JWTHandler` SHALL validate JWT tokens using RSA public keys loaded from JWK endpoints or local files.

#### Scenario: Valid JWT token
- GIVEN a JWT token signed with RSA and a matching `kid` in the JWK set
- WHEN the token is presented in the `Authorization: Bearer {token}` header
- THEN the token SHALL be parsed and validated
- AND the signing method SHALL be verified as RSA (`*jwt.SigningMethodRSA`)
- AND the parsed `*jwt.Token` SHALL be stored in the request context under `ContextAuthKey`

#### Scenario: Invalid or expired token
- GIVEN a JWT token that is expired, malformed, or signed with an unknown key
- WHEN the token is validated
- THEN the server SHALL respond with `401 Unauthorized`
- AND a warning SHALL be logged (without exposing token content)

### Requirement: JWK Key Loading

The handler SHALL support loading keys from multiple URLs (`--jwk-cert-url`) and a local file (`--jwk-cert-file`). All configured sources SHALL be loaded additively into a single merged key map.

#### Scenario: Multiple URL-based key loading
- GIVEN one or more JWK endpoint URLs are configured via `--jwk-cert-url`
- WHEN keys are loaded
- THEN the handler SHALL fetch the JWK set from each URL via HTTP GET
- AND parse all RSA keys from each response's `keys` array
- AND merge all keys into a single in-memory key map keyed by `kid`
- AND one URL failing SHALL NOT prevent other URLs from loading
- AND a warning SHALL be logged for each URL that fails

#### Scenario: File-based key loading
- GIVEN a local JWK file path is configured via `--jwk-cert-file`
- WHEN keys are loaded
- THEN the handler SHALL read and parse the file
- AND this mode SHALL refresh every 5 minutes (for Kubernetes-mounted secrets rotation)

#### Scenario: Additive file and URL key merging
- GIVEN both `--jwk-cert-file` and `--jwk-cert-url` are configured
- WHEN keys are loaded
- THEN the handler SHALL load keys from the file first, then from all URLs
- AND all keys SHALL be merged additively into a single key map
- AND file-sourced keys and URL-sourced keys SHALL coexist
- AND this behavior SHALL be consistent between the HTTP `JWTHandler` and the gRPC `JWKKeyProvider`

### Requirement: Automatic Key Refresh

The handler SHALL periodically refresh JWK keys and support on-demand refresh for unknown key IDs.

#### Scenario: Periodic refresh
- GIVEN URL-based key loading is configured
- THEN keys SHALL be refreshed every 1 hour via a background goroutine

#### Scenario: Unknown kid on-demand refresh
- GIVEN a token presents a `kid` not in the current key map
- AND the last on-demand refresh was more than 30 seconds ago (`kidRefreshWait`)
- WHEN validation encounters the unknown kid
- THEN a one-shot key refresh SHALL be attempted from all configured sources (file and all URLs)
- AND if the kid is found after refresh, validation SHALL proceed

### Requirement: Public Path Bypass

The handler SHALL support configuring paths that bypass JWT authentication.

#### Scenario: Public path access
- GIVEN `/api/rh-trex/v1` and `/api/rh-trex/v1/openapi` are configured as public paths
- WHEN a request arrives for `/api/rh-trex/v1/openapi`
- THEN the request SHALL be forwarded without JWT validation

#### Scenario: Path traversal prevention
- GIVEN `/api/rh-trex/v1` is a public path
- WHEN a request arrives for `/api/rh-trex/v1/dinosaurs` (a sub-path)
- THEN the request SHALL NOT bypass authentication
- AND only exact path matches (with optional trailing slash) SHALL be treated as public

### Requirement: Claims Extraction

The `Payload` struct SHALL extract user identity from JWT claims with fallback chain support.

#### Scenario: RHSSO token claims
- GIVEN a token with claims `{"username": "jdoe", "first_name": "Jane", "last_name": "Doe"}`
- WHEN `GetAuthPayloadFromContext` is called
- THEN `payload.Username` SHALL be "jdoe"

#### Scenario: Standard OIDC token fallback
- GIVEN a token with claims `{"preferred_username": "jdoe", "given_name": "Jane", "name": "Jane Doe"}`
- WHEN `GetAuthPayloadFromContext` is called
- THEN `payload.Username` SHALL fall back to `preferred_username`
- AND `payload.FirstName` SHALL fall back to `given_name`
- AND if `given_name` is empty, SHALL split `name` on space

### Requirement: Account Authentication Middleware

The `Middleware.AuthenticateAccountJWT` SHALL extract the username from the JWT payload and inject it into the request context.

#### Scenario: Username context injection
- GIVEN a validated JWT token in the request context
- WHEN the `AuthenticateAccountJWT` middleware runs
- THEN the username SHALL be extracted via `GetAuthPayload`
- AND stored in context under `ContextUsernameKey`
- AND the request SHALL be forwarded to the next handler

### Requirement: gRPC Authentication

gRPC requests SHALL support JWT authentication via the `authorization` metadata field using the same JWK infrastructure.

#### Scenario: gRPC JWT validation
- GIVEN a gRPC request with metadata `authorization: Bearer {token}`
- WHEN the `AuthUnaryInterceptor` processes the request
- THEN the token SHALL be extracted from gRPC metadata
- AND validated using the same `JWKKeyProvider` as the HTTP handler
- AND the authenticated username SHALL be injected into the gRPC context

### Requirement: Multi-Issuer Support

The authentication system SHALL support multiple JWK certificate URLs for multi-issuer token validation on both HTTP and gRPC paths.

#### Scenario: Multiple OIDC providers on HTTP
- GIVEN `--jwk-cert-url` is configured with multiple comma-separated URLs (e.g., a cluster-local Keycloak and sso.redhat.com)
- WHEN a token signed by any configured issuer is presented to an HTTP REST endpoint
- THEN keys from all configured JWK endpoints SHALL be available for validation
- AND the token SHALL be accepted if its `kid` matches any loaded key

#### Scenario: Multiple OIDC providers on gRPC
- GIVEN `--jwk-cert-url` or `--grpc-jwk-cert-url` is configured with multiple URLs
- WHEN a token signed by any configured issuer is presented via gRPC metadata
- THEN keys from all configured JWK endpoints SHALL be available for validation
- AND the token SHALL be accepted if its `kid` matches any loaded key

#### Scenario: Mixed file and URL multi-issuer
- GIVEN `--jwk-cert-file` contains keys from issuer A and `--jwk-cert-url` points to issuer B
- WHEN a token from either issuer A or issuer B is presented
- THEN the token SHALL be accepted on both HTTP and gRPC paths
- AND neither source SHALL shadow or exclude the other

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Custom JWTHandler over third-party library | Removes OCM SDK dependency; supports multi-provider OIDC |
| Atomic key map replacement | Thread-safe; avoids partial key set state during refresh |
| 30-second cooldown on kid-refresh | Prevents refresh storms from invalid tokens |
| Exact path matching for public paths | Prevents auth bypass via path prefix exploitation |
| Same JWK infrastructure for HTTP and gRPC | Consistent authentication; single key management surface |
| Additive multi-source key merging | File and URL sources are complementary, not mutually exclusive; enables mixed-issuer deployments (e.g., cluster-local Keycloak + sso.redhat.com) |
| File-based keys refresh every 5 minutes | Supports Kubernetes secret rotation lifecycle |
