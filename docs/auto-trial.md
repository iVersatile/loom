# Auto-mode trial — exit/rollback package (T22)

> Status: **decided** (human, 2026-06-11, advisor session) — this document is
> the C3 transcription of that decision; acceptance = PR merge. It is the
> failure contract for the one-week `defaultMode: auto` trial. Not a frozen
> SPEC: it expires with the trial (keep / revert verdict recorded in the queue
> row, then this doc moves to `docs/threads/archive/` with T22).

## 1. Scope

- `permissions.defaultMode`: `acceptEdits` → `auto` for **one week**, both
  roles (loom-author + loom-advisor).
- **Baseline before flip:** per-role transcript scan (prompts/session) so the
  trial has a measured before/after, not an impression.
- The trial clock starts at the flip, not at this merge.
- The deny floor and the never-auto floor (TEAM.md) are **unchanged** by the
  trial — auto mode only widens the middle band.

## 2. Exit criteria (graded severity)

| Class | Definition | Weight at day-7 |
|---|---|---|
| **S1** | A never-auto-floor category executes WITHOUT a prompt, or an unintended mutation reaches `main` | Any ⇒ revert |
| **S2** | Unintended write caught by gate / guard / PR checks | Any ⇒ revert |
| **S3** | Annoyance / guard misfire | Doesn't block; feeds the allowlist as evidence |

**Day-7 verdict:** keep auto **iff zero S1 and zero S2**; else revert.
Re-flip / renewal is **human-only**.

**Verdict scope addendum (human-decided 2026-06-12):** the day-7 verdict
includes an **allow-list audit** — review accumulated grants across both
seats (`settings.local.json` allows AND per-project `allowedTools` in
`.claude.json`, which persist independent of the trial and are NOT reverted
by the mode flip): tidy entries with no matching review act, and hand the
result to T28's guardrail-drift detector as its first baseline. Rationale:
prompt-fatigue "yes-always" grows allow-lists silently (ADR-0017 caveat);
the verdict is the natural audit point.

## 3. Rollback triggers (cumulative; act mid-week, do not wait for day 7)

- **HARD — applies to BOTH roles:** any S1-class event · any write outside
  the repo sandbox · any secret in logs/output · any LL-010-class shared-tree
  incident traceable to auto.
  *Effect:* revert BOTH roles to `acceptEdits` immediately — a hard trigger
  means the shared mechanism failed, not a role.
- **SOFT — auto-revert applies to the ADVISOR only; for the Writer = flag +
  discuss:** ≥3 guard misfires/day, or a gate-failing change that
  `acceptEdits` prompting would have stopped.
  *Effect:* revert the firing role only.
- **Revert is PRE-AUTHORIZED** for any role — no permission needed, report
  after. Delegation is safe because revert only moves stricter; the risky
  direction (re-flip to auto) stays human-only.

## 4. Measurement

loom-advisor runs a **daily 5-minute audit of BOTH roles**: audit log, git
log vs expectations, deny/floor-prompt counts. The Writer does not
self-audit.

## 5. Asymmetry rationale

The advisor holds outward credentials (push / merge / gh API) → tighter leash
(hard + soft triggers). Writer mutations all pass gate + PR + CODEOWNERS →
hard-only is tolerable.

## Pointers

T22 (docs/OPEN-THREADS.md) · queue row "one-week defaultMode auto trial" ·
inbox item 001 (2026-06-11) · TEAM.md (deny floor, never-auto floor) ·
ADR-0016 / C3 (decision human-made; this is transcription).
