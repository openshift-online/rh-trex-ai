---
name: integration-test
description: >
  Run integration tests with a real PostgreSQL database via testcontainers.
  Trigger phrases: "run integration tests", "make test-integration",
  "test with database", "full test suite"
---

# Integration Test

Run integration tests against a real PostgreSQL database using testcontainers.

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Run integration tests**:
   ```bash
   make test-integration
   ```
   This runs with `OCM_ENV=integration_testing` and covers `./test/integration/...` and `./plugins/...`.
   Testcontainers automatically starts and stops a PostgreSQL container.

2. **Run specific test** (if $ARGUMENTS specifies a test name):
   ```bash
   OCM_ENV=integration_testing go test -v ./plugins/{kind}/ -run {TestName}
   ```

3. **Analyze failures**:
   - **Container startup failure**: Check Docker/Podman is running
   - **Migration failure**: May need schema updates; check migration files
   - **Test data issues**: Check factory functions for correct test data setup

4. **Enable database debug** for troubleshooting:
   ```bash
   OCM_ENV=integration_testing go test -v ./plugins/{kind}/ -run {TestName} -args --enable-db-debug
   ```

## Related Specs
- `specs/standards/testing.spec.md` (STD-003)
