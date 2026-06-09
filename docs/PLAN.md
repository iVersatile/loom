# PLAN — Loom Roadmap

Staged, thin-vertical-slice first (prove the spine before breadth). Specs precede
code (ADR-0006: the contracts are the product).

## Phase 0 — Seed (this commit)
- Charter, ADRs (0001–0007), SPEC-playbook, SPEC-verbs, RULES, AGENTS, this PLAN.
- No engine code yet. Freeze the schema + verb contracts via review first.

## Phase 1 — Spine (scenario 1, one stack, local)
Goal: `detect → plan → build` end-to-end for a single Go project (Loom itself,
ADR-0009), with `--json` and guardrails wired. Dogfood: build Loom inside the
container Loom builds.
- Playbook parser (base + one overlay; merge rule).
- Resolver: intent → concrete (apt/uv, go from base image) for a small tool set;
  write loom.lock.
- `build`: one container from a shared base + project overlay.
- `detect` + `plan` with `--json`; `teardown` (reuse validated tiered script).
- Guardrails: guard-bash, branch-guard, protect-paths active.
Exit criteria: a fresh machine reaches a working Go env in one guided run;
an agent can `plan` then `build` unattended; guardrails block a destructive test.
FR registry seeded from verb contracts; verify FR↔test and FR↔spec checks green
(advisory during the phase, blocking at phase close).

## Phase 2 — Evolution + second stack
- `update` (delta reconcile; real removal). Add a Python or TS stack to prove
  container-per-project with different stacks side-by-side (ADR-0001).
- Credential continuity: `detect --emit-playbook`, detect+report, `--migrate`.
- Menu-driven `start` entry (situation detection) for scenario 1/2.
- **Prebuilt base image (ADR-0012):** bake the toolchain (apt deps + Go + gopls +
  uv) into a published, digest-pinned base so `build` provisioning is a thin
  overlay — needs the build/publish/scan pipeline. The durable fix for the
  constrained-VM / fragile-local-apt provisioning failures that ADR-0011's
  resilience only bridges.

## Phase 3 — AI-first surface hardened
- Full `--json` on all verbs; action log; `doctor`.
- Decide + (maybe) build MCP server wrapping the verbs (open question Q1).
- Playbook guards (agent can't weaken deny-rules / add exfiltrating tools).
- **agent-debug-and-fix** (note, candidate — needs its own ADR before code):
  close the autonomous loop. On a failed mutating verb, the agent reads the
  diagnostic log (ADR-0010), diagnoses, and applies a *bounded* fix (edit
  overlay / re-resolve / retry) unattended, inside guardrails. `doctor` is the
  read side; this is the act side. Surfaced by Phase 1 build-troubleshooting:
  diagnostics make failures legible, but legible ≠ self-healing. Open: how the
  fix is bounded (which mutations an agent may self-authorize vs must escalate),
  and whether it's a new verb or a `--fix` mode. Spec before building.

## Phase 4 — Devcontainer compatibility (staged, ADR-0003)
- `import` (deterministic core; AI skill for awkward fields).
- Stage 1→2→3 enrichment path documented + tooled.
- `export` (lossy, later).

## Phase 5 — Cloud sandbox sibling (ADR-0007)
- VM provisioning reusing the same playbook + engine internals.
- Durable state (volume/secret store/git); auto-shutdown cost control; remote access.

## Open items (not blocking specs)
- Working env for building Loom: cmux on Mac attached to the Loom dev container,
  Claude Code in-container (dogfood). Verify cmux holds a long-lived exec pane;
  fallback = mount loom + claude into /usr/local/bin in-container.
- **Agent-initiated container lifecycle with task continuity.** The agent runs
  *inside* the container it operates on; a `teardown`/restart (even `docker stop`)
  kills the agent and its in-flight task context (observed 2026-06-09: a
  `docker stop devenv-dev` lost a live session mid-task). For "operable by an
  autonomous agent, no human in the loop" to hold, the agent's task state must
  persist **outside** the container and **rehydrate** on bring-up. Relates to
  ADR-0007 (durable state) and the agent-debug-and-fix note (Phase 3). See the
  spec question below.
