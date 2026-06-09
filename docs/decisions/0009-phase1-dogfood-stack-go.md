# ADR-0009 — Phase 1 dogfood stack is Go
**Date:** 2026-06-08   **Status:** Accepted

## Context
PLAN Phase 1 and SPEC-playbook's example overlay (`prompiler`/`stack: python`) were
written when the engine language was undecided. ADR-0008 fixed the engine language to
Go. RULES §8 and the CHARTER require dogfooding "from the first runnable slice" — Loom
is built using Loom. A Python-first Phase 1 would prove a stack that does *not* build
the thing being built, so the first slice would never actually dogfood.

## Decision
Phase 1 proves the **Go** stack: `stacks/go` + `overlays/loom`, building Loom's own dev
environment inside the container Loom builds. Python (and/or TS) move to **Phase 2** as
the "second stack side-by-side" proof for container-per-project (ADR-0001) alongside
`update`. The Python examples remaining in the specs are illustrative only; the proven
Phase 1 path is Go.

## Alternatives considered
- **Keep Python as the first stack** (as originally written) — would require a second,
  non-dogfooded environment to exist before Loom can build itself. Rejected: violates
  dogfood-from-first-slice (RULES §8) and adds scope.
- **Prove Go and Python together in Phase 1** — doubles Phase 1 surface (two resolvers,
  two stacks) before the spine is stable. Rejected: Phase 1 is the thin vertical slice;
  the second stack is exactly what Phase 2 exists to prove.

## Consequences
- Positive: Loom dogfoods from the first runnable slice; the engine and its first target
  share the Go toolchain, so "build Loom in a Loom container + `go test`" is the Phase 1
  acceptance path; Phase 2's second-stack goal gains a concrete contrast (Go vs Python).
- Trade-offs: the cross-stack-isolation property (ADR-0001) is not exercised until Phase 2
  (only one stack exists in Phase 1); SPEC/PLAN examples that used Python need a note that
  Go is the proven path.
- Revisit if: dogfooding moves off Go, or Phase 1 scope expands to require two stacks.
