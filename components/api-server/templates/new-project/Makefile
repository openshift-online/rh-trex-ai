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

.PHONY: ensure-deps
ensure-deps:
	@test -d pkg/api/openapi || $(MAKE) generate
	@test -f go.sum || go mod tidy

.PHONY: binary
binary: ensure-deps
	@echo "Building version: $(build_version)"
	go build -ldflags="$(ldflags)" -o $(BINARY_NAME) ./cmd/my-service

.PHONY: install
install: ensure-deps
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

# Verify that entities declared in specs/entities.yaml match what the TRex
# generator would produce from the currently-required library version. Run
# this after upgrading the TRex library to detect generated-code drift.
.PHONY: verify-entities
verify-entities: binary
	./$(BINARY_NAME) generate-entity --manifest specs/entities.yaml --verify \
		--project $(BINARY_NAME) \
		--repo $(REPO) \
		--library $(LIB_MODULE) \
		--module $(REPO)/$(BINARY_NAME) \
		--proto-package $(PROTO_PACKAGE)

# Regenerate every entity declared in specs/entities.yaml in place.
.PHONY: generate-entities
generate-entities: binary
	./$(BINARY_NAME) generate-entity --manifest specs/entities.yaml --regenerate \
		--project $(BINARY_NAME) \
		--repo $(REPO) \
		--library $(LIB_MODULE) \
		--module $(REPO)/$(BINARY_NAME) \
		--proto-package $(PROTO_PACKAGE)

# Combined verification: entity conformance + ctl conformance + vet + formatting.
.PHONY: verify
verify: verify-entities verify-ctl
	go vet ./...
	@test -z "$$(gofmt -l cmd pkg plugins)" || (echo "Unformatted files:"; gofmt -l cmd pkg plugins; exit 1)

# TRex source root: derived from the replace directive in go.mod (the parent
# directory of the components/api-server module path). Empty when the library
# is resolved via a proxy (published); then the generators cannot be run from
# a local checkout and the targets below fail with this error.
TREX_REPLACE := $(shell grep '^replace github.com/openshift-online/rh-trex-ai/components/api-server' go.mod | awk '{print $$4}')
TREX_SRC := $(if $(TREX_REPLACE),$(patsubst %/components/api-server,%,$(TREX_REPLACE)),)
CTL_OUT ?= generated/ctl
CTL_BINARY ?= $(BINARY_NAME)ctl

# Generate the <name>ctl component: the Cobra CLI plus the TUI command,
# sourced from the TRex library's cli-generator and tui-generator.
.PHONY: generate-cli
generate-cli:
	@test -n "$(TREX_SRC)" || (echo "error: TRex source not found (set TREX_SRC or the go.mod replace path)" >&2; exit 1)
	rm -rf $(CTL_OUT)
	cd $(TREX_SRC)/scripts/cli-generator && go run . \
		--spec $(abspath openapi/openapi.yaml) \
		--out $(abspath $(CTL_OUT)) \
		--binary $(CTL_BINARY) \
		--module $(REPO)/$(CTL_BINARY) \
		--api-prefix /api/$(BINARY_NAME)/v1 \
		--project $(BINARY_NAME) \
		--tui
	cd $(TREX_SRC)/scripts/tui-generator && go run . \
		--spec $(abspath openapi/openapi.yaml) \
		--out $(abspath $(CTL_OUT)/data/generated/tui)
	sed -i.bak 's#../path/to/rh-trex-ai/components/api-server#$(TREX_SRC)/components/api-server#' $(CTL_OUT)/go.mod
	rm -f $(CTL_OUT)/go.mod.bak

# Build the <name>ctl binary (CLI + TUI).
.PHONY: ctl
ctl: build-ctl

.PHONY: build-ctl
build-ctl: generate-cli
	cd $(CTL_OUT) && go mod tidy && go build -o $(CTL_BINARY) ./cmd/$(CTL_BINARY)

# Verify that the generated ctl component matches what the generators produce
# from the current OpenAPI spec. Regenerates into a temp dir and diffs sources
# (build artifacts: go.sum, the binary, and generated dirs are ignored).
.PHONY: verify-ctl
verify-ctl:
	@test -n "$(TREX_SRC)" || (echo "error: TRex source not found (set TREX_SRC or the go.mod replace path)" >&2; exit 1)
	@tmp=$$(mktemp -d); \
	$(MAKE) --no-print-directory generate-cli CTL_OUT="$$tmp" >/dev/null 2>&1 || { echo "error: could not regenerate ctl" >&2; rm -rf "$$tmp"; exit 1; }; \
	( cd "$$tmp" && go mod tidy ) >/dev/null 2>&1; \
	( cd $(abspath $(CTL_OUT)) && find . -type f ! -name go.sum ! -name '$(CTL_BINARY)' ! -path './.git/*' | sort ) > "$$tmp/files.txt"; \
	fail=0; \
	while IFS= read -r rel; do \
		[ -z "$$rel" ] && continue; \
		if ! cmp -s $(abspath $(CTL_OUT))/$$rel "$$tmp/$$rel"; then echo "  differs: $$rel"; fail=1; fi; \
	done < "$$tmp/files.txt"; \
	rm -rf "$$tmp"; \
	if [ "$$fail" = 0 ]; then echo "OK: ctl component matches regenerated output"; else echo "Generated ctl out of sync:"; exit 1; fi

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