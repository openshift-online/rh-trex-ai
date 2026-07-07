---
name: add-field
description: >
  Add a new field to an existing entity across all layers.
  Trigger phrases: "add field", "new column", "add property", "extend entity",
  "add attribute to"
---

# Add Field to Entity

Add a new field to an existing entity across all 5 layers: model, migration, presenter, handler, and OpenAPI.

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Parse arguments** to extract:
   - Entity name (e.g., `Dinosaur`)
   - Field name (e.g., `habitat`)
   - Field type (e.g., `string`, `int`, `bool`, `float`, `time`)
   - Nullability (required or optional, default: optional)

2. **Update API Model** (`plugins/{kindLowerPlural}/model.go`):
   - Add the field to the entity struct with correct Go type and JSON tag
   - Add the field to the `PatchRequest` struct (always as pointer type)

3. **Create new migration** (`plugins/{kindLowerPlural}/migration_{timestamp}.go`):
   - NEVER modify the existing migration file
   - Create a new migration with the extended struct
   - Register via `db.RegisterMigration()` in `init()`

4. **Update presenter** (`plugins/{kindLowerPlural}/presenter.go`):
   - Add field mapping in both directions (model ↔ OpenAPI)
   - Handle nil for nullable fields

5. **Update handler** (`plugins/{kindLowerPlural}/handler.go`):
   - Add field to the Patch method's update logic
   - Add validation if the field is required

6. **Update OpenAPI spec** (`openapi/openapi.{kindLowerPlural}.yaml`):
   - Add the field to the entity schema
   - Add to PatchRequest schema
   - Add to `required` array if non-nullable
   - Run `make generate` to regenerate the Go client

7. **Verify**:
   ```bash
   make binary
   make db/teardown && make db/setup
   make test-integration
   ```

## Related Specs
- `specs/framework/entity-lifecycle.spec.md` (FW-002)
- `specs/data/migration-pattern.spec.md` (DA-002)
