# T5 — Lockfile doesn't pin what it claims (host-probed `resolved` + no per-tool digest)   ✅ resolved

Archived from docs/OPEN-THREADS.md on resolution (T21-era convention:
archive on resolution; the stub in OPEN-THREADS carries the pointers).

Origin: inspecting a generated `loom.lock` before committing it; traced into
`internal/lock` + `internal/resolver` + `internal/engine`.

The lockfile is the reproducibility pin (ADR-0002) and SPEC-playbook Q3 froze its
granularity: per-tool `{intent, resolved, source, digest}` **and** a base-image
digest (`SPEC-playbook.md:14-16,125-126`). The producer doesn't meet that, in two
independent ways:

**(a) `resolved` is probed from the build HOST, not the target container.** The
resolver fills `resolved` via a host-PATH version probe (`internal/resolver/resolver.go:55`
→ `internal/engine/probe.go:18-29`; the `found` bool is discarded with `_`). A Mac
build therefore wrote host values into a lock meant to pin a `debian:bookworm-slim`
container:
```yaml
git:  resolved: git version 2.50.1 (Apple Git-155)   # the Mac's git
go:   resolved: go version go1.26.4 darwin/arm64      # the Mac's go
jq:   resolved: jq-1.7.1-apple
gitleaks/gopls/ripgrep/claude-code: resolved: ""      # not on the Mac PATH
```
`ripgrep` is `source: apt` (a container install) yet `resolved: ""` because `rg`
isn't on the host. This is a **correctness bug**: the lock records the wrong machine,
so it can't be committed as a reproducibility artifact regardless of which host
regenerates it. Fix: probe inside the resolved container (or from the install
source), not the host PATH; stop swallowing the not-found bool.

**(b) Per-tool `digest` has a field but no producer.** `LockedTool.Digest` exists
(`internal/lock/lock.go:24`, `json:"digest,omitempty"`) but the only construction
site never sets it (`internal/resolver/resolver.go:56-60`), so it silently vanishes
from YAML. Deliberately deferred in code (`resolver.go:8-9`: "pinned … (later);
Phase 1 records the resolved version"). **But SPEC-playbook overstates reality** —
it says digests are frozen *and* "code implements them … the example already
reflects this shape" (`SPEC-playbook.md:5,18`). Spec↔code drift: either implement
per-tool digests or amend the spec to mark them Phase-2.

Base-image digest is the one part that **works** (`internal/engine/build.go:85-89`,
`container.go:87-96` via `docker buildx imagetools inspect`; `build_test.go:130`).

**Options (no decision yet).**
1. Fix (a) now (container/source probe) — it's a plain correctness bug, no spec
   change; gate a lock-commit on it.
2. Resolve (b) by spec edit: mark per-tool digest Phase-2 in SPEC-playbook so the
   spec stops claiming it's implemented (human-authored, not AI — RULES §5 / C3).
3. Or implement per-tool digest (heavier; pin against the pulled image as the
   resolver comment envisions).

Lean: (1) is a must before any `loom.lock` is committed; (2) is the honest
short-term reconciliation for the digest half. Until both, **do not commit a
generated `loom.lock`.**

**Resolution (2026-06-10, `fix/t5-lock-fidelity`):** options (1)+(2), as leaned.
(a) Build now resolves `resolved` by probing **inside the converged container**
(`ContainerRuntime.Probe` via a login shell; `lock` re-pinned post-provision,
carried forward pre-container so unchanged setups stay no-ops); not-found stays
`""` — never a host value. Covered by **FR-BUILD-007** /
`engine.TestBuildLockRecordsContainerVersions`. (b) SPEC-playbook's lockfile-
granularity decision gained an honest *Phase status*: per-tool `digest` producer
is **Phase 2** (field in schema, not yet populated); base-image digest produced.
A regenerated `loom.lock` is now committable; per-tool digest (option 3) remains
the Phase-2 follow-up.
