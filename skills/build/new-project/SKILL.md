---
name: new-project
description: >
  Create a new project that imports TRex as a framework library.
  Trigger phrases: "new project", "create project", "scaffold project",
  "bootstrap service", "new microservice"
---

# New Project

Create a new Go microservice project that imports rh-trex-ai as a framework library.

The `trex new-project` command scaffolds a complete application from the embedded
template (`components/api-server/templates/new-project/`) and substitutes all
placeholders automatically. The scaffolded project's binary inherits all TRex
commands (`serve`, `migrate`, `tui`, `new-entity`, `new-project`).

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Parse arguments** to extract:
   - Project name (e.g., `my-service`)
   - Repository/module base (e.g., `github.com/myorg`)
   - Target parent directory (optional)

2. **Run the scaffolder**:
   ```bash
   cd <path-to-trex>/components/api-server
   go run ./cmd/trex new-project {project_name} --repo {repo_path} --dir {target_directory}
   ```

3. **Point the replace directive at a local TRex checkout** (for development):
   ```bash
   cd {target_directory}/{project_name}
   sed -i '' 's#../path/to/rh-trex-ai/components/api-server#/absolute/path/to/rh-trex-ai/components/api-server#' go.mod
   ```

4. **Generate the OpenAPI client first** (required before tidy/build, and after every entity change):
   ```bash
   make generate
   ```

5. **Tidy and build**:
   ```bash
   go mod tidy
   make binary
   ```

6. **Generate your first entity** (note `--module` must be the new project's own module path):
   ```bash
   ./{project_name} new-entity \
     --kind {Kind} \
     --fields "{fields}" \
     --project {project_name} \
     --repo {repo_path} \
     --library github.com/openshift-online/rh-trex-ai/components/api-server \
     --module {repo_path}/{project_name} \
     --proto-package {project_name_snake}
   ```

7. **Regenerate proto + OpenAPI client, then build**:
   ```bash
   make proto
   make generate
   go build ./...
   ```

8. **Set up database and run**:
   ```bash
   make db/setup
   ./{project_name} migrate
   make run-no-auth
   ```

## Related Specs
- `specs/codegen/entity-generator.spec.md` (CG-001) — for adding entities to the new project
