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
| **autonomy closed-loop: PARK → pull-next → re-surface** (fixes Writer-halts-on-block + human-as-transport) — PRIORITY autonomy item, ahead of T10 PR 4 | convergence round (adv-051) → ADR (human) BEFORE build (§9 close-the-circle; RULES §5 convergence) | agent-initiated lifecycle / task continuity (phase roadmap); removes human from transport, keeps human on gates | loom-advisor + loom-author (converge); build = loom-author | in review — CONVERGED (rounds 1–3, ledger exhausted) → **ADR-0020 MERGED #136**. AUTHOR BUILD SLICE (THIS, adv-054): `config/hooks/resurface-decide` (injection-proof fixed-vocab decision: deliver/skip-parked/resurface/over-age-ESCALATE/skip-superseded) + `config/hooks/pull-next` (R5 no-inversion picker) + `internal/guard/resurface_test.go` (gates the hook diff) + TEAM.md PARK schema & park-on-block behavior + FR-GUARD-RESURFACE/FR-LOOP-001. Gate+fr-verify green. REMAINING: trust-path drain-hook diff = human-applied #137 (patch prepared by advisor) | ADR-0020, #136 |
| **autonomy substrate (ADR-0022): ephemeral worker + backlog-ready pull** — extends ADR-0020; kills writer-idle (self-wake + self-refill from backlog) | ADR-0022 MERGED #149 (red-teamed → 5 amendments folded); spawner slice gated on ADR-0019 PR 4 | agent-initiated lifecycle / task continuity; removes the human from the inbox-refill valve, keeps human on gates + self-selection confirm | build = loom-author (advisor reviews — independent context, designer ≠ builder) | SLICE 1 in review (adv-058): offline readiness-runner SHIPPED — `scripts/readiness-decide` (external-truth deps, NOT subject-grep; structured `[class:exec]` tag = exec-ready/self-selection floor; no-inversion R5; fixed-vocab, injection-proof) + `internal/guard/readiness_test.go` (forged-deps + injection cases) + TEAM.md/scripts README docs. Gate green; awaiting advisor review + git-controller push+PR. Slice-1 home = `scripts/` (agent-committable, offline); relocates under `config/hooks/` behind protect-paths (human-applied) when load-bearing. Slices 2–5: promote-next (full-key serves) · cross-spawn rate ledger + HALT-before-spawn · spawner/actuator (gated PR4) · FR+guard wiring. Trust-path bits human-applied | ADR-0022, #149 |
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
| T16 engine work (`harness:` schema + materialize handlers) | ADR-0015 ✓, T7 fix ✓, `harness:` clause accepted ✓ (#53) | T12 criterion 4 | loom-author | done — PR 1 #86 + PR 2 #87 + PR 3 #107 all merged 2026-06-12; T16 ✅ resolved (ADR-0015 Accepted) | #86, #87, #107 |
| T9 shell impl (TTY sugar over exec; session-open audit) | exec merged ✓ (#43) | human topologies entry | loom-author | done — FR-SHELL-001 | #85 |
| ci-red auto-file workflow (CI failure on main → auto-filed issue; dedup, ci-red label) | — | reds become tracked work, not human-noticed accidents | loom-author | done | #73 |
| /achievements project skill (queue-anchored daily/period digest: spec/decision/mechanism categories; report-only, no tree writes) | — | consistent shipped-work narration for human + reviews | loom-author | done | #63 |
| context economy + intake lane + /coordinate (TEAM clause, fyi/draft kinds, drain guards + tests, two-mode skill, hat authority, standup rendering) | — | cross-role context without relay or over-minting; coordinator hat mechanized | loom-author | done | #78 |
| Phase-1 review fix batch (item 024) follow-ups: build observability (F-a ensured/written summary split, F-b home.sync audit entry, F-c diag-log append; F-d lock refresh = host digest verification first) | #92 merged ✓ | live-build e2e findings (.scratch/live-build-experiment.md, R1 audit-delta family) | loom-author | done — F-a/F-b/F-c merged 2026-06-12 (#111); F-d (deliberate lock refresh) still HELD on host digest verification (034) | #111 |
| guard-bash segment-aware evaluation (specimen: --force + main matched one regex across unrelated chain segments) | T29 design red-teamed + decided ✓ (conditional pass 2026-06-12) | fewer false blocks AND false prompts — deny floor match set must never shrink | loom-author | done — merged 2026-06-12: #112 design, #121 impl (amendments 1–5: two-pass fail-closed, both patterns re-anchored w/ was-false-blocking regressions, class-action wording, broad indirection taint, ONE shared splitter + cross-segment trace), #123 allow-compound adopts the shared splitter; T29 ✅ implemented | #112, #121, #123 |
| T20 egress: observe→enforce proxy (decided 2026-06-11: logged sidecar, evidence-built allowlist, graduate to playbook `network:` field) | trial verdict (post-trial build) | code-level exfil closed; unblocks bootstrap-fetch amendment, D-stage minting, full-auto re-eval | loom-author | queued — decision blessed, envelope = inbox item 009 | — |
| T4 container PATH single declarative owner | — | env correctness — decided 2026-06-11 (option 2 + spec note) | loom-author | done — merged 2026-06-12, human acceptance (SPEC note) | #105 |
| weekly report: /coordinate WEEKLY MODE (docs/reports/YYYY-WW.md; confidence lens, dependency graph, coverage map, EXPERIMENTS section; absorbs P8) | — | human-decided P12 2026-06-11; first issue 2026-06-18 = trial day-7 verdict report | loom-author | done — merged 2026-06-12 (relay push by advisor git-controller routine) | #109 |
| phase-close review gate transcription (TEAM.md clause + rubric doc + RULES pointer [frozen — human admin-merge]) | — | human-decided P7 2026-06-11; gate already executed for Phase 1 (docs/reviews/phase-1-review.md) | loom-author | done — merged 2026-06-12 (#114; TEAM.md clause + docs/review-gate.md rubric; frozen-contract, human admin-merge = acceptance) | #114 |
| work-selection policy transcription (cascade + principles p1–p5 + dormant mechanics; TEAM.md coordinator section, /coordinate + /replan SKILL.md) | — | human-blessed 018 2026-06-11; coordinator runs already apply it | loom-author | done — TEAM.md clause merged 2026-06-12 (#108; frozen-contract, human admin-merge = acceptance); /coordinate SKILL.md diff applied (#115, envelope 023) | #108, #115 |
| Phase-1 review findings, Critical/High fix batch (C1 guardrail wiring [RULED FIX-NOW 2026-06-12]; H2–H4; F1 teardown state surface; F2 plan blind dimensions) | C1 ruling ✓; review on record (docs/reviews/phase-1-review.md) | phase-close gate: Critical = no waiver; Highs = fix or written acceptance | loom-author | done — merged 2026-06-12 (C1 doctor-wiring+githooks, H2 fail-closed audit, H3/H4 guard-bash+deny floor, F1 clean-state real, F2 plan dimensions); C1 fix verified live by e2e build-under-session same day (.scratch/live-build-experiment.md) | #92 |
| live-build e2e findings F-a..F-d (build summary≠writes; home-sync audit gap; build.log overwrite; deliberate lock refresh w/ digest provenance) | — (e2e record: .scratch/live-build-experiment.md, promote to docs/e2e/ pending) | R1 audit-delta family (T26); build observability | loom-author | done — merged 2026-06-12 (F-a/F-b/F-c; F-d held for host digest check) | #111 |
| 036 harness-state durability: playbook materializes trust/opt-in flags at build ("2-plus", human-decided 2026-06-12; `.claude.json` sibling file on overlay dies on recreate — BOTH seats, advisor-verified; declare-or-rederive per devcontainer/Codespaces precedent, ADR-0014/0015 aligned) | — | Writer durable opt-in; autopilot survives recreate; prerequisite for ADR-0017 target state | loom-author | done — merged 2026-06-12 (#113; harness.<agent>.trust -> $HOME/.<agent>.json; ADR-0018 + SPEC#harness clause, human-merge = acceptance; FR-BUILD-014) | #113 |
| settings-files write protection (029.B): extend never-auto floor + protect-paths to `.claude/settings*.json` + `.claude/hooks/**` (deny Edit/Write + pre-commit guard) | — | T28/C1-family: trust config can't be agent-rewritten; closes the in-container judgment-only gap | loom-author (harness-deny half: human) | done — both halves merged 2026-06-12: #118 git-side (protect-paths trust class + ALLOW_TRUST_CHANGE audited override, guard suite + FR-GUARD-TRUST-CONFIG, gate scrub, TEAM never-auto floor) + #120 harness-layer deny on trust config (029.C, human-instructed) | #118, #120 |
| git-discipline transcription (028: end-on-main, currency contract, hand-off per-branch, resolver-as-tool script; 2 specimens) [TEAM clause frozen — human admin-merge] | — | shared-tree git discipline mechanized; human-blessed 2026-06-12 | loom-author | done — merged 2026-06-12 (#116; TEAM.md clause [4 rules + 2 specimens] + /coordinate currency step + scripts/resolve-plan-union.go; frozen-contract, human admin-merge = acceptance) | #116 |
| Writer SessionStart checkpoint hook (user-level /root/.claude, volume-backed; T28-A triad: own HALT sentinel + data-framing + 60-line cap) | — | cold-start continuity = mechanism on Writer seat (loss class: docs/e2e/cold-start-continuity.md) | loom-author | done — merged 2026-06-12 (#119; declared via playbook → build materializes to /root/.claude/*; FR-BUILD-009; T28-A triad tested). ACCEPTANCE PROVEN 2026-06-13: this cold-start session was injected with the checkpoint at SessionStart | #119 |
| T15 successor: AI-first credential acquisition (incl. VCS/gh) | trial verdict | autonomy (T18 push gap) | loom-author | queued — decided C′→D 2026-06-11, envelope item 008 (self-promotes at verdict) | — |
| T10 non-root user / parameterized $HOME | — | hardening; one of three full-auto re-eval gates | loom-author | in review — PR 1 #122 + PR 2 #130 MERGED 2026-06-13 (design, clause, schema/merge/validate, ContainerSpec.User/Home plumbing, ADR-0019, FR-SCHEMA-009). Writer caught an ADR-0019 contradiction (decisions 2 vs 4) building PR 3 → advisor ruled MODEL A, decision 2 amended (#131): container runs as root, entry verbs `exec -u <user>` BY NAME. PR 3 MERGED (#132): run-as-user engine behavior — collision-tolerant useradd, home retarget to /home/<user>, scoped chown (prunes ro creds bind), entry-verb `-u`, FR-EXEC-004 + FR-BUILD-015 (gate + fr-verify green; docker path integration-validated on CI). PR 4 role-marker + doctor `container:user` = **DEFERRED 2026-06-14** (optional polish; the autonomy loop verified on the current root fallback). ⏰ **REMINDER — apply WHEN loom-dev goes non-root** (a `user:` is set in loom.yml): going non-root WITHOUT PR 4 SILENTLY BREAKS the drain role-guard (`id -un==root` stops resolving → drain no-ops → autonomous delivery stops). Baseline patch + 3-part order (engine marker → recreate → human trust-diff swap; RE-DIFF first) at `docs/patches/0022-drain-guard-role-marker.md`. Bridge: `LOOM_SESSION_ROLE=loom-author` | #122, #130, #132 |
| one-week defaultMode "auto" trial (classifier mode; deny floor + never-auto floor unchanged) | exit/rollback package T22 ✓ (docs/auto-trial.md) | prompt volume ↓ | **human flipped 2026-06-11; revert pre-authorized, re-flip human-only** | RUNNING — flipped #70, clock started 2026-06-11; day-7 verdict 2026-06-18: keep iff zero S1+S2 | #70 |
| T22 transcription: auto-trial exit/rollback package (thread stub + docs/auto-trial.md; inbox item 001) | — | trial failure contract on record | loom-author | done | #56 |
| T23 transcription: AUTOPILOT scoping (role × project, HALT kill-switch + test, flips.log; inbox item 003) | T21 ✓, LL-011 fix ✓ (#54) | transport trust model on record; atomic both-roles rollback | loom-author | done | #57 |
| baseline scan, Writer side (would-prompt metric; inbox item 004 — flip gate) | advisor scan ✓ (#58) | auto-trial §1 precondition; the flip waits on this | loom-author | done | #60 |
| guided-run runbook (docs/guided-run.md) — orphan-PR backfill, /replan 2026-06-11 audit | — | Phase-1 close prep | loom-advisor (human's hands) | done | #62 |
| bootstrap-entry spec clause (loom-bootstrap.sh first touch — no clause covers it; C3) | human decision 2026-06-11 ✓ (inbox item 007) | FR-BUILD-008 remainder | loom-author transcribed — human admin-merge | done — accepted; FR extraction = inbox item 010 | #64 |
| spec-map v1 + r2 (docs/spec-map.md, threads shadow on SPECs/FRs) — orphan backfill, /replan audit | — | spec/thread shape visible | loom-advisor (human's hands) | done | #67, #71 |
| spec-map r3 (post drain-proof: yellow band cleared, guards 4 FR, T28→lockfile edge, T10 authorized) | — | spec/thread shape current at Phase-2 open | loom-advisor | done | #124 |
| /achievements format amendment (lifecycle dashboard) — orphan backfill, /replan audit | — | validated report format | loom-advisor (human's hands) | done | #65 |
| FR extraction: entry:bootstrap → FR-ENTRY-001..004 + hermetic sh-fixture tests (inbox item 010; spec-map node → green) | clause accepted ✓ (#64) | registry debt closed; spec→FR→test joint | loom-author | done | #74 |
| /specmap project skill (regenerates docs/spec-map.md; writes via PR — inbox item 011; row may dup docs/specmap-row) | spec-map v1 ✓ (#67) | spec→FR→thread shape on demand | loom-author | done | #80 |
| guided-run findings batch (⑦ plan/build convergence disagreement +LL +regression; ⑧ doctor probe scope; ⑨ teardown unconfirmed; ①–⑥ doc/ergonomics gaps) | guided run ✓ (docs/guided-run.md results) | criterion-1 evidence → fixes; stranger-path ergonomics | loom-author | done — ⑦ #79, ⑧ #82, ⑨ #83, ①–⑥ #84 (item 013) | #79, #82–#84 |
| Phase-1 close (PLAN edit) | guided run ✓ MET 2026-06-11 (docs/guided-run.md verdict); review gate 2026-06-12 (docs/reviews/phase-1-review.md) | phase boundary | **human — no agent self-approves phase completion** | done — **PHASE 1 CLOSED 2026-06-12**, human squash-merged the close edit (evidence block in the Phase-1 roadmap section); Phase 2 active | #99 |
| T12 closeout: delete archived devenv | archive + 14d | cutover bookkeeping | human | scheduled 2026-06-24 | — |
| re-run auto-mode evaluation (full-auto clearance) | T16 hooks landed ✓ (#107), T10 non-root, T20 decided | autonomy | loom-advisor | blocked — event-driven (no calendar): T16 dep satisfied 2026-06-12; still awaiting T10 non-root + T20. When all three flip, checklist-diff vs the recorded evaluation (allowlist ✓, deny-floor ✓, code-exec egress, root, guard hooks) | — |
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
