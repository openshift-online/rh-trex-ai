---
name: openapi-generate
description: >
  Regenerate the OpenAPI Go client from YAML specifications.
  Trigger phrases: "regenerate openapi", "make generate", "update client",
  "openapi client", "regenerate models"
---

# OpenAPI Generate

Regenerate the Go client code from OpenAPI YAML specifications.

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Prerequisites**: Docker or Podman must be running (the generator runs in a container).

2. **Run the generator**:
   ```bash
   make generate
   ```
   This takes 2-3 minutes and generates:
   - `pkg/api/openapi/model_*.go` — Go model structs
   - `pkg/api/openapi/api_default.go` — API client methods
   - `pkg/api/openapi/docs/*.md` — Generated documentation

3. **Verify**:
   ```bash
   make binary
   ```

4. **Troubleshooting**:
   - If Docker is not running: `systemctl start docker` or `podman machine start`
   - If models are missing: Check the `openapi/openapi.yaml` `$ref` links
   - If compilation fails: Check for naming conflicts in the OpenAPI schemas

## Related Specs
- `specs/api/rest-conventions.spec.md` (API-001)
