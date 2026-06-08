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

## Known Phase 1 boundary (needs docker to finish + validate)

`build`'s container step (`dockerRuntime.Ensure`) creates the base container and
seeds `$HOME`, but **installing the resolved tool set into the container** (apt /
go install per source) is not yet implemented — it requires a docker environment
to develop and validate, which the dev sandbox lacks. Until then, the built
container is the shared base with materialized config, not yet a fully provisioned
Go toolchain. The host-side spine (resolve → lock → materialize → audit) is fully
implemented and tested.
