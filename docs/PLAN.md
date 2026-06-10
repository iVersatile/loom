# PLAN — Loom Roadmap

Staged, thin-vertical-slice first (prove the spine before breadth). Specs precede
code (ADR-0006: the contracts are the product).

<!-- BEGIN TACTICAL QUEUE (agent-maintained — agents edit ONLY this fenced
     section; the phase roadmap below is human-owned) -->
## Tactical queue (agent-maintained)

Bookkeeping rule: **every PR updates its own row in that same PR** (status, PR
link); unplanned work **adds its row in the PR that does it**. The Coordinator
hat (docs/TEAM.md) audits queue integrity and proposes re-sorts via `/replan`.

| task | depends-on | serves | owner | status | PR |
| --- | --- | --- | --- | --- | --- |
| public pre-flight (history sweep, PI review, front door) | — | repo-public flip | loom-author | done — go report 2026-06-10 | governance batch |
| governance batch (queue, TEAM.md, CODEOWNERS, /replan, push script, ADR-0015 flip) | — | team model | loom-author | done | #28–#34 |
| T7 home re-sync fix | — | T16 precondition | loom-author | done | #25 + #26 |
| topology doc (mac-dev / windows-dev / ai-user) | — | public face; topology-aware design | loom-author | done | #27 |
| ADR-0015 bookkeeping (status → Accepted) | PR #24 merged | T16 | loom-author | done | #28–#34 |
| harness synthesis doc (HARNESS.md, status-marked) | ADR-0015 ✓ | orientation; T16 PRs update its markers | loom-author | done | #28–#34 |
| T16 engine work (`harness:` schema + materialize handlers) | ADR-0015 ✓, T7 fix | T12 criterion 4 | loom-author | next | — |
| T9 spec clause (exec/shell — human-authored, C3) | — (decided, see T9 thread) | T12 criterion 2 | human | done — clause is law | #35–#40 |
| T9 exec impl (cli + engine + FRs; shell follows) | clause merged ✓; ADR-0016 | T12 criterion 2 | loom-author | in progress | feat/t9-exec |
| gate hermeticity: GIT_* scrub + fixture hardening (LL-010 incident) | — | gate integrity; every git client everywhere | loom-author | in review | fix/gate-git-env-hermeticity |
| ADR-0016 entry verbs (Proposed; FR drafts staged inside) | T9 decided ✓ | T9 impl | loom-author | in review | docs/adr-0016-entry-verbs |
| T4 container PATH single declarative owner | — | env correctness | loom-author | queued | — |
| T15 successor: AI-first credential acquisition (incl. VCS/gh) | — | autonomy (T18 push gap) | loom-author | queued | — |
| T10 non-root user / parameterized $HOME | — | hardening | loom-author | queued | — |
| T12 closeout: delete archived devenv | archive + 14d | cutover bookkeeping | human | scheduled 2026-06-24 | — |
| re-run auto-mode evaluation (full-auto clearance) | T16 hooks landed, T10 non-root, T20 decided | autonomy | loom-advisor | blocked — event-driven (no calendar): when all three deps flip, checklist-diff vs the recorded evaluation (allowlist ✓, deny-floor ✓, code-exec egress, root, guard hooks) | — |
<!-- END TACTICAL QUEUE -->

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
