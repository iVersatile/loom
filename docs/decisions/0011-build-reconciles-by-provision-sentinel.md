# ADR-0011 — build reconciles by a provision sentinel (presence ≠ converged)
**Date:** 2026-06-09   **Status:** Accepted

## Context
SPEC-verbs `build` is a reconciler: "re-running converges; already-**correct**
items are no-ops." The Phase 1 docker runtime took a shortcut — it treated a
container that merely *exists* (`docker container inspect` succeeds) as converged
and skipped provisioning. On the happy path existence implies a finished
provision, so the gap stayed latent.

An **interrupted build** breaks that coincidence. The provision sequence is
`docker run` → `cp $HOME` → run the provision script (apt → Go tarball →
`go install`). If the process dies between the `cp` and the end of provisioning
(the container is stopped, OOM on a small VM, the host VM goes down — see PLAN
open item on agent-initiated lifecycle), the container exists with `$HOME` seeded
but **no toolchain**. The next `build` (without `--force`) trusts existence and
declares convergence — the container is permanently wedged, escapable only by
`--force` (full rebuild), which is the opposite of *converge*. The integration
test `TestE2EBuildAndSurviveRebuild` surfaced this after a session's container was
stopped mid-provision. Per ADR-0006 / RULES §2, the frozen spec mandates converge,
so **the code was wrong**; this ADR records the *mechanism* that fixes it.

## Decision
The provision script writes, **as its last step**, a sentinel file inside the
container — `/var/lib/loom/provisioned` — containing a digest of the tool set it
just installed (`toolsetDigest`: a sorted, hashed fingerprint of `name|source`).
Because `set -e` aborts the script on any failed install, the sentinel is written
**only on a fully-completed provision**.

On `build`, when the container already exists and `--force` is not set, the
runtime reads the sentinel and compares:
- **absent or different digest** → the container is under-provisioned (interrupted)
  or has drifted (the declared tool set changed) → re-seed `$HOME` and re-run the
  idempotent provision; report container status **`converged`** (audited as
  `container.reconcile`).
- **digest matches** → genuinely converged → no-op, status `exists`.

This makes `build` self-healing for interrupted provisions and drift-aware for
tool-set changes, without a full rebuild.

## Alternatives considered
- **Always re-provision an existing container.** Correct but pays `apt-get update`
  + Go-version check on every build even when nothing changed. The sentinel buys
  the same correctness with a fast steady-state no-op. Rejected as the default.
- **Track provision state host-side (lock / action log).** The lock describes
  *intent*; it cannot witness what actually completed *inside* a container that an
  interrupted run left behind. The witness must live in the container. Rejected.
- **Leave existence == converged and just harden the test.** Hides a real
  reconcile defect behind green CI and leaves the self-heal gap open. Rejected.
- **Freeze the sentinel path/format as a contract.** Over-specs an impl detail
  (cf. ADR-0010 on the diagnostic log). Its path and content are free to change;
  only the *behavior* (build converges an under-provisioned container) is the
  contract. Kept free-form.

## Consequences
- Positive: `build` is a true reconciler; a half-built container heals on the next
  run instead of wedging; tool-set drift triggers re-provision; the integration
  test's survive-rebuild property holds even after an interrupted provision. This
  is the concrete first step toward agent-debug-and-fix (PLAN Phase 3).
- Trade-offs: one extra `docker exec cat` per build against an existing container;
  a new container status value (`converged`) and audit action
  (`container.reconcile`).
- Revisit if: provisioning becomes layered/cached (image build) where the sentinel
  moves into the image, or reconcile needs per-tool granularity rather than a
  whole-set digest.
