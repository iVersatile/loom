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
| T9 spec clause (exec/shell — human-authored, C3) | — (decided, see T9 thread) | T12 criterion 2 | human | done — clause is law | #35–#40 |
| T9 exec impl (cli + engine + FRs; shell follows) | clause merged ✓ (#35–#40) | T12 criterion 2 — met; criterion 5 human-verified through the verb | loom-author | done | #43 |
| gate hermeticity: GIT_* scrub + fixture hardening (LL-010 incident) | — | gate integrity; every git client everywhere | loom-author | done | #41 |
| ADR-0016 entry verbs (Accepted — human merge) | T9 decided ✓ | T9 impl | loom-author | done | #42 |
| prompt-volume relief: evidence-based allowlist + compound allow-hook | — | human unbottlenecked; deny floor unchanged | loom-author | done | #45 (+ TEAM floor #46) |
| FR-BUILD-008 clean-machine proxy (Phase-1 criterion 1, T1 doctrine) | — | phase-close verification debt | loom-author | done | #47 |
| Phase-1 reality audit (README quickstart + shape, T12 thread flips) | — | guided-run brief | loom-author | done | #48 |
| T21 thread + OPEN-THREADS archival diet (11 ✅ threads → docs/threads/archive/, stubs + convention) | — | durable design record; relay → mechanism | loom-author | done | #49 |
| T21 mechanism: inbox + Stop-hook drain + dispatcher | T21 thread merged | human stops being the message bus | loom-author | done | #50 |
| AGENTS.md communication section (brief replies, lists/options, action+summary) | — | human-stated reply-style preference, team-wide | loom-advisor (human's hands) | done | #51 |
| queue replan 2026-06-11 (flip #47–#51 → done, re-sort into done/live/blocked bands) | — | queue integrity (bookkeeping rule) | loom-author | done | #52 |
| SPEC-playbook `harness:` clause (drafted by agent, ALLOW_SPEC_CHANGE human-authorized 2026-06-11; merge = acceptance) | ADR-0015 ✓ | T16 engine work — FR grounding | loom-author (human accepts) | done — accepted | #53 |
| drain-hook role guard (LL-011: advisor stop drained Writer's inbox; inbox item 002) | — | T21 transport correctness; AUTOPILOT re-flip gate | loom-author | done | #54 |
| auto-trial evidence ledger (day-0 LL-011 record + daily audit table; trial spec links it) | — | trial measurement (decided package §4) | loom-advisor (human's hands) | done | #55 |
| T16 engine work (`harness:` schema + materialize handlers) | ADR-0015 ✓, T7 fix ✓, `harness:` clause accepted ✓ (#53) | T12 criterion 4 | loom-author | in progress — PR 1 #86 + PR 2 #87 merged 2026-06-12; PR 3 (gitconfig+doctor) remains, was queued behind both chains (now merged) | #86, #87 |
| T9 shell impl (TTY sugar over exec; session-open audit) | exec merged ✓ (#43) | human topologies entry | loom-author | done — FR-SHELL-001 | #85 |
| ci-red auto-file workflow (CI failure on main → auto-filed issue; dedup, ci-red label) | — | reds become tracked work, not human-noticed accidents | loom-author | done | #73 |
| /achievements project skill (queue-anchored daily/period digest: spec/decision/mechanism categories; report-only, no tree writes) | — | consistent shipped-work narration for human + reviews | loom-author | done | #63 |
| context economy + intake lane + /coordinate (TEAM clause, fyi/draft kinds, drain guards + tests, two-mode skill, hat authority, standup rendering) | — | cross-role context without relay or over-minting; coordinator hat mechanized | loom-author | done | #78 |
| guard-bash segment-aware evaluation (specimen: --force + main matched one regex across unrelated chain segments) | — | fewer false blocks AND false prompts | loom-author | queued | — |
| T20 egress: observe→enforce proxy (decided 2026-06-11: logged sidecar, evidence-built allowlist, graduate to playbook `network:` field) | trial verdict (post-trial build) | code-level exfil closed; unblocks bootstrap-fetch amendment, D-stage minting, full-auto re-eval | loom-author | queued — decision blessed, envelope = inbox item 009 | — |
| T4 container PATH single declarative owner | — | env correctness — decided 2026-06-11 (option 2 + spec note) | loom-author | queued — envelope item 020 (triage 2026-06-12); rides the T16 track (same files as T16 PR 1) | — |
| weekly report: /coordinate WEEKLY MODE (docs/reports/YYYY-WW.md; confidence lens, dependency graph, coverage map, EXPERIMENTS section; absorbs P8) | — | human-decided P12 2026-06-11; first issue 2026-06-18 = trial day-7 verdict report | loom-author | queued — envelope item 021 (triage 2026-06-12) | — |
| phase-close review gate transcription (TEAM.md clause + rubric doc + RULES pointer [frozen — human admin-merge]) | — | human-decided P7 2026-06-11; gate already executed for Phase 1 (docs/reviews/phase-1-review.md) | loom-author | queued — envelope item 022 (triage 2026-06-12) | — |
| work-selection policy transcription (cascade + principles p1–p5 + dormant mechanics; TEAM.md coordinator section, /coordinate + /replan SKILL.md) | — | human-blessed 018 2026-06-11; coordinator runs already apply it | loom-author | queued — envelope item 023 (triage 2026-06-12) | — |
| Phase-1 review findings, Critical/High fix batch (C1 guardrail wiring [RULED FIX-NOW 2026-06-12]; H2–H4; F1 teardown state surface; F2 plan blind dimensions) | C1 ruling ✓; review on record (docs/reviews/phase-1-review.md) | phase-close gate: Critical = no waiver; Highs = fix or written acceptance | loom-author | done — merged 2026-06-12 (C1 doctor-wiring+githooks, H2 fail-closed audit, H3/H4 guard-bash+deny floor, F1 clean-state real, F2 plan dimensions); C1 fix verified live by e2e build-under-session same day (.scratch/live-build-experiment.md) | #92 |
| live-build e2e findings F-a..F-d (build summary≠writes; home-sync audit gap; build.log overwrite; deliberate lock refresh w/ digest provenance) | — (e2e record: .scratch/live-build-experiment.md, promote to docs/e2e/ pending) | R1 audit-delta family (T26); build observability | loom-author | queued — envelope 038 (2026-06-12) | — |
| 036 harness-state durability: playbook materializes trust/opt-in flags at build ("2-plus", human-decided 2026-06-12; `.claude.json` sibling file on overlay dies on recreate — BOTH seats, advisor-verified; declare-or-rederive per devcontainer/Codespaces precedent, ADR-0014/0015 aligned) | — | Writer durable opt-in; autopilot survives recreate; prerequisite for ADR-0017 target state | loom-author | queued — envelope 036 (ruled 2026-06-12) | — |
| T15 successor: AI-first credential acquisition (incl. VCS/gh) | trial verdict | autonomy (T18 push gap) | loom-author | queued — decided C′→D 2026-06-11, envelope item 008 (self-promotes at verdict) | — |
| T10 non-root user / parameterized $HOME | — | hardening | loom-author | queued | — |
| one-week defaultMode "auto" trial (classifier mode; deny floor + never-auto floor unchanged) | exit/rollback package T22 ✓ (docs/auto-trial.md) | prompt volume ↓ | **human flipped 2026-06-11; revert pre-authorized, re-flip human-only** | RUNNING — flipped #70, clock started 2026-06-11; day-7 verdict 2026-06-18: keep iff zero S1+S2 | #70 |
| T22 transcription: auto-trial exit/rollback package (thread stub + docs/auto-trial.md; inbox item 001) | — | trial failure contract on record | loom-author | done | #56 |
| T23 transcription: AUTOPILOT scoping (role × project, HALT kill-switch + test, flips.log; inbox item 003) | T21 ✓, LL-011 fix ✓ (#54) | transport trust model on record; atomic both-roles rollback | loom-author | done | #57 |
| baseline scan, Writer side (would-prompt metric; inbox item 004 — flip gate) | advisor scan ✓ (#58) | auto-trial §1 precondition; the flip waits on this | loom-author | done | #60 |
| guided-run runbook (docs/guided-run.md) — orphan-PR backfill, /replan 2026-06-11 audit | — | Phase-1 close prep | loom-advisor (human's hands) | done | #62 |
| bootstrap-entry spec clause (loom-bootstrap.sh first touch — no clause covers it; C3) | human decision 2026-06-11 ✓ (inbox item 007) | FR-BUILD-008 remainder | loom-author transcribed — human admin-merge | done — accepted; FR extraction = inbox item 010 | #64 |
| spec-map v1 + r2 (docs/spec-map.md, threads shadow on SPECs/FRs) — orphan backfill, /replan audit | — | spec/thread shape visible | loom-advisor (human's hands) | done | #67, #71 |
| /achievements format amendment (lifecycle dashboard) — orphan backfill, /replan audit | — | validated report format | loom-advisor (human's hands) | done | #65 |
| FR extraction: entry:bootstrap → FR-ENTRY-001..004 + hermetic sh-fixture tests (inbox item 010; spec-map node → green) | clause accepted ✓ (#64) | registry debt closed; spec→FR→test joint | loom-author | done | #74 |
| /specmap project skill (regenerates docs/spec-map.md; writes via PR — inbox item 011; row may dup docs/specmap-row) | spec-map v1 ✓ (#67) | spec→FR→thread shape on demand | loom-author | done | #80 |
| guided-run findings batch (⑦ plan/build convergence disagreement +LL +regression; ⑧ doctor probe scope; ⑨ teardown unconfirmed; ①–⑥ doc/ergonomics gaps) | guided run ✓ (docs/guided-run.md results) | criterion-1 evidence → fixes; stranger-path ergonomics | loom-author | done — ⑦ #79, ⑧ #82, ⑨ #83, ①–⑥ #84 (item 013) | #79, #82–#84 |
| Phase-1 close (PLAN edit) | guided run ✓ MET 2026-06-11 (docs/guided-run.md verdict); review gate 2026-06-12 (docs/reviews/phase-1-review.md) | phase boundary | **human — no agent self-approves phase completion** | done — **PHASE 1 CLOSED 2026-06-12**, human squash-merged the close edit (evidence block in the Phase-1 roadmap section); Phase 2 active | #99 |
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

**Status: CLOSED 2026-06-12 (human).** Exit criteria met with evidence:
- *Fresh machine, one guided run* — executed 2026-06-11, human verdict in
  docs/guided-run.md (go 1.26.4 in-container via exec; build 3m08s, rebuild
  2.5s, teardown clean); findings ①–⑨ filed and fixed (#79, #82–#84).
- *Agent can plan then build unattended* — dogfood exceeded: `loom exec --
  make gate` PASS on the Mac host (T12 criterion 5, loom-native); plan/build
  convergence disagreement found and fixed (#79, FR-PLAN-003).
- *Guardrails block a destructive test* — guard suite green plus live
  specimens on record (guard-bash H4 block; deny floor held under the
  2026-06-12 auto-trial stress, docs/trial-auto-evidence.md).
- *FR registry seeded, FR↔test / FR↔spec checks green* — fr-verify is a
  required branch-protection check; FR-BUILD-008 clean-machine proxy (#47).

Phase-close review gate (P7, first application): docs/reviews/phase-1-review.md
— C1 Critical (guardrails not wired into built containers) ruled fix-now,
fixed #92, and verified live by the build-under-session e2e the same day;
H2–H4 and F1/F2 fixed in the same batch. No waivers outstanding.
Phase 2 is the active phase.

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
