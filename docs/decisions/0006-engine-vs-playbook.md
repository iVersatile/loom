# ADR-0006 — Build engine vs playbook (mechanism/policy separation)
**Date:** 2026-06-05   **Status:** Accepted

## Context
Tools and content kept changing in the prototype because install machinery and
install content were entangled (heredocs inside the setup script).

## Decision
Separate **mechanism** (the build engine: stable, knows *how* — detect/plan/
build/update/teardown) from **policy** (the playbook: versioned, declares *what*).
The engine contains only logic; all content comes from the playbook dependency.
Models: docker-compose reads a YAML it doesn't hardcode; Terraform plans/applies a
spec. Loom's engine reconciles reality to the playbook.

## Alternatives considered
- **Content baked into scripts (heredocs)** — couples machinery to content; can't
  version policy independently. Rejected (this was the prototype's pain).

## Consequences
- Positive: policy versions independently (per-person or team); engine stays
  stable; "configs as a dependency" realized; evolution = committed playbook edits.
- Trade-offs: needs a resolver and a clear engine/playbook contract.
- Revisit if: never expected — this is foundational.
