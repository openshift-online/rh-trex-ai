# Naming Conventions Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** STD-001
**Implements:** `scripts/generator.go` (naming transforms), all generated code

---

## Purpose

Define the naming conventions used consistently across Go code, API paths, JSON fields, database columns, and protobuf definitions.

## Requirements

### Requirement: Go Type Naming

Go struct types SHALL use PascalCase.

#### Scenario: Entity type names
- GIVEN a kind "FizzBuzz"
- THEN the Go types SHALL be: `FizzBuzz`, `FizzBuzzList`, `FizzBuzzPatchRequest`

### Requirement: Go Variable Naming

Go local variables and function parameters SHALL use camelCase.

#### Scenario: Variable names
- GIVEN a kind "FizzBuzz"
- THEN variables SHALL be named: `fizzBuzz`, `fizzBuzzs` (plural)

### Requirement: JSON and API Path Naming

JSON field names and API URL path segments SHALL use snake_case.

#### Scenario: JSON and URL naming
- GIVEN a Go field `MaxSpeed int`
- THEN the JSON tag SHALL be `json:"max_speed"`
- AND the API path segment SHALL be `max_speed`

#### Scenario: API URL paths
- GIVEN a kind "FizzBuzz"
- THEN the API path SHALL be `/api/rh-trex/v1/fizz_buzzs`

### Requirement: Database Column Naming

Database column names SHALL use snake_case, matching GORM's default convention.

#### Scenario: Column names
- GIVEN a Go field `FuelType string`
- THEN the database column SHALL be `fuel_type`

### Requirement: Plugin Directory Naming

Plugin directories SHALL use the camelCase plural form of the kind.

#### Scenario: Plugin directory
- GIVEN a kind "FizzBuzz"
- THEN the plugin directory SHALL be `plugins/fizzBuzzs/`

### Requirement: Proto File Naming

Protobuf files SHALL use the snake_case plural form of the kind.

#### Scenario: Proto file naming
- GIVEN a kind "FizzBuzz"
- THEN the proto file SHALL be `proto/rh_trex/v1/fizz_buzzs.proto`
- AND the proto service SHALL be `FizzBuzzService` (PascalCase)
- AND proto message fields SHALL use snake_case

### Requirement: OpenAPI File Naming

OpenAPI specification files SHALL use the camelCase plural form.

#### Scenario: OpenAPI file naming
- GIVEN a kind "FizzBuzz"
- THEN the OpenAPI file SHALL be `openapi/openapi.fizzBuzzs.yaml`

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| snake_case for API/JSON/DB | Industry standard for REST APIs; matches PostgreSQL defaults |
| PascalCase for Go types | Go convention for exported types |
| camelCase for Go variables | Go convention for unexported identifiers |
| camelCase plural for directories | Matches Go variable naming; avoids hyphens in import paths |
| Consistent transformation in generator | Single source of truth for naming rules |
