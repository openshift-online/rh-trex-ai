CONTAINER_ENGINE?=$(shell command -v podman 2>/dev/null || echo docker)

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
