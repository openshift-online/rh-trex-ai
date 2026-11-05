# Authorization Specification

**Date:** 2026-11-04
**Status:** Active
**ID:** SEC-002
**Related:** [Authentication](authentication.spec.md), [REST Conventions](../api/rest-conventions.spec.md), [gRPC Conventions](../api/grpc-conventions.spec.md)
**Implements:** `pkg/authz/`, `pkg/environments/types.go`, `pkg/environments/framework.go`, `pkg/server/routes.go`, `pkg/server/routebuilder.go`, `pkg/server/grpc_server.go`

---

## Purpose

TRex owns authorization as a framework concern. It decides *where* and *when*
authorization runs, on REST and gRPC alike. A downstream application decides
*the policy*, by supplying one `Authorizer` through its environment. TRex
gives the policy structured input (subject, action, resource), so the policy
never parses URLs or gRPC method names. Every route and every gRPC method is
protected by default.

This spec replaces the previous per-plugin `AuthorizationMiddleware` design.
Breaking changes for downstream applications are accepted.

## Requirements

### Requirement: Authorizer Interface

TRex SHALL define, in package `pkg/authz`, one interface that makes every
authorization decision for REST and gRPC:

```go
type Authorizer interface {
    Authorize(ctx context.Context, req Request) (Decision, error)
}

type Request struct {
    Subject    Subject
    Action     string        // get, list, create, update, patch, delete, watch, or a custom verb
    Resource   Resource
    HTTP       *http.Request // REST only
    GRPCMethod string        // gRPC only, full method name
    GRPCMsg    any           // gRPC only, the request message
}

type Subject struct {
    Username string
    Claims   jwt.MapClaims // verified token claims; nil when JWT is disabled
}

type Resource struct {
    Surface string            // "v1", or a route prefix such as "ext"; empty on gRPC
    Kind    string            // e.g. "dinosaurs"
    ID      string            // empty for list and create
    Parent  map[string]string // other path variables, e.g. {"dinosaur_id": "d-1"}
}

type Decision struct {
    Allowed       bool
    Reason        string
    HideExistence bool // deny as "not found"
}
```

`pkg/auth` SHALL remain about authentication only.

#### Scenario: Allowed decision
- GIVEN an authenticated request
- WHEN the `Authorizer` returns `Decision{Allowed: true}` and no error
- THEN the request SHALL reach the handler

#### Scenario: Denied decision
- GIVEN an authenticated request
- WHEN the `Authorizer` returns `Decision{Allowed: false}`
- THEN REST SHALL respond `403 Forbidden`
- AND gRPC SHALL return `codes.PermissionDenied`

#### Scenario: Denied with hidden existence
- GIVEN the `Authorizer` returns `Decision{Allowed: false, HideExistence: true}`
- WHEN the response is written
- THEN REST SHALL respond `404 Not Found`
- AND gRPC SHALL return `codes.NotFound`

#### Scenario: Authorizer error
- GIVEN the `Authorizer` returns a non-nil error
- WHEN the response is written
- THEN the request MUST NOT be allowed
- AND REST SHALL respond `503 Service Unavailable`
- AND gRPC SHALL return `codes.Unavailable`

### Requirement: Built-in Policies

TRex SHALL provide two `Authorizer` implementations:

1. `authz.RequireIdentity()` allows a request when `Subject.Username` is not empty.
2. `authz.AllowAll()` allows every request.

#### Scenario: RequireIdentity without a username
- GIVEN the active authorizer is `RequireIdentity()`
- WHEN a request arrives with an empty `Subject.Username`
- THEN the request SHALL be denied

#### Scenario: AllowAll
- GIVEN the active authorizer is `AllowAll()`
- WHEN any request arrives
- THEN the request SHALL be allowed

### Requirement: Environment Selects the Authorizer

`EnvironmentImpl` SHALL include a required method:

```go
OverrideAuthorizer(env *Env) (authz.Authorizer, error)
```

`Env` SHALL have an `Authorizer authz.Authorizer` field. The framework SHALL
call `OverrideAuthorizer` after `OverrideServices`, store the result in
`Env.Authorizer`, and log the authorizer's type at startup.

TRex's own environments SHALL return `authz.FromFlag(enableAuthz)`:
`RequireIdentity()` when `--enable-authz=true` and `AllowAll()` when
`--enable-authz=false`. Other environments MAY ignore `--enable-authz`.

#### Scenario: Policy built from services
- GIVEN an environment whose policy needs a registered service
- WHEN `OverrideAuthorizer` is called
- THEN all services SHALL already be loaded

#### Scenario: Nil authorizer
- GIVEN `OverrideAuthorizer` returns `nil` and no error
- WHEN the environment initializes
- THEN initialization SHALL fail with an error

#### Scenario: Hook error
- GIVEN `OverrideAuthorizer` returns an error
- WHEN the environment initializes
- THEN initialization SHALL fail and report that error

### Requirement: Subject Enrichment

An `Authorizer` MAY also implement:

```go
type SubjectEnricher interface {
    Enrich(ctx context.Context, s Subject) (context.Context, error)
}
```

When it does, TRex SHALL call `Enrich` once per request, after
authentication and before `Authorize`, on REST and gRPC. The context `Enrich`
returns SHALL be used for `Authorize` and passed on to the handler.
`authz.SubjectFromContext(ctx)` SHALL return the verified `Subject`.

#### Scenario: Local user mapped before the decision
- GIVEN an authorizer that implements `SubjectEnricher`
- AND `Enrich` stores a local user ID in the context
- WHEN a request is authorized
- THEN `Authorize` SHALL see that user ID in its context
- AND the handler SHALL see it too

#### Scenario: Enrich error
- GIVEN `Enrich` returns an error
- WHEN a request arrives
- THEN `Authorize` MUST NOT be called
- AND the request SHALL fail as in "Authorizer error"

### Requirement: Framework-Enforced REST Authorization

TRex SHALL attach the authorization middleware itself, after authentication,
to the `/v1` router and to every prefixed router. Plugins MUST NOT attach
authorization. `RouteRegistrationFunc` SHALL NOT receive an authentication or
authorization middleware:

```go
type RouteRegistrationFunc func(r *APIRouter, services ServicesInterface)
```

Routes outside `/v1` and the prefixed routers (OpenAPI, `/auth/config`,
metadata) SHALL keep their current unauthenticated behavior.

When JWT authentication is disabled, TRex SHALL set the local development
username `dev-user` on REST and gRPC requests, so policies such as
`RequireIdentity` still see a subject.

#### Scenario: Plugin without authorization code
- GIVEN a plugin that registers routes and does not call any authorization middleware
- WHEN a request reaches one of its routes
- THEN the active `Authorizer` SHALL be called before the handler

#### Scenario: Authorization runs after authentication
- GIVEN JWT authentication is enabled
- WHEN the authorization middleware runs
- THEN the verified username SHALL already be available as `Subject.Username`

### Requirement: Resource Declaration on REST

`APIRouter` SHALL provide:

1. `Resource(kind string) *mux.Router`: a subrouter whose requests are tagged with `kind`.
2. `Public() *mux.Router`: a subrouter whose requests skip authorization but stay authenticated.

The middleware SHALL fill the `Request` as follows:

| Field | Source |
|---|---|
| `Resource.Kind` | the tag set by `Resource(kind)` |
| `Resource.ID` | the `{id}` path variable |
| `Resource.Parent` | every other path variable, keyed by its name (e.g. `dinosaur_id`) |
| `Action` | GET with ID → `get`; GET without ID → `list`; POST → `create`; PUT → `update`; PATCH → `patch`; DELETE → `delete` |
| `HTTP` | the request |

#### Scenario: Get by ID
- GIVEN a plugin registered `r.Resource("dinosaurs")` with route `/{id}`
- WHEN `GET /api/rh-trex-ai/v1/dinosaurs/d-1` arrives
- THEN `Authorize` SHALL receive `Action="get"`, `Kind="dinosaurs"`, `ID="d-1"`, `Surface="v1"`

#### Scenario: Nested resource
- GIVEN a route `/dinosaurs/{dinosaur_id}/fossils` on `r.Resource("fossils")`
- WHEN `POST /api/rh-trex-ai/v1/dinosaurs/d-1/fossils` arrives
- THEN `Authorize` SHALL receive `Action="create"`, `Kind="fossils"`, `Parent={"dinosaur_id": "d-1"}`

#### Scenario: Undeclared route fails closed
- GIVEN a route registered on the router without `Resource()` or `Public()`
- WHEN a request reaches that route
- THEN the request SHALL be denied with `403 Forbidden`
- AND `Authorize` MUST NOT be called

#### Scenario: Public route
- GIVEN a route registered on `r.Public()`
- WHEN an authenticated request reaches it
- THEN `Authorize` MUST NOT be called

### Requirement: Prefixed Surfaces

Routes registered with `RegisterPrefixedRoutes(name, prefix, fn)` SHALL be
authorized exactly like `/v1` routes. `Resource.Surface` SHALL be the prefix
for those routes and `"v1"` for the main surface.

#### Scenario: Prefixed route is protected
- GIVEN a plugin registered under prefix `ext`
- WHEN a request reaches `/api/rh-trex-ai/ext/<kind>`
- THEN the active `Authorizer` SHALL be called
- AND `Resource.Surface` SHALL be `"ext"`

#### Scenario: Same kind on two surfaces
- GIVEN kind `fossils` is registered on `v1` and on prefix `ext`
- WHEN requests reach both
- THEN each request SHALL carry its own `Surface`, so a policy can apply different rules

### Requirement: Framework-Enforced gRPC Authorization

TRex SHALL add a unary and a stream authorization interceptor right after the
authentication interceptors and before any post-auth interceptors. They SHALL
use the same `Env.Authorizer` as REST.

Each gRPC service SHALL declare a method table when it registers:

```go
type GRPCResource struct {
    Kind   string
    Action string
}
type GRPCResources map[string]GRPCResource // full method name → resource
```

`authz.CRUDResources(service, kind, singular, plural)` SHALL build the table
for a service that follows the TRex conventions (`Get`, `Create`, `Update`,
`Delete<Singular>`, `List` and `Watch<Plural>`). Declaring the same method in
two services SHALL fail server startup.

The interceptor SHALL set `GRPCMethod`, `GRPCMsg`, `Resource.Kind` and
`Action` from the table. `Resource.ID` and `Resource.Parent` MAY be left empty
on gRPC; the policy reads them from `GRPCMsg`.

#### Scenario: Unary call
- GIVEN `ListDinosaurs` is declared as `{Kind: "dinosaurs", Action: "list"}`
- WHEN a client calls it
- THEN `Authorize` SHALL receive `Kind="dinosaurs"`, `Action="list"`, and the request message in `GRPCMsg`

#### Scenario: Undeclared method fails closed
- GIVEN a method that is in no table, not in `authConfig.BypassMethods`, and not a health or reflection method
- WHEN a client calls it
- THEN the call SHALL fail with `codes.PermissionDenied`

#### Scenario: Bypass methods
- GIVEN a method listed in `authConfig.BypassMethods` (for example health and reflection)
- WHEN a client calls it
- THEN `Authorize` MUST NOT be called

#### Scenario: Stream checked per message
- GIVEN a client stream on a declared method
- WHEN the stream opens
- THEN `Authorize` SHALL be called once with `GRPCMsg` empty
- AND WHEN each message is received
- THEN `Authorize` SHALL be called again with that message in `GRPCMsg`
- AND a denied message SHALL end the stream with the mapped status code

### Requirement: Handler-Level Checks Stay in the Application

Checks that need the request body or a loaded record, capability flags in
responses, and visibility filtering of lists are application logic. TRex
SHALL NOT pass bodies or records to the `Authorizer`. Handlers MAY read the
verified subject with `authz.SubjectFromContext(ctx)`.

#### Scenario: Body-based check
- GIVEN a create request whose permission depends on a field in the body
- WHEN the handler runs
- THEN the handler SHALL make that check with application code

### Requirement: Removal of the Old Interface

TRex SHALL remove `auth.AuthorizationMiddleware`, its mock, the
`authzMiddleware` and `authMiddleware` parameters of `RouteRegistrationFunc`
and `LoadDiscovered*Routes`, and every `router.Use(authzMiddleware.AuthorizeApi)`
line in plugins, the entity generator templates, and `templates/new-project/`.

#### Scenario: Generated entity
- GIVEN the entity generator creates a new kind
- WHEN its plugin is generated
- THEN it SHALL register routes with `r.Resource(<kind plural snake case>)`
- AND it SHALL NOT reference any authorization middleware
- AND its gRPC service SHALL declare a `GRPCResources` table

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| TRex owns authorization; the app owns the policy | Hook points, ordering, and route metadata are framework knowledge. Apps that wire authorization themselves repeat it and get the order wrong. |
| One interface for REST and gRPC | One policy, one place to read it, no drift between transports. |
| Required `OverrideAuthorizer` hook | Explicit, matches the other `Override*` hooks, runs after services exist. The compiler forces every environment to choose. Rejected: a field set from `init()` (implicit order, global access); an optional interface found by type assertion (exists only to avoid breaking builds, which is not a goal). |
| Framework attaches the middleware and interceptors | Secure by default. A plugin or a new prefix cannot be left open by forgetting one line. |
| Plugins declare kinds; gRPC services declare a method table | TRex knows kind, ID, parents, and surface at registration time. The policy never parses URLs or method names. |
| Fail closed on undeclared routes and methods | A missing declaration is a bug, and a deny makes it visible. |
| `Resource.Surface` | Prefixed routers such as `/ext` may need different rules for the same kind. |
| `HideExistence` | Some policies must not reveal whether a record exists. |
| Errors never allow | A failed lookup must not become a grant. 503 / Unavailable lets clients retry. |
| Per-message stream checks | Long-lived streams can carry messages about records the caller may not touch. |
| `SubjectEnricher` is the only optional extra | Because TRex now controls the chain order, an app can no longer insert its own "load the user" step between authentication and authorization. |
| Left out: bodies or records in `Request`, list scoping, audit or activity recording | Handlers already hold that data and can call app code. These can be added later without breaking the core. |
| New package `pkg/authz` | Keeps `pkg/auth` about authentication. |
| `Resource(kind)` returns a subrouter that runs authorization itself | Parent-router middleware runs before the plugin's subrouter is matched, so it cannot see the kind. After registration, TRex walks the routes and replaces any route outside `Resource()` or `Public()` with a deny handler. |
| `Parent` keyed by path variable name | No pluralization rules; the policy sees exactly what the route declared. |
| `dev-user` when JWT is off, on both transports | Plugins used to attach this mock themselves on REST; TRex now does it once, and gRPC matches REST. |
