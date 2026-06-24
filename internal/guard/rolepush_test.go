package guard

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// role-push-guard (advisor-in-loom T34, Slice A): a PreToolUse(Bash)
// deny-override that lets ONLY a loom-advisor session push / open a PR, narrowing
// the role-blind union push allow. These tests mirror the spike forge assertions
// (.scratch/spikes/advisor-push-enforcement.md): author blocked on every vector
// incl. the inline-env forge; advisor allowed; non-push falls through.

// runRolePushGuard runs the hook with a hermetic env: the test box's own
// LOOM_SESSION_ROLE / LOOM_ROLE_MARKER are stripped so they cannot leak in, the
// marker is pointed at a guaranteed-absent path (so the root-marker fallback is
// deterministic), and the test's explicit role is appended. role=="" exercises
// the truly-unset, fail-closed path. Returns (blocked, combined output).
func runRolePushGuard(t *testing.T, role, cmd string) (bool, string) {
	t.Helper()
	var env []string
	for _, kv := range hermeticEnv() {
		if strings.HasPrefix(kv, "LOOM_SESSION_ROLE=") || strings.HasPrefix(kv, "LOOM_ROLE_MARKER=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "LOOM_ROLE_MARKER="+filepath.Join(t.TempDir(), "no-such-role"))
	if role != "" {
		env = append(env, "LOOM_SESSION_ROLE="+role)
	}
	// Drive the PRODUCTION path: no argv, the command piped as PreToolUse JSON
	// on STDIN. The old helper passed cmd as an arg, exercising a path the
	// engine never takes — which is exactly why the ` gh ` token rule looked
	// green here while being INERT in production (#189/#192).
	c := exec.Command("sh", absHook(t, "role-push-guard"))
	c.Env = env
	c.Stdin = strings.NewReader(preToolUseJSON(t, cmd))
	out, err := c.CombinedOutput()
	// A PreToolUse hook BLOCKS the tool call ONLY on exit code 2 (Claude Code's
	// contract — mirrors branch-guard.sh). A non-2 non-zero (e.g. exit 1) is a
	// NON-blocking error: the command still runs. So "blocked" must mean exit==2,
	// not merely non-zero — the old `err != nil` check accepted exit 1 and so
	// missed the real defect (an author push under exit-1 was NOT blocked,
	// confirmed in-container 2026-06-18). A regression to exit 1 fails here.
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	return code == 2, string(out)
}

// TestRolePushGuardAuthorAllowedForBranchPush (ADR-0017 Decision 1): a loom-author
// session MAY push to feature branches and open PRs — the gh fine-grained PAT is
// shared and the merge gate is enforced separately (see the blocked-for-merge
// test). These vectors must fall through (NOT blocked): raw push, an explicit
// non-main ref, `gh pr create`, the `gh api POST .../pulls` PR-create vector, and
// a branch push in a later chain segment.
func TestRolePushGuardAuthorAllowedForBranchPush(t *testing.T) {
	for _, cmd := range []string{
		"git push",
		"git push origin feat/x",
		"gh pr create --fill",
		"gh api -X POST repos/o/r/pulls -f title=x", // PR create via raw API — not a merge
		"git status && git push",                    // branch push in a later segment
	} {
		if blocked, out := runRolePushGuard(t, "loom-author", cmd); blocked {
			t.Errorf("author must be ALLOWED (branch-push/PR-create) on %q\n%s", cmd, out)
		}
	}
}

// TestRolePushGuardAuthorBlockedForMerge: the merge gate stays — a loom-author is
// NEVER allowed to land work. Every merge / main / force / indirection vector must
// block (exit 2), including the INLINE-ENV FORGE: the hook reads role from its OWN
// launch env (loom-author), so an inline `LOOM_SESSION_ROLE=loom-advisor` prefix is
// inert text that cannot escalate an author to a merge (spike Q1, confer Q2).
func TestRolePushGuardAuthorBlockedForMerge(t *testing.T) {
	for _, cmd := range []string{
		"gh pr merge --auto 123",                         // the merge verb
		"gh api -X PUT repos/o/r/pulls/1/merge",          // the REST merge vector
		"git push --force origin main",                   // force + main
		"git push origin main",                           // direct main push
		"git  push   origin   main",                      // whitespace evasion of the main push
		"LOOM_SESSION_ROLE=loom-advisor gh pr merge 123", // env-prefix forge → merge (inert, stays blocked)
		"p=push; git $p origin",                          // indirection: push assembled via a var → fail-closed block
	} {
		if blocked, out := runRolePushGuard(t, "loom-author", cmd); !blocked {
			t.Errorf("author must be BLOCKED (merge gate) on %q\n%s", cmd, out)
		}
	}
}

// TestRolePushGuardFailClosedWhenRoleUnset: no role env and no readable marker
// => NOT advisor => blocked. Push DEFAULTS to deny (the inversion of
// drain-inbox's author-acts default).
func TestRolePushGuardFailClosedWhenRoleUnset(t *testing.T) {
	for _, cmd := range []string{
		"git push",
		"gh pr create --fill",
	} {
		if blocked, out := runRolePushGuard(t, "", cmd); !blocked {
			t.Errorf("unset role must FAIL CLOSED (block) on %q\n%s", cmd, out)
		}
	}
}

// TestRolePushGuardAllowsAdvisor: a loom-advisor session pushes / opens a PR with
// no block — the hook exits 0 and the union allow auto-approves.
func TestRolePushGuardAllowsAdvisor(t *testing.T) {
	for _, cmd := range []string{
		"git push",
		"git push origin feat/x",
		"gh pr create --fill",
		"gh pr merge --auto 123",
		"gh api -X POST repos/o/r/pulls -f title=x",
	} {
		if blocked, out := runRolePushGuard(t, "loom-advisor", cmd); blocked {
			t.Errorf("advisor must be ALLOWED on %q\n%s", cmd, out)
		}
	}
}

// TestRolePushGuardIgnoresNonPush: non-push commands pass for everyone (author
// here) — the guard only gates push/PR ops; everything else falls through.
func TestRolePushGuardIgnoresNonPush(t *testing.T) {
	for _, cmd := range []string{
		"git status",
		"git log --oneline",
		"git commit -m wip",
		"git fetch origin",
		"git add -A",
		"ls -la && go test ./...",
		"cat /usr/bin/gh",     // a path mention, not a `gh ` command token
		"git log $ref",        // expansion NOT in subcommand position — indirection check must not over-block
		"x=1; git diff $a $b", // tainted (assignment+expansion) but a benign read — must not over-block
	} {
		if blocked, out := runRolePushGuard(t, "loom-author", cmd); blocked {
			t.Errorf("non-push must be ALLOWED on %q\n%s", cmd, out)
		}
	}
}

// runRolePushRaw runs the hook with NO argv, piping rawStdin verbatim (not the
// preToolUseJSON envelope) — for the fail-safe paths where the stdin is not a
// well-formed JSON envelope. Returns (blocked == exit 2, output).
func runRolePushRaw(t *testing.T, role, rawStdin string) (bool, string) {
	t.Helper()
	var env []string
	for _, kv := range hermeticEnv() {
		if strings.HasPrefix(kv, "LOOM_SESSION_ROLE=") || strings.HasPrefix(kv, "LOOM_ROLE_MARKER=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "LOOM_ROLE_MARKER="+filepath.Join(t.TempDir(), "no-such-role"))
	if role != "" {
		env = append(env, "LOOM_SESSION_ROLE="+role)
	}
	c := exec.Command("sh", absHook(t, "role-push-guard"))
	c.Env = env
	c.Stdin = strings.NewReader(rawStdin)
	out, err := c.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	return code == 2, string(out)
}

// TestRolePushGuardArgvPathStillBlocks: a human (or test) that invokes the hook
// with the command as ARGV must still enforce the merge gate — read_tool_command
// keeps argv winning over stdin, so the human-invocation deny path is unbroken.
// (Branch-push is now allowed for an author; the MERGE deny is what must hold here.)
func TestRolePushGuardArgvPathStillBlocks(t *testing.T) {
	for _, cmd := range []string{"gh pr merge --auto 123", "git push --force origin main"} {
		c := exec.Command("sh", absHook(t, "role-push-guard"), cmd)
		var env []string
		for _, kv := range hermeticEnv() {
			if strings.HasPrefix(kv, "LOOM_SESSION_ROLE=") || strings.HasPrefix(kv, "LOOM_ROLE_MARKER=") {
				continue
			}
			env = append(env, kv)
		}
		env = append(env, "LOOM_ROLE_MARKER="+filepath.Join(t.TempDir(), "no-such-role"), "LOOM_SESSION_ROLE=loom-author")
		c.Env = env
		out, err := c.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		if code != 2 {
			t.Errorf("argv path must still BLOCK %q with exit 2, got %d\n%s", cmd, code, out)
		}
	}
}

// TestRolePushGuardFailSafeStdin: the extractor must NEVER silently allow a MERGE
// op on a stdin it cannot cleanly parse. A non-JSON pipe and a MALFORMED JSON blob
// both fall back to the raw bytes, where the contiguous merge/main substring still
// fires — fail-safe (jq-absence takes the same raw-fallback branch). (Branch-push
// is allowed for an author now; the merge gate is what must survive a bad parse.)
func TestRolePushGuardFailSafeStdin(t *testing.T) {
	// NOTE the raw-fallback catches CONTIGUOUS tokens only — `*"git push"*main*`
	// and force fire on the raw bytes, but the space-padded ` gh ` token can't
	// match `gh` when a JSON quote precedes it. That gh-blindness on MALFORMED
	// input is pre-existing (it predates the author tier); the PRODUCTION path
	// (well-formed JSON → jq extract) blocks `gh pr merge` correctly, as the
	// merge test above asserts. So the fail-safe is exercised here with the
	// contiguously-detectable main/force vector.
	for _, raw := range []string{
		"git push --force origin main",                    // non-JSON direct pipe -> raw passthrough; force+main fires
		`{"tool_input":{"command":"git push origin main"`, // malformed JSON -> jq fails -> raw fallback; the contiguous `git push`…`main` still fires
	} {
		if blocked, out := runRolePushRaw(t, "loom-author", raw); !blocked {
			t.Errorf("fail-safe: author merge/main push must be BLOCKED on raw stdin %q\n%s", raw, out)
		}
	}
}
