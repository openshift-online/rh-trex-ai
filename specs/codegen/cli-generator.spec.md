# CLI Generator Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** CG-002
**Related:** [REST Conventions](../api/rest-conventions.spec.md)
**Implements:** `scripts/cli-generator/`

---

## Purpose

Define the CLI tool generator that scaffolds complete command-line interfaces from OpenAPI specifications.

## Requirements

### Requirement: OpenAPI-Driven Generation

The CLI generator SHALL read the project's OpenAPI specification to discover entities and their CRUD operations.

#### Scenario: CLI generation from spec
- GIVEN a project with `openapi/openapi.yaml` defining Dinosaur, Fossil, and Scientist entities
- WHEN the CLI generator runs
- THEN commands SHALL be generated for each entity: `list`, `get`, `create`, `update`, `delete`

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
