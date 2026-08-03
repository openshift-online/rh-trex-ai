# Dependency Supply Chain Specification

**Date:** 2026-08-03
**Status:** Active
**ID:** STD-004
**Related:** [Testing Standards](testing.spec.md), [OpenAPI Intermediate Representation](../codegen/openapi-ir.spec.md)
**Implements:** `Makefile`, `scripts/check-dependency-age.py`, `scripts/test_check_dependency_age.py`, `dependency-age-tools.json`, `dependency-age-allowlist.json`, `scripts/console-plugin-generator/templates/`, `.tekton/`

---

## Purpose

Define reproducible dependency declarations and a minimum-age admission policy for Go and JavaScript dependencies used by TRex builds, tests, and generated artifacts.

## Requirements

### Requirement: Exact Dependency Declarations

Repository-controlled build and test tools SHALL use exact versions. Generated JavaScript manifests SHALL declare exact dependency versions without range operators, wildcard versions, floating tags, or implicit latest-version resolution. Container images introduced for repository-controlled build or test tooling SHALL use an immutable digest together with a human-readable version tag.

#### Scenario: Generator acceptance toolchain
- GIVEN generator acceptance tests require Node.js, npm, or TypeScript
- WHEN the tests select their toolchain
- THEN the Node.js image SHALL include an exact release tag and immutable digest
- AND TypeScript and other downloaded tools SHALL use exact versions
- AND the host SHALL NOT install an unversioned Node.js or npm package for the test

### Requirement: Locked JavaScript Dependency Graph

Every generated JavaScript project SHALL include a package-manager lockfile that resolves the complete dependency graph, and automated builds SHALL use the package manager's frozen or clean-install mode without rewriting that lockfile.

#### Scenario: Generated console plugin build
- GIVEN the console generator emits `package.json`
- WHEN it emits and builds the plugin
- THEN it SHALL also emit the matching lockfile
- AND the build SHALL use `npm ci` or an equivalent frozen-lock operation
- AND repeated installs from the unchanged manifest and lockfile SHALL select the same package versions

### Requirement: Minimum Dependency Age

Continuous integration SHALL reject a resolved Go module or npm package version published fewer than 14 complete days before the check time. The checker SHALL inspect all repository-controlled Go module graphs and npm lockfiles, SHALL support a deterministic UTC time override for tests, and SHALL fail closed when registry or module metadata cannot establish a publish time.

#### Scenario: Newly published dependency
- GIVEN a resolved dependency was published after the UTC cutoff of check time minus 14 days
- WHEN the dependency-age check runs
- THEN the check SHALL fail
- AND the diagnostic SHALL identify the ecosystem, package, version, source manifest, publish time, and cutoff

#### Scenario: Deterministic policy test
- GIVEN the checker receives an explicit UTC `--now` value
- WHEN it evaluates fixtures on either side of the cutoff
- THEN its result SHALL depend on that supplied time rather than the host clock

### Requirement: Audited Minimum-Age Exceptions

A dependency younger than the minimum age MAY be admitted only by an exact ecosystem, package-name, and version allowlist entry. Every entry SHALL include a non-empty reason and compensating verification. An exception SHALL NOT match another package or another version.

#### Scenario: Exact exception match
- GIVEN one version is allowlisted with a reason and compensating verification
- WHEN the checker encounters that exact version and a newer non-allowlisted version
- THEN it SHALL admit only the exact allowlisted tuple
- AND it SHALL reject malformed or incomplete allowlist entries

### Requirement: Dependency Policy Verification

The dependency policy checker SHALL have offline unit tests for manifest parsing, cutoff boundaries, pseudo-version timestamps, malformed metadata, and allowlist validation. Repository verification SHALL reject floating versions in the generator dependency declarations and SHALL run the live minimum-age check in continuous integration.

#### Scenario: Continuous integration enforcement
- GIVEN a change adds or updates a Go or npm dependency used by the repository or generated console project
- WHEN continuous integration runs
- THEN offline policy tests SHALL pass
- AND the live dependency-age check SHALL pass before generator acceptance is considered successful

### Requirement: Actionable and Safe Metadata Access

The checker SHALL access registries only over HTTPS with bounded retries and timeouts, SHALL NOT execute package lifecycle scripts, and SHALL report metadata failures without printing credentials or response bodies.

#### Scenario: Registry metadata unavailable
- GIVEN package publish metadata cannot be retrieved securely within the configured retry and timeout bounds
- WHEN the dependency-age check runs
- THEN it SHALL fail with the affected ecosystem, package, version, and source manifest
- AND it SHALL NOT expose registry credentials or untrusted response content

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Fourteen-day minimum age | Reduces exposure to freshly published compromised or defective releases while keeping upgrades practical |
| Resolve Go graphs and npm lockfiles | Admission decisions apply to the versions actually executed, including transitive dependencies |
| Exact, documented exceptions | Emergency upgrades remain possible without making the policy broad or silent |
| Tag plus digest for test images | The tag communicates the intended runtime while the digest makes selection immutable |
| Node in a container | Keeps Node/npm out of the host and unit-test base image while making acceptance reproducible |
