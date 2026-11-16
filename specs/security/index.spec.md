# Security Specifications

**Domain:** security
**Status:** Active

---

## Specs

| ID | Spec | Description |
|----|------|-------------|
| SEC-001 | [Authentication](authentication.spec.md) | JWT/OIDC, JWK key management, multi-issuer, bearer tokens |
| SEC-002 | [Authorization](authorization.spec.md) | Framework-owned `Authorizer` for REST and gRPC; environment selects the policy |
| SEC-003 | [Secrets Management](secrets-management.spec.md) | File-based secrets, no logging, TLS configuration |
| SEC-004 | [Audit Logging](audit-logging.spec.md) | Framework-captured audit events, pluggable recorder, JSON-lines default, per-kind read opt-in |
