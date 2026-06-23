# OPEN-REQUIREMENTS — the genuine unfinished set

> Consolidated 2026-06-23 from the 2026-W26 weekly report's three research
> passes (SPEC-vs-FR coverage · the 6 Proposed ADRs · the open-board sweep).
> This is the **single view of what is genuinely unfinished** — open SPEC
> surface, ADRs awaiting closure, live decisions, active design threads, and
> the parked backlog — each with a way forward. It is a NAVIGATION doc, not a
> frozen contract: the queue (`docs/PLAN.md`) and the backlog
> (`docs/BACKLOG.md`) remain the systems of record; this file points at them.
>
> Re-derive, don't trust-on-faith: every row cites its source. When a row here
> and its source disagree, the source wins (regenerate this file).

---

## 1. Open SPEC surface — 2 verbs (the only non-green SPEC nodes)

All 93 active FRs are tested, but FRs only exist for *built* behavior. Two
specced verbs have **zero FRs and zero code** (not even a stub option struct) —
the only RED nodes in `docs/spec-map.md:34-35`.

| Verb | SPEC | What it does | Status | Phase (was → now) |
|---|---|---|---|---|
| **`update`** | SPEC-verbs.md:87 | Reconcile a running env to the edited playbook — apply only the delta; real removal (a dropped tool is uninstalled); re-resolve + rewrite `loom.lock`. `plan.go:175` already defers tool-removal to it. | declared-but-unbuilt | Phase 2 → **pulled to near-term (human 2026-06-23)** |
| **`export`** | SPEC-verbs.md:282 | Emit a `devcontainer.json` from the env fields for VS Code/Codespaces hand-off. Lossy (policy/hooks/AI-context don't map). | declared-but-unbuilt, "later, lossy" | Phase 4 → **pulled to near-term (human 2026-06-23)** |

**Way forward (both are now near-term per the 2026-06-23 scope call):**
1. **`update` first** (higher value — it's the desired-state reconcile verb the
   engine already gestures at). Path: SPEC clause is already written → extract
   **FR-UPDATE-\*** → build `internal/cli/update.go` + `engine.Update` (plan-then-
   apply-delta, real `Remove`) → must satisfy FR-INV-\* (`--json`/idempotent/
   audited). Closes review-finding **R6** (the `noop` enum becomes reachable).
2. **`export` second** (lower value, lossy). Path: tighten the SPEC clause (it's
   thin) → **FR-EXPORT-\*** → deterministic emitter + the import-enrich skill for
   residual judgment. Completes the devcontainer round-trip (import ↔ export).
3. No new ADR needed — ADR-0002 (lockfile) and ADR-0003 (devcontainer) already cover them.

> ⚠️ Phase note: Phase 1 is formally CLOSED (`PLAN.md:115`). "Include in phase-1"
> is realized as **front-of-queue near-term rows**, not a reopening of the closed
> phase. See the new queue rows + the §6 roadmap.

---

## 2. ADRs awaiting closure — 6 Proposed (5 are bookkeeping, 1 is real)

"Acceptance = PR merge" (RULES §5/C3), so for most the merge already happened and
only the header is stale.

| ADR | Proposes | Real status | FRs | Way forward |
|---|---|---|---|---|
| **0016** entry verbs | exec/shell, no `--json` | ✅ shipped #42 | EXEC-001..004, SHELL-001 | **header flip → Accepted** (done in this PR) |
| **0018** trust flags as config | materialize at build | ✅ shipped #113 | BUILD-014, SCHEMA-008 | **header flip → Accepted** (this PR) |
| **0020** PARK/pull-next/re-surface | autonomy closed-loop | ✅ ADR #136 + scripts | GUARD-RESURFACE, LOOP-001 | **header flip → Accepted** (this PR); residual drain-hook wiring #137 is human-trust, orthogonal |
| **0022** autonomy substrate | self-wake + backlog refill | ✅ #149 + slices 1–5, self-wake verified | LOOP-002/003/004, GUARD-SPAWN-RATE/REAP | **header flip → Accepted** (this PR); host-hardening (§3) is operational, not acceptance |
| **0019** non-root user | root container, `-u` verbs, role marker | 🟡 engine merged; trust tails open | SCHEMA-009/010, BUILD-015/016, EXEC-004 | header NOT flipped — needs: drain-guard swap (Part 2, human trust) + non-root topology flip (Part 3) + **§5 spec-drift amendment** (still states the superseded "non-root+empty=error" rule) |
| **0017** Writer remote-trust | Writer pushes branches; merge-to-main gated | 🟠 policy live, headline UNREALIZED — Writer still advisor-only push | none (no FR-PUSH — behavior unshipped) | blocked on the **036 credential clear** + a human outward-widening settings PR; tiered-merge amendment drafted (adv-068, frozen-path) |

Also: `docs/decisions/README.md` index is missing **0023/0025** and skips **0024**
(unused number) — corrected in this PR.

---

## 3. Live decisions — need a ruling or a /confer (3 + 3)

**Teed for /confer (opened 2026-06-23):**
| Decision | Tension | Lean |
|---|---|---|
| **Append-only vs STRICT** (slice-5 host) | STRICT's `[ -w ]` fail-closes on `chattr +a` append-only ledgers, but spawn-guard must append grants | refine STRICT to test *rewritability*, not the write-bit |
| **Settings source-path protection hole** (draft 044) | `config/hooks/**` + `config/dotfiles/*` are unprotected source paths for trust-bearing files | fold into ALLOW_TRUST_CHANGE protect-paths (option a) |
| **ADR-0019 §5 spec-drift** | SPEC `#role` + ADR §5 still state the superseded "non-root+empty=error" rule (RULES §2 code/spec disagreement) | amend §5 to the role-alone rule the engine already ships |

**Other open decisions (human disposes, not yet teed):**
| Decision | Location | State |
|---|---|---|
| **ADR-0023** shared-tree edit-guard | docs/patches/0023-shared-tree-edit-guard.md | drafted + LL-015; human accept → then hook+FR |
| **ADR-0017 tiered-merge amendment** | .scratch (adv-068) | drafted; frozen-path, needs ALLOW_SPEC_CHANGE |
| **Context-management for agents** | CTX-MGMT-DRAFT-2026-06-16 (inbox, pinned) | promote to a T-thread at next `/coordinate verdict` |

---

## 4. Active design threads (🟡 genuinely open in OPEN-THREADS)

| Thread | What's open | Way forward |
|---|---|---|
| **T18** multi-agent dogfood perms | Writer-can't-push + permission pre-allowlist friction | converges with 0017/036 — resolves when Writer push lands |
| **T27** AUTOPILOT control + observability | OVERRIDE keywords (in-band) + decision-trace/orphan/no-progress tiers | needs its own ADR before code |
| **T28** harness self-defense | attack taxonomy A–H; wants a **guardrail-drift detector** (the new mechanism this thread most wants) | scope the drift-detector as a guard slice |
| **T32** loom-supervising-box | always-on cron/monitor home for recurring work | deferred behind cold-floor + T34; revisit after the autopull trial verdict |
| **T34** advisor-in-loom | executing cutover; next = devenv quarantine (manual) + Slice D (in-loom spawn + reaper) | proceed slice-by-slice; devenv quarantine after a confidence session |

---

## 5. Parked backlog (`docs/BACKLOG.md`, review-by 2026-07-12)

Durable-by-intent; one line each; dropped by lazy consent at the weekly pass if
unclaimed. **Soft cap ~25; currently 24.** Not "open work" — parked on purpose.

**Phase-1 review findings (R1–R10, all Medium except R10 Low):**
- **R1** audit fail-open + tamperable · **R2** unpinned provisioning (curl-pipe-sh, checksumless Go tarball as root) · **R3** drain prompt-injection surface · **R4** inert declared flags (`build --stack/--overlay`; note `--emit-playbook`/`--migrate` now shipped — R4 is partly stale) · **R5** FR↔spec joint nearly unfailable (substring anchor) · **R6** build `noop` enum unreachable (closed by `update`) · **R7** engine-level teardown gate (cobra-only consent) · **R8** doctor outside conformance nets · **R9** stack knowledge hardcoded in Go switches (the **Phase-2 second-stack seam**) · **R10** flips.log + snapshot hygiene.

**Judgment-trial candidates (C1–C4, sequencing human 2026-06-19):**
- **C1** drain best-fit pick · **C2** stale-TAKEN reclaim · **C3** drain budget interior · **C4** session-snapshot content.

**Research / Phase-2+ (P-items):**
- **P1** token efficiency · **P2** memory measure (run 1 = 5.5/6; program open) · **P3** multi-loom fail-fast · **P4** context-window inspector · **P5** security self-audit · **P6** exhaust-before-prompt · **P9** windows early deploy · **P10** mobile p2p (charter-scope question first) · **P11** host security map.

**Way forward:** the **2026-07-12 weekly pass** re-blesses or drops each by lazy
consent. Promote-now candidates worth pulling early: **R9** (unblocks Phase-2's
second stack), **R1/R2/R3** (security hardening — feed T28), **R5** (strengthens
the FR gate). The rest stay parked.

---

## 6. Consolidated roadmap / ways forward

**Near-term (pulled forward, human 2026-06-23):**
1. `update` verb — SPEC clause (exists) → FR-UPDATE-\* → build (closes R6).
2. `export` verb — SPEC tighten → FR-EXPORT-\* → deterministic emitter.
3. ADR bookkeeping — flip 0016/0018/0020/0022 → Accepted (this PR).
4. /confer the 3 live decisions (§3) → rulings → apply.

**Phase 2 (active) remainder, beyond the two verbs:**
- **Second stack** (Python or TS) to prove container-per-project side-by-side —
  gated on **R9** (externalize sourcePolicy/goModule/provisionScript/containerHome
  from Go switches into the config tree).
- **Prebuilt base image (ADR-0012)** — bake toolchain into a digest-pinned base;
  needs the build/publish/scan pipeline (the durable fix for the #75 OOM class).
- Credential continuity + `start` menu — largely shipped (RUN-005..008, RUN-001..004).

**Phase 3+ (unchanged):** full `--json`/doctor hardening · playbook guards ·
**agent-debug-and-fix** (needs its own ADR) · cloud sandbox sibling (ADR-0007).

**Open-decision closure order:** ADR-0019 §5 drift (RULES §2 violation — highest) →
append-vs-STRICT (unblocks slice-5 host) → settings source-path hole (T28 surface).
