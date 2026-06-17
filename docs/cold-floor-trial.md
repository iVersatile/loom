# Cold-start floor — auto-nudge trial (ADR-0022, option B)

> Status: **decided** (human, 2026-06-16, via advisor envelope adv-078) — this
> document is the C3 transcription of that decision; acceptance = PR merge. It is
> the exit/rollback + verdict contract for a **7-day trial** of option B
> (host-cron auto-nudges the idle Writer on cold + queued work). Not a frozen
> SPEC: it expires with the trial (keep / revert verdict recorded in the queue
> row, then this doc moves to `docs/threads/archive/`).

## 1. What this closes

Self-wake shape A (#172/#173) lets a **live** `/loop` re-arm itself, but a
**fully-stopped** session fires no Stop and runs no tick — nothing re-wakes it
when new queued work lands. The cold-start floor is the external pulse that
covers that residual (T27-A): an hourly host-cron asks `scripts/cold-check`
"is there queued work worth waking the seat for?" and, on `NUDGE`, wakes it.

## 2. Scope (load-bearing — the safety boundary)

- The empty-`LOOM_AUTOPULL_CLASSES` floor **still holds.** The nudge only
  drains work **already QUEUED** in the inbox (exactly what the Stop-hook drain
  would deliver — `cold-check` reuses `config/hooks/resurface-decide` as the
  single deliverable-truth source). It does **not** promote / self-select
  backlog; that stays CONFIRM-REQUIRED (`promote-next` / `spawn-loop`).
- The trial tests **"auto-nudge to drain queued work,"** not "decide what to
  work on." The autonomy gate the human still owns (what enters the queue) is
  untouched.
- The deny floor, never-auto floor (TEAM.md), and HALT kill-switch are
  **unchanged.** `cold-check` is HALT-first: a HALTed seat is never nudged.
- **Clock starts when the host-cron is wired** (human-applied, host-side), not
  at this merge.

## 3. Mechanism

| Piece | Home | Who |
|---|---|---|
| Decision + evidence log (`NUDGE` / `NO-OP` / `HALT-blocked`) | `scripts/cold-check` | agent (ordinary, this PR) |
| Append-only trial log | `.scratch/inbox/.cold-floor-log` (host/append-only when load-bearing) | `cold-check` writes |
| Hourly cron `0 * * * *` + the auto-nudge inject on `NUDGE` | host-side / topology | **human-applied** (flagged) |

`cold-check` **emits only** — it decides + logs + prints the verdict; it never
nudges. The host-cron reads `NUDGE` and performs the option-B inject. Each fire
appends one line: `epoch | verdict | summary | nudged=yes|no`.

## 4. Exit criteria (verdict at day 7)

**Keep option B iff, over 7 days:**

| Check | Pass condition |
|---|---|
| Zero spurious nudges | Never nudged with no deliverable QUEUED work |
| Zero HALT violations | Never nudged while HALT present |
| No S1/S2 incident | No unintended mutation / no write outside sandbox (auto-trial severity classes) |
| Nudge volume sane | No thrash — redundant nudges to an already-live session stay bounded (see §6) |
| Cost acceptable | Per-fire cost (if measured) within budget |

Else revert. Re-enable is **human-only**.

## 5. Rollback (pre-authorized, anytime)

Disable the host-cron → reverts to option (a) human-nudge. No permission needed;
report after. Revert only removes the auto-pulse; the live drain + self-wake and
the manual human-nudge path all remain. The risky direction (re-enable) stays
human-only.

## 6. Watch-items / known-benign

- **Redundant nudge to a live session.** If the cron fires in the <1h window
  after queued work lands but before the live drain clears it, it may nudge an
  already-live seat. Harmless (the drain is idempotent: TAKEN + `.drain-count`),
  but counted under "nudge volume sane." This is *why* `cold-check` needs no
  separate liveness probe: queued-and-undrained work is itself the
  idle-with-pending-work signal, and the hourly cadence supplies the "cold."
- **Log lives in agent-writable `.scratch/`.** Before the floor is load-bearing
  the log + cron must move host-side / append-only (same carry-forward class as
  the spawn-ledger and merged-refs external-truth files).

## 7. Human action

Wire the hourly host-cron (`0 * * * *`) to run `scripts/cold-check` against the
loom-dev seat paths and, on a `NUDGE` verdict, inject the auto-nudge to the
Writer session. Host-side/topology — like a trust bit. The trial clock starts
at that wiring.
