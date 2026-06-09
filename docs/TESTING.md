# Testing — Phase 1

Two tiers, one shared entry point (`make gate` / `make gate-integration`, RULES §7).

## Tiers

- **Unit / gate** (`make gate`, no docker): runs everywhere incl. pre-commit and
  CI. Format, vet, lint, spec-conformance, unit tests, secret scan.
- **Integration** (`make gate-integration`, `-tags integration`): docker-backed
  e2e. Skips cleanly when no daemon is present; runs in CI on a docker host.

## Phase 1 exit criteria → proof

| Exit criterion (PLAN) | Proof |
|---|---|
| Fresh machine → working env in one run | `engine.TestBuildWritesLockMaterializesAndAudits` (host-side: lock + `$HOME` materialized); `engine.TestE2EBuildAndSurviveRebuild` (integration: container + in-container `$HOME`); `engine.TestDoctorChecks`; `bootstrap/loom-bootstrap.sh` smoke |
| Agent can `plan` then `build` unattended | `engine.TestPlanDriftAndConverged` + `cli.TestPlanEmitsJSONAndExitCode` (exit 2/0, valid `--json`, no prompt); `cli.TestSpecConformance` (every verb's `--json` matches SPEC-verbs.md) |
| Guardrails block a destructive test | `guard.TestGuardBashBlocksDangerousAllowsBenign`, `guard.TestBranchGuardBlocksMainAllowsOverrideAndBranch`, `guard.TestProtectPathsBlocksFrozenContract` (real hook scripts, incl. audited override); in-container parity in the integration tier |

## Docker-validated pieces (integration tier)

These run only on a docker host (CI / a one-off `make gate-integration` locally),
never the dev sandbox, which has no daemon:

- **In-container provisioning** (`dockerRuntime.Ensure` → `provisionScript`): apt
  packages, Go from the official tarball, `go install` for gopls/gitleaks, the uv
  installer; `$HOME` seeded and `~/.bashrc` wired to load the prompt. Asserted by
  `TestE2EBuildAndSurviveRebuild` (go usable, gopls present).
- **Manifest-list digest pinning** (`ResolveBaseDigest`): `loom.lock` base image
  is `…@sha256:<index-digest>`, reproducible across arm64/amd64.
- **ghcr base mirror**: CI builds against `ghcr.io/<owner>/loom-base` via
  `LOOM_BASE_IMAGE`; local defaults to Docker Hub.

The host-side spine (resolve → lock → materialize → audit) and the digest/
provisioning *logic* are unit-tested in the normal gate; the docker execution is
validated by the integration tier.

## Timing benchmark

A `-timeout` guards each tier so a *hung* test fails fast instead of sitting at
Go's 10-min default; it is a hang-guard, not the budget.

| Tier | Budget (wall-clock) | Investigate if | Guard |
|---|---|---|---|
| Unit (`make gate` tests) | sub-second/pkg | > 5s total | `-timeout 120s` |
| Integration (`make gate-integration`) | < 5 min | > 8 min | `-timeout 600s` |

The integration tier is heavy because it provisions a real container — compiling
`gopls`/`gitleaks` from source (measured 2026-06-09: `engine` ~122s, `cli` ~71s).
**ADR-0012** (prebuilt base image with the toolchain baked in) is the lever that
drops this from minutes to seconds; treat a rising integration time as a reason to
prioritise it, not to trim tests.

## Coverage baseline

Measured with `make cover` (unit tier — what the gate exercises). Baseline as of
2026-06-09, **total 68.2%**:

| Package | Cov | | Package | Cov |
|---|---|---|---|---|
| `guard` | 100% | | `playbook` | 79.3% |
| `render` | 100% | | `cli` | 77.5% |
| `resolver` | 100% | | `lock` | 76.2% |
| `source` | 81.2% | | `audit` | 74.2% |
| | | | `engine` | 60.5% |
| | | | `cmd/loom` | 0% |

**Floor (soft):** no package regresses below its current %, and total stays ≥ 68%
— coverage ratchets up, never down. Not wired into `gate` as a hard fail on
purpose: `engine`'s 60.5% is understated because its docker paths (`Ensure`,
`provision`, the reconcile branch) only execute under the integration tier, and
`cmd/loom` is the thin `main` wrapper — a hard unit-tier floor would mis-police
both. Check with `make cover` before merging; raise the baseline here when it
improves.
