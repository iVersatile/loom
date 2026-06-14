# Prepared patch (BASELINE, DEFERRED) — drain-guard role marker (T10 PR 4 / ADR-0019 §5)

**Status: DEFERRED — baseline only.** Captured 2026-06-14 as context + a starting
point. **Do NOT apply yet.** loom-dev runs as **root** today, so the drain-guard's
current `id -un==root ⇒ loom-author` fallback works and the verified loop relies
on it. This patch is needed only **when loom-dev goes non-root** (a `user:` is set
in `loom.yml`). **Expect to UPDATE this patch + the steps when that moment comes**
— the drain hook will have evolved (it's already the thin orchestrator now), so
re-diff against the live hook before applying.

## ⚠️ Why the timing is load-bearing
The swap REMOVES the `id -un==root ⇒ loom-author` guess. After it, the role
resolves only via `LOOM_SESSION_ROLE` env → `/var/lib/loom/role` marker →
UNRESOLVED = no-op (fail-closed). So:
- If loom-dev goes **non-root WITHOUT** this patch: `id -un != root` → the current
  guard fails to resolve → the Writer's drain **silently no-ops** → autonomous
  delivery stops.
- If this patch is applied **before** the marker exists (no engine Part 1 +
  recreate, and `LOOM_SESSION_ROLE` unset): same silent no-op.

Both directions break the loop the same way. That's the trigger this patch's
reminder watches for.

## The 3 parts (and order) — see the T10 queue row
1. **Engine (Writer build, non-trust):** `ContainerSpec.Role` (loom-dev overlay,
   never a playbook key) + provision writes `/var/lib/loom/role` (root-owned) +
   doctor `container:user`.
2. **This patch (trust path, human-applied):** the drain-guard role-resolution
   swap below.
3. **Recreate** loom-dev so provision writes the marker.
**Order:** Part 1 merges → recreate (marker exists) → THEN this patch. Bridge:
`export LOOM_SESSION_ROLE=loom-author` in the Writer session covers the gap.

## Proposed diff — `.claude/hooks/drain-inbox.sh` (RE-DIFF against the live hook first)
Today's role-resolution block (the `Resolution:` comment region) reads roughly:
```sh
role="${LOOM_SESSION_ROLE:-}"
if [ -z "$role" ] && [ "$(id -un 2>/dev/null)" = "root" ]; then
	role="loom-author"
fi
[ "$role" = "loom-author" ] || exit 0
```
Replace with:
```sh
# Role resolution (T10 PR 4, ADR-0019 §5): LOOM_SESSION_ROLE env (explicit +
# test seam) → the root-owned /var/lib/loom/role marker (provision-written) →
# UNRESOLVED = no-op (fail-closed). The old id -un==root guess is gone — it
# breaks the moment the container is non-root.
role="${LOOM_SESSION_ROLE:-}"
if [ -z "$role" ] && [ -r /var/lib/loom/role ]; then
	role=$(cat /var/lib/loom/role 2>/dev/null)
fi
[ "$role" = "loom-author" ] || exit 0
```
Update the `Revisit at T10…` comment above the block to past tense. Apply as a
trust-path human diff: `ALLOW_TRUST_CHANGE=1 git commit …`, after the engine
marker exists.

## Apply checklist (when the moment comes)
- [ ] Engine Part 1 merged (marker + ContainerSpec.Role + doctor claim).
- [ ] loom-dev recreated; confirm `/var/lib/loom/role` exists = `loom-author`.
- [ ] RE-DIFF this block against the current `drain-inbox.sh` (it will have moved).
- [ ] `LOOM_SESSION_ROLE=loom-author` set in the Writer session (bridge).
- [ ] Apply the swap; `loom doctor` clean; verify the drain still re-surfaces.
