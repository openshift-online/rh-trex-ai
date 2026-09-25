# gRPC Conventions Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** API-002
**Related:** [Event-Driven Controllers](../framework/event-driven-controllers.spec.md), [Authentication](../security/authentication.spec.md)
**Implements:** `pkg/server/grpc_server.go`, `pkg/server/grpc_registry.go`, `pkg/server/grpc_tls.go`, `pkg/config/grpc.go`, `proto/rh_trex/v1/`, `plugins/*/grpc_handler.go`, `components/control-plane/internal/grpcclient/`

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

### Requirement: gRPC Transport Security

The gRPC server SHALL serve TLS when `--grpc-enable-tls=true`, using `--grpc-tls-cert-file` and `--grpc-tls-key-file`, independently of the shared `--enable-tls` flag. When the shared TLS configuration (`--enable-tls`) is enabled it SHALL take precedence for the gRPC listener as well, so deployments that already use one shared certificate keep working. The gRPC-only TLS configuration SHALL require TLS 1.2 or newer (or the shared `--tls-min-version` when that is higher), SHALL offer only `h2` over ALPN, and SHALL fail server startup when either file is unset, missing, or unparsable. It SHALL serve a renewed key pair on new connections without a restart when either file changes on disk. A renewal that cannot be loaded SHALL NOT take the listener down; the previous key pair SHALL keep being served and the failure SHALL be logged. The control-plane watch client SHALL dial with TLS when `TREX_GRPC_TLS=true` and in plaintext otherwise.

#### Scenario: gRPC-only TLS
- GIVEN `--enable-tls` is off
- AND `--grpc-enable-tls=true` with `--grpc-tls-cert-file` and `--grpc-tls-key-file` pointing at a valid key pair
- WHEN a client that trusts the certificate calls `grpc.health.v1.Health/Check`
- THEN the TLS handshake SHALL negotiate TLS 1.2 or newer with ALPN protocol `h2`
- AND the call SHALL succeed
- AND a plaintext client calling the same service SHALL fail

#### Scenario: Shared TLS precedence
- GIVEN `--enable-tls` is on with its own `--tls-cert-file` and `--tls-key-file`
- AND `--grpc-enable-tls=true` with a different key pair
- WHEN a client connects to the gRPC listener
- THEN the server SHALL present the shared certificate

#### Scenario: Missing files fail fast
- GIVEN `--grpc-enable-tls=true` and `--enable-tls` off
- WHEN `--grpc-tls-cert-file` or `--grpc-tls-key-file` is unset, or names a missing or unparsable file
- THEN `NewDefaultGRPCServer` SHALL report an error naming the offending flag or file
- AND the process SHALL exit non-zero before listening

#### Scenario: Renewal without restart
- GIVEN a gRPC server serving gRPC-only TLS
- WHEN the certificate and key files are replaced with a new key pair
- THEN the next new connection SHALL be presented the new certificate
- AND the server SHALL NOT be restarted

#### Scenario: Unreadable renewal keeps serving
- GIVEN a gRPC server serving gRPC-only TLS
- WHEN the certificate file is replaced with content that cannot be parsed
- THEN new connections SHALL still be presented the previous certificate
- AND the reload failure SHALL be logged

#### Scenario: Control-plane TLS client
- GIVEN the control plane starts with `TREX_GRPC_TLS=true`
- WHEN it dials `TREX_GRPC_SERVER_ADDR`
- THEN it SHALL use TLS 1.2 or newer and verify the server certificate against the system roots, or against `TREX_GRPC_TLS_CA_FILE` when set
- AND `TREX_GRPC_TLS_SERVER_NAME`, when set, SHALL override the name used for SNI and verification
- AND an unparsable `TREX_GRPC_TLS` value or an unreadable CA file SHALL be a startup error
- AND without `TREX_GRPC_TLS=true` the dial SHALL stay plaintext

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
| gRPC TLS independent of shared `--enable-tls` | REST is commonly edge-terminated by a Route or Ingress while gRPC needs end-to-end TLS (passthrough); enabling shared TLS would also switch REST to HTTPS |
| Shared TLS wins when both are enabled | Keeps existing single-certificate deployments working unchanged |
| Reload gRPC key pair on file change, keep old pair on failure | cert-manager and service-CA rotate files in place; a half-written renewal must not take the listener down |
