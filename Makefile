# linklens developer tasks. Run `make help` for a summary.

GO_DIRS      := ./cmd/... ./internal/... ./web
GOFMT_PATHS  := cmd internal web/embed.go
BINARY       := bin/linklens
DATABASE_URL ?= postgres://linklens:linklens@localhost:5432/linklens?sslmode=disable

.PHONY: help test test-go test-web test-integration bench lint lint-go lint-web \
        run build web-install web-build docker-build compose-up clean

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

test: test-go test-web ## Run all unit tests (no external services needed)

test-go: ## Go unit tests with the race detector
	go test -race $(GO_DIRS)

test-web: web-install ## Frontend tests (vitest)
	cd web && npm test

test-integration: ## PostgreSQL integration tests (needs INTEGRATION=1 and a database)
	INTEGRATION=1 DATABASE_URL="$(DATABASE_URL)" go test -race -tags integration -count=1 ./internal/store/postgres/...

bench: ## Redirect hot-path benchmark
	go test -run '^$$' -bench Redirect -benchmem ./internal/api/

lint: lint-go lint-web ## Run all linters

lint-go: ## gofmt + go vet
	@out="$$(gofmt -l $(GOFMT_PATHS))"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet $(GO_DIRS)
	go vet -tags integration ./internal/store/postgres/...

lint-web: web-install ## eslint + tsc
	cd web && npm run lint

run: web-build ## Build the dashboard and run the server with in-memory backends
	go run ./cmd/server

build: web-build ## Build a self-contained binary into bin/
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/server

web-install:
	@[ -d web/node_modules ] || (cd web && npm ci)

web-build: web-install ## Build the React dashboard into web/dist (embedded by Go)
	cd web && npm run build

docker-build: ## Build the container image
	docker build -t linklens:local .

compose-up: ## Start app + PostgreSQL + Redis
	docker compose up --build

clean:
	rm -rf bin web/node_modules
	find web/dist -mindepth 1 ! -name .gitkeep -exec rm -rf {} +
