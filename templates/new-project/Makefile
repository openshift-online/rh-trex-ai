BINARY_NAME=my-service

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
	@echo "Integration tests not implemented in template"
	@echo "Add integration tests to test/integration/ after setting up your project"

.PHONY: clean
clean:
	rm -f $(BINARY_NAME)

.PHONY: generate
generate:
	@echo "OpenAPI client generation not implemented in template"
	@echo "Run this after setting up your project:"
	@echo "  go generate ./..."

.PHONY: db/setup
db/setup:
	docker run -d --name my-service-postgres \
		-p 5432:5432 \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_DB=my_service \
		postgres:13

.PHONY: db/teardown
db/teardown:
	docker stop my-service-postgres || true
	docker rm my-service-postgres || true

.PHONY: db/login
db/login:
	docker exec -it my-service-postgres psql -U postgres -d my_service