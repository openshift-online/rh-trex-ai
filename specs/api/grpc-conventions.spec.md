# gRPC Conventions Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** API-002
**Related:** [Event-Driven Controllers](../framework/event-driven-controllers.spec.md), [Authentication](../security/authentication.spec.md)
**Implements:** `pkg/server/grpc_server.go`, `pkg/server/grpc_registry.go`, `proto/rh_trex/v1/`, `plugins/*/grpc_handler.go`

---

## Purpose

Define the gRPC server conventions including protobuf schema design, interceptor chain ordering, server-streaming patterns, and service registration.

## Requirements

### Requirement: Protobuf Schema Convention

Proto files SHALL reside in `proto/rh_trex/v1/` and use the `rh_trex.v1` package with `go_package` pointing to `pkg/api/grpc/rh_trex/v1`.

#### Scenario: Proto file for a new entity
- GIVEN a new entity "Widget"
- WHEN the proto file is generated
- THEN `proto/rh_trex/v1/widgets.proto` SHALL define:
  - `WidgetService` with RPCs: `GetWidget`, `CreateWidget`, `UpdateWidget`, `DeleteWidget`, `ListWidgets`, `WatchWidgets`
  - `Widget` message matching the API model fields
  - `WatchWidgets` SHALL be a server-streaming RPC returning `WatchWidgetsResponse`

### Requirement: Interceptor Chain Ordering

The gRPC server SHALL apply interceptors in this exact order for both unary and streaming RPCs.

#### Scenario: Interceptor execution order
- GIVEN a gRPC request arrives at the server
- THEN interceptors SHALL execute in this order:
  1. Recovery (panic recovery with stack trace logging)
  2. Logging (request/response logging with duration)
  3. Metrics (Prometheus request counting and latency)
  4. Transaction (GORM session injection — unary only)
  5. Pre-auth interceptors (registered via `RegisterPreAuthGRPCUnaryInterceptor`)
  6. JWT Authentication (token validation and claims extraction)
  7. Post-auth interceptors (registered via `RegisterPostAuthGRPCUnaryInterceptor`)

### Requirement: Pre-Auth and Post-Auth Extension Points

The gRPC server SHALL support registration of custom interceptors that run before and after JWT authentication.

#### Scenario: Bearer token pre-auth interceptor
- GIVEN `authConfig.EnableBearer` is true and a `BearerToken` is configured
- WHEN the gRPC server initializes
- THEN bearer token unary and stream interceptors SHALL be auto-registered as pre-auth interceptors

#### Scenario: Custom post-auth interceptor
- GIVEN a downstream project registers a post-auth unary interceptor via `RegisterPostAuthGRPCUnaryInterceptor`
- WHEN a gRPC request is processed
- THEN the custom interceptor SHALL execute after JWT authentication
- AND the authenticated username SHALL be available in the context

### Requirement: Server-Streaming Watch Pattern

Each entity SHALL support a `Watch{Kinds}` server-streaming RPC for real-time event notifications.

#### Scenario: gRPC watch stream
- GIVEN a client calls `WatchDinosaurs` RPC
- WHEN a dinosaur is created, updated, or deleted
- THEN the EventBroker SHALL publish the event
- AND the gRPC handler SHALL send a `WatchDinosaursResponse` to the client stream
- AND the response SHALL include `event_type`, `dinosaur` (the full entity), and metadata

### Requirement: Service Registration via Plugin

gRPC services SHALL be registered via `pkgserver.RegisterGRPCService()` in the plugin's `init()` function.

#### Scenario: gRPC service auto-discovery
- GIVEN three plugins register gRPC services
- WHEN `LoadDiscoveredGRPCServices(grpcServer, services)` is called during server startup
- THEN all three services SHALL be registered on the gRPC server

### Requirement: Health Check and Reflection

The gRPC server SHALL register the gRPC health check service and reflection service.

#### Scenario: Health check probe
- GIVEN a running gRPC server
- WHEN a health check probe calls `grpc.health.v1.Health/Check`
- THEN the server SHALL respond with `SERVING` status

### Requirement: Buf-Managed Code Generation

Proto stub generation SHALL use `buf` via `make proto` with configuration in `buf.yaml` and `buf.gen.yaml`.

#### Scenario: Proto code generation
- GIVEN a modified `.proto` file
- WHEN `make proto` is executed
- THEN Go stubs SHALL be generated into `pkg/api/grpc/rh_trex/v1/`
- AND `make proto-lint` SHALL validate proto style
- AND `make proto-breaking` SHALL check for breaking changes against the main branch

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Separate proto file per entity | Mirrors REST OpenAPI separation; independent evolution |
| Server-streaming for Watch (not bidirectional) | Simpler client implementation; server pushes events, client only subscribes |
| EventBroker bridge between NOTIFY and gRPC | Decouples PostgreSQL events from gRPC transport; enables filtering and buffering |
| Pre/post auth interceptor hooks | Enables downstream projects to inject custom auth without modifying framework |
| Reflection enabled by default | Enables grpcurl and gRPC GUI tools for development |
| Transaction interceptor for unary only | Streaming RPCs have different lifecycle; transactions don't span multiple messages |
