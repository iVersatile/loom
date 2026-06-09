# ADR-0012 — Prebuilt base image bakes the toolchain (provision becomes a thin overlay)
**Date:** 2026-06-09   **Status:** Accepted — implementation Phase 2 (needs the publish pipeline)

## Context
Phase 1 provisions every container from `debian:bookworm-slim` **at build time**:
`apt-get update` + install, download the Go tarball, then **compile `gopls` /
`gitleaks` from source**. Two problems surfaced building Loom itself:

- **Memory:** the source compiles (gopls especially) OOM small VMs / ~7GB CI
  boxes — the reason `GOFLAGS=-p` / `GOMEMLIMIT` mitigations exist (ADR-0011).
- **Environment fragility:** on this Docker Desktop (Apple Silicon) the apt
  **package-list cache build** fails *deterministically* — reproducible with a
  vanilla `docker run debian:bookworm-slim apt-get update` (download succeeds,
  "Reading package lists…" never completes), with 7+ GiB free. So the live-apt
  path is fragile independent of Loom, and CI runners are ~7GB too — "add RAM"
  does not generalize.

ADR-0011 made `build` self-healing and added retries/caps, but that hardens a
path that is heavy and environment-sensitive *by construction*: every container
create recompiles the world and runs fragile live apt. That is treating the
symptom.

## Decision
Move the heavy, environment-sensitive work **out of the per-build hot path** by
baking the common toolchain — apt deps + the Go toolchain + `gopls` / `gitleaks`
+ `uv` — into a **prebuilt base image**, built on a healthy/CI builder and
published to the ghcr mirror. Per-project `build` then **pulls** that image
(`LOOM_BASE_IMAGE`, already wired in `build.go`) and provisioning shrinks to a
**thin overlay**: seed `$HOME`, plus only project-specific extras not already in
the base. The base image is digest-pinned in `loom.lock` (consistent with the
existing base-image digest pin).

The from-source in-container provisioning path (ADR-0011) **remains as the
fallback / bootstrap** — for tools not in the base, and for environments without
the mirror — gated by the same provision sentinel so an existing container still
reconciles. This is ADR-0001's shared-base-image principle taken to its
conclusion: the base is a **toolchain floor**, not just an OS floor.

## Alternatives considered
- **Only harden from-slim provisioning (ADR-0011 retries/caps), keep it the
  default.** Treats the symptom: every build still compiles `gopls` and runs the
  fragile apt cache build; broken where local apt is broken. Kept as fallback,
  rejected as the end state.
- **Bake everything into per-project images.** Defeats the shared-base dedup
  (ADR-0001/0002 thin surface) and explodes image count/storage. Rejected.
- **Install Go/`gopls` from distro packages instead of tarball/source.** Version
  skew vs the resolver's intent pins; Debian's Go lags. Rejected.

## Consequences
- Positive: `build` is fast and reliable on constrained/odd VMs (no `gopls`
  compile, no live apt cache build at build time); the fragile local-apt path is
  bypassed; CI parity (image built once on a healthy builder); reproducible
  (digest-pinned base). Realizes ADR-0001.
- Trade-offs: a new artifact + pipeline to **build / publish / scan** the base
  image — a supply-chain surface that must be digest-pinned and
  provenance-checked (RULES guardrails: the agent may not add an unverified base);
  a base-image refresh cadence to manage; two provisioning paths (baked + source
  fallback) to keep coherent via the sentinel + resolver.
- Revisit if: per-project toolchains diverge enough that one shared base stops
  paying (→ per-stack base images), or the cloud track (ADR-0007) moves where
  images live.
