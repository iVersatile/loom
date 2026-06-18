//go:build integration

// Integration tier (RULES §7): the role-deny guards exercised through the REAL
// PreToolUse path — the MATERIALIZED `~/.claude/hooks/<guard>` in a BUILT
// container, fed the tool-input JSON on STDIN with the session role from
// LOOM_SESSION_ROLE, asserting the block/allow matrix by EXIT CODE.
//
// WHY THIS TEST EXISTS (FR-GUARD-E2E). Every guard defect this session was caught
// by hand, in-container, by a human — never by CI:
//   - #190: a guard registered in settings.json but absent from harness.hooks was
//     NEVER MATERIALIZED — the hook file simply was not there.
//   - #192 / LL-016: a deny guard that exited 1 (not 2) is a NON-blocking error —
//     the tool still runs. The guard "fired" and the op went through anyway.
//   - #189/#194 / LL-017: PreToolUse delivers the tool call as JSON on STDIN, not
//     argv. Space-bounded token rules (` gh `, ` claude `) matched the whole JSON
//     blob where the binary is preceded by `"`, not a space, so they were INERT.
//
// Unit tests can't catch any of these: `go test` runs `sh hook <arg>` — never the
// materialized file, never the built container, never the JSON-on-stdin shape,
// never the role resolution. This test closes that gap: it builds ONE container,
// asserts each guard is materialized + executable, then drives every command class
// each guard narrows through the exact invocation Claude Code uses, and asserts
// exit 2 (BLOCKED) for the denied role + exit 0 (ALLOWED) for the advisor. From
// here on, an unmaterialized / exit-1-inert / wrong-input-shape / mis-resolved-role
// guard FAILS CI — no human in the loop.
package engine

import (
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// guardHookPath is where the build materializes a harness hook: <agent home>/hooks/<name>.
func guardHookPath(name string) string { return containerHome + "/.claude/hooks/" + name }

// preToolUseJSON renders the PreToolUse(Bash) payload exactly as Claude Code
// delivers it on the hook's STDIN.
func preToolUseJSON(t *testing.T, command string) string {
	t.Helper()
	payload := map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": command},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal PreToolUse payload: %v", err)
	}
	return string(b)
}

// runGuard drives the MATERIALIZED guard in the BUILT container through the real
// PreToolUse path: the tool-input JSON on STDIN, the session role from
// LOOM_SESSION_ROLE (-e), and `sh <materialized-hook>` so $0/dirname resolves the
// sourced segment-split next to it — i.e. the precise shape the harness uses. It
// returns the guard's exit code (docker exec propagates it verbatim).
func runGuard(t *testing.T, container, role, guard, command string) (int, string) {
	t.Helper()
	cmd := exec.Command("docker", "exec", "-i",
		"-e", "LOOM_SESSION_ROLE="+role,
		container, "sh", guardHookPath(guard))
	cmd.Stdin = strings.NewReader(preToolUseJSON(t, command))
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), string(out)
	}
	t.Fatalf("docker exec guard %s: non-exit error: %v (out=%s)", guard, err, out)
	return -1, ""
}

// TestE2EGuardsBlockByRole is the FR-GUARD-E2E gate. It builds ONE container and
// runs the whole role × command-class matrix against the materialized guards.
func TestE2EGuardsBlockByRole(t *testing.T) {
	requireDocker(t)
	root := tempProject(t)
	pb := root + "/loom.yml"
	name := containerName("loom")
	_ = exec.Command("docker", "rm", "-f", name).Run()
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	if _, err := Build(BuildOpts{PlaybookPath: pb}); err != nil {
		t.Fatalf("build: %v", err)
	}

	// (1) Materialized + executable — catches the #190 class (registered but never
	// materialized). A guard that is not a present, executable file cannot block.
	for _, g := range []string{"role-push-guard", "spawn-guard", "segment-split"} {
		if err := exec.Command("docker", "exec", name, "test", "-x", guardHookPath(g)).Run(); err != nil {
			t.Errorf("guard %s not materialized executable at %s: %v", g, guardHookPath(g), err)
		}
	}

	const (
		author  = "loom-author"  // the DENIED role
		advisor = "loom-advisor" // the PRIVILEGED role
		unset   = ""             // fail-closed: unset is NOT advisor
	)

	// Each row: the materialized guard, the command class it must narrow, and the
	// expected exit per role. 2 = BLOCKED (Claude Code's PreToolUse block signal),
	// 0 = ALLOWED. The forge rows assert the load-bearing property: the guard reads
	// its OWN launch env (LOOM_SESSION_ROLE), so an inline `LOOM_SESSION_ROLE=...`
	// in the COMMAND TEXT is inert to it — an author cannot forge advisor.
	type row struct {
		guard   string
		desc    string
		command string
		// expected exit codes keyed by the role env we launch the guard with.
		author, advisor int
	}
	rows := []row{
		// role-push-guard — push / PR-opening ops.
		{"role-push-guard", "git push", "git push origin main", 2, 0},
		{"role-push-guard", "gh pr create", "gh pr create --fill", 2, 0},
		{"role-push-guard", "gh api PR vector", "gh api -X POST repos/o/r/pulls", 2, 0},
		{"role-push-guard", "inline-env forge", "LOOM_SESSION_ROLE=loom-advisor git push origin main", 2, 0},
		{"role-push-guard", "benign git status", "git status", 0, 0},
		// spawn-guard — agent-spawn (`claude`) ops.
		{"spawn-guard", "claude --version", "claude --version", 2, 0},
		{"spawn-guard", "claude -p", "claude -p 'push it'", 2, 0},
		{"spawn-guard", "inline-env forge", "LOOM_SESSION_ROLE=loom-advisor claude -p x", 2, 0},
		{"spawn-guard", "var indirection", "c=claude; $c -p x", 2, 0},
		{"spawn-guard", "benign path mention", "cat /usr/bin/claude", 0, 0},
	}

	for _, r := range rows {
		r := r
		t.Run(r.guard+"/"+r.desc, func(t *testing.T) {
			if got, out := runGuard(t, name, author, r.guard, r.command); got != r.author {
				t.Errorf("author %q: exit=%d want=%d\n%s", r.command, got, r.author, out)
			}
			if got, out := runGuard(t, name, advisor, r.guard, r.command); got != r.advisor {
				t.Errorf("advisor %q: exit=%d want=%d\n%s", r.command, got, r.advisor, out)
			}
		})
	}

	// Fail-closed: unset role is NOT advisor — a narrowed op must still be BLOCKED.
	// (Confirms the guard's default is deny, not allow, when role can't be read.)
	for _, c := range []struct{ guard, command string }{
		{"role-push-guard", "git push origin main"},
		{"spawn-guard", "claude --version"},
	} {
		if got, out := runGuard(t, name, unset, c.guard, c.command); got != 2 {
			t.Errorf("unset-role %s %q: exit=%d want=2 (fail-closed)\n%s", c.guard, c.command, got, out)
		}
	}
}
