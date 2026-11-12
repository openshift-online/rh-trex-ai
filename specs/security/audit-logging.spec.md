# Audit Logging Specification

**Date:** 2026-11-12
**Status:** Active
**ID:** SEC-004
**Related:** [Authentication](authentication.spec.md), [Authorization](authorization.spec.md), [Secrets Management](secrets-management.spec.md), [Migration Pattern](../data/migration-pattern.spec.md), [Logging](../standards/logging.spec.md)
**Implements:** `components/api-server/pkg/audit/`, `components/api-server/pkg/authz/observation.go`, `components/api-server/pkg/config/audit.go`, `components/api-server/pkg/cmd/audit.go`, `components/api-server/pkg/cmd/serve.go`, `components/api-server/pkg/server/routes.go`, `components/api-server/pkg/server/routebuilder.go`, `components/api-server/pkg/server/apiserver.go`, `components/api-server/pkg/server/grpc_server.go`, `components/api-server/pkg/db/db_session/testcontainer.go`, `components/api-server/pkg/environments/framework.go`

---

## Purpose

TRex records who did what, to which resource, when, and with what result, for
every service built on it. Recording is a framework concern, like
authorization (SEC-002): a plugin gets an audit trail without writing audit
code, and cannot opt out by forgetting a line. The trail is durable (stored in
PostgreSQL), tamper-evident (a keyed hash chain exposes any edit, insertion,
or deletion), and exported to a SIEM (Security Information and Event
Management system, such as Splunk, Elastic, or Sentinel) as a stable,
versioned JSON-lines stream. The exported copy is the independent witness that
detects tampering by anyone who controls the database.

## Requirements

### Requirement: Audit Event Record

TRex SHALL define, in package `pkg/audit`, one event record that every audit
entry uses. An event SHALL carry:

| Field | Meaning |
|---|---|
| `event_id` | Unique KSUID of the event; the SIEM deduplication key |
| `occurred_at` | UTC time with microsecond precision |
| `actor` | `type` (`user`, `system`, or `anonymous`), `username`, and, when a verified token is present, `subject` (`sub`), `issuer` (`iss`), and `client_id` (`azp`, else `clientId`) |
| `action` | The authorization action (`get`, `list`, `create`, `update`, `patch`, `delete`, `watch`, or a custom verb) |
| `outcome` | `success`, `failure`, `denied`, `unauthenticated`, or `error` |
| `reason` | Short non-secret text explaining a non-success outcome |
| `resource` | `surface`, `kind`, `id`, and `parent` (the path variables other than `id`), as in SEC-002 |
| `transport` | `http`, `grpc`, or `internal` |
| `http` | `method`, `path` (no query string), and `status` (non-success outcomes only) |
| `grpc` | `method` (full method name) and `code` (non-success outcomes only) |
| `source` | `ip` (the transport peer address) and `user_agent` |
| `operation_id` | The request operation ID (STD-005 `opid`) |
| `trace_id` | The W3C trace ID of the request, when a `traceparent` header or span context is present |
| `details` | String key/value pairs supplied by application events |

An event MUST NOT contain request or response bodies, query strings, header
values other than `User-Agent`, tokens, passwords, or other credentials.
Before storing, TRex SHALL replace invalid UTF-8 and NUL bytes in every text
field and bound client-controlled text to 2048 bytes, so a client cannot make
its own events fail to store.

#### Scenario: Mutation by a user
- GIVEN user `alice` with a verified token
- WHEN she sends `DELETE /api/rh-trex-ai/v1/dinosaurs/d-1` and it succeeds
- THEN one event SHALL be stored with `actor.type="user"`, `actor.username="alice"`, `action="delete"`, `outcome="success"`, `resource.kind="dinosaurs"`, `resource.id="d-1"`, `resource.surface="v1"`, and `transport="http"`

#### Scenario: No credentials recorded
- GIVEN a request with an `Authorization` header, a `Cookie` header, a query string, and a JSON body
- WHEN it is audited
- THEN the stored and exported event SHALL contain none of those values

### Requirement: Framework-Recorded Requests

TRex SHALL record audit events for REST and gRPC requests itself. Plugins MUST
NOT attach audit middleware or interceptors. TRex SHALL record:

1. Every request whose action is not `get`, `list`, or `watch`, whatever its outcome.
2. Every request that authorization denies, including denials reported as not found (`HideExistence`) and requests to undeclared routes or methods. Outcome `denied`.
3. Every request whose authorizer or subject enricher returns an error. Outcome `error`.
4. Every request that fails authentication (REST `401`, gRPC `codes.Unauthenticated`) on an authenticated surface. Outcome `unauthenticated`, `actor.type="anonymous"`.
5. Allowed `get`, `list`, and `watch` requests only when `--audit-reads=true`.

Routes outside the authenticated surfaces (OpenAPI, `/auth/config`, metadata,
health, metrics), gRPC bypass methods, and REST requests that match no route or
no route method (`404`/`405` from the router) SHALL NOT be recorded.
Authentication failures SHALL be recorded both when a surface's middleware
refuses the request and when an authentication handler that wraps the whole
router (the JWT handler) refuses it before routing, for paths under the API
base path. A request SHALL be recorded once.

A gRPC stream holds no transaction. TRex SHALL record a stream when it ends,
treating `codes.OK` and `codes.Canceled` (client closed the stream) as
success.

#### Scenario: Plugin without audit code
- GIVEN a plugin that registers routes with `r.Resource("fossils")` and contains no audit code
- WHEN a client creates a fossil
- THEN TRex SHALL store an event with `action="create"` and `resource.kind="fossils"`

#### Scenario: Denied read
- GIVEN an authorizer that denies `get` on `dinosaurs/d-1` for `bob`
- WHEN `bob` sends `GET /api/rh-trex-ai/v1/dinosaurs/d-1`
- THEN an event SHALL be stored with `outcome="denied"`, `action="get"`, and `http.status=403`

#### Scenario: Denied as not found
- GIVEN the authorizer returns `Decision{Allowed: false, HideExistence: true}`
- WHEN the request is audited
- THEN the event SHALL have `outcome="denied"` even though the client receives `404`

#### Scenario: Authentication failure
- GIVEN JWT authentication is enabled
- WHEN a request to `/api/rh-trex-ai/v1/dinosaurs` carries no valid token
- THEN an event SHALL be stored with `outcome="unauthenticated"` and `actor.type="anonymous"`

#### Scenario: Allowed read not recorded by default
- GIVEN `--audit-reads=false`
- WHEN an allowed `GET /api/rh-trex-ai/v1/dinosaurs` succeeds
- THEN no event SHALL be stored

#### Scenario: gRPC mutation
- GIVEN `DeleteDinosaur` is declared as `{Kind: "dinosaurs", Action: "delete"}`
- WHEN a client calls it and it returns an error
- THEN an event SHALL be stored with `transport="grpc"`, `outcome="failure"`, `grpc.method="/rh_trex.v1.DinosaurService/DeleteDinosaur"`, and the returned status code

### Requirement: Success Records Commit With the Change

The event of an allowed request that TRex records SHALL be inserted in the
request's database transaction, before the handler runs. It SHALL commit
exactly when the request's changes commit. If the insert fails, the handler
MUST NOT run, REST SHALL respond `503 Service Unavailable`, and gRPC SHALL
return `codes.Unavailable`.

When the request's transaction is rolled back (a REST status of 400 or more, a
gRPC error, or a panic), TRex SHALL record the request again outside that
transaction with outcome `failure` and the response status or code.

Events with outcome `denied`, `unauthenticated`, or `error` SHALL be written
outside the request transaction. A failure to write such an event, or a
`failure` event, SHALL be logged at Error level and counted in the
`trex_audit_write_failures_total` metric. It SHALL NOT change the response.
These writes SHALL run after the request has released its transaction
connection and SHALL time out after 5 seconds, so a burst of refused requests
cannot exhaust the connection pool.

#### Scenario: Audit store unavailable
- GIVEN the audit insert fails
- WHEN a client sends a create request
- THEN the handler SHALL NOT run
- AND REST SHALL respond `503`
- AND no resource SHALL be created

#### Scenario: Handler fails after the insert
- GIVEN an allowed `PATCH` whose handler responds `409 Conflict`
- WHEN the transaction rolls back
- THEN no `success` event SHALL remain
- AND one event with `outcome="failure"` and `http.status=409` SHALL be stored

### Requirement: Application Events

`pkg/audit` SHALL provide `audit.Record(ctx, audit.Event) error` for events
that a request does not express, such as a background job, a controller
action, or a domain step inside a handler. The event SHALL be written in the
transaction held by `ctx` when there is one, and on its own otherwise. Missing
`event_id`, `occurred_at`, `operation_id`, `trace_id`, and, inside a request,
actor fields SHALL be filled from the context. An event without an actor
outside a request SHALL have `actor.type="system"`. Callers SHALL propagate the
returned error.

#### Scenario: Controller action
- GIVEN a controller handler with no request in its context
- WHEN it calls `audit.Record` with `action="reconcile"` and `resource.kind="dinosaurs"`
- THEN an event SHALL be stored with `actor.type="system"` and `transport="internal"`

#### Scenario: Domain event in a request transaction
- GIVEN a handler that calls `audit.Record` and then fails with `500`
- WHEN the transaction rolls back
- THEN the application event SHALL be rolled back with it

### Requirement: Append-Only Storage

Events SHALL be stored in the table `trex_audit_events`, created by a
framework migration (DA-002). A database trigger SHALL reject `DELETE` and
`TRUNCATE` on the table. It SHALL reject every `UPDATE` except the single
sealing update that sets the seal columns (`chain_seq`, `prev_hash`, `hash`,
`key_id`, `sealed_at`) of an unsealed row and changes nothing else. That update
SHALL place the row at the next `chain_seq` and link `prev_hash` to the hash of
the previous position, so no row can be hidden outside the chain.

#### Scenario: Edit attempt
- GIVEN a stored event
- WHEN any client runs `UPDATE trex_audit_events SET outcome='success'` on it
- THEN the database SHALL reject the statement

#### Scenario: Hiding an event with a fake seal
- GIVEN an unsealed event
- WHEN a client sets its `chain_seq` to `-1` or to a position whose `prev_hash` does not link
- THEN the database SHALL reject the statement
- AND verification SHALL report any row with `chain_seq` below 1 that a superuser forced in

#### Scenario: Delete attempt
- WHEN any client runs `DELETE FROM trex_audit_events` or `TRUNCATE trex_audit_events`
- THEN the database SHALL reject the statement

### Requirement: Tamper-Evident Hash Chain

A sealer SHALL link committed events into one chain, in the order it seals
them. For each event it SHALL assign the next `chain_seq` (starting at 1, with
no gaps) and compute:

```
hash = MAC(key, "trex-audit-v1\n" + prev_hash + "\n" + canonical_json(event))
```

`prev_hash` is the `hash` of the event with the previous `chain_seq`, or 64
zeros for the first event. `canonical_json` SHALL be the exported line of the
event (see "SIEM Export") without `prev_hash` and `hash`: the event fields,
`schema_version`, `chain_seq`, and `key_id`, in the fixed field order of
`audit.Line`, with sorted map keys, no HTML escaping, empty maps as `{}`, and
times as `YYYY-MM-DDTHH:MM:SS.ffffffZ`. `MAC` SHALL
be HMAC-SHA256 when a key is configured. Without a key, `MAC` SHALL be plain
SHA-256 of the same input and `key_id` SHALL be `none`. With a key, `key_id`
SHALL be the first 16 hex characters of the SHA-256 of the key.

Sealing SHALL run at most once at a time across all replicas, using a
PostgreSQL transaction-scoped advisory lock. It SHALL run every
`--audit-seal-interval` (default `1s`) and SHALL NOT block requests.

#### Scenario: Edited event detected
- GIVEN a sealed chain
- WHEN someone with database owner rights disables the trigger and changes an event's `actor_username`
- THEN verification SHALL report that event's `chain_seq` as a hash mismatch

#### Scenario: Deleted event detected
- GIVEN a sealed chain
- WHEN an event in the middle is removed
- THEN verification SHALL report a `chain_seq` gap

#### Scenario: Concurrent replicas
- GIVEN two server replicas sealing at the same time
- WHEN both run a sealing pass
- THEN every event SHALL get exactly one `chain_seq`
- AND `chain_seq` values SHALL be contiguous

### Requirement: Keyed Hashing Secret

The HMAC key SHALL be read from `--audit-hmac-key-file` (SEC-003). It SHALL
be at least 32 bytes after trimming trailing whitespace. An unreadable or
short key file SHALL stop startup with an error. When no key file is
configured, the server SHALL start with plain SHA-256 and SHALL log a warning
that the chain detects accidental damage but not a forger who can recompute
hashes. The key MUST NOT be stored in the database, logged, or exported.

#### Scenario: Short key
- GIVEN `--audit-hmac-key-file` points to a 16-byte file
- WHEN the service starts
- THEN startup SHALL fail with an error naming the flag

#### Scenario: No key
- GIVEN no `--audit-hmac-key-file`
- WHEN the service starts
- THEN it SHALL start and log one warning
- AND sealed events SHALL have `key_id="none"`

### Requirement: SIEM Export

After a sealing transaction commits, TRex SHALL write each sealed event, in `chain_seq` order, as one
JSON object per line to the sink selected by `--audit-sink`: `stdout`
(default), `file:<path>` (opened for append, mode `0600`), or `none`. Each line
SHALL contain every event field plus `schema_version` (`1`), `chain_seq`,
`prev_hash`, `hash`, and `key_id`, so a SIEM can verify the chain on its own
copy. Field names and meanings in schema version 1 SHALL NOT change; new
fields MAY be added.

Export SHALL be at least once. TRex SHALL store the last exported `chain_seq`
per sink in the database and advance it only after the lines are written. A
SIEM SHALL deduplicate by `event_id`. Export SHALL run in its own
transaction under the same advisory lock as sealing, so only one replica
exports at a time, a sink only receives committed seals, and a failing sink
does not stop sealing.

#### Scenario: Line format
- GIVEN `--audit-sink=stdout`
- WHEN an event is sealed
- THEN stdout SHALL receive one line that parses as JSON with `"schema_version":1`, `chain_seq`, `hash`, and `event_id`
- AND application logs SHALL stay on stderr (STD-005)

#### Scenario: Restart after a crash
- GIVEN a replica that wrote lines but crashed before saving its export position
- WHEN a replica exports again
- THEN it SHALL resend the unsaved events
- AND it SHALL NOT skip any event

#### Scenario: Truncated tail detected by the SIEM copy
- GIVEN the SIEM's newest event for the service has `chain_seq` 120 and hash `H`
- WHEN verification runs with `--expect-head 120:H` against a database whose chain now ends at 117
- THEN verification SHALL fail and report the missing tail

### Requirement: Chain Verification Command

TRex SHALL provide `NewAuditCommand(serviceName)` with a `verify` subcommand
that accepts the environment's database and audit flags. It SHALL read the
chain in `chain_seq` order and report the first gap, `prev_hash` mismatch,
hash mismatch, or `key_id` that does not match the configured key. It SHALL
report events left unsealed for longer than `--max-unsealed-age` (default
`5m`). `--expect-head <seq>:<hash>` SHALL require the chain to contain that
event. The command SHALL exit `0` when the chain is intact and `1` otherwise.
The TRex binary and `templates/new-project/` SHALL register the command.

#### Scenario: Intact chain
- GIVEN an untouched chain of 1000 sealed events
- WHEN `trex audit verify` runs with the same key
- THEN it SHALL print the verified count and head and exit `0`

#### Scenario: Wrong key
- GIVEN a chain sealed with key A
- WHEN verification runs with key B
- THEN it SHALL report a `key_id` mismatch and exit `1`

### Requirement: Configuration and Defaults

The `serve` command SHALL accept the flags below. `audit verify` SHALL
accept the database flags, `--audit-hmac-key-file`, `--expect-head`, and
`--max-unsealed-age`.

| Flag | Default | Meaning |
|---|---|---|
| `--enable-audit` | `true` | Record, seal, and export audit events |
| `--audit-reads` | `false` | Also record allowed `get`, `list`, and `watch` requests |
| `--audit-sink` | `stdout` | `stdout`, `file:<path>`, or `none` |
| `--audit-hmac-key-file` | empty | HMAC key file |
| `--audit-seal-interval` | `1s` | Time between sealing passes; at least `100ms` |

Invalid values SHALL stop startup with an error. Programs MAY replace the
sink with `audit.WithSink`. With `--enable-audit=false`,
TRex SHALL record nothing, start no sealer, log a warning at startup, and
`audit.Record` SHALL return `nil`. The audit table migration SHALL run
whatever the flag.

#### Scenario: Audit disabled for local development
- GIVEN `--enable-audit=false`
- WHEN a client creates a dinosaur
- THEN no event SHALL be stored
- AND the server SHALL have logged a warning at startup

### Requirement: Audit Metrics

TRex SHALL expose Prometheus metrics on the metrics server:
`trex_audit_events_recorded_total{outcome}`,
`trex_audit_write_failures_total`, `trex_audit_seal_failures_total`,
`trex_audit_sealed_total`, `trex_audit_exported_total`, and
`trex_audit_unsealed_events` (a gauge set by each sealing pass).

#### Scenario: Sealer stalled
- GIVEN sealing fails on every pass
- WHEN an operator reads the metrics
- THEN `trex_audit_seal_failures_total` SHALL increase
- AND `trex_audit_unsealed_events` SHALL show the backlog

### Requirement: Test Database Reset

Test database factories that empty tables between tests SHALL reset
`trex_audit_events` and the export positions despite the append-only trigger,
by disabling triggers only for the reset transaction
(`SET LOCAL session_replication_role = replica`).

#### Scenario: Integration test reset
- GIVEN an integration test that stored audit events
- WHEN the helper resets the database
- THEN `trex_audit_events` SHALL be empty for the next test

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Record at the authorization hook points | TRex already knows subject, action, kind, ID, parents, and surface there (SEC-002). Every plugin is covered by default, and policies, audit, and route metadata cannot drift. |
| Success event inside the request transaction, written before the handler | The event commits exactly when the change commits. A failed insert stops the request before anything changes (fail closed). Buffering responses to write the event after the handler was rejected: it breaks streaming writers and still needs the transaction. |
| Failure, denial, and authentication events outside the transaction | Those transactions roll back, so an event written inside them would vanish. These requests changed nothing, so an occasional lost write is logged and counted instead of failing the response. |
| Seal asynchronously in commit order | Chaining at insert time would hold one global lock for the length of every write request. A separate sealer chains in the order events become visible, with no request-path contention. The cost is a short window (about the seal interval) in which an unsealed event is protected only by the trigger. |
| Hash chain plus SIEM copy | A chain alone cannot detect a truncated tail or a forger who holds the key and the database. An external, append-only SIEM copy that carries `chain_seq` and `hash` closes both gaps; `--expect-head` checks the database against it. |
| HMAC with a key file, plain SHA-256 without one | A key outside the database stops a database administrator from recomputing hashes. Starting without one keeps local development simple, and the warning makes the weaker guarantee explicit. |
| Trigger-enforced append-only table | Stops accidental or application-level edits cheaply. It is not the security boundary, because a superuser can disable triggers; the chain and the SIEM copy are. |
| Own JSON-lines schema (version 1) on stdout | Every SIEM collector (Fluent Bit, Vector, Splunk forwarders, OpenTelemetry Collector) ingests JSON lines from container stdout with no plugin. Logs stay on stderr, so collectors can route the two streams apart. Mapping to OCSF or CEF is left to the collector. |
| Table `trex_audit_events` | The `trex_` prefix avoids clashing with an application kind named `AuditEvent`. |
| String-only `details` | Keeps canonical encoding deterministic across Go, PostgreSQL, and a SIEM, and discourages storing structured payloads that could hold secrets. |
| Retention and a read API are out of scope | Pruning a chain needs signed checkpoints, and a read API needs its own authorization model. Both can be added later without changing schema version 1. |
