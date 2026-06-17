# Decision-trio skills — apply-steps (ADR-0022 trio; adv-073/076/077)

**Status: READY.** The three trio TOOLS + their guard tests landed on
`feat/decision-trio` (commit `de033e0`, gate + fr-verify green):
`scripts/ask-other-seat`, `scripts/cost-vs-gain`, `scripts/spike-sandbox`,
`internal/guard/trio_test.go`.

This doc stages the **skills' prose** half — three `.claude/skills/**/SKILL.md`
files. They are **ordinary-class** (NOT a protect-path / trust diff): the only
reason they are staged here instead of committed in place is that the **Writer
session that built the trio could not write under `.claude/`** (a per-session write
gate on that path). Apply them host-side, no `ALLOW_TRUST_CHANGE` needed.

## Apply steps (host-side, ordinary)
1. Create the two new skill dirs and write the files verbatim from the blocks below:
   - `.claude/skills/zoom-out/SKILL.md`  (new)
   - `.claude/skills/confer/SKILL.md`    (new)
   - `.claude/skills/spike/SKILL.md`     (REPLACE the existing #170 file — retrofits
     the sandbox tool into step 3 + discipline; description unchanged in spirit)
2. **`chmod +x` the three tool scripts** — they committed at mode `100644` because
   the in-session mode-set was gated; repo convention is `100755` (cf
   `scripts/cold-check`, `scripts/spawn-loop`):
   ```
   chmod +x scripts/ask-other-seat scripts/cost-vs-gain scripts/spike-sandbox
   git add --chmod=+x scripts/ask-other-seat scripts/cost-vs-gain scripts/spike-sandbox
   ```
3. `make gate` (skills are inert to the Go gate; this just re-confirms green), then
   the branch is complete for review + relay. **NO self-merge.**

## Queue / thread bookkeeping
- Serves adv-073 (skills-with-tools) + adv-077 (trio shape ruling). Both QUEUED items
  in the Writer inbox are satisfied by this branch.
- No new FR (methodology skills + tools; orphan tests are advisory). No PLAN row
  existed for the trio — add one at relay if the Coordinator wants it tracked.

---

## `.claude/skills/zoom-out/SKILL.md` (new)

````markdown
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
````

---

## `.claude/skills/confer/SKILL.md` (new)

````markdown
---
name: confer
description: Bring a decision that needs the OTHER seat's judgement to a structured, collaborative resolution — STEELMAN each option (argue every side at its best AND hunt each for the break) before converging, pull in the author/advisor across the seam, then take grounded findings to the human. Use on "/confer", or whenever a call genuinely needs the perspective of the seat you are not (the A-vs-B move). Collaborators, not opponents — adversarial rigor in the protocol, not in the relationship.
---

# /confer — decide WITH the other seat (collaborators, not opponents)

**The rule (mechanized).** Some decisions are not yours to make alone: the seat that
lives in the box and the seat outside it see different failure modes, and a call made
from one chair carries that chair's blind spot. */confer* is the move that pulls the
other perspective in — but as a **collaborator**, not a debate opponent. The name was
chosen with the human (2026-06-16): "debate" rewards defending your position over
finding the truth; "confer" keeps the relationship cooperative while the *protocol*
keeps debate's rigor. Confer the relationship; adversarial the method.

## When to confer
- A decision has a real **A-vs-B fork** and you can argue either side honestly.
- The call **depends on context the other seat holds** (host-side ops, in-container
  reality, outward credentials, the design's history).
- You notice you are about to decide alone something that touches the *other* lane —
  the exact miss `/zoom-out` step 3 also guards (the advisor's skipped-ask on the
  self-wake actuator).

If it is purely your lane and you can measure it, you may not need a confer — `/spike`
it or just decide. Confer is for *judgement* shared across the seam.

## Protocol

### 1. Frame the options neutrally
State each option in one line, without tilting the language toward your favorite. If
you have more than two, run `/zoom-out`'s enumeration first so the set is complete.

### 2. STEELMAN every option — the rigor (do this BEFORE converging)
For EACH option, in turn:
- **Best case** — argue it as its strongest advocate would. What makes it the right call?
- **Break it** — then attack that same option at its weakest joint. Where does it fail,
  and under what condition?

You are not allowed to converge until every option has been argued at its best AND
hunted for its break. This is where "debate's rigor" lives — a steelman you skip is a
position you never tested. (Optionally feed the survivors to `scripts/cost-vs-gain`
for an explicit ranked trade-off; the table is an input, not the verdict.)

### 3. Pull in the other seat — TOOL: `scripts/ask-other-seat`
Queue the framed fork + your steelmans across the seam, and ask specifically what they
see breaking:

```
scripts/ask-other-seat "confer: <decision>" "A vs B, my steelman of each + where I think each breaks. What do YOU see breaking?"
```

It appends a `status: QUEUED` envelope to the other role's inbox. The point is their
*independent* read — do not pre-answer for them.

### 4. Converge — find the truth, not the win
With both reads in hand, converge on the option that survives the most breaks, not the
one either seat started attached to. If a steelman exposed a fatal flaw, that option is
out regardless of whose it was. Record WHY the winner won (the break the loser could
not survive) — that reasoning is the deliverable, not just the choice.

### 5. Grounded findings to the human
Bring the human the converged recommendation + the load-bearing reason + the live
trade-off — not a re-litigation. Per C3 (ADR-0016) the human's call is the
acceptance; confer is how the two seats hand them a *grounded* choice instead of a
blind one.

## Discipline (non-negotiable)
- **Steelman before converge** — skipping an option's best case is the failure this
  skill exists to prevent. No exceptions.
- **Collaborators, not opponents** — attack options, never the other seat. The moment
  it becomes about whose idea wins, you have lost the truth.
- **The other seat's read is independent** — `ask-other-seat` frames the question; it
  does not answer it for them. Wait for the real reply.

## Relationship to the rest of the decision trio
- **/confer** — shared judgement across the seam (this skill): steelman both sides, ask, converge.
- **/zoom-out** — when the problem is "are these even the right options?", widen first.
- **/spike** — when a steelman hits an unknown you can't argue from the armchair, probe
  it empirically (`scripts/spike-sandbox`) and bring data back to the confer.
````

---

## `.claude/skills/spike/SKILL.md` (REPLACE existing #170)

````markdown
---
name: spike
description: Prototype a design decision before bringing it to the human. Use when a choice has cost / security / credential / integration unknowns — instead of asking the human to pick blind, advisor + author run a throwaway, time-boxed probe that answers the questions empirically and exposes gaps the design can't see, then bring grounded findings. Use on "/spike", "spike it out", or whenever you are about to AskUserQuestion between options whose trade-offs you cannot actually measure yet.
---

# /spike — prototype before the human decides

**The rule (mechanized).** A design decision with real unknowns does NOT go to the
human as a blind choice. Advisor + author **spike it first** — a throwaway,
time-boxed probe that answers the open questions by measurement and surfaces gaps
the design missed. *When in doubt, spike it out.*

This is the mechanism behind the standing methodology; an unenforced "we should
prototype" is trust, not a rule (RULES §5). The skill is how it actually happens, and
`scripts/spike-sandbox` makes the "run it isolated, then tear it down" half mechanical
rather than a discipline you must remember every time.

## When a spike is MANDATORY
Before an `AskUserQuestion` / a decision, when ANY of these is unknown and a probe
would settle it:
- **cost** — cold-start time, token/$ per run, latency.
- **security** — the "would the guardrails hold if you tried the worst thing" answer.
- **credentials / integration** — does it authenticate? where do creds come from?
  does it even RUN in the target environment? (loom's credential/login fragility is
  a classic gap — headless sessions, non-root creds readability, the WRITER LOGIN
  ERROR family.)
- **fit** — does the existing mechanism actually behave the way the design assumes?

If you cannot measure the trade-off you are asking the human to weigh, you owe a spike.

## Protocol
1. **Frame the unknowns as questions** with success criteria. ORDER them — put the
   load-bearing one first (often auth/creds); a spike that fails it short-circuits.
2. **Split the work** (advisor + author, per docs/TEAM.md roles):
   - advisor defines the spike, analyses, sketches the host-side / design parts;
   - author runs the in-container / environment-specific probes (it has that context).
3. **Run it isolated — TOOL: `scripts/spike-sandbox`.** Don't hand-roll the isolation;
   the tool gives you a throwaway sandbox with **guaranteed teardown** (a trap removes
   it even if the probe crashes), so a spike can never silently leak into prod or get
   mistaken for real work:
   ```
   scripts/spike-sandbox <topic> -- <probe command>          # dir sandbox, auto-torn-down
   scripts/spike-sandbox --worktree <topic> -- <probe>        # detached git worktree probe
   scripts/spike-sandbox <topic>                              # print a path for a manual probe
   scripts/spike-sandbox --clean <topic>                      # remove a leftover manual sandbox
   ```
   NEVER wire a spike into an autonomous loop, and never merge it as a feature — the
   sandbox is detached/throwaway precisely so it cannot become one.
4. **Write findings, not code**, to `.scratch/spikes/<topic>.md`: the answer to each
   question + any GAP that reorders the work.
5. **Then decide with the human** — bring findings, not guesses. A blocking gap
   (e.g. auth fails) becomes the next work item; only then revisit the original fork.

## Discipline (non-negotiable)
- **Throwaway + time-boxed.** A spike is a learning probe; the real build follows the
  answers and is written fresh. Don't gold-plate a spike into production.
- **A gap reorders the work.** If the spike exposes a blocker, solve it first; the
  original decision waits on it.
- **Don't hardcode a tool to dodge a spike.** Coupling loom to one tool (e.g. tmux
  send-keys as a wake mechanism) to avoid measuring the alternative is dispreferred —
  it fights loom's tool-agnostic design. If forced to a dispreferred option,
  OPEN-THREAD it to amend later; don't silently bless it.

## Output
A `.scratch/spikes/<topic>.md` findings note + a grounded recommendation — NOT a
merged feature. The decision that follows is the human's, now informed.

## Relationship to the rest of the decision trio
- **/spike** — measure an unknown empirically (this skill) when the trade-off can't be
  reasoned from the armchair.
- **/zoom-out** — when the real question is whether you have the right options at all.
- **/confer** — when the call needs the other seat's judgement, not just a measurement.
````
