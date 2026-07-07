# Framework Specifications

**Domain:** framework
**Status:** Active

---

## Specs

| ID | Spec | Description |
|----|------|-------------|
| FW-001 | [Plugin Architecture](plugin-architecture.spec.md) | Plugin registration, init()-based discovery, auto-wiring |
| FW-002 | [Entity Lifecycle](entity-lifecycle.spec.md) | Generator → Model → DAO → Service → Handler → Tests |
| FW-003 | [Event-Driven Controllers](event-driven-controllers.spec.md) | PostgreSQL LISTEN/NOTIFY, idempotent handlers, advisory locks |
| FW-004 | [Service Locator](service-locator.spec.md) | Registry pattern, dependency injection, thread-safe access |
