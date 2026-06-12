# E2E — autopilot drain: from silent failure to (pending) proof

**Scenario family:** T21 transport / T27 control+observability.
**Subject:** loom-author Stop-hook drain (`.claude/hooks/drain-inbox.sh`).
**Run:** organic, 2026-06-12 (trial day 2) — not a protocol run; scored
because it surfaced two failure classes and validated the diagnosis loop.

## Timeline

| Time (Z) | Event |
|---|---|
| ~06-11→12 | Drain never fires; root cause 1 found by hand after ~6h: Writer launched with cwd `/`, repo settings never load, **hook never registers** (033 amendment). Human types "continue" at every stop — human-as-message-bus, the exact failure T21 exists to kill. |
| 12:36 | cwd fixed; drain fires ONCE (count=1) — only fire to date. |
| ~17:40 | Advisor triage files envelopes 040/041/042 with **descriptive `serves:` labels**. |
| 18:08 | Writer relaunched clean (`docker exec -w`, pwd verified); human kick → 039 built (direct pick). |
| 18:19 | Writer Stops. Drain runs, orphan-skips ALL THREE envelopes (guard (a): `serves:` must be a **literal queue-row substring**; labels match nothing). Exits silently — count stays 1. **Nobody notices for ~1h** until the human asks why the Writer is idle. |
| ~19:16 | Human kick #2 → 041 built (direct pick). Same silent orphan-skip at Stop. |
| 19:30 | Advisor root-causes from script source; fixes `serves:` on 040/042 to verbatim row fragments; files falsifiable prediction (fyi 043): next Stop picks 040, count→2. |
| ~19:45 | Human kick #3 → Writer takes 040 by **direct pick** (count still 1, status not flipped — drain untested by this). Prediction moves one Stop later: at 040 completion, drain must inject 042, flip TAKEN, count→2. |
| — | **PENDING:** the 042 injection is the first zero-human-input chain. Update this row with the result. |

## Failure classes (new)

1. **Hook present but never registered** (cwd/launch hygiene) — a hook you
   can't see register is indistinguishable from a working one that has
   nothing to do. Caught by hand, 6h late.
2. **Whole-set orphan-skip is invisible by construction** — guard (a)
   skips are flagged only in the continuation report of a *successful*
   pick; when every eligible item is orphaned there is no continuation,
   no log, no counter change. The mechanism reported failure as success
   (a normal stop).
3. **Authoring-contract gap:** `serves:` is a substring contract with no
   validation at filing time. The advisor (a careful writer, same day,
   same repo) violated it three-for-three. A contract violated silently
   by its most attentive user is a missing mechanism, not a discipline
   failure.
4. **Direct-pick discipline gap:** a kicked Writer taking work directly
   does not flip the envelope to TAKEN (the drain does it mechanically);
   stale-QUEUED states lie to the next drain pass.

## Mechanisms this episode argues for (T27 / fyi 043)

- **Decision trace (033.A):** one log line per drain run — fired-at,
  picked X / skipped Y-because-Z / nothing-eligible. Would have caught
  class 2 at 18:19 instead of via human question at 19:1x.
- **Watchdog (033.B):** "AUTOPILOT on + eligible QUEUED + idle = anomaly"
  page. Would have caught class 1 on day one.
- **serves: validation at filing** (fr-verify pattern): an envelope whose
  `serves:` matches no queue row fails LOUD at write time, not silently
  at drain time.
- **TAKEN-flip on direct pick** — fold into the 041 git-discipline
  family: whoever takes work flips the status, mechanically if possible.

## Scoring note

Same rubric as cold-start-continuity: recovery counted only when the
mechanism lands unprompted. Every recovery in this episode was
human-triggered (three kicks + one "why is it idle"). Autopilot score to
date: 1 mechanical fire, 5+ human interventions. The pending 042
injection is the first chance at an unprompted point.
