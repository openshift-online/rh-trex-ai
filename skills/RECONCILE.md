# Reconciliation Checkpoint

**Last Updated:** 2026-07-07
**Last Run By:** Claude (reconcile skill — APP-002 console UI implementation)

---

## Coverage Summary

| Domain | Specs | Requirements | Covered | Partial | Missing | Coverage |
|--------|-------|-------------|---------|---------|---------|----------|
| framework | 4 | 24 | 24 | 0 | 0 | 100% |
| api | 2 | 16 | 16 | 0 | 0 | 100% |
| data | 2 | 12 | 12 | 0 | 0 | 100% |
| security | 3 | 17 | 17 | 0 | 0 | 100% |
| codegen | 4 | 17 | 17 | 0 | 0 | 100% |
| standards | 3 | 19 | 19 | 0 | 0 | 100% |
| app | 2 | 27 | 26 | 0 | 1 | 96.3% |
| **Total** | **20** | **132** | **131** | **0** | **1** | **99.2%** |

## Spec Dependency Order

Reconciliation MUST proceed in this order to respect dependencies:

- **Layer 0:** STD-001, SEC-003
- **Layer 1:** FW-001
- **Layer 2:** FW-002, FW-003, FW-004
- **Layer 3:** DA-001, API-001, API-002
- **Layer 4:** DA-002, SEC-001, STD-002
- **Layer 5:** SEC-002, STD-003
- **Layer 6:** CG-001, CG-002, CG-003, CG-004
- **Layer 7:** APP-001, APP-002

## Gap Table

| ID | Spec | Requirement | Status | Severity | Notes |
|----|------|-------------|--------|----------|-------|
| GAP-001 | SEC-001 | JWK Key Loading: Multi-URL support on HTTP | closed | critical | Fixed: `JWTHandler.keysURL string` → `keysURLs []string`. `apiserver.go` now passes full `JwkCertURLs` slice via `WithKeysURLs()`. |
| GAP-002 | SEC-001 | JWK Key Loading: Additive file+URL merging on HTTP | closed | critical | Fixed: `loadKeys()` restructured to load file first, then iterate all URLs additively into a combined `newKeys` map. `parseJWKSet()` → `parseAndStoreKeys()` merges into target map. Mirrors gRPC `JWKKeyProvider` architecture. |
| GAP-003 | SEC-001 | Automatic Key Refresh: On-demand refresh from ALL sources on HTTP | closed | major | Auto-resolved by GAP-002: `validateToken()` calls `loadKeys()` which now loads from all configured sources (file + all URLs). |
| GAP-004 | SEC-001 | Multi-Issuer Support: HTTP/gRPC behavioral consistency | closed | major | Auto-resolved by GAP-001 + GAP-002: HTTP `JWTHandler` now has architectural parity with gRPC `JWKKeyProvider` — multi-URL, additive merging, all-source refresh. |
| GAP-005 | APP-001 | Project entity — plugin, model, DAO, service, handler, migration, OpenAPI, presenter | closed | major | Generated via `go run ./scripts/generator.go --kind Project`. Full plugin at `plugins/projects/`. OpenAPI at `openapi/openapi.projects.yaml`. |
| GAP-006 | APP-001 | EntityDefinition entity — plugin, model, DAO, service, handler, migration, OpenAPI, presenter | closed | major | Generated via entity generator. Full plugin at `plugins/entityDefinitions/`. `FindByProjectID` DAO method added. Composite unique index `(project_id, kind_name)` in migration. |
| GAP-007 | APP-001 | FieldDefinition entity — plugin, model, DAO, service, handler, migration, OpenAPI, presenter | closed | major | Generated via entity generator. Full plugin at `plugins/fieldDefinitions/`. `FindByEntityDefinitionID` DAO method added. Composite unique index `(entity_definition_id, field_name)` in migration. |
| GAP-008 | APP-001 | Relationship entity — plugin, model, DAO, service, handler, migration, OpenAPI, presenter | closed | major | Generated via entity generator. Full plugin at `plugins/relationships/`. `FindByProjectID` and `FindByEntityID` DAO methods added. |
| GAP-009 | APP-001 | Build entity — plugin, model, DAO, service, handler, migration, OpenAPI, presenter | closed | major | Generated via entity generator. Full plugin at `plugins/builds/`. `FindByProjectID` DAO method added. `CompletedAt` as `*time.Time`. |
| GAP-010 | APP-001 | Build controller — subprocess-based generation orchestration | closed | major | `OnUpsert` handler in `plugins/builds/service.go`. Checks pending → building → calls `executeBuild()` → succeeded/failed. `executeBuild()` fetches EntityDefinitions, assembles `--fields` from FieldDefinitions, runs `go run ./scripts/generator.go` as subprocess per entity. |
| GAP-011 | APP-001 | Project status state machine — ValidateStatusTransition | closed | major | `ValidateProjectStatusTransition()` in `plugins/projects/service.go`. Valid: `draft` → `active` → `archived` (terminal). Invalid transitions rejected with 400. |
| GAP-012 | APP-001 | Build status state machine — ValidateStatusTransition | closed | major | `ValidateBuildStatusTransition()` in `plugins/builds/service.go`. Valid: `pending` → `building` → `succeeded`/`failed` (terminal). Create enforces `pending` only. |
| GAP-013 | APP-001 | Parent-child cascade delete — Project → children | closed | major | Project `Delete()` cascades: EntityDefinitions (→ their FieldDefinitions), Relationships, Builds, then Project itself. Cross-plugin DAO injection via ServiceLocator. |
| GAP-014 | APP-001 | Parent-child cascade delete — EntityDefinition → children | closed | major | EntityDefinition `Delete()` cascades: FieldDefinitions via `FindByEntityDefinitionID`, Relationships via `FindByEntityID` (source OR target), then EntityDefinition itself. |
| GAP-015 | APP-001 | Composite unique indexes and 409 Conflict mapping | closed | major | `uniqueIndex:idx_entity_def_project_kind` on `(ProjectId, KindName)` in EntityDefinition migration. `uniqueIndex:idx_field_def_entity_name` on `(EntityDefinitionId, FieldName)` in FieldDefinition migration. |
| GAP-016 | APP-001 | Parent-scoped query filtering — FindBy{Parent}ID DAO methods | closed | minor | All 4 child entity handlers inject query parameter filters into `listArgs.Search` via TSL expressions. EntityDefinitions: `?project_id=`, FieldDefinitions: `?entity_definition_id=`, Relationships: `?project_id=`, Builds: `?project_id=`. |
| GAP-017 | APP-001 | Generator flag mapping — FieldDefinition → `--fields` argument assembly | closed | major | `executeBuild()` in `plugins/builds/service.go` assembles `--fields` from FieldDefinitions: `{FieldName}:{FieldType}[:required]` joined by commas. Handles `--plural` from `PluralOverride`. |
| GAP-018 | APP-001 | OpenAPI spec entries for all 5 entities | closed | major | All 5 OpenAPI YAML files generated and customized with query parameters: `openapi.projects.yaml`, `openapi.entityDefinitions.yaml`, `openapi.fieldDefinitions.yaml`, `openapi.relationships.yaml`, `openapi.builds.yaml`. `make generate` run to regenerate client. |
| GAP-019 | APP-002 | Generated CRUD for All Entities — run CG-004 generator | closed | major | Ran `go run . --spec ../../openapi/openapi.yaml --out /tmp/trex-console --name trex-console`. Generated 8 resources (Build, Dinosaur, EntityDefinition, FieldDefinition, Fossil, Project, Relationship, Scientist). Copied to `console/` directory. |
| GAP-020 | APP-002 | Project-Scoped Navigation — query parameter propagation | closed | major | Customized all list pages: `EntityDefinitionListPage` reads `?project_id=`, `BuildListPage` reads `?project_id=`, `RelationshipListPage` reads `?project_id=`, `FieldDefinitionListPage` reads `?entity_definition_id=`. Create pages pre-populate parent IDs from URL. `ProjectDetailsPage` links to scoped child lists. |
| GAP-021 | APP-002 | Build Workflow — trigger button, status polling, log viewer | closed | major | `BuildCreatePage` rewritten as "Trigger Build" with auto `status: 'pending'`. `BuildDetailsPage` has 3s `setInterval` polling for non-terminal statuses, auto-scroll dark monospace `<pre>` log viewer, inline `Spinner` for active builds. |
| GAP-022 | APP-002 | Status Visualization — color-coded badges | closed | minor | Created `StatusBadge.tsx` component using PatternFly `Label`. Project: `draft`=grey, `active`=green, `archived`=orange. Build: `pending`=grey, `building`=blue+Spinner, `succeeded`=green, `failed`=red. Used across all list and detail pages. |
| GAP-023 | APP-002 | Constrained Input Controls — dropdowns for enums | closed | minor | `FieldDefinitionCreatePage`: `FormSelect` for `field_type` (string, int, int64, bool, float, time), `Switch` for `is_required`. `RelationshipCreatePage`: `FormSelect` for `relationship_type` (4 options), entity pickers that load by `project_id` and display as `kind_name (ID...)` dropdowns with TextInput fallback. |
| GAP-024 | APP-002 | Standalone Deployment — dual API client | closed | major | Rewrote `api.ts`: `getBaseURL()` with 3-tier fallback (`window.__TREX_API_URL__` → `process.env.REACT_APP_API_URL` → console proxy). Dynamic `import('@openshift-console/dynamic-plugin-sdk')` with `window.fetch` fallback. Added `FilteredListOptions` with `projectId`/`entityDefinitionId`. Added `delete` methods for all entities. |
| GAP-025 | APP-002 | ProjectDashboard — landing page component | closed | minor | Created `ProjectDashboard.tsx`: 3-card gallery (Entity Definitions count, Relationships count, Latest Build status). Uses `StatusBadge` for project and build status. Parallel API calls via `Promise.all`. Route at `/trex-console/projects/:id/dashboard`. |
| GAP-026 | APP-002 | ERD Visualization (Phase 2) — interactive diagram | deferred | minor | New component: `ERDViewer.tsx` using `reactflow` or `elkjs`. Renders EntityDefinitions as boxes with fields, Relationships as labeled edges. Spec marks as SHOULD / Phase 2. Deferred — requires additional npm dependency. |

### Gap Execution Plan — APP-002

7 gaps (GAP-019 through GAP-025, excluding GAP-026 Phase 2). The dependency graph:

**Phase 1 — Generator scaffold** (CG-004):
1. **GAP-019** — Run console plugin generator to produce baseline CRUD pages

**Phase 2 — Post-generation customization** (hand-coded):
2. **GAP-020** — Project-scoped navigation (query parameter propagation)
3. **GAP-022** — Status badges (PatternFly Label components)
4. **GAP-023** — Constrained input controls (dropdowns, entity pickers)
5. **GAP-025** — ProjectDashboard component
6. **GAP-021** — Build workflow (trigger + polling + log viewer)

**Phase 3 — Architecture** (requires API client abstraction):
7. **GAP-024** — Standalone SPA mode (dual API client, build scripts)

**Deferred (Phase 2 of APP-002):**
- **GAP-026** — ERD Visualization (SHOULD, not SHALL)

### Gap Execution Plan — APP-001 (closed)

All 14 gaps (GAP-005 through GAP-018) closed. Entity generation + post-generation customization complete. Build passes, verify passes, 67 tests pass, lint clean.

### Gap Execution Plan — SEC-001 (closed)

All 4 gaps (GAP-001 through GAP-004) closed.

## Pre-Existing Infrastructure Issues

| ID | Area | Issue | Severity | Notes |
|----|------|-------|----------|-------|
| INFRA-001 | Testing | `TestLoadServices` in `cmd/trex/environments/framework_test.go:33` crashes with SIGABRT | closed | Fixed: created `secrets/db.password` placeholder file so `env.Initialize()` can read DB config files without aborting. |
| INFRA-002 | Makefile | `make lint` fails: `unknown shorthand flag: 'e' in -e` | closed | Fixed: removed `-e unused` flag from Makefile lint target. golangci-lint v2 uses `.golangci.yml` for configuration. |

## Reconciliation History

| Date | Coverage | Delta | Agent |
|------|----------|-------|-------|
| 2026-07-06 | — | Initial seeding | Manual |
| 2026-07-06 | 96.2% (101/105) | First reconciliation run: 4 partial gaps in SEC-001 (HTTP JWTHandler multi-URL parity with gRPC) | Claude |
| 2026-07-06 | 100% (105/105) | Closed GAP-001–004: HTTP JWTHandler now supports multi-URL additive key loading with file+URL merging, matching gRPC JWKKeyProvider. Changed `pkg/auth/middleware.go` (~80 lines) and `pkg/server/apiserver.go` (1 line). All tests pass. | Claude |
| 2026-07-07 | 88.2% (105/119) | APP-001 merged (PR #40). Added 14 new gaps (GAP-005–018) — zero implementation exists for 5 entities (Project, EntityDefinition, FieldDefinition, Relationship, Build). Also identified 2 pre-existing infra issues: TestLoadServices SIGABRT (missing `secrets/db.password`), `make lint` broken flag (`-e` → golangci-lint v2). Build passes, verify passes, lint passes (direct invocation). | Claude |
| 2026-07-07 | 90.2% (119/132) | Closed GAP-005–018 (all APP-001 gaps): generated 5 entities, added status state machines, cascade deletes, parent-scoped filtering, composite unique indexes, build controller with subprocess orchestration. Closed INFRA-001 and INFRA-002. Added APP-002 spec (console UI) with 8 new gaps (GAP-019–026). PR #41. `make binary` ✅, `make verify` ✅, `make test` (67) ✅, `make lint` (0 issues) ✅. | Claude |
| 2026-07-07 | 99.2% (131/132) | Closed GAP-019–025 (APP-002 console UI): ran CG-004 generator, customized all pages with project-scoped navigation, build workflow (trigger/polling/log), status badges, constrained inputs, standalone SPA mode, ProjectDashboard. Updated App.tsx (default→projects, dashboard route) and ResourceNav.tsx (reordered). GAP-026 (ERD viz) deferred as Phase 2. Go server: `make binary` ✅, `make verify` ✅, `make test` (67) ✅, `make lint` (0 issues) ✅. | Claude |
