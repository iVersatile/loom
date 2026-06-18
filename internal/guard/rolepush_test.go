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
	c := exec.Command("sh", absHook(t, "role-push-guard"), cmd)
	c.Env = env
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

// TestRolePushGuardBlocksNonAdvisor: an author session is blocked on every
// push / PR vector — raw push over the shared helper, the gh PR subcommands, the
// `gh api` POST vector, whitespace evasion, a push in a later chain segment, and
// crucially the INLINE-ENV FORGE: the hook reads role from its OWN launch env
// (loom-author), so an inline `LOOM_SESSION_ROLE=loom-advisor` prefix is inert
// text that cannot escalate (spike Q1, confer Q2).
func TestRolePushGuardBlocksNonAdvisor(t *testing.T) {
	for _, cmd := range []string{
		"git push",
		"git push origin feat/x",
		"git push --force origin main",
		"gh pr create --fill",
		"gh pr merge --auto 123",
		"gh api -X POST repos/o/r/pulls -f title=x",
		"LOOM_SESSION_ROLE=loom-advisor git push",     // env-prefix forge (git)
		"LOOM_SESSION_ROLE=loom-advisor gh pr create", // env-prefix forge (gh)
		"git  push   origin   main",                   // whitespace evasion
		"git status && git push",                      // push in a later segment
		"p=push; git $p origin",                       // indirection: subcommand assembled via a var (guard-bash parity)
	} {
		if blocked, out := runRolePushGuard(t, "loom-author", cmd); !blocked {
			t.Errorf("author must be BLOCKED on %q\n%s", cmd, out)
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
