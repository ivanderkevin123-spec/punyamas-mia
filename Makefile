# MIA — Punyamas Marketing Intelligence Agent
# Go executable: defaults to the verified native toolchain (go1.27.2 darwin/arm64).
# Override with: make GO=/path/to/go <target>
GO ?= $(HOME)/sdk/go1.27.2/bin/go
# Local, project-scoped caches (git-ignored) so builds work in sandboxed sessions; override if you prefer the defaults.
GOCACHE ?= $(CURDIR)/.toolchain/gocache
GOMODCACHE ?= $(CURDIR)/.toolchain/modcache
export GOCACHE GOMODCACHE
export GOTOOLCHAIN := local
export GOFLAGS := -mod=mod

.PHONY: help go-version build test test-race vet run-mock config-check clean

help: ## Show targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

go-version: ## Print the Go binary and version actually used (must be go1.27.2 darwin/arm64)
	@echo "GO=$(GO)" && $(GO) version

build: ## Build bin/mia with the selected toolchain
	@mkdir -p bin && $(GO) build -o bin/mia ./cmd/mia

test: ## Unit, integration and workflow tests (mock provider only, no network)
	$(GO) test -count=1 ./...

test-race: ## Tests with the race detector
	$(GO) test -race -count=1 ./...

vet: ## go vet
	$(GO) vet ./...

run-mock: build ## End-to-end mock run on the sample batch
	./bin/mia run research --inputs testdata/sample-batch

config-check: build ## Fingerprint workspace assets and report drift
	./bin/mia config check

clean: ## Remove build output (keeps data/ and reports/)
	rm -rf bin
