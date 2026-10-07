# TRUSS developer tasks. Requires Go (version from go.mod) and a POSIX
# shell (on Windows, use Git Bash or WSL). CI invokes the same commands.

SHELL := /bin/sh

MODULE  := github.com/afri-bit/truss
BIN_DIR := bin
BINARY  := $(BIN_DIR)/truss$(shell go env GOEXE)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

GOLANGCI_LINT_VERSION ?= v2.14.0
GOLANGCI_LINT         ?= $(BIN_DIR)/golangci-lint

COVER_PROFILE ?= coverage.out

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the truss binary into bin/
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/truss

.PHONY: test
test: ## Run unit tests
	go test ./...

.PHONY: test-race
test-race: ## Run unit tests with the race detector (needs cgo)
	go test -race ./...

.PHONY: test-integration
test-integration: ## Run unit + integration tests (needs Docker)
	go test -race -tags integration ./...

.PHONY: lint
lint: $(GOLANGCI_LINT) spec-check ## Run golangci-lint and repository checks
	$(GOLANGCI_LINT) run ./...

.PHONY: fmt
fmt: $(GOLANGCI_LINT) ## Format Go code (gofmt + goimports)
	$(GOLANGCI_LINT) fmt ./...

.PHONY: tidy
tidy: ## Tidy go.mod and go.sum
	go mod tidy

.PHONY: cover
cover: ## Run tests with coverage and print a summary
	go test -covermode=atomic -coverprofile=$(COVER_PROFILE) ./...
	go tool cover -func=$(COVER_PROFILE) | tail -n 1

.PHONY: spec-check
spec-check: ## Verify specs/_template matches .specify/templates
	@for f in spec plan tasks; do \
		cmp -s specs/_template/$$f.md .specify/templates/$$f-template.md || { \
			echo "specs/_template/$$f.md differs from .specify/templates/$$f-template.md"; exit 1; }; \
	done

.PHONY: tools
tools: $(GOLANGCI_LINT) ## Install pinned development tools into bin/

$(GOLANGCI_LINT):
	GOTOOLCHAIN=$$(go env GOVERSION) GOBIN=$(CURDIR)/$(BIN_DIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: clean
clean: ## Remove build and coverage output
	rm -rf $(BIN_DIR)/truss $(BIN_DIR)/truss.exe $(COVER_PROFILE) dist
