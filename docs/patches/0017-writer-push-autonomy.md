# Patch 0017 — Writer push-to-branch autonomy (clears the "036" blocker)

> Status: **DRAFTED by advisor 2026-06-23; awaiting human apply.** This is a
> TRUST-PATH change (`config/hooks/**` + `.claude/settings.json`) → it must be
> committed with **`ALLOW_TRUST_CHANGE=1`** (human-only; separate from
> ALLOW_SPEC_CHANGE — protect-paths keeps the two authorizations distinct).
> Realizes ADR-0017 Decision 1. Re-diff against live files before applying.

## What this actually clears (the misnomer corrected)

ADR-0017 calls the blocker "the 036 credential blocker." Verified 2026-06-23 —
that framing is **partly wrong**:

- The gh credential is **already provisioned and shared.** `gh auth status` →
  logged in as `iVersatile` via a **fine-grained PAT** (`github_pat_`) stored at
  `~/.config/gh/hosts.yml` (no env var), live since 2026-06-17. Both seats run as
  the same `loom` user in the same container (`loom.yml:48` `user: loom`,
  `HOME=/home/loom`), so they **share the same token**. The advisor pushes with it
  daily. **No second `gh auth login` is needed** — the ADR-0026 "human must
  provision" note predates the 2026-06-17 provisioning.
- "036 proper" (the `.claude.json` trust-flag durability hole) was already fixed
  by ADR-0018 (#113).

So the Writer is **not missing a credential.** The only thing distinguishing
advisor-can-push from author-cannot is **two deliberate config gates**, both fixed
here. The merge gate ("no seat merges its own work") is preserved.

## The two gates, and the fix

### Gate 1 — `config/hooks/role-push-guard` (the hard deny)

Today (`:106`): only `loom-advisor` is exempt; every other role (incl. the author
default) hits `exit 2` for ANY `git push` / `gh` op. Add a **`loom-author` tier**:
may push to branches + open PRs, but NEVER merge. Two backstops keep main safe —
(a) GitHub branch protection rejects direct pushes to `main` server-side, and
(b) this guard blocks client-side merge verbs.

Replace the single advisor exemption:

```sh
[ "$role" = "loom-advisor" ] && exit 0
```

with the tiered form:

```sh
[ "$role" = "loom-advisor" ] && exit 0

# loom-author: branch-push + PR-create allowed; MERGE always denied (the merge
# gate stays — no seat merges its own work). Fail-closed: is_merge_op matches
# broadly; anything uncertain is treated as a merge and blocked.
if [ "$role" = "loom-author" ]; then
	if is_merge_op "$cmd"; then
		echo "role-push-guard: BLOCKED merge op for loom-author (push/PR-create OK, merge stays advisor/human): $raw" >&2
		exit 2
	fi
	exit 0
fi
```

Add the classifier next to `is_push_op` (same fail-closed, segment+whole-line
discipline):

```sh
# is_merge_op — a merge-to-main / land op the author must NEVER do. Deny broadly:
#   gh pr merge ...            (the obvious verb)
#   gh api ... /merge | /merges (the REST merge vector, any -X)
#   git push ... main | HEAD:main | :main | --force/-f (belt-and-suspenders;
#     branch protection also blocks these server-side)
is_merge_op() {
	case " $1 " in
	*" gh "*" merge"*) return 0 ;;   # gh pr merge / gh api .../merge
	esac
	case "$1" in
	*"/merge"*|*"/merges"*) return 0 ;;
	*"git push"*"main"*) return 0 ;;
	*"git push"*" -f"*|*"git push"*"--force"*) return 0 ;;
	esac
	return 1
}
```

> Apply the same whole-line + per-segment evaluation `is_push_op` already gets
> (loop over `split_segments` and the whole `$cmd`), so a chained `... && gh pr
> merge` is caught. Under `indirection_taint`, treat a tainted author push/`gh`
> op as a merge (fail-closed) rather than allowing it.

### Gate 2 — the author seat's outward allow-list

`.claude/settings.json` (the shared/author floor) grants no push/PR verbs. Add
exactly the two the Writer needs (NOT merge):

```json
"Bash(git push:*)",
"Bash(gh pr create:*)",
```

(The advisor keeps its broader set in its own user settings; this only lifts the
author floor to branch-push + PR-create. `role-push-guard` remains the load-
bearing gate — the allow-list just stops the per-op prompt.)

## After applying (buildable, normal flow)

1. **Guard test** — extend `internal/guard/*push*_test.go`: assert `loom-author`
   is ALLOWED for `git push origin feat/x` and `gh pr create`, and BLOCKED for
   `gh pr merge`, `gh api -X PUT .../merge`, `git push origin main`, and the
   chained/indirection variants. Advisor stays fully allowed; unset/other roles
   stay fully blocked.
2. **FR-PUSH-001** — register the new behavior in `docs/FR-registry.yml` linking
   that test (closes the "no FR-PUSH" gap noted in OPEN-REQUIREMENTS §2).
3. **Verify the merge gate holds** — the author still cannot land on main
   (server-side protection + `is_merge_op`).

## Sequencing / risk

- **Blast radius:** the author gains branch-push + PR-create only; merge stays
  advisor/human. Reverting is stricter-direction (drop the author tier) → safe.
- This unblocks **Chain C** (Writer-push → T18 multi-agent perms) in
  OPEN-REQUIREMENTS §7.
- Related but SEPARATE: the ADR-0017 tiered-merge amendment (adv-068) is about
  *who auto-merges*; this patch is only about *author push-to-branch*. Apply
  independently.
