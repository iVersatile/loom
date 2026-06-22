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
        cover fr-verify gate gate-integration hooks tools clean

all: gate

# CGO_ENABLED=0 → a fully static loom binary (no glibc/loader dependency). Required
# by T20 S2b (ADR-0028 A1): `egress: allowlist` cp's the loom binary into a slim
# sidecar and runs `loom __egress-proxy` there — a dynamically-linked binary may
# fail to exec in a base with a different libc. There is no `import "C"` in-tree, so
# this is a pure-Go build with the native net resolver; it never breaks the build.
build:
	CGO_ENABLED=0 $(GO) build -o $(BIN) ./cmd/loom

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
#
# Hermetic (RULES §5, FR-INV-004): scrub ambient override/config env
# (LOOM_*/ALLOW_*) and neutralize docker (a failing shim on PATH) so no unit test
# can read those vars or provision a real container — the local gate cannot diverge
# from CI on host env/tooling (LL-006/008). GIT_* repo-redirection vars are
# scrubbed too: a commit run with GIT_DIR/GIT_WORK_TREE set (e.g. from a host
# worktree) leaks them into the hook's gate, and git-shelling tests then write
# into the REAL repo's .git instead of their fixtures (LL-010 incident:
# core.worktree + identity clobbered in the shared .git/config). The integration
# tier deliberately keeps docker + LOOM_BASE_IMAGE.
test:
	@d=$$(mktemp -d); printf '#!/bin/sh\nexit 1\n' > "$$d/docker"; chmod +x "$$d/docker"; \
	env -u LOOM_BASE_IMAGE -u ALLOW_SPEC_CHANGE -u ALLOW_TRUST_CHANGE -u ALLOW_MAIN_COMMIT \
	  -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE -u GIT_OBJECT_DIRECTORY -u GIT_COMMON_DIR \
	  PATH="$$d:$$PATH" \
	  $(GO) test -timeout 120s ./...; \
	rc=$$?; rm -rf "$$d"; exit $$rc

# Integration tier is docker-backed: each e2e provisions a real container, so the
# wall-clock scales with the number of e2e tests (the guard e2e, FR-GUARD-E2E, adds
# another full build). The timeout is a hang-guard, not the budget — keep it well
# above the cumulative provisioning time. ADR-0012 (baked base image) will cut this
# from minutes to seconds.
test-integration:
	$(GO) test -timeout 1200s -tags integration ./...

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

# FR traceability (ADR-0013): every FR links a passing test (dangling = blocking)
# and cites an existing spec section; orphan tests are advisory. Out of the
# per-commit gate by design (advisory by default); blocking at the merge boundary
# (the CI fr-verify job).
fr-verify:
	$(GO) test -tags frcheck ./internal/fr/

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
