# ADR-0022 — Autonomy substrate: ephemeral worker + backlog-ready pull
**Date:** 2026-06-14   **Status:** Proposed (advisor-drafted from in-session human-confirmed decisions 2026-06-14: "yes, auto-pull next ready FR"; "ephemeral-worker confirmed". Acceptance = PR merge, per RULES §5 / C3. Not yet red-teamed; extends ADR-0020.)

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
     session? fire a tick" — pure logic over durable artifacts.
   - *Actuator* (per-harness adapter verb, `wake(session)`/spawn): "deliver one
     tick to session X of harness H," implemented in the same adapter layer that
     already knows how to *launch* each harness (`loom.lock` `agents:`
     claude-code/gemini). Harness-specificity is quarantined there; `send-keys`
     and `-p` are candidate adapter bodies, never the design. **No harness-specific
     logic may live in the loop.**
3. **Backlog-ready pull — extend the loop from inbox-scoped to backlog-scoped.**
   When the inbox is clear, a promoter auto-pulls the next **ready** PLAN/FR row.
   A row is auto-pullable iff the **readiness predicate** holds:
   (a) **deps cleared** (every named dependency merged/closed — reuse the drain's
   merged-PR cache, ADR-0020 R8); (b) **execution-ready, not design-first** (not a
   design-stub / thread-incubator row — PLAN order ≠ execution order); (c) **no
   priority inversion** (highest-priority *ready* row; reuse `pull-next` R5);
   (d) **not superseded / in-flight** (couples to T27 facet-B supersede-
   revalidation); (e) **within the never-auto floor** (selects WORK, never
   escalates PERMISSION). Lean: a **separate `promote-next` step that mints a real
   inbox envelope** (audit trail + supersede-aware), keeping the drain's existing
   inbox-delivery contract and decision-trace intact — not a deliver-direct path.
4. **The floor is unchanged — spawn and pull escalate NOTHING.** A spawned worker
   runs the **existing guarded drain** (HALT precedence, AUTOPILOT gate, orphan-
   guard, budget cap, never-auto floor — ADR-0017/0020, T23 all carry over). The
   pull only *selects* already-ready, already-authorized backlog work. The design
   test (AGENTS.md): a compromised loop can at most run already-bounded machinery
   on already-ready work — it cannot widen authority, forge a role, or exfiltrate.

## Alternatives considered
- **Warm-session `send-keys` wake as the substrate** — rejected: blind injection
  into live pane state, harness-specific TTY driving, conflates the human seat
  with the worker. Retained only as an *optional human-convenience poke* for the
  warm seat, never load-bearing for autonomy (T27 mechanism A, demoted).
- **Keep the human as the inbox-refill valve** — rejected: it is the meta-cause of
  idle and task proliferation. RULES §5 keeps the human on **gates**, not
  **transport**; auto-refill (bounded by the readiness predicate + never-auto
  floor) keeps that split.
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
  `ai-user-topology` north star; the human moves fully off transport onto gates.
- **Trade-offs:** (1) cold-start cost per worker — mitigated: checkpoint-inject
  rehydrates, and a builder executing a *specified* FR does not need warm chat
  context (revisit if some work class proves too lossy). (2) A new control surface
  (the spawner drives a trusted session) — bounded by decision 4 ("runs the
  existing guarded drain"); the spawner/actuator + promote-next are **trust-path,
  human-applied** diffs. (3) The readiness predicate encodes judgment — it must be
  **conservative: fail to NOT-pull** rather than grind on wrong/blocked work.
- **Revisit-if:** cold-start is too lossy for a work class (→ warm-session for
  that class only); a harness ships no headless/launch path (→ per-harness
  fallback, possibly the demoted send-keys poke); the readiness predicate proves
  too eager (→ tighten, add a human-confirm tier).
- **Slicing (T30 + T27):** (1) offline readiness-runner (`resurface-decide`-style,
  non-agent, "deliverable? + ready backlog row?"); (2) `promote-next` (mints the
  envelope); (3) the spawner/actuator (the per-harness wake verb); (4) FR +
  guard test (FR-LOOP-00x: pull iff predicate ∧ ¬HALT ∧ AUTOPILOT; spawn runs the
  guarded drain; never mid-turn). Trust-path bits human-applied.
- **Scope boundary:** orthogonal to ADR-0021 (role resolution / multi-agent),
  which is gated on the git-credential blocker — this substrate works for today's
  single root loom-dev author. Extends ADR-0020 (inbox-scoped → backlog-scoped +
  the wake/spawn actuator); does **not** touch the never-auto floor (T23).

## Links
- ADR-0020 (the closed-loop this extends) · ADR-0017 (writer-trust floor) ·
  T27 (wake primitive + facet-A ephemeral-worker resolution) · T30 (backlog→inbox
  readiness predicate) · T21 (transport correctness) · T23 (AUTOPILOT scoping).
- docs/TOPOLOGY.md `ai-user-topology` · `config/hooks/checkpoint-inject`
  (rehydration) · `.claude/hooks/drain-inbox.sh` + `config/hooks/{pull-next,
  resurface-decide}` · docs/PLAN.md "agent-initiated lifecycle / task continuity".
