---
name: server-start
description: >
  Build and start the TRex server in development mode.
  Trigger phrases: "start server", "run server", "make run",
  "start the api", "launch server"
---

# Server Start

Build the binary and start the TRex server with authentication disabled for local development.

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Build the binary**:
   ```bash
   make binary
   ```

2. **Start the server** (auth disabled):
   ```bash
   make run-no-auth
   ```
   This starts four servers:
   - REST API: `localhost:8000`
   - Metrics: `localhost:8080`
   - Health check: `localhost:8083`
   - gRPC: `localhost:9000`

3. **Quick verification**:
   ```bash
   curl http://localhost:8000/api/rh-trex/v1/dinosaurs | jq
   ```

## Prerequisites
- PostgreSQL must be running (`make db/setup`)
- Migrations must be applied (`./trex migrate`)
