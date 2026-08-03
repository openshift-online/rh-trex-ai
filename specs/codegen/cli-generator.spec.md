# CLI Generator Specification

**Date:** 2026-08-03
**Status:** Active
**ID:** CG-002
**Related:** [REST Conventions](../api/rest-conventions.spec.md), [Testing Standards](../standards/testing.spec.md), [OpenAPI Intermediate Representation](openapi-ir.spec.md)
**Implements:** `scripts/cli-generator/`

---

## Purpose

Define the CLI tool generator that scaffolds complete command-line interfaces from OpenAPI specifications.

## Requirements

### Requirement: OpenAPI-Driven Generation

The CLI generator SHALL use the canonical OpenAPI IR to discover resource views and their documented operations.

#### Scenario: CLI generation from spec
- GIVEN a project with `openapi/openapi.yaml` exposing complete CRUD operations for Dinosaur, Fossil, and Scientist resource views
- WHEN the CLI generator runs
- THEN commands SHALL be generated for each entity: `list`, `get`, `create`, `update`, `delete`
- AND an operation that is absent from the OpenAPI document SHALL NOT produce a command

### Requirement: Shared IR Consumption

The CLI generator SHALL consume the shared normalized OpenAPI IR and SHALL NOT maintain an independent raw OpenAPI parser or schema-to-resource heuristic.

#### Scenario: Helper schema
- GIVEN the canonical IR contains `AgentPatchRequest` only as a request schema
- WHEN the CLI generator runs
- THEN it SHALL NOT generate an `agent-patch-requests` command group

### Requirement: Scoped Path Fidelity

Generated commands SHALL bind every parameter required by their operation's exact path, including parameters that scope a resource view through one or more parents.

#### Scenario: Nested inbox command
- GIVEN `listAgentInbox` has path `/organizations/{organization_id}/agents/{agent_id}/inbox`
- WHEN the corresponding CLI command is generated
- THEN it SHALL require or otherwise resolve both `organization_id` and `agent_id`
- AND it SHALL call the exact documented path

### Requirement: Generated CLI Acceptance Tests

The CLI generator SHALL have black-box acceptance tests that generate a standalone CLI into a temporary directory, build it, inspect its command surface, and execute representative commands against an in-process mock HTTP server. The tests SHALL verify operation-derived command presence, absence of unsupported commands, flags and required inputs, exact methods and paths, query and body serialization, and authentication behavior. Covered legacy cases SHALL use the same assertions before and after migration to the canonical IR.

#### Scenario: Generated scoped command
- GIVEN a fixture exposes a parent-scoped operation with path, query, body, and Bearer authentication inputs
- WHEN the generated CLI command is built and executed against the mock server
- THEN the observed request SHALL match the documented method, expanded path, query, body, and authorization header
- AND no command SHALL exist for an operation absent from the fixture

### Requirement: Authentication Integration

The generated CLI SHALL support login via JWT token or OIDC flow.

#### Scenario: CLI login
- GIVEN a generated CLI tool
- WHEN `mytool login --token={JWT}` is executed
- THEN the token SHALL be stored for subsequent API calls

### Requirement: Standalone Module

The CLI generator SHALL produce a standalone Go module with its own `go.mod`.

#### Scenario: Independent build
- GIVEN the generated CLI code
- WHEN `go build` is run in the CLI directory
- THEN the CLI binary SHALL build without depending on the API server's module

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Separate `go.mod` | CLI is a client, not part of the server; independent versioning |
| OpenAPI as source of truth | Ensures CLI matches API exactly; single spec drives both |
| Commands project operations, not schemas | Avoids commands for helper models and unsupported CRUD assumptions |
| Test the generated binary boundary | Command registration, flags, configuration, authentication, and HTTP behavior can regress even when generator internals compile |
