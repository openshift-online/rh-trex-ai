# Reconciliation Checkpoint

**Last Updated:** 2026-08-04
**Last Run By:** Codex (reconcile skill — no-auth TUI/server security alignment)

---

## Coverage Summary

| Domain | Specs | Requirements | Covered | Partial | Missing | Coverage |
|--------|-------|-------------|---------|---------|---------|----------|
| framework | 4 | 24 | 24 | 0 | 0 | 100% |
| api | 2 | 20 | 20 | 0 | 0 | 100% |
| data | 2 | 14 | 13 | 1 | 0 | 92.9% |
| security | 3 | 17 | 17 | 0 | 0 | 100% |
| codegen | 6 | 89 | 79 | 9 | 1 | 88.8% |
| standards | 4 | 30 | 30 | 0 | 0 | 100% |
| **Total** | **21** | **194** | **183** | **10** | **1** | **94.3%** |

## Spec Dependency Order

Reconciliation MUST proceed in this order to respect dependencies:

- **Layer 0:** STD-001, STD-004, SEC-003
- **Layer 1:** FW-001
- **Layer 2:** FW-002, FW-003, FW-004
- **Layer 3:** DA-001, API-001, API-002
- **Layer 4:** DA-002, SEC-001, STD-002
- **Layer 5:** SEC-002, STD-003
- **Layer 6:** CG-001, CG-005
- **Layer 7:** CG-002, CG-003, CG-004, CG-006

## Gap Table

| ID | Spec | Requirement | Status | Severity | Notes |
|----|------|-------------|--------|----------|-------|
| GAP-001 | SEC-001 | JWK Key Loading: Multi-URL support on HTTP | closed | critical | Fixed: `JWTHandler.keysURL string` → `keysURLs []string`. `apiserver.go` now passes full `JwkCertURLs` slice via `WithKeysURLs()`. |
| GAP-002 | SEC-001 | JWK Key Loading: Additive file+URL merging on HTTP | closed | critical | Fixed: `loadKeys()` restructured to load file first, then iterate all URLs additively into a combined `newKeys` map. `parseJWKSet()` → `parseAndStoreKeys()` merges into target map. Mirrors gRPC `JWKKeyProvider` architecture. |
| GAP-003 | SEC-001 | Automatic Key Refresh: On-demand refresh from ALL sources on HTTP | closed | major | Auto-resolved by GAP-002: `validateToken()` calls `loadKeys()` which now loads from all configured sources (file + all URLs). |
| GAP-004 | SEC-001 | Multi-Issuer Support: HTTP/gRPC behavioral consistency | closed | major | Auto-resolved by GAP-001 + GAP-002: HTTP `JWTHandler` now has architectural parity with gRPC `JWKKeyProvider` — multi-URL, additive merging, all-source refresh. |
| GAP-005 | DA-002 | Advisory Lock for Migration Concurrency | partial | major | A reusable `db.Migrations` advisory-lock type exists in `pkg/db/advisory_locks.go`, but `pkg/db/migrations.go` still invokes gormigrate without using it. |
| GAP-006 | API-001 | OpenAPI Specification Compliance | closed | major | All registered dinosaur, fossil, and scientist CRUD methods, including DELETE, are documented in the split OpenAPI files and generated clients. |
| GAP-007 | API-001 | Stable Operation Identity | closed | major | The resolved root document has 15 unique semantic IDs (`list`, `create`, `get`, `update`, and `delete` for each entity), and all generated consumers compile against the migration. |
| GAP-008 | API-001 | Canonical OpenAPI Completeness | closed | major | The fully resolved root document matches every registered application route and method; generated embedded and Go-client specifications were regenerated from it. |
| GAP-033 | API-001 | Automated Route-Spec Parity | closed | major | `cmd/trex/route_openapi_parity_test.go` resolves split path-item references and compares normalized method/path sets against the discovered Gorilla router. |
| GAP-009 | CG-001 | Generated Operation Identity | closed | minor | Both entity OpenAPI templates emit semantic IDs and complete DELETE contracts; `entity_openapi_template_test.go` renders and resolves a synthetic entity to assert all five operations. |
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
| GAP-020 | CG-005 | Canonical OpenAPI Front End | closed | minor | `scripts/openapi-ir` is the shared normalized front end consumed by CLI, SDK, console, and TUI modules. |
| GAP-021 | CG-005 | Reference Resolution | closed | minor | The loader resolves split local references, preserves recursive schema identity, and diagnoses unresolved and cyclic non-schema references. |
| GAP-022 | CG-005 | Operation Identity | closed | minor | The IR rejects missing and duplicate IDs with source diagnostics; all current documented operations declare unique IDs. |
| GAP-023 | CG-005 | Operation Fidelity | closed | minor | Operations retain ordered routes, all input locations, serialization, request/response content, metadata, servers, and inherit/none/override security states. |
| GAP-024 | CG-005 | Schema Fidelity | closed | minor | Canonical schema nodes retain composition, constraints, access modes, nullability, discriminator data, defaults, examples, and reference identity. |
| GAP-025 | CG-005 | Usage-Based Schema Roles | closed | minor | Request, response, list-item, error, parameter, and event roles are derived from operation usage rather than names. |
| GAP-026 | CG-005 | Resource View Graph | closed | minor | Multi-scope collection/item views are distinct graph nodes and may share schema identities. |
| GAP-027 | CG-005 | Relationship Semantics | closed | minor | Link Objects and conservative inferred containment retain endpoints, full standard runtime-expression mappings, and explicit/inferred provenance; ambiguous inference remains disconnected. |
| GAP-028 | CG-005 | Operation-Derived Capabilities | closed | minor | Canonical capabilities represent only actual CRUD, action, and streaming operations. |
| GAP-029 | CG-005 | Extension Preservation | closed | minor | Document, path, operation, parameter, schema, and property extensions retain values and source locations. |
| GAP-030 | CG-005 | Deterministic Normalization | closed | minor | Repeated normalization and all target generation are byte-stable in tests. |
| GAP-031 | CG-005 | Actionable Diagnostics | closed | minor | Validation errors include source files, JSON Pointers, and operation/schema context before rendering begins. |
| GAP-032 | CG-005 | Loader Conformance Fixtures | closed | minor | Tracked fixtures cover single/split documents, recursion, unresolved references, invalid cycles, and root-boundary rejection. |
| GAP-034 | CG-005 | Bounded Reference Resolution | closed | minor | References are canonicalized and constrained by allowed roots, including symlink, traversal, absolute-path, and URI checks. |
| GAP-035 | CG-005 | Safe Target Projection | closed | minor | Shared identifier/path validation, safe joins, target escaping, and adversarial projection tests prevent output-root and interpolation escapes. |
| GAP-036 | CG-005 | Atomic Contract Evolution | closed | minor | `make test-generators` compiles and tests the IR and all four nested consumer modules; unit CI invokes that target. |
| GAP-041 | CG-005 | Pre-Migration Characterization Gate | closed | minor | Repository and shared-fixture characterization tests plus ignored pre-implementation SHA manifests prove legacy-compatible outcomes across parser migration. |
| GAP-042 | CG-005 | Repository OpenAPI Generation Gate | closed | minor | CLI, SDK, console, and TUI consumers generate the real split-file repository spec into temporary roots and run target acceptance without a database or API service. |
| GAP-037 | CG-005 | Operation and Security Conformance Fixtures | closed | minor | Fixtures assert flat/scoped operations, actions, streams, serialization, and inherited/none/override security. |
| GAP-038 | CG-005 | Schema and Role Conformance Fixtures | closed | minor | Fixtures assert recursive/composed schema semantics and all required usage roles without helper-resource promotion. |
| GAP-039 | CG-005 | Resource View and Metadata Conformance Fixtures | closed | minor | Fixtures assert multi-scope views, explicit and inferred relationships, ambiguity handling, parameter mappings, and extensions. |
| GAP-040 | CG-005 | Consumer Fixture Conformance | closed | minor | All four consumer suites load the shared fixture through the canonical IR; the TUI asserts supported operations, paths, relationships, security, and its required diagnostic for the fixture's unsupported OAuth operations. |
| GAP-055 | CG-006 | Canonical IR Consumption | closed | minor | `scripts/tui-generator` loads only `scripts/openapi-ir` and projects its normalized document; no independent YAML traversal exists. |
| GAP-056 | CG-006 | Descriptor-Driven Generic Runtime | closed | minor | OpenAPI resources project to stable descriptors consumed by one resource-agnostic Bubble Tea model with no entity-specific tables or clients. |
| GAP-057 | CG-006 | Integrated Service Subcommand | closed | major | `cmd/trex` registers `pkg/cmd.NewTUICommand` with the embedded `data/generated/tui` descriptor; `binary` and `install` regenerate it before compiling the single primary executable, and the command constructs the shared model directly with no child executable or wrapper. |
| GAP-078 | CG-006 | Full-Screen Application Shell | closed | major | `Shell.Render` exclusively owns the header, conditional command bar, framed semantic page, breadcrumb, contextual hints, modal overlay, and final-row alert rail; page transitions replace content without remounting chrome. |
| GAP-079 | CG-006 | Service-Neutral Header and Semantic Theme | closed | minor | The centralized theme explicitly applies primary headers, selection-accent unselected text, and black selected text on that exact same accent background instead of inheriting Bubbles' pre-set foreground. |
| GAP-093 | CG-006 | Contextual Header Shortcut Palette | closed | minor | `ShortcutPalette` measures one shared key-token width, pads variable-width tokens, gives every column the same display-cell width, and keeps every Action at the same relative offset in the terminal-right palette. |
| GAP-080 | CG-006 | Centralized Responsive Layout | closed | major | `CalculateShellLayout` continuously clamps dimensions, preserves the fixed alert and breadcrumb rows, allocates three prompt rows whenever available, and returns all three rows to the page when the prompt closes. |
| GAP-081 | CG-006 | Reusable Presentation Component Architecture | closed | major | The single runtime and all component tests now live in `pkg/tui`; generation emits only the service-specific descriptor package and no runtime copies. |
| GAP-082 | CG-006 | Unified Page Contract | closed | major | The initial resource catalog is a synthetic semantic collection page; it uses the same `Page` lifecycle and persistent shell as collection, detail, stream, and state pages. |
| GAP-083 | CG-006 | Shared Resource Table Page | closed | minor | Every catalog and descriptor table consumes the corrected shared style, making selected cells legible while unselected text exactly matches the theme's selected-row background without page-specific overrides. |
| GAP-094 | CG-006 | Shared Breadcrumb Trail | closed | minor | One shared component renders sanitized lowercased navigation frames as padded `<segment>` badges, differentiates ancestors from the active badge, and elides only complete oldest ancestors while retaining the active location. |
| GAP-092 | CG-006 | Content-Aware Column Sizing and Horizontal Overflow | closed | minor | Centralized display-cell sizing includes the protected left-prefix sort decoration, with a narrow-column regression proving both direction markers survive ellipsis truncation. |
| GAP-084 | CG-006 | Shared Detail and Stream Pages | closed | minor | `DetailStreamComponent` presents ACP-style right-aligned dim keys, two-cell gaps, aligned wrapped bright-white values, resize reflow, and sanitized indented raw JSON with lossless semantic syntax highlighting. |
| GAP-085 | CG-006 | Command, Filter, and Help Chrome | closed | minor | One fully bordered three-row prompt renders `🦖>` or `🦕/` without a widget-owned duplicate prefix; resource suggestions update inline, cycle deterministically, and accept through Tab, Right, or Ctrl+F while filters retain history. |
| GAP-086 | CG-006 | Single Keybinding and Hint Registry | closed | major | `KeyRegistry` owns contextual `<r> raw` presentation and dispatch, suppresses it without a selected API object, and reserves `r` against generated operation hotkeys. |
| GAP-087 | CG-006 | Consistent Alert and Error Rail | closed | major | Foreground API failures open a compact TRex-aware error summary while retaining complete redacted structured details; the fixed rail persists and background refresh failures remain non-disruptive. |
| GAP-088 | CG-006 | Shared Dialog Host and Dialog Primitives | closed | major | The shared modal host owns compact Close-default error summaries and bounded scrollable details, with centralized buttons, danger styling, resize behavior, and deterministic focus/back transitions. |
| GAP-089 | CG-006 | Schema-Driven Form Dialog | closed | major | Forms remain visibly locked until response success; failures preserve exact values and focus, return confirmed actions to editing, and require confirmation again before retry. |
| GAP-090 | CG-006 | Refresh and Stale-Data Lifecycle | closed | major | The generated `--refresh-interval` defaults to five seconds and accepts zero; active readable frames poll without overlap, streams/hidden frames are excluded, late results are ignored, post-action refresh is immediate, and stale/error/selection/last-success state is preserved and recovered. |
| GAP-091 | CG-006 | Presentation Component Conformance Gate | closed | Root tests now exercise the shared runtime in place and command tests prove descriptor parsing, direct model construction, established option propagation, root-help discovery, clean error propagation, and no wrapper process. |
| GAP-058 | CG-006 | Resource View Graph Projection | closed | minor | Descriptors retain global/scoped views, explicit and inferred edge provenance, explicit precedence, and diagnostics for ambiguous disconnected views. |
| GAP-059 | CG-006 | Multi-Parent Views and Navigation Stack | closed | minor | Runtime frames preserve the actual incoming edge, selected identity, bindings, and parent-specific selection across push/pop navigation. |
| GAP-060 | CG-006 | Deterministic Path-Parameter Binding | closed | major | The shared action-candidate resolver evaluates the same navigable same-schema collection-to-item edge plan used by navigation and carries selected-row values into item forms and requests. |
| GAP-061 | CG-006 | Typed Resource Presentation Extension | closed | minor | The grammar validates and preserves presentation metadata plus schema type/format; runtime priority now controls deterministic compression resistance without reordering or making any declared column inaccessible. |
| GAP-062 | CG-006 | Deterministic Presentation Defaults | closed | minor | Metadata-free resources derive stable labels, identity, readable columns, priority order, and sorting from normalized schemas. |
| GAP-063 | CG-006 | Typed Operation Presentation Metadata | closed | major | Projection validates hotkey conflicts across deduplicated collection and same-schema highlighted-item actions after final relationship precedence, with source-located cross-view conflict coverage. |
| GAP-064 | CG-006 | Resource Switching, Tables, Filtering, and Detail | closed | minor | The home catalog uses the simple `Resources` title and one full-table-width resource-name column containing only unscoped collection views; scoped views remain available through bound parent relationships and the contextual switcher. |
| GAP-065 | CG-006 | Capability-Driven Operations | closed | major | One deterministic action-candidate path supplies chooser rows, generated hotkeys, forms, and execution from collection operations plus documented same-schema item operations for the current highlighted row. |
| GAP-066 | CG-006 | Exact HTTP Request Construction | closed | major | `teatest` now proves a highlighted `dinosaur/7` update omits the bound ID input and sends the documented PATCH with `/dinosaurs/dinosaur%2F7` and the exact JSON body. |
| GAP-067 | CG-006 | Operation Security and Credential Safety | closed | major | Inherit/none/override and optional anonymous alternatives are preserved; supplied tokens use declared schemes and cannot cross origins without explicit trust, while an absent token is sent as an anonymous request for the server to authorize, including `run-no-auth`. |
| GAP-068 | CG-006 | Terminal-Safe Rendering | closed | critical | Tables, details, breadcrumbs, errors, streams, labels, and statuses pass through idempotent sanitizers covering CSI, OSC, DCS, string controls, C0/C1, DEL, layout controls, and framework markup. |
| GAP-069 | CG-006 | Actionable Projection Diagnostics | closed | minor | Projection aggregates safe failures with file, JSON Pointer, operation/view, and field context before installing any output. |
| GAP-070 | CG-006 | Repository Generation Workflow | closed | `make generate-tui` atomically owns `data/generated/tui`, normal `make generate` invokes it, and `generate-all` plus generator CI include it. |
| GAP-071 | CG-006 | Graph Conformance Gate | closed | minor | TUI fixtures assert flat/global and multiply scoped views, two explicit parents, explicit-over-inferred precedence, collection-item inference, and ambiguous disconnection. |
| GAP-072 | CG-006 | Parameter-Binding and Request Gate | closed | major | `httptest` cases exercise standard Link expressions, inherited frames, selected rows, multi-scope routes, styles, collisions, validation, exact bodies/headers/auth, and failure without a request. |
| GAP-073 | CG-006 | Capability Conformance Gate | closed | minor | A list/update/stream-only fixture and runtime chooser assertions prove documented partial capabilities are retained without inventing create/get/delete controls. |
| GAP-074 | CG-006 | Runtime Navigation Gate | closed | minor | Generated-runtime acceptance opens compact foreground errors, navigates complete scrollable details, restores source state, and corrects retained failed-action forms before successful retry. |
| GAP-075 | CG-006 | Terminal Injection Gate | closed | critical | Unit and `teatest` suites inject all specified terminal-control classes through table, detail, breadcrumb, error, and stream contexts and assert safe, idempotent output. |
| GAP-076 | CG-006 | Deterministic Generation Gate | closed | The two-run SHA-256 test asserts the exact descriptor-only tree (`descriptor.go`, `descriptor.json`, and ownership marker), stable modes and contents, and no host paths. |
| GAP-077 | CG-006 | Repository OpenAPI Acceptance Gate | closed | The real repository spec generates an isolated descriptor package that is compiled with the shared command/runtime; acceptance asserts root and TUI help, established flags, and absence of standalone-module output. |
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

1. **CLI operation fidelity:** GAP-010, GAP-012, and GAP-043 — project arbitrary capabilities and scopes and exercise exact requests against a mock server.
2. **SDK operation/schema fidelity:** GAP-013, GAP-015, GAP-016, and GAP-044 — render arbitrary scoped/action/stream operations and behavior-test all languages.
3. **Console view fidelity:** GAP-017, GAP-019, and GAP-045 — project scoped views/actions and component-test supported and absent capabilities.
4. **Independent data gap:** GAP-005 — connect the existing advisory-lock abstraction to migration execution.

API parity, CG-005, CG-006, STD-003, and STD-004 are fully covered. The remaining codegen gaps belong to CLI, SDK, and console fidelity, plus the independent migration-lock gap.

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
| 2026-08-04 | 93.8% (166/177) | Closed API/entity-generator parity and all 23 CG-006 requirements with a canonical-IR TUI graph, descriptor-driven runtime, exact HTTP/auth and Link semantics, safe atomic output, deterministic generation, and real-spec acceptance. Reclassified the existing-but-unused migration lock as partial. | Codex |
| 2026-08-04 | 86.4% (165/191) | Added 14 CG-006 requirements for a service-neutral full-screen shell, reusable pages/components, centralized theme/layout/keys, fixed bottom alert rail, modal forms/dialogs, refresh lifecycle, and presentation conformance gates; promoted operation metadata from reserved to typed and identified 7 partial and 7 missing presentation requirements. | Codex |
| 2026-08-04 | 85.4% (164/192) | Added content-aware Unicode display-cell sizing, centralized width bounds and compression, horizontal column scrolling, directional overflow counts, and arrow-key hints; identified the equal-width, inaccessible-column behavior as partial and reopened priority semantics. | Codex |
| 2026-08-04 | 86.5% (166/192) | Closed GAP-061 and GAP-092 with schema-aware Unicode column measurement, bounded priority compression, per-frame horizontal offsets, arrow-key scrolling, off-screen counts and hints, regenerated standalone output, and focused generator/runtime tests. | Codex |
| 2026-08-04 | 94.3% (181/192) | Closed GAP-063 and GAP-078–091 with a reusable semantic shell and page system, continuous layout, centralized theme/keys/alerts/modals, shared table/detail/stream/command/form components, safe confirmations, refresh/stale lifecycle, deterministic snapshots, and architecture duplication gates. | Codex |
| 2026-08-04 | 94.3% (182/193) | Added and closed GAP-093 with a k9s-style contextual top shortcut palette, single-registry header/help parity, six-row measured packing, responsive priority elision, mode-correct visibility, and no duplicate bottom strip. | Codex |
| 2026-08-04 | 94.3% (182/193) | Refined GAP-079, GAP-091, and GAP-093 so the connected server anchors the upper-left while an equal-column shortcut grid shares those rows at the terminal-right edge, with coordinate and alignment regression tests. | Codex |
| 2026-08-04 | 94.3% (182/193) | Refined GAP-079 and GAP-091 to vertically anchor service title at the top and server/status on the final two left-region rows, reserve flexible blank padding between them, and keep page identity in the frame and breadcrumb. | Codex |
| 2026-08-04 | 91.8% (178/194) | Added one shared breadcrumb requirement and reopened four presentation requirements for labeled header key/value rows, globally aligned shortcut Actions, a centered semantically segmented resource title, complete breadcrumb badges, and deterministic conformance coverage. | Codex |
| 2026-08-04 | 91.2% (177/194) | Refined GAP-089 and GAP-091 so shared action forms group required fields first and present only a visually emphasized field name with muted type and requiredness metadata. | Codex |
| 2026-08-04 | 94.3% (183/194) | Closed GAP-079, GAP-083, GAP-089, GAP-091, GAP-093, and GAP-094 with labeled service context, globally aligned shortcuts, centered semantic frame titles, active-preserving breadcrumb badges, required-first simplified forms, live rendering, generated acceptance, and architecture tests. | Codex |
| 2026-08-04 | 93.3% (181/194) | Reopened GAP-087 and GAP-091 because routine successful reads still create success alerts instead of updating content and refresh state silently. | Codex |
| 2026-08-04 | 94.3% (183/194) | Closed GAP-087 and GAP-091 by suppressing routine read success alerts while preserving explicit operation success through follow-up refreshes, with generated runtime coverage for list, detail, polling, and mutation flows. | Codex |
| 2026-08-04 | 91.8% (178/194) | Refined GAP-064, GAP-080, GAP-083, GAP-085, and GAP-091 for a complete three-row k9s-style query prompt, binding-aware inline resource completion, and live/persisted `</filter>` frame-title badges. | Codex |
| 2026-08-04 | 90.7% (176/194) | Refined GAP-085, GAP-088, GAP-089, and GAP-091 for dinosaur prompt icons without duplicate widget prompts, display-cell-aligned form fields, danger-styled inline validation, and a primary rightmost submit action. | Codex |
| 2026-08-04 | 94.3% (183/194) | Closed GAP-064, GAP-080, GAP-083, GAP-085, GAP-088, GAP-089, and GAP-091 with a fully bordered dinosaur query prompt, binding-aware completion, filter title badges, aligned shared forms, danger validation, and primary rightmost submission. | Codex |
| 2026-08-04 | 92.3% (179/194) | Refined GAP-064, GAP-074, GAP-088, and GAP-091 for a one-column unscoped home catalog and one compact ACP-style shared confirmation component; reopened the conflicting catalog and confirmation implementation behavior. | Codex |
| 2026-08-04 | 94.3% (183/194) | Closed GAP-064, GAP-074, GAP-088, and GAP-091 with a simple `Resources` home page whose one-column selections fill the table width, top-level-only rows, preserved scoped parent navigation, and one compact selected-button confirmation component with safe focus and no redundant chrome. | Codex |
| 2026-08-04 | 92.8% (180/194) | Refined GAP-084, GAP-086, and GAP-091 for contextual `r` raw-resource JSON inspection, reserved-key parity, safe rendering, no-request behavior, and state-preserving return. | Codex |
| 2026-08-04 | 94.3% (183/194) | Closed GAP-084, GAP-086, and GAP-091 with a shared sanitized raw JSON viewport, contextual reserved `r` shortcut, no-request inspection, and state-preserving return coverage. | Codex |
| 2026-08-04 | 92.8% (180/194) | Refined GAP-079, GAP-083, and GAP-091 for theme-relative table contrast: unselected text matching the selected-row background plus explicit black selected text overriding Bubbles defaults. | Codex |
| 2026-08-04 | 94.3% (183/194) | Closed GAP-079, GAP-083, and GAP-091 by explicitly overriding Bubbles table colors and asserting exact equality between the unselected foreground and selected background, with black selected text. | Codex |
| 2026-08-04 | 94.3% (183/194) | Refined and verified GAP-084 and GAP-091 so readable detail and raw JSON content use the semantic normal foreground rather than muted secondary text. | Codex |
| 2026-08-04 | 94.3% (183/194) | Refined and verified GAP-084 and GAP-091 with ACP-style dim right-aligned keys, two-cell value alignment and resize reflow, bright-white values, and lossless semantic JSON syntax highlighting. | Codex |
| 2026-08-04 | 92.3% (179/194) | Reopened GAP-074, GAP-087, GAP-088, and GAP-091 for automatic foreground API-error dialogs with complete safe structured details, scrolling/resizing, state-preserving dismissal, and non-disruptive background failures. | Codex |
| 2026-08-04 | 91.8% (178/194) | Refined GAP-087–091 so failed actions retain their editable form, values, and focus until success; confirmed failures return to editing and require confirmation again on retry. | Codex |
| 2026-08-04 | 94.3% (183/194) | Closed GAP-074, GAP-087–089, and GAP-091 with compact Close-default TRex errors, safe scrollable details, non-disruptive background failures, and confirmed/unconfirmed form correction retained until success. | Codex |
| 2026-08-04 | 91.2% (177/194) | Replaced the standalone TUI target with an integrated primary-binary `tui` contract and reopened six gaps for shared runtime ownership, descriptor-only generation, command wiring, workflow, and acceptance coverage. | Codex |
| 2026-08-04 | 94.3% (183/194) | Closed GAP-057, GAP-070, GAP-076, GAP-077, GAP-081, and GAP-091 by moving the reusable runtime to `pkg/tui`, generating only embedded in-module descriptors, registering the real primary-binary Cobra command, removing standalone output, wiring normal generation, and proving deterministic integrated builds and command behavior. | Codex |
| 2026-08-04 | 94.3% (183/194) | Kept CG-006 fully covered while making the standard `binary` and `install` targets regenerate the embedded TUI descriptor, so one top-level build produces the unified CLI/TUI executable without a separate TUI step. | Codex |
| 2026-08-05 | 94.3% (183/194) | Kept CG-006 fully covered by deferring missing-token enforcement to the configured server: `run-no-auth` now accepts anonymous TUI requests, supplied Bearer credentials retain origin protections, and authentication-enabled servers still surface their `401` response safely. | Codex |
