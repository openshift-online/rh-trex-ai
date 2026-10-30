# /trex.project.new

Creates a new project using TRex as an imported framework library.

The `trex new-project` command scaffolds a complete application from the embedded
template (`components/api-server/templates/new-project/`), substituting all
placeholders automatically.

## Input

Ask the user for:
- Project name (required), e.g. `user-service`
- Repository/module base (optional, defaults to `github.com/example`), e.g. `github.com/myorg`
- Target parent directory (optional, defaults to current directory)

## Steps

1. **Run the scaffolder** from the TRex repository:
   ```bash
   cd /path/to/rh-trex/components/api-server
   go run ./cmd/trex new-project user-service --repo github.com/myorg --dir <parent>
   ```
   The command creates `<parent>/user-service/`, renames directories
   (`cmd/my-service` → `cmd/user-service`, `proto/my_service` → `proto/user_service`),
   and substitutes all placeholders (`my-service`, `my_service`, `github.com/example`).

2. **Point the replace directive at a local TRex checkout** (for development):
   ```bash
   cd user-service
   sed -i '' 's#../path/to/rh-trex-ai/components/api-server#/absolute/path/to/rh-trex-ai/components/api-server#' go.mod
   ```

3. **Generate the OpenAPI client, tidy, and build**:
   ```bash
   make generate
   go mod tidy
   make binary
   ```

4. **Add the first entity** (the scaffolded binary inherits the commands):
   ```bash
   ./user-service new-entity --kind HelloWorld --fields "message:string:required" \
     --project user-service --repo github.com/myorg \
     --library github.com/openshift-online/rh-trex-ai/components/api-server \
     --module github.com/myorg/user-service --proto-package user_service
   make proto && make generate && go build ./...
   ```

## Template Structure

The scaffolded project includes:
- `go.mod` with TRex library dependencies (replace placeholder for local dev)
- `cmd/<name>/main.go` wired with `serve`, `migrate`, `tui`, `generate-entity`/`new-entity`, and `new-project`
- `pkg/api/` — OpenAPI spec loader
- `proto/<snake>/v1/common.proto` + buf config
- `Makefile` with build, proto, generate, db, and test targets
- `secrets/` — database and service configuration files
- `test/` — integration test helper
- `.claude/commands/` — Claude Code slash commands for common workflows
- `openapi/openapi.yaml` — base spec with `# AUTO-ADD NEW PATHS` / `# AUTO-ADD NEW SCHEMAS` anchors
