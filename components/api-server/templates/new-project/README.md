# My Service

This project is generated from the TRex.AI library template and provides a foundation for building REST API microservices. It imports core types from the TRex.AI library and provides stubs for service-specific implementations.

## Quick Start

Before you start, point the `replace` directive in `go.mod` at a local checkout of the TRex library (the directory containing `components/api-server/go.mod`).

1. **Generate the OpenAPI client** (required first — the build and `go mod tidy` depend on it):
   ```bash
   make generate
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Build the service:**
   ```bash
   make binary
   ```

4. **Run tests:**
   ```bash
   make test
   ```

## Generating New Kinds

This project includes the TRex.AI Kind generator to create complete CRUD functionality for new resource types.

### Important Note

The template provides the basic structure and generator. To create your first Kind with complete functionality, you should:

1. First ensure your project compiles and tests pass: `make test`
2. Then generate your Kind and implement the needed TRex.AI interfaces
3. Or use the full TRex.AI library environment for complete plugin functionality

### Basic Example: Generate a HelloWorld Kind

```bash
# 1. Generate the OpenAPI client (required before tidy/build, and after every entity change)
make generate

# 2. Generate a HelloWorld Kind with a Message attribute.
#    --module MUST be your own module path; --library points at TRex.
./my-service generate-entity \
  --kind HelloWorld \
  --fields "message:string:required" \
  --project "my-service" \
  --repo "github.com/example" \
  --library github.com/openshift-online/rh-trex-ai/components/api-server \
  --module github.com/example/my-service \
  --proto-package my_service

# 3. Regenerate protobuf stubs, refresh the OpenAPI client, and build
make proto
make generate
go build ./...
```

### Field Types and Options

The generator supports these field types:
- `string` - Text data
- `int` - 32-bit integer  
- `int64` - 64-bit integer
- `bool` - Boolean true/false
- `float` or `float64` - Floating point numbers
- `time` - Timestamp (time.Time)

Field nullability options:
- `:required` - Non-nullable field (base types like `string`, `int`)
- `:optional` - Nullable field (pointer types like `*string`, `*int`) - default

### What the Generator Creates

For each Kind, the generator automatically creates:

- **API model** (`plugins/{kinds}/model.go`) - Go structs for the Kind
- **HTTP handlers** (`plugins/{kinds}/handler.go`) - REST API endpoints  
- **Service layer** (`plugins/{kinds}/service.go`) - Business logic with event handlers
- **Data access** (`plugins/{kinds}/dao.go`) - Database operations
- **Database migration** (`plugins/{kinds}/migration.go`) - Schema changes
- **OpenAPI spec** (`openapi/openapi.{kinds}.yaml`) - API documentation
- **Tests** (`plugins/{kinds}/*_test.go`) - Unit and integration tests
- **Plugin registration** (`plugins/{kinds}/plugin.go`) - Auto-wires everything together

## Database Operations

### Start PostgreSQL
```bash
make db/setup
```

### Run Migrations
```bash
./my-service migrate
```

### Stop Database
```bash
make db/teardown
```

## Running the Service

### With Authentication
```bash
make run
```

### Without Authentication (Development)
```bash
make run-no-auth
```

The service will be available at `http://localhost:8000`.

### Terminal UI

In another terminal, browse and operate on resources exposed by the service's OpenAPI specification:

```bash
./my-service tui --server http://localhost:8000
```

Use `--token-file` when the server requires authentication.

## API Endpoints

After generating Kinds, API endpoints follow this pattern:
- `GET /api/my-service/v1/{kinds}` - List all items
- `POST /api/my-service/v1/{kinds}` - Create new item
- `GET /api/my-service/v1/{kinds}/{id}` - Get specific item
- `PATCH /api/my-service/v1/{kinds}/{id}` - Update specific item

## Generate the Client Component (`<name>ctl`)

Generate and build the CLI + TUI component (one binary) from the project's OpenAPI spec:

```bash
make ctl
./generated/ctl/my-servicectl --help
```

The component includes all CRUD commands plus a terminal UI:

```bash
./my-servicectl login --token-file /dev/stdin --url http://localhost:8000
./my-servicectl list <resource>
./my-servicectl create <resource> --<field> <value>
./my-servicectl tui --server http://localhost:8000
```

After adding entities, regenerate the OpenAPI client first so the component includes them:

```bash
make proto && make generate
make ctl
```

## Conformance & Upgrading

The entities declared in `scripts/entities.yaml` are owned by the TRex entity generator. `make verify-entities` regenerates each declared entity from the *currently required* TRex library's templates and fails if your on-disk generated files have drifted:

```bash
make verify-entities
```

After upgrading the TRex library (for example bumping the module version or the `replace` target), run `make verify-entities` to discover which generated files need refreshing. Fix drift by regenerating the affected entities:

```bash
./my-service generate-entity --manifest scripts/entities.yaml --project my-service --repo github.com/example --module github.com/example/my-service --proto-package my_service
make proto && make generate
```

`make verify` composes entity conformance with `go vet` and `gofmt` checks.

## Development Workflow

1. **Generate new Kind**: `./my-service generate-entity --kind MyKind --fields "name:string:required" --library github.com/openshift-online/rh-trex-ai/components/api-server --module github.com/example/my-service --proto-package my_service`
2. **Regenerate proto + OpenAPI client**: `make proto && make generate`
3. **Run migrations**: `./my-service migrate`
4. **Test the API**: `make test && make test-integration`
5. **Start service**: `make run-no-auth`
6. **Test endpoints**: `curl http://localhost:8000/api/my-service/v1/my_kinds`

## Project Structure

```
├── cmd/my-service/          # Main application entry point
├── pkg/                     # Core packages
│   ├── api/                 # API types (re-exports from TRex.AI library)
│   ├── auth/                # Authentication stubs
│   ├── db/                  # Database utilities stubs
│   ├── errors/              # Error handling stubs
│   └── ...                  # Other core package stubs
├── plugins/                 # Generated Kinds (business logic)
│   └── {kinds}/             # Each Kind gets its own plugin
├── openapi/                 # OpenAPI specifications
├── scripts/                 # Code generator
├── templates/               # Generator templates
└── test/                    # Test utilities
```

## Generated Plugin Architecture

Each generated Kind is a self-contained plugin with:

- **Event-driven controllers** - Process CREATE/UPDATE/DELETE events automatically
- **Idempotent handlers** - Safe to run multiple times
- **Complete CRUD** - Create, Read, Update, Delete, Search operations
- **OpenAPI integration** - Automatic API documentation generation
- **Test coverage** - Unit and integration tests included

## Next Steps

1. Generate your first Kind to see the full functionality
2. Customize the generated code for your specific business logic
3. Add custom validation in the handlers
4. Extend the service layer with additional business rules
5. Add integration tests for your specific use cases

For more information, see the [TRex.AI documentation](../../CLAUDE.md).
