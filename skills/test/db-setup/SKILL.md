---
name: db-setup
description: >
  Set up a fresh local PostgreSQL database for development.
  Trigger phrases: "setup database", "db setup", "fresh database",
  "reset database", "recreate db"
---

# Database Setup

Tear down any existing database and create a fresh PostgreSQL container with migrations applied.

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Tear down existing database** (if running):
   ```bash
   make db/teardown
   ```

2. **Start fresh PostgreSQL container**:
   ```bash
   make db/setup
   ```
   This starts PostgreSQL on `localhost:5432` with database `rhtrex`, user `trex`.

3. **Build the binary** (if not already built):
   ```bash
   make binary
   ```

4. **Run migrations**:
   ```bash
   ./trex migrate
   ```

5. **Verify** the database is ready:
   ```bash
   make db/login
   # Then: \dt to list tables
   ```

## Connection Details
- Host: `localhost`
- Port: `5432`
- Database: `rhtrex`
- User: `trex`
- Password: (see `secrets/db.password`)

## Related Specs
- `specs/data/migration-pattern.spec.md` (DA-002)
