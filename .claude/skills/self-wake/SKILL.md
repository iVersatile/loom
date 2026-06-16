---
name: self-wake
description: Arm a bounded self-scheduling /loop so an idle Writer wakes ITSELF when there's deliverable work — shape A of the ADR-0022 wake mechanism. Use when the Writer is about to go idle but the backlog/inbox may yield work soon, or on "/self-wake" / "arm self-wake". It runs scripts/self-wake-tick (which wraps the spawn-loop decision) each turn and arms the harness ScheduleWakeup primitive on a WAKE verdict — NO spawn, NO new agent, NO trust-path hook. NOT for the cold fully-stopped case (an external trigger re-wakes that; see Cold-start floor).
---

# /self-wake — the Writer wakes itself (shape A, ADR-0022)

**The mechanism (decided).** ADR-0022 first reached for an external ephemeral
worker (`claude -p`); the adv-071 spike found that path hits two walls — headless
auth AND the permission classifier (an agent may not self-bootstrap another agent).
**Shape A** wins (adv-075 spike, human-confirmed adv-076): the agent arms its OWN
next inbox re-check via the harness `ScheduleWakeup` primitive — a first-party tool,
not a spawn. This is the missing wake-primitive (T27 facet A / #129): the drain
handles "work exists at THIS stop"; self-wake handles "make a stop RECUR when idle."

The decision logic is already built + merged (`scripts/spawn-loop`, #172): refill
from the READY backlog → rate/HALT gate → emit a fixed-vocab wake verdict. This
skill is the wiring recipe — the `/loop` that turns a verdict into an actual wake.

## The recipe (what the /loop does each turn)
Run the one tick command and act on its leading verdict token:

```
scripts/self-wake-tick
```

| verdict | the agent does |
| --- | --- |
| `WAKE delay=N` | work refilled → `ScheduleWakeup(N, "re-check inbox")`, then stop |
| `WAKE-POLL delay=N` | no work yet, under the give-up bound → `ScheduleWakeup(N, …)`, then stop |
| `WAKE-BACKOFF delay=N` | rate-bounded → `ScheduleWakeup(N, …)`, then stop |
| `IDLE-STOP …` | gave up after K empty ticks → **do NOT re-arm**; let the session stop |
| `NO-WAKE …` | HALT / human-tier / fail-safe → **do NOT re-arm**; let the session stop |

`self-wake-tick` wraps `scripts/spawn-loop` with the seat's standard paths
(`docs/PLAN.md`, `.scratch/inbox/loom-author.md`, the merged-refs cache, the
spawn-ledger, the HALT sentinel, the wake-tick counter). The agent never recomputes
the decision — it only relays the verdict to `ScheduleWakeup`.

## Bounds (already in spawn-loop — do NOT re-implement here)
- **Cadence clamp** `[60, 3600]s` — honors the prompt-cache economics; never a
  sub-minute busy-poll. Defaults: work 60s, idle 1800s (`LOOM_WAKE_*` to tune).
- **Give-up after K empty ticks** (`LOOM_WAKE_MAX_EMPTY`, default 5) → `IDLE-STOP`,
  no re-arm. Prevents a perpetual-motion waker.
- **HALT-first** — a present `.scratch/inbox/HALT` ⇒ `NO-WAKE`, always.
- **Human-interruptible** — the human can cancel any scheduled wake or set HALT.

## When to arm / not arm
- **Arm** before going idle when backlog/inbox work may become deliverable soon.
- **Do NOT arm** past a HALT, and never on a `NO-WAKE`/`IDLE-STOP` verdict (that is
  the loop telling you to stop — respect it; arming anyway is a spin).
- Arming a live self-wake loop is an autonomy posture change — keep it under the
  same human-blessed AUTOPILOT/HALT controls as the drain (flips.log, T23).

## Cold-start floor (the gap shape A does NOT close)
After `IDLE-STOP` the session is FULLY stopped — no live `/loop` runs, so nothing
re-arms. Re-waking a cold session when NEW backlog appears is inherently an
**external trigger** (adv-075 finding: neither shape A nor a Stop-hook can re-wake a
session that generates no Stop). The honest, bounded options, cheapest first:
1. the **human's next message** (the natural trigger `IDLE-STOP` defers to);
2. a **low-frequency host-side tick** (e.g. hourly cron) that runs the cold-check
   and re-engages only if cold-idle AND deliverable work waits — host-side because a
   cloud agent cannot reach the in-container Writer (adv-076 item 2, proposed);
3. a backlog/PR-merge event hook.
Do not build a perpetual waker — `IDLE-STOP` + an external trigger is the correct shape.
