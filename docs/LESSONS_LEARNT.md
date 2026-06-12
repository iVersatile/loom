# LESSONS_LEARNT

Append-only, indexed by surface-area tag. Before fixing a bug, grep this file for
the relevant tag; cite `Applying LL-NNN` in the commit body.

## Tag registry
- `schema` · `resolver` · `engine` · `detect` · `cli` · `ci` · `compat` · `cloud`

## LL-012 — A verb must name the machine it grades; verdict and action must share a domain
- Date: 2026-06-11
- Tags: `engine`
- Symptom: the guided run (criterion-1 pass, finding ⑦) hit `build` creating
  a converged container while `plan` immediately reported 4 installs
  outstanding (null→latest) and a second `build` no-op'd. Both could not be
  true. Diagnosis: `plan`'s tool dimension probed the HOST PATH
  (`execProber`) while `build` converged the CONTAINER — on a Mac host
  without ripgrep/gitleaks/golangci-lint/gopls, plan graded the wrong
  machine. `doctor` had the same disease (finding ⑧, fixed separately).
- Lesson: every verb that grades an environment must state — and test —
  WHICH machine it grades. The T5 rule ("the lock pins the container's
  reality, never the build host's") applies to every read verb, not just the
  lock writer. When a verdict verb (plan) and an action verb (build) share a
  dimension, a regression must pin verdict == action on it.
- Fix shape: plan grades the container — live in-container probe when
  running; the lock's container-pinned `resolved` when stopped (plan never
  mutates, so it must not Start to ask); absent container ⇒ create + every
  declared tool is an install (no environment exists to grade — the host
  PATH is never consulted). Regression: `engine.TestPlanGradesContainerNotHost`,
  `engine.TestPlanStoppedContainerUsesLock` (FR-PLAN-003).

## LL-011 — Repo-level hooks fire in EVERY session on a shared tree; role-scoped behavior needs an explicit role check
- Date: 2026-06-11
- Tags: `ci`
- Symptom: the T21 drain Stop hook (`.claude/hooks/drain-inbox.sh`) hardwired
  loom-author's inbox with no role resolution. Registered repo-level on the
  shared bind mount, it fired in an ADVISOR session's stop: the wrong session
  drained the Writer's cargo and flipped item 001 to TAKEN — which the
  Writer's own drain would then have skipped silently. (Advisor contained it:
  AUTOPILOT off, item restored, counter reset; no work was done on the item.)
- Lesson: `.claude/settings.json` hooks travel with the TREE, not with a
  session or role — every session on a shared mount runs them. Any hook whose
  behavior is role-scoped (own-inbox drain, role-specific guards) must resolve
  the session's role explicitly and no-op for everyone else. Same family as
  the branch-guard/worktree and LL-010 shared-tree lessons: shared state +
  per-context behavior requires an explicit context check, never an implicit
  "whoever runs me is the owner".
- Fix shape: `LOOM_SESSION_ROLE` env wins (explicit marker + test seam);
  fallback `id -un` = root ⇒ loom-author (ground truth in loom-dev today;
  revisit at T10 non-root). Regression: `guard.TestDrainRoleGuard*` —
  foreign-role stop with AUTOPILOT on exits 0 and leaves the inbox untouched.

## LL-010 — Git-shelling tests must be hermetic to GIT_* repo-redirection env
- Date: 2026-06-10
- Tags: `ci`
- Symptom: after a commit attempt run with `GIT_DIR`/`GIT_WORK_TREE` set (a
  host-worktree invocation), every git command on every machine sharing the
  repo died: the real `.git/config` had gained
  `core.worktree=/tmp/Test...` and `user.name`/`email=t@example.com`.
- Root cause: the pre-commit gate inherited the caller's `GIT_DIR`/
  `GIT_WORK_TREE`; the guard package's tests shell out to git against
  `/tmp` fixture repos relying on `cmd.Dir`, but **`GIT_DIR` overrides both
  cwd and `-C`** (verified), so `git init`/`config` resolved through the
  leaked gitdir's commondir and wrote into the real shared `.git/config`.
  Hand-repaired by direct file edit + index rebuild.
- Fix (three layers, each sufficient): (1) the gate's targeted scrub (T2
  mechanism) also unsets `GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`,
  `GIT_OBJECT_DIRECTORY`, `GIT_COMMON_DIR`; (2) fixtures build their env from
  `hermeticEnv()` (strips all `GIT_*`, pins `GIT_CONFIG_GLOBAL/SYSTEM` to
  /dev/null) and pass explicit `-C`; (3) regression test
  `TestGateHermeticToGitEnv` poisons `GIT_DIR` at a victim repo and asserts
  its config is byte-identical after the fixture flows.
- Prevention: any test that shells out to git must construct `cmd.Env`
  explicitly — never inherit ambient `GIT_*`; the gate scrub is the backstop,
  not the contract. Same class as LL-006 (ambient override env), opposite
  direction: env that *redirects writes* instead of env that *weakens guards*.

## LL-009 — Cherry-pick already-gated commits onto a new base; don't rebase frozen-contract commits
- Date: 2026-06-09
- Tags: `ci`
- Symptom: rebasing an ADR branch onto an updated `main`
  (`git rebase --onto origin/main <base> docs/adr-0013`) repeatedly stalled — the
  pre-commit gate re-ran for each replayed commit, `protect-paths` blocked the
  frozen-contract (ADR) change, and aborted attempts left `README.md`/`PLAN.md` as
  dirty-tree debris that blocked the next try.
- Root cause: the rebase sequencer re-runs the pre-commit hook per replayed
  commit, so a commit touching `docs/decisions/` or `SPEC-*.md` hits
  `protect-paths` and needs `ALLOW_SPEC_CHANGE`, which is awkward to thread through
  rebase; partial aborts then leave the working tree dirty.
- Fix: `git reset --hard <new-base>` then `git cherry-pick <commits>`. Cherry-pick
  does **not** invoke the pre-commit hook, and the commits were already gated when
  first created, so they re-applied cleanly with no override and no replay drama.
- Prevention: to move already-gated commits that touch frozen contracts onto an
  updated base, prefer reset + cherry-pick over rebase. Caveat: cherry-pick
  bypassing the gate is safe **only** because the content is byte-identical to
  already-gated commits and applies without conflict — if a cherry-pick conflicts
  or the content changes, re-run `make gate` before pushing.

## LL-008 — Unit-tier tests must be hermetic to host-installed tooling
- Date: 2026-06-09
- Tags: `ci` · `cli` · `engine`
- Symptom: `gate` (unit tier) passed locally and in early CI (taking 4–6 min!) but
  **timed out at 120s** once `-timeout 120s` was added. `TestSpecConformance` hung
  inside an `os/exec` call within the `build` verb.
- Root cause: the test runs the *real* `build`, which **provisions a real
  container (compiles gopls)** when docker is present on the host. Local sandboxes
  have no docker, so `build` failed fast and the test looked like a quick unit
  test; CI runners *have* docker, so the "unit" gate silently did minutes of real
  provisioning. The `-timeout` didn't cause the problem — it surfaced the latent
  host-dependence as a hard failure.
- Fix: neuter `PATH` in the test (`t.Setenv("PATH", t.TempDir())`) so docker and
  the prober's tool lookups fail fast; the result JSON shape is still emitted,
  which is all the test asserts.
- Prevention: a unit test that drives a verb touching docker/tools must force that
  path off (empty `PATH`, or an injected fake runtime) so the gate is fast and
  deterministic regardless of what the runner has installed. "Passes locally" is
  not "hermetic" when local lacks a binary CI has (cf. LL-006).

## LL-007 — A job-level `permissions:` block replaces the workflow default
- Date: 2026-06-09
- Tags: `ci`
- Symptom: the `integration` CI job failed at `actions/checkout` with "fatal:
  repository not found" (3 retries, ~33s) before any test ran; `gate` (no
  job-level permissions) checked out fine on the same run.
- Root cause: the job set `permissions: { packages: read }`. A job-level
  permissions block **replaces** the top-level default (it is absolute, not
  additive), so `contents` fell to `none` and the `GITHUB_TOKEN` could not read
  the (private) repo.
- Fix: re-grant `contents: read` alongside `packages: read` on the job.
- Prevention: when narrowing permissions per job, restate **every** scope the job
  needs — `actions/checkout` always needs `contents: read`.

## LL-006 — Hook/guardrail tests must not inherit the ambient override env
- Date: 2026-06-09
- Tags: `ci` · `engine`
- Symptom: `guard.TestProtectPathsBlocksFrozenContract` failed only inside a
  `ALLOW_SPEC_CHANGE=1 git commit …` (the pre-commit gate), not in a bare
  `make test`. Earlier spec commits passed only because `go test` had the guard
  result **cached**; `-count=1` runs exposed it.
- Root cause: `runHook` built the child env as `append(os.Environ(), …)`, so the
  ambient `ALLOW_SPEC_CHANGE` leaked into the "should BLOCK" case — the protect-
  paths guard's own test defeated by the very override it polices. The go test
  cache (which ignores runtime `os.Getenv`) masked the non-determinism.
- Fix: `runHook` strips `ALLOW_*` from the inherited env (`withoutOverrides`); an
  override now only takes effect when a test passes it explicitly.
- Prevention: tests that exec real hooks/tools must control the override
  environment explicitly and never inherit it; don't trust a cached PASS for a
  test whose behaviour depends on `os.Getenv`.

## LL-001 — A signal number says *how* a process died, not *why*
- Date: 2026-06-09
- Tags: `engine` · `ci`
- Symptom: `build` provisioning died with `exit status 137` (and later "container
  is not running"). Assumed memory exhaustion and spent rounds on memory theories.
- Root cause: misdiagnosis. 137 = 128+SIGKILL — it only says the process was
  killed, not by what. `docker stats` showed the VM had **7.4 GiB free**, so it was
  never VM-memory exhaustion. The real causes were elsewhere (apt cache build;
  later a `go install` path error killing the script).
- Fix: stopped inferring from the exit number; reproduced with the **minimal
  vanilla command outside the system under test** (`docker run --rm
  debian:bookworm-slim apt-get update`) and inspected container state
  (`docker inspect -f '{{.State.OOMKilled}} {{.State.ExitCode}}'`).
- Prevention: before acting on a kill/exit code, confirm the *cause* — check
  `State.OOMKilled`, free memory, and a bare repro. Don't theorize past the
  evidence; each probe should eliminate one layer.

## LL-002 — A diagnostic log is only useful if emitted on *every* failure path
- Date: 2026-06-09
- Tags: `engine`
- Symptom: several blind troubleshooting round-trips — the provisioning trace
  existed in `.loom/logs/build.log` but the integration test only surfaced it on
  the go-version assertion, not when `Build` returned an error.
- Root cause: observability gated to one code path; the most common failure (a
  build error) printed nothing.
- Fix: dump `build.log` on any build failure in the test.
- Prevention: a diagnostic log (ADR-0010) must be surfaced at every exit, not the
  happy-ish one. Self-healing / agent-debug-and-fix needs failures legible *where
  they happen*.

## LL-003 — Resilience that swallows the first failure is an anti-pattern
- Date: 2026-06-09
- Tags: `engine`
- Symptom: the real provisioning error was replaced by a misleading "container is
  not running" from the retry's second attempt.
- Root cause: the outer provision retry re-ran `docker exec` against a container
  that had already exited, and returned *that* error instead of attempt 1's trace.
- Fix: detect a dead container, stop retrying, and surface the original failure.
- Prevention: a retry wrapper must preserve/return the first failure and check its
  precondition (is the target still alive?) before re-attempting. Retries hide
  bugs when they don't.

## LL-004 — Verify third-party `go install` module paths against the module's go.mod
- Date: 2026-06-09
- Tags: `engine` · `resolver`
- Symptom: `go install github.com/gitleaks/gitleaks/v8@latest` failed with a
  "version constraints conflict … module declares its path as
  github.com/zricethezav/gitleaks/v8".
- Root cause: the GitHub org (`gitleaks/gitleaks`) ≠ the Go module path; v8's
  go.mod still declares the legacy `github.com/zricethezav/gitleaks/v8`. The
  install path was assumed from the repo URL.
- Fix: install `github.com/zricethezav/gitleaks/v8@latest`.
- Prevention: pin `go install` paths to the module's declared `module` line, not
  the repo URL; module paths drift independently of GitHub org renames.

## LL-005 — Don't run heavy/fragile live provisioning at build time on constrained targets
- Date: 2026-06-09
- Tags: `engine` · `ci`
- Symptom: `apt-get update` failed deterministically on Apple-Silicon Docker
  Desktop (download OK, "Reading package lists…" never completes) with RAM free;
  compiling `gopls` from source is the historical OOM risk on small VMs.
- Root cause: provisioning every container from `debian:bookworm-slim` runs a
  fragile, memory-heavy path (apt package-list cache build + source compiles) on
  whatever VM `build` happens to run on. CI runners are ~7 GB too, so "add RAM"
  doesn't generalize.
- Fix (bridge): `apt-get -o Acquire::Languages=none` trims the cache build (the
  failing step); retries + `GOMEMLIMIT`/`GOMAXPROCS`/`-p=1` bound the Go installs.
- Prevention (durable): bake the toolchain into a prebuilt, digest-pinned base
  image built on a healthy builder (ADR-0012); keep build-time provisioning a thin
  overlay. Reproduce build paths on the *constrained* target, not a beefy dev box.
