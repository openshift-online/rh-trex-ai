# Logging Specification

**Date:** 2026-10-21
**Status:** Active
**ID:** STD-005
**Related:** [gRPC Conventions](../api/grpc-conventions.spec.md), [Secrets Management](../security/secrets-management.spec.md)
**Implements:** `pkg/logger/`, `pkg/config/logging.go`, `pkg/cmd/root.go`, `pkg/environments/framework.go`, `pkg/server/logging/`

---

## Purpose

Define how trex and services built on it write logs, so that every record can reach any slog handler a program installs (for example an OpenTelemetry bridge) together with the request context.

## Requirements

### Requirement: log/slog Only

All trex code SHALL log through the standard library `log/slog` package. Code SHALL NOT import `github.com/golang/glog` or define its own logger interface. Generated code templates SHALL follow the same rule.

#### Scenario: Logging inside a request
- GIVEN code with a `context.Context` in scope (an HTTP request, a gRPC call, an event handler)
- WHEN it writes a log record
- THEN it SHALL use the `...Context` functions (`slog.InfoContext`, `slog.DebugContext`, `slog.WarnContext`, `slog.ErrorContext`) with that context

#### Scenario: Message and attributes
- WHEN code writes a log record
- THEN the message SHALL be a constant string without format verbs
- AND variable data SHALL be passed as attributes with snake_case keys
- AND errors SHALL use the attribute key `error`

### Requirement: Request Attributes From Context

The handler trex installs SHALL add request attributes read from the context to every record: `opid` (set by `OperationIDMiddleware` or `logger.WithOpID`) and `accountID`, each only when present. `logger.WithContextAttrs` SHALL provide this behavior as a wrapper for any `slog.Handler`.

#### Scenario: gRPC call log
- GIVEN a gRPC call whose context carries an operation ID
- WHEN the logging interceptor logs the call
- THEN the record SHALL carry an `opid` attribute with that operation ID

### Requirement: Pluggable Default Handler

`slog.SetDefault` SHALL be the only plug point. At startup trex SHALL install its own handler only when the program has not set its own default. A program that calls `slog.SetDefault` before or after trex's startup SHALL receive every trex record with the original context.

#### Scenario: Program installs an OpenTelemetry handler
- GIVEN a program that calls `slog.SetDefault` with a handler wrapping an OpenTelemetry slog bridge
- WHEN trex logs inside a request whose context carries a span
- THEN the program's handler SHALL receive the record with that context, so the trace and span IDs are reachable

#### Scenario: Standard log package unchanged
- WHEN trex installs its default handler
- THEN output written with the standard `log` package SHALL keep its previous destination and flags

### Requirement: Level and Format Flags

The `serve` and `migrate` commands SHALL accept `--log-level` (`debug`, `info`, `warn`, `error`; default `info`) and `--log-format` (`text`, `json`; default `text`). Logs SHALL be written to stderr. Invalid values SHALL stop startup with an error.

#### Scenario: Deprecated glog flags
- GIVEN a deployment that still passes `-v`, `--logtostderr` or `--alsologtostderr`
- WHEN the service starts
- THEN it SHALL start, print a deprecation warning, and map `-v` of 2 or more to `debug` and lower values to `info`
- AND `--log-level`, when also given, SHALL take precedence over `-v`
- AND a value passed on the command line SHALL take precedence over an environment's flag defaults

### Requirement: No Credentials in Logs

Request and response details logged at debug level SHALL NOT include credential values. The `Authorization`, `Proxy-Authorization`, `Cookie` and `Set-Cookie` headers SHALL be redacted.

#### Scenario: Debug request log
- GIVEN `--log-level=debug`
- WHEN a request with an `Authorization` header is logged
- THEN the logged header value SHALL be `REDACTED`

### Requirement: Request Body Not Logged

The request log SHALL NOT include the request body, because it may hold credentials. At debug level it SHALL include the body length (`request_content_length`) when the client declared one. Formatting the request log SHALL NOT read or replace the request body, and SHALL succeed whatever type wraps the body (for example an OpenTelemetry body wrapper).

#### Scenario: Debug request with a body
- GIVEN `--log-level=debug`
- AND a middleware that wraps the request body in a type `encoding/json` cannot marshal
- WHEN a request with a non-empty body is logged
- THEN the log entry SHALL be valid JSON with `request_content_length` set to the body length
- AND it SHALL NOT contain the body content
- AND the handler SHALL still read the full body

### Requirement: GORM Logs Through slog

The production database session factory SHALL configure GORM with `logger.NewGormLogger`, which writes through slog with the statement context and without terminal color codes. Test session factories SHALL stay silent unless database debug is enabled.

#### Scenario: Debug SQL
- GIVEN `--enable-db-debug`
- WHEN a query runs inside a request
- THEN an Info record with `elapsed`, `rows` and `sql` attributes SHALL be logged with the request context

### Requirement: Fatal Conditions

Code SHALL return errors instead of exiting where the function signature allows. Where a process must stop, it SHALL log at Error level and call `os.Exit(1)`. Code SHALL NOT call `panic` to report errors.
