# Plugin Architecture Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** FW-001
**Related:** [Entity Lifecycle](entity-lifecycle.spec.md), [Service Locator](service-locator.spec.md)
**Implements:** `pkg/registry/`, `pkg/server/routes.go`, `pkg/server/controllers.go`, `pkg/server/grpc_registry.go`, `plugins/*/plugin.go`

---

## Purpose

Define the plugin-based architecture that enables self-contained entities with auto-registration via Go `init()` functions, eliminating centralized wiring and enabling downstream projects to compose services declaratively.

## Requirements

### Requirement: Plugin Self-Containment

Each entity plugin SHALL be a self-contained Go package under `plugins/{kindLowerPlural}/` that encapsulates model, DAO, service, handler, presenter, migration, gRPC handler, and tests.

#### Scenario: New plugin directory structure
- GIVEN a new entity "Widget"
- WHEN the plugin is generated
- THEN the directory `plugins/widgets/` SHALL contain: `model.go`, `dao.go`, `mock_dao.go`, `service.go`, `handler.go`, `presenter.go`, `grpc_handler.go`, `grpc_presenter.go`, `migration.go`, `plugin.go`, `integration_test.go`, `grpc_integration_test.go`, `factory_test.go`, `testmain_test.go`

### Requirement: Init-Based Auto-Registration

Each plugin SHALL register its components via a single `init()` function in `plugin.go` using the global registries.

#### Scenario: Plugin registration on import
- GIVEN a plugin package with an `init()` function
- WHEN the package is imported via blank import (`_ "repo/plugins/widgets"`)
- THEN the following registrations SHALL occur:
  - Service locator via `registry.RegisterService(name, locatorFunc)`
  - Routes via `pkgserver.RegisterRoutes(name, routeFunc)`
  - Controllers via `pkgserver.RegisterController(name, controllerFunc)`
  - gRPC services via `pkgserver.RegisterGRPCService(name, grpcFunc)`
  - Presenters via `presenters.RegisterPath()` and `presenters.RegisterKind()`
  - Migration via `db.RegisterMigration(migration())`

### Requirement: Blank Import Activation

The application entry point (`cmd/{cmd}/main.go`) SHALL activate plugins exclusively through blank imports.

#### Scenario: Adding a new plugin
- GIVEN a new plugin at `plugins/widgets/`
- WHEN the import `_ "repo/project/plugins/widgets"` is added to `main.go`
- THEN all widget components SHALL be available at runtime without modifying any other file

### Requirement: Registry Thread Safety

The `ServiceRegistry` SHALL use `sync.RWMutex` to ensure thread-safe concurrent registration and discovery.

#### Scenario: Concurrent service access
- GIVEN multiple goroutines accessing the service registry
- WHEN one goroutine registers a service while others read
- THEN no data race SHALL occur
- AND reads SHALL use `RLock` for non-blocking concurrent access

### Requirement: Discovery Loading

The framework SHALL provide `LoadDiscovered*` functions that iterate over registered components and wire them into the runtime.

#### Scenario: Route discovery
- GIVEN three plugins have registered routes via `RegisterRoutes`
- WHEN `LoadDiscoveredRoutes(router, services, authMW, authzMW)` is called
- THEN all three plugins' route registration functions SHALL be invoked with the provided router and middleware

### Requirement: Migration Auto-Registration

Database migrations SHALL be registered via `db.RegisterMigration()` and sorted by ID for deterministic ordering.

#### Scenario: Migration ordering
- GIVEN plugins A (ID: "202507010001") and B (ID: "202507020001") register migrations
- WHEN `LoadDiscoveredMigrations()` is called
- THEN plugin A's migration SHALL precede plugin B's migration in the returned slice

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| `init()` over explicit registration | Eliminates centralized wiring files; plugins are self-contained |
| Blank imports in `main.go` | Single point of plugin composition; easy to add/remove |
| Separate registries per component type | Routes, controllers, gRPC services, and migrations have different signatures |
| `sync.RWMutex` on ServiceRegistry | Services may be accessed concurrently during initialization and runtime |
| Map-based registries for routes/controllers/gRPC | Prevents duplicate registration; last-write-wins semantics |
| Slice-based registry for migrations | Allows sorted ordering by migration ID string |
