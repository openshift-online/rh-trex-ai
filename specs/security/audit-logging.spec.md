# Audit Logging Specification

**Date:** 2026-11-13
**Status:** Active
**ID:** SEC-004
**Related:** [Authentication](authentication.spec.md), [Authorization](authorization.spec.md), [Logging](../standards/logging.spec.md)
**Implements:** `components/api-server/pkg/audit/`, `components/api-server/pkg/authz/observation.go`, `components/api-server/pkg/config/audit.go`, `components/api-server/pkg/environments/types.go`, `components/api-server/pkg/environments/framework.go`, `components/api-server/pkg/server/routebuilder.go`, `components/api-server/pkg/server/apiserver.go`, `components/api-server/pkg/server/grpc_server.go`, `components/api-server/pkg/cmd/serve.go`

---

## Purpose

TRex captures who did what, to which resource, when, and with what result,
for every service built on it. Capture is a framework concern, like
authorization (SEC-002): a plugin is covered without writing audit code and
cannot opt out by forgetting a line. Where events go is a deployment concern:
TRex hands each event to a `Recorder` that the environment chooses. The
default recorder writes one JSON line per event to stdout or a file, for a log
pipeline or SIEM (Security Information and Event Management system) to
collect.

## Requirements

### Requirement: Audit Event Record

TRex SHALL define, in package `pkg/audit`, one event record that every audit
entry uses. An event SHALL carry:

| Field | Meaning |
|---|---|
| `event_id` | Unique KSUID of the event; a collector's deduplication key |
| `occurred_at` | UTC time with microsecond precision |
| `actor` | `type` (`user`, `system`, or `anonymous`), `username`, and, when a verified token is present, `subject` (`sub`), `issuer` (`iss`), and `client_id` (`azp`, else `clientId`) |
| `action` | The authorization action (`get`, `list`, `create`, `update`, `patch`, `delete`, `watch`, or a custom verb) |
| `outcome` | `success`, `failure`, `denied`, `unauthenticated`, or `error` |
| `reason` | Short non-secret text explaining a non-success outcome |
| `resource` | `surface`, `kind`, `id`, and `parent` (the path variables other than `id`), as in SEC-002 |
| `transport` | `http`, `grpc`, or `internal` |
| `http` | `method`, `path` (no query string), and the response `status` |
| `grpc` | `method` (full method name) and the status `code` |
| `source` | `ip` (the transport peer address) and `user_agent` |
| `operation_id` | The request operation ID (STD-005 `opid`) |
| `trace_id` | The W3C trace ID of the request, when a `traceparent` header or span context is present |
| `details` | String key/value pairs supplied by application events |

An event MUST NOT contain request or response bodies, query strings, header
values other than `User-Agent`, tokens, passwords, or other credentials.

#### Scenario: Mutation by a user
- GIVEN user `alice` with a verified token
- WHEN she sends `DELETE /api/rh-trex-ai/v1/dinosaurs/d-1` and it succeeds with `204`
- THEN one event SHALL be recorded with `actor.type="user"`, `actor.username="alice"`, `action="delete"`, `outcome="success"`, `resource.kind="dinosaurs"`, `resource.id="d-1"`, `resource.surface="v1"`, `transport="http"`, and `http.status=204`

#### Scenario: No credentials recorded
- GIVEN a request with an `Authorization` header, a `Cookie` header, a query string, and a JSON body
- WHEN it is audited
- THEN the recorded event SHALL contain none of those values

### Requirement: Framework-Captured Requests

TRex SHALL capture audit events for REST and gRPC requests itself, in hooks it
attaches around authentication and authorization. Plugins MUST NOT attach
audit middleware or interceptors. TRex SHALL record one event per request,
after it completes, for:

1. Every request whose action is not `get`, `list`, or `watch`, whatever its outcome (`success` below status 400 or with `codes.OK`, `failure` otherwise).
2. Every request that authorization denies, including denials reported as not found (`HideExistence`) and requests to undeclared routes or methods. Outcome `denied`.
3. Every request whose authorizer or subject enricher returns an error. Outcome `error`.
4. Every request that fails authentication (REST `401`, gRPC `codes.Unauthenticated`) on an authenticated surface, including requests the JWT handler refuses before routing. Outcome `unauthenticated`, `actor.type="anonymous"`.
5. Allowed `get`, `list`, and `watch` requests only on kinds registered with `audit.RecordReads(kind...)`, or on every kind when `--audit-reads=true`.

Routes outside the authenticated surfaces (OpenAPI, `/auth/config`, metadata,
health, metrics), gRPC bypass methods, and REST requests that match no route
or no route method (`404`/`405` from the router) SHALL NOT be recorded. A gRPC
stream SHALL be recorded when it ends; `codes.Canceled` (the client closed it)
SHALL count as success.

Reads are opt-in per kind because most reads (for example listing gateways)
carry little audit value, while some (for example reading secrets) carry a
lot. A plugin SHALL opt a kind in from its `init()`; the registration applies
to REST and gRPC alike.

#### Scenario: Plugin without audit code
- GIVEN a plugin that registers routes with `r.Resource("fossils")` and contains no audit code
- WHEN a client creates a fossil
- THEN TRex SHALL record an event with `action="create"` and `resource.kind="fossils"`

#### Scenario: Failed mutation
- GIVEN an allowed `PATCH` whose handler responds `404`
- WHEN the request completes
- THEN one event with `outcome="failure"` and `http.status=404` SHALL be recorded

#### Scenario: Denied as not found
- GIVEN the authorizer returns `Decision{Allowed: false, HideExistence: true}`
- WHEN the request is audited
- THEN the event SHALL have `outcome="denied"` even though the client receives `404`

#### Scenario: Authentication failure
- GIVEN JWT authentication is enabled
- WHEN a request to `/api/rh-trex-ai/v1/dinosaurs` carries no valid token
- THEN an event SHALL be recorded with `outcome="unauthenticated"` and `actor.type="anonymous"`

#### Scenario: Allowed read not recorded by default
- GIVEN `--audit-reads=false` and no `RecordReads("dinosaurs")`
- WHEN an allowed `GET /api/rh-trex-ai/v1/dinosaurs` succeeds
- THEN no event SHALL be recorded

#### Scenario: Reads of an opted-in kind
- GIVEN a plugin calls `audit.RecordReads("secrets")` in `init()`
- WHEN an allowed `GET /api/rh-trex-ai/v1/secrets/s-1` or a gRPC `GetSecret` call succeeds
- THEN an event with `action="get"` and `resource.kind="secrets"` SHALL be recorded
- AND reads of other kinds SHALL still not be recorded

#### Scenario: gRPC mutation
- GIVEN `UpdateDinosaur` is declared as `{Kind: "dinosaurs", Action: "update"}`
- WHEN a client calls it and it returns `codes.NotFound`
- THEN an event SHALL be recorded with `transport="grpc"`, `outcome="failure"`, `grpc.method="/rh_trex.v1.DinosaurService/UpdateDinosaur"`, and `grpc.code="NotFound"`

### Requirement: Recorder Interface

`pkg/audit` SHALL define:

```go
type Recorder interface {
    Record(ctx context.Context, e Event) error
}
```

TRex SHALL hand every captured event to the environment's recorder. Record
SHALL be called after the request completes, with a context that is not
canceled with the request and that has a 5-second deadline. A recorder SHALL
be safe for concurrent use. When a recorder also implements `io.Closer`, TRex
SHALL close it when `serve` shuts down.

The gRPC hook SHALL run outside the transaction interceptor, so a recorder
that writes to the service database does so after the call has released its
connection.

#### Scenario: Custom recorder
- GIVEN an environment whose `OverrideAudit` returns a recorder that ships events to a message queue
- WHEN a client deletes a dinosaur
- THEN that recorder SHALL receive the event
- AND no JSON line SHALL be written by TRex

### Requirement: Environment Selects the Recorder

`EnvironmentImpl` SHALL include a required method:

```go
OverrideAudit(env *Env) (audit.Recorder, error)
```

The framework SHALL call it after `OverrideAuthorizer`, build `Env.Audit` from
its result and the audit configuration, and log the recorder's type at
startup. A `nil` recorder or an error SHALL stop initialization. TRex's own
production and development environments, and those in
`templates/new-project/`, SHALL return `audit.DefaultRecorder(env.Config.Audit)`;
its testing environments SHALL return `audit.NewMemoryRecorder()`.

#### Scenario: Nil recorder
- GIVEN `OverrideAudit` returns `nil` and no error
- WHEN the environment initializes
- THEN initialization SHALL fail with an error

### Requirement: Default JSON-Lines Recorder

`audit.DefaultRecorder` SHALL write each event as one JSON object per line to
the destination selected by `--audit-sink`: `stdout` (default) or
`file:<path>` (opened for append, mode `0600`). Each line SHALL contain every
event field plus `schema_version` (`1`). Field names and meanings in schema
version 1 SHALL NOT change; new fields MAY be added. Lines SHALL be JSON
whatever `--log-format` says, because they are a machine contract, and SHALL
stay on stdout while logs stay on stderr (STD-005), so collectors can route the
two streams apart.

Before encoding, text SHALL be made valid UTF-8 without NUL bytes and
client-controlled text SHALL be bounded to 2048 bytes. Map keys SHALL be
sorted, HTML SHALL NOT be escaped, and empty maps SHALL be `{}`.

`pkg/audit` SHALL also provide `NewJSONRecorder(io.Writer)` and, for tests,
`NewMemoryRecorder()`.

#### Scenario: Line format
- GIVEN the default recorder with `--audit-sink=stdout` and `--log-format=text`
- WHEN an event is recorded
- THEN stdout SHALL receive one line that parses as JSON with `"schema_version":1` and an `event_id`
- AND application logs SHALL stay on stderr in text format

### Requirement: Application Events

`pkg/audit` SHALL provide `audit.Record(ctx, audit.Event) error` for events a
request does not express, such as a background job, a controller action, or a
domain step inside a handler. Missing `event_id`, `occurred_at`,
`operation_id`, `trace_id`, and, inside a request, actor fields SHALL be filled
from the context. An event without an actor outside a request SHALL have
`actor.type="system"` and `transport="internal"`. An event without an action,
or with an unknown outcome or actor type, SHALL be rejected with an error.
The recorder's error SHALL be returned to the caller.

#### Scenario: Controller action
- GIVEN a controller handler with no request in its context
- WHEN it calls `audit.Record` with `action="reconcile"` and `resource.kind="dinosaurs"`
- THEN an event SHALL be recorded with `actor.type="system"` and `transport="internal"`

### Requirement: Recording Failures

A recorder error for an event captured by a request hook SHALL be logged at
Error level and counted in `trex_audit_record_failures_total`. It SHALL NOT
change the response, which has already been decided.

#### Scenario: Collector unavailable
- GIVEN a recorder whose destination is unavailable
- WHEN a client creates a dinosaur
- THEN the client SHALL receive `201`
- AND the failure SHALL be logged and counted

### Requirement: Configuration and Defaults

The `serve` command SHALL accept:

| Flag | Default | Meaning |
|---|---|---|
| `--enable-audit` | `true` | Capture audit events |
| `--audit-reads` | `false` | Record allowed `get`, `list`, and `watch` requests on every kind, not only kinds registered with `audit.RecordReads` |
| `--audit-sink` | `stdout` | Destination of the default recorder: `stdout` or `file:<path>` |

Invalid values SHALL stop startup with an error. With `--enable-audit=false`,
TRex SHALL capture nothing, log a warning at startup, and `audit.Record` SHALL
return `nil`.

#### Scenario: Audit disabled for local development
- GIVEN `--enable-audit=false`
- WHEN a client creates a dinosaur
- THEN no event SHALL be recorded
- AND the server SHALL have logged a warning at startup

### Requirement: Audit Metrics

TRex SHALL expose `trex_audit_events_total{outcome}` and
`trex_audit_record_failures_total` on the metrics server.

#### Scenario: Counting outcomes
- GIVEN three successful creates and one denied delete
- WHEN an operator reads the metrics
- THEN `trex_audit_events_total` SHALL show 3 for `success` and 1 for `denied`

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| TRex owns capture; the environment owns storage | Capture (hook placement, ordering, route metadata, field hygiene) is the part every service gets wrong when it does it alone. Storage depends on each deployment's log pipeline, SIEM, and compliance needs, so it sits behind one interface. |
| Capture at the authorization hook points | TRex already knows subject, action, kind, ID, parents, and surface there (SEC-002). Every plugin is covered by default, and policies, audit, and route metadata cannot drift. |
| JSON lines on stdout by default | No database writes and no new infrastructure. Every collector (Fluent Bit, Vector, Splunk forwarders, OpenTelemetry Collector) ingests JSON lines from container stdout. Kubernetes' API server audit log takes the same approach. |
| Required `OverrideAudit` hook | Matches `OverrideAuthorizer` (SEC-002): explicit, runs after services exist, and the compiler makes every environment choose. A one-line `return audit.DefaultRecorder(env.Config.Audit)` keeps the default easy. |
| One event per request, after it completes | The event carries the final status and outcome. Recording before the handler would need a second event or an update for the result. |
| Read auditing opt-in per kind | Most reads have little audit value and would dominate volume; some (secrets) matter. A per-kind registration covers REST and gRPC in one line. |
| Known limits of the default | Events are recorded after the change commits, so a crash between commit and write can lose one, and a recorder failure does not fail the request. The default trail is not tamper-evident by itself: its integrity depends on shipping lines promptly to a store administrators cannot edit. A recorder that writes in the request transaction and hash-chains events can lift both limits; a prototype is kept on the `feat/audit-logging-chain` branch for when a requirement calls for it. |
| `created_by`/`updated_by` fields are separate | Last-writer fields on entities are useful to users but are not an audit trail (one writer, overwritten, no denials). They are left to a separate spec. |
| String-only `details` | Keeps encoding deterministic and discourages structured payloads that could hold secrets. |
