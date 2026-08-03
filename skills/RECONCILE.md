# Reconciliation Checkpoint

**Last Updated:** 2026-08-03
**Last Run By:** Codex (reconcile skill — hermetic test bootstrap)

---

## Coverage Summary

| Domain | Specs | Requirements | Covered | Partial | Missing | Coverage |
|--------|-------|-------------|---------|---------|---------|----------|
| framework | 4 | 24 | 24 | 0 | 0 | 100% |
| api | 2 | 20 | 16 | 2 | 2 | 80.0% |
| data | 2 | 14 | 13 | 0 | 1 | 92.9% |
| security | 3 | 17 | 17 | 0 | 0 | 100% |
| codegen | 5 | 49 | 13 | 11 | 25 | 26.5% |
| standards | 3 | 21 | 21 | 0 | 0 | 100% |
| **Total** | **19** | **145** | **104** | **13** | **28** | **71.7%** |

## Spec Dependency Order

Reconciliation MUST proceed in this order to respect dependencies:

- **Layer 0:** STD-001, SEC-003
- **Layer 1:** FW-001
- **Layer 2:** FW-002, FW-003, FW-004
- **Layer 3:** DA-001, API-001, API-002
- **Layer 4:** DA-002, SEC-001, STD-002
- **Layer 5:** SEC-002, STD-003
- **Layer 6:** CG-001, CG-005
- **Layer 7:** CG-002, CG-003, CG-004

## Gap Table

| ID | Spec | Requirement | Status | Severity | Notes |
|----|------|-------------|--------|----------|-------|
| GAP-001 | SEC-001 | JWK Key Loading: Multi-URL support on HTTP | closed | critical | Fixed: `JWTHandler.keysURL string` → `keysURLs []string`. `apiserver.go` now passes full `JwkCertURLs` slice via `WithKeysURLs()`. |
| GAP-002 | SEC-001 | JWK Key Loading: Additive file+URL merging on HTTP | closed | critical | Fixed: `loadKeys()` restructured to load file first, then iterate all URLs additively into a combined `newKeys` map. `parseJWKSet()` → `parseAndStoreKeys()` merges into target map. Mirrors gRPC `JWKKeyProvider` architecture. |
| GAP-003 | SEC-001 | Automatic Key Refresh: On-demand refresh from ALL sources on HTTP | closed | major | Auto-resolved by GAP-002: `validateToken()` calls `loadKeys()` which now loads from all configured sources (file + all URLs). |
| GAP-004 | SEC-001 | Multi-Issuer Support: HTTP/gRPC behavioral consistency | closed | major | Auto-resolved by GAP-001 + GAP-002: HTTP `JWTHandler` now has architectural parity with gRPC `JWKKeyProvider` — multi-URL, additive merging, all-source refresh. |
| GAP-005 | DA-002 | Advisory Lock for Migration Concurrency | missing | major | `pkg/db/migrations.go` invokes gormigrate directly. A `db.Migrations` advisory-lock type exists in `pkg/db/advisory_locks.go` but is not used by the migrate command. |
| GAP-006 | API-001 | OpenAPI Specification Compliance | partial | major | Entity handlers register DELETE, but `openapi/openapi.{dinosaurs,fossils,scientists}.yaml` define only GET, POST, and PATCH. |
| GAP-007 | API-001 | Stable Operation Identity | missing | major | No `operationId` appears in `openapi/`; cross-file operations therefore have no standard stable key. |
| GAP-008 | API-001 | Canonical OpenAPI Completeness | partial | major | Registered DELETE routes are absent from the resolved root document, and no route-to-spec parity test exists. |
| GAP-033 | API-001 | Automated Route-Spec Parity | missing | major | No automated test compares registered plugin application routes with the fully resolved root OpenAPI methods and normalized paths. |
| GAP-009 | CG-001 | Generated Operation Identity | missing | minor | `templates/generate-openapi.txt` does not emit `operationId` or generated DELETE documentation. |
| GAP-010 | CG-002 | OpenAPI-Driven Generation | partial | minor | The CLI discovers root schemas and assumes resources. It renders list/get/create commands only, rather than projecting exactly the operations present in OpenAPI. |
| GAP-011 | CG-002 | Shared IR Consumption | missing | minor | `scripts/cli-generator/main.go` unmarshals and interprets raw YAML independently. |
| GAP-012 | CG-002 | Scoped Path Fidelity | missing | minor | CLI resources retain one `PathSegment`; nested scope parameters and exact route templates are discarded. |
| GAP-043 | CG-002 | Generated CLI Acceptance Tests | missing | minor | `go test ./...` in `scripts/cli-generator` reports `[no test files]`; no test generates, builds, inspects, or executes the CLI against a mock server. |
| GAP-013 | CG-003 | Multi-Language Output | partial | minor | All three languages are generated, but methods are derived from inferred resources and selected heuristics rather than every documented operation. |
| GAP-014 | CG-003 | Shared IR Consumption | missing | minor | `scripts/sdk-generator/parser.go` owns a separate raw-YAML parser and schema-name discovery rules. |
| GAP-015 | CG-003 | Operation and Path Fidelity | partial | minor | SDK models reduce routes to an API prefix plus one path segment; arbitrary nested parameters and general operation inputs are not retained. |
| GAP-016 | CG-003 | OpenAPI Specification Compliance | partial | minor | Basic types, requiredness, and limited `allOf` are handled, but the parser does not preserve the complete schema semantics required for exact fidelity. |
| GAP-044 | CG-003 | Generated SDK Acceptance Tests | missing | minor | `go test ./...` in `scripts/sdk-generator` reports `[no test files]`; generated Go, Python, and TypeScript artifacts are not compiled and behavior-tested together. |
| GAP-017 | CG-004 | OpenShift Console Plugin Structure | partial | minor | Pages are generated from root schema names and forms are assumed, rather than being projected from displayable resource views and actual operations. |
| GAP-018 | CG-004 | Shared IR Consumption | missing | minor | `scripts/console-plugin-generator/main.go` duplicates raw-YAML and schema-to-resource interpretation. |
| GAP-019 | CG-004 | Scoped View and Action Fidelity | partial | minor | Patch/delete flags are detected for flat resources, but parent scopes, general actions, streams, and exact operation routes are not modeled. |
| GAP-045 | CG-004 | Generated Console Acceptance Tests | missing | minor | `go test ./...` in `scripts/console-plugin-generator` reports `[no test files]`; no generated plugin build, component, or API-client acceptance suite exists. |
| GAP-020 | CG-005 | Canonical OpenAPI Front End | missing | minor | No shared loader or normalized model package exists; the three consumers interpret OpenAPI separately. |
| GAP-021 | CG-005 | Reference Resolution | partial | minor | Generators manually follow a narrow root-schema-to-file `$ref` pattern; general local references, recursive schemas, and reference diagnostics are unsupported. |
| GAP-022 | CG-005 | Operation Identity | missing | minor | Parsers do not load or validate `operationId`, and current documents do not define it. |
| GAP-023 | CG-005 | Operation Fidelity | partial | minor | Some methods and paths are inspected, but ordered segments, complete parameter locations, responses, operation metadata, and the inherit/none/override security states are not normalized. |
| GAP-024 | CG-005 | Schema Fidelity | partial | minor | Existing parsers retain a subset of types and fields but lose composition, constraints, discriminators, write-only semantics, and other schema metadata. |
| GAP-025 | CG-005 | Usage-Based Schema Roles | missing | minor | Resource discovery is driven by schema names and suffix exclusions rather than request/response usage edges. |
| GAP-026 | CG-005 | Resource View Graph | missing | minor | Existing generator models allow one inferred collection path per schema and cannot retain multiple scoped views. |
| GAP-027 | CG-005 | Relationship Semantics | missing | minor | No generator reads OpenAPI Link Objects or records relationship parameter mappings and provenance. |
| GAP-028 | CG-005 | Operation-Derived Capabilities | partial | minor | SDK and console detect selected patch/delete/action cases, while CLI assumes a fixed command set; no canonical capability model exists. |
| GAP-029 | CG-005 | Extension Preservation | missing | minor | Raw `x-` values and their source scopes are discarded by the current parser structs. |
| GAP-030 | CG-005 | Deterministic Normalization | missing | minor | Some target models are sorted, but there is no canonical IR or deterministic normalized representation to verify. |
| GAP-031 | CG-005 | Actionable Diagnostics | partial | minor | File read and YAML errors are surfaced, but invalid nodes are often skipped and diagnostics lack JSON Pointers and operation/schema context. |
| GAP-032 | CG-005 | Loader Conformance Fixtures | missing | minor | No shared loader tests cover single/split documents, recursive references, invalid cycles, unresolved targets, or document-root escapes. |
| GAP-034 | CG-005 | Bounded Reference Resolution | missing | minor | Existing generators join `$ref` file values to the spec directory and call `os.ReadFile` without canonical-root, symlink, absolute-path, or URI-scheme enforcement. |
| GAP-035 | CG-005 | Safe Target Projection | missing | minor | No shared policy validates OpenAPI-derived identifiers and output paths or guarantees context-specific escaping across generated source and markup. |
| GAP-036 | CG-005 | Atomic Contract Evolution | missing | minor | The generators are separate Go modules, and current verification/CI does not compile and test all generator consumers as one required change set. |
| GAP-041 | CG-005 | Pre-Migration Characterization Gate | missing | minor | None of the three existing generator modules has black-box tests that can run unchanged before and after parser replacement; current server/client integration tests exercise a different generated Go client. |
| GAP-042 | CG-005 | Repository OpenAPI Generation Gate | missing | minor | CI tests the root module but does not generate and acceptance-test CLI, SDK, and console artifacts from the repository root OpenAPI document. |
| GAP-037 | CG-005 | Operation and Security Conformance Fixtures | missing | minor | No shared fixtures cover CRUD, nested routes, actions, streams, serialization, and all three operation-security states. |
| GAP-038 | CG-005 | Schema and Role Conformance Fixtures | missing | minor | No shared fixtures verify complete schema semantics or request, response, list-item, helper, and error roles. |
| GAP-039 | CG-005 | Resource View and Metadata Conformance Fixtures | missing | minor | No shared fixtures verify multi-scope views, links, ambiguity, relationship provenance, parameter mappings, or extension preservation. |
| GAP-040 | CG-005 | Consumer Fixture Conformance | missing | minor | CLI, SDK, and console generator directories have no tests consuming shared normalized fixture outputs. |

### Gap Execution Plan

Recommended implementation order for the OpenAPI IR work:

1. **API contract prerequisites:** GAP-006–009 and GAP-033 — complete generated CRUD documents, add stable operation IDs, and enforce route/spec parity before making operation identity mandatory at IR load time.
2. **Characterization before replacement:** GAP-041 plus the covered-behavior portions of GAP-043–045 — establish black-box assertions against the legacy generators before changing their parsers.
3. **Shared front end and safety boundary:** GAP-020–032 and GAP-034–040 — implement bounded reference loading, normalized nodes, safe projections, resource views, security states, links, diagnostics, and grouped conformance fixtures as one shared contract.
4. **SDK migration and acceptance:** GAP-013–016 and GAP-044 — exercise the broadest operation and schema surface first, compile every language, and verify requests against a mock server.
5. **CLI migration and acceptance:** GAP-010–012 and GAP-043 — generate commands from capabilities, retain every scope parameter, build the binary, and verify command behavior.
6. **Console migration and acceptance:** GAP-017–019 and GAP-045 — generate navigation, pages, and actions from resource views, then build and component-test the plugin.
7. **Repository generation gate:** GAP-042 — run all consumer acceptance suites against the real split-file root OpenAPI document in CI.
8. **TUI specification:** author and register CG-006 for generated TUI navigation, presentation metadata, actions, streams, authentication, and runtime boundaries using the canonical IR.
9. **TUI implementation:** reconcile CG-006 and generate the TUI from the same IR and conformance fixtures as the existing consumers.
10. **Independent data gap:** GAP-005 — add migration advisory locking outside the codegen workstream.

GAP-001–004 remain closed and require no further action.

## Reconciliation History

| Date | Coverage | Delta | Agent |
|------|----------|-------|-------|
| 2026-07-06 | — | Initial seeding | Manual |
| 2026-07-06 | 96.2% (101/105) | First reconciliation run: 4 partial gaps in SEC-001 (HTTP JWTHandler multi-URL parity with gRPC) | Claude |
| 2026-07-06 | 100% (105/105) | Closed GAP-001–004: HTTP JWTHandler now supports multi-URL additive key loading with file+URL merging, matching gRPC JWKKeyProvider. Changed `pkg/auth/middleware.go` (~80 lines) and `pkg/server/apiserver.go` (1 line). All tests pass. | Claude |
| 2026-08-03 | 78.5% (102/130) | Added CG-005 and operation/view requirements across API and codegen specs; found 13 partial and 15 missing requirements, including one previously unreconciled migration-lock gap. | Codex |
| 2026-08-03 | 73.9% (102/138) | Hardened CG-005 after review with bounded references, safe projections, explicit security inheritance, atomic consumer evolution, grouped conformance requirements, automated API parity, and an explicit CG-006 TUI milestone. | Codex |
| 2026-08-03 | 71.3% (102/143) | Verified all three generator modules have no tests and CI skips their nested modules; added pre-migration characterization, real-spec generation, and target artifact acceptance requirements. | Codex |
| 2026-08-03 | 71.7% (104/145) | Covered reproducible test tooling and hermetic unit-test credentials: test targets use module-pinned `gotestsum`, CI no longer installs `@latest`, and configuration tests own temporary password fixtures. | Codex |
