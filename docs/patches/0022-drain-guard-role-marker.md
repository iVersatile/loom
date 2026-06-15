# Drain-guard role marker — Part 2 apply-steps (T10 PR 4 / ADR-0019 §5)

**Status: READY (Part 1 merged).** Part 1 — the engine role marker — landed in
**PR #154** (2026-06-15): `ContainerSpec.Role` (sourced from the build env
`LOOM_SESSION_ROLE`, charset-validated) + provision writes a root-owned, 0644,
single-line `/var/lib/loom/role` on both create + converge + a `container:user`
doctor claim. The diff below was **re-diffed against the live `drain-inbox.sh`**
on 2026-06-15 (it is a single-line swap, not the multi-line block the original
baseline illustrated). Apply Part 2 after a marker-writing recreate (see
preconditions). Part 3 (the loom-dev non-root flip) remains a deferred topology
call.

## ⚠️ Why the timing is load-bearing
The swap REMOVES the `id -un==root ⇒ loom-author` guess. After it, the role
resolves only via `LOOM_SESSION_ROLE` env → `/var/lib/loom/role` marker →
UNRESOLVED = no-op (fail-closed). So:
- If loom-dev goes **non-root WITHOUT** this patch: `id -un != root` → the current
  guard fails to resolve → the Writer's drain **silently no-ops** → autonomous
  delivery stops.
- If this patch is applied **before** the marker exists (no recreate) **and**
  `LOOM_SESSION_ROLE` is unset: same silent no-op.

The bridge (`export LOOM_SESSION_ROLE=loom-author` in the Writer session) keeps the
env-first path alive and covers both gaps; the marker is the fallback for when the
env is unset (e.g. a spawned worker the slice-4 actuator launches).

## The 3 parts (and order)
1. **Engine (Writer build, non-trust):** `ContainerSpec.Role` + provision writes
   root-owned `/var/lib/loom/role` + `container:user` doctor claim. **DONE — PR #154.**
2. **This patch (trust path, human-applied):** the drain-guard role-resolution swap
   below. `ALLOW_TRUST_CHANGE=1` (protect-paths guards `.claude/hooks/**`; the
   override is human-only — not advisor-self-authorized off an autopilot drain).
3. **loom-dev non-root flip** (`user:` in `loom.yml`): deferred topology decision.
   Part 2 is behavior-neutral while root, so it MAY be applied now to pre-position,
   OR bundled with this flip (when it actually changes behavior).

**Order:** Part 1 merged (#154) → recreate with the marker → THEN Part 2.

## Preconditions (verify ALL before editing the hook)
- [x] Engine Part 1 merged — PR #154.
- [ ] loom-dev recreated **with `LOOM_SESSION_ROLE=loom-author` in the build env**
      (empty role ⇒ no marker), then confirm inside the container:
      - `loom exec loom-dev -- cat /var/lib/loom/role` → `loom-author`
      - `loom exec loom-dev -- stat -c '%U %a' /var/lib/loom/role` → `root 644`
- [ ] `LOOM_SESSION_ROLE=loom-author` set in the Writer session (bridge).

## The exact diff — `.claude/hooks/drain-inbox.sh` (live as of 2026-06-15)
The live role-guard block (lines ~22–25):
```sh
# Role guard (LL-011): own loom-author's inbox only.
role="${LOOM_SESSION_ROLE:-}"
if [ -z "$role" ] && [ "$(id -un 2>/dev/null)" = "root" ]; then role="loom-author"; fi
[ "$role" = "loom-author" ] || exit 0
```
Replace with:
```sh
# Role guard (LL-011): own loom-author's inbox only. Resolution (ADR-0019 §5,
# T10 PR4): LOOM_SESSION_ROLE env → root-owned /var/lib/loom/role marker →
# UNRESOLVED = no-op (fail-closed). The id -un==root guess is gone (it broke
# the moment the container went non-root).
role="${LOOM_SESSION_ROLE:-}"
if [ -z "$role" ] && [ -r /var/lib/loom/role ]; then role=$(cat /var/lib/loom/role 2>/dev/null); fi
[ "$role" = "loom-author" ] || exit 0
```
Only line ~24 changes behavior (the `id -un==root` guess → the marker read); the
comment is updated to past tense. **RE-DIFF again if the hook has moved since.**

## Apply sequence (trust-path)
```sh
git checkout -b fix/0019-pr4-part2-drain-guard-marker
# edit .claude/hooks/drain-inbox.sh per the diff above
ALLOW_TRUST_CHANGE=1 git commit -am \
  "fix: drain-guard resolves role via /var/lib/loom/role marker (ADR-0019 PR4 Part 2)"
git push -u origin fix/0019-pr4-part2-drain-guard-marker
gh pr create ...   # advisor may do branch/edit/push/PR; the ALLOW_TRUST_CHANGE
                   # commit + the merge are the human's acceptance acts.
```

## Verify (after merge + the Writer picks up the new hook)
- `cat /var/lib/loom/role` = `loom-author` (precondition).
- Drain still delivers: watch the next real drain re-surface, or queue a throwaway item.
- ⚠️ **`doctor host:role-marker` does NOT grade while root** — `roleMarkerWired` only
  fires when `pb.User != root`. So while loom-dev is still root, verify Part 2
  **manually** (above); the doctor green flip arrives only at the non-root flip (Part 3).

## Rollback
Revert the one-line change (role resolution back to the `id -un==root` guess) on a
branch, `ALLOW_TRUST_CHANGE=1` commit, merge. Faster interim safety: ensure
`LOOM_SESSION_ROLE=loom-author` is set in the session — the env-first path keeps the
drain alive regardless of the marker.
