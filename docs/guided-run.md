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
| No instance collision (finding ①) | the host runs no container for this playbook's `name:` (covered by the fresh-state grep above) | absent — two checkouts of one project collide by design; rename `name:` to coexist |
| Plan-first on a non-fresh machine (finding ②) | anything loom-ish already present ⇒ `bin/loom plan` before any `build` | read-only verdict naming the target container and outstanding work; only `build` mutates |
| Timing honesty (finding ⑥) | `docker images \| grep bookworm` — is the base image already cached? | record cold-pull vs cached; a warm host (e.g. it runs another loom container) flatters build time |

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

## Results (run executed 2026-06-11, a day early — operator: human; clerk: loom-advisor)

Setup: fresh clone as `~/loom-guided`; `loom.yml` `name:` edited to
`loom-guided` (collision guard — the host runs the real `loom-dev`);
engine = prebuilt `loom-darwin-arm64` copied to `bin/loom` (Mac has no Go);
pre-build `plan` verified target `loom-guided-dev` via `--json`.

| # | Step | Time | Observed | Pass/Deviation |
|---|---|---|---|---|
| 1 | bootstrap build | 3m08s | created `loom-guided-dev`, lock written, 3 materialized | ✅ (⑥ base image pre-cached, pull free) |
| 2 | doctor | — | 9/13 — ripgrep/gitleaks/golangci-lint/gopls missing; probes answered by HOST tools (Apple git, go darwin/arm64) | ⚠ ⑧ |
| 3 | exec go version | — | `go1.26.4 linux/arm64` from inside the container | ✅ **criterion met** |
| 4 | plan | — | `+0 create, 4 install` outstanding (the 4 doctor-missing tools, `null→latest`) | ❌ expected no drift |
| 5 | plan --json | — | valid JSON, verdict identical to 4 | ✅ invariant holds |
| 6 | rebuild no-op | 2.5s | `converged`, lock untouched | ⚠ fast as expected, but contradicts step 4 → ⑦ |
| 7 | teardown (added by operator) | — | `teardown stop: removed 1 containers, 0 volumes, 0 images`; `loom-guided-dev` gone, other containers untouched; `loom.lock` survives (observation, defensible) | ⚠ ⑨ no confirmation prompt observed |

**Deviations / notes:**
① `loom.yml name:` edit as collision guard — two instances of one project on
a host collide by design (container name derives from playbook `name:`).
② plan-first safety check before build (not in README flow).
③ `bin/` absent in a fresh clone — the "provide a prebuilt binary" path needs
`mkdir -p bin`; no doc says so.
④ binary-provenance gap — a stranger has no way to know where prebuilt
binaries live (no releases/registry pointer).
⑤ `plan` human output doesn't name its target container — safety can't be
verified without `--json`.
⑥ base image pre-cached (host runs loom-dev on the same base) — 3m08s build
time flatters a truly cold machine.
⑦ **plan/build convergence disagreement** — plan reports 4 installs
outstanding; build reports converged. Both can't be true (ADR-0011 /
plan-semantics class). Filed to Writer.
⑧ **doctor probe-scope ambiguity** — doctor reads the host PATH, not the
container; a clean/failed bill doesn't say which environment it grades.
Filed to Writer.
⑨ **teardown executed without a confirmation prompt** — transcript goes
straight from invocation to removal result. The harness deny-floor covers
agents; the verb itself confirmed nothing for a human operator. The
`teardown stop:` message prefix is a minor UX oddity. Filed to Writer.

**Verdict (human, 2026-06-11): Criterion 1 MET.** A fresh project instance
reached a working in-container Go environment (`go1.26.4 linux/arm64` via
`loom exec`) in one guided pass — bootstrap→build 3m08s, rebuild no-op 2.5s,
teardown clean, zero restarts, zero collateral to the two running dev
containers. Met **with findings**: deviations ①–⑥ are documentation/
ergonomics gaps a true stranger would hit (queued as doc fixes); ⑦/⑧/⑨ are
product defects, filed to the Writer. The criterion's substance — one guided
run, working env — stands; the defects are exactly what the run existed to
surface.
