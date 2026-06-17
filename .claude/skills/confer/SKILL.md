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
