---
name: achievements
description: Narrate what shipped since a given point — queue rows flipped done/in-review, with PRs, threads, lessons, and trust-flips hanging off them — as the validated lifecycle dashboard (Item | Lifecycle | Current). Use when the user asks /achievements, "what shipped", "what did we get done", or wants a progress narrative since a date or PR. Report-only: never writes the tree.
---

# /achievements — queue-anchored shipped-work narration

Sibling of `/replan`: that one audits the queue, this one narrates it. The
**queue is the anchor** — rows flipped `done`/`in review` since `<since>` are
the "what shipped" truth; everything else (PRs, OPEN-THREADS stubs, LL
entries, flips.log, inbox DONE items) hangs off those rows. Never anchor on
git alone: commits without a queue row are housekeeping, not achievements.

**Report-only — NO tree writes.** This skill must work for any role in any
mode. If you notice queue drift while gathering (done-but-unmarked rows,
orphans), put it in the housekeeping section and suggest `/replan`; do not
fix it here.

## Arguments

`/achievements [since]` where `since` is one of:
- a date (`2026-06-10`) — taken as midnight, local repo time;
- `yesterday` (default when omitted) — the prior calendar day's midnight;
- a PR number (`#54` or `54`) — resolved to that PR's merge time via
  `git log --merges`.

## Procedure

1. **Resolve `<since>`** to a `git log --since`-compatible string. For a PR
   number: `git log --merges --grep "pull request #NN" --pretty=%cI -1 main`.
2. **Gather mechanically** — run the helper (read-only):
   `sh .claude/skills/achievements/gather.sh "<since>"`.
   It emits: merged PRs since; current done/in-review queue rows;
   OPEN-THREADS headings + status markers; LL headers; flips.log; inbox DONE
   ids per role. The helper gathers; **synthesis and categorization are
   yours**, not the script's.
3. **Select the achievement set:** queue rows whose status flipped to
   `done`/`in review` within the window (the row's PR column → merge dates
   from step 2 output decide membership; a row already done before the window
   doesn't count). For in-review rows, mark them as such — shipped-to-review
   is an achievement with a pending lifecycle stage.
4. **Categorize each row** — one of:
   - `spec` — frozen-contract or doc-of-record changes (SPEC clauses, ADR
     flips, RULES/TEAM/charter text, FR registry);
   - `decision` — decided-and-transcribed packages (thread stubs, trial
     contracts, scoping clauses) that bind behavior but aren't engine code;
   - `mechanism` — code/hooks/tests/scripts that enforce or do something
     (engine work, guard hooks, skills, CI).
5. **Render the report** (validated format, human-confirmed 2026-06-11 —
   the lifecycle DASHBOARD, not a category table):
   - **One table:** `| Item | Lifecycle | Current |` where
     - *Item* — short name (+ thread T-NN where one exists);
     - *Lifecycle* — the arrow chain walked so far, e.g.
       `discussion → transcribed → live`, `incident → fixed → live`,
       `decision → transcribed → in acceptance`; mark a stalled stage
       explicitly (e.g. `→ not live`);
     - *Current* — concrete state NOW: pointers (#PR, item id), blockers,
       and who/what it waits on (e.g. "merged #57; HALT armed",
       "all gates cleared — only the human flip remains").
   - The `category` (spec/decision/mechanism) from step 4 is NOT a column;
     mention it inline only if the user asks for categories.
   - **Housekeeping (optional, after the table):** bookkeeping-only merges
     (queue flips, row syncs), plus any drift noticed — flagged, never
     fixed here.
6. **Cite as you go:** every line carries its PR number(s) and, where
   relevant, thread (T-NN) / lesson (LL-NNN) / inbox item id. No achievement
   without a pointer a reader can follow.

## Boundaries

- Read-only, always — no Edit/Write, no git mutations, no inbox status
  changes (not even DONE flips; that's the owning role's drain/ship flow).
- The queue is canon: if git shows work the queue doesn't, report the gap in
  housekeeping rather than promoting the commit to an achievement.
- Cross-role: inbox DONE items are reported for BOTH roles' files when
  readable; never edit either.
