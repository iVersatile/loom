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
prototype" is trust, not a rule (RULES §5). The skill is how it actually happens.

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
3. **Run it isolated** — manual probes on a scratch branch. NEVER wire a spike into
   an autonomous loop, and never merge it as a feature.
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
