---
name: new-project
description: >
  Create a new project that imports TRex as a framework library.
  Trigger phrases: "new project", "create project", "scaffold project",
  "bootstrap service", "new microservice"
---

# New Project

Create a new Go microservice project that imports rh-trex-ai as a framework library.

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Parse arguments** to extract:
   - Project name (e.g., `my-service`)
   - Repository path (e.g., `github.com/myorg`)
   - Target directory (optional)

2. **Copy template**:
   ```bash
   cp -r templates/new-project/ {target_directory}/{project_name}
   ```

3. **Replace placeholders** in all files:
   - `my-service` → `{project_name}`
   - `github.com/example` → `{repo_path}`
   - Rename `cmd/my-service/` → `cmd/{project_name}/`

4. **Initialize Go module**:
   ```bash
   cd {target_directory}/{project_name}
   go mod init {repo_path}/{project_name}
   go mod tidy
   ```

5. **Verify the scaffold**:
   ```bash
   make binary
   ```

6. **Guide next steps**:
   - Generate entities with `go run ./scripts/generator.go --kind {Kind} --library github.com/openshift-online/rh-trex-ai`
   - Set up database with `make db/setup`
   - Run with `make run`

## Related Specs
- `specs/codegen/entity-generator.spec.md` (CG-001) — for adding entities to the new project
