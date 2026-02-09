# Generate Entity Command

Generate a complete CRUD entity in the TRex application following the **plugin-based architecture**.

## Plugin System Overview

TRex uses a plugin system where each entity is fully self-contained in `plugins/{entity}s/`. The plugin auto-registers routes, controllers, presenters, and service locators via `init()` functions.

**All entity code lives in the plugin package** — NOT in `pkg/api/`, `pkg/dao/`, `pkg/services/`, `pkg/handlers/`, or `test/`. Only the migration lives in `pkg/db/migrations/` and the OpenAPI spec in `openapi/`.

## Recommended: Use the Generator

The fastest and most reliable approach is to use the automated generator:

```bash
# Basic entity with default "species" field
go run ./scripts/generator.go --kind Widget

# Entity with custom fields (snake_case field names)
go run ./scripts/generator.go --kind Widget \
  --fields "name:string:required,description:string,count:int,active:bool"

# After generation, rebuild OpenAPI client
make generate
```

**Supported field types:** `string`, `int`, `int64`, `bool`, `float`, `float64`, `time`
**Nullability:** Fields are nullable by default. Use `:required` to make non-nullable, `:optional` to be explicit.

The generator creates ALL files and updates ALL existing files automatically. See CLAUDE.md for full details.

## Manual Approach (if generator is not suitable)

If you need to create an entity manually, use the dinosaur plugin as the reference: `plugins/dinosaurs/`.

### Step 1: Gather Requirements

Ask the user for:
1. **Entity Name** (singular, PascalCase): e.g., "Widget"
2. **Entity Fields**: Beyond base Meta fields (ID, CreatedAt, UpdatedAt, DeletedAt)
3. **API Path** (snake_case plural): e.g., "widgets", "fizz_buzzs"

### Step 2: Create Plugin Files

All entity files go in `plugins/{entity}s/`. Use TodoWrite to track progress.

#### 2.1 Model (`plugins/{entity}s/model.go`)

The model is in the **plugin package**, not `pkg/api/`.

Reference: `plugins/dinosaurs/model.go`
```go
package widgets

import (
    "github.com/openshift-online/rh-trex/pkg/api"
    "gorm.io/gorm"
)

type Widget struct {
    api.Meta
    Name        string
    Description *string
}

type WidgetList []*Widget
type WidgetIndex map[string]*Widget

func (l WidgetList) Index() WidgetIndex {
    index := WidgetIndex{}
    for _, o := range l {
        index[o.ID] = o
    }
    return index
}

func (w *Widget) BeforeCreate(tx *gorm.DB) error {
    w.ID = api.NewID()
    return nil
}

type WidgetPatchRequest struct {
    Name        *string `json:"name,omitempty"`
    Description *string `json:"description,omitempty"`
}
```

#### 2.2 DAO (`plugins/{entity}s/dao.go`)

Reference: `plugins/dinosaurs/dao.go`

#### 2.3 Mock DAO (`plugins/{entity}s/mock_dao.go`)

Reference: `plugins/dinosaurs/mock_dao.go`

#### 2.4 Service (`plugins/{entity}s/service.go`)

Reference: `plugins/dinosaurs/service.go`

#### 2.5 Presenter (`plugins/{entity}s/presenter.go`)

Reference: `plugins/dinosaurs/presenter.go`

#### 2.6 Handler (`plugins/{entity}s/handler.go`)

Reference: `plugins/dinosaurs/handler.go`

#### 2.7 Plugin Registration (`plugins/{entity}s/plugin.go`)

This is the core integration file. It uses **local package types** (not `pkg/services`, `pkg/dao`, `pkg/handlers`).

Reference: `plugins/dinosaurs/plugin.go`
```go
package widgets

import (
    "net/http"

    "github.com/gorilla/mux"
    "github.com/openshift-online/rh-trex/cmd/trex/environments"
    "github.com/openshift-online/rh-trex/cmd/trex/environments/registry"
    "github.com/openshift-online/rh-trex/cmd/trex/server"
    "github.com/openshift-online/rh-trex/pkg/api"
    "github.com/openshift-online/rh-trex/pkg/api/presenters"
    "github.com/openshift-online/rh-trex/pkg/auth"
    "github.com/openshift-online/rh-trex/pkg/controllers"
    "github.com/openshift-online/rh-trex/pkg/db"
    "github.com/openshift-online/rh-trex/plugins/events"
    "github.com/openshift-online/rh-trex/plugins/generic"
)

type ServiceLocator func() WidgetService

func NewServiceLocator(env *environments.Env) ServiceLocator {
    return func() WidgetService {
        return NewWidgetService(
            db.NewAdvisoryLockFactory(env.Database.SessionFactory),
            NewWidgetDao(&env.Database.SessionFactory),
            events.Service(&env.Services),
        )
    }
}

func Service(s *environments.Services) WidgetService {
    if s == nil {
        return nil
    }
    if obj := s.GetService("Widgets"); obj != nil {
        locator := obj.(ServiceLocator)
        return locator()
    }
    return nil
}

func init() {
    registry.RegisterService("Widgets", func(env interface{}) interface{} {
        return NewServiceLocator(env.(*environments.Env))
    })

    server.RegisterRoutes("widgets", func(apiV1Router *mux.Router, services server.ServicesInterface, authMiddleware auth.JWTMiddleware, authzMiddleware auth.AuthorizationMiddleware) {
        envServices := services.(*environments.Services)
        widgetHandler := NewWidgetHandler(Service(envServices), generic.Service(envServices))

        widgetsRouter := apiV1Router.PathPrefix("/widgets").Subrouter()
        widgetsRouter.HandleFunc("", widgetHandler.List).Methods(http.MethodGet)
        widgetsRouter.HandleFunc("/{id}", widgetHandler.Get).Methods(http.MethodGet)
        widgetsRouter.HandleFunc("", widgetHandler.Create).Methods(http.MethodPost)
        widgetsRouter.HandleFunc("/{id}", widgetHandler.Patch).Methods(http.MethodPatch)
        widgetsRouter.HandleFunc("/{id}", widgetHandler.Delete).Methods(http.MethodDelete)
        widgetsRouter.Use(authMiddleware.AuthenticateAccountJWT)
        widgetsRouter.Use(authzMiddleware.AuthorizeApi)
    })

    server.RegisterController("Widgets", func(manager *controllers.KindControllerManager, services *environments.Services) {
        widgetServices := Service(services)

        manager.Add(&controllers.ControllerConfig{
            Source: "Widgets",
            Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
                api.CreateEventType: {widgetServices.OnUpsert},
                api.UpdateEventType: {widgetServices.OnUpsert},
                api.DeleteEventType: {widgetServices.OnDelete},
            },
        })
    })

    presenters.RegisterPath(Widget{}, "widgets")
    presenters.RegisterPath(&Widget{}, "widgets")
    presenters.RegisterKind(Widget{}, "Widget")
    presenters.RegisterKind(&Widget{}, "Widget")
}
```

**Key patterns in the plugin file:**
- `ServiceLocator` and `Service()` are **local** types/functions (not from `pkg/services`)
- Event service accessed via `events.Service(&env.Services)` (not `env.Services.Events()`)
- Generic service accessed via `generic.Service(envServices)` (not `envServices.Generic()`)
- Handler created with `NewWidgetHandler()` (local, not `handlers.NewWidgetHandler()`)
- Presenter registers `Widget{}` (local type, not `api.Widget{}`)

#### 2.8 Test Files

- `plugins/{entity}s/testmain_test.go` - Test setup (reference: `plugins/dinosaurs/testmain_test.go`)
- `plugins/{entity}s/factory_test.go` - Test factories (reference: `plugins/dinosaurs/factory_test.go`)
- `plugins/{entity}s/integration_test.go` - Integration tests (reference: `plugins/dinosaurs/integration_test.go`)

#### 2.9 Database Migration (`pkg/db/migrations/{YYYYMMDDHHMM}_add_{entity}s.go`)

Use inline struct definitions. Never import from the plugin package.

Reference: `pkg/db/migrations/201911212019_add_dinosaurs.go`

#### 2.10 OpenAPI Specification (`openapi/openapi.{entity}s.yaml`)

Reference: `openapi/openapi.dinosaurs.yaml`

### Step 3: Update Existing Files (only 3)

#### 3.1 `cmd/trex/main.go` - Add plugin import
```go
_ "github.com/openshift-online/rh-trex/plugins/widgets"
```

#### 3.2 `pkg/db/migrations/migration_structs.go` - Add migration
```go
var MigrationList = []*gormigrate.Migration{
    addDinosaurs(),
    addEvents(),
    addWidgets(),
}
```

#### 3.3 `openapi/openapi.yaml` - Add reference to entity spec

### Step 4: Generate OpenAPI Client

```bash
make generate
```

Wait for completion (2-3 minutes), then verify:
```bash
ls pkg/api/openapi/model_{entity}*.go
make binary
```

### Step 5: Test

```bash
make db/teardown && make db/setup
./trex migrate
make test-integration
```

### Checklist

- [ ] All plugin files created in `plugins/{entity}s/`
- [ ] Migration in `pkg/db/migrations/`
- [ ] OpenAPI spec in `openapi/`
- [ ] Plugin import in `cmd/trex/main.go`
- [ ] Migration added to `migration_structs.go`
- [ ] OpenAPI ref added to `openapi.yaml`
- [ ] `make generate` completed
- [ ] `make binary` compiles
- [ ] `make test-integration` passes
