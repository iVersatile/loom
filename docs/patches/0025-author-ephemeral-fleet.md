# ADR-0025 (Proposed) — Author as an ephemeral headless fleet; advisor as the persistent supervisor

- **Status:** Proposed (drafted by advisor 2026-06-17; acceptance = human, ALLOW for any frozen-contract touch)
- **Threads:** T33. **Extends:** ADR-0022 (autonomy substrate — ephemeral worker), ADR-0020 (closed loop). **Hard-depends:** ADR-0023 (own-worktree). **Couples:** T27 (wake), T30 (refill), T32 (supervising-box).
- **Origin:** the cold-floor wake spike of 2026-06-17 — a stopped author was woken by `loom exec → claude -p --permission-mode acceptEdits`, drained its inbox, built the decision-trio (#178/#179), and exited. That run made a latent truth explicit (see Context).

## Context

The autonomous loop has carried an unexamined assumption: **loom-author is a long-lived, human-facing session.** The wake spike showed that assumption is the source of three distinct problems, and that it is not actually required:

1. **Cold-stop (obs-5):** a fully-stopped session cannot wake itself; there is no "resume-from-dead" primitive. The cron does **not** revive the stopped session — it spawns a *fresh* worker. So the persistent session, once stopped, is **abandoned**; it is not the thing that resumes work.
2. **Compaction/`/clear`:** a persistent session accrues context that must be compacted/cleared and rehydrated — an entire problem class.
3. **Idle-vs-dead ambiguity:** "is the author idle or stopped?" is undecidable from outside (the same liveness question as T31).

The spike's ephemeral worker had **none** of these: it was born fresh, rehydrated from the checkpoint + inbox hooks, did one work-item, committed, and exited. Statelessness was a feature.

## Decision (proposed)

Split the conflated "loom-author" into two modes and make the autonomous backbone the ephemeral one:

| Mode | Nature | Role |
|---|---|---|
| **author-worker** | headless, **ephemeral**, parallelizable | the autonomous loop's unit: drain → build → commit → exit |
| **author-interactive** | human-facing, **on-demand** | when a human pairs/steers — optional, off the critical path |

And adopt **1 persistent advisor ↔ N ephemeral author-workers**:

- The **advisor** is the *only* long-lived seat — coordinator, durable memory, git-controller/relay, GC, gate-holder. This is the **supervising-box** (T32).
- **author-workers** are spawned per work-item (by the cold-floor cron, or by the advisor), each ephemeral; N may drain the queue in parallel.
- The persistent always-on author session is **retired as the autonomous-loop backbone** (it remains available as `author-interactive` on demand).

This is not a new invention — it is the completion of ADR-0022's "ephemeral worker" thesis, and it is more faithful to the north star (ADR-0005, ai-user-topology: the author was always conceptually an AI worker, not a human seat).

## Alternatives considered (`/confer`, steelmanned)

> `/confer` note: in this model there is no persistent author peer to confer with. The "cross-seat" becomes "cross-**perspective**," instantiated either by (a) the advisor steelmanning both sides, or (b) spawning an **ad-hoc author-worker** to argue the author side. This decision was run as (a) — `/confer` is not yet an applied skill (patch 0024), and spawning a worker for a design debate is heavyweight. The recursion is worth noting: *"confer with the author" = spawn an author.*

- **A — Ephemeral fleet (proposed).** *Steelman:* no cold-stop, no compaction, parallelizable, stateless (continuity = the checkpoint+inbox, which are durable), faithful to ADR-0022/0005. *Cost:* own-worktree becomes mandatory (ADR-0023); needs work-item locking, a reaper, a standing `acceptEdits` trust grant, and per-spawn cost; long-lived in-session context is given up (only as good as the checkpoint).
- **B — Keep persistent session, fix wake by resume.** *Steelman:* preserves long-lived session continuity; familiar. *Cost:* cold-stop stays unsolved — you can only `send-keys` into an *alive-idle* pane (fragile, blind), never a dead one; no parallelism; compaction remains; the wake primitive is the weakest link.
- **C — Hybrid / dual-mode.** *Steelman:* interactive when a human is present, ephemeral for autonomous drains — best of both. *Cost:* two modes to keep coherent; risk of a persistent session and an ephemeral worker **double-working** the same item (needs the same locking as A).

**Lean:** **A for the autonomous loop**, with `author-interactive` available on demand (i.e., C's interactive mode layered on an A backbone). B is rejected — it leaves obs-5 unsolved.

## Consequences — prerequisites this makes mandatory

1. **Own-worktree per worker (ADR-0023)** is no longer optional — N>1 collides in the shared checkout (the LL-015 quirk we hit when the spike worked in the shared tree).
2. **Work-item locking** — extend the drain's `TAKEN` / `.drain-count` into a real per-item claim so two workers never grab the same item.
3. **Failure-mode reaper** — see below (Q3).
4. **Standing permission posture** — `acceptEdits` for unattended workers is a durable trust grant (deny-rules still protect trust paths; NOT `--dangerously-skip`).
5. **Concurrency + cost cap** — the spawn-guard rate ledger (#169) already seeds this; the fleet size N must be bounded.
6. **`/confer` adaptation** — cross-seat → cross-perspective (advisor self-steelman, or ad-hoc author spawn).

## Failure-mode cleanup for ephemeral workers (the reaper)

An ephemeral worker that completes cleanly self-terminates and is GC'd at relay (advisor prunes the worktree+branch — done manually in the spike). The **failure** case (crash / timeout / hang mid-item) needs a mechanism, because it leaves: a dirty worktree, an uncommitted/partial branch, and a work-item stuck `TAKEN` but undone.

**Proposed: `scripts/reap-workers` (offline, advisor runs it each git-controller tick; same discipline class as `cold-check`/`spawn-guard`):**

- **Inputs:** the worktree set under `.claude/worktrees/`, the spawn-ledger (start time per worker, #169), the HALT sentinel.
- **Liveness signal** (the crux — same question as T31): a per-worktree **heartbeat** the worker's wrapper refreshes (`.worker-heartbeat` mtime), cross-referenced with the spawn-ledger start time. Absent a heartbeat, fall back to a generous **max-age TTL** on the worktree.
- **Fixed-vocab verdict per worktree** (injection-proof, HALT-aware, idempotent):
  - `KEEP` — committed work present (advisor relays it) **OR** heartbeat fresh (worker alive — never reap a live worker).
  - `REAP` — no commit ahead of main **AND** (heartbeat stale > T **OR** ledger says past max-duration) **AND** no HALT → remove the worktree, delete the partial branch.
  - `REQUEUE` — on REAP, if the worker had claimed an item (`TAKEN`), flip it back to `QUEUED` (or `PARK` with a `retry:n` note + an escalate-after-k-retries cap) so the next wake retries rather than the item being lost.
- **Safety:** never reap on a fresh heartbeat (avoids killing a slow-but-live worker — the cold-floor's "redundant nudge is harmless" logic inverts here: a *wrong* reap is destructive, so the liveness bar is conservative). HALT freezes the reaper too.

This reuses the T31 discipline (a recency/heartbeat heuristic rather than a fragile external liveness probe) and the spawn-ledger external-truth file.

## Open questions (for human acceptance)

- Do we retire the persistent author session now, or run A and B side-by-side for a transition?
- Fleet size N and the cost cap (ties to the spawn-guard ledger).
- Heartbeat mechanism: wrapper-refreshed file vs. ledger-only max-age.
- Where the supervising-box (advisor) ultimately runs (T32) — host daemon vs. container.

## What this does NOT change

The human stays on the same two gates: **what enters the queue** (backlog refill / self-selection — the empty `LOOM_AUTOPULL_CLASSES` floor) and **trust-path merges**. The fleet is "autonomy inside an envelope," not unbounded.
