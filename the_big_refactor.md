# The Big Refactor: rh-trex Template to Library

## Overview

Over 8 phases of work, rh-trex was transformed from a fork-and-modify template into a reusable Go library. Its first consumer, the Registry Credentials Service (RCS), went from maintaining ~13,700 lines of duplicated framework code to importing rh-trex directly — keeping only its domain-specific business logic locally.

This document summarizes the intent, impact, and future value of each phase.

---

## The Starting Point

rh-trex was designed as a "clone and customize" template. When RCS was created, the entire codebase was forked. Over time, both projects evolved independently, and the framework code — error handling, database access, authentication, HTTP server infrastructure, logging, metrics, test utilities — diverged. Bugs fixed in one project didn't propagate to the other. Improvements required double implementation.

RCS had ~90% structural overlap with rh-trex, but the two could not share code because:
- Hardcoded service names (`"rh-trex"`) were embedded throughout `pkg/`
- Entity types (Dinosaur) were mixed into framework packages
- `cmd/` infrastructure used `init()` singletons and global state that prevented importability
- Test utilities were copy-pasted with subtle differences

---

## Phase-by-Phase Summary

### Phase 1: Parameterization

**Intent:** Make rh-trex's `pkg/` packages consumable by other projects without forking.

Every hardcoded `"rh-trex"` string — in error responses, API paths, metadata endpoints, Sentry config, CORS headers — was replaced with runtime configuration. A `pkg/trex/config.go` module provides `Init()` and `GetConfig()`, allowing consumers to set their service name, base path, and project root at startup.

A critical timing issue was resolved: the panic response body in `pkg/api/error.go` was constructed during `init()`, before any consumer could call `Init()`. This was changed to lazy construction via `sync.Once`.

**Impact:** `pkg/` became safe to import without getting rh-trex-specific behavior baked in.

### Phase 2: Self-Contained Plugin Architecture

**Intent:** Separate framework types from entity types so consumers don't inherit rh-trex's example entities.

The Dinosaur entity (model, presenter, service, handler, DAO, mock, migration, tests) was moved from `pkg/` into a self-contained `plugins/dinosaurs/` directory. Framework types (`Meta`, `Event`, `Error`) stayed in `pkg/api/`. A plugin-based migration registration system (`db.RegisterMigration()` / `LoadDiscoveredMigrations()`) replaced the centralized migration list, and handler/service helper functions were exported for consumer use.

**Impact:** Consumers can import `pkg/api/` for framework types without pulling in example business logic. Entity code is fully encapsulated — each plugin registers itself via `init()`.

### Phase 3: RCS Import Migration

**Intent:** Replace all of RCS's duplicated `pkg/` framework code with imports from rh-trex.

RCS restructured its 3 entities (registryCredentials, accessTokens, registrys) into self-contained plugins mirroring rh-trex's pattern, deleted its dinosaur entity entirely, and replaced all local framework packages — errors, util, logger, config, auth, db, dao, controllers, services, handlers, API types, presenters — with `github.com/openshift-online/rh-trex/pkg/...` imports across 46 files.

**Impact:** The bulk of RCS's duplicated framework code was eliminated in one phase. 19/19 integration tests passed after migration.

### Phase 4: Post-Migration Cleanup

**Intent:** Remove dead code revealed by the migration.

Deleted the unused `generate-servicelocator.txt` template and empty migration directories from both projects. Updated code generators to use the `ProjectPascalCase` template field. Simplified the migration system to use only the plugin-based `LoadDiscoveredMigrations()`.

**Impact:** Cleaner codebase with no vestigial artifacts from the pre-library era.

### Phase 5: Server and Environment Extraction

**Intent:** Eliminate RCS's 25 duplicate `cmd/` files (~90% identical to rh-trex) by extracting reusable server infrastructure.

This was the largest and most complex phase. A detailed diff of all 25 files revealed 13 exact duplicates, 8 near-duplicates (differing only by service name strings), and 4 truly project-specific files.

The extraction created several new packages:
- `pkg/server/` — HTTP server interface, API server builder, route and controller registries, health check and metrics servers, logging middleware, Prometheus metrics (with `sync.Once` to prevent duplicate registration panics)
- `pkg/environments/` — Environment types, framework logic, explicit `NewEnvironment()` constructor (replacing the `init()` singleton), and all four standard environment implementations (development, production, unit testing, integration testing)
- `pkg/registry/` — Service auto-discovery registry

Key technical challenges solved:
- The `init()` singleton in `framework.go` was replaced with an explicit constructor while maintaining backward compatibility via a global accessor
- `prometheus.MustRegister()` panics when called twice — wrapped in `sync.Once`
- CORS origins were parameterized with Red Hat defaults via `trex.GetCORSOrigins()`
- The `DB_FACTORY_MODE` environment variable was added to support both testcontainer (Docker) and external (Podman) Postgres for integration testing
- The OCM client package was consolidated — RCS's copy was byte-for-byte identical to rh-trex's

The clone command was deleted from both projects, as the code generator fully replaces its purpose.

**Impact:** RCS deleted its entire `server/` directory, all environment implementation files, its OCM client copy, and the clone command. Its `cmd/` directory went from 25 files to 12 thin wrappers.

### Phase 6: Plugin Consolidation

**Intent:** Share the `plugins/events/` and `plugins/generic/` infrastructure plugins from rh-trex rather than maintaining copies.

Previously these plugins couldn't be shared because they imported `cmd/trex/environments`, creating a dependency on rh-trex's binary-specific code. Phase 5's extraction of environment types to `pkg/environments/` removed this blocker. Both plugins were updated to import from `pkg/environments/` and remaining hardcoded test paths were parameterized.

**Impact:** RCS deleted its local `plugins/events/` and `plugins/generic/` directories entirely and imported them from rh-trex. The events table schema — framework infrastructure, not business logic — is now owned by rh-trex.

### Phase 7: Test Infrastructure Consolidation

**Intent:** Share test utilities (~25 helper methods, 3 mock files) instead of maintaining copies that drift.

Created `pkg/testutil/` with a `BaseHelper` struct containing generic test methods (JWT creation, ID generation, account factories, database operations, URL helpers) and a `mocks/` package (mock server timeout, JWK cert server mock, OCM authorization mock).

Each project's `test/helper.go` now embeds `BaseHelper` and adds only project-specific methods (API client creation, authenticated context setup, OpenAPI error handling).

Two bugs were fixed during this phase:
- A value receiver bug on `OCMAuthzValidatorMock.Reset()` — mutations were silently discarded
- A corrupted base64 JWT key — a single byte difference at position 845 caused `x509.ParsePKCS8PrivateKey` failures

**Impact:** RCS deleted `test/mocks/` and `test/support/`, removed ~25 duplicated methods from `test/helper.go`. Both projects now share identical JWT keys, database cleanup logic (dynamic `information_schema` discovery instead of hardcoded table lists), and mock implementations.

### Phase 8: Command Thinning

**Intent:** Reduce RCS's remaining 12 `cmd/` wrapper files (~644 lines) to the absolute minimum.

Created `pkg/cmd/` with reusable Cobra commands (`NewRootCommand`, `NewServeCommand`, `NewMigrateCommand`) and promoted the API server, route builder, and controller/healthcheck/metrics server factories into `pkg/server/` as high-level entry points (`NewDefaultAPIServer`, `BuildDefaultRoutes`, `NewDefaultControllersServer`, etc.).

The serve command accepts a `getSpecData` callback so each project provides its own OpenAPI spec bytes without coupling. All server constructors accept `*Env` as a parameter rather than calling the global accessor internally, keeping them testable.

**Impact:** RCS's `cmd/` directory went from 12 files (644 lines) to 3 files (~90 lines). The entire `server/` subdirectory, `servecmd/`, `migrate/`, environment type aliases, and the custom integration testing environment were eliminated. A pre-existing bug in `registration.go` (calling `ResetDB()` on the wrong receiver) was also fixed.

---

## By the Numbers

### RCS Reduction

| Metric | Before | After | Reduction |
|--------|--------|-------|-----------|
| Framework code (non-entity Go) | ~13,700 lines | ~3,430 lines | **75%** |
| `cmd/` files | 25 files | 3 files | **88%** |
| `cmd/` lines | ~1,800 lines | ~124 lines | **93%** |
| Duplicated `pkg/` framework packages | 15 packages | 0 | **100%** |
| Test mock files | 3 files | 0 (imported) | **100%** |
| Test helper duplicated methods | ~25 methods | 0 (inherited) | **100%** |

### What RCS Keeps Locally (60 Go files)

- **3 `cmd/` files** — `main.go`, `framework.go`, `framework_test.go`
- **24 plugin files** — 3 entities x 8 files each (model, handler, service, DAO, mock, migration, presenter, plugin)
- **14 generated OpenAPI files** — project-specific API client
- **13 test files** — helper, registration, factories, integration tests
- **6 other files** — OpenAPI embed, generated bindata, templates, generator script

### What RCS Imports from rh-trex

- `pkg/cmd/` — CLI commands
- `pkg/server/` — full HTTP server stack (API, metrics, healthcheck, controllers, logging)
- `pkg/environments/` — environment framework with 4 standard implementations
- `pkg/registry/` — service auto-discovery
- `pkg/testutil/` — test helpers and mocks
- `pkg/trex/` — runtime configuration
- `pkg/` framework — errors, config, auth, db, dao, controllers, services, handlers, API types, presenters
- `pkg/client/ocm/` — OCM client
- `plugins/events/` and `plugins/generic/` — infrastructure plugins

### rh-trex New Importable Packages

| Package | Files | Lines | Purpose |
|---------|-------|-------|---------|
| `pkg/server/` (+ `logging/`) | 14 | 803 | HTTP server infrastructure |
| `pkg/environments/` | 6 | 503 | Environment framework |
| `pkg/cmd/` | 3 | 122 | Reusable Cobra commands |
| `pkg/testutil/` (+ `mocks/`) | 4 | 409 | Test infrastructure |
| `pkg/trex/` | 1 | 89 | Runtime configuration |
| `pkg/registry/` | 1 | 36 | Service auto-discovery |
| **Total new** | **29** | **~1,962** | |

---

## Configuration Improvements

The refactoring made configuration significantly more straightforward for consumers:

**Service identity** is set once via `trex.Init()` in the environment package's `init()` function — service name, API base path, error href, metadata ID, and project root directory. Everything downstream (CORS headers, JWT public paths, metadata responses, error responses, log suppression, route prefixes) derives from this single configuration point.

**CORS origins** default to 17 Red Hat domains but can be overridden via `trex.Config.CORSOrigins`.

**Database test mode** is controlled by the `DB_FACTORY_MODE` environment variable — `"external"` for pre-started Postgres (Podman), default for testcontainers (Docker). No code changes needed to switch.

**A new consumer** needs to provide:
1. A `trex.Init()` call with 5 config values
2. Plugin imports in `main.go`
3. An OpenAPI spec embed
4. Entity plugins (generated via `go run ./scripts/generator.go --kind MyEntity`)

---

## Bugs Fixed Along the Way

Several latent bugs were discovered and fixed during the refactoring:

1. **`OCMAuthzValidatorMock.Reset()` value receiver** — Both projects had a `Reset()` method on a value receiver, meaning all mutations were silently discarded. Changed to pointer receiver.

2. **Corrupted base64 JWT key** — The inline base64 string for the test JWT private key had a single corrupted byte at position 845 (0x52 vs 0x51), causing `x509.ParsePKCS8PrivateKey` failures. Re-encoded from the source PEM file.

3. **`prometheus.MustRegister()` double-registration panic** — When both rh-trex and a consumer imported the metrics middleware, Prometheus metrics would be registered twice, causing a panic. Wrapped in `sync.Once`.

4. **`registration.go` wrong receiver** — RCS's test registration called `helper.DBFactory.ResetDB()` instead of `helper.ResetDB()`, hitting a `Default.ResetDB()` panic.

5. **`panicBody` `init()` timing** — The panic response body was constructed during `init()` before `trex.Init()` could set the service name. Changed to lazy construction.

---

## Future Value

### For New Services

Any new Red Hat microservice built on rh-trex gets a production-ready foundation by importing the library and running the entity generator. The minimum local code for a new service is ~10 files: `main.go`, `framework.go`, an OpenAPI spec, and one generated entity plugin. Everything else — authentication, authorization, database management, migration framework, health checks, metrics, structured logging, error handling, event-driven controllers — comes from the library.

### For Existing Consumers

When rh-trex improves its server infrastructure, logging, or test utilities, consumers get the improvements by updating their `go.mod` dependency. There is no manual porting or diffing against a template. Security patches to authentication or authorization logic propagate automatically.

### For the Generator

The code generator produces entities that wire into the library's plugin system with zero manual steps. Generated plugins are self-contained: each registers its own routes, controllers, migrations, presenters, and service locators via `init()` functions. The generator supports custom fields with configurable types and nullability.

### Deferred: Functional Options for Environment Customization

A Phase 9 design was proposed and reviewed: functional options on `NewDefaultEnvironment()` that would let consumers tweak environment behavior inline (e.g., change a dev flag, add a config override) without writing new types or files. The design was approved architecturally but deferred — no consumer needs it today, and it's a backward-compatible addition that can be added when the need arises.

---

## Coordination Model

The refactoring was coordinated between two independent Claude Code sessions — one working on rh-trex, one on RCS — communicating via a shared `trex_comms.md` file. Each phase followed a propose-review-implement-verify cycle: TRex proposed changes, RCS reviewed and answered design questions, TRex implemented and verified, then RCS consumed the changes and verified on their side. This relay model proved effective for managing cross-project architectural changes where both codebases need to stay buildable and tested at every step.
