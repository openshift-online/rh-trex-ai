# Secrets Management Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** SEC-003
**Implements:** `pkg/config/`, `secrets/`

---

## Purpose

Define the secrets management practices ensuring credentials are never exposed in logs, source code, or API responses.

## Requirements

### Requirement: File-Based Secret Loading

All secrets (database credentials, TLS keys, Sentry keys) SHALL be loaded from files, not environment variables or command-line arguments.

#### Scenario: Database credential loading
- GIVEN configuration flags `--db-host-file`, `--db-name-file`, `--db-user-file`, `--db-password-file`, `--db-port-file`
- WHEN the server starts
- THEN each file SHALL be read to obtain the corresponding credential value
- AND default paths SHALL point to `secrets/db.{field}`

### Requirement: No Secret Logging

Secrets SHALL NEVER be logged, even at debug log levels.

#### Scenario: Token logging
- GIVEN code that processes a JWT token or database password
- WHEN debug logging is enabled
- THEN only `len(token)` or `"[REDACTED]"` SHALL be logged
- AND the actual secret value SHALL never appear in log output

### Requirement: Secret File Gitignore

The `secrets/` directory SHALL be listed in `.gitignore` to prevent accidental commits.

#### Scenario: Accidental secret commit
- GIVEN a developer creates `secrets/db.password` with a real credential
- WHEN `git add .` is executed
- THEN the secrets directory SHALL be excluded from staging

### Requirement: TLS Certificate Management

TLS certificates for HTTPS and gRPC SHALL be loaded from file paths configured via flags.

#### Scenario: TLS configuration
- GIVEN `--https-cert-file` and `--https-key-file` are specified
- WHEN the API server starts with `--enable-https`
- THEN the TLS certificate and key SHALL be loaded from the specified files
- AND the server SHALL support TLS 1.2 as the minimum version

### Requirement: Development Defaults

Default secret file paths SHALL work for local development without requiring production credentials.

#### Scenario: Local development setup
- GIVEN default configuration with `secrets/db.host` containing "localhost"
- WHEN `make run` is executed
- THEN the server SHALL connect to the local PostgreSQL container using file-based credentials

### Requirement: No Upstream API Client Credentials

The server SHALL NOT read upstream API client credentials (the former OCM/AMS client's `secrets/api-service.*` files) in any environment.
The old API client flags `--enable-mock`, `--debug`, `--api-client-id-file`, `--api-client-secret-file`, `--self-token-file`, `--api-base-url` and `--api-token-url` are removed. An environment or command line that still sets one SHALL fail at startup with a "no such flag" error.

#### Scenario: Production start without API client secrets
- GIVEN the `production` environment and no `secrets/api-service.clientId` or `secrets/api-service.clientSecret` file
- WHEN the server starts
- THEN it SHALL start without a configuration-file error

#### Scenario: Old flags still set
- GIVEN a command line, or an environment `Flags()` map, that sets `enable-mock` or `debug`
- WHEN the server starts
- THEN it SHALL fail at startup with a "no such flag" error

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| File-based over environment variables | Safer in container environments; supports Kubernetes secret mounts |
| Default paths in `secrets/` | Convention-based; zero-config for local development |
| Separate file per credential | Fine-grained Kubernetes secret mounting; independent rotation |
| TLS minimum version 1.2 | Red Hat security baseline; TLS 1.0/1.1 are deprecated |
