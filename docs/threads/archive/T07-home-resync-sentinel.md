# T7 — `build` converge skips container `$HOME` re-sync on dotfile-only change   ✅ resolved

Archived from docs/OPEN-THREADS.md on resolution (T21-era convention:
archive on resolution; the stub in OPEN-THREADS carries the pointers).

**Resolution (2026-06-10, `fix/t7-home-resync`):** option (1) — a **home
sentinel** (`/var/lib/loom/home`), the ADR-0011 pattern applied to the $HOME
surface ADR-0015 materializes. `homeDigest()` fingerprints the staging tree
(rel path + mode + content); `needsHomeSync()` mirrors `needsReprovision`;
`Ensure` converges on either sentinel going stale. Home drift triggers only
the `docker cp` + sentinel write — provision stays gated on the toolset digest
(a dotfile change never re-runs apt/go-install; the T4 interplay note held).
The misleading-status defect (option 3) dissolves by construction: "exists"
now means nothing needed syncing. Tests: `TestHomeDigestDetectsDotfileChange`,
`TestNeedsHomeSync` (unit), `TestE2EDotfileChangeConverges` (integration:
edit → rebuild → content in container + status converged → third build
"exists"); linked from FR-BUILD-004. One-time effect on merge: existing
containers have no home sentinel, so their next build re-syncs $HOME once.
Entry kept below for the original analysis.

Origin: a real change (richer `claude/statusline.sh`) was committed, built, and
reported `converged … 3 materialized` — but a session **inside** `loom-loom-dev`
still saw the old statusline. `docker exec … cat /root/.claude/statusline.sh`
confirmed the container kept the old file while host staging had the new one.

**Root cause (confirmed in code).** The container `$HOME` re-sync is gated on the
**toolset digest only**, so a dotfile-only change never triggers it:
- `toolsetDigest` hashes only `tools` (`Name|Source`) — nothing about dotfiles
  (`internal/engine/container.go:228-239`).
- In `Ensure`, an existing container with an unchanged toolset **early-returns
  `"exists"` and skips the `docker cp` of `$HOME`** (`container.go:119-120`). The
  reconcile copy (`container.go:122-126`) sits *after* that return, so it runs only
  when tools changed or a prior provision was interrupted (`needsReprovision`).
- Result: changed dotfiles reach host staging `.loom/home` but never an
  already-built container. Only `--force`/teardown (the create path,
  `container.go:138-141`) copies `$HOME`.

**Misleading status message (second defect).** `build` prints `converged … N
materialized` driven by the **host** `changed` flag (`materializeDotfiles` wrote
staging, `build.go:113-125,172-173`), even though `Ensure` returned `"exists"` and
pushed nothing to the container. `build.go`'s status switch has no `"exists"` case
(`build.go:146-166`), so the message reports staging state, not container state — a
user reasonably reads "3 materialized" as "3 files are now in my container."

**Why it matters.** This is the most user-visible of the current bug cluster: the
declared-desired-state promise (edit config → `build` → container reflects it) is
silently broken for the entire dotfiles surface (statusline, prompt, claude
settings, any future `bash/*`). The only way to apply a dotfile edit today is a
destructive full rebuild.

**Options (no decision yet).**
1. Extend the reconcile trigger: fold a **dotfiles/home digest** into the sentinel
   (or compare staging vs container) so `needsReprovision` (rename → `needsConverge`)
   also fires on `$HOME` drift; always `docker cp` staging when it differs. Cheap,
   non-destructive, matches reconcile intent.
2. Always `docker cp` staging on every build (idempotent; cheap for a few files) and
   keep the provision (tool install) gated on the toolset digest as today.
3. At minimum, fix the **message**: add an `"exists"` status case so `build` does
   not claim `converged/N materialized` when nothing reached the container.

Lean: (1) or (2) for the real fix + (3) regardless (honest status). Note the
`len(spec.Tools) > 0` gate on the `.bashrc.d` sourcing loop (T4) interacts here:
the home-sync fix should not depend on tools being present.

Promote to: an engine bugfix (home-drift reconcile) + a status-accuracy fix, with a
regression test (dotfile edit on an existing container → file present in container,
status reflects it); FR once `verify` covers the build reconcile path.
