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

3. **Scan specs by dependency layer** (Layer 0 first, Layer 7 last):
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

7. **Generate CLI** (after API server entities are implemented):
   - Run the CLI generator against the merged OpenAPI spec:
     ```bash
     cd scripts/cli-generator
     go run . --spec ../../openapi/openapi.yaml --out /tmp/trex-cli
     ```
   - Verify the generated CLI builds:
     ```bash
     cd /tmp/trex-cli && go build -o trex-cli .
     ```
   - Record CLI coverage status per entity in the gap table

8. **Generate SDKs** (after API server entities are implemented):
   - Run the SDK generator against the merged OpenAPI spec:
     ```bash
     cd scripts/sdk-generator
     go run . --spec ../../openapi/openapi.yaml \
       --go-out /tmp/trex-sdk-go \
       --python-out /tmp/trex-sdk-python \
       --ts-out /tmp/trex-sdk-ts
     ```
   - Verify Go SDK compiles:
     ```bash
     cd /tmp/trex-sdk-go && go build ./...
     ```
   - Record SDK coverage status per language in the gap table

9. **Generate Console UI** (after SDKs are generated — requires TypeScript SDK):
   - Run the console plugin generator:
     ```bash
     cd scripts/console-plugin-generator
     go run . --spec ../../openapi/openapi.yaml --out /tmp/trex-console --name trex-console
     ```
   - Or generate a standalone CRUD UI (APP-002) if not targeting OpenShift Console
   - Verify the UI builds:
     ```bash
     cd /tmp/trex-console && npm install && npm run build
     ```

10. **Commit checkpoint**: Update `skills/RECONCILE.md` with new coverage metrics and history entry.

## Execution Order

The full reconciliation pipeline follows this dependency chain:

```
API Server entities (CG-001)
  → Post-generation customization (hand-coded)
    → OpenAPI spec regeneration (make generate)
      → CLI generation (CG-002)
      → SDK generation (CG-003: Go, Python, TypeScript)
        → Console UI generation (CG-004 / APP-002)
          → Verification (make binary && make verify && make test && make lint)
```

CLI and SDK generation depend on a correct and complete OpenAPI spec. Always run `make generate` after entity changes before generating CLIs or SDKs.

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

### Client Generation Status
| Artifact | Generator | Status | Output |
|----------|-----------|--------|--------|
| CLI (trex-cli) | CG-002 `scripts/cli-generator/` | {status} | {path} |
| Go SDK | CG-003 `scripts/sdk-generator/` | {status} | {path} |
| Python SDK | CG-003 `scripts/sdk-generator/` | {status} | {path} |
| TypeScript SDK | CG-003 `scripts/sdk-generator/` | {status} | {path} |
| Console UI | CG-004 `scripts/console-plugin-generator/` | {status} | {path} |

### History
| Date | Coverage | Delta |
|------|----------|-------|
| ...
```

## Related Specs
- `specs/index.spec.md` (all specs)
- `specs/app/managed-api-platform.spec.md` (APP-001 — CLI and SDK sections)
- `specs/codegen/cli-generator.spec.md` (CG-002)
- `specs/codegen/sdk-generator.spec.md` (CG-003)
- `specs/codegen/console-plugin-generator.spec.md` (CG-004)
- `specs/app/console-ui.spec.md` (APP-002 — CRUD UI)
- All domain specs referenced via the registry
