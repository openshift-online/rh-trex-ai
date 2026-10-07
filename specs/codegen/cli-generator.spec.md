# CLI Generator Specification

**Date:** 2026-08-03
**Status:** Active
**ID:** CG-002
**Related:** [REST Conventions](../api/rest-conventions.spec.md), [Testing Standards](../standards/testing.spec.md), [OpenAPI Intermediate Representation](openapi-ir.spec.md)
**Implements:** `scripts/cli-generator/`

---

## Purpose

Define the CLI tool generator that scaffolds complete command-line interfaces from OpenAPI specifications.

## Requirements

### Requirement: OpenAPI-Driven Generation

The CLI generator SHALL use the canonical OpenAPI IR to discover resource views and their documented operations.

#### Scenario: CLI generation from spec
- GIVEN a project with `openapi/openapi.yaml` exposing complete CRUD operations for Dinosaur, Fossil, and Scientist resource views
- WHEN the CLI generator runs
- THEN commands SHALL be generated for each entity: `list`, `get`, `create`, `update`, `delete`
- AND an operation that is absent from the OpenAPI document SHALL NOT produce a command

### Requirement: Shared IR Consumption

The CLI generator SHALL consume the shared normalized OpenAPI IR and SHALL NOT maintain an independent raw OpenAPI parser or schema-to-resource heuristic.

#### Scenario: Helper schema
- GIVEN the canonical IR contains `AgentPatchRequest` only as a request schema
- WHEN the CLI generator runs
- THEN it SHALL NOT generate an `agent-patch-requests` command group

#### Scenario: Singleton endpoint
- GIVEN a `GET` on a path without a trailing parameter, such as `/metadata`, whose response is one object and not a list
- WHEN the CLI generator runs
- THEN it SHALL NOT generate list, get or create commands for that object's schema

### Requirement: Scoped Path Fidelity

Generated commands SHALL bind every parameter required by their operation's exact path, including parameters that scope a resource view through one or more parents.

#### Scenario: Nested inbox command
- GIVEN `listAgentInbox` has path `/organizations/{organization_id}/agents/{agent_id}/inbox`
- WHEN the corresponding CLI command is generated
- THEN it SHALL require or otherwise resolve both `organization_id` and `agent_id`
- AND it SHALL call the exact documented path

### Requirement: Generated CLI Acceptance Tests

The CLI generator SHALL have black-box acceptance tests that generate a standalone CLI into a temporary directory, build it, inspect its command surface, and execute representative commands against an in-process mock HTTP server. The tests SHALL verify operation-derived command presence, absence of unsupported commands, flags and required inputs, exact methods and paths, query and body serialization, and authentication behavior. Covered legacy cases SHALL use the same assertions before and after migration to the canonical IR.

#### Scenario: Generated scoped command
- GIVEN a fixture exposes a parent-scoped operation with path, query, body, and Bearer authentication inputs
- WHEN the generated CLI command is built and executed against the mock server
- THEN the observed request SHALL match the documented method, expanded path, query, body, and authorization header
- AND no command SHALL exist for an operation absent from the fixture

### Requirement: Delete and Update Commands

For each top-level resource whose item view (the collection path plus one trailing path parameter) declares a DELETE operation, the generated CLI SHALL provide `delete <resource> ID`; for each that declares a PATCH operation, or a PUT operation when no PATCH exists, it SHALL provide `update <resource> ID`. A command SHALL NOT be generated for an operation the OpenAPI document does not declare, and the `delete` and `update` parent commands SHALL exist only when at least one resource has the corresponding command. Scoped resources and non-CRUD action operations are not covered by these commands.

`delete` SHALL ask for confirmation before sending any request. It SHALL write the prompt to standard error, SHALL proceed only on `y` or `yes` (case-insensitive) or when `--yes` is given, and SHALL fail with an error and a non-zero exit status, without sending a request, when the user declines or when standard input is not a terminal and `--yes` is absent. It SHALL treat 200, 202 and 204 as success and print `<Kind> <id> deleted.` to standard output. The confirmation question SHALL be replaceable per resource without replacing the command.

`update` SHALL take the resource ID as its single positional argument and SHALL offer one flag per writable field of the operation's request body schema, falling back to the resource's writable fields, plus `--body` for a JSON file. When built from flags it SHALL send only the flags that were explicitly set, so an explicit zero value is sent and an unset flag is omitted, and it SHALL fail without sending a request when no field is set. It SHALL use the operation's declared method.

#### Scenario: Delete requires confirmation
- GIVEN a resource whose item path declares DELETE
- WHEN `delete <resource> ID` runs with standard input that is not a terminal and no `--yes`
- THEN the command SHALL fail naming `--yes` and SHALL NOT send a request
- AND with `--yes` it SHALL send exactly one DELETE to the item path and print `<Kind> <id> deleted.`

#### Scenario: Declined confirmation is a failure
- GIVEN an interactive terminal
- WHEN the user answers anything other than `y` or `yes`
- THEN the command SHALL exit non-zero and SHALL NOT send a request

#### Scenario: Update sends only the flags that were set
- GIVEN a resource whose item path declares PATCH
- WHEN `update <resource> ID --field value` runs
- THEN exactly one PATCH SHALL be sent to the item path whose body contains only that field
- AND running it with no field flag and no `--body` SHALL fail without sending a request

#### Scenario: Operations absent from the specification produce no command
- GIVEN an OpenAPI document with a list and a get operation for a resource and no delete or update operation
- WHEN the CLI is generated
- THEN no delete or update command, parent command, registration or confirmation package SHALL be generated

#### Scenario: Customize a generated delete or update
- GIVEN a project needs different confirmation text, extra flags or a different output contract for one resource
- WHEN it sets that resource's `ConfirmPrompt`, or removes the generated command from its parent with `RemoveCommand` and adds its own from the hand-maintained `main.go`
- THEN regeneration SHALL NOT overwrite that customization
- AND the generated parent `delete` and `update` commands SHALL remain registered by the regenerated code

### Requirement: Authentication Integration

The generated CLI SHALL support login with a bearer token the user already has and with OpenID Connect, SHALL renew an OpenID Connect session automatically, SHALL revoke it on logout, and SHALL report the logged-in identity.

`login` SHALL take the token path (`--token-file`, `--token`) exactly as before unless `--issuer-url` is given. A token login SHALL clear any saved OpenID Connect session fields. `--issuer-url` combined with a token flag SHALL be rejected. `login` SHALL return errors rather than exit the process.

With `--issuer-url` the CLI SHALL discover endpoints from `<issuer>/.well-known/openid-configuration`, SHALL reject a discovery document whose issuer differs, and SHALL accept a plain-HTTP issuer only for loopback hosts or with `--insecure`. It SHALL log in as a public client (`--client-id`, defaulting to the generator's `--oidc-client-id` option and otherwise to the binary name; the generator SHALL reject an option value that is not safe to write into source) requesting the scopes given by `--scope` (default `openid`). By default it SHALL use the authorization code flow with PKCE (S256) on a loopback redirect (`127.0.0.1`, an OS-assigned port, RFC 8252), SHALL print the authorization URL, and SHALL abort when the callback state does not match. With `--no-browser` it SHALL use the device authorization grant, honoring the polling interval and `slow_down`. It SHALL save the access token, refresh token, issuer URL, client ID and access token expiry in the configuration, SHALL write the file with mode `0600`, and SHALL NOT include token values in errors or logs.

The default token provider SHALL, when the configuration holds a refresh token, issuer URL and client ID, renew the access token before a request when it is expired or within a small margin of expiry, using the JWT `exp` claim or, for opaque tokens, the saved expiry. It SHALL save the rotated refresh token, SHALL adopt a newer session already saved by another invocation instead of refreshing with a stale token, SHALL NOT refresh a token that is still fresh, and SHALL fail with a "session expired, run login" error and send no API request when the issuer rejects the refresh token, leaving the configuration unchanged. Saving SHALL go through the registered configuration store. The connection package SHALL export this default provider (`NewConfigTokenProvider`) so a project that builds its own connection, or hands a token provider to another client such as the TUI, reuses the same refresh behavior instead of copying it.

`logout` SHALL revoke the saved refresh token at the discovered revocation endpoint on a best-effort basis, printing a warning and continuing when revocation fails or is unsupported, and SHALL then clear every saved credential field.

`whoami` SHALL decode the saved access token without verifying its signature and print the user (`preferred_username`, else `sub`), email, issuer, API URL and expiry, with `--show-token` and `--show-token-decoded` to print the raw token and the claims. It SHALL NOT fail for a token that is not a JWT, printing the API URL and that the token is opaque. The decoded claims are for display only and SHALL NOT be used for any authorization decision.

#### Scenario: CLI login with a token
- GIVEN a generated CLI tool and a saved OpenID Connect session
- WHEN `mytool login --token-file token --url URL` is executed
- THEN the token SHALL be stored for subsequent API calls
- AND the saved refresh token, issuer URL, client ID and expiry SHALL be removed

#### Scenario: Issuer URL selects OpenID Connect
- GIVEN an issuer that serves discovery, device authorization and token endpoints
- WHEN `mytool login --issuer-url ISSUER --no-browser` is executed
- THEN the CLI SHALL print the verification URL and user code and poll until the user approves
- AND the access token, refresh token, issuer URL, client ID and expiry SHALL be saved

#### Scenario: Default client ID is configurable at generation time
- GIVEN the generator is run with `--oidc-client-id custom-client`
- WHEN the CLI is generated
- THEN `login --client-id` SHALL default to `custom-client`
- AND without the option it SHALL default to the binary name

#### Scenario: Expired access token is renewed once
- GIVEN a saved expired access token and a refresh token
- WHEN a generated command runs
- THEN exactly one refresh SHALL be made before the API request and the request SHALL carry the new token
- AND the rotated refresh token SHALL be saved and used for the next renewal

#### Scenario: Rejected refresh token
- GIVEN a saved expired access token whose refresh token the issuer rejects
- WHEN a generated command runs
- THEN it SHALL fail with a session-expired error, send no API request, and leave the configuration unchanged

#### Scenario: Forged callback state
- GIVEN a browser login in progress
- WHEN the loopback callback carries a state other than the one issued
- THEN the login SHALL abort and no tokens SHALL be requested

#### Scenario: Logout revokes
- GIVEN a saved OpenID Connect session
- WHEN `mytool logout` is executed
- THEN the latest saved refresh token SHALL be revoked at the issuer and every credential field cleared
- AND a revocation failure SHALL print a warning and still clear the credentials

#### Scenario: Whoami with an opaque token
- GIVEN a saved access token that is not a JWT
- WHEN `mytool whoami` is executed
- THEN it SHALL succeed, print the API URL, and state that the token is opaque

### Requirement: Configurable Config File Name

The configuration file location SHALL derive from a config name that defaults to the binary name and that the generator SHALL accept as an option (`--config-name`) so a project can keep the file names its users already have. The config name SHALL determine the home-directory file `~/.<name>.json`, the `<user config dir>/<name>/config.json` fallback, and the override environment variable, which SHALL be the name upper-cased with `-` and `.` replaced by `_`, followed by `_CONFIG`. The generator SHALL reject a name that is not letters, digits, `.`, `_` and `-` starting with a letter or digit, or that contains `..`, because it becomes a file name and an environment variable name.

#### Scenario: Custom config name
- GIVEN the generator is run with `--config-name my-app.v2` for a binary named `mytool`
- WHEN the generated CLI resolves its configuration location
- THEN `MY_APP_V2_CONFIG` SHALL override the path, an existing `~/.my-app.v2.json` SHALL be used, and otherwise `<user config dir>/my-app.v2/config.json` SHALL be used
- AND without the option the same rules SHALL apply to the binary name

### Requirement: Downstream Extension Hooks

The generated CLI SHALL let a downstream project change HTTP behavior, authentication and configuration persistence without modifying generator templates, and SHALL apply those changes to every generated command. The connection package SHALL define `HTTPClient` and `TokenProvider` interfaces, SHALL offer `New(httpClient, tokenProvider, baseURL)` for direct injection, and SHALL let a project register factories for them (`SetHTTPClientFactory`, `SetTokenProviderFactory`) that the connection builder used by generated commands consults before falling back to the default client and the default token provider. The config package SHALL define a `Store` interface with `Load` and `Save` and SHALL let a project register one (`SetStore`) that `Load` and `Save`, and therefore every generated command including `login`, `logout` and token renewal, delegate to before falling back to the default file-backed implementation; `Location` SHALL NOT change. Registering `nil` SHALL restore a default, and registration SHALL be safe for concurrent use. The default HTTP client SHALL honor the standard `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` environment variables, with and without the insecure option, so a generated CLI works behind a proxy without registering a custom client.

#### Scenario: Registered factories apply to generated commands
- GIVEN a project registers an HTTP client factory and a token provider factory
- WHEN a generated command builds its connection
- THEN the request SHALL go through the registered client with the registered provider's token
- AND registering `nil` SHALL restore the default client and token provider

#### Scenario: Registered config store replaces the file
- GIVEN a project registers a config store and a generated command loads and saves configuration
- WHEN `login` or `logout` saves and a later command loads
- THEN both SHALL use the registered store and the default config file SHALL NOT be written
- AND registering `nil` SHALL restore the file-backed behavior

#### Scenario: Default client honors proxy environment
- GIVEN `HTTPS_PROXY` is set when the generated CLI starts and `NO_PROXY` does not cover the target host
- WHEN the default HTTP client issues a request to an HTTPS API host
- THEN the request SHALL be sent through the configured proxy
- AND the same SHALL hold when the insecure option is enabled

### Requirement: Hand-Maintained Entry Point

The generator SHALL create `cmd/<binary>/main.go` and `go.mod` only when they are absent and SHALL NOT overwrite them. It SHALL register every generated command in `cmd/<binary>/generated_commands.go`, rewritten on every generation, through one function that the generated `main.go` calls, so newly generated commands become reachable without editing `main.go`. When an existing `main.go` does not call that function, the generator SHALL print a warning that newly generated commands are not registered. The generated connection package SHALL keep `NewConnection()` returning the connection builder, so hand-written commands that build a connection keep compiling after regeneration.

#### Scenario: Regeneration adds a command
- GIVEN a generated CLI whose `main.go` was edited by hand
- WHEN the OpenAPI document gains an operation and the CLI is regenerated
- THEN `main.go` SHALL be unchanged
- AND the new command SHALL be registered

#### Scenario: Entry point that predates generated registration
- GIVEN an existing `main.go` that registers commands itself
- WHEN the CLI is regenerated
- THEN `main.go` SHALL be unchanged
- AND the generator SHALL warn that generated commands are not registered through it

### Requirement: Standalone Module

The CLI generator SHALL produce a standalone Go module with its own `go.mod`.

#### Scenario: Independent build
- GIVEN the generated CLI code
- WHEN `go build` is run in the CLI directory
- THEN the CLI binary SHALL build without depending on the API server's module

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Separate `go.mod` | CLI is a client, not part of the server; independent versioning |
| OpenAPI as source of truth | Ensures CLI matches API exactly; single spec drives both |
| Commands project operations, not schemas | Avoids commands for helper models and unsupported CRUD assumptions |
| Test the generated binary boundary | Command registration, flags, configuration, authentication, and HTTP behavior can regress even when generator internals compile |
