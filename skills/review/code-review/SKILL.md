---
name: code-review
description: >
  Perform a comprehensive code review against TRex standards.
  Trigger phrases: "review code", "code review", "review changes",
  "check my code", "review PR"
---

# Code Review

Perform a comprehensive code review against TRex framework standards, security requirements, and best practices.

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Load review context** — read the following files:
   - `specs/framework/plugin-architecture.spec.md`
   - `specs/framework/entity-lifecycle.spec.md`
   - `specs/security/authentication.spec.md`
   - `specs/standards/error-handling.spec.md`
   - `specs/standards/testing.spec.md`
   - `specs/standards/naming-conventions.spec.md`
   - `.claude/context/backend-development.md`
   - `.claude/context/security-standards.md`
   - `.claude/patterns/error-handling.md`

2. **Identify changes** to review:
   ```bash
   git diff --name-only HEAD~1  # Or against the base branch
   ```

3. **Review each changed file** against 7 axes:

   | Axis | Key Checks |
   |------|-----------|
   | Framework Compliance | Plugin architecture, layered design, service locator pattern |
   | Security | JWT validation, no secret logging, input validation, SQL injection prevention |
   | Database | Migration patterns, GORM usage, transaction handling, advisory locks |
   | Testing | Unit + integration coverage, factory patterns, mock usage |
   | Performance | N+1 queries, pagination, advisory locks, context timeouts |
   | API Design | REST conventions, OpenAPI compliance, error responses |
   | Event Architecture | Idempotent handlers, event creation, controller registration |

4. **Classify findings** by severity:
   - **Blocker**: Security vulnerabilities, data corruption risk, server crashes
   - **Critical**: Framework violations, missing auth, `panic()` calls, breaking changes
   - **Major**: Poor error handling, missing tests, performance issues
   - **Minor**: Style, naming, documentation gaps

5. **Produce review report**:
   ```
   ## Review Summary
   - Files reviewed: {n}
   - Findings: {blockers} blockers, {critical} critical, {major} major, {minor} minor
   - Approval: APPROVED / CHANGES REQUESTED / BLOCKED

   ## Findings
   ### [Severity] {file}:{line} — {title}
   {description}
   Spec reference: {spec_id}
   ```

## Related Specs
- All specs (cross-cutting review)
