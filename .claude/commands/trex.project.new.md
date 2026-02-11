# /trex.project.new

Creates a new project using TRex as an imported framework library.

**Note:** TRex is now a framework that projects import as a library. The `templates/new-project/` provides a convenient starter template.

## Manual Process

Ask the user for:
- `--name`: Name of the new service (required)
- `--destination`: Target directory for new project (required)  
- `--repo-base`: Git repository base URL (optional, defaults to "github.com/example")

## Steps

### Step 1: Copy Template

```bash
# Copy the template directory from TRex repository
cp -r /path/to/rh-trex/templates/new-project/ <destination-path>
cd <destination-path>
```

### Step 2: Replace Placeholders

Use find/replace to update template placeholders:

```bash
# Replace service name
find . -type f \( -name "*.go" -o -name "*.mod" -o -name "*.sum" -o -name "Makefile" \) \
  -exec sed -i 's/my-service/<service-name>/g' {} \;

# Replace repository paths
find . -type f \( -name "*.go" -o -name "*.mod" \) \
  -exec sed -i 's|github.com/example/my-service|<repo-base>/<service-name>|g' {} \;

# Replace repository base
find . -type f \( -name "*.go" -o -name "*.mod" \) \
  -exec sed -i 's|github.com/example|<repo-base>|g' {} \;
```

### Step 3: Rename Directories

```bash
# Rename my-service directory if it exists in template
find . -name "*my-service*" -type d -exec bash -c 'mv "$1" "${1//my-service/<service-name>}"' _ {} \;
find . -name "*my-service*" -type f -exec bash -c 'mv "$1" "${1//my-service/<service-name>}"' _ {} \;
```

## Template Structure

The template includes:
- `go.mod` and `go.sum` with TRex library dependencies
- `main.go` with basic TRex application structure
- `pkg/api/openapi_embed.go` for OpenAPI specification
- `Makefile` with common build and development tasks
- `secrets/` directory with database and service configuration files
- `.gitignore` configured for Go projects

## Examples

### Basic Usage
```bash
cp -r templates/new-project/ ./user-service
cd ./user-service
# Replace placeholders:
find . -type f \( -name "*.go" -o -name "*.mod" -o -name "Makefile" \) \
  -exec sed -i 's/my-service/user-service/g' {} \;
find . -type f \( -name "*.go" -o -name "*.mod" \) \
  -exec sed -i 's|github.com/example|github.com/example|g' {} \;
```

### Custom Repository Base
```bash
cp -r templates/new-project/ ./user-service  
cd ./user-service
# Replace placeholders:
find . -type f \( -name "*.go" -o -name "*.mod" -o -name "Makefile" \) \
  -exec sed -i 's/my-service/user-service/g' {} \;
find . -type f \( -name "*.go" -o -name "*.mod" \) \
  -exec sed -i 's|github.com/example|github.com/myorg|g' {} \;
```

### Step 4: Initialize Project

```bash
# Initialize Go module
go mod init <repo-base>/<service-name>
go mod tidy

# Build and test
make binary
make test
make test-integration

# Set up development environment  
make db/setup
./<service-name> migrate
make run-no-auth
```

## Template Structure

The `templates/new-project/` template includes:
- `go.mod` that imports TRex as a framework library dependency
- `cmd/my-service/main.go` that uses TRex's command framework (`pkg/cmd`)
- Basic project structure that leverages TRex components
- `Makefile` configured for TRex-based development
- `secrets/` directory for configuration files

## Framework Architecture

The template demonstrates how to use TRex as a framework:
- **Import TRex library**: Projects import `github.com/openshift-online/rh-trex-ai/pkg/*` packages
- **Use TRex commands**: Leverage `pkgcmd.NewRootCommand`, `pkgcmd.NewMigrateCommand`, etc.
- **Extend with entities**: Use the generator to add business entities to your project
- **Independent deployment**: Your service runs independently but uses TRex components

## Template Customization

To customize the template in the TRex repository:

1. Modify files in `templates/new-project/`
2. Use placeholder `my-service` for service name substitution
3. Use `github.com/example` for repository base substitution
4. Template demonstrates proper TRex framework usage patterns

## Notes

- **Framework approach**: Projects import TRex as a library rather than cloning code
- **Generator compatibility**: You can still use TRex's generator in your project: `go run /path/to/rh-trex/scripts/generator.go --kind YourKind --project <service-name> --repo <repo-base>`
- **Independent versioning**: Your project has its own versioning and can update TRex dependency as needed
- **Lighter footprint**: Only imports needed TRex components rather than entire codebase