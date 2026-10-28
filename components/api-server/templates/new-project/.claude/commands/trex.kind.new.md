# Generate a New Kind

Generate a complete CRUD entity (Kind) using the project's `generate-entity` command.

## Instructions

Use the TodoWrite tool to track progress.

### Step 1: Gather Requirements

Ask the user for:
1. **Kind name** (singular, PascalCase): e.g., "Widget", "Rocket", "Customer"
2. **Fields** beyond the base Meta fields (optional):
   - Name (use snake_case): e.g., "fuel_type", "max_speed"
   - Type: `string`, `int`, `int64`, `bool`, `float64`, `time`
   - Nullability: `:required` or `:optional` (default: nullable)
3. **Library path** (default): `github.com/openshift-online/rh-trex-ai/components/api-server`
4. **Module path** (your project's own module): e.g., `github.com/myorg/my-service`

### Step 2: Run the Generator

```bash
./my-service generate-entity --kind {KindName} \
  --project my-service \
  --repo github.com/myorg \
  --library github.com/openshift-online/rh-trex-ai/components/api-server \
  --module github.com/myorg/my-service \
  --proto-package my_service
```

With custom fields:
```bash
./my-service generate-entity --kind {KindName} \
  --fields "{field1}:{type1}:{nullability},{field2}:{type2}" \
  --project my-service \
  --repo github.com/myorg \
  --library github.com/openshift-online/rh-trex-ai/components/api-server \
  --module github.com/myorg/my-service \
  --proto-package my_service
```

The generator:
- Creates the plugin files (model, presenter, DAO, mock, service, handler, migration, tests, gRPC)
- Creates the per-kind OpenAPI spec and proto file
- Adds the plugin import to `cmd/my-service/main.go`
- Slices the new `$ref`s into `openapi/openapi.yaml`

### Step 3: Regenerate Proto + OpenAPI Client

```bash
make proto
make generate
```

### Step 4: Build and Verify

```bash
go build ./...
make binary
```

Fix any compilation errors if they occur.

### Step 5: Test

```bash
make test-integration
```

Or run just the new Kind's tests:
```bash
API_ENV=integration_testing go test ./plugins/{plural}/... -v
```

## Field Type Reference

| User Type | Go (required) | Go (nullable) | OpenAPI type | OpenAPI format | DB Type |
|-----------|--------------|---------------|-------------|---------------|---------|
| string | `string` | `*string` | string | | varchar |
| int | `int` | `*int` | integer | int32 | integer |
| int64 | `int64` | `*int64` | integer | int64 | bigint |
| bool | `bool` | `*bool` | boolean | | boolean |
| float64 | `float64` | `*float64` | number | double | float8 |
| time | `time.Time` | `*time.Time` | string | date-time | timestamptz |

## Naming Convention Reference

| Context | Convention | Example |
|---------|-----------|---------|
| Go struct | PascalCase | `ProductCategory` |
| Go variable | camelCase | `productCategory` |
| Go package | lowercase plural | `productcategories` |
| API path | snake_case plural | `/product_categories` |
| JSON field | snake_case | `category_name` |
| DB table | snake_case plural | `product_categories` |
| DB column | snake_case | `category_name` |
| Plugin registry | PascalCase plural | `ProductCategories` |

Rules:
- Plural: add "s" (irregular rules in the generator override common cases)
- snake_case: insert `_` before each uppercase letter, then lowercase everything
