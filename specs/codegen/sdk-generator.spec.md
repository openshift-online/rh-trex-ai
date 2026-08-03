# SDK Generator Specification

**Date:** 2026-08-03
**Status:** Active
**ID:** CG-003
**Related:** [REST Conventions](../api/rest-conventions.spec.md), [Testing Standards](../standards/testing.spec.md), [OpenAPI Intermediate Representation](openapi-ir.spec.md)
**Implements:** `scripts/sdk-generator/`

---

## Purpose

Define the SDK generator that produces client libraries in Go, Python, and TypeScript from OpenAPI specifications.

## Requirements

### Requirement: Multi-Language Output

The SDK generator SHALL produce client SDKs in Go, Python, and TypeScript.

#### Scenario: SDK generation
- GIVEN a project's OpenAPI specification
- WHEN the SDK generator runs
- THEN client libraries SHALL be generated for all three languages
- AND each SDK SHALL provide typed methods for all documented operations

### Requirement: Shared IR Consumption

The SDK generator SHALL consume the shared normalized OpenAPI IR and SHALL NOT maintain an independent raw OpenAPI parser or infer client methods from schema names.

#### Scenario: One interpretation in every language
- GIVEN one schema is exposed through global and parent-scoped collection operations
- WHEN all SDKs are generated
- THEN Go, Python, and TypeScript SHALL expose both operations from the same IR operation nodes

### Requirement: Operation and Path Fidelity

Every generated SDK method SHALL use the operation's exact HTTP method and path and SHALL accept all documented path, query, header, cookie, and body inputs with their requiredness and serialization rules preserved. CRUD, action, scoped, and streaming operations SHALL remain distinct methods.

#### Scenario: Nested action
- GIVEN `interruptAgent` is `POST /organizations/{organization_id}/agents/{agent_id}:interrupt`
- WHEN an SDK is generated
- THEN its typed method SHALL accept both path parameters
- AND it SHALL issue POST to the exact documented route
- AND the absence of a delete operation SHALL NOT produce a delete method

### Requirement: Generated SDK Acceptance Tests

The SDK generator SHALL have acceptance tests for Go, Python, and TypeScript that generate each SDK into an isolated temporary directory and compile, import, or type-check it with the target language's pinned toolchain. Behavioral tests SHALL invoke representative generated methods against an in-process mock HTTP server and verify method names, typed inputs and outputs, exact HTTP methods and paths, parameter and body serialization, response decoding, and authentication behavior. Covered legacy cases SHALL use the same assertions before and after migration to the canonical IR.

#### Scenario: Cross-language request fidelity
- GIVEN one fixture operation has scoped path parameters, a query parameter, a JSON request body, a typed response, and inherited authentication
- WHEN the corresponding generated method is exercised in each SDK
- THEN Go, Python, and TypeScript SHALL send semantically equivalent requests to the mock server
- AND each SDK SHALL decode the response into its documented target-language type

### Requirement: OpenAPI Specification Compliance

Generated SDKs SHALL match the API's OpenAPI specification exactly.

#### Scenario: Model fidelity
- GIVEN an entity with required field `name:string` and optional field `count:*int`
- WHEN the SDK is generated
- THEN the Go SDK SHALL use `string` and `*int` types
- AND the Python SDK SHALL use `str` and `Optional[int]` types
- AND the TypeScript SDK SHALL use `string` and `number | undefined` types

### Requirement: Standalone Module

Each language SDK SHALL be independently packaged and publishable.

#### Scenario: Independent package builds
- GIVEN generated Go, Python, and TypeScript SDK directories
- WHEN each target's standard package build is run
- THEN it SHALL succeed without importing API server source code
- AND each package SHALL contain the metadata required for independent publication

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Three languages | Covers primary Red Hat ecosystem: Go (operators), Python (data science), TypeScript (console) |
| Separate generator module | Independent of the server; can be versioned and released separately |
| Methods project operations, not model names | Helper schemas remain usable types without becoming fictional API clients |
| Compile and exercise every language | Template rendering success alone cannot prove that generated packages are valid or behaviorally equivalent |
