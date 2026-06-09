# ADR-0007 — Cloud sandbox as a sibling track, not an installer mode
**Date:** 2026-06-05   **Status:** Accepted

## Context
Scenario 3 evolved from "shared machine / container-in-container" to a
load-on-demand cloud VM (dev-sandbox). Its concerns — provisioning, durable
state, cost controls, remote access — differ fundamentally from local containers.

## Decision
Keep the local two-tier engine as-is; design the cloud sandbox as a **sibling
deliverable** that reuses the same playbook + engine internals (config identical
local vs cloud) but wraps them in cloud provisioning. Same brain, different body.

## Alternatives considered
- **Cloud as a mode of the local installer** — muddies both; different lifecycle
  and failure modes. Rejected.

## Consequences
- Positive: clean separation; shared config guarantees parity; each track stays
  focused.
- Trade-offs: two delivery surfaces; must keep the shared internals stable.
- Revisit if: cloud and local converge enough to unify (unlikely near-term).
