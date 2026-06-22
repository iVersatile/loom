---
name: self-wake-off
description: Disarm the self-wake /loop — the inverse of /self-wake. Stop re-arming the recurring backlog waker and cancel any pending ScheduleWakeup, leaving the Stop-hook drain, AUTOPILOT, and manual work untouched (this is NOT the broad HALT). Record the posture change in flips.log. Use on "/self-wake-off" / "stop self-wake" / "disarm self-wake" / "turn off self-wake", or to wind the recurring waker down deliberately. For a total emergency stop of ALL autonomy (drain + spawn too), use the HALT sentinel instead.
---

# /self-wake-off — disarm the recurring waker (the inverse of /self-wake)

`/self-wake` arms a bounded `/loop` that re-arms `ScheduleWakeup` each turn; this
skill **stops that re-arming** and cancels any wake already scheduled — nothing
else. It is the deliberate, audited counterpart to the loop's automatic
`IDLE-STOP` (the give-up-after-K-empty-ticks bound in `scripts/spawn-loop`).

## What this is NOT
- **NOT HALT.** HALT (`.scratch/inbox/HALT`) is the total kill — it blocks the
  drain, spawn-guard, AND self-wake (HALT-first everywhere). Use HALT for an
  emergency stop of *all* autonomy. `/self-wake-off` stops *only* the recurring
  waker; the Stop-hook drain and manual work keep running.
- **NOT an env change.** It does not unset `LOOM_AUTOPULL_CLASSES` or flip the
  `AUTOPILOT` header. The floor stays as declared; the loop simply stops recurring.

## The recipe (what the agent does)
Self-wake has no daemon — it persists only by re-arming each turn. So "off" is:

1. **Do NOT re-arm.** This turn, skip the `ScheduleWakeup` call (do not run the
   `/self-wake` recipe / `scripts/self-wake-tick`). With no re-arm, the loop ends
   after this turn.
2. **Cancel any pending wake.** If a wake is already scheduled, stop the harness
   `/loop` that `/self-wake` started so the deferred tick does not fire.
3. **Record the posture change** (an autonomy flip → audit trail, T23 convention) —
   append one line to `.scratch/inbox/flips.log`:
   ```
   <timestamp> | <actor> | self-wake: on→off | <reason>
   ```
4. **Confirm** to the human: self-wake disarmed; drain + AUTOPILOT + HALT
   unchanged; re-arm with `/self-wake` when wanted.

## Verify
Run `scripts/self-wake-doctor`. After disarm the `[cadence]` wake-ticks stop
advancing. Note: the doctor's `ARMED & READY` verdict reflects *eligibility*
(floor open + a READY `[class:exec]` row), NOT that a `/loop` is running — so
"off" is confirmed by the ticks no longer advancing, not by the verdict changing.

## Relationship to the other stops
| Mechanism | Scope | Trigger |
| --- | --- | --- |
| `IDLE-STOP` (`scripts/spawn-loop`) | self-wake only | automatic after K empty ticks |
| **`/self-wake-off`** (this) | self-wake only | **deliberate, audited** |
| `unset LOOM_AUTOPULL_CLASSES` / `AUTOPILOT: off` | self-wake winds down | env / posture change |
| `HALT` sentinel | ALL autonomy (drain + spawn + wake) | emergency, instant |

Re-arm is `/self-wake` — the same human-blessed AUTOPILOT/HALT controls apply
(arming a live waker is an autonomy posture change; keep it under flips.log + T23).
