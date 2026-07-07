# SDK Generator Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** CG-003
**Related:** [REST Conventions](../api/rest-conventions.spec.md)
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
- AND each SDK SHALL provide typed methods for all CRUD operations

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

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Three languages | Covers primary Red Hat ecosystem: Go (operators), Python (data science), TypeScript (console) |
| Separate generator module | Independent of the server; can be versioned and released separately |
