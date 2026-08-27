# Entity Generator Specification

**Date:** 2026-08-03
**Status:** Active
**ID:** CG-001
**Related:** [Plugin Architecture](../framework/plugin-architecture.spec.md), [Entity Lifecycle](../framework/entity-lifecycle.spec.md), [Migration Pattern](../data/migration-pattern.spec.md)
**Implements:** `scripts/generator.go`, `templates/generate-*.txt`
**Skill:** `skills/build/entity-generator/`

---

## Purpose

Define the entity generator that produces complete CRUD functionality for a new Kind with zero manual steps required.

## Requirements

### Requirement: Single Command Generation

Running `go run ./scripts/generator.go --kind KindName` SHALL produce a complete, working entity implementation.

#### Scenario: Generate a new entity
- GIVEN the command `go run ./scripts/generator.go --kind FizzBuzz`
- WHEN the generator completes
- THEN all files SHALL be created in `plugins/fizzBuzzs/`
- AND `openapi/openapi.fizzBuzzs.yaml` SHALL be created
- AND `proto/rh_trex/v1/fizz_buzzs.proto` SHALL be created
- AND `cmd/trex/main.go` SHALL have the blank import added
- AND `openapi/openapi.yaml` SHALL reference the new endpoint and schema
- AND `make proto` SHALL be executed automatically
- AND `make generate` SHALL be executed automatically

### Requirement: Generated File Set

The generator SHALL produce exactly 16 files per entity.

#### Scenario: File manifest
- GIVEN a kind "Widget"
- THEN the following files SHALL be generated:
  1. `plugins/widgets/model.go` — API model struct
  2. `plugins/widgets/presenter.go` — Presenter conversion
  3. `plugins/widgets/dao.go` — Data access object
  4. `plugins/widgets/mock_dao.go` — Mock DAO for testing
  5. `plugins/widgets/service.go` — Business logic with event handlers
  6. `plugins/widgets/handler.go` — HTTP REST handlers
  7. `plugins/widgets/grpc_handler.go` — gRPC service implementation
  8. `plugins/widgets/grpc_presenter.go` — gRPC protobuf conversion
  9. `plugins/widgets/migration.go` — Database migration
  10. `plugins/widgets/plugin.go` — Auto-registration init()
  11. `plugins/widgets/integration_test.go` — REST integration tests
  12. `plugins/widgets/grpc_integration_test.go` — gRPC integration tests
  13. `plugins/widgets/factory_test.go` — Test data factories
  14. `plugins/widgets/testmain_test.go` — Test setup/teardown
  15. `openapi/openapi.widgets.yaml` — OpenAPI specification
  16. `proto/rh_trex/v1/widgets.proto` — Protobuf definition

### Requirement: Custom Fields Support

The `--fields` flag SHALL support typed fields with optional nullability modifiers.

#### Scenario: Field type mapping
- GIVEN `--fields "name:string:required,count:int,active:bool,ratio:float,created:time"`
- THEN the following Go types SHALL be generated:
  - `Name string` (non-pointer, required)
  - `Count *int` (pointer, nullable)
  - `Active *bool` (pointer, nullable)
  - `Ratio *float64` (pointer, nullable)
  - `Created *time.Time` (pointer, nullable)

### Requirement: Naming Convention Transformation

The generator SHALL consistently transform kind names across naming conventions.

#### Scenario: Multi-word kind name
- GIVEN `--kind FizzBuzz`
- THEN:
  - `Kind` = "FizzBuzz" (PascalCase)
  - `KindPlural` = "FizzBuzzs" (PascalCase plural)
  - `KindLowerSingular` = "fizzBuzz" (camelCase)
  - `KindLowerPlural` = "fizzBuzzs" (camelCase plural)
  - `KindSnakeCasePlural` = "fizz_buzzs" (snake_case plural, for API paths)

### Requirement: Irregular Plural Support

The generator SHALL handle irregular plurals for common English word endings.

#### Scenario: Irregular plural
- GIVEN `--kind Policy`
- THEN `KindPlural` SHALL be "Policies" (not "Policys")
- AND the custom `--plural` flag SHALL override automatic pluralization

### Requirement: OpenAPI Auto-Wiring

The generator SHALL modify `openapi/openapi.yaml` to add `$ref` links for the new entity's paths and schemas.

#### Scenario: OpenAPI reference injection
- GIVEN the generator creates `openapi/openapi.widgets.yaml`
- WHEN the main `openapi/openapi.yaml` is modified
- THEN path references SHALL be added after `# AUTO-ADD NEW PATHS`
- AND schema references SHALL be added after `# AUTO-ADD NEW SCHEMAS`

### Requirement: Generated Operation Identity

The entity generator SHALL emit deterministic, globally unique `operationId` values for every generated CRUD operation.

#### Scenario: Operation IDs for a generated entity
- GIVEN the generator creates the Widget OpenAPI document
- THEN its list, create, get, update, and delete operations SHALL declare `listWidgets`, `createWidget`, `getWidget`, `updateWidget`, and `deleteWidget` respectively
- AND rerunning generation for the same kind SHALL produce the same operation IDs

#### Scenario: Complete generated CRUD path item
- GIVEN the generator creates the Widget OpenAPI document
- THEN `/api/{project}/v1/widgets` SHALL document `GET` and `POST`
- AND `/api/{project}/v1/widgets/{id}` SHALL document `GET`, `PATCH`, and `DELETE`
- AND the DELETE response SHALL declare `204 No Content`

### Requirement: Post-Generation Code Formatting

All generated `.go` files SHALL be formatted with `gofmt -w`.

#### Scenario: Code formatting
- GIVEN a generated Go file
- WHEN generation completes for that file
- THEN `gofmt -w {file}` SHALL be executed
- AND a warning SHALL be logged (not a fatal error) if formatting fails

### Requirement: Library Mode Support

The generator SHALL support the `--library` flag for projects importing rh-trex-ai as a framework.

#### Scenario: Downstream project generation
- GIVEN `--library github.com/openshift-online/rh-trex-ai --repo github.com/myorg --project my-service`
- WHEN the generator runs
- THEN framework imports SHALL use the `--library` module path
- AND entity-specific code SHALL use `--repo/--project`

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Template-based generation | Go `text/template` provides logic, loops, and conditionals; templates in `templates/` are easy to modify |
| Auto-run `make proto` and `make generate` | Zero manual steps; generator output is immediately buildable |
| SHA256-based migration ID hash | Deterministic per kind name; prevents collision when generating multiple kinds simultaneously |
| Plugin directory per entity | Self-contained; easy to add, remove, or transplant entities |
| `gofmt` per file, not batch | Fail-safe; one formatting error doesn't prevent other files from being written |
