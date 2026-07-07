---
name: entity-generator
description: >
  Generate a new entity (Kind) with complete CRUD functionality.
  Trigger phrases: "generate entity", "new kind", "create entity", "add kind",
  "scaffold entity", "generate crud", "new resource type"
---

# Entity Generator

Generate a complete entity implementation with REST, gRPC, database, and tests.

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Parse arguments** to extract:
   - `--kind` (required): PascalCase entity name (e.g., `Rocket`)
   - `--fields` (optional): Comma-separated field definitions (e.g., `name:string:required,speed:int`)
   - `--plural` (optional): Custom plural form for irregular nouns

2. **Validate naming** against specs:
   - Read `specs/standards/naming-conventions.spec.md` for naming rules
   - Verify the kind name is PascalCase
   - Check for reserved names (Event, Meta, Model, Error)

3. **Run the generator**:
   ```bash
   go run ./scripts/generator.go --kind {Kind} [--fields "{fields}"] [--plural "{plural}"]
   ```
   This automatically:
   - Creates 16 files in `plugins/{kindLowerPlural}/`
   - Creates OpenAPI spec in `openapi/`
   - Creates proto file in `proto/rh_trex/v1/`
   - Adds blank import to `cmd/trex/main.go`
   - Wires OpenAPI references in `openapi/openapi.yaml`
   - Runs `make proto` for protobuf code generation
   - Runs `make generate` for OpenAPI client regeneration

4. **Verify generation**:
   ```bash
   make binary
   ```

5. **Run tests** to confirm:
   ```bash
   make test-integration
   ```

## Field Type Reference

| Type | Go Type (nullable) | Go Type (required) | OpenAPI Type |
|------|-------------------|-------------------|-------------|
| string | *string | string | string |
| int | *int | int | integer (int32) |
| int64 | *int64 | int64 | integer (int64) |
| bool | *bool | bool | boolean |
| float | *float64 | float64 | number (double) |
| time | *time.Time | time.Time | string (date-time) |

## Related Specs
- `specs/codegen/entity-generator.spec.md` (CG-001)
- `specs/framework/plugin-architecture.spec.md` (FW-001)
- `specs/framework/entity-lifecycle.spec.md` (FW-002)

## Troubleshooting

- **Build fails**: Check that `make proto` completed (buf must be installed)
- **Import cycle**: Verify plugin uses `db.Model` in migration, not the API model
- **Tests fail with DB error**: Run `make db/teardown && make db/setup`
