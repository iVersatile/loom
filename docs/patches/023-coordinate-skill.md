# Prepared diff — /coordinate SKILL.md, envelope 023 (apply + delete this file)

> Why this file exists: the auto-mode classifier denied the Writer's direct
> edit to `.claude/skills/coordinate/SKILL.md` (self-modification class:
> agent-behavior config, task sourced from an inbox envelope). Same pattern
> as item 022's RULES pointer: the diff is prepared, a human (or an
> explicitly-instructed session) applies it. Delete this file in the same
> commit that applies it. Note for draft 029-B: the classifier allowed the
> sibling `replan/SKILL.md` edit in the same batch — protection today is
> inconsistent; mechanism (protect-paths on `.claude/**`) would not be.

In `.claude/skills/coordinate/SKILL.md`, mode: read — replace steps 4–5:

```markdown
4. **Render** as the standup: **yesterday** (achievements-style, queue rows
   flipped since the last run) / **today** (the live band: next + in-review
   + what each waits on) / **blockers** (human-owned gates, drift findings,
   pending acks). Close with refresh prompts when stale: "/achievements
   since last run?", "/specmap regen?" — prompts, not auto-runs.
5. During the trial week, append the daily audit row data (S1/S2/S3
   observations) for the advisor's ledger — observation, not the ledger
   write itself.
```

with:

```markdown
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
```
