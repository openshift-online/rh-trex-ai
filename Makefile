CONTAINER_ENGINE?=$(shell command -v podman 2>/dev/null || echo docker)
GO ?= go
GOPATH ?= $(shell $(GO) env GOPATH)

GOTESTSUM_VERSION := v1.13.0
GOTESTSUM ?= $(GO) run gotest.tools/gotestsum@$(GOTESTSUM_VERSION)

NODE_TEST_IMAGE := registry.access.redhat.com/ubi9/nodejs-20:1-1778648167@sha256:74cc7b1d13592b1e425074f434b90e470ab209da85fd1fdb8e6e9e4cabaec51a
TYPESCRIPT_VERSION := 5.3.3

OPENAPI_SPEC ?= $(PWD)/components/api-server/openapi/openapi.yaml
SDK_GO_OUT ?= $(PWD)/generated/sdk/go
SDK_PYTHON_OUT ?= $(PWD)/generated/sdk/python
SDK_TS_OUT ?= $(PWD)/generated/sdk/typescript
SDK_MODULE ?= github.com/openshift-online/rh-trex-ai-sdk
SDK_API_PREFIX ?= /api/rh-trex-ai/v1
CLI_OUT ?= $(PWD)/generated/cli
CLI_BINARY ?= trex-cli
CLI_MODULE ?= github.com/openshift-online/rh-trex-ai-cli
CONSOLE_PLUGIN_OUT ?= $(PWD)/generated/console-plugin
CONSOLE_PLUGIN_NAME ?= rh-trex-ai-console

container_tool ?= podman

.PHONY: build-all
build-all:
	cd components/api-server && $(MAKE) binary

.PHONY: lint
lint:
	cd components/api-server && $(MAKE) lint

.PHONY: verify
verify:
	cd components/api-server && $(MAKE) verify

.PHONY: test
test:
	cd components/api-server && $(MAKE) test
	$(MAKE) test-generators

.PHONY: test-integration
test-integration:
	cd components/api-server && $(MAKE) test-integration

.PHONY: test-dependency-policy
test-dependency-policy:
	PYTHONDONTWRITEBYTECODE=1 python3 -m unittest scripts/test_check_dependency_age.py

.PHONY: dependency-age
dependency-age: test-dependency-policy
	PYTHONDONTWRITEBYTECODE=1 python3 scripts/check-dependency-age.py --min-age-days 14

.PHONY: test-generators
test-generators:
	@for module in \
		scripts/openapi-ir \
		scripts/sdk-generator \
		scripts/cli-generator \
		scripts/console-plugin-generator; do \
			echo "Testing $$module"; \
			(cd "$$module" && \
				GOWORK=off \
				TREX_CONTAINER_TOOL="$(container_tool)" \
				TREX_NODE_IMAGE="$(NODE_TEST_IMAGE)" \
				TREX_TYPESCRIPT_VERSION="$(TYPESCRIPT_VERSION)" \
				$(GOTESTSUM) --format short-verbose -- ./...) || exit 1; \
	done

.PHONY: generate
generate:
	cd components/api-server && $(MAKE) generate

.PHONY: proto
proto:
	cd components/api-server && $(MAKE) proto

.PHONY: db/setup
db/setup:
	cd components/api-server && $(MAKE) db/setup

.PHONY: db/teardown
db/teardown:
	cd components/api-server && $(MAKE) db/teardown

.PHONY: image
image:
	cd components/api-server && $(MAKE) image

.PHONY: generate-sdk
generate-sdk:
	cd scripts/sdk-generator && $(GO) run . \
		--spec $(OPENAPI_SPEC) \
		--go-out $(SDK_GO_OUT) \
		--python-out $(SDK_PYTHON_OUT) \
		--ts-out $(SDK_TS_OUT) \
		--module $(SDK_MODULE) \
		--api-prefix $(SDK_API_PREFIX) \
		--project rh-trex-ai

.PHONY: generate-sdk-go
generate-sdk-go:
	cd scripts/sdk-generator && $(GO) run . \
		--spec $(OPENAPI_SPEC) \
		--go-out $(SDK_GO_OUT) \
		--module $(SDK_MODULE) \
		--api-prefix $(SDK_API_PREFIX) \
		--project rh-trex-ai

.PHONY: generate-sdk-python
generate-sdk-python:
	cd scripts/sdk-generator && $(GO) run . \
		--spec $(OPENAPI_SPEC) \
		--python-out $(SDK_PYTHON_OUT) \
		--api-prefix $(SDK_API_PREFIX) \
		--project rh-trex-ai

.PHONY: generate-sdk-ts
generate-sdk-ts:
	cd scripts/sdk-generator && $(GO) run . \
		--spec $(OPENAPI_SPEC) \
		--ts-out $(SDK_TS_OUT) \
		--api-prefix $(SDK_API_PREFIX) \
		--project rh-trex-ai

.PHONY: generate-cli
generate-cli:
	cd scripts/cli-generator && $(GO) run . \
		--spec $(OPENAPI_SPEC) \
		--out $(CLI_OUT) \
		--binary $(CLI_BINARY) \
		--module $(CLI_MODULE) \
		--api-prefix $(SDK_API_PREFIX) \
		--project rh-trex-ai

.PHONY: generate-console-plugin
generate-console-plugin:
	cd scripts/console-plugin-generator && $(GO) run . \
		--spec $(OPENAPI_SPEC) \
		--out $(CONSOLE_PLUGIN_OUT) \
		--name $(CONSOLE_PLUGIN_NAME) \
		--api-prefix $(SDK_API_PREFIX) \
		--project rh-trex-ai

.PHONY: generate-all
generate-all: generate-sdk generate-cli generate-console-plugin

.PHONY: generate-clean
generate-clean:
	rm -rf $(PWD)/generated/
