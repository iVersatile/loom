# Guided run — Phase-1 criterion 1 runbook

> Phase-1 exit criterion 1 (docs/PLAN.md): *"a fresh machine reaches a working
> Go env in one guided run."* This runbook is the guide for that run; the
> results section at the bottom is filled **during** the run and is the
> criterion's evidence. The verdict is human-only (RULES — no agent
> self-approves phase completion). Scheduled: 2026-06-12, after the
> auto-trial flip.

## Ground rules (stranger-audit framing)

- The operator follows ONLY this runbook and the README quickstart. Any
  improvisation — an undocumented flag, a "oh you also need X", a detour into
  docs — gets written down in the deviations column; deviations are the
  finding, not a failure to hide.
- "One guided run" means one pass of the steps below, no restarts. A restart
  = record why, reset, count the attempt.
- Time each step (rough minutes are fine).

## Preconditions (verify before starting the clock)

| Check | How | Expected |
|---|---|---|
| Fresh machine state | `docker ps -a \| grep loom`; `ls loom.lock bin/loom 2>/dev/null` | no loom container, no lock, no binary |
| Docker present + running | `docker info >/dev/null && echo ok` | ok |
| Engine path decided | Go ≥ 1.26 (`go version`) **or** a prebuilt `bin/loom` (Mac topology: cross-compiled in loom-dev/CI — the Mac host has no Go) | one of the two; record which |
| Repo | fresh `git clone https://github.com/iVersatile/loom && cd loom` | clean checkout of main |

## The run

| # | Step | Command | Expected |
|---|---|---|---|
| 1 | Bootstrap + first build | `sh bootstrap/loom-bootstrap.sh build` | engine built (or prebuilt reused) → ONE unattended build: container up, `loom.lock` written. Minutes, not seconds (live provision from `debian:bookworm-slim` until ADR-0012's baked base) |
| 2 | Doctor | `bin/loom doctor` | clean bill: config refs resolve, guardrails verified |
| 3 | Working Go env (the criterion) | `bin/loom exec -- go version` | `go1.26.x` from INSIDE the container (login env, project at `/workspace/loom`) |
| 4 | Converged state | `bin/loom plan` | no drift vs the playbook |
| 5 | AI-first invariant spot-check | `bin/loom plan --json` | valid JSON, same verdict as 4 |
| 6 | Build is idempotent | `bin/loom build` (second run) | no-op / converged, fast |

## Pass criteria

- Steps 1–6 green in one pass, with step 3 proving the working Go env.
- Zero undocumented human improvisation — or every deviation recorded below
  (deviations feed README/bootstrap fixes; a deviation-heavy pass can still
  close the criterion at the human's judgment, with the fixes queued).

## Results (fill during the run — 2026-06-12)

| # | Step | Time | Observed | Pass/Deviation |
|---|---|---|---|---|
| 1 | bootstrap build | | | |
| 2 | doctor | | | |
| 3 | exec go version | | | |
| 4 | plan | | | |
| 5 | plan --json | | | |
| 6 | rebuild no-op | | | |

**Deviations / notes:**

**Verdict (human-only):** _pending_
