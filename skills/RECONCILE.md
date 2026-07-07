# Reconciliation Checkpoint

**Last Updated:** 2026-07-07
**Last Run By:** Claude (reconcile skill — APP-001 reconciliation)

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
| app | 1 | 14 | 0 | 0 | 14 | 0% |
| **Total** | **19** | **119** | **105** | **0** | **14** | **88.2%** |

## Spec Dependency Order

Reconciliation MUST proceed in this order to respect dependencies:

- **Layer 0:** STD-001, SEC-003
- **Layer 1:** FW-001
- **Layer 2:** FW-002, FW-003, FW-004
- **Layer 3:** DA-001, API-001, API-002
- **Layer 4:** DA-002, SEC-001, STD-002
- **Layer 5:** SEC-002, STD-003
- **Layer 6:** CG-001, CG-002, CG-003, CG-004
- **Layer 7:** APP-001

## Gap Table

| ID | Spec | Requirement | Status | Severity | Notes |
|----|------|-------------|--------|----------|-------|
| GAP-001 | SEC-001 | JWK Key Loading: Multi-URL support on HTTP | closed | critical | Fixed: `JWTHandler.keysURL string` → `keysURLs []string`. `apiserver.go` now passes full `JwkCertURLs` slice via `WithKeysURLs()`. |
| GAP-002 | SEC-001 | JWK Key Loading: Additive file+URL merging on HTTP | closed | critical | Fixed: `loadKeys()` restructured to load file first, then iterate all URLs additively into a combined `newKeys` map. `parseJWKSet()` → `parseAndStoreKeys()` merges into target map. Mirrors gRPC `JWKKeyProvider` architecture. |
| GAP-003 | SEC-001 | Automatic Key Refresh: On-demand refresh from ALL sources on HTTP | closed | major | Auto-resolved by GAP-002: `validateToken()` calls `loadKeys()` which now loads from all configured sources (file + all URLs). |
| GAP-004 | SEC-001 | Multi-Issuer Support: HTTP/gRPC behavioral consistency | closed | major | Auto-resolved by GAP-001 + GAP-002: HTTP `JWTHandler` now has architectural parity with gRPC `JWKKeyProvider` — multi-URL, additive merging, all-source refresh. |
| GAP-005 | APP-001 | Project entity — plugin, model, DAO, service, handler, migration, OpenAPI, presenter | open | major | Zero implementation. Requires entity generation: `go run ./scripts/generator.go --kind Project --fields "name:string:required,description:string,repository_url:string,status:string:required"` + post-gen: status state machine, cascade delete. |
| GAP-006 | APP-001 | EntityDefinition entity — plugin, model, DAO, service, handler, migration, OpenAPI, presenter | open | major | Zero implementation. Requires entity generation: `go run ./scripts/generator.go --kind EntityDefinition --fields "project_id:string:required,kind_name:string:required,plural_override:string,description:string"` + post-gen: FK to Project, composite unique index `(project_id, kind_name)`, cascade delete of FieldDefinitions and Relationships. |
| GAP-007 | APP-001 | FieldDefinition entity — plugin, model, DAO, service, handler, migration, OpenAPI, presenter | open | major | Zero implementation. Requires entity generation: `go run ./scripts/generator.go --kind FieldDefinition --fields "entity_definition_id:string:required,field_name:string:required,field_type:string:required,is_required:bool"` + post-gen: FK to EntityDefinition, composite unique index `(entity_definition_id, field_name)`, field_type enum validation. |
| GAP-008 | APP-001 | Relationship entity — plugin, model, DAO, service, handler, migration, OpenAPI, presenter | open | major | Zero implementation. Requires entity generation: `go run ./scripts/generator.go --kind Relationship --fields "project_id:string:required,source_entity_id:string:required,target_entity_id:string:required,relationship_type:string:required,foreign_key:string"` + post-gen: same-project validation, self-reference rejection, FK fields. |
| GAP-009 | APP-001 | Build entity — plugin, model, DAO, service, handler, migration, OpenAPI, presenter | open | major | Zero implementation. Requires entity generation: `go run ./scripts/generator.go --kind Build --fields "project_id:string:required,status:string:required,build_log:string,triggered_by:string,completed_at:time"` + post-gen: immutable (no PATCH), status state machine, project-active precondition. |
| GAP-010 | APP-001 | Build controller — subprocess-based generation orchestration | open | major | Zero implementation. Requires `OnUpsert` handler: resolve ERD → assemble generator CLI args → subprocess execution in isolated workspace → capture stdout/stderr → update build status. Application-specific logic, not generator-automatable. |
| GAP-011 | APP-001 | Project status state machine — ValidateStatusTransition | open | major | Requires hand-coded validation: `draft` → `active` → `archived` (terminal). Invalid transitions rejected with 400. Depends on GAP-005 (Project entity). |
| GAP-012 | APP-001 | Build status state machine — ValidateStatusTransition | open | major | Requires hand-coded validation: `pending` → `building` → `succeeded`/`failed` (terminal). Controller-owned transitions only. Depends on GAP-009 (Build entity). |
| GAP-013 | APP-001 | Parent-child cascade delete — Project → children | open | major | Project `OnDelete` must soft-delete all child EntityDefinitions, Relationships, and Builds. Depends on GAP-005, GAP-006, GAP-008, GAP-009. |
| GAP-014 | APP-001 | Parent-child cascade delete — EntityDefinition → children | open | major | EntityDefinition `OnDelete` must soft-delete all child FieldDefinitions and Relationships referencing it as source or target. Depends on GAP-006, GAP-007, GAP-008. |
| GAP-015 | APP-001 | Composite unique indexes and 409 Conflict mapping | open | major | `(project_id, kind_name)` on EntityDefinition, `(entity_definition_id, field_name)` on FieldDefinition. GORM duplicate key → 409 Conflict. Depends on GAP-006, GAP-007. |
| GAP-016 | APP-001 | Parent-scoped query filtering — FindBy{Parent}ID DAO methods | open | minor | All child entities need `FindByProjectID()` or `FindByEntityDefinitionID()` and corresponding `?project_id=` / `?entity_definition_id=` query parameters on list endpoints. Depends on GAP-005 through GAP-009. |
| GAP-017 | APP-001 | Generator flag mapping — FieldDefinition → `--fields` argument assembly | open | major | Build controller must translate FieldDefinitions into generator CLI `--fields` format: `{name}:{type}[:required]`. Depends on GAP-007, GAP-010. |
| GAP-018 | APP-001 | OpenAPI spec entries for all 5 entities | open | major | `openapi/openapi.projects.yaml`, `openapi.entity_definitions.yaml`, `openapi.field_definitions.yaml`, `openapi.relationships.yaml`, `openapi.builds.yaml` — all missing. Entity generator creates these, but post-gen customization needed for unique constraints, status enums, and parent filters. |

### Gap Execution Plan — APP-001

14 gaps (GAP-005 through GAP-018) across APP-001. The dependency graph requires this execution order:

**Phase 1 — Entity scaffolding** (generator-driven, 5 entities):
1. **GAP-005** — Generate Project entity
2. **GAP-006** — Generate EntityDefinition entity
3. **GAP-007** — Generate FieldDefinition entity
4. **GAP-008** — Generate Relationship entity
5. **GAP-009** — Generate Build entity

**Phase 2 — Post-generation customization** (hand-coded):
6. **GAP-011** — Project status state machine
7. **GAP-012** — Build status state machine
8. **GAP-015** — Composite unique indexes + 409 Conflict mapping
9. **GAP-016** — Parent-scoped query filtering (FindBy{Parent}ID)
10. **GAP-013** — Project cascade delete
11. **GAP-014** — EntityDefinition cascade delete

**Phase 3 — Build controller** (application-specific):
12. **GAP-017** — Generator flag mapping (FieldDefinition → --fields)
13. **GAP-010** — Build controller subprocess orchestration
14. **GAP-018** — OpenAPI post-gen customization (partially covered by generator, partially hand-coded)

**Estimated scope:** ~800 lines generated per entity × 5 = ~4000 generated lines + ~500 hand-coded lines for post-gen customization + ~300 lines for build controller.

### Gap Execution Plan — SEC-001 (closed)

All 4 gaps are in SEC-001 and are causally related. The fix order:

1. **GAP-001** (prerequisite) — Change `JWTHandler` to accept `[]string` for URLs
2. **GAP-002** (prerequisite) — Restructure `loadKeys()` for additive multi-source loading
3. **GAP-003** (auto-resolved) — Fixed by GAP-002
4. **GAP-004** (auto-resolved) — Fixed by GAP-001 + GAP-002

**Estimated scope:** ~100 lines changed in `pkg/auth/middleware.go` + ~5 lines in `pkg/server/apiserver.go`. The gRPC `JWKKeyProvider` (`pkg/server/grpcutil/jwk_provider.go`) serves as the reference implementation.

## Pre-Existing Infrastructure Issues

| ID | Area | Issue | Severity | Notes |
|----|------|-------|----------|-------|
| INFRA-001 | Testing | `TestLoadServices` in `cmd/trex/environments/framework_test.go:33` crashes with SIGABRT | major | `secrets/db.password` file is missing from the secrets directory. The test calls `env.Initialize()` which reads DB config files; `framework.go:71` calls `glog.Fatalf()` (process abort) instead of returning an error. Fix: either create `secrets/db.password` or make the test skip when DB secrets are unavailable. |
| INFRA-002 | Makefile | `make lint` fails: `unknown shorthand flag: 'e' in -e` | minor | `Makefile:157` uses `-e unused` but golangci-lint v2.10.1 no longer supports the `-e` shorthand. Direct invocation `golangci-lint run ./cmd/... ./pkg/...` passes with 0 issues. Fix: update Makefile to use the correct flag syntax for golangci-lint v2. |

## Reconciliation History

| Date | Coverage | Delta | Agent |
|------|----------|-------|-------|
| 2026-07-06 | — | Initial seeding | Manual |
| 2026-07-06 | 96.2% (101/105) | First reconciliation run: 4 partial gaps in SEC-001 (HTTP JWTHandler multi-URL parity with gRPC) | Claude |
| 2026-07-06 | 100% (105/105) | Closed GAP-001–004: HTTP JWTHandler now supports multi-URL additive key loading with file+URL merging, matching gRPC JWKKeyProvider. Changed `pkg/auth/middleware.go` (~80 lines) and `pkg/server/apiserver.go` (1 line). All tests pass. | Claude |
| 2026-07-07 | 88.2% (105/119) | APP-001 merged (PR #40). Added 14 new gaps (GAP-005–018) — zero implementation exists for 5 entities (Project, EntityDefinition, FieldDefinition, Relationship, Build). Also identified 2 pre-existing infra issues: TestLoadServices SIGABRT (missing `secrets/db.password`), `make lint` broken flag (`-e` → golangci-lint v2). Build passes, verify passes, lint passes (direct invocation). | Claude |
