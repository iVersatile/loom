# ADR-0022 — Autonomy substrate: ephemeral worker + backlog-ready pull
**Date:** 2026-06-14   **Status:** Accepted (header reconciled 2026-06-23 — acceptance-by-merge already on record, #149 + slices 1–5; advisor-drafted from in-session human-confirmed decisions 2026-06-14: "yes, auto-pull next ready FR"; "ephemeral-worker confirmed". **Red-teamed 2026-06-14 → BLOCK with 5 binding amendments; all folded in below** [the original "escalates NOTHING" framing was disproven — see Decision 4]. Acceptance = PR merge, per RULES §5 / C3. Extends ADR-0020.)

## Context
ADR-0020 gave the autonomy closed-loop (PARK → pull-next → re-surface), but
dogfeeding exposed that it does not actually keep the Writer working. Two
structural gaps, traced 2026-06-14 from the *writer-idle* meta-cause (the Writer
idles → the human pours focus into advisor design-thinking → new tasks/mechanisms
spawn → priorities entangle — the human becomes the thing that un-sticks it):

1. **The loop is Stop-triggered.** The drain (`.claude/hooks/drain-inbox.sh`) runs
   only when the agent finishes a turn and returns `{decision:"block"}` to
   continue. An idle, fully-stopped session produces no Stop, so nothing re-fires
   the loop when work arrives later. Nothing inside the harness's hook system can
   wake a truly-idle session — a turn must come from **outside** (T27).
2. **The loop closes over the INBOX, not the PLAN/FR backlog.** The drain delivers
   items from `.scratch/inbox/loom-author.md`; it reads `docs/PLAN.md` only to
   *validate* (`serves:` = orphan-guard). Nothing promotes the next backlog row
   INTO the inbox — that refill is a deliberate human/coordinator triage gate. So
   even a perfect wake, on a dry inbox, finds nothing deliverable and re-idles.
   "Why doesn't it self-redirect to the next FR?" — no link reaches the backlog
   (T30).

Resolving #1 first stalled on a false dilemma (T27, 2026-06-13): host-side
`send-keys` is **blind** (injects into whatever the pane shows; no idle
introspection), and headless `claude -p` is **claude-specific** (loom is
harness-agnostic — it also runs gemini). This ADR records the zoom-out that
dissolves the dilemma and unifies #1 + #2 into one autonomy decision.

## Decision
1. **The unit of autonomy is an EPHEMERAL WORKER loom SPAWNS — not a warm session
   injected into.** A wake = loom launches a fresh worker that rehydrates from
   durable state (`config/hooks/checkpoint-inject` already does this), runs the
   guarded drain, acts, persists, and exits. The warm interactive session is
   reframed as **the human's seat** (design-thinking), never the robot's worker —
   removing the impedance mismatch that produced blind-injection. This matches
   `ai-user-topology` ("state persisted outside the container, rehydrated on
   bring-up") and reuses what is ~80% built: checkpoint-inject (rehydration) +
   the inbox/PLAN (durable state).
2. **The wake is two layers — a harness-NEUTRAL loop over a per-harness ACTUATOR.**
   - *Loop* (loom-owned, harness-agnostic): "is there ready work? for which
     session? fire a tick" — pure logic over durable artifacts. **The loop checks
     HALT BEFORE it spawns (amendment 4):** if `.scratch/inbox/HALT` exists, the
     loop spawns NOTHING — fail-safe, symmetric to and ahead of the drain's own
     in-worker HALT check (`drain-inbox.sh:28`). A HALTed system is quiet: it must
     not even pay spawn/rehydrate cost. HALT-before-spawn is a required guard test,
     not merely `¬HALT` on the pull.
   - *Actuator* (per-harness adapter verb, `wake(session)`/spawn): "deliver one
     tick to session X of harness H," implemented in the same adapter layer that
     already knows how to *launch* each harness (`loom.lock` `agents:`
     claude-code/gemini). Harness-specificity is quarantined there; `send-keys`
     and `-p` are candidate adapter bodies, never the design. **No harness-specific
     logic may live in the loop.**
   - **Constant-payload invariant — carried forward from T27, NON-NEGOTIABLE
     (amendment 3):** the actuator's delivered keystroke/payload is a HARDCODED
     CONSTANT (e.g. `drain` / `continue`). The ONLY request-derived input is WHICH
     session to wake; no request content — row id, "reason", body — may reach the
     keystroke/argv. If the payload could carry text, the actuator becomes
     arbitrary-command-injection into a trusted auto-mode worker (the exact channel
     T27 §911-918 forbids). This holds for every actuator body, `send-keys`
     included.
3. **Backlog-ready pull — extend the loop from inbox-scoped to backlog-scoped.**
   This is an **autonomy ESCALATION, not transport** (amendment 5): choosing which
   backlog row runs next is a prioritization JUDGMENT the human triage gate held —
   the ADR names it as such rather than reclassifying it as plumbing. It is gated
   by the **readiness predicate** AND a self-selection floor. A row is auto-pullable
   iff:
   (a) **deps cleared — by EXTERNAL TRUTH only (amendment 2).** Dependency merge
   status resolves via `gh pr view <n> --json state,mergedAt` or a CI/host-written
   merged-refs file the agent CANNOT write — **never** the drain's commit-subject
   grep cache (`drain-inbox.sh:45`), which is agent-forgeable: the Writer controls
   commit subjects, so a one-line `fix: … (#150)` would otherwise mark a
   human-BLOCKED PR as merged and auto-execute the gated row. Forbid the
   subject-grep cache for self-selected work (consistent with ADR-0020 R7:
   external-truth over agent-writable for security-sensitive deps).
   (b) **execution-ready, not design-first** — not a design-stub / thread-incubator
   row (PLAN order ≠ execution order). The detector must NOT rely on the drain's
   `/thread stub/` substring (`drain-inbox.sh:76`, trivially evaded); use a
   structured row-class field.
   (c) **no priority inversion** (highest-priority *ready* row; reuse `pull-next` R5).
   (d) **not superseded / in-flight** (couples to T27 facet-B supersede-revalidation).
   (e) **within the never-auto floor** (selects WORK, never escalates PERMISSION) —
   AND a **self-selection floor (amendment 5):** the FIRST auto-pull of any
   never-before-triaged row requires a human-confirm tier, OR the row must belong
   to a pre-declared auto-pullable row-class allow-list. The never-auto floor
   governs *permission*; this floor governs *which work self-selects* — a distinct
   gate the predicate alone does not provide.
   Lean: a **separate `promote-next` step that mints a real inbox envelope** (audit
   trail + supersede-aware), keeping the drain's existing inbox-delivery contract
   and decision-trace intact — not a deliver-direct path. The minted envelope's
   `serves:` must match a PLAN row by **full-key equality**, not the orphan-guard's
   substring test (`drain-inbox.sh:62`, review M7 weakness) — a human who would have
   caught a sloppy `serves` is no longer in the loop.
4. **The PERMISSION floor is unchanged; but spawn + auto-pull DO add two new
   bounded axes that the loop must own (corrected after red-team — the original
   "escalate NOTHING" was overstated).** A spawned worker runs the **existing
   guarded drain** (HALT, AUTOPILOT gate, orphan-guard, never-auto floor —
   ADR-0017/0020, T23 carry over), and the pull never widens authority, exfiltrates,
   or escalates permission. **What it DOES newly add, and the bound for each:**
   - **Cadence/throughput (amendment 1).** The drain's 3-per-cycle budget
     (`drain-inbox.sh:36-42`) is gated on `stop_hook_active` — true ONLY within one
     process's Stop-chain. A freshly *spawned* worker starts at `count=0`, so the
     cap is **intra-process and does NOT bound work across spawns.** The human seat
     rate-limited by hand; the spawner removes that. Therefore the **loop must own a
     durable cross-spawn rate bound**: a spawn-ledger (e.g. `.scratch/inbox/
     .spawn-log`, timestamped) checked before each spawn, with a max-spawns-per-
     window + bounded backoff (spirit of T27 "one request = one wake, bounded
     backoff"). `.drain-count` is explicitly NOT this bound.
   - **Self-selected work (Decision 3e floor).** Removing the human triage eyeball
     is bounded by the readiness predicate (external-truth deps) + the
     self-selection floor, not by "it's the same drain."
   - **Role non-forgeability is NOT yet true (S3 caveat).** The live role guard
     (`drain-inbox.sh:23-25`) still uses the `id -un==root ⇒ loom-author` fallback;
     ADR-0019 PR 4 (marker-file resolution) is DEFERRED. So **today any root-running
     spawned process drains loom-author's inbox** — "cannot forge a role" is
     aspirational until PR 4 lands. The spawn substrate's build therefore **depends
     on ADR-0019 PR 4** and must not ship the spawner before it.
   The design test (AGENTS.md) now passes only WITH amendments 1–2: without the
   cross-spawn bound and external-truth deps, "would the guardrails hold if you
   tried the worst thing?" FAILS — a one-line commit subject auto-executes
   human-blocked work, and a tight spawn loop drains unboundedly.

## Alternatives considered
- **Warm-session `send-keys` wake as the substrate** — rejected: blind injection
  into live pane state, harness-specific TTY driving, conflates the human seat
  with the worker. Retained only as an *optional human-convenience poke* for the
  warm seat, never load-bearing for autonomy (T27 mechanism A, demoted).
- **Keep the human as the inbox-refill valve** — rejected as the *default*, but
  NOT reclassified as transport (amendment 5 / red-team S2). Choosing next work is
  a judgment; auto-pull is an autonomy escalation, bounded by the readiness
  predicate + the self-selection floor (Decision 3e), with the human retained on
  the first-pull confirm tier / row-class allow-list rather than every refill.
- **Hardcoded `claude -p`** — rejected: not harness-agnostic; generalized to the
  per-harness launch/actuator adapter.
- **`/loop` in-band self-poll** — rejected as primary: polling cost + up-to-N-min
  latency + not dependable immediate intervention (T27 mechanism C).
- **Deliver-direct from backlog (no inbox envelope)** — rejected as lean: loses
  the audit trail and supersede-revalidation the inbox provides; the promoter
  mints a real envelope instead.

## Consequences (positive / trade-offs / revisit-if)
- **Positive:** writer-idle dies (the loop self-refills *and* self-wakes);
  harness-agnostic by construction (gemini and future harnesses via the adapter);
  reuses checkpoint-inject + durable inbox/PLAN; advances toward the
  `ai-user-topology` north star; the human moves off *transport* and the routine
  refill, retaining the gates + the self-selection confirm tier.
- **Trade-offs:** (1) cold-start cost per worker — mitigated: checkpoint-inject
  rehydrates, and a builder executing a *specified* FR does not need warm chat
  context (revisit if some work class proves too lossy). (2) A new control surface
  (the spawner drives a trusted session) — bounded by Decision 4's named axes
  (cross-spawn rate ledger + constant-payload actuator + HALT-before-spawn), NOT by
  "it's the same drain"; the spawner/actuator + promote-next are **trust-path,
  human-applied** diffs. (3) The readiness predicate encodes judgment — it must be
  **conservative: fail to NOT-pull** rather than grind on wrong/blocked work.
  (4) **Rehydration-poisoning is amplified, not contained (red-team S2):**
  `checkpoint-inject:49` injects a `tail` of the agent-writable, NOT-protect-path-
  guarded inbox into every fresh worker. Removing the per-turn human eyeball means
  one poisoned turn boots EVERY subsequent spawn into that context. Mitigation:
  spawned-worker rehydration + the readiness/deps logic must draw only from
  **structured, parser-validated** inbox-envelope headers + external truth, never
  from raw free-text `tail` that could carry trailing instructions; the 60-line cap
  + data-not-instructions framing were sized for occasional supervised cold starts,
  not a continuous unsupervised spawn loop.
- **Revisit-if:** cold-start is too lossy for a work class (→ warm-session for
  that class only); a harness ships no headless/launch path (→ per-harness
  fallback, possibly the demoted send-keys poke); the readiness predicate proves
  too eager (→ tighten, add a human-confirm tier).
- **Slicing (T30 + T27), amended order:** (1) offline readiness-runner
  (`resurface-decide`-style, non-agent; "ready backlog row?" via **external-truth
  deps**, not the subject-grep cache); (2) `promote-next` (mints the envelope,
  full-key `serves`, structured row-class check); (3) the cross-spawn rate
  ledger + HALT-before-spawn in the loop; (4) the spawner/actuator (per-harness
  wake verb, constant payload) — **gated on ADR-0019 PR 4 landing** (role
  non-forgeability); (5) FR + guard test (FR-LOOP-00x: pull iff predicate ∧
  external-truth-deps ∧ ¬HALT ∧ AUTOPILOT ∧ self-selection-floor; loop HALT-checks
  before spawn; spawn rate-bounded; actuator payload is constant; never mid-turn).
  Trust-path bits human-applied.
- **Scope boundary:** orthogonal to ADR-0021 (role resolution / multi-agent),
  which is gated on the git-credential blocker — this substrate works for today's
  single root loom-dev author. **Build dependency:** the spawner (slice 4) must not
  ship before ADR-0019 PR 4 (marker-file role resolution), else any root spawn
  drains as loom-author. Extends ADR-0020 (inbox-scoped → backlog-scoped + the
  wake/spawn actuator); does **not** touch the never-auto *permission* floor (T23).

## Links
- ADR-0020 (the closed-loop this extends) · ADR-0017 (writer-trust floor) ·
  ADR-0019 PR 4 (role-marker — build dependency for the spawner) ·
  T27 (wake primitive + facet-A ephemeral-worker resolution) · T30 (backlog→inbox
  readiness predicate) · T21 (transport correctness) · T23 (AUTOPILOT scoping).
- Red-team 2026-06-14 (adversarial-reviewer): BLOCK → 5 binding amendments folded
  in (cross-spawn rate bound · external-truth deps · constant-payload actuator ·
  HALT-before-spawn · self-selection floor) + S3 notes (full-key `serves`, PR 4
  role dependency). The "escalates NOTHING" claim was disproven and corrected.
- docs/TOPOLOGY.md `ai-user-topology` · `config/hooks/checkpoint-inject`
  (rehydration) · `.claude/hooks/drain-inbox.sh` + `config/hooks/{pull-next,
  resurface-decide}` · docs/PLAN.md "agent-initiated lifecycle / task continuity".

## Amendment (2026-06-16) — REFILL ≠ WAKE: the wake actuator is self-schedule, not external-spawn

**Context.** This ADR framed the substrate as "ephemeral worker + backlog pull" and
built the wake actuator as an EXTERNAL SPAWN (a fresh `claude -p` worker). A spike
(adv-071) plus the author's in-container analysis (advisor-inbox 073), reached via a
deliberate zoom-out, showed the design **conflated two separable concerns**:
- **REFILL** (backlog → inbox) — readiness-decide / promote-next / spawn-guard /
  spawn-loop (slices 1–4). Genuinely useful; **retained unchanged.**
- **WAKE** (resume a stopped session) — solved here via external spawn, which hit two
  hard walls: a credentials/auth wall AND the harness permission classifier refusing
  an agent to self-bootstrap another autonomous agent. Those walls are the
  "would the guardrails hold if you tried the worst thing" test **passing** — an agent
  correctly cannot spawn a new agent.

**Decision — the wake actuator is SELF-SCHEDULE, not external-spawn.**
- At a Stop with no eligible work, the agent arms `ScheduleWakeup(delay, "re-check
  inbox")` — a **first-party harness primitive** (schedule one's own resume), NOT a
  nested agent. It has none of the spawn walls: no new creds, no classifier-bootstrap,
  no host-side runner. It is how the advisor's own `/loop` runs.
- **The refill substrate (slices 1–4) is retained unchanged.** The spawn-loop's verdict
  logic (HALT-first → readiness → promote-next → rate) stays; **only the ALLOW branch
  changes from `SPAWN`(new agent) to `WAKE`(self-schedule).**
- **Ephemeral release is dropped for the wake half.** The trade is one continuous
  (harness-summarized) session vs. N fresh sessions paying cold-start every cycle and
  hitting the spawn walls; for SERIAL wake (the stated goal) summarization gives the
  context-economy benefit without the cost. Ephemeral earns its keep only for TRUE
  PARALLELISM (concurrent workers), which is out of scope here.
- **Bounds (safety):** HALT-first still gates (no self-wake past a HALT); the self-poll
  is bounded — cadence clamp (long idle ticks, `ScheduleWakeup` cache economics),
  give-up-after-K-empty-ticks, human-interruptible — so it cannot spin.
- **Closes** the open wake-primitive gap (T27 facet A / #129 — "nothing re-wakes a
  fully-idle session").

**Supersedes** the external-spawn actuator (the spawner-as-launcher). The `loom wake`
verb, a host-side runner, and a `Bash(claude -p:*)` allow-rule are **dropped** — the
last would defeat the classifier safety the spike surfaced (adv-071).

**Authorship chain:** human-decided (zoom-out 2026-06-16; author's in-box analysis
adopted) → agent-transcribed (this amendment) → human-accepted (merge = acceptance,
ALLOW_SPEC_CHANGE, per RULES §5/C3).
