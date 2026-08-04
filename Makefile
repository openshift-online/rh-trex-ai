CONTAINER_ENGINE?=$(shell command -v podman 2>/dev/null || echo docker)
GO ?= go

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

.PHONY: build-all
build-all:
	cd components/api-server && $(MAKE) binary
	cd components/control-plane && $(GO) build ./...

.PHONY: lint
lint:
	cd components/api-server && $(MAKE) lint
	cd components/control-plane && $(GO) fmt ./... && $(GO) vet ./...

.PHONY: verify
verify:
	cd components/api-server && $(MAKE) verify
	cd components/control-plane && $(GO) vet ./...

.PHONY: test
test:
	cd components/api-server && $(MAKE) test

.PHONY: test-integration
test-integration:
	cd components/api-server && $(MAKE) test-integration

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
