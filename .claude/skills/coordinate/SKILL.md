---
name: coordinate
description: Run the Coordinator hat's standup or draft-triage. Read mode (any role): queue audit, drift warnings, gate/watch status, human to-do — rendered yesterday/today/blockers, report-only. Verdict mode (hat-holder arms it): triage kind:draft inbox items into promote/merge-into/park/drop as ONE batch PR. Use when the user asks /coordinate, wants the daily standup, or wants drafts triaged.
---

# /coordinate — the Coordinator hat, mechanized (T25)

Two modes, one hat. **Authority is pinned to the HAT, not this skill**
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
4. **Render** as the standup: **yesterday** (achievements-style, queue rows
   flipped since the last run) / **today** (the live band: next + in-review
   + what each waits on) / **blockers** (human-owned gates, drift findings,
   pending acks). Close with refresh prompts when stale: "/achievements
   since last run?", "/specmap regen?" — prompts, not auto-runs.
5. During the trial week, append the daily audit row data (S1/S2/S3
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

## Boundaries

- Read mode: zero writes anywhere (fyi READ-marking is the one exception —
  it is this role's own inbox status, the allowed surface).
- Verdict mode: writes ONLY via the batch PR (branch + gate + review);
  never disposes drafts directly; never touches frozen paths (a promote
  whose work needs a SPEC change carries that as a flagged follow-up, not
  an edit).
- Cross-role: never edit the other role's inbox except APPENDING promoted
  envelopes (the cross-agent write rule).
- Gates are events, never times (TEAM.md context economy): a verdict may
  set `after:` conditions, never wall-clock waits.
