# Reconciliation Checkpoint

**Last Updated:** 2026-08-04
**Last Run By:** Codex (reconcile skill — OpenAPI IR and secure pull request automation merge)

---

## Coverage Summary

| Domain | Specs | Requirements | Covered | Partial | Missing | Coverage |
|--------|-------|-------------|---------|---------|---------|----------|
| framework | 4 | 24 | 24 | 0 | 0 | 100% |
| api | 2 | 20 | 16 | 3 | 1 | 80.0% |
| data | 2 | 14 | 13 | 0 | 1 | 92.9% |
| security | 3 | 17 | 17 | 0 | 0 | 100% |
| codegen | 5 | 49 | 38 | 9 | 2 | 77.6% |
| standards | 4 | 30 | 30 | 0 | 0 | 100% |
| **Total** | **20** | **154** | **138** | **12** | **4** | **89.6%** |

## Spec Dependency Order

Reconciliation MUST proceed in this order to respect dependencies:

- **Layer 0:** STD-001, STD-004, SEC-003
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
| GAP-007 | API-001 | Stable Operation Identity | partial | major | All 12 documented operations now have unique IDs, but their compatibility-preserving generated names do not yet match the semantic IDs prescribed by API-001, and undocumented DELETE routes still have no operation identity. |
| GAP-008 | API-001 | Canonical OpenAPI Completeness | partial | major | Registered DELETE routes are absent from the resolved root document, and no route-to-spec parity test exists. |
| GAP-033 | API-001 | Automated Route-Spec Parity | missing | major | No automated test compares registered plugin application routes with the fully resolved root OpenAPI methods and normalized paths. |
| GAP-009 | CG-001 | Generated Operation Identity | missing | minor | `templates/generate-openapi.txt` does not emit `operationId` or generated DELETE documentation. |
| GAP-010 | CG-002 | OpenAPI-Driven Generation | partial | minor | The CLI discovers root schemas and assumes resources. It renders list/get/create commands only, rather than projecting exactly the operations present in OpenAPI. |
| GAP-011 | CG-002 | Shared IR Consumption | closed | minor | The CLI imports `scripts/openapi-ir`, projects resource views from the normalized document, and contains no raw YAML parser. |
| GAP-012 | CG-002 | Scoped Path Fidelity | missing | minor | CLI resources retain one `PathSegment`; nested scope parameters and exact route templates are discarded. |
| GAP-043 | CG-002 | Generated CLI Acceptance Tests | partial | minor | Tests now generate, test, build, and execute the CLI and inspect its exact repository route, but do not yet exercise scoped/query/body/auth requests against a mock server. |
| GAP-013 | CG-003 | Multi-Language Output | partial | minor | All three languages are generated, but methods are derived from inferred resources and selected heuristics rather than every documented operation. |
| GAP-014 | CG-003 | Shared IR Consumption | closed | minor | SDK projection is backed solely by `scripts/openapi-ir`; independent YAML traversal and schema-name resource discovery were removed. |
| GAP-015 | CG-003 | Operation and Path Fidelity | partial | minor | SDK models reduce routes to an API prefix plus one path segment; arbitrary nested parameters and general operation inputs are not retained. |
| GAP-016 | CG-003 | OpenAPI Specification Compliance | partial | minor | Basic types, requiredness, and limited `allOf` are handled, but the parser does not preserve the complete schema semantics required for exact fidelity. |
| GAP-044 | CG-003 | Generated SDK Acceptance Tests | partial | minor | Go is compiled and behavior-tested, Python is compiled/imported, and TypeScript is type-checked with pinned tooling; cross-language scoped/query/body behavioral parity remains absent. |
| GAP-017 | CG-004 | OpenShift Console Plugin Structure | partial | minor | Pages are generated from root schema names and forms are assumed, rather than being projected from displayable resource views and actual operations. |
| GAP-018 | CG-004 | Shared IR Consumption | closed | minor | Console resources are projected from canonical IR resource views and schemas; the generator no longer parses raw YAML. |
| GAP-019 | CG-004 | Scoped View and Action Fidelity | partial | minor | Patch/delete flags are detected for flat resources, but parent scopes, general actions, streams, and exact operation routes are not modeled. |
| GAP-045 | CG-004 | Generated Console Acceptance Tests | partial | minor | The pinned lock graph is installed and the generated production plugin builds; scoped components, unsupported actions, and exact authenticated requests still lack runtime assertions. |
| GAP-020 | CG-005 | Canonical OpenAPI Front End | closed | minor | `scripts/openapi-ir` is the shared normalized front end consumed by CLI, SDK, and console modules. |
| GAP-021 | CG-005 | Reference Resolution | closed | minor | The loader resolves split local references, preserves recursive schema identity, and diagnoses unresolved and cyclic non-schema references. |
| GAP-022 | CG-005 | Operation Identity | closed | minor | The IR rejects missing and duplicate IDs with source diagnostics; all current documented operations declare unique IDs. |
| GAP-023 | CG-005 | Operation Fidelity | closed | minor | Operations retain ordered routes, all input locations, serialization, request/response content, metadata, servers, and inherit/none/override security states. |
| GAP-024 | CG-005 | Schema Fidelity | closed | minor | Canonical schema nodes retain composition, constraints, access modes, nullability, discriminator data, defaults, examples, and reference identity. |
| GAP-025 | CG-005 | Usage-Based Schema Roles | closed | minor | Request, response, list-item, error, parameter, and event roles are derived from operation usage rather than names. |
| GAP-026 | CG-005 | Resource View Graph | closed | minor | Multi-scope collection/item views are distinct graph nodes and may share schema identities. |
| GAP-027 | CG-005 | Relationship Semantics | closed | minor | Link Objects and conservative inferred containment retain target mappings and explicit/inferred provenance. |
| GAP-028 | CG-005 | Operation-Derived Capabilities | closed | minor | Canonical capabilities represent only actual CRUD, action, and streaming operations. |
| GAP-029 | CG-005 | Extension Preservation | closed | minor | Document, path, operation, parameter, schema, and property extensions retain values and source locations. |
| GAP-030 | CG-005 | Deterministic Normalization | closed | minor | Repeated normalization and all target generation are byte-stable in tests. |
| GAP-031 | CG-005 | Actionable Diagnostics | closed | minor | Validation errors include source files, JSON Pointers, and operation/schema context before rendering begins. |
| GAP-032 | CG-005 | Loader Conformance Fixtures | closed | minor | Tracked fixtures cover single/split documents, recursion, unresolved references, invalid cycles, and root-boundary rejection. |
| GAP-034 | CG-005 | Bounded Reference Resolution | closed | minor | References are canonicalized and constrained by allowed roots, including symlink, traversal, absolute-path, and URI checks. |
| GAP-035 | CG-005 | Safe Target Projection | closed | minor | Shared identifier/path validation, safe joins, target escaping, and adversarial projection tests prevent output-root and interpolation escapes. |
| GAP-036 | CG-005 | Atomic Contract Evolution | closed | minor | `make test-generators` compiles and tests the IR and all three nested consumer modules; unit CI invokes that target. |
| GAP-041 | CG-005 | Pre-Migration Characterization Gate | closed | minor | Repository and shared-fixture characterization tests plus ignored pre-implementation SHA manifests prove legacy-compatible outcomes across parser migration. |
| GAP-042 | CG-005 | Repository OpenAPI Generation Gate | closed | minor | Every consumer generates the real split-file repository spec into temporary roots and runs target acceptance without a database or API service. |
| GAP-037 | CG-005 | Operation and Security Conformance Fixtures | closed | minor | Fixtures assert flat/scoped operations, actions, streams, serialization, and inherited/none/override security. |
| GAP-038 | CG-005 | Schema and Role Conformance Fixtures | closed | minor | Fixtures assert recursive/composed schema semantics and all required usage roles without helper-resource promotion. |
| GAP-039 | CG-005 | Resource View and Metadata Conformance Fixtures | closed | minor | Fixtures assert multi-scope views, explicit and inferred relationships, ambiguity handling, parameter mappings, and extensions. |
| GAP-040 | CG-005 | Consumer Fixture Conformance | closed | minor | CLI, SDK, and console suites consume the shared fixture through the canonical IR and assert target projections. |
| GAP-046 | STD-004 | Exact Dependency Declarations | closed | major | Node acceptance and generated container images use exact tag+digest references; npm, TypeScript, and gotestsum versions are exact. |
| GAP-047 | STD-004 | Locked JavaScript Dependency Graph | closed | major | Console output includes a complete npm v3 lockfile and both acceptance and generated Docker builds use `npm ci --ignore-scripts`. |
| GAP-048 | STD-004 | Minimum Dependency Age | closed | major | The live checker admits only Go and npm versions at least 14 days old, including transitive lock entries and standalone tools. |
| GAP-049 | STD-004 | Audited Minimum-Age Exceptions | closed | major | The exact tuple allowlist validates mandatory reason and compensating-verification fields; the current list is empty. |
| GAP-050 | STD-004 | Dependency Policy Verification | closed | major | Nine offline policy tests cover parsing and boundary cases, while `ci-test-unit` runs the live gate. |
| GAP-051 | STD-004 | Actionable and Safe Metadata Access | closed | major | Metadata access is HTTPS-only with bounded retries/timeouts, no lifecycle execution, fail-closed behavior, and package-scoped diagnostics. |
| GAP-052 | STD-003 | Untrusted Pull Request Isolation | closed | major | Fixed: `.github/workflows/trex-pr-ci.yml` runs fork code on `pull_request` with only `contents: read`, no persisted checkout credentials, immutable action SHAs, draft-transition coverage, and no secrets. |
| GAP-053 | STD-003 | Privilege-Separated Review Comments | closed | major | Fixed: `.github/workflows/trex-auto-review.yml` consumes completed CI through `workflow_run`, verifies the current open PR/head SHA via GitHub APIs, treats patches as data, and creates or updates one marker-owned comment without checking out or executing fork content. |
| GAP-054 | STD-003 | Workflow Trust-Boundary Verification | closed | major | Fixed: `scripts/test_trex_review_workflows.py` validates triggers, exact permissions, immutable pins, valid expression operators, and prohibited privileged operations, with unsafe mutation cases. |

### Gap Execution Plan

Recommended implementation order for the remaining gaps:

1. **API and entity-generator parity:** GAP-006–009 and GAP-033 — document DELETE, emit semantic operation IDs from the entity generator, and enforce route/spec parity.
2. **CLI operation fidelity:** GAP-010, GAP-012, and GAP-043 — project arbitrary capabilities and scopes and exercise exact requests against a mock server.
3. **SDK operation/schema fidelity:** GAP-013, GAP-015, GAP-016, and GAP-044 — render arbitrary scoped/action/stream operations and behavior-test all languages.
4. **Console view fidelity:** GAP-017, GAP-019, and GAP-045 — project scoped views/actions and component-test supported and absent capabilities.
5. **TUI specification and implementation:** author CG-006, then consume the now-covered canonical IR and conformance fixtures.
6. **Independent data gap:** GAP-005 — add migration advisory locking outside the codegen workstream.

CG-005, STD-003, and STD-004 are fully covered. GAP-001–004 and GAP-052–054 remain closed and require no further action.

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
| 2026-08-03 | 71.7% (104/145) | Preserved test-tooling coverage while isolating pinned `gotestsum` from the root module graph so downstream consumers do not inherit development-only dependencies. | Codex |
| 2026-08-03 | 68.9% (104/151) | Added STD-004 for exact generator toolchain pins, locked npm graphs, a 14-day dependency cooldown, audited exceptions, and CI enforcement; identified six implementation gaps. | Codex |
| 2026-08-03 | 89.4% (135/151) | Fully reconciled CG-005 and STD-004: added the canonical bounded IR, migrated all three consumers, added shared and real-spec acceptance gates, proved deterministic generation with SHA-256 baselines, pinned the Node/npm graph, and enforced a 14-day dependency cooldown. | Codex |
| 2026-08-04 | 97.2% (105/108) | Added secure pull request execution, privilege-separated commenting, and workflow trust-boundary verification requirements; identified three implementation gaps. | Codex |
| 2026-08-04 | 100% (108/108) | Closed the three STD-003 workflow gaps with read-only PR CI, an API-only trusted commenter, immutable action pins, and offline trust-boundary mutation tests. | Codex |
| 2026-08-04 | 89.6% (138/154) | Merged the OpenAPI IR and secure pull request automation requirement sets, renumbered the CI gaps to preserve unique identifiers, and retained both implementations. | Codex |
