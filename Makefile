SHELL := /bin/bash
GO    ?= go
PKG   := ./...
BIN   := bin/api

.PHONY: help
help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## compile the api binary into ./bin/api
	$(GO) build -o $(BIN) ./cmd/api

.PHONY: run
run: ## build and run the api
	$(GO) run ./cmd/api

.PHONY: test
test: ## run the test suite
	$(GO) test -count=1 $(PKG)

.PHONY: test-race
test-race: ## run tests with the race detector
	$(GO) test -race -count=1 $(PKG)

.PHONY: test-postgres
test-postgres: ## run tests with the postgres integration enabled
	@test -n "$$DATABASE_URL" || (echo "DATABASE_URL is required"; exit 1)
	$(GO) test -count=1 $(PKG)

.PHONY: cover
cover: ## run tests and write coverage.out
	$(GO) test -count=1 -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: vet
vet: ## run go vet
	$(GO) vet $(PKG)

.PHONY: lint
lint: ## run golangci-lint
	@golangci-lint run --timeout=5m

.PHONY: tidy
tidy: ## run go mod tidy
	$(GO) mod tidy

.PHONY: verify
verify: vet lint test ## vet + lint + test

.PHONY: swagger
swagger: ## regenerate the swagger documentation
	$(GO) install github.com/swaggo/swag/cmd/swag@latest
	$(GO) generate ./cmd/api

.PHONY: dev-up
dev-up: ## bring up the docker compose dev stack
	./scripts/dev-up.sh

.PHONY: dev-down
dev-down: ## stop the docker compose dev stack
	./scripts/dev-down.sh

.PHONY: clean
clean: ## remove build artifacts
	rm -rf bin/ coverage.out tmp/
