# TRex Framework Specification Registry

**Date:** 2026-08-04
**Status:** Living Document

---

## Purpose

This file is the machine-readable registry of all specifications governing the rh-trex-ai framework. Agents use this index to discover specs, understand dependencies, and drive reconciliation.

## Spec Registry

| ID | Spec | Domain | Status | Depends On | Path |
|----|------|--------|--------|------------|------|
| FW-001 | Plugin Architecture | framework | Active | — | `framework/plugin-architecture.spec.md` |
| FW-002 | Entity Lifecycle | framework | Active | FW-001 | `framework/entity-lifecycle.spec.md` |
| FW-003 | Event-Driven Controllers | framework | Active | FW-001 | `framework/event-driven-controllers.spec.md` |
| FW-004 | Service Locator | framework | Active | FW-001 | `framework/service-locator.spec.md` |
| API-001 | REST Conventions | api | Active | FW-002 | `api/rest-conventions.spec.md` |
| API-002 | gRPC Conventions | api | Active | FW-003 | `api/grpc-conventions.spec.md` |
| DA-001 | DAO Pattern | data | Active | FW-004 | `data/dao-pattern.spec.md` |
| DA-002 | Migration Pattern | data | Active | DA-001 | `data/migration-pattern.spec.md` |
| SEC-001 | Authentication | security | Active | API-001, API-002 | `security/authentication.spec.md` |
| SEC-002 | Authorization | security | Active | SEC-001 | `security/authorization.spec.md` |
| SEC-003 | Secrets Management | security | Active | — | `security/secrets-management.spec.md` |
| CG-001 | Entity Generator | codegen | Active | FW-001, FW-002, DA-002, API-001, API-002 | `codegen/entity-generator.spec.md` |
| CG-002 | CLI Generator | codegen | Active | API-001 | `codegen/cli-generator.spec.md` |
| CG-003 | SDK Generator | codegen | Active | API-001 | `codegen/sdk-generator.spec.md` |
| CG-004 | Console Plugin Generator | codegen | Active | API-001 | `codegen/console-plugin-generator.spec.md` |
| STD-001 | Naming Conventions | standards | Active | — | `standards/naming-conventions.spec.md` |
| STD-002 | Error Handling | standards | Active | API-001 | `standards/error-handling.spec.md` |
| STD-003 | Testing Standards | standards | Active | FW-002 | `standards/testing.spec.md` |

## Spec Dependency Order

Topological layers for reconciliation:

- **Layer 0 (no deps):** STD-001, SEC-003
- **Layer 1 (foundational):** FW-001
- **Layer 2 (framework):** FW-002, FW-003, FW-004
- **Layer 3 (data + api):** DA-001, API-001, API-002
- **Layer 4 (data + security):** DA-002, SEC-001, STD-002
- **Layer 5 (auth + standards):** SEC-002, STD-003
- **Layer 6 (codegen):** CG-001, CG-002, CG-003, CG-004

## SDLC Workflow

```
0. /reconcile             — autonomous spec-to-code reconciliation
1. /spec                  — define desired state
2. /entity-generator      — build the entity
3. /db-setup              — prepare local database
4. /unit-test             — validate with unit tests
5. /integration-test      — validate with integration tests
6. /verify                — static analysis
7. /code-review           — standards review
```
