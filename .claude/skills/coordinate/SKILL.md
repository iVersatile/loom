---
name: coordinate
description: Run the Coordinator hat's standup, draft-triage, or weekly report. Read mode (any role): queue audit, drift warnings, gate/watch status, human to-do — rendered yesterday/today/blockers, report-only. Verdict mode (hat-holder arms it): triage kind:draft inbox items into promote/merge-into/park/drop as ONE batch PR. Weekly mode: compose docs/reports/YYYY-WW.md (confidence lens, dependency graph, coverage map, EXPERIMENTS) via PR. Use when the user asks /coordinate, wants the daily standup, wants drafts triaged, or wants the weekly report.
---

# /coordinate — the Coordinator hat, mechanized (T25)

Three modes, one hat. **Authority is pinned to the HAT, not this skill**
(docs/TEAM.md "Coordinator authority"): propose-only always; scheduled runs
are hat-holder only (advisor today); a non-hat run is allowed but its output
MUST open with "non-hat run".

## Mode: read (default — `/coordinate`)

Report-only, any role, any mode. No tree writes, no inbox status changes.

1. **Queue audit** — run the `/replan` procedure steps 1–2 (audit only,
   never apply): orphan PRs, stale blockers, done-but-unmarked rows, orphan
   inbox items, stale TAKEN envelopes.
2. **Gates & watches** — `after:` conditions in both inboxes vs reality
   (row-done / pr-merged); the trial clock and rollback triggers
   (docs/auto-trial.md) during the trial week; verify-loom-dev claims if
   run recently.
3. **Context sweep** — UNREAD fyis in this role's inbox (report → mark
   READ); flips.log tail; drafts awaiting triage (count + ids, no verdicts
   in read mode).
4. **Work-source cascade** (TEAM.md "Work selection") — if the queue's
   live band is dry, generate candidate rows from **spec gaps** (uncovered
   FRs, clauses without FRs, spec-map yellow/red verbs, unmet phase
   criteria) as PROPOSALS in the report — propose-only, human disposes;
   this runs in the DAILY run only, never per-session. If specs are
   exhausted too, render the **"PHASE SCOPE COMPLETE" report** (queue dry
   · coverage % · phase-criteria status · candidate next-scope menu) —
   never "project done"; phase boundaries are human sentences.
5. **Render** as the standup: **yesterday** (achievements-style, queue rows
   flipped since the last run) / **today** (the live band: next + in-review
   + what each waits on) / **blockers** (human-owned gates, drift findings,
   pending acks). The standup carries the **mix line** (mandatory telemetry,
   TEAM.md work selection): `work mix: N% product-spec / M% meta; phase
   criteria last touched D days ago`. Close with refresh prompts when
   stale: "/achievements since last run?", "/specmap regen?" — prompts,
   not auto-runs.
6. During the trial week, append the daily audit row data (S1/S2/S3
   observations) for the advisor's ledger — observation, not the ledger
   write itself.

## Mode: verdict (`/coordinate verdict` — triage the draft lane)

Input: every `kind: draft` item across both inboxes. For each, ONE verdict:

| Verdict | Effect (all land in ONE batch PR) |
|---|---|
| **promote** | queue row + a real work envelope (task/design) appended to the owner role's inbox; draft marked with the verdict |
| **merge-into** | content appended to the EXISTING item/row it duplicates (one decision = one envelope) |
| **park** | thread stub in OPEN-THREADS (durable, not work); draft marked |
| **drop** | one-line reason in the triage record; **requires cross-role ack before disposal** — the only verdict that destroys information |

- Output is **one batch PR** carrying: queue rows, envelopes, stubs, and a
  triage record (verdict + reason per draft). **The PR is the proposal** —
  disposal of the drafts themselves stays with the arming role on merge.
- **Self-verdict flags:** any verdict on a draft the runner authored, or
  that routes work to/away from the runner, is marked `⚑ self-verdict —
  arm-er must confirm` in the triage record.

## Mode: weekly (`/coordinate weekly` — the weekly report, P12)

Output: `docs/reports/YYYY-WW.md` (ISO week), shipped **via the normal
branch + PR flow** — like verdict mode, the PR is the proposal. Decided by
the human 2026-06-12 (draft 015 + experiments amendment): the weekly report
is a /coordinate mode, NOT a new channel. **Visual-first**: maps, tables,
percentages — prose only where a number can't carry it.

Compose these sections, in order:

1. **Shape** — the current spec→FR→thread map: embed or link
   `docs/spec-map.md` (regenerate via `/specmap` first if registry or
   threads changed this week; that lands as its own PR per that skill).
2. **Confidence lens** — FR coverage % by `kind` × test tier, computed from
   `docs/FR-registry.yml` (`tests:` joints) crossed with `go test -json
   ./...` results: per kind (behavioral / invariant / guardrail / schema),
   how many FRs have all named tests present AND passing, at which tier
   (gate-local vs integration). Plus: gate status (last `make gate`), and
   evidence freshness (dates of guided-run / reviews / e2e artifacts cited
   by open rows).
3. **Shipped** — the `/achievements` lifecycle dashboard over the week's
   window (queue rows flipped done/in-review, PRs, threads, lessons).
4. **Dependency graph** — the queue's `depends-on` column rendered as a
   Mermaid digraph: nodes = live rows (queued / in progress / in review /
   blocked), edges = depends-on, human-owned gates marked distinctly. Done
   rows are omitted (the Shipped section owns them).
5. **Coverage map** — FRs→tests→tiers shaded onto the SPEC sections they
   cite (absorbs P8): per SPEC anchor, green (all FRs covered) / yellow
   (partial) / red (FRs with no automated coverage — ADR-0013 violations).
6. **EXPERIMENTS** — one row per active or just-completed experiment:
   `name | hypothesis | telemetry | status / verdict date | key numbers`.
   Seed set (issue 1, 2026-06-18):
   - **auto-trial (T22)** — acceptEdits→auto; S1/S2/S3 counts vs baselines
     (advisor 2.2 would-prompt/turn, writer 2.5); day-7 verdict that day.
   - **coordinator principles-trial (draft 018)** — judgment vs rules;
     work-mix line history; switch-trigger status (2-week horizon).
   - **cold-start continuity (P2 run 1)** — scored 5.5/6
     (docs/e2e/cold-start-continuity.md); loss classes; next run ad hoc.
   A completed experiment stays listed for ONE issue past its verdict, then
   graduates to the thread/LL record.
7. **Trial/ops + human to-do** — ledger excerpt (docs/auto-trial.md
   evidence table) + flips.log tail for the week; the human-owned gate list
   from read mode, deduplicated.

Issue 1 (2026-06-18) doubles as the **trial day-7 verdict report**: the
EXPERIMENTS auto-trial row carries the keep/revert verdict input (zero
S1+S2 = keep), and the report leads with it.

## Boundaries

- Read mode: zero writes anywhere (fyi READ-marking is the one exception —
  it is this role's own inbox status, the allowed surface).
- Verdict mode: writes ONLY via the batch PR (branch + gate + review);
  never disposes drafts directly; never touches frozen paths (a promote
  whose work needs a SPEC change carries that as a flagged follow-up, not
  an edit).
- Weekly mode: writes ONLY `docs/reports/YYYY-WW.md` (+ its queue row) via
  its PR; it composes other artifacts but never regenerates them in-place —
  a stale spec-map prompts a `/specmap` run, never an inline edit. Numbers
  come from the named sources (registry, go test -json, ledger, flips.log),
  never from memory.
- Cross-role: never edit the other role's inbox except APPENDING promoted
  envelopes (the cross-agent write rule).
- Gates are events, never times (TEAM.md context economy): a verdict may
  set `after:` conditions, never wall-clock waits.
