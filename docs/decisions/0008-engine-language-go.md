# ADR-0008 — Engine language: Go
**Date:** 2026-06-08   **Status:** Accepted

## Context
Phase 1 starts the engine codebase, so the implementation language must be fixed
first (RULES §2: specs/contracts before code). The engine is mechanism (ADR-0006):
orchestration glue that parses the playbook, validates the schema, computes diffs
(`plan`), shells out to the real workers (docker/podman, brew, uv), scans the
machine (`detect`), and renders dual human + `--json` output (ADR-0005). The heavy
lifting is delegated; the engine is logic, not a hot path.

The decision was weighed against three owner-stated factors: (1) performance and
lightweight footprint when a team spins up / tears down multiple looms; (2) lowest
possible learning curve; (3) compatibility with established tools and tech.
Reference points from comparable products: devcontainers/cli = TypeScript/Node,
Devbox = Go, devenv = Rust CLI over Nix config.

## Decision
Build the Loom engine in **Go**, with a thin POSIX-sh bootstrap for first-touch
on a bare machine (detect situation → ensure engine present → hand off). Engine
toolchain: `go test`, `golangci-lint`, `go vet`. Playbooks remain YAML/JSON
(ADR-0002) — the engine language does not change the authoring surface.

## Alternatives considered
- **Python (uv/ruff/mypy/pytest)** — lowest learning curve and matches the
  owner's env-wide toolchain; best authoring speed and agent-legibility. But
  heavier footprint (interpreter + venv per environment), slower cold start across
  many invocations, and less native to container tech. Strong #2; rejected because
  footprint/startup lose on factor 1 and it is less container-native on factor 3.
- **Rust (over a Nix-style core)** — top performance, but steepest learning curve
  by a wide margin (borrow checker, lifetimes, async ceremony), directly violating
  factor 2. Performance is not engine-bound here, so the cost buys little. Rejected.
- **Bash** — the prototype's pain (ADR-0006); cannot do `--json`/schema/idempotence
  cleanly. Kept only as the ~20-line bootstrap. Rejected as the engine.

## Consequences
- Positive: single static binary, instant cold start, tiny footprint → fast,
  lightweight multi-loom spin-up/teardown (factor 1); Go is the native language of
  the container/cloud-native ecosystem (Docker/containerd/BuildKit), so first-party
  client libraries and idioms are available instead of parsing CLI output
  (factor 3); simple language, quick to onboard (factor 2); mirrors Devbox, the
  closest analog to Loom's job.
- Trade-offs: slower feature authoring and more boilerplate than Python; engine
  toolchain diverges from the owner's Python-tuned env (go test/golangci-lint, not
  pytest/ruff/mypy); a modestly higher bar for agents editing the engine (minor —
  Go is well-represented). Python stays a *stack Loom provisions*, never the engine.
- Revisit if: the engine stops being delegation-glue and needs an embedded
  evaluator/DSL (would reweigh Rust), or distribution stops mattering because the
  engine only ever runs inside a Loom-owned image (would reopen Python).
