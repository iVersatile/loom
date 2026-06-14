# Prepared patch — protect-paths: extend the trust class to `config/hooks/**`

**Trust path / guard change → human-applied.** Closes the red-team gap from PR
#138: `config/hooks/resurface-decide` holds the injection-proof predicate
evaluator (security-sensitive — it gates what auto-delivers to an auto-mode
agent), but protect-paths' trust class was `.claude/hooks/**` + `.claude/settings*`
only, leaving `config/hooks/**` agent-editable. The same gap left the **git-side
guards themselves** (`protect-paths`, `guard-bash`, `branch-guard`,
`segment-split`) un-self-protected. This extends the trust class to cover them.

## How to apply
The FIRST commit runs under the *current* protect-paths (which does NOT yet
guard `config/hooks/**`), so it needs no override; after it lands, every later
`config/hooks/**` edit requires `ALLOW_TRUST_CHANGE=1` (audited). Human hands.

```sh
# edit config/hooks/protect-paths per the diff below, then:
git commit -m "fix: protect-paths guards config/hooks/** (close the resurface-decide + git-guard self-protection gap)"
```

## Diff — `config/hooks/protect-paths`
```diff
-#   TRUST CONFIG (.claude/settings*.json, .claude/hooks/**) — the harness's
+#   TRUST CONFIG (.claude/settings*.json, .claude/hooks/**, config/hooks/**) —
+#   the harness's
@@ is_trust matcher @@
 	case "$1" in
 	.claude/settings*.json) return 0 ;;
 	.claude/hooks/*) return 0 ;;
+	config/hooks/*) return 0 ;;
 	*) return 1 ;;
 	esac
```

## Why this is the right fix (vs moving the evaluator back inline)
- Keeps the Writer's cleaner factoring (thin drain *calls* `resurface-decide`).
- Protects the evaluator AND the pre-existing blind spot (the git-side guards)
  in one move — defense in depth, "guardrails are mechanism not trust".
- After this lands, weakening `resurface-decide` (e.g. to eval the raw predicate
  and re-open the R2 injection) is denied at commit time without an audited
  `ALLOW_TRUST_CHANGE`, the same floor as the agent hooks.

## Sequencing
Apply this **with** the revised drain patch (`0020-drain-park-resurface.md`,
thin version) and only after PR #138's guard tests are green — so the loop goes
live only once `resurface-decide` is both correct AND guarded.
