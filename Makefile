# Loom gate — ONE entry point shared by the pre-commit hook and CI so the two can
# never drift (RULES §7). `gate` is the fast, blocking local core; `gate-integration`
# adds the docker-backed e2e tier (CI, or local where docker is available).

GO       ?= go
BIN      := bin/loom
GOBIN    := $(shell $(GO) env GOPATH)/bin
# Resolve tools whether or not GOPATH/bin is on PATH (e.g. inside a git hook).
GOLANGCI ?= $(shell command -v golangci-lint 2>/dev/null || echo $(GOBIN)/golangci-lint)
GITLEAKS ?= $(shell command -v gitleaks 2>/dev/null)

.PHONY: all build fmt fmt-check vet lint spec-check test test-integration secrets \
        cover gate gate-integration hooks tools clean

all: gate

build:
	$(GO) build -o $(BIN) ./cmd/loom

fmt:
	gofmt -w .

fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi

vet:
	$(GO) vet ./...

lint:
	@test -x "$(GOLANGCI)" || { echo "golangci-lint not found; run 'make tools'"; exit 1; }
	$(GOLANGCI) run

# Explicit, named contract check: live --json must match SPEC-verbs.md shapes.
spec-check:
	$(GO) test -run TestSpecConformance ./internal/cli/

# Unit tier is sub-second; the timeout only fails a *hung* test fast (vs Go's
# 10-min default). Benchmark/baseline: docs/TESTING.md.
test:
	$(GO) test -timeout 120s ./...

# Integration tier is docker-backed (provisions a real container). Budget < 5 min;
# the timeout is a hang-guard, not the budget. ADR-0012 (baked base image) will
# cut this from minutes to seconds.
test-integration:
	$(GO) test -timeout 600s -tags integration ./...

# Coverage report (unit tier). Baseline/floor recorded in docs/TESTING.md.
cover:
	$(GO) test -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

secrets:
	@test -n "$(GITLEAKS)" || { echo "gitleaks not found"; exit 1; }
	$(GITLEAKS) detect --no-git --no-banner --redact

# Fast, blocking local gate (RULES §7).
gate: fmt-check vet lint spec-check test secrets
	@echo "gate: PASS"

# Heavier CI tier: docker-backed integration/e2e (lands in Work 7).
gate-integration: test-integration
	@echo "gate-integration: PASS"

# Install pinned dev tools into GOPATH/bin.
tools:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	# gitleaks v8's module path is the legacy zricethezav/ one (LL-004); the
	# github.com/gitleaks/ path fails `go install` with a path conflict.
	$(GO) install github.com/zricethezav/gitleaks/v8@latest

# Enable the tracked git hooks.
hooks:
	git config core.hooksPath .githooks
	@echo "hooks: core.hooksPath -> .githooks"

clean:
	rm -rf $(BIN)
