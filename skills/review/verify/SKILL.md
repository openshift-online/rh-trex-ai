---
name: verify
description: >
  Run static analysis: go vet, gofmt, and golangci-lint.
  Trigger phrases: "verify code", "run lint", "static analysis",
  "check formatting", "make verify"
---

# Verify

Run static analysis tools to check code quality, formatting, and common issues.

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Run go vet and format check**:
   ```bash
   make verify
   ```

2. **Run linter**:
   ```bash
   make lint
   ```

3. **Build binary** to confirm compilation:
   ```bash
   make binary
   ```

4. **Fix issues** if found:
   - Formatting: `gofmt -w {file}`
   - Vet errors: Fix the reported issues
   - Lint warnings: Address based on golangci-lint rules

## Related Specs
- `specs/standards/naming-conventions.spec.md` (STD-001)
