---
name: replan
description: Audit and re-sort the tactical queue in docs/PLAN.md — re-prioritize by dependency/priority, flag orphan PRs, stale blockers, and done-but-unmarked rows, then propose a queue diff. Use when the user (or the Coordinator hat) asks to replan, re-sort the queue, or audit queue integrity. Touches nothing outside the fenced queue section.
---

# /replan — tactical queue audit + re-sort

You are operating the Coordinator hat (docs/TEAM.md). Your write surface is
EXACTLY the fenced block in `docs/PLAN.md` between
`<!-- BEGIN TACTICAL QUEUE` and `<!-- END TACTICAL QUEUE -->`. Never edit
anything else in PLAN.md (the phase roadmap is human-owned) or any other file.

## Procedure

1. **Read the inputs:**
   - The queue section in `docs/PLAN.md` (rows: task | depends-on | serves |
     owner | status | PR).
   - `docs/OPEN-THREADS.md` — thread statuses are the ground truth for
     blocked/resolved claims.
   - Recent merge history (`git log --oneline -30` on main) and, when
     available, open PRs (host-provided list or `gh pr list` output relayed
     in) — the container itself has no GitHub access (T18).
2. **Audit — flag, with evidence, every:**
   - *Orphan PR:* a merged/open PR with no queue row (bookkeeping rule
     violated — the row must be added by whoever ships next).
   - *Stale blocker:* a row whose depends-on is already satisfied (e.g. the
     thread it cites is ✅/merged) but still reads blocked/queued.
   - *Done-but-unmarked:* a row whose work is visibly merged but status isn't
     done.
   - *Dependency inversion:* a row ordered above something it depends on.
3. **Re-sort** by: unblocked work first, dependency order, then serves-weight
   (cutover/criteria items above hardening), human-owned rows where they fall.
4. **Output a queue-diff proposal:** the current table, the proposed table,
   and a one-line rationale per moved/changed row. If invoked with authority
   to apply (the user said so), edit the fenced section to match; otherwise
   STOP at the proposal — the Coordinator proposes, the queue changes land via
   a normal PR.

## Boundaries

- Read-only outside `docs/PLAN.md`'s fenced section, always.
- Never change the bookkeeping rule, the fence markers, or the column set.
- Never mark a frozen-contract item accepted — that is a human act
  (docs/TEAM.md merge policy).
