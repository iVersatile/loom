# LESSONS_LEARNT

Append-only, indexed by surface-area tag. Before fixing a bug, grep this file for
the relevant tag; cite `Applying LL-NNN` in the commit body.

## Tag registry
- `schema` · `resolver` · `engine` · `detect` · `cli` · `ci` · `compat` · `cloud`

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
