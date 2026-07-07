# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TRex (**T**rusted **R**EST **Ex**ample) is a Go-based REST and gRPC API template that serves as a full-featured foundation for building new microservices. It provides CRUD operations for "dinosaurs" as example business logic to be replaced. See [README.md](./README.md) for quick start and usage.

## Agent Rules

### Always Define Specs

Before implementing any new feature or changing behavior, create or update the relevant spec in `specs/`. Specs define the desired state using RFC 2119 keywords (SHALL, MUST, SHOULD, MAY) with GIVEN/WHEN/THEN scenarios. Use the `/spec` skill (`skills/plan/spec/SKILL.md`) to author specs. Run `/reconcile` after spec changes to identify gaps.

### Always Implement with Skills

Use the skills in `skills/` for all standard operations. Never improvise manual steps when a skill exists. Skills are the canonical procedures — follow them exactly.

| Operation | Skill |
|-----------|-------|
| Generate entity | `skills/build/entity-generator/` |
| Add field to entity | `skills/build/add-field/` |
| Remove entity | `skills/build/remove-entity/` |
| Reconcile specs to code | `skills/build/reconcile/` |
| Run unit tests | `skills/test/unit-test/` |
| Run integration tests | `skills/test/integration-test/` |
| Set up database | `skills/test/db-setup/` |
| Start server | `skills/deploy/server-start/` |
| Create release tag | `skills/deploy/release-tag/` |
| Code review | `skills/review/code-review/` |
| Verify (vet/fmt) | `skills/review/verify/` |
| Author a spec | `skills/plan/spec/` |
| Regenerate OpenAPI client | `skills/tooling/openapi-generate/` |

## Spec-Driven Development (SDD)

Specs (`specs/`) define desired state. Skills (`skills/`) define procedures. `skills/RECONCILE.md` tracks gaps across sessions.

**Spec registry:** `specs/index.spec.md` — all 18 specs with dependency graph across 6 domains (framework, api, data, security, codegen, standards).

**SDLC workflow:**
```
/reconcile → /spec → /entity-generator → /db-setup → /unit-test → /integration-test → /verify → /code-review
```

## Development Commands

### Build & Run
- `make proto` — generate protobuf stubs (required before `make binary`)
- `make binary` — build the trex binary
- `make install` — build and install to GOPATH/bin
- `make run` — run with auth enabled
- `make run-no-auth` — run with auth disabled (local dev)

### Test & Quality
- `make test` — unit tests
- `make test-integration` — integration tests
- `make verify` — vet + formatting
- `make lint` — golangci-lint

### Database
- `make db/setup` — start PostgreSQL container
- `make db/teardown` — stop and remove PostgreSQL container
- `make db/login` — connect to local PostgreSQL
- `./trex migrate` — run database migrations

### Protobuf
- `make proto` — generate Go stubs from `.proto` files
- `make proto-lint` — lint proto files
- `make proto-breaking` — check breaking changes against main

### OpenAPI & Code Generation
- `make generate` — regenerate OpenAPI client and models
- `make clean` — remove temporary generated files

### OpenShift
- `make crc/login` — login to CRC
- `make image` / `make push` / `make deploy` / `make undeploy`

## CLI Flags Reference

### `trex serve`

**Server:** `--api-server-bindaddress` (default: `localhost:8000`), `--api-server-hostname`, `--enable-https`, `--https-cert-file`, `--https-key-file`

**Database:** `--db-host-file`, `--db-name-file`, `--db-user-file`, `--db-password-file`, `--db-port-file` (all default to `secrets/db.*`), `--db-sslmode` (default: `disable`), `--db-max-open-connections` (default: 50), `--enable-db-debug`

**Auth:** `--enable-jwt` (default: true), `--enable-authz` (default: true), `--jwk-cert-url` (default: Red Hat SSO), `--jwk-cert-file`, `--acl-file`

**gRPC:** `--enable-grpc` (default: true), `--grpc-server-bindaddress` (default: `localhost:9000`), `--grpc-enable-tls`, `--grpc-tls-cert-file`, `--grpc-tls-key-file`

**Monitoring:** `--health-check-server-bindaddress` (default: `localhost:8083`), `--metrics-server-bindaddress` (default: `localhost:8080`), `--enable-sentry`

**Performance:** `--http-read-timeout` (default: 5s), `--http-write-timeout` (default: 30s)

### `trex migrate`

Same database flags as `trex serve`. Idempotent — safe to run multiple times.

## Architecture

**Layers:** Handler → Service → DAO → Model

**Key packages:**
- `pkg/api/` — models, OpenAPI client, presenters
- `pkg/handlers/` — REST endpoints
- `pkg/services/` — business logic + event handlers
- `pkg/dao/` — GORM data access
- `pkg/db/migrations/` — schema versioning
- `pkg/auth/` — JWT with multi-issuer JWK loading
- `pkg/server/` — gRPC server, routing, event broker, interceptors
- `pkg/environments/` — dev/test/prod environment framework
- `pkg/registry/` — service auto-discovery
- `plugins/` — self-registering entity plugins (init()-based)

**Event flow:** API operation → Event creation → PostgreSQL NOTIFY → Controller listener → Idempotent handler

## Code Generation

### Entity Generator

```bash
go run ./scripts/generator.go --kind FizzBuzz
go run ./scripts/generator.go --kind FizzBuzz --fields "name:string:required,count:int"
```

Creates: model, DAO, service, handlers, presenter, migration, OpenAPI spec, tests, factory, plugin. Updates: `main.go` imports, `migration_structs.go`, `openapi.yaml`. Runs `make generate`.

**Field types:** `string`, `int`, `int64`, `bool`, `float`, `time`
**Nullability:** default nullable (pointer), `:required` for non-nullable, `:optional` explicit nullable

**Template fields:** `{{.Kind}}`, `{{.KindPlural}}`, `{{.KindLowerSingular}}`, `{{.KindLowerPlural}}`, `{{.KindSnakeCasePlural}}`, `{{.Project}}`, `{{.ProjectPascalCase}}`, `{{.Repo}}`, `{{.Cmd}}`, `{{.ID}}`

### CLI Generator

```bash
cd scripts/cli-generator
go run . --spec ../../openapi/openapi.yaml --out /tmp/trex-cli
```

Generates a complete Cobra CLI with `login`, `list`, `get`, `create` commands from the OpenAPI spec.

### Post-Generation Workflow

```bash
make binary
make db/teardown && make db/setup
./trex migrate
make run-no-auth
```

### Cleanup (removing a generated Kind)

```bash
rm -rf pkg/api/{kind}.go pkg/api/presenters/{kind}.go pkg/handlers/{kind}.go \
  pkg/services/{kind}.go pkg/dao/{kind}.go pkg/dao/mocks/{kind}.go \
  pkg/db/migrations/*{kind}* test/integration/{kinds}_test.go \
  test/factories/{kinds}.go openapi/openapi.{kinds}.yaml plugins/{kinds}/
git checkout HEAD -- cmd/trex/main.go pkg/db/migrations/migration_structs.go openapi/openapi.yaml
make generate
```

## Authentication

JWT-based with configurable OIDC providers. Default: Red Hat SSO. Multi-issuer via `--jwk-cert-url` (repeatable) + `--jwk-cert-file`.

```bash
# Disable for local dev
--enable-jwt=false

# With token
curl -H "Authorization: Bearer $OIDC_TOKEN" http://localhost:8000/api/rh-trex/v1/dinosaurs

# With generated CLI
trex-cli login --token-file /dev/stdin --url http://localhost:8000 <<< "$OIDC_TOKEN"
trex-cli list dinosaurs
```

## Code Review Standards

Use `/trex.review` for standardized reviews. Pre-commit:

- `make verify` + `make lint` + `make test` + `make test-integration` must pass
- No secrets in logs or code
- Event handlers must be idempotent
- Follow Handler → Service → DAO → Model pattern
- Use TRex error types (`errors.BadRequest`, `errors.NotFound`, etc.)

Review guidance in `.claude/context/` and `.claude/patterns/`.
