# TRex

**T**rusted **R**EST **Ex**ample

![TRex](rhtap-trex_sm.png)

A production-ready REST and gRPC API template for bootstrapping new Go microservices. Ships with "dinosaurs" as placeholder business logic — replace them with your own.

## Features

- REST + gRPC dual-protocol API with OpenAPI spec generation
- Plugin-based entity architecture with auto-registration
- Entity code generator (`go run ./scripts/generator.go --kind YourKind`)
- CLI code generator (`scripts/cli-generator/`) — generates a typed CLI from your OpenAPI spec
- PostgreSQL with GORM, migrations, and advisory locks
- Event-driven controllers via PostgreSQL LISTEN/NOTIFY
- JWT authentication with multi-issuer support (multiple JWK URLs + file)
- Server-streaming gRPC (watch for real-time events)
- Prometheus metrics, health checks, structured logging
- Spec-Driven Development with agent-executable skills

## Quick Start

```sh
# Prerequisites: Go 1.24+, Docker, buf (for protobuf)
# See PREREQUISITES.md for details

# Build
make proto
make binary

# Database
make db/setup
./trex migrate

# Run (no auth, for local dev)
make run-no-auth
```

REST API: `http://localhost:8000` | gRPC: `localhost:9000` | Metrics: `localhost:8080` | Health: `localhost:8083`

## Usage

```sh
# REST
curl http://localhost:8000/api/rh-trex/v1/dinosaurs | jq
curl -X POST http://localhost:8000/api/rh-trex/v1/dinosaurs \
  -H "Content-Type: application/json" -d '{"species": "Velociraptor"}' | jq

# gRPC
grpcurl -plaintext localhost:9000 rh_trex.v1.DinosaurService/ListDinosaurs
grpcurl -plaintext localhost:9000 rh_trex.v1.DinosaurService/WatchDinosaurs
```

## Generate a New Entity

```sh
# Basic
go run ./scripts/generator.go --kind Rocket

# With typed fields (nullable by default, :required for non-nullable)
go run ./scripts/generator.go --kind Rocket \
  --fields "name:string:required,fuel_type:string,max_speed:int,active:bool"
```

Generates the full stack: model, DAO, service, handlers, migration, OpenAPI spec, tests, and plugin registration. No manual wiring.

Supported types: `string`, `int`, `int64`, `bool`, `float`, `time`

After generation:

```sh
make binary
make db/teardown && make db/setup
./trex migrate
make run-no-auth
```

## Generate a CLI

Generate a typed CLI from the OpenAPI spec:

```sh
cd scripts/cli-generator
go run . --spec ../../openapi/openapi.yaml --out /tmp/trex-cli
cd /tmp/trex-cli
go build -o trex-cli ./cmd/trex-cli
```

Use the generated CLI:

```sh
# Login
trex-cli login --token-file /dev/stdin --url http://localhost:8000 <<< "$OIDC_TOKEN"

# CRUD
trex-cli list dinosaurs
trex-cli create dinosaur --species Velociraptor
trex-cli get dinosaur <id>
```

## Run With Authentication

```sh
make run

# Using the generated CLI
trex-cli login --token-file /dev/stdin --url http://localhost:8000 <<< "$OIDC_TOKEN"
trex-cli list dinosaurs

# Or with curl
curl -H "Authorization: Bearer $OIDC_TOKEN" http://localhost:8000/api/rh-trex/v1/dinosaurs
```

Supports any OIDC provider. Default: Red Hat SSO. Configure via `--jwk-cert-url`.

## Testing

```sh
make test              # Unit tests
make test-integration  # Integration tests (requires running PostgreSQL)
make verify            # Vet + formatting checks
make lint              # golangci-lint
```

## Architecture

```
cmd/trex/              CLI entrypoint (serve, migrate)
pkg/api/               Models and OpenAPI client
pkg/handlers/          REST handlers
pkg/services/          Business logic + event handlers
pkg/dao/               Data access layer (GORM)
pkg/db/migrations/     Schema migrations
pkg/auth/              JWT authentication
pkg/server/            gRPC server, routing, event broker
plugins/               Self-registering entity plugins
proto/                 Protobuf definitions
scripts/generator.go   Entity code generator
scripts/cli-generator/ CLI code generator
specs/                 Formal requirements (SDD)
skills/                Agent-executable procedures
```

## Deploy to OpenShift

```sh
make crc/login
make image
make push
make deploy
```

## Further Reading

- [PREREQUISITES.md](./PREREQUISITES.md) — Development environment setup
- [GRPC.md](./GRPC.md) — gRPC API details
- [CLAUDE.md](./CLAUDE.md) — Agent/developer reference (code generation, CLI flags, architecture details)
