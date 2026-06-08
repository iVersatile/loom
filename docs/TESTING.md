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
