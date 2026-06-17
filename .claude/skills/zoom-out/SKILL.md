---
name: zoom-out
description: Stop and widen the frame before committing to a heavy or irreversible build — enumerate EVERY option (including "what are we already doing elsewhere that solves this?"), score them on an explicit cost/gain/risk/reversibility table, and pull in the other seat. Use on "/zoom-out", after repeated surprises in a row, or when you are about to commit to a costly/one-way path. Backs the enumeration with two tools (scripts/cost-vs-gain, scripts/ask-other-seat) so the widening is mechanical, not a good intention.
---

# /zoom-out — widen the frame before you commit

**The rule (mechanized).** When you are about to pour effort into a heavy or
one-way build — or when the same problem has surprised you two or three times in a
row — the failure mode is tunnel vision: you optimize the option in front of you and
never see the one beside it. *Zoom out before you dig in.* This skill forces the
wide look and backs the two mechanizable parts with tools, so it is a step that
actually happens, not an intention you trust yourself to remember (RULES §5).

The canonical miss this prevents: the advisor designed the self-wake **actuator**
toward an external-spawn before noticing loom *already had* the answer in hand — the
harness `ScheduleWakeup` primitive the advisor's own `/loop` was using. An
enumeration that asked "what are we already doing elsewhere?" would have surfaced it
on line one.

## When to zoom out
- **About to commit** to a costly / irreversible / wide-blast-radius build.
- **Repeated surprises** — the design has bitten you ≥2× in a row (a sign the frame,
  not the detail, is wrong).
- Before an `AskUserQuestion` that offers the human a narrow menu — make sure the
  menu is the *whole* menu first.

## Protocol

### 1. Exhaust the options — PROSE CHECKLIST (not a tool)
Enumeration is a reasoning act; no tool can truly do it (a "tool" here would just be
a checklist wearing a script's clothes — adv-077 ruling). So this step is a
checklist you work honestly. List EVERY option, including the ones you have already
half-dismissed, and explicitly answer each prompt:

- [ ] **The obvious one** — the path you were already on. Name it as one option, not the default.
- [ ] **"What are we ALREADY doing elsewhere that solves this?"** — the blind-spot
      catcher. Search the codebase / skills / existing mechanisms before inventing.
      (This is the prompt that would have caught `ScheduleWakeup`.) Run an `Explore`
      pass or grep if you are not certain.
- [ ] **The cheap / do-nothing option** — defer, no-op, or let an existing trigger handle it.
- [ ] **The opposite** — invert the assumption the obvious option rests on.
- [ ] **The borrowed** — how does another project / the other seat solve this?
- [ ] **What would make this decision unnecessary?** — reframe so the choice dissolves.

If the list has one entry, you have not zoomed out yet.

### 2. Score the trade-off — TOOL: `scripts/cost-vs-gain`
Make the comparison explicit and ranked, not narrative. Feed one line per option:

```
scripts/cost-vs-gain <<'EOF'
ScheduleWakeup-loop: cost=1 gain=5 risk=1 reversibility=5
External-spawn:      cost=4 gain=4 risk=4 reversibility=3
Do-nothing:          cost=1 gain=1 risk=1 reversibility=5
EOF
```

Each axis is 1–5 (`cost`/`risk` favorable when LOW, `gain`/`reversibility` when
HIGH); the tool prints a ranked table with the top option marked `RECOMMEND`. An
unscored axis is rejected on purpose — refusing to score *is* the hand-wave this
step removes. The ranking is an input to judgement, not a verdict: if the top row
feels wrong, the scores encode an assumption worth surfacing.

### 3. Pull in the other seat — TOOL: `scripts/ask-other-seat`
The seat that lives in the box sees gaps the seat outside it cannot (and vice
versa). Queue the framed question across the seam instead of deciding alone:

```
scripts/ask-other-seat "zoom-out: <decision>" "Options + my cost-vs-gain table; which do you see breaking?"
```

It appends a `status: QUEUED` envelope to the *other* role's inbox. This is the step
the advisor SKIPPED on the actuator — mechanized so it is not skipped again.

### 4. Converge, then decide
With the full option set, the ranked table, and the other seat's read, bring a
grounded recommendation to the human (or proceed if it is your call). If the wide
look reordered the work — a cheaper option, or one you already had — take *that*; do
not finish digging the hole just because you started it.

## Relationship to the rest of the decision trio
- **/zoom-out** — widen the frame (this skill): enumerate, score, confer.
- **/spike** — when an option's trade-off can't be *measured* from the table, probe
  it empirically (`scripts/spike-sandbox`) before choosing.
- **/confer** — when the decision genuinely needs the other seat's judgement, run the
  steelman-both-sides protocol, not a one-shot question.

A tool is mechanism; prose is trust. This skill is three steps — two are tools, one
(enumeration) is honestly a checklist, and the skill says so rather than faking it.
