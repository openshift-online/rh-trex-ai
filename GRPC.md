# gRPC Integration Plan for rh-trex-ai

> **Implementation Status:** Phases 1-3 are fully implemented. Phase 5 (streaming) is partially implemented — WatchDinosaurs server-streaming with EventBroker is complete; BulkCreateDinosaurs (client-streaming) is not yet implemented. Phase 4 (generator integration) and Phase 6 (grpc-gateway) are not yet implemented.

## Overview

Add gRPC as a parallel transport layer alongside the existing REST API. The service layer is already protocol-agnostic (`context.Context` + domain types), so gRPC handlers will call the same services without modification. The plugin auto-registration pattern extends naturally to support gRPC service registration.

## Architecture

```
                    ┌─────────────────────────────────────────────┐
                    │               cmd/trex/main.go              │
                    │         (blank imports trigger init())       │
                    └──────────────────┬──────────────────────────┘
                                       │
                    ┌──────────────────▼──────────────────────────┐
                    │          runServe() with signal handling     │
                    │                                              │
                    │  go APIServer.Start()        :8000 (REST)    │
                    │  go GRPCServer.Start()        :9000 (gRPC)  │  ◄── NEW
                    │  go MetricsServer.Start()     :8080          │
                    │  go HealthCheckServer.Start()  :8083          │
                    │  go ControllersServer.Start()                │
                    │                                              │
                    │  ← SIGTERM/SIGINT triggers GracefulStop() →  │
                    └──────────────────┬──────────────────────────┘
                                       │
          ┌────────────────────────────┼────────────────────────────┐
          │                            │                            │
   ┌──────▼──────┐            ┌────────▼───────┐           ┌───────▼───────┐
   │  REST Layer │            │  gRPC Layer    │           │  Controllers  │
   │  (gorilla)  │            │  (grpc-go)     │           │  (events)     │
   │             │            │                │           │               │
   │  Handlers   │            │  gRPC Handlers │           │  OnUpsert     │
   │  JSON ↔ API │            │  Proto ↔ API   │           │  OnDelete     │
   └──────┬──────┘            └────────┬───────┘           └───────┬───────┘
          │                            │                            │
          └────────────────────────────┼────────────────────────────┘
                                       │
                            ┌──────────▼──────────┐
                            │   Service Layer     │
                            │   (unchanged)       │
                            │                     │
                            │  DinosaurService    │
                            │  Get / Create / ... │
                            └──────────┬──────────┘
                                       │
                            ┌──────────▼──────────┐
                            │   DAO / Database    │
                            │   (unchanged)       │
                            └─────────────────────┘
```

## What Changes vs What Stays the Same

### Unchanged
- Service layer (`plugins/*/service.go`) — called by both REST and gRPC handlers
- DAO layer (`pkg/dao/`, `plugins/*/dao.go`)
- Database, migrations, event system
- Environment / DI framework (`pkg/environments/`)
- REST API — no breaking changes
- Controller event system
- Plugin blank-import activation in `main.go`

### New
- Proto definitions per entity
- gRPC server infrastructure (`pkg/server/grpc_server.go`)
- gRPC service registration pattern (`pkg/server/grpc_registry.go`)
- gRPC interceptors for auth, logging, metrics, transactions
- gRPC config (`pkg/config/grpc.go`)
- Per-plugin gRPC handler files (`plugins/*/grpc_handler.go`)
- Build targets for proto compilation via `buf`
- Signal-based graceful shutdown for all servers

## Implementation Phases

---

### Phase 1: Infrastructure — Proto Tooling & Build System **[IMPLEMENTED]**

**Goal:** Establish the proto compilation pipeline using `buf` so `.proto` files produce Go stubs.

#### 1.1 Toolchain: buf (not raw protoc)

We use [`buf`](https://buf.build) for proto management. It handles linting, breaking change detection, dependency resolution, and code generation in a single tool — replacing ad-hoc `protoc` invocations.

Required tools:
```
buf                       (proto management CLI — replaces protoc)
protoc-gen-go             (Go message stubs)
protoc-gen-go-grpc        (Go gRPC service stubs)
```

Install via:
```bash
# buf — see https://buf.build/docs/installation
# or: go install github.com/bufbuild/buf/cmd/buf@latest

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

#### 1.2 Proto Directory Structure

```
proto/
├── buf.yaml                          # buf module configuration
├── buf.gen.yaml                      # code generation configuration
├── buf.lock                          # dependency lock file
├── rh_trex/
│   └── v1/
│       ├── common.proto              # shared messages (Meta, ListMeta, Error)
│       ├── dinosaurs.proto           # Dinosaur service definition
│       └── ...                       # one proto per entity
```

Generated code goes to `pkg/api/grpc/` (not a separate `proto/generated/` tree):
```
pkg/api/grpc/
└── rh_trex/
    └── v1/
        ├── common.pb.go
        ├── dinosaurs.pb.go
        └── dinosaurs_grpc.pb.go
```

#### 1.3 Generated Code Policy

Generated proto Go code is **gitignored**. CI runs `make proto` before `make binary`.

Rationale:
- Generated `.pb.go` files are large and change whenever proto tooling is updated
- Checking them in creates constant merge conflicts with no human-meaningful diffs
- The proto source files in `proto/` are the source of truth

Add to `.gitignore`:
```
pkg/api/grpc/
```

Add to CI pipeline (before `make binary`):
```bash
make proto
```

#### 1.4 buf Configuration

`proto/buf.yaml`:
```yaml
version: v2
modules:
  - path: .
    name: buf.build/openshift-online/rh-trex-ai
deps:
  - buf.build/googleapis/googleapis
lint:
  use:
    - DEFAULT
breaking:
  use:
    - WIRE_JSON
```

`proto/buf.gen.yaml`:
```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: ../pkg/api/grpc
    opt: paths=source_relative
  - remote: buf.build/grpc/go
    out: ../pkg/api/grpc
    opt: paths=source_relative
```

#### 1.5 Makefile Targets

```makefile
.PHONY: proto
proto:
	cd proto && buf generate

.PHONY: proto-lint
proto-lint:
	cd proto && buf lint

.PHONY: proto-breaking
proto-breaking:
	cd proto && buf breaking --against '.git#subdir=proto'

.PHONY: proto-clean
proto-clean:
	rm -rf pkg/api/grpc/
```

#### 1.6 Common Proto Messages

`proto/rh_trex/v1/common.proto`:
```protobuf
syntax = "proto3";

package rh_trex.v1;

option go_package = "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1;rh_trex_v1";

import "google/protobuf/timestamp.proto";

message ObjectReference {
  string id = 1;
  google.protobuf.Timestamp created_at = 2;
  google.protobuf.Timestamp updated_at = 3;
  string kind = 4;
  string href = 5;
}

message ListMeta {
  int32 page = 1;
  int32 size = 2;
  int32 total = 3;
}

message Error {
  string id = 1;
  string kind = 2;
  string href = 3;
  int32 code = 4;
  string reason = 5;
  string operation_id = 6;
}
```

#### 1.7 Proto Versioning & Backward Compatibility

**Rules for proto evolution:**

1. **Never remove or renumber existing fields.** Mark deprecated fields with `reserved` and the `[deprecated = true]` option.
2. **Never change field types** on existing field numbers. A `string` at field 2 stays a `string` at field 2 forever.
3. **New fields get new field numbers.** Adding `optional string habitat = 3;` is safe. Reusing a removed field number is not.
4. **Never rename RPC methods** in a published service. Add new methods instead, deprecate old ones.
5. **Run `buf breaking`** in CI against the main branch to enforce wire compatibility automatically. This is configured via the `breaking` section in `buf.yaml` with the `WIRE_JSON` category, which catches field number reuse, type changes, and service/method removal.

**When v2 is needed:**
- Create `proto/rh_trex/v2/` with a new `package rh_trex.v2` and `go_package` pointing to `.../rh_trex/v2;rh_trex_v2`
- Both v1 and v2 services run concurrently on the same gRPC server
- Deprecate v1 service methods by adding `option deprecated = true;`
- Remove v1 only after all clients have migrated (announced with a timeline)

---

### Phase 2: gRPC Server Infrastructure **[IMPLEMENTED]**

**Goal:** Create the gRPC server, registration system, interceptors, and graceful shutdown — mirroring the existing REST server patterns.

#### 2.1 gRPC Configuration

`pkg/config/grpc.go`:
```go
type GRPCConfig struct {
    EnableGRPC  bool
    BindAddress string        // default: "localhost:9000"
    EnableTLS   bool
    TLSCertFile string
    TLSKeyFile  string
}

func NewGRPCConfig() *GRPCConfig {
    return &GRPCConfig{
        EnableGRPC:  true,
        BindAddress: "localhost:9000",
    }
}

func (c *GRPCConfig) AddFlags(fs *pflag.FlagSet) {
    fs.BoolVar(&c.EnableGRPC, "enable-grpc", c.EnableGRPC, "Enable gRPC server")
    fs.StringVar(&c.BindAddress, "grpc-server-bindaddress", c.BindAddress, "gRPC server bind address")
    fs.BoolVar(&c.EnableTLS, "grpc-enable-tls", c.EnableTLS, "Enable TLS for gRPC server")
    fs.StringVar(&c.TLSCertFile, "grpc-tls-cert-file", c.TLSCertFile, "gRPC TLS certificate file")
    fs.StringVar(&c.TLSKeyFile, "grpc-tls-key-file", c.TLSKeyFile, "gRPC TLS key file")
}

func (c *GRPCConfig) ReadFiles() error {
    return nil
}
```

Add to `ApplicationConfig`:
```go
type ApplicationConfig struct {
    Server      *ServerConfig
    GRPC        *GRPCConfig     // ◄── NEW
    Metrics     *MetricsConfig
    HealthCheck *HealthCheckConfig
    Database    *DatabaseConfig
    OCM         *OCMConfig
    Sentry      *SentryConfig
}
```

CLI flags:
```
--enable-grpc                   Enable gRPC server (default: true)
--grpc-server-bindaddress       gRPC bind address (default: "localhost:9000")
--grpc-enable-tls               Enable TLS for gRPC
--grpc-tls-cert-file            TLS cert file for gRPC
--grpc-tls-key-file             TLS key file for gRPC
```

#### 2.2 gRPC Service Registration (mirrors `routes.go` pattern)

`pkg/server/grpc_registry.go`:
```go
package server

import (
    "google.golang.org/grpc"
)

type GRPCServiceRegistrationFunc func(grpcServer *grpc.Server, services ServicesInterface)

var grpcServiceRegistry = make(map[string]GRPCServiceRegistrationFunc)

func RegisterGRPCService(name string, registrationFunc GRPCServiceRegistrationFunc) {
    grpcServiceRegistry[name] = registrationFunc
}

func LoadDiscoveredGRPCServices(grpcServer *grpc.Server, services ServicesInterface) {
    for _, registrationFunc := range grpcServiceRegistry {
        registrationFunc(grpcServer, services)
    }
}
```

This mirrors the existing pattern exactly:

| REST | gRPC |
|------|------|
| `RegisterRoutes(name, func)` | `RegisterGRPCService(name, func)` |
| `LoadDiscoveredRoutes(router, services, ...)` | `LoadDiscoveredGRPCServices(grpcServer, services)` |
| `RouteRegistrationFunc` | `GRPCServiceRegistrationFunc` |

#### 2.3 gRPC Server Implementation

`pkg/server/grpc_server.go`:
```go
package server

import (
    "context"
    "net"

    "github.com/golang/glog"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/health"
    healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
    "google.golang.org/grpc/reflection"

    "github.com/openshift-online/rh-trex-ai/pkg/environments"
)

type grpcAPIServer struct {
    grpcServer *grpc.Server
    env        *environments.Env
}

var _ Server = &grpcAPIServer{}

func NewDefaultGRPCServer(env *environments.Env) Server {
    opts := []grpc.ServerOption{
        grpc.ChainUnaryInterceptor(
            RecoveryUnaryInterceptor(env.Config.Sentry.Timeout),
            LoggingUnaryInterceptor(),
            MetricsUnaryInterceptor(),
            TransactionUnaryInterceptor(env.Database.SessionFactory),
            AuthUnaryInterceptor(env),
        ),
        grpc.ChainStreamInterceptor(
            RecoveryStreamInterceptor(env.Config.Sentry.Timeout),
            LoggingStreamInterceptor(),
            MetricsStreamInterceptor(),
            AuthStreamInterceptor(env),
        ),
    }

    if env.Config.GRPC.EnableTLS {
        creds, err := credentials.NewServerTLSFromFile(
            env.Config.GRPC.TLSCertFile,
            env.Config.GRPC.TLSKeyFile,
        )
        if err != nil {
            glog.Fatalf("Failed to load gRPC TLS credentials: %v", err)
        }
        opts = append(opts, grpc.Creds(creds))
    }

    s := &grpcAPIServer{
        grpcServer: grpc.NewServer(opts...),
        env:        env,
    }

    LoadDiscoveredGRPCServices(s.grpcServer, &env.Services)

    healthServer := health.NewServer()
    healthgrpc.RegisterHealthServer(s.grpcServer, healthServer)

    reflection.Register(s.grpcServer)

    return s
}

func (s *grpcAPIServer) Start() {
    listener, err := s.Listen()
    if err != nil {
        glog.Fatalf("Unable to start gRPC server: %v", err)
    }
    glog.Infof("gRPC server listening at %s", s.env.Config.GRPC.BindAddress)
    s.Serve(listener)
}

func (s *grpcAPIServer) Listen() (net.Listener, error) {
    return net.Listen("tcp", s.env.Config.GRPC.BindAddress)
}

func (s *grpcAPIServer) Serve(listener net.Listener) {
    if err := s.grpcServer.Serve(listener); err != nil {
        Check(err, "gRPC server terminated with errors", s.env.Config.Sentry.Timeout)
    }
    glog.Info("gRPC server terminated")
}

func (s *grpcAPIServer) Stop() error {
    glog.Info("gRPC server shutting down gracefully")
    s.grpcServer.GracefulStop()
    return nil
}
```

**Error handling strategy** (consistent with `defaultAPIServer`):
- `Listen()` returns errors to the caller. `Start()` calls `glog.Fatalf` if listen fails — the server cannot start, so the process must exit. This matches the existing `defaultAPIServer.Start()` pattern.
- `Serve()` delegates to `Check()` (from `server.go`) which logs to sentry and calls `os.Exit(1)` on real errors, but ignores `http.ErrServerClosed`-equivalent scenarios.
- TLS credential failures are fatal — if you configured TLS but the cert is broken, the server refuses to start. No silent fallback to plaintext.

#### 2.4 gRPC Interceptors (mirrors HTTP middleware)

`pkg/server/grpc_interceptors.go`:

| HTTP Middleware | gRPC Interceptor | Purpose |
|-----------------|-------------------|---------|
| `RequestLoggingMiddleware` | `LoggingUnaryInterceptor` | Log method, duration, status code |
| `MetricsMiddleware` | `MetricsUnaryInterceptor` | Prometheus counters/histograms |
| `AuthenticateAccountJWT` | `AuthUnaryInterceptor` | Extract JWT from metadata, validate, set username in context |
| `AuthorizeApi` | (inside `AuthUnaryInterceptor`) | Check permissions via OCM |
| `TransactionMiddleware` | `TransactionUnaryInterceptor` | Wrap calls in DB transaction via `db.NewContext` / `db.Resolve` |
| (panic recovery) | `RecoveryUnaryInterceptor` | Recover from panics, log to sentry, return `codes.Internal` |

##### TransactionUnaryInterceptor

The REST side wraps every request in a DB transaction via `db.TransactionMiddleware` which calls `db.NewContext()` to begin a transaction and `db.Resolve()` to commit/rollback. The gRPC side must do the same:

```go
func TransactionUnaryInterceptor(sessionFactory db.SessionFactory) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        ctx, err := db.NewContext(ctx, sessionFactory)
        if err != nil {
            glog.Errorf("Failed to create DB transaction for gRPC call %s: %v", info.FullMethod, err)
            return nil, status.Error(codes.Internal, "internal database error")
        }
        defer func() { db.Resolve(ctx) }()

        return handler(ctx, req)
    }
}
```

This ensures gRPC writes have the same transaction semantics as REST writes — `db.NewContext` begins the transaction, `db.Resolve` commits on success or rolls back on error/panic.

##### AuthUnaryInterceptor

```go
func AuthUnaryInterceptor(env *environments.Env) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        if !env.Config.Server.EnableJWT {
            return handler(ctx, req)
        }

        // Skip auth for gRPC health checks and reflection
        if info.FullMethod == "/grpc.health.v1.Health/Check" ||
            info.FullMethod == "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo" {
            return handler(ctx, req)
        }

        md, ok := metadata.FromIncomingContext(ctx)
        if !ok {
            return nil, status.Error(codes.Unauthenticated, "missing metadata")
        }

        authHeader := md.Get("authorization")
        if len(authHeader) == 0 {
            return nil, status.Error(codes.Unauthenticated, "missing authorization token")
        }

        token := strings.TrimPrefix(authHeader[0], "Bearer ")
        token = strings.TrimPrefix(token, "bearer ")

        // Parse and validate JWT claims
        parser := jwt.NewParser()
        jwtToken, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
        if err != nil {
            return nil, status.Error(codes.Unauthenticated, "invalid token format")
        }

        claims, ok := jwtToken.Claims.(jwt.MapClaims)
        if !ok {
            return nil, status.Error(codes.Unauthenticated, "invalid token claims")
        }

        username, _ := claims["username"].(string)
        if username == "" {
            username, _ = claims["preferred_username"].(string)
        }
        if username == "" {
            return nil, status.Error(codes.Unauthenticated, "token missing username claim")
        }

        ctx = auth.SetUsernameContext(ctx, username)

        // Authorization check via OCM (if enabled)
        if env.Config.Server.EnableAuthz && env.Clients.OCM != nil {
            allowed, err := env.Clients.OCM.Authorization.AccessReview(
                ctx, username, "get", "Dinosaur", "", "", "")
            if err != nil {
                return nil, status.Error(codes.Internal, "authorization check failed")
            }
            if !allowed {
                return nil, status.Error(codes.PermissionDenied, "access denied")
            }
        }

        return handler(ctx, req)
    }
}
```

Note: Full JWT signature verification should use the same `ocm-sdk-go/authentication` library as the REST side. The above shows the claim extraction flow — production implementation should validate against the JWK cert URL/file configured in `env.Config.Server.JwkCertURL` / `env.Config.Server.JwkCertFile`.

##### RecoveryUnaryInterceptor

```go
func RecoveryUnaryInterceptor(sentryTimeout time.Duration) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
        defer func() {
            if r := recover(); r != nil {
                glog.Errorf("gRPC panic in %s: %v\n%s", info.FullMethod, r, debug.Stack())
                sentry.CurrentHub().Recover(r)
                sentry.Flush(sentryTimeout)
                err = status.Error(codes.Internal, "internal server error")
            }
        }()
        return handler(ctx, req)
    }
}
```

##### LoggingUnaryInterceptor

```go
func LoggingUnaryInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        start := time.Now()
        operationID := logger.NewOperationID()
        ctx = logger.SetOperationID(ctx, operationID)
        log := logger.NewOCMLogger(ctx)

        resp, err := handler(ctx, req)

        duration := time.Since(start)
        code := status.Code(err)
        log.Infof("gRPC %s %s %s", info.FullMethod, code, duration)

        return resp, err
    }
}
```

##### MetricsUnaryInterceptor

```go
func MetricsUnaryInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        start := time.Now()
        resp, err := handler(ctx, req)
        duration := time.Since(start)
        code := status.Code(err)

        grpcRequestCount.WithLabelValues(info.FullMethod, code.String()).Inc()
        grpcRequestDuration.WithLabelValues(info.FullMethod).Observe(duration.Seconds())

        return resp, err
    }
}
```

Prometheus metrics registered alongside existing REST metrics:
```go
var (
    grpcRequestCount = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "grpc_requests_total",
            Help: "Total number of gRPC requests",
        },
        []string{"method", "code"},
    )
    grpcRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "grpc_request_duration_seconds",
            Help:    "gRPC request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method"},
    )
)

func init() {
    prometheus.MustRegister(grpcRequestCount)
    prometheus.MustRegister(grpcRequestDuration)
}
```

##### Stream Interceptors

Stream interceptors follow the same patterns but operate on `grpc.StreamServerInterceptor` signatures. The transaction interceptor is **not applied to streams** — streaming RPCs manage their own transaction boundaries per-message rather than wrapping the entire stream in one transaction.

#### 2.5 Graceful Shutdown

The existing codebase has **no signal handling** — `runServe()` blocks on `select {}` and SIGTERM kills the process without cleanup. This is a pre-existing issue, not gRPC-specific. Adding gRPC is the right time to fix it for all servers.

`pkg/cmd/serve.go`:
```go
func runServe(getSpecData func() ([]byte, error)) {
    env := environments.Environment()
    env.Initialize()
    specData, err := getSpecData()
    if err != nil {
        glog.Fatalf("Failed to load OpenAPI spec: %v", err)
    }

    var servers []pkgserver.Server

    apiServer := pkgserver.NewDefaultAPIServer(env, specData)
    servers = append(servers, apiServer)
    go apiServer.Start()

    if env.Config.GRPC.EnableGRPC {
        grpcServer := pkgserver.NewDefaultGRPCServer(env)
        servers = append(servers, grpcServer)
        go grpcServer.Start()
    }

    metricsServer := pkgserver.NewDefaultMetricsServer(env)
    servers = append(servers, metricsServer)
    go metricsServer.Start()

    healthCheckServer := pkgserver.NewDefaultHealthCheckServer(env)
    servers = append(servers, healthCheckServer)
    go healthCheckServer.Start()

    controllersServer := pkgserver.NewDefaultControllersServer(env)
    go controllersServer.Start()

    // Wait for shutdown signal
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
    sig := <-sigCh
    glog.Infof("Received signal %v, shutting down", sig)

    // Graceful shutdown with timeout
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    var wg sync.WaitGroup
    for _, s := range servers {
        wg.Add(1)
        go func(srv pkgserver.Server) {
            defer wg.Done()
            if err := srv.Stop(); err != nil {
                glog.Errorf("Error stopping server: %v", err)
            }
        }(s)
    }

    // Wait for all servers to stop or timeout
    doneCh := make(chan struct{})
    go func() {
        wg.Wait()
        close(doneCh)
    }()

    select {
    case <-doneCh:
        glog.Info("All servers stopped gracefully")
    case <-shutdownCtx.Done():
        glog.Warning("Shutdown timed out, forcing exit")
    }

    env.Database.SessionFactory.Close()
    glog.Info("Database connections closed")
}
```

This replaces the existing `select {}` with proper signal handling. All servers (REST, gRPC, metrics, health check) get `Stop()` called concurrently with a 30-second timeout. Database connections are closed after all servers stop.

Note: `ControllersServer` currently has no `Stop()` method — a `Stop()` implementation should be added that cancels the listener context and drains in-flight event handlers.

---

### Phase 3: First Entity — Dinosaurs gRPC Service **[IMPLEMENTED]**

**Goal:** Implement gRPC for the Dinosaur entity as the reference pattern for all future entities.

#### 3.1 Proto Definition

`proto/rh_trex/v1/dinosaurs.proto`:
```protobuf
syntax = "proto3";

package rh_trex.v1;

option go_package = "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1;rh_trex_v1";

import "rh_trex/v1/common.proto";

message Dinosaur {
  ObjectReference metadata = 1;
  string species = 2;
}

message CreateDinosaurRequest {
  string species = 1;
}

message GetDinosaurRequest {
  string id = 1;
}

message UpdateDinosaurRequest {
  string id = 1;
  optional string species = 2;
}

message DeleteDinosaurRequest {
  string id = 1;
}

message ListDinosaursRequest {
  int32 page = 1;
  int32 size = 2;
}

message ListDinosaursResponse {
  repeated Dinosaur items = 1;
  ListMeta metadata = 2;
}

message DeleteDinosaurResponse {}

service DinosaurService {
  rpc GetDinosaur(GetDinosaurRequest) returns (Dinosaur);
  rpc CreateDinosaur(CreateDinosaurRequest) returns (Dinosaur);
  rpc UpdateDinosaur(UpdateDinosaurRequest) returns (Dinosaur);
  rpc DeleteDinosaur(DeleteDinosaurRequest) returns (DeleteDinosaurResponse);
  rpc ListDinosaurs(ListDinosaursRequest) returns (ListDinosaursResponse);
}
```

#### 3.2 Input Validation

gRPC does not validate field contents — proto3 happily accepts empty strings, negative page numbers, and 10MB payloads. Every gRPC handler must validate input before calling the service layer.

**Approach: Manual validation with a shared helper pattern.**

We use manual validation rather than `protovalidate` to keep the dependency footprint small and because our validation rules are simple. If validation rules become complex (regex patterns, cross-field constraints), we revisit and adopt `protovalidate`.

```go
package grpcutil

import (
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

const (
    MaxStringFieldLength = 255
    MaxPageSize          = 500
    DefaultPageSize      = 20
    DefaultPage          = 1
)

func ValidateRequiredID(id string) error {
    if id == "" {
        return status.Error(codes.InvalidArgument, "id is required")
    }
    return nil
}

func ValidateStringField(name, value string, required bool) error {
    if required && value == "" {
        return status.Errorf(codes.InvalidArgument, "%s is required", name)
    }
    if len(value) > MaxStringFieldLength {
        return status.Errorf(codes.InvalidArgument, "%s exceeds maximum length of %d", name, MaxStringFieldLength)
    }
    return nil
}

func NormalizePagination(page, size int32) (int32, int32) {
    if page < 1 {
        page = DefaultPage
    }
    if size < 1 || size > MaxPageSize {
        size = DefaultPageSize
    }
    return page, size
}
```

Every handler calls validation before the service layer. See handler code in 3.3.

#### 3.3 gRPC Handler in Plugin

`plugins/dinosaurs/grpc_handler.go`:

The handler lives in the `dinosaurs` package, which defines `DinosaurService` and `Dinosaur` (the domain model that embeds `api.Meta`). These types are local to the package — no cross-package type confusion.

```go
package dinosaurs

import (
    "context"

    pb "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1"
    "github.com/openshift-online/rh-trex-ai/pkg/server/grpcutil"
)

type dinosaurGRPCHandler struct {
    pb.UnimplementedDinosaurServiceServer
    service DinosaurService
}

func NewDinosaurGRPCHandler(svc DinosaurService) pb.DinosaurServiceServer {
    return &dinosaurGRPCHandler{service: svc}
}

func (h *dinosaurGRPCHandler) GetDinosaur(ctx context.Context, req *pb.GetDinosaurRequest) (*pb.Dinosaur, error) {
    if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
        return nil, err
    }

    dino, svcErr := h.service.Get(ctx, req.Id)
    if svcErr != nil {
        return nil, serviceErrorToGRPC(svcErr)
    }
    return dinosaurToProto(dino), nil
}

func (h *dinosaurGRPCHandler) CreateDinosaur(ctx context.Context, req *pb.CreateDinosaurRequest) (*pb.Dinosaur, error) {
    if err := grpcutil.ValidateStringField("species", req.Species, true); err != nil {
        return nil, err
    }

    dino := &Dinosaur{Species: req.Species}
    result, svcErr := h.service.Create(ctx, dino)
    if svcErr != nil {
        return nil, serviceErrorToGRPC(svcErr)
    }
    return dinosaurToProto(result), nil
}

func (h *dinosaurGRPCHandler) UpdateDinosaur(ctx context.Context, req *pb.UpdateDinosaurRequest) (*pb.Dinosaur, error) {
    if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
        return nil, err
    }
    if req.Species != nil {
        if err := grpcutil.ValidateStringField("species", *req.Species, false); err != nil {
            return nil, err
        }
    }

    dino, svcErr := h.service.Get(ctx, req.Id)
    if svcErr != nil {
        return nil, serviceErrorToGRPC(svcErr)
    }
    if req.Species != nil {
        dino.Species = *req.Species
    }
    result, svcErr := h.service.Replace(ctx, dino)
    if svcErr != nil {
        return nil, serviceErrorToGRPC(svcErr)
    }
    return dinosaurToProto(result), nil
}

func (h *dinosaurGRPCHandler) DeleteDinosaur(ctx context.Context, req *pb.DeleteDinosaurRequest) (*pb.DeleteDinosaurResponse, error) {
    if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
        return nil, err
    }

    svcErr := h.service.Delete(ctx, req.Id)
    if svcErr != nil {
        return nil, serviceErrorToGRPC(svcErr)
    }
    return &pb.DeleteDinosaurResponse{}, nil
}

func (h *dinosaurGRPCHandler) ListDinosaurs(ctx context.Context, req *pb.ListDinosaursRequest) (*pb.ListDinosaursResponse, error) {
    page, size := grpcutil.NormalizePagination(req.Page, req.Size)

    // TODO: The current DinosaurService.All() does not support pagination.
    // When pagination is added to the service layer (returning PagingMeta),
    // pass page/size through. For now, fetch all and paginate in-memory.
    allDinos, svcErr := h.service.All(ctx)
    if svcErr != nil {
        return nil, serviceErrorToGRPC(svcErr)
    }

    total := int32(len(allDinos))
    start := (page - 1) * size
    if start >= total {
        return &pb.ListDinosaursResponse{
            Items:    []*pb.Dinosaur{},
            Metadata: &pb.ListMeta{Page: page, Size: size, Total: total},
        }, nil
    }
    end := start + size
    if end > total {
        end = total
    }
    pageItems := allDinos[start:end]

    items := make([]*pb.Dinosaur, len(pageItems))
    for i, d := range pageItems {
        items[i] = dinosaurToProto(d)
    }

    return &pb.ListDinosaursResponse{
        Items:    items,
        Metadata: &pb.ListMeta{Page: page, Size: size, Total: total},
    }, nil
}
```

**Pagination note:** The current `DinosaurService.All()` returns everything — it has no pagination support. The REST side has the same limitation. The gRPC handler does in-memory pagination as a stopgap. The correct fix is adding `List(ctx, page, size int) (DinosaurList, PagingMeta, *errors.ServiceError)` to the service interface, which benefits both REST and gRPC. This is tracked as a follow-up.

#### 3.4 Proto ↔ Domain Converters and Error Mapping

`plugins/dinosaurs/grpc_presenter.go`:
```go
package dinosaurs

import (
    "net/http"

    pb "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1"
    "github.com/openshift-online/rh-trex-ai/pkg/errors"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/protobuf/types/known/timestamppb"
)

func dinosaurToProto(d *Dinosaur) *pb.Dinosaur {
    return &pb.Dinosaur{
        Metadata: &pb.ObjectReference{
            Id:        d.ID,
            CreatedAt: timestamppb.New(d.CreatedAt),
            UpdatedAt: timestamppb.New(d.UpdatedAt),
            Kind:      "Dinosaur",
            Href:      "/api/rh-trex/v1/dinosaurs/" + d.ID,
        },
        Species: d.Species,
    }
}

func serviceErrorToGRPC(svcErr *errors.ServiceError) error {
    code := httpStatusToGRPCCode(svcErr.HttpCode)
    return status.Error(code, svcErr.Reason)
}

func httpStatusToGRPCCode(httpCode int) codes.Code {
    switch httpCode {
    case http.StatusBadRequest:
        return codes.InvalidArgument
    case http.StatusUnauthorized:
        return codes.Unauthenticated
    case http.StatusForbidden:
        return codes.PermissionDenied
    case http.StatusNotFound:
        return codes.NotFound
    case http.StatusConflict:
        return codes.AlreadyExists
    case http.StatusUnprocessableEntity:
        return codes.InvalidArgument
    case http.StatusTooManyRequests:
        return codes.ResourceExhausted
    case http.StatusServiceUnavailable:
        return codes.Unavailable
    case http.StatusGatewayTimeout:
        return codes.DeadlineExceeded
    default:
        if httpCode >= 400 && httpCode < 500 {
            return codes.InvalidArgument
        }
        return codes.Internal
    }
}
```

#### 3.5 Plugin Registration

Add to `plugins/dinosaurs/plugin.go` `init()`:
```go
import (
    // ... existing imports ...
    "google.golang.org/grpc"
    pb "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1"
)

func init() {
    // ... existing registrations (service, routes, controllers, presenters, migration) ...

    pkgserver.RegisterGRPCService("dinosaurs", func(grpcServer *grpc.Server, services pkgserver.ServicesInterface) {
        envServices := services.(*environments.Services)
        dinoService := Service(envServices)
        pb.RegisterDinosaurServiceServer(grpcServer, NewDinosaurGRPCHandler(dinoService))
    })
}
```

This uses the same `Service()` locator function that the REST route registration uses — same service instance, same dependencies.

---

### Phase 4: Generator Integration **[NOT YET IMPLEMENTED]**

**Goal:** Update the code generator to produce gRPC files automatically for new entities.

#### 4.1 New Generator Templates

Add these templates to `generator/templates/`:

| Template | Output | Purpose |
|----------|--------|---------|
| `grpc_handler.go.tmpl` | `plugins/{{.KindLowerPlural}}/grpc_handler.go` | gRPC handler with validation, calling service layer |
| `grpc_presenter.go.tmpl` | `plugins/{{.KindLowerPlural}}/grpc_presenter.go` | Proto ↔ domain converters + error mapping |
| `kind.proto.tmpl` | `proto/rh_trex/v1/{{.KindSnakeCasePlural}}.proto` | Protobuf service definition |

#### 4.2 Generator Updates

Modify `scripts/generator.go` to:
1. Generate `.proto` file from template with custom fields mapped to protobuf types
2. Generate `grpc_handler.go` and `grpc_presenter.go` in the plugin directory
3. Add `RegisterGRPCService()` call to the generated `plugin.go` template
4. Run `buf generate` after file generation (via `make proto`)

Field type mapping for proto generation:
| Go Type | Protobuf Type | Proto Import Required |
|---------|---------------|----------------------|
| `string` | `string` | none |
| `int` | `int32` | none |
| `int64` | `int64` | none |
| `bool` | `bool` | none |
| `float64` | `double` | none |
| `time.Time` | `google.protobuf.Timestamp` | `google/protobuf/timestamp.proto` |

---

### Phase 5: Streaming Support **[PARTIALLY IMPLEMENTED]**

> **Status:** WatchDinosaurs server-streaming with EventBroker fan-out is fully implemented (`pkg/server/event_broker.go`, `plugins/dinosaurs/grpc_handler.go`). BulkCreateDinosaurs client-streaming is not yet implemented.

**Goal:** Add gRPC server-streaming for real-time entity change notifications and client-streaming for bulk operations.

This phase requires its own detailed design because the existing event system was not built for fan-out to external subscribers. The sections below define the architecture, not just the proto definitions.

#### 5.1 Event Streaming Architecture

**Problem:** The existing `KindControllerManager` processes events via `OnUpsert`/`OnDelete` handlers. These are internal consumers — they run inside the server process, one handler per event type. There is no mechanism for external clients to subscribe to event streams.

**Solution: Event Broker with Fan-Out**

Introduce an `EventBroker` that sits between PostgreSQL LISTEN/NOTIFY and gRPC stream clients:

```
PostgreSQL NOTIFY("events", id)
        │
        ▼
┌───────────────────┐
│  SessionFactory   │
│  .NewListener()   │  (existing — unchanged)
└────────┬──────────┘
         │
    ┌────▼────┐
    │  Router │  (new — routes events to both controllers AND broker)
    └────┬────┘
         │
    ┌────┴──────────────┐
    │                   │
    ▼                   ▼
┌──────────┐    ┌──────────────┐
│Controller│    │ EventBroker  │  (new)
│ Manager  │    │              │
│(existing)│    │ subscribers: │
└──────────┘    │   map[id]    │
                │     chan     │
                └──────┬───────┘
                       │
              ┌────────┼────────┐
              ▼        ▼        ▼
           client1  client2  client3
           (gRPC    (gRPC    (gRPC
            stream)  stream)  stream)
```

**EventBroker design:**

```go
type EventBroker struct {
    mu          sync.RWMutex
    subscribers map[string]chan *Event    // subscriber ID → event channel
    bufferSize  int                      // per-subscriber channel buffer (e.g., 256)
    metrics     *brokerMetrics
}

type Subscription struct {
    ID     string
    Events <-chan *Event
    cancel context.CancelFunc
}
```

**Key behaviors:**

1. **Subscribe:** Creates a buffered channel and adds it to the subscriber map. Returns a `Subscription` with a read-only channel and cancel function.

2. **Publish:** Non-blocking send to all subscriber channels. If a subscriber's channel is full (backpressure), the event is dropped for that subscriber and a `grpc_stream_events_dropped_total` counter is incremented. This prevents a slow client from blocking the entire event pipeline.

3. **Unsubscribe:** Triggered by `cancel()` (client disconnect, context cancellation, or explicit close). Removes from map, closes channel.

4. **Client lifecycle:**
   - Client calls `WatchDinosaurs` → server creates subscription → enters send loop
   - Send loop reads from subscription channel, sends to gRPC stream
   - If `stream.Send()` returns an error (client disconnected), cancel subscription and return
   - If context is cancelled (client timeout, server shutdown), cancel subscription and return
   - Server shutdown calls `GracefulStop()` which cancels all active stream contexts

5. **Backpressure handling:**
   - Channel buffer size: 256 events per subscriber (configurable)
   - On buffer full: drop event, increment metric, log warning
   - Alternative considered: block and apply per-subscriber timeout → rejected because it couples slow clients to event throughput
   - Client can detect gaps via event sequence numbers (if needed in future)

**Proto definition:**

```protobuf
enum EventType {
  EVENT_TYPE_UNSPECIFIED = 0;
  EVENT_TYPE_CREATED = 1;
  EVENT_TYPE_UPDATED = 2;
  EVENT_TYPE_DELETED = 3;
}

message WatchDinosaursRequest {
  // Future: add filter fields (e.g., species, label selectors)
}

message DinosaurWatchEvent {
  EventType type = 1;
  Dinosaur dinosaur = 2;       // populated for CREATED/UPDATED, nil for DELETED
  string resource_id = 3;     // always populated (needed for DELETED where dinosaur is nil)
}

service DinosaurService {
  // ... existing unary RPCs ...
  rpc WatchDinosaurs(WatchDinosaursRequest) returns (stream DinosaurWatchEvent);
}
```

**Handler implementation sketch:**

```go
func (h *dinosaurGRPCHandler) WatchDinosaurs(req *pb.WatchDinosaursRequest, stream pb.DinosaurService_WatchDinosaursServer) error {
    sub := h.broker.Subscribe(stream.Context())
    defer h.broker.Unsubscribe(sub.ID)

    for {
        select {
        case event, ok := <-sub.Events:
            if !ok {
                return nil // channel closed, broker shutting down
            }
            watchEvent, err := h.eventToWatchEvent(stream.Context(), event)
            if err != nil {
                glog.Warningf("Failed to convert event %s: %v", event.ID, err)
                continue // skip this event, don't kill the stream
            }
            if err := stream.Send(watchEvent); err != nil {
                return err // client disconnected
            }
        case <-stream.Context().Done():
            return stream.Context().Err()
        }
    }
}
```

**Metrics:**
- `grpc_stream_subscribers_active` (gauge) — current number of active watch streams
- `grpc_stream_events_sent_total` (counter) — events successfully sent to clients
- `grpc_stream_events_dropped_total` (counter) — events dropped due to slow clients
- `grpc_stream_duration_seconds` (histogram) — how long watch streams stay open

**Wiring into existing event system:**

The `SessionFactory.NewListener()` callback currently routes to `KindControllerManager.Handle`. This is modified to route to both the controller manager AND the event broker:

```go
func NewEventRouter(controllerManager *controllers.KindControllerManager, broker *EventBroker) func(id string) {
    return func(id string) {
        controllerManager.Handle(id)
        broker.Publish(id)
    }
}
```

The broker's `Publish(id)` loads the event from the database (via EventService) to get the full event details (source type, event type, resource ID), then fans out to relevant subscribers.

#### 5.2 Bulk Operations

Client-streaming for batch imports:

```protobuf
message BulkCreateDinosaursResponse {
  int32 created = 1;
  int32 failed = 2;
  repeated Error errors = 3;
}

service DinosaurService {
  rpc BulkCreateDinosaurs(stream CreateDinosaurRequest) returns (BulkCreateDinosaursResponse);
}
```

**Implementation notes:**
- Each message in the stream is validated individually
- Valid items are batched and committed in groups (e.g., 100 per transaction)
- Failed items are collected into the `errors` array with their index
- The stream is not wrapped in a single transaction — that would hold a DB transaction open for the entire stream duration, which could be minutes
- Instead: per-batch transactions with explicit commit points

---

### Phase 6: Optional — grpc-gateway (REST from Proto) **[NOT YET IMPLEMENTED]**

**Goal:** Generate REST endpoints from `.proto` files so both APIs are defined in one place.

This is a longer-term option. It would:
1. Add `google/api/annotations.proto` HTTP annotations to proto files
2. Use `grpc-gateway` to generate a reverse proxy
3. Eventually replace the hand-written gorilla/mux REST handlers with the generated gateway
4. Generate OpenAPI specs from proto files (replacing hand-written `openapi.*.yaml` files)

This is a **major migration** and should only be pursued once the parallel gRPC transport (Phases 1-4) is stable and proven.

**Realistic estimate: 4-8 weeks** for production-ready feature parity, including:
- Migrating all entities to proto-first definitions (1-2 weeks)
- Implementing custom HTTP error formatting to match existing REST error shapes (1 week)
- Achieving OpenAPI spec parity with existing hand-written specs (1-2 weeks)
- Updating all client consumers and integration tests (1-2 weeks)
- Gradual rollout with dual-serving (old REST + new gateway) for validation (1 week)

---

## File Inventory

### New Files

```
proto/buf.yaml                                   # buf module configuration
proto/buf.gen.yaml                               # buf code generation configuration
proto/rh_trex/v1/common.proto                    # shared protobuf messages
proto/rh_trex/v1/dinosaurs.proto                 # Dinosaur service definition
pkg/api/grpc/                                    # generated proto Go code (gitignored)
pkg/config/grpc.go                               # gRPC configuration struct + flags
pkg/server/grpc_server.go                        # gRPC server implementing Server interface
pkg/server/grpc_registry.go                      # RegisterGRPCService / LoadDiscoveredGRPCServices
pkg/server/grpc_interceptors.go                  # Auth, logging, metrics, transaction, recovery interceptors
pkg/server/grpcutil/validation.go                # Shared gRPC input validation helpers
plugins/dinosaurs/grpc_handler.go                # gRPC handler for Dinosaurs (with validation)
plugins/dinosaurs/grpc_presenter.go              # Proto ↔ domain converters + error mapping
generator/templates/grpc_handler.go.tmpl         # Generator template for gRPC handlers
generator/templates/grpc_presenter.go.tmpl       # Generator template for converters
generator/templates/kind.proto.tmpl              # Generator template for proto files
```

### Modified Files

```
.gitignore                                       # Add pkg/api/grpc/
pkg/config/config.go                             # Add GRPCConfig to ApplicationConfig
pkg/cmd/serve.go                                 # Add gRPC server goroutine + signal handling
plugins/dinosaurs/plugin.go                      # Add RegisterGRPCService() call in init()
scripts/generator.go                             # Add proto + gRPC handler generation
go.mod                                           # Promote grpc + protobuf to direct deps
Makefile                                         # Add proto, proto-lint, proto-breaking targets
```

## Dependencies

Already present as indirect dependencies in `go.mod` — promote to direct:

```
google.golang.org/grpc v1.75.1
google.golang.org/protobuf v1.36.10
google.golang.org/genproto/googleapis/rpc
```

New direct dependencies:
```
google.golang.org/grpc/health           # gRPC health checking protocol
google.golang.org/grpc/reflection       # server reflection for grpcurl
```

Build tools (installed via `go install`, not Go module dependencies):
```
buf                                     # proto management (lint, breaking, generate)
protoc-gen-go                           # Go message code generator
protoc-gen-go-grpc                      # Go gRPC service code generator
```

## Error Code Mapping

| HTTP Status | gRPC Code | ServiceError Code |
|-------------|-----------|-------------------|
| 400 | `InvalidArgument` | `ErrorBadRequest` |
| 401 | `Unauthenticated` | `ErrorUnauthenticated` |
| 403 | `PermissionDenied` | `ErrorForbidden` |
| 404 | `NotFound` | `ErrorNotFound` |
| 409 | `AlreadyExists` | `ErrorConflict` |
| 422 | `InvalidArgument` | `ErrorValidation` |
| 429 | `ResourceExhausted` | (rate limiting) |
| 500 | `Internal` | `ErrorGeneral` |
| 503 | `Unavailable` | (service unavailable) |

## Testing Strategy

### Unit Tests
- Mock `DinosaurService` interface (already exists in test infrastructure)
- Test gRPC handlers directly using `bufconn` (in-memory gRPC connections — no real TCP)
- Test each interceptor independently with mock handlers
- Test `EventBroker` fan-out, backpressure, and unsubscribe behavior
- Test validation helpers with boundary values (empty, max length, negative numbers)

### Integration Tests
- Extend existing integration test framework in `test/`
- Create gRPC client connections to real server in test setup
- Verify CRUD operations via gRPC produce same database state as REST
- Test streaming with real PostgreSQL LISTEN/NOTIFY
- Test auth interceptor with valid/invalid/missing JWT tokens
- Test transaction rollback on handler errors

### Manual Testing
```bash
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# List services (requires reflection)
grpcurl -plaintext localhost:9000 list

# Create a dinosaur
grpcurl -plaintext -d '{"species": "velociraptor"}' \
  localhost:9000 rh_trex.v1.DinosaurService/CreateDinosaur

# List dinosaurs (with pagination)
grpcurl -plaintext -d '{"page": 1, "size": 10}' \
  localhost:9000 rh_trex.v1.DinosaurService/ListDinosaurs

# Get by ID
grpcurl -plaintext -d '{"id": "abc123"}' \
  localhost:9000 rh_trex.v1.DinosaurService/GetDinosaur

# Test validation (should return InvalidArgument)
grpcurl -plaintext -d '{}' \
  localhost:9000 rh_trex.v1.DinosaurService/CreateDinosaur

# Watch for events (streaming)
grpcurl -plaintext localhost:9000 rh_trex.v1.DinosaurService/WatchDinosaurs
```

## Implementation Order

| Phase | Effort | Deliverable |
|-------|--------|-------------|
| Phase 1: Proto Tooling | 1-2 days | `buf generate` works, `buf lint` passes, common.proto compiles, CI pipeline updated |
| Phase 2: Server Infra | 3-4 days | gRPC server starts, all interceptors work (auth, tx, metrics, logging, recovery), graceful shutdown for all servers, `--enable-grpc` flag |
| Phase 3: Dinosaurs | 2-3 days | Full CRUD via gRPC with validation and pagination, integration tests pass |
| Phase 4: Generator | 2-3 days | `go run ./scripts/generator.go --kind Foo` produces proto + gRPC files |
| Phase 5: Streaming | 5-7 days | EventBroker with fan-out, WatchDinosaurs streaming, BulkCreate, backpressure metrics |
| Phase 6: grpc-gateway | 4-8 weeks | Optional: REST generated from protos with full feature parity |

**Total for Phases 1-4 (core integration): ~8-12 days**
**Total for Phases 1-5 (with streaming): ~13-19 days**
