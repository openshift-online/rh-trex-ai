BINARY_NAME=my-service
CONTAINER_ENGINE?=$(shell command -v podman 2>/dev/null || echo docker)

# Module base for the project (e.g. github.com/myorg); used by verify-entities.
REPO?=github.com/example
# Protobuf package (snake_case of the project name). Set if not the default.
PROTO_PACKAGE?=my_service
# Path of the TRex library module; used by verify-entities.
LIB_MODULE=github.com/openshift-online/rh-trex-ai/components/api-server

# Version information for ldflags
git_sha:=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
git_dirty:=$(shell git diff --quiet 2>/dev/null || echo "-modified")
build_version:=$(git_sha)$(git_dirty)
build_time:=$(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
ldflags=-X github.com/example/my-service/pkg/api.Version=$(build_version) -X 'github.com/example/my-service/pkg/api.BuildTime=$(build_time)'

.PHONY: binary
binary:
	@echo "Building version: $(build_version)"
	go build -ldflags="$(ldflags)" -o $(BINARY_NAME) ./cmd/my-service

.PHONY: install
install:
	go install -ldflags="$(ldflags)" ./cmd/my-service

.PHONY: run
run: binary
	./$(BINARY_NAME) migrate
	./$(BINARY_NAME) serve

.PHONY: run-no-auth
run-no-auth: binary
	./$(BINARY_NAME) migrate
	./$(BINARY_NAME) serve --enable-authz=false --enable-jwt=false

.PHONY: test
test: install
	go test -v ./...

# Verify that entities declared in scripts/entities.yaml match what the TRex
# generator would produce from the currently-required library version. Run
# this after upgrading the TRex library to detect generated-code drift.
.PHONY: verify-entities
verify-entities: binary
	./$(BINARY_NAME) generate-entity --manifest scripts/entities.yaml --verify \
		--project $(BINARY_NAME) \
		--repo $(REPO) \
		--library $(LIB_MODULE) \
		--module $(REPO)/$(BINARY_NAME) \
		--proto-package $(PROTO_PACKAGE)

# Combined verification: entity conformance + vet + formatting.
.PHONY: verify
verify: verify-entities
	go vet ./...
	@test -z "$$(gofmt -l cmd pkg plugins)" || (echo "Unformatted files:"; gofmt -l cmd pkg plugins; exit 1)

# TRex source root: derived from the replace directive in go.mod (the parent
# directory of the components/api-server module path). Empty when the library
# is resolved via a proxy (published); then the CLI generator module is used.
TREX_REPLACE := $(shell grep '^replace github.com/openshift-online/rh-trex-ai/components/api-server' go.mod | awk '{print $$4}')
TREX_SRC := $(if $(TREX_REPLACE),$(patsubst %/components/api-server,%,$(TREX_REPLACE)),)
CLI_MODULE ?= github.com/openshift-online/rh-trex-ai/scripts/cli-generator
CLI_OUT ?= generated/cli
CLI_BINARY ?= $(BINARY_NAME)-cli

# Generate a Cobra CLI client from the project's OpenAPI spec.
.PHONY: generate-cli
generate-cli:
	rm -rf $(CLI_OUT)
ifneq ($(TREX_SRC),)
	cd $(TREX_SRC)/scripts/cli-generator && go run . \
		--spec $(abspath openapi/openapi.yaml) \
		--out $(abspath $(CLI_OUT)) \
		--binary $(CLI_BINARY) \
		--module $(REPO)/$(CLI_BINARY) \
		--api-prefix /api/$(BINARY_NAME)/v1 \
		--project $(BINARY_NAME)
else
	go run $(CLI_MODULE)@latest \
		--spec $(abspath openapi/openapi.yaml) \
		--out $(abspath $(CLI_OUT)) \
		--binary $(CLI_BINARY) \
		--module $(REPO)/$(CLI_BINARY) \
		--api-prefix /api/$(BINARY_NAME)/v1 \
		--project $(BINARY_NAME)
endif

.PHONY: test-integration
test-integration: install
	API_ENV=integration_testing go test -p 1 -v ./plugins/... -count=1

.PHONY: clean
clean:
	rm -f $(BINARY_NAME)

.PHONY: generate
generate:
	rm -rf pkg/api/openapi
	$(CONTAINER_ENGINE) build -t $(BINARY_NAME)-openapi -f Dockerfile.openapi .
	id=$$($(CONTAINER_ENGINE) create $(BINARY_NAME)-openapi) && \
	$(CONTAINER_ENGINE) cp $$id:/local/pkg/api/openapi ./pkg/api/openapi && \
	$(CONTAINER_ENGINE) rm $$id

.PHONY: proto
proto:
	cd proto && buf generate

.PHONY: proto-clean
proto-clean:
	rm -rf pkg/api/grpc

.PHONY: db/setup
db/setup:
	$(CONTAINER_ENGINE) run -d --name my-service-postgres \
		-p 5432:5432 \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_DB=my_service \
		postgres:13

.PHONY: db/teardown
db/teardown:
	$(CONTAINER_ENGINE) stop my-service-postgres || true
	$(CONTAINER_ENGINE) rm my-service-postgres || true

.PHONY: db/login
db/login:
	$(CONTAINER_ENGINE) exec -it my-service-postgres psql -U postgres -d my_service