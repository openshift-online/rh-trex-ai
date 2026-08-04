BINARY_NAME=my-service
CONTAINER_ENGINE?=$(shell command -v podman 2>/dev/null || echo docker)

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