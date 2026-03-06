# TRex Code Generation Service

## Overview

TRex is a deployable REST+gRPC service with a plugin-based generator that produces 92+ files per Kind across entity, SDK, CLI, and Console Plugin families. This proposal adds a **stateless code generation endpoint** to TRex that renders templates in-memory and returns the results as a list of readable links.

The design is intentionally minimal: no database records, no async jobs, no git credentials, no PR machinery. TRex generates code and returns it. The caller walks the links, reads the files, and applies them to its own repository.

```
Caller                        TRex
  │                             │
  │  POST /generate             │
  │  { kind, fields }           │
  │ ─────────────────────────▶  │
  │                             │  render all templates
  │                             │  in memory
  │  { files: [{path, href}] }  │
  │ ◀─────────────────────────  │
  │                             │
  │  GET /generate/{id}/model.go│
  │ ─────────────────────────▶  │
  │  "package fossils..."       │
  │ ◀─────────────────────────  │
  │                             │
  │  GET /generate/{id}/dao.go  │
  │ ─────────────────────────▶  │
  │  "package fossils..."       │
  │ ◀─────────────────────────  │
```

The MCP adapter is a thin JSON-RPC layer over these same REST endpoints — or MCP clients can call the REST endpoints directly.

---

## API

### `POST /api/rh-trex/v1/generate`

Render all templates for a Kind and return links to the generated files.

**Request:**

```json
{
  "kind": "Fossil",
  "fields": "discovery_location:string:required,estimated_age:int,fossil_type:string,excavator_name:string",
  "plural": "",
  "generators": ["entity", "sdk-go", "sdk-ts", "cli"]
}
```


| Field        | Type     | Required | Description                                                   |
| ------------ | -------- | -------- | ------------------------------------------------------------- |
| `kind`       | string   | yes      | PascalCase Kind name (e.g.`Fossil`)                           |
| `fields`     | string   | no       | Comma-separated`name:type[:required|optional]` pairs          |
| `plural`     | string   | no       | Override plural form. Defaults to automatic pluralization.    |
| `generators` | []string | no       | Which generator families to include. Defaults to`["entity"]`. |

**Response:**

```json
{
  "id": "b3f2a1",
  "kind": "Fossil",
  "expires_at": "2026-03-03T12:05:00Z",
  "files": [
    { "path": "plugins/fossils/model.go",                "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/model.go",                "generator": "entity" },
    { "path": "plugins/fossils/dao.go",                  "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/dao.go",                  "generator": "entity" },
    { "path": "plugins/fossils/service.go",              "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/service.go",              "generator": "entity" },
    { "path": "plugins/fossils/handler.go",              "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/handler.go",              "generator": "entity" },
    { "path": "plugins/fossils/presenter.go",            "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/presenter.go",            "generator": "entity" },
    { "path": "plugins/fossils/migration.go",            "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/migration.go",            "generator": "entity" },
    { "path": "plugins/fossils/mock_dao.go",             "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/mock_dao.go",             "generator": "entity" },
    { "path": "plugins/fossils/plugin.go",               "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/plugin.go",              "generator": "entity" },
    { "path": "plugins/fossils/integration_test.go",     "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/integration_test.go",    "generator": "entity" },
    { "path": "plugins/fossils/factory_test.go",         "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/factory_test.go",        "generator": "entity" },
    { "path": "plugins/fossils/testmain_test.go",        "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/testmain_test.go",       "generator": "entity" },
    { "path": "plugins/fossils/grpc_handler.go",         "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/grpc_handler.go",        "generator": "entity" },
    { "path": "plugins/fossils/grpc_presenter.go",       "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/grpc_presenter.go",      "generator": "entity" },
    { "path": "plugins/fossils/grpc_integration_test.go","href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/grpc_integration_test.go","generator": "entity" },
    { "path": "openapi/openapi.fossils.yaml",            "href": "/api/rh-trex/v1/generate/b3f2a1/openapi/openapi.fossils.yaml",           "generator": "entity" },
    { "path": "proto/rh_trex/v1/fossils.proto",          "href": "/api/rh-trex/v1/generate/b3f2a1/proto/rh_trex/v1/fossils.proto",         "generator": "entity" },
    { "path": "types/fossil.go",                         "href": "/api/rh-trex/v1/generate/b3f2a1/types/fossil.go",                        "generator": "sdk-go" },
    { "path": "client/fossil_api.go",                    "href": "/api/rh-trex/v1/generate/b3f2a1/client/fossil_api.go",                   "generator": "sdk-go" }
  ]
}
```

The `id` is a short random token. Results are held in a short-lived in-memory cache (default 5 minutes, configurable). There is no database record.

---

### `GET /api/rh-trex/v1/generate/{id}/{path...}`

Read the content of a single generated file.

**Response:** `Content-Type: text/plain` — the raw rendered file content.

```
package fossils

import (
    "context"
    ...
)

type Fossil struct {
    api.Meta
    DiscoveryLocation string  `json:"discovery_location"`
    EstimatedAge      *int    `json:"estimated_age,omitempty"`
    ...
}
```

Returns `404` if the `id` has expired or the `path` was not part of the generation set.

---

### `GET /api/rh-trex/v1/generate/{id}`

Re-read the file list for an existing generation result without re-rendering.

**Response:** Same shape as the `POST` response. Returns `404` if expired.

---

## Implementation

### What Changes

Three things are added to TRex. Everything else is existing infrastructure.

```
plugins/generate/
  plugin.go      # registers routes, wires handler
  handler.go     # POST /generate and GET /generate/{id}/{path...}
  renderer.go    # extracted generator logic: (kind, fields) → map[path]content
  cache.go       # short-lived in-memory store: id → rendered results
```

### `renderer.go` — The Core

The generator logic in `scripts/generator.go` is currently a standalone CLI program. `renderer.go` extracts the template rendering step into a callable function:

```go
type RenderRequest struct {
    Kind       string
    Fields     string
    Plural     string
    Generators []string
}

type RenderedFile struct {
    Path      string
    Content   string
    Generator string
}

func Render(req RenderRequest) ([]RenderedFile, error)
```

`Render` does exactly what `generator.go main()` does today — parses fields, loads templates from `templates/`, executes them with `text/template` — but writes to `bytes.Buffer` instead of `os.File`. No disk I/O, no `exec.Command`, no `gofmt` subprocess for the in-memory path (formatting is a nice-to-have, not required for correctness).

The templates are embedded in the binary at build time via `go:embed templates/` so the deployed container needs no filesystem access to the template directory.

### `cache.go` — Short-Lived Results

```go
const (
    defaultTTL      = 30 * time.Minute
    defaultMaxItems = 1000
)

type Cache struct {
    mu       sync.RWMutex
    entries  map[string]*CacheEntry
    lruOrder []string          // insertion-order list for LRU eviction
    ttl      time.Duration
    maxItems int
}

type CacheEntry struct {
    Files     []RenderedFile
    Kind      string
    CreatedAt time.Time
}

func (c *Cache) Set(id, kind string, files []RenderedFile) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if len(c.entries) >= c.maxItems {
        // evict oldest
        oldest := c.lruOrder[0]
        c.lruOrder = c.lruOrder[1:]
        delete(c.entries, oldest)
    }
    c.entries[id] = &CacheEntry{Files: files, Kind: kind, CreatedAt: time.Now()}
    c.lruOrder = append(c.lruOrder, id)
}
```

A background goroutine sweeps expired entries every minute. The cache is bounded at `maxItems` (default 1000) entries with LRU eviction as a safety valve — at ~80KB per result this caps memory consumption at ~80MB. The cache is intentionally not shared across replicas — each TRex pod holds its own results for the TTL window. Since the render is stateless and fast, a caller that hits a different pod on the `GET` just re-`POST`s.

### `handler.go`

```go
var (
    kindPattern  = regexp.MustCompile(`^[A-Z][A-Za-z0-9]+$`)
    fieldName    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
    validTypes   = map[string]bool{"string": true, "int": true, "int64": true, "bool": true, "float": true, "time": true}
    validMods    = map[string]bool{"required": true, "optional": true}
    hrefPrefix   = "/api/rh-trex/v1/generate/"
)

func validateKind(kind string) error {
    if !kindPattern.MatchString(kind) {
        return fmt.Errorf("kind must match ^[A-Z][A-Za-z0-9]+$")
    }
    return nil
}

func validateFields(fields string) error {
    if fields == "" {
        return nil
    }
    for _, segment := range strings.Split(fields, ",") {
        parts := strings.Split(strings.TrimSpace(segment), ":")
        if len(parts) < 2 || len(parts) > 3 {
            return fmt.Errorf("invalid field %q: expected name:type or name:type:modifier", segment)
        }
        if !fieldName.MatchString(parts[0]) {
            return fmt.Errorf("field name %q must match ^[a-z][a-z0-9_]*$", parts[0])
        }
        if !validTypes[parts[1]] {
            return fmt.Errorf("field type %q not in allowlist (string, int, int64, bool, float, time)", parts[1])
        }
        if len(parts) == 3 && !validMods[parts[2]] {
            return fmt.Errorf("field modifier %q must be 'required' or 'optional'", parts[2])
        }
    }
    return nil
}

// newID returns a cryptographically random 16-byte hex string.
func newID() string {
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        panic(fmt.Sprintf("crypto/rand failed: %v", err))
    }
    return hex.EncodeToString(b)
}

type GenerateHandler struct {
    cache    *cache.Cache
    renderer *renderer.Renderer
}

// POST /api/rh-trex/v1/generate
func (h *GenerateHandler) Generate(w http.ResponseWriter, r *http.Request) {
    var req GenerateRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        handlers.SendError(w, errors.BadRequest("invalid request body: %v", err))
        return
    }
    if err := validateKind(req.Kind); err != nil {
        handlers.SendError(w, errors.BadRequest("%v", err))
        return
    }
    if err := validateFields(req.Fields); err != nil {
        handlers.SendError(w, errors.BadRequest("%v", err))
        return
    }

    files, err := h.renderer.Render(renderer.RenderRequest{
        Kind:       req.Kind,
        Fields:     req.Fields,
        Plural:     req.Plural,
        Generators: req.Generators,
    })
    if err != nil {
        handlers.SendError(w, errors.GeneralError("render failed: %v", err))
        return
    }

    id := newID()
    h.cache.Set(id, req.Kind, files)
    handlers.SendResponse(w, http.StatusCreated, toResponse(id, req.Kind, files))
}

// GET /api/rh-trex/v1/generate/{id}/{path...}
func (h *GenerateHandler) GetFile(w http.ResponseWriter, r *http.Request) {
    id   := mux.Vars(r)["id"]
    path := mux.Vars(r)["path"]

    entry, ok := h.cache.Get(id)
    if !ok {
        handlers.SendError(w, errors.NotFound("generation result %s not found or expired", id))
        return
    }

    for _, f := range entry.Files {
        if f.Path == path {
            w.Header().Set("Content-Type", "text/plain; charset=utf-8")
            w.WriteHeader(http.StatusOK)
            fmt.Fprint(w, f.Content)
            return
        }
    }
    handlers.SendError(w, errors.NotFound("file %s not found in generation result %s", path, id))
}
```

### `plugin.go`

Follows the existing `init()` auto-registration pattern — no manual wiring.

Route registration order matters: the more-specific `/{id}/{path:.*}` route is registered first so gorilla/mux matches it before the bare `/{id}` route:

```go
func init() {
    pkgserver.RegisterRoutes("generate", func(apiV1Router *mux.Router, srv *server.Server) {
        h := NewGenerateHandler(srv)
        generateRouter := apiV1Router.PathPrefix("/generate").Subrouter()

        // More-specific routes registered first to prevent shadowing.
        generateRouter.HandleFunc("/{id}/{path:.*}", h.GetFile).Methods(http.MethodGet)
        generateRouter.HandleFunc("/{id}",           h.GetResult).Methods(http.MethodGet)
        generateRouter.HandleFunc("",                h.Generate).Methods(http.MethodPost)

        if srv.Config().GenerateRequireAuth {
            generateRouter.Use(authMiddleware.AuthenticateAccountJWT)
        }
    })
}
```

Auth is controlled by `--generate-require-auth` (default `true`). Explicitly set `--generate-require-auth=false` to disable for embedded/internal deployments. This prevents accidental unauthenticated external exposure.

---

## MCP Adapter

The MCP layer is a thin JSON-RPC wrapper over the REST endpoints. It adds no logic of its own.

### Tool: `trex_generate`

```json
{
  "name": "trex_generate",
  "description": "Generate CRUD code for a new Kind. Returns a list of file links to read.",
  "inputSchema": {
    "type": "object",
    "required": ["kind"],
    "properties": {
      "kind":       { "type": "string", "description": "PascalCase Kind name, e.g. Fossil" },
      "fields":     { "type": "string", "description": "Comma-separated name:type[:required] pairs" },
      "plural":     { "type": "string", "description": "Override plural. Defaults to automatic." },
      "generators": { "type": "array",  "items": { "type": "string" }, "default": ["entity"] }
    }
  }
}
```

**Returns:**

```json
{
  "id": "b3f2a1",
  "kind": "Fossil",
  "file_count": 18,
  "files": [
    { "path": "plugins/fossils/model.go", "href": "/api/rh-trex/v1/generate/b3f2a1/plugins/fossils/model.go" },
    ...
  ]
}
```

### Tool: `trex_read_file`

```json
{
  "name": "trex_read_file",
  "description": "Read the content of one generated file by its href. href must begin with /api/rh-trex/v1/generate/.",
  "inputSchema": {
    "required": ["href"],
    "properties": {
      "href": {
        "type": "string",
        "pattern": "^/api/rh-trex/v1/generate/[0-9a-f]{32}/"
      }
    }
  }
}
```

The MCP adapter validates that `href` begins with `/api/rh-trex/v1/generate/` before forwarding the request. Hrefs not matching this prefix are rejected with an error before any HTTP call is made.

**Returns:** `{ "path": "plugins/fossils/model.go", "content": "package fossils\n..." }`

### Tool: `trex_list_templates`

```json
{
  "name": "trex_list_templates",
  "description": "List available generator templates and the files each one produces.",
  "inputSchema": { "type": "object", "properties": {} }
}
```

**Returns:** Static metadata about available generators and their output file patterns — useful for an agent deciding which `generators` to request.

### MCP Resource: `trex://generate/{id}`

Each generation result is also exposed as an MCP resource. An agent can subscribe to the resource list and walk it without making separate `trex_read_file` calls:

```
trex://generate/b3f2a1/plugins/fossils/model.go
trex://generate/b3f2a1/plugins/fossils/dao.go
...
```

---

## Agent Usage Pattern

```
Agent (e.g., Ambient API agent)
│
│  Detects new entity needed from ERD change
│
├── Call trex_generate(kind="Fossil", fields="discovery_location:string:required,...")
│   └── Returns: { id: "b3f2a1", files: [ {path, href}, ... ] }
│
├── For each file in files:
│   └── Call trex_read_file(href) → get content
│       OR read MCP resource trex://generate/b3f2a1/{path}
│
├── Apply files to local repo (write each content to its path)
│
├── Run: make binary && make test
│
├── Commit + push → open PR
│
└── Reviewer agent reviews PR
```

The agent is in full control of what happens with the generated code. TRex is stateless — it renders and serves, nothing more. The agent decides which files to apply, whether to modify any before committing, and when to open the PR.

---

## What This Is Not

This proposal deliberately excludes:

- **No git operations** — TRex does not clone, push, or open PRs. The caller does.
- **No build or test execution** — TRex does not run `make binary` or `make test`. The caller does.
- **No async jobs** — every `POST /generate` is synchronous. Rendering 16 templates takes milliseconds.
- **No database records** — no `GenerationRequest` table, no event-driven controller, no PostgreSQL dependency for the generate endpoint.
- **No authentication** — for the embedded/internal deployment case. Add `authMiddleware.AuthenticateAccountJWT` to the router when exposing externally.

These are all valid future additions. They are not needed for the proof of concept.

---

## Implementation Plan


| Step      | What                                                                                                                        | Effort      |
| --------- | --------------------------------------------------------------------------------------------------------------------------- | ----------- |
| 1         | Extract renderer: move template logic from`scripts/generator.go` into `plugins/generate/renderer.go` as a callable function | 1 day       |
| 2         | Write`cache.go`: in-memory TTL store with background sweep                                                                  | 0.5 day     |
| 3         | Write`handler.go`: POST and GET handlers following existing TRex handler patterns                                           | 0.5 day     |
| 4         | Write`plugin.go`: register routes via `init()`, add to `cmd/trex/main.go` import                                            | 0.5 day     |
| 5         | Embed templates: add`//go:embed templates/` so the binary is self-contained                                                 | 0.5 day     |
| 6         | Write MCP adapter: JSON-RPC router with`trex_generate`, `trex_read_file`, `trex_list_templates`                             | 1 day       |
| **Total** |                                                                                                                             | **~4 days** |

Steps 1–5 are entirely within existing TRex patterns. The only new surface is step 6.

---

## Future Extensions

Once the POC proves the pattern, the natural next steps are:


| Extension                         | What it adds                                                                                                  |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `POST /generate/erd`              | Accept a raw Mermaid ERD block, parse it, generate all Kinds in one call, return a unified file list          |
| `GET /generate/diff?repo_url=...` | Compare ERD to a scanned repo, return which Kinds are missing                                                 |
| Auth middleware                   | Add`AuthenticateAccountJWT` to the generate router for external deployments                                   |
| Persistent results                | Store generation results in a`GenerationResult` Kind (using TRex's own entity generator) for audit and replay |
| Build webhook                     | Accept a callback URL; after the caller commits, TRex verifies the PR compiles and tests pass                 |
