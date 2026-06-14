# ADR-0020 — Autonomy closed-loop: PARK / pull-next / re-surface

**Date:** 2026-06-14   **Status:** Proposed (authorship chain: **human-decided to
build** — "tackle this earlier than later", 2026-06-14 — → **author↔advisor
converged** — rounds 1–3 with prototypes, `.scratch/autonomy-loop-experiment.md`,
risk ledger exhausted — → **agent-transcribed** — this ADR — → **human-accepted**;
acceptance = PR merge, per RULES §5 / C3)

## Context
A blocked Writer **halts**. The 2026-06-13 specimen: building T10 PR 3 it found
an ADR-0019 contradiction, declined the envelope, marked it BLOCKED, and stopped
— correct judgment, but the session then idled until a human relayed the
question and poked it back to life. The human became the **transport**.

Root cause: the AUTOPILOT drain is **Stop-triggered, single-pass,
dependency-blind, and cannot wake a session**. So a blocked agent's only safe
move is to halt. Per RULES §9 (Close → Smooth → Skillify), the fix is to **close
the circle first**: no block should halt-and-drop; every parked task has a
guaranteed resume path, even if slow and human-gated.

## Decision (Phase 1 — close the circle; poll-based, no wake primitive)
A blocked task is **parked, not halted**; the session pulls the next independent
work and is re-surfaced when its dependency clears — all decided from inbox
state, by polling each drain run.

1. **PARKED disposition.** A delivered task the agent cannot proceed on becomes
   `status: PARKED` with `parked-on: <predicate>` and `parked-at: <ts>` (durable
   in the inbox). The agent commits WIP to the task branch (nothing lost) and
   escalates the blocker to the counterpart's inbox.
2. **Predicate vocabulary — FIXED, fail-closed, NEVER eval'd.** `parked-on` and
   `superseded-by` are agent-written, so they are evaluated only by a fixed
   dispatch — `exists:<path>`, `pr-merged:<n>`, `item-status:<id>=<STATUS>` —
   that never passes the value to a shell (awk `getline` / file lookup /
   charset-validated compare). Unknown kind or malformed value = fail-closed
   (stay PARKED). *Proof: `proto/resurface-safe.sh` + `proto/injection-test.sh`
   — the `${IFS}` payload creates no `PWNED`.*
3. **Re-surface (POLL).** Each drain run re-evaluates every PARKED item's
   `parked-on`; cleared → flip to QUEUED. No wake primitive — the poll closes the
   circle. (Push is Phase 2, the wake primitive #129.)
4. **Supersede-skip.** `superseded-by: <id>` present → the item is skipped, never
   delivered (the staleness gap; specimen: `adv-049` superseded by draft 050).
5. **Pull-next.** On PARK, the agent pulls the next eligible item whose deps are
   clear, **skipping anything that depends on the parked one** (no priority
   inversion), bounded by the work-selection cascade + budget + the never-auto
   floor. *Proof: `proto/pullnext.sh`.*
6. **Max-park-age → ESCALATE, never drop.** A PARKED item with
   `age = now - parked-at > MAX` and dep still uncleared is ESCALATED (surfaced
   to advisor/human), **never auto-dropped** (lost work is worse than a stale
   park). Missing `parked-at` = fail-safe park. Hooks the pull-next
   "all-blocked → idle/escalate" seam. *Proof: `proto/parkage.sh`.*

### Trust + cost bounds
- **Resurface causes a bounded TURN, never an action** — the never-auto floor +
  guard envelope gate what runs, so a forged `item-status:DONE` buys a wasted
  turn, not an escalation (same doctrine as the wake primitive). **Predicates are
  ranked by trust:** external-truth (`pr-merged:`, `exists:` on a root-owned /
  CI-written marker) over the agent-writable `item-status:`; security-sensitive
  parks must use external-truth deps.
- **The drain stays O(local):** `pr-merged:` resolves against a local merged-refs
  cache refreshed once per drain (not per-item, not network); `exists:` /
  `item-status:` are local reads.

## Consequences
- The Writer no longer halts-and-relays: it parks, pulls next, and auto-resumes
  on a poll. The human drops out of **transport** and stays on **acceptance
  gates** only — the split this work exists to make.
- **Build slice:** the drain-hook changes are a **trust path** (`.claude/hooks/**`)
  → human-applied diff (R6); the pull-next work-selection + the inbox schema are
  the agent's. This ADR is design-of-record; the build follows it, not before.
- **§9 phases:** Phase 1 (this ADR) closes the circle, poll-based. Phase 2
  (smooth) adds the wake primitive (push, #129) per its own ADR + measures the
  loop under repetition. Phase 3 (skillify) turns the steps into `/park`,
  `/converge`, `/pull-next`, `/wake` skills with benchmarks, wiring the
  human-gate precondition to `/converge`'s postcondition.
- **Convergence evidence:** `.scratch/autonomy-loop-experiment.md` (rounds 1–3),
  prototypes `resurface-safe.sh` / `pullnext.sh` / `parkage.sh` /
  `injection-test.sh` — produced by the author↔advisor convergence the loop is
  itself meant to enable (dogfood).

## Links
- T27 (AUTOPILOT control + observability), T21 (transport correctness), the wake
  primitive (#129 design), the supersede-revalidation gap (#131).
- RULES §5 (convergence: exhaust risk before claiming/escalating), §9
  (Close → Smooth → Skillify).
- PLAN "agent-initiated lifecycle / task continuity" — the top-ranked queue row.
