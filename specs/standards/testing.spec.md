# Testing Standards Specification

**Date:** 2026-08-04
**Status:** Active
**ID:** STD-003
**Related:** [Entity Lifecycle](../framework/entity-lifecycle.spec.md), [Secrets Management](../security/secrets-management.spec.md)
**Implements:** `test/`, `plugins/*/integration_test.go`, `plugins/*/grpc_integration_test.go`, `plugins/*/factory_test.go`, `.github/workflows/trex-pr-ci.yml`, `.github/workflows/trex-auto-review.yml`, `scripts/test_trex_review_workflows.py`

---

## Purpose

Define the testing standards for unit tests, integration tests, test factories, and test infrastructure.

## Requirements

### Requirement: Test Separation

Unit tests and integration tests SHALL be separated by environment and execution command.

#### Scenario: Unit tests
- GIVEN `make test` is executed
- WHEN unit tests run with `OCM_ENV=unit_testing`
- THEN only `./pkg/...` and `./cmd/...` SHALL be tested
- AND NO database connection SHALL be required
- AND mock DAOs SHALL be used for all data access

#### Scenario: Integration tests
- GIVEN `make test-integration` is executed
- WHEN integration tests run with `OCM_ENV=integration_testing`
- THEN a real PostgreSQL database SHALL be available via testcontainers
- AND full CRUD operations SHALL be tested against the database

### Requirement: Testcontainers Integration

Integration tests SHALL use testcontainers-go to automatically provision a PostgreSQL container.

#### Scenario: Automatic database provisioning
- GIVEN an integration test calls `test.RegisterIntegration(t)`
- WHEN the test environment initializes
- THEN a PostgreSQL container SHALL be started
- AND migrations SHALL be applied
- AND the container SHALL be torn down after tests complete

### Requirement: Test Factory Pattern

Each entity SHALL have a test factory providing convenience builders for test data.

#### Scenario: Factory usage
- GIVEN a `DinosaurFactory` in `plugins/dinosaurs/factory_test.go`
- THEN it SHALL provide:
  - `NewDinosaur()` — creates a minimal valid instance
  - Named builders (e.g., `BuildTRex()`, `BuildTriceratops()`) for common test cases
  - Pointer helper functions (`StringPtr`, `IntPtr`, `BoolPtr`) for nullable fields

### Requirement: Integration Test Coverage

Generated integration tests SHALL cover all CRUD operations and error cases.

#### Scenario: REST integration test coverage
- GIVEN a generated entity's `integration_test.go`
- THEN tests SHALL cover:
  - `TestCreate` — successful creation with 201 response
  - `TestGet` — retrieval by ID with 200 response
  - `TestList` — paginated list with 200 response
  - `TestPatch` — partial update with 200 response
  - `TestDelete` — deletion with 204 response
  - `TestGetNotFound` — 404 for non-existent ID

#### Scenario: gRPC integration test coverage
- GIVEN a generated entity's `grpc_integration_test.go`
- THEN tests SHALL cover:
  - `TestGRPCCreate` — successful creation via gRPC
  - `TestGRPCGet` — retrieval via gRPC
  - `TestGRPCList` — listing via gRPC
  - `TestGRPCUpdate` — update via gRPC
  - `TestGRPCDelete` — deletion via gRPC
  - `TestGRPCWatch` — server-streaming event subscription

### Requirement: Test Main Pattern

Each plugin's test suite SHALL have a `testmain_test.go` implementing `TestMain(m *testing.M)`.

#### Scenario: TestMain lifecycle
- GIVEN `testmain_test.go` in a plugin directory
- WHEN `go test` runs
- THEN `TestMain` SHALL:
  1. Set up the test environment (database, migrations, server)
  2. Run all tests via `m.Run()`
  3. Tear down the environment
  4. Exit with the test result code

### Requirement: Database Reset Between Tests

Integration tests SHALL reset the database state between test functions to prevent cross-test contamination.

#### Scenario: Test isolation
- GIVEN `TestCreate` runs and inserts a dinosaur
- WHEN `TestList` runs next
- THEN the dinosaur from `TestCreate` SHALL NOT be visible
- AND each test SHALL start with a clean database state

### Requirement: Untrusted Pull Request Isolation

Repository build and test automation for pull requests SHALL run through the `pull_request` event with a read-only `contents` token. It SHALL NOT receive repository write permissions or secrets, and third-party actions SHALL be pinned to immutable commit SHAs. The workflow SHALL run for opened, synchronized, reopened, and ready-for-review pull requests and SHALL skip drafts until they become ready.

#### Scenario: Pull request from a fork
- GIVEN an untrusted contributor opens or updates a pull request from a fork
- WHEN pull request CI executes code from that pull request
- THEN the workflow SHALL have only read access to repository contents
- AND the workflow SHALL NOT be able to write comments, reviews, issues, or repository contents
- AND marking a draft ready for review SHALL trigger CI

### Requirement: Privilege-Separated Review Comments

Automated review comments SHALL be posted by a trusted `workflow_run` workflow after pull request CI completes. The commenting workflow SHALL use the workflow definition from the default branch, SHALL associate the completed head SHA with its open pull request through trusted GitHub API data, and SHALL use only the minimum `actions: read`, `contents: read`, `issues: write`, and `pull-requests: read` permissions. It SHALL NOT check out, execute, import, evaluate, cache, or restore pull request code or artifacts. Pull request filenames and patches SHALL be treated as untrusted data, and the workflow SHALL update one marker-owned bot comment rather than create a new comment on every synchronization.

#### Scenario: Completed fork CI run
- GIVEN read-only CI completes for a pull request from a fork
- WHEN the privileged review workflow receives the completion event
- THEN it SHALL read the CI result and changed-file patches through the GitHub API
- AND it SHALL create or update the automated review comment
- AND it SHALL NOT download or execute any content produced by the fork

### Requirement: Workflow Trust-Boundary Verification

The repository SHALL have an offline automated test that validates the event triggers, permissions, immutable action pins, draft transition coverage, and prohibited privileged-workflow operations required by this specification.

#### Scenario: Unsafe workflow regression
- GIVEN a change adds `pull_request_target`, a pull request checkout, shell execution, artifact transfer, or cache use to the privileged commenting workflow
- WHEN the workflow policy test runs
- THEN the test SHALL fail before the workflow change is accepted

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Testcontainers over shared database | Hermetic tests; no external dependencies; parallel-safe |
| Factory pattern over fixture files | Type-safe; composable; discoverable via IDE completion |
| Tests co-located with plugin code | Tests live next to the code they test; easy to navigate |
| TestMain for lifecycle management | Standard Go pattern; controls setup/teardown for entire package |
| Separate unit and integration commands | Unit tests are fast (no DB); integration tests are thorough |
| Two-stage pull request automation | Untrusted code can be tested while comment writes remain confined to trusted default-branch logic |
| API-only privileged review | Patch analysis and comment updates do not require checking out or executing fork-controlled content |
| Marker-owned comment updates | One auditable bot comment reflects the latest run without notification spam |
| Cache-warming serial Go validation | Measured PR runs showed one serial runner was faster than job-level or in-runner parallelism because build artifacts were reused by vet and unit tests without CPU contention |
