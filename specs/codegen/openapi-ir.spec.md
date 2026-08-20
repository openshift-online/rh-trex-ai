# OpenAPI Intermediate Representation Specification

**Date:** 2026-08-03
**Status:** Active
**ID:** CG-005
**Related:** [REST Conventions](../api/rest-conventions.spec.md), [Testing Standards](../standards/testing.spec.md), [Dependency Supply Chain](../standards/dependency-supply-chain.spec.md), [CLI Generator](cli-generator.spec.md), [SDK Generator](sdk-generator.spec.md), [Console Plugin Generator](console-plugin-generator.spec.md), [TUI Generator](tui-generator.spec.md)
**Implements:** `scripts/openapi-ir/`

---

## Purpose

Define one canonical, operation-oriented intermediate representation (IR) of an OpenAPI document for all TRex code generators. The IR separates OpenAPI loading and interpretation from target-specific rendering, and represents scoped resources as a graph of API operations and views rather than forcing schemas into a single kind hierarchy.

## Conceptual Model

The normalized IR is a graph with the following implementation-independent node types:

| Node | Identity and responsibility |
|------|-----------------------------|
| Document | OpenAPI version, API metadata, servers, security schemes, and document extensions |
| Operation | Stable `operationId` plus the exact method, route, inputs, outputs, servers, declared security state, and operation metadata |
| Route Segment | One ordered literal or parameter segment; parameter segments point to normalized Parameter nodes |
| Parameter | Location, name, requiredness, style, explode behavior, and schema |
| Schema | Stable canonical reference plus structural and validation semantics |
| Schema Use | An edge from an operation input or output to a schema with a contextual role |
| Resource View | One collection or item exposure with a stable identity at a particular route and scope, with links to its supported operations and represented schemas |
| Relationship | A directed edge with stable source and target operation and resource-view identities, parameter bindings, and explicit-link or inferred-path provenance |
| Capability | A projection of an actual CRUD, action, or streaming operation available through a resource view |

Target generators MAY define additional presentation models, but those models are projections of this graph and are not alternative OpenAPI interpretations.

## Requirements

### Requirement: Canonical OpenAPI Front End

TRex generators SHALL obtain OpenAPI semantics through one shared loader and normalized IR. A generator MAY add target-specific projections after normalization, but SHALL NOT independently traverse raw OpenAPI YAML to rediscover operations, schemas, or relationships.

#### Scenario: Multiple consumers use one interpretation
- GIVEN the CLI, SDK, console, and TUI generators receive the same OpenAPI document
- WHEN each generator prepares its target-specific model
- THEN each SHALL consume the shared normalized IR
- AND operation, schema, and relationship discovery SHALL have identical semantics across the generators

### Requirement: Reference Resolution

The loader SHALL resolve local and relative-file `$ref` values across split OpenAPI documents before semantic normalization, subject to the configured reference boundary. It SHALL handle recursive schema references without unbounded traversal and SHALL reject unresolved or cyclic non-schema references with a diagnostic.

#### Scenario: Split document with recursive models
- GIVEN a root document that references entity path and schema files
- AND a schema that recursively references itself
- WHEN the document is loaded
- THEN all resolvable references SHALL point to canonical IR nodes
- AND the recursive schema SHALL be represented without infinite expansion

### Requirement: Bounded Reference Resolution

The loader SHALL restrict file reference resolution to one or more explicitly configured document roots, defaulting to the directory containing the root OpenAPI document. It SHALL canonicalize a target, including symbolic links, before reading it and SHALL reject a target outside every allowed root. Absolute filesystem references and non-file URI schemes SHALL be rejected unless the caller explicitly enables an additional trusted source.

#### Scenario: Traversal outside the document root
- GIVEN the document root is `/workspace/openapi`
- AND an OpenAPI node declares `$ref: ../../etc/passwd`
- WHEN the loader resolves the reference
- THEN it SHALL reject the reference before reading the target
- AND the diagnostic SHALL identify the referring OpenAPI node without including target-file contents

#### Scenario: Symbolic link escape
- GIVEN a file beneath the document root is a symbolic link to a file outside every allowed root
- WHEN a `$ref` targets that symbolic link
- THEN the loader SHALL reject the canonical target before reading it

### Requirement: Operation Identity

Each operation in the IR SHALL be keyed by its globally unique OpenAPI `operationId`. Missing or duplicate operation IDs SHALL be validation errors rather than being synthesized from schema names or URL segments.

#### Scenario: Duplicate operation ID
- GIVEN two operations declare `operationId: listAgents`
- WHEN normalization runs
- THEN normalization SHALL fail
- AND the diagnostic SHALL identify both operation source locations

### Requirement: Operation Fidelity

The IR SHALL retain each operation's HTTP method, ordered path segments, path and operation parameters, parameter serialization rules, request bodies, responses, content types, server overrides, security requirements, tags, deprecation state, summary, and description. It SHALL distinguish an omitted operation-level security field that inherits document security, an explicit empty `security: []` declaration that disables inherited security, and a non-empty operation override, including alternative requirements and OAuth scopes. Path parameters SHALL remain associated with their exact segment and SHALL NOT be reduced to a single resource ID.

#### Scenario: Multiply scoped operation
- GIVEN `GET /organizations/{organization_id}/projects/{project_id}/agents/{agent_id}/inbox`
- WHEN the operation is normalized
- THEN all three path parameters SHALL be retained in route order
- AND a consumer SHALL be able to construct the exact path without parsing the raw OpenAPI document

#### Scenario: Explicit authentication opt-out
- GIVEN document-level security requires Bearer authentication
- AND one operation omits its `security` field
- AND another operation declares `security: []`
- WHEN both operations are normalized
- THEN the first operation SHALL retain an inherited-security state
- AND the second operation SHALL retain an explicitly-unauthenticated state

### Requirement: Schema Fidelity

The IR SHALL retain schema composition, required fields, read-only and write-only fields, nullability, arrays, maps, enums, defaults, examples, descriptions, formats, constraints, and discriminator metadata. Schema references SHALL preserve identity even when schemas participate in `allOf`, `oneOf`, or `anyOf` composition.

#### Scenario: Request and response projections
- GIVEN a resource schema with a read-only `id`, a required `name`, and a nullable enum field
- WHEN target-specific request and response models are projected from the IR
- THEN the projection SHALL be able to distinguish all three field semantics without rereading OpenAPI

### Requirement: Usage-Based Schema Roles

The IR SHALL classify schema uses from their positions in operations, including request, response, list item, error, parameter, and event payload roles. It SHALL NOT classify a schema as a resource solely from its name or suffix.

#### Scenario: Helper schemas are not resources
- GIVEN schemas named `Agent`, `AgentList`, `AgentPatchRequest`, and `Error`
- AND only `Agent` is returned as the item of a collection operation
- WHEN normalization runs
- THEN `Agent` SHALL be associated with that resource view
- AND the other schemas SHALL retain their actual usage roles without becoming independent resources

### Requirement: Resource View Graph

The IR SHALL model every collection or item exposure as a distinct resource view connected to its operations and schemas. Each resource view SHALL have a unique, deterministic identity that distinguishes route, scope, and collection or item role without relying only on schema name. A schema MAY be exposed by multiple views at different paths or scopes, and the IR SHALL NOT require one canonical parent or one canonical collection path for a schema.

#### Scenario: One schema in global and scoped collections
- GIVEN `GET /inbox` and `GET /agents/{agent_id}/inbox` both return `MessageList`
- WHEN normalization runs
- THEN the IR SHALL contain two collection views of `Message`
- AND the scoped view SHALL retain `agent_id` as scope
- AND each view SHALL have a distinct stable identity
- AND neither view SHALL overwrite the other

### Requirement: Relationship Semantics

The IR SHALL represent standard OpenAPI Link Objects as directed operation relationships, including stable source and target operation identities, stable source and target resource-view identities when the operations belong to views, and the target parameter mapping values and runtime expressions needed by a consumer to construct a binding plan. An explicit relationship SHALL retain the exact source response and target capability selected by the Link rather than substituting another operation over the same schema.

The IR MAY infer a containment relationship from path structure only when exactly one parent item view and one child collection view are unambiguous. An inferred relationship SHALL identify the parent item-read operation and child collection-list operation that provide the navigable capabilities; it SHALL NOT select an update, delete, create, action, or streaming operation merely because that operation uses the same schema or path. Every inferred relationship SHALL record its inferred provenance and the structural parameter bindings or unsatisfied target parameters used to reach that conclusion.

#### Scenario: Explicit relationship takes precedence
- GIVEN a response Link Object targets `listAgentInbox` and maps `agent_id` from the source response
- AND path structure could suggest more than one parent
- WHEN normalization runs
- THEN the explicit link SHALL define the relationship edge
- AND the IR SHALL NOT invent a canonical parent from the ambiguous path structure

#### Scenario: Explicit relationship retains endpoints and bindings
- GIVEN the `getAgent` response defines a Link to `listAgentInbox`
- AND the Link maps target `agent_id` from `$response.body#/id`
- WHEN normalization runs
- THEN the relationship SHALL identify `getAgent` and its item view as its source
- AND it SHALL identify `listAgentInbox` and its collection view as its target
- AND it SHALL retain the `agent_id` runtime expression without interpreting it as a schema-name convention

#### Scenario: Inference selects navigation capabilities
- GIVEN an Agent item view supports get, update, delete, and action operations
- AND its nested Inbox collection view supports list and create operations
- WHEN an unambiguous containment relationship is inferred
- THEN its source operation SHALL be the Agent item-read capability
- AND its target operation SHALL be the Inbox collection-list capability
- AND normalization SHALL NOT depend on map iteration or operation declaration order

### Requirement: Operation-Derived Capabilities

The IR SHALL derive capabilities from documented operations rather than assuming every resource supports CRUD. It SHALL represent non-CRUD actions, streaming responses, and multiple operations over the same schema without coercing them into CRUD methods.

#### Scenario: Read-only resource with action
- GIVEN an API exposes list and get operations plus `POST /agents/{agent_id}:interrupt`
- AND it exposes no create, update, or delete operation
- WHEN normalization runs
- THEN the view SHALL advertise only its documented capabilities
- AND interrupt SHALL remain a distinct action operation

### Requirement: Extension Preservation

The IR SHALL preserve `x-` extension values and their source locations at document, path, operation, parameter, schema, and property scopes. Unknown extensions SHALL remain available to target-specific projections without changing the canonical core model.

#### Scenario: TUI presentation metadata
- GIVEN an operation contains an `x-trex-tui` extension
- WHEN normalization runs
- THEN the extension value and its operation association SHALL be available to a TUI projection
- AND generators that do not understand the extension SHALL still normalize the document successfully

### Requirement: Safe Target Projection

Every target projection SHALL treat all OpenAPI-derived values, including extension values, descriptions, examples, names, identifiers, and paths, as untrusted input. A projection SHALL validate values used as identifiers or file paths, SHALL contextually escape values interpolated into source code or markup, SHALL keep generated files within its configured output root, and SHALL NOT evaluate an OpenAPI-derived value as template syntax or a shell command.

#### Scenario: Source-code interpolation
- GIVEN an extension value contains quotes, template delimiters, markup, or shell metacharacters
- WHEN a generator includes that value in target output
- THEN it SHALL validate or escape the value for that exact target context
- AND the emitted value SHALL remain data without breaking the surrounding target syntax or introducing executable directives

#### Scenario: Generated path traversal
- GIVEN an OpenAPI-derived name would produce a path outside the configured output root
- WHEN a target projection computes its output paths
- THEN generation SHALL fail with a diagnostic before writing any file outside the output root

### Requirement: Atomic Contract Evolution

The canonical IR SHALL be an internal, unversioned compile-time contract rather than a published compatibility API. A breaking IR change SHALL update all in-repository consumers atomically, and continuous integration SHALL compile and test every generator module that consumes the IR in the same change. A serialized IR fixture SHALL NOT be treated as a persistent public format unless a separate versioned-format specification defines it.

#### Scenario: Breaking IR field change
- GIVEN an IR field used by the SDK, CLI, console, or TUI generator is removed or changes meaning
- WHEN the change is proposed
- THEN the same change SHALL update every affected consumer
- AND continuous integration SHALL fail if any consumer module no longer compiles or passes its conformance tests

### Requirement: Pre-Migration Characterization Gate

Before an existing generator replaces its independent OpenAPI parser with the canonical IR, it SHALL have black-box characterization tests for the observable behavior selected for preservation and not contradicted by an active specification. The same fixture cases and semantic assertions SHALL run unchanged against the legacy and IR-backed implementations during migration. Characterization tests SHALL compare public outcomes rather than private parser structs or incidental source formatting, and SHALL NOT preserve behavior recorded as partial or missing merely because the legacy implementation exhibits it.

#### Scenario: Preserve covered legacy behavior
- GIVEN an existing generator passes a baseline fixture for behavior already classified as covered
- WHEN its legacy parser is replaced by the canonical IR
- THEN the same black-box test case and assertions SHALL pass without modification
- AND the assertions SHALL compare operations, exact routes and inputs, schema-derived fields, security behavior, or executable artifact behavior as applicable

#### Scenario: Known legacy gap
- GIVEN reconciliation records a generator behavior as partial or missing
- WHEN characterization expectations are established
- THEN the incorrect or absent legacy behavior SHALL NOT become a compatibility expectation
- AND the active generator and IR specifications SHALL remain the source of truth for the corrected expectation

### Requirement: Repository OpenAPI Generation Gate

Continuous integration SHALL run every IR-consuming generator against the repository's resolved root `openapi/openapi.yaml` as well as the shared conformance fixtures. It SHALL generate into isolated temporary output roots and run each consumer's artifact acceptance tests so split-file resolution, real project schemas, and target integration are exercised together without requiring a database or external API service.

#### Scenario: Real specification smoke test
- GIVEN the repository root OpenAPI document and all referenced entity documents
- WHEN the generator test job runs
- THEN CLI, SDK, console, and TUI artifacts SHALL be generated in temporary directories
- AND each artifact SHALL pass its target-specific build, type-check, and behavioral acceptance checks
- AND the test SHALL leave the working tree unchanged

### Requirement: Deterministic Normalization

Equivalent OpenAPI inputs SHALL produce deterministically ordered IR output independent of YAML map iteration or referenced-file traversal order.

#### Scenario: Repeated generation
- GIVEN an unchanged OpenAPI document
- WHEN normalization and generation run twice
- THEN serialized IR fixtures and all generated outputs SHALL be byte-for-byte stable

### Requirement: Actionable Diagnostics

Validation and normalization failures SHALL report the source file and JSON Pointer of the offending OpenAPI node and SHALL include an operation ID or schema name when available. A generator SHALL stop before writing target output when the canonical IR contains an error.

#### Scenario: Unresolved path parameter
- GIVEN an operation path contains `{agent_id}` without a matching path parameter definition
- WHEN normalization runs
- THEN it SHALL fail before rendering
- AND the diagnostic SHALL identify the source file, path item, and missing parameter

### Requirement: Loader Conformance Fixtures

The shared OpenAPI front end SHALL have loader fixtures covering single-document input, split-file references, recursive schemas, unresolved references, cyclic non-schema references, and references rejected by the configured document-root boundary.

#### Scenario: Loader fixture suite
- GIVEN the shared loader test suite
- WHEN its conformance fixtures are executed
- THEN valid local and recursive references SHALL normalize to the expected canonical nodes
- AND unresolved, cyclic non-schema, and out-of-bound references SHALL produce the expected diagnostics

### Requirement: Operation and Security Conformance Fixtures

The shared OpenAPI front end SHALL have operation fixtures covering flat CRUD, multiply nested path parameters, non-CRUD actions, streaming responses, parameter serialization, inherited security, explicit `security: []`, and non-empty operation security overrides.

#### Scenario: Operation fixture suite
- GIVEN the shared operation fixture set
- WHEN it is normalized
- THEN every expected method, exact route, input, response, capability, stream, and security state SHALL be represented without target-specific inference

### Requirement: Schema and Role Conformance Fixtures

The shared OpenAPI front end SHALL have schema fixtures covering recursive and composed schemas, request and response projections, helper schemas, list item roles, error roles, read-only and write-only properties, nullability, enums, and constraints.

#### Scenario: Schema fixture suite
- GIVEN resource, request, list, helper, and error schemas participate in operations
- WHEN the fixture is normalized
- THEN structural semantics and usage-based roles SHALL match the expected canonical IR
- AND helper schemas SHALL NOT become resource views solely because of their names

### Requirement: Resource View and Metadata Conformance Fixtures

The shared OpenAPI front end SHALL have graph fixtures covering one schema exposed at multiple scopes, stable resource-view identities, Link Objects with endpoint identities and parameter mappings, ambiguous path relationships, capability-correct inferred endpoints and bindings, inferred relationship provenance, and preserved extensions at every supported scope.

#### Scenario: Resource graph fixture suite
- GIVEN global and parent-scoped operations expose the same item schema
- AND explicit links, ambiguous paths, and extensions are present
- WHEN the fixture is normalized
- THEN distinct stable resource-view identities, capability-correct relationship endpoints, explicit and inferred provenance, parameter mappings and bindings, and scoped extension values SHALL match the expected canonical IR

### Requirement: Consumer Fixture Conformance

Every generator that consumes the canonical IR SHALL run target tests from shared fixture cases and expected normalized semantics rather than independently parsing raw fixtures to derive its expectations. During migration, a fixture case MAY feed the legacy implementation directly, but the test's target-level assertions SHALL remain the same after the fixture is loaded through the canonical IR.

#### Scenario: New IR consumer
- GIVEN a new generator is added to TRex
- WHEN its test suite runs against the shared fixtures
- THEN it SHALL demonstrate that operation, schema, resource-view, security, and relationship discovery comes from the expected canonical IR

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Operations are primary nodes | Paths and operations define what a client can do; schemas alone do not define resources or capabilities |
| Resource views form a graph | The same kind can appear globally, under multiple parents, or through action and stream operations |
| `operationId` is the stable key | It is a standard OpenAPI field and avoids target-specific naming guesses |
| OpenAPI Links are explicit relationship edges | Links are the standard mechanism for connecting operations and carrying parameter mappings |
| Conservative path inference | Path nesting is useful evidence but cannot always establish semantic ownership |
| Extensions remain lossless | Presentation metadata can evolve without coupling the canonical IR to one generated target |
| Reference access is allowlisted | OpenAPI permits external references, but unattended generation must not grant specifications arbitrary filesystem or network access |
| Preserve first, escape at projection | The canonical model remains lossless while every output context applies its own validation and escaping rules |
| Internal contract changes are atomic | The IR need not provide public version compatibility, but every separately built consumer must move and be tested together |
| Characterize behavior, not implementation | Black-box assertions survive parser replacement without freezing private data structures, formatting, or known defects |
| Real and synthetic inputs are complementary | Fixtures isolate semantics while the repository OpenAPI document proves integration with the API that TRex actually ships |
| Normalize once, project many times | Shared semantics prevent the CLI, SDK, console, and TUI from disagreeing about the API |
