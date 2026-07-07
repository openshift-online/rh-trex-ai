---
name: reconcile
description: >
  Autonomous spec-to-code reconciliation. Reads specs, analyzes code,
  identifies gaps, and drives implementation to close them.
  Trigger phrases: "reconcile", "sync specs", "close gaps",
  "spec coverage", "what's missing"
---

# Reconcile

Autonomous spec-to-code reconciliation orchestrator. Reads all specs, analyzes the codebase, identifies gaps between desired state (specs) and actual state (code), then drives implementation to close them.

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Load checkpoint**: Read `skills/RECONCILE.md` for current gap state and coverage metrics.

2. **Read spec registry**: Parse `specs/index.spec.md` to discover all specs and their dependency order.

3. **Scan specs by dependency layer** (Layer 0 first, Layer 6 last):
   For each spec:
   - Read the spec file
   - Extract all requirements and scenarios
   - Check the codebase for implementation evidence
   - Record coverage status: `covered`, `partial`, `missing`

4. **Update gap table** in `skills/RECONCILE.md`:
   - Add new gaps for missing/partial requirements
   - Mark resolved gaps as `closed`
   - Update coverage percentages per domain

5. **Prioritize gaps** by severity:
   - **Blocker**: Security requirements (SEC-*) not met
   - **Critical**: Framework contracts (FW-*) violated
   - **Major**: API conventions (API-*) or data patterns (DA-*) incomplete
   - **Minor**: Standards (STD-*) or codegen (CG-*) gaps

6. **Execute fixes** (if `--fix` is specified):
   - Work through gaps in dependency order
   - Use appropriate skills (entity-generator, add-field, etc.) for implementation
   - Run verification after each fix: `make verify && make lint && make test`

7. **Commit checkpoint**: Update `skills/RECONCILE.md` with new coverage metrics and history entry.

## Output Format

```
## Reconciliation Report — {date}

### Coverage Summary
| Domain | Specs | Requirements | Covered | Partial | Missing | Coverage |
|--------|-------|-------------|---------|---------|---------|----------|
| framework | 4 | {n} | {n} | {n} | {n} | {%} |
| api | 2 | {n} | {n} | {n} | {n} | {%} |
| ...

### Gaps (sorted by priority)
| ID | Spec | Requirement | Status | Severity |
|----|------|-------------|--------|----------|
| ...

### History
| Date | Coverage | Delta |
|------|----------|-------|
| ...
```

## Related Specs
- `specs/index.spec.md` (all specs)
- All domain specs referenced via the registry
