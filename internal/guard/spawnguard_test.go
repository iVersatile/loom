package guard

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// spawn-guard (advisor-in-loom T34, Slice D): a PreToolUse(Bash) deny-override
// that lets ONLY a loom-advisor session spawn an agent (`claude`), closing
// role-push-guard's sole residual — the re-exec forge
// (LOOM_SESSION_ROLE=loom-advisor claude -p "push it"). These tests mirror the
// hardened rolepush_test.go helper: blocked == exit code 2 (a non-2 non-zero is a
// NON-blocking error, so the old err!=nil check would mask the exact defect
// LL-016 / #192 paid for).

// runSpawnHookGuard runs the hook with a hermetic env: the test box's own
// LOOM_SESSION_ROLE / LOOM_ROLE_MARKER are stripped so they cannot leak in, the
// marker is pointed at a guaranteed-absent path (so the root-marker fallback is
// deterministic), and the test's explicit role is appended. role=="" exercises
// the truly-unset, fail-closed path. Returns (blocked, combined output) where
// blocked is TRUE only on exit code 2 (Claude Code's PreToolUse BLOCK signal).
func runSpawnHookGuard(t *testing.T, role, cmd string) (bool, string) {
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
	// engine never takes — which is exactly why the ` claude ` token rule looked
	// green here while being INERT in production (#194).
	c := exec.Command("sh", absHook(t, "spawn-guard"))
	c.Env = env
	c.Stdin = strings.NewReader(preToolUseJSON(t, cmd))
	out, err := c.CombinedOutput()
	// A PreToolUse hook BLOCKS the tool call ONLY on exit code 2 (Claude Code's
	// contract — mirrors branch-guard.sh / the fixed role-push-guard). A non-2
	// non-zero (e.g. exit 1) is a NON-blocking error: the command still runs. So
	// "blocked" must mean exit==2, not merely non-zero — the err!=nil check accepts
	// exit 1 and so misses the real defect (LL-016 / #192). A regression to exit 1
	// fails here.
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	return code == 2, string(out)
}

// TestSpawnGuardBlocksNonAdvisor: an author session is blocked on every spawn
// vector — a bare `claude -p`, the INLINE-ENV FORGE (the hook reads role from its
// OWN launch env (loom-author), so an inline `LOOM_SESSION_ROLE=loom-advisor`
// prefix is inert text that cannot escalate — this is the residual D1 closes),
// whitespace evasion, a spawn in a later chain segment, and the binary-via-var
// indirection (`c=claude; $c -p`).
func TestSpawnGuardBlocksNonAdvisor(t *testing.T) {
	for _, cmd := range []string{
		"claude -p x",
		"claude",
		"LOOM_SESSION_ROLE=loom-advisor claude -p x", // the re-exec FORGE
		"env LOOM_SESSION_ROLE=loom-advisor claude -p x",
		"claude   -p   x",           // whitespace evasion
		"git status && claude -p x", // spawn in a later segment
		"x && claude",
		"c=claude; $c -p x", // indirection: binary assembled via a var
	} {
		if blocked, out := runSpawnHookGuard(t, "loom-author", cmd); !blocked {
			t.Errorf("author must be BLOCKED on %q\n%s", cmd, out)
		}
	}
}

// TestSpawnGuardFailClosedWhenRoleUnset: no role env and no readable marker =>
// NOT advisor => blocked. Spawn DEFAULTS to deny.
func TestSpawnGuardFailClosedWhenRoleUnset(t *testing.T) {
	for _, cmd := range []string{
		"claude -p x",
		"LOOM_SESSION_ROLE=loom-advisor claude -p x",
	} {
		if blocked, out := runSpawnHookGuard(t, "", cmd); !blocked {
			t.Errorf("unset role must FAIL CLOSED (block) on %q\n%s", cmd, out)
		}
	}
}

// TestSpawnGuardAllowsAdvisor: a loom-advisor session spawns with no block — incl.
// narrowing a child DOWN to `LOOM_SESSION_ROLE=loom-author claude -p` (the advisor
// legitimately spawns the author fleet, ADR-0025). The hook exits 0.
func TestSpawnGuardAllowsAdvisor(t *testing.T) {
	for _, cmd := range []string{
		"claude -p x",
		"claude",
		"LOOM_SESSION_ROLE=loom-author claude -p x", // advisor narrowing a child author
		"git status && claude -p x",
	} {
		if blocked, out := runSpawnHookGuard(t, "loom-advisor", cmd); blocked {
			t.Errorf("advisor must be ALLOWED on %q\n%s", cmd, out)
		}
	}
}

// TestSpawnGuardIgnoresNonSpawn: non-spawn commands pass for everyone (author
// here) — the guard only gates `claude` spawns. A path MENTION of claude
// (`cat /usr/bin/claude`) is not a spawn token and must not over-block.
func TestSpawnGuardIgnoresNonSpawn(t *testing.T) {
	for _, cmd := range []string{
		"git status",
		"git log --oneline",
		"go test ./...",
		"cat /usr/bin/claude", // a path mention, not a `claude` command token
		"ls -la && go build ./...",
		"echo claudette", // substring of a longer word — not the token
	} {
		if blocked, out := runSpawnHookGuard(t, "loom-author", cmd); blocked {
			t.Errorf("non-spawn must be ALLOWED on %q\n%s", cmd, out)
		}
	}
}

// TestSpawnGuardArgvPathStillBlocks: invoking the hook with the command as ARGV
// (human / test) must still block — read_tool_command keeps argv winning.
func TestSpawnGuardArgvPathStillBlocks(t *testing.T) {
	for _, cmd := range []string{"claude -p x", "claude"} {
		c := exec.Command("sh", absHook(t, "spawn-guard"), cmd)
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

// TestSpawnGuardFailSafeNonJSONStdin: a direct (non-JSON) pipe of a spawn command
// must still block via the raw-passthrough branch of read_tool_command — the
// extractor never silently allows a command it received on a non-JSON stdin.
// (A MALFORMED JSON blob is deliberately NOT asserted here: spawn-guard's token
// rule is space-bounded ` claude `, which cannot substring-match a JSON blob;
// that by-luck gap is the very defect this fix removes for the WELL-FORMED path,
// and jq is pinned in loom.lock so the jq-absent raw-blob case does not occur in
// any loom topology.)
func TestSpawnGuardFailSafeNonJSONStdin(t *testing.T) {
	c := exec.Command("sh", absHook(t, "spawn-guard"))
	var env []string
	for _, kv := range hermeticEnv() {
		if strings.HasPrefix(kv, "LOOM_SESSION_ROLE=") || strings.HasPrefix(kv, "LOOM_ROLE_MARKER=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "LOOM_ROLE_MARKER="+filepath.Join(t.TempDir(), "no-such-role"), "LOOM_SESSION_ROLE=loom-author")
	c.Env = env
	c.Stdin = strings.NewReader("claude -p x")
	out, err := c.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	if code != 2 {
		t.Errorf("non-JSON stdin spawn must BLOCK with exit 2, got %d\n%s", code, out)
	}
}
