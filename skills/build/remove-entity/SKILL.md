---
name: remove-entity
description: >
  Completely remove a generated entity (Kind) and all its artifacts.
  Trigger phrases: "remove entity", "delete kind", "remove kind",
  "clean up entity", "undo generation"
---

# Remove Entity

Completely remove a generated Kind and all its artifacts from the codebase.

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Parse arguments** to extract the entity name (PascalCase, e.g., `FizzBuzz`)

2. **Derive naming variants**:
   - `kindLowerPlural` (e.g., `fizzBuzzs`)
   - `kindSnakeCasePlural` (e.g., `fizz_buzzs`)
   - `kindLowerSingular` (e.g., `fizzBuzz`)

3. **Remove generated files**:
   ```bash
   rm -rf plugins/{kindLowerPlural}/
   rm -f openapi/openapi.{kindLowerPlural}.yaml
   rm -f proto/rh_trex/v1/{kindSnakeCasePlural}.proto
   ```

4. **Remove OpenAPI client files**:
   ```bash
   rm -f pkg/api/openapi/model_{snake_case}*.go
   rm -f pkg/api/openapi/docs/{Kind}*.md
   ```

5. **Remove generated gRPC stubs**:
   ```bash
   rm -f pkg/api/grpc/rh_trex/v1/{kindSnakeCasePlural}*.go
   ```

6. **Unwire from main.go**:
   - Remove `_ "repo/project/plugins/{kindLowerPlural}"` import from `cmd/trex/main.go`

7. **Unwire from openapi.yaml**:
   - Remove `$ref` lines referencing `openapi.{kindLowerPlural}.yaml` from `openapi/openapi.yaml`

8. **Regenerate OpenAPI client**:
   ```bash
   make generate
   ```

9. **Verify clean removal**:
   ```bash
   rg -l "{Kind}" --type go  # Should find no references
   make binary               # Should build cleanly
   ```

## Related Specs
- `specs/codegen/entity-generator.spec.md` (CG-001)
- `specs/framework/plugin-architecture.spec.md` (FW-001)
