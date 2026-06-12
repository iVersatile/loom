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

Scored result (advisor, 2026-06-11 ~20:10Z): **5.5 / 6 — artifacts won.**
- Q1 ✓ last work named precisely (⑦ fix, LL-012/FR-PLAN-003) with an
  honest stale-refs caveat instead of a guess.
- Q2 ✓✓ exceeded the key: surfaced that T16 PR 1 (40309ec) was committed
  but NEVER PUSHED (no upstream) and its own queue row's "in review" was
  therefore false — a real bookkeeping lie neither the advisor nor the
  key had caught. All pending DONE-flips correctly enumerated.
- Q3 ✓ priority order reconstructed (T16 push → PR 2/3 → 013 remainder
  → T9), including that PR 2 consumes T25's deferred-boards decision —
  recovered from envelopes, not memory.
- Q4 ✓ honest, zero confabulation. Confirmed context-only losses, all
  predicted classes: (a) the WHY a push was held (transcript-only),
  (b) merge-states invisible in-container (T18 gap, known), (c) the
  day's rationale residue. Bonus hygiene catches: six stale /tmp
  worktree stubs; day-stale session-start snapshot.
- Delivery ✓ used the day-old T25 transport unprompted, as-of stamped,
  with a ground-truth disclaimer. Half-point deduction: chose `fyi`
  (expiring) for non-expiring experiment data — `draft` was the correct
  kind; kind-choice was a scored item by design.
Verdict: the context-economy thesis held its first crash. Loss classes
feed P2; the snapshot-staleness and worktree-stub catches feed the
harness-health review checklist (P7).

### Organic cold start — 2026-06-12 (subject: loom-advisor, post-reboot)

Not a protocol run (no kill, no no-hint prompt) — an ordinary reboot
cold start, scored honestly because it produced a NEW loss class.

- **FAIL on landing**: the session oriented from PLAN.md + git log and
  did not surface its own pickup list, last task, or the open human-todo
  until the human steered it twice — even though all three were written
  in the top STATE checkpoint of the advisor's workstream memory file.
- **New loss class: "memory present but unread at orient."** Distinct
  from every Run-1 class: the data survived (artifacts won) but the
  orientation PROCEDURE never consulted the surface that held it.
  Contributing cause: the memory index hook line advertised a day-old
  state ("@2026-06-11 EOD"), under-selling the fresh checkpoint.
- **Fixes applied same day** (advisor memory layer, off-tree): a
  standing read-checkpoint-first orientation rule + an imperative,
  date-current index hook ("READ TOP CHECKPOINT AT COLD START").
- **Rubric impact**: add to the scoring table — recovery counts only if
  the subject lands unprompted; "recoverable after human steering" is a
  procedure failure even when no data was lost. Harness-health checklist
  (P7) inherits: stale index/snapshot hooks are a named hazard
  (Run 1 already flagged snapshot staleness; this is the same family on
  the memory index).

### Organic cold start — 2026-06-12 (subject: loom-advisor, same reboot, second find)

Caught by the human mid-afternoon, hours after landing: five built Writer
branches (T4, r1-build, t16-gitconfig, work-selection, weekly-mode) sat
unpushed/un-PR'd across the restart; nobody surfaced them until the human
asked "where are the weekly report artifacts?"

- **New loss class: "in-flight branches present but unrelayed at orient."**
  Sibling of "memory present but unread": the data survived (branch refs
  held the work, 035 amendment even recorded the merge order), but no
  orientation surface listed UNMERGED BRANCHES as parked deliverables —
  the checkpoint's "branches swept" line referred to merged ones and
  read as all-clear.
- **Cost**: the weekly-report generator (needed 2026-06-18) was stranded
  on a local branch; ~5h of built work invisible to the relay holder.
- **Fixes applied same day**: (a) session-start hook (repo-clean-check.sh,
  advisor seat) now emits an "UNRELAYED BRANCHES" block from
  `git branch --no-merged main` with commit counts; (b) standing
  git-controller routine (memory layer): sweep → push → PR → merge per
  authority rules, muted; first sweep relayed all five (PRs #105–#109).
- **Rubric impact**: orientation surfaces must enumerate WORK PRODUCTS,
  not just tree cleanliness — a clean status with unmerged branches is
  not "clean", it is "parked". Harness-health checklist (P7) inherits:
  any "swept/clean" claim in a checkpoint must state its scope.
