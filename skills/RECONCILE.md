# Reconciliation Checkpoint

**Last Updated:** 2026-08-04
**Last Run By:** Codex (reconcile skill — secure pull request automation implementation)

---

## Coverage Summary

| Domain | Specs | Requirements | Covered | Partial | Missing | Coverage |
|--------|-------|-------------|---------|---------|---------|----------|
| framework | 4 | 24 | 24 | 0 | 0 | 100% |
| api | 2 | 16 | 16 | 0 | 0 | 100% |
| data | 2 | 12 | 12 | 0 | 0 | 100% |
| security | 3 | 17 | 17 | 0 | 0 | 100% |
| codegen | 4 | 17 | 17 | 0 | 0 | 100% |
| standards | 3 | 22 | 22 | 0 | 0 | 100% |
| **Total** | **18** | **108** | **108** | **0** | **0** | **100%** |

## Spec Dependency Order

Reconciliation MUST proceed in this order to respect dependencies:

- **Layer 0:** STD-001, SEC-003
- **Layer 1:** FW-001
- **Layer 2:** FW-002, FW-003, FW-004
- **Layer 3:** DA-001, API-001, API-002
- **Layer 4:** DA-002, SEC-001, STD-002
- **Layer 5:** SEC-002, STD-003
- **Layer 6:** CG-001, CG-002, CG-003, CG-004

## Gap Table

| ID | Spec | Requirement | Status | Severity | Notes |
|----|------|-------------|--------|----------|-------|
| GAP-001 | SEC-001 | JWK Key Loading: Multi-URL support on HTTP | closed | critical | Fixed: `JWTHandler.keysURL string` → `keysURLs []string`. `apiserver.go` now passes full `JwkCertURLs` slice via `WithKeysURLs()`. |
| GAP-002 | SEC-001 | JWK Key Loading: Additive file+URL merging on HTTP | closed | critical | Fixed: `loadKeys()` restructured to load file first, then iterate all URLs additively into a combined `newKeys` map. `parseJWKSet()` → `parseAndStoreKeys()` merges into target map. Mirrors gRPC `JWKKeyProvider` architecture. |
| GAP-003 | SEC-001 | Automatic Key Refresh: On-demand refresh from ALL sources on HTTP | closed | major | Auto-resolved by GAP-002: `validateToken()` calls `loadKeys()` which now loads from all configured sources (file + all URLs). |
| GAP-004 | SEC-001 | Multi-Issuer Support: HTTP/gRPC behavioral consistency | closed | major | Auto-resolved by GAP-001 + GAP-002: HTTP `JWTHandler` now has architectural parity with gRPC `JWKKeyProvider` — multi-URL, additive merging, all-source refresh. |
| GAP-005 | STD-003 | Untrusted Pull Request Isolation | closed | major | Fixed: `.github/workflows/trex-pr-ci.yml` runs fork code on `pull_request` with only `contents: read`, no persisted checkout credentials, immutable action SHAs, draft-transition coverage, and no secrets. |
| GAP-006 | STD-003 | Privilege-Separated Review Comments | closed | major | Fixed: `.github/workflows/trex-auto-review.yml` now consumes completed CI through `workflow_run`, verifies the current open PR/head SHA via GitHub APIs, treats patches as data, and creates or updates one marker-owned comment without checking out or executing fork content. |
| GAP-007 | STD-003 | Workflow Trust-Boundary Verification | closed | major | Fixed: `scripts/test_trex_review_workflows.py` validates triggers, exact permissions, immutable pins, valid expression operators, and prohibited privileged operations, with unsafe mutation cases. |

### Gap Execution Plan

All identified gaps are closed. Future workflow changes remain gated by the offline trust-boundary test in `scripts/test_trex_review_workflows.py`.

## Reconciliation History

| Date | Coverage | Delta | Agent |
|------|----------|-------|-------|
| 2026-07-06 | — | Initial seeding | Manual |
| 2026-07-06 | 96.2% (101/105) | First reconciliation run: 4 partial gaps in SEC-001 (HTTP JWTHandler multi-URL parity with gRPC) | Claude |
| 2026-07-06 | 100% (105/105) | Closed GAP-001–004: HTTP JWTHandler now supports multi-URL additive key loading with file+URL merging, matching gRPC JWKKeyProvider. Changed `pkg/auth/middleware.go` (~80 lines) and `pkg/server/apiserver.go` (1 line). All tests pass. | Claude |
| 2026-08-04 | 97.2% (105/108) | Added secure pull request execution, privilege-separated commenting, and workflow trust-boundary verification requirements; identified three implementation gaps. | Codex |
| 2026-08-04 | 100% (108/108) | Closed GAP-005–007 with read-only PR CI, an API-only trusted commenter, immutable action pins, and offline trust-boundary mutation tests. | Codex |
