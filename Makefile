# Build tooling for the Go version of swosh. Run `make` to list all targets.

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

GO ?= go
COMPOSE ?= docker compose
COMPOSE_FILE := docker-compose.dev.yml

BIN := $(CURDIR)/bin
MIGRATIONS := internal/db/migrations
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/Edholm/swosh/internal/cli.version=$(VERSION)

SWOSH_DATABASE_URL ?= postgres://swosh:swosh@localhost:5432/swosh?sslmode=disable

# Tools are built with the toolchain go.mod asks for, not whatever `go` is on
# PATH: golangci-lint refuses to lint code newer than the Go it was built with.
GO_TOOLCHAIN := $(shell $(GO) env GOVERSION)

SQLC_VERSION ?= v1.31.1
GOLANGCI_LINT_VERSION ?= v2.15.0
MIGRATE_VERSION ?= v4.20.1

SQLC := $(BIN)/sqlc
GOLANGCI_LINT := $(BIN)/golangci-lint
MIGRATE := $(BIN)/migrate

# Stamp files make `make tools` reinstall a tool when its pinned version changes.
SQLC_STAMP := $(BIN)/.sqlc-$(SQLC_VERSION)
GOLANGCI_LINT_STAMP := $(BIN)/.golangci-lint-$(GOLANGCI_LINT_VERSION)
MIGRATE_STAMP := $(BIN)/.migrate-$(MIGRATE_VERSION)

GO_INSTALL := GOTOOLCHAIN=$(GO_TOOLCHAIN) GOBIN=$(BIN) $(GO) install

##@ General

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n"} \
		/^[a-zA-Z0-9_-]+:.*##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

.PHONY: tools
tools: $(SQLC_STAMP) $(GOLANGCI_LINT_STAMP) $(MIGRATE_STAMP) ## Install pinned dev tools into ./bin

$(SQLC_STAMP):
	@rm -f $(BIN)/.sqlc-*
	$(GO_INSTALL) github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	@touch $@

$(GOLANGCI_LINT_STAMP):
	@rm -f $(BIN)/.golangci-lint-*
	$(GO_INSTALL) github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@touch $@

$(MIGRATE_STAMP):
	@rm -f $(BIN)/.migrate-*
	$(GO_INSTALL) -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)
	@touch $@

.PHONY: clean
clean: ## Remove build output and installed tools
	rm -rf $(BIN) coverage.out

##@ Development

.PHONY: generate
generate: $(SQLC_STAMP) ## Regenerate sqlc code from migrations and queries
	$(SQLC) generate

.PHONY: generate-check
generate-check: $(SQLC_STAMP) ## Fail if the generated sqlc code is out of date
	$(SQLC) diff

.PHONY: fmt
fmt: $(GOLANGCI_LINT_STAMP) ## Format the code
	$(GOLANGCI_LINT) fmt ./...

.PHONY: lint
lint: $(GOLANGCI_LINT_STAMP) ## Run golangci-lint
	$(GOLANGCI_LINT) run ./...

.PHONY: tidy
tidy: ## Tidy go.mod and go.sum
	$(GO) mod tidy

.PHONY: test
test: ## Run tests (database tests skip unless SWOSH_TEST_DATABASE_URL is set)
	$(GO) test -race -coverprofile=coverage.out ./...

.PHONY: test-integration
test-integration: db-up ## Run all tests, including database tests, against the dev Postgres
	SWOSH_TEST_DATABASE_URL='$(SWOSH_DATABASE_URL)' $(MAKE) test

.PHONY: check
check: generate-check lint test ## Run everything CI runs, except the build

.PHONY: build
build: ## Build all binaries into ./bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN)/ ./cmd/...

.PHONY: run
run: db-up ## Run the server against the dev Postgres
	$(GO) run -ldflags '$(LDFLAGS)' ./cmd/swosh serve

##@ Database

.PHONY: db-up
db-up: ## Start the dev Postgres and wait until it accepts connections
	$(COMPOSE) -f $(COMPOSE_FILE) up -d postgres
	@for _ in $$(seq 30); do \
		$(COMPOSE) -f $(COMPOSE_FILE) exec -T postgres pg_isready -q -U swosh -d swosh && exit 0; \
		sleep 1; \
	done; \
	echo "Postgres did not become ready in time" >&2; exit 1

.PHONY: db-down
db-down: ## Stop the dev databases (data is kept in a volume)
	$(COMPOSE) -f $(COMPOSE_FILE) --profile mongo down

.PHONY: migrate-new
migrate-new: $(MIGRATE_STAMP) ## Create a new migration pair: make migrate-new name=add_foo
	@test -n "$(name)" || { echo "usage: make migrate-new name=<snake_case_name>" >&2; exit 1; }
	$(MIGRATE) create -ext sql -dir $(MIGRATIONS) -seq $(name)

.PHONY: migrate-up
migrate-up: $(MIGRATE_STAMP) ## Apply all pending migrations to SWOSH_DATABASE_URL
	$(MIGRATE) -path $(MIGRATIONS) -database '$(SWOSH_DATABASE_URL)' up

.PHONY: migrate-down
migrate-down: $(MIGRATE_STAMP) ## Roll back the most recent migration on SWOSH_DATABASE_URL
	$(MIGRATE) -path $(MIGRATIONS) -database '$(SWOSH_DATABASE_URL)' down 1
