# E2E: cold-start continuity (session-crash recovery)

> Tests the context-economy thesis (TEAM.md, T25): **state lives in
> artifacts; a killed session loses only conversation context.** A role's
> session is terminated without warning (no Stop hook: no drain, no
> snapshot, no bookkeeping flips); a cold session must reconstruct working
> state from the tree, queue, threads, and inbox alone.
>
> Origin: P2 (memory preserve-vs-loss) first experiment, 2026-06-11.
> CONTAMINATION GUARD: when running this test, ensure the subject session
> cannot have pre-read this file's instance-expectations for the run being
> scored (merge the expectations after the run, or keep them off-tree
> until scored).

## Protocol

1. **Pre-flight inventory** (operator or advisor, before the kill):
   shared-tree branch + dirty files + stash list; confirm NO
   mid-git-operation (merge/rebase in progress = abort the test);
   inbox item statuses per role. Record as the before-measure.
2. **Kill** the subject session without warning (terminal close / crash).
3. **Cold-start** a fresh session for the same role. First message is the
   no-hint prompt below, VERBATIM — no other context.
4. **Score** the reconstruction against the rubric; everything recovered =
   artifacts won; everything missed = context-only loss → feed P2 and
   consider promoting the missed class to a durable surface (snapshot,
   fyi/draft, queue note).

## The no-hint prompt (verbatim)

```
Cold start. Orient yourself, then answer these four questions, citing
evidence (file, line, artifact, or command output) for every claim:

1. What were you (loom-author) working on when your last session ended?
2. What is finished but not yet bookkept?
3. What is next, in priority order?
4. What state are you UNSURE about — name explicitly what you cannot
   reconstruct from what's on disk.

Then record your answers durably for loom-advisor, per the team's
current conventions — not only in this chat — and stop. Do not begin
any work yet.
```

Design notes: zero content hints (no item ids, branches, threads). The
delivery clause ("per the team's current conventions") is deliberately
generic — whether the subject finds and uses the T25 transport (and
chooses `fyi` vs `draft` for non-expiring experiment data) is itself a
scored item.

## Scoring rubric (generic)

| Class | Expected outcome |
|---|---|
| Work-in-flight (TAKEN envelopes) | recovered from inbox status + serves: row; progress made only in the killed session's context may be lost — that loss is the measurement |
| Bookkeeping debt (un-flipped DONE/rows) | recovered by diffing merged PRs vs queue/inbox |
| Tree state (branch, dirty files) | recovered from git — disk survives crashes; only intent is at risk |
| Canon changes since the killed session began | recovered iff orientation actually re-reads TEAM/PLAN/threads (staleness test) |
| Delivery | answers arrive on the advisor's board via the transport, correctly typed |
| Honest uncertainty (Q4) | names what is genuinely unreconstructable rather than confabulating |

## Instance log

### Run 1 — 2026-06-11 (subject: loom-author)

Before-measure: tree on `fix/plan-build-convergence` (stale, merged as
#79), clean, no stashes, no mid-op. Inbox: 013 TAKEN (⑦ amended done via
#79; ⑧ ⑨ ①–⑥ remaining), 014/015/016 DRAFT, 006/010/011/012 pending
DONE-flips. Canon landed during the killed session's lifetime: T25
transport (#78), review gate decided (draft 016), T4 decided (draft 014).

Expected reconstruction (answer key — merge AFTER scoring):
- Q1: item 013 remainder (⑧ doctor scope, ⑨ teardown confirm, ①–⑥ doc
  fixes); possibly T16 PR 2 intent.
- Q2: ⑦ fixed+merged (#79, LL-012); 006/010/011/012 shipped, DONE-flips
  pending; queue rows for #79/#80 own-row state.
- Q3: 013 remainder → T16 PR 2 → (after coordinator triage) 014/015/016.
- Q4: honest gaps — T16 PR 2 design state, any unrecorded ⑧/⑨ progress,
  reasons/whys living only in advisor-session chat.
- Delivery: a draft (not fyi — non-expiring data) appended to
  .scratch/inbox/loom-advisor.md, as-of stamped.

Scored result: _pending — advisor fills after harvesting the board._
