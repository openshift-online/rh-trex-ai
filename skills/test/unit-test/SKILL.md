---
name: unit-test
description: >
  Run unit tests for the TRex framework.
  Trigger phrases: "run unit tests", "make test", "test unit",
  "run tests without database"
---

# Unit Test

Run unit tests using mock dependencies (no database required).

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Run unit tests**:
   ```bash
   make test
   ```
   This runs with `OCM_ENV=unit_testing` and covers `./pkg/...` and `./cmd/...`.

2. **Analyze failures** if any tests fail:
   - Check mock expectations are set correctly
   - Verify mock DAO returns match expected service behavior
   - Look for import issues or missing mocks

3. **Run specific test** (if $ARGUMENTS specifies a test name):
   ```bash
   OCM_ENV=unit_testing go test -v ./pkg/... -run {TestName}
   ```

## Related Specs
- `specs/standards/testing.spec.md` (STD-003)
