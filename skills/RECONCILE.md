# Reconciliation Checkpoint

**Last Updated:** 2026-07-06
**Last Run By:** Claude (reconcile skill — gap closure)

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
| **Total** | **18** | **105** | **105** | **0** | **0** | **100%** |

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

### Gap Execution Plan

All 4 gaps are in SEC-001 and are causally related. The fix order:

1. **GAP-001** (prerequisite) — Change `JWTHandler` to accept `[]string` for URLs
2. **GAP-002** (prerequisite) — Restructure `loadKeys()` for additive multi-source loading
3. **GAP-003** (auto-resolved) — Fixed by GAP-002
4. **GAP-004** (auto-resolved) — Fixed by GAP-001 + GAP-002

**Estimated scope:** ~100 lines changed in `pkg/auth/middleware.go` + ~5 lines in `pkg/server/apiserver.go`. The gRPC `JWKKeyProvider` (`pkg/server/grpcutil/jwk_provider.go`) serves as the reference implementation.

## Reconciliation History

| Date | Coverage | Delta | Agent |
|------|----------|-------|-------|
| 2026-07-06 | — | Initial seeding | Manual |
| 2026-07-06 | 96.2% (101/105) | First reconciliation run: 4 partial gaps in SEC-001 (HTTP JWTHandler multi-URL parity with gRPC) | Claude |
| 2026-07-06 | 100% (105/105) | Closed GAP-001–004: HTTP JWTHandler now supports multi-URL additive key loading with file+URL merging, matching gRPC JWKKeyProvider. Changed `pkg/auth/middleware.go` (~80 lines) and `pkg/server/apiserver.go` (1 line). All tests pass. | Claude |
