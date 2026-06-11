# defaultMode "auto" trial — evidence ledger

Advisor-maintained record for the one-week `defaultMode: auto` trial
(decision 2026-06-11, human; package transcription = inbox item 001 → trial
spec doc, which should link here). Severity taxonomy per the decided package:
**S1** = never-auto-floor category executes without a prompt, or an unintended
mutation reaches main · **S2** = unintended write caught by gate/guard/checks ·
**S3** = annoyance/guard misfire (feeds the allowlist as evidence).

Status: **pre-trial** — the mode flip has not happened; entries below are
baseline-period evidence. The trial clock starts at the flip.

## Day-1 (2026-06-11, pre-trial) — AUTOPILOT first flip & the LL-011 incident

### Timeline
| Time (approx, Z) | Event |
|---|---|
| 08:50 | First AUTOPILOT flip ON (`loom-author` inbox header; human instruction) |
| 08:55 | Misfire #1: the ADVISOR session's Stop hook drained Writer cargo — `drain-inbox.sh` had no role scoping (repo-level hook fires in every session on the shared tree). Item 001 flipped to TAKEN by the wrong session. Contained: AUTOPILOT off, item restored, drain counter reset; no work done by the wrong role. Fix filed as inbox item 002. |
| ~09:15 | Human override: ON with the known cost accepted; advisor babysits (refuse stolen cargo, restore by item id). |
| 09:15–09:45 | Misfires #2–#10: nine further advisor-session drains across stop-chains (item 001 ×7, item 002 ×2). All refused + restored. |
| ~09:40 | Babysit race specimen: a blind first-match restore briefly flipped the Writer's *legitimate* `TAKEN` on item 002; caught in seconds, recipe corrected to by-id restores. |
| 09:53 | **PR #54 merged**: role guard (`LOOM_SESSION_ROLE` env wins; `root` ⇒ loom-author fallback, revisit at T10 non-root) + LL-011 + regression test (foreign-role stop ⇒ exit 0, inbox untouched). Misfire loop dead. |
| ~10:00 | Item 002 DONE; AUTOPILOT re-flipped ON post-fix (gate satisfied, standing human instruction). |

### Classification
- 1 × **S2-shaped** (misfire #1: unintended inbox-state mutation, caught before
  cargo loss).
- 9 × **S3** (known-cause repeats during the accepted babysit window).
- **S1: zero.** The never-auto permission floor was never breached; no
  protected operation ran un-prompted at any point.

### Mechanism scorecard
| Guard | Verdict |
|---|---|
| Never-auto floor | ✅ held throughout |
| Drain budget (3/chain) | ✅ capped every runaway chain |
| Orphan refusal / design-envelope | ✅ (cargo carried valid `serves:`; not stress-tested) |
| Role scoping | ❌ absent → **LL-011**, fixed same day (#54) |
| Manual containment | ⚠️ workable but racy (first-match restore specimen above) — workarounds for mechanism bugs are themselves evidence for fixing the mechanism |

### Lessons fed forward
- **LL-011** (docs/LESSONS_LEARNT.md): repo-level hooks fire in every session
  on a shared tree; role-scoped behavior needs an explicit role check.
- Babysit/containment edits to shared mail must target item **ids**, never
  first-match patterns.
- Governance gaps surfaced → inbox item 003 (scoping clause): flag is per
  role × per project; HALT kill-switch for atomic revert-both; `flips.log`
  audit — this day's on/off/on dance was previously recorded nowhere but a
  conversation. This ledger closes that gap for trial-relevant events.

## Daily audit log (trial week — to be appended)

| Day | Date | S1 | S2 | S3 | Notes |
|---|---|---|---|---|---|
| 0 (pre-trial) | 2026-06-11 | 0 | 1 | 9 | LL-011 incident + fix (#54); see above |
