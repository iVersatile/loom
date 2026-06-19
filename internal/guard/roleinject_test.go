package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin the role-inject SessionStart hook (2026-06-19): it surfaces the
// session SEAT mechanically (LOOM_SESSION_ROLE wins, else the marker, else
// fail-closed loom-author) so the agent never infers its role from docs, and it
// clarifies that marker!=env is by design (the fail-closed floor), not drift.

// roleInject runs the hook with HOME pinned and extra env (LOOM_SESSION_ROLE,
// LOOM_ROLE_MARKER) appended. Must exit 0.
func roleInject(t *testing.T, home string, extra ...string) string {
	t.Helper()
	// Minimal, fully-controlled env (NOT hermeticEnv, which carries the runner's
	// ambient LOOM_SESSION_ROLE in): only PATH (for `cat`/`sh`) + HOME + the
	// test's explicit role vars. So each test owns the seat resolution.
	c := exec.Command("sh", absHook(t, "role-inject"))
	c.Env = append([]string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}, extra...)
	out, err := c.Output()
	if err != nil {
		t.Fatalf("role-inject must exit 0: %v", err)
	}
	return string(out)
}

// roleMarkerFixture writes a marker file under a temp dir and returns its path.
func roleMarkerFixture(t *testing.T, role string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "role")
	if err := os.WriteFile(p, []byte(role+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The launch env elevates the seat over the marker floor — the exact case that
// failed by inference in the 2026-06-19 confidence session.
func TestRoleInjectEnvWinsOverMarker(t *testing.T) {
	home := t.TempDir()
	marker := roleMarkerFixture(t, "loom-author")
	out := roleInject(t, home, "LOOM_SESSION_ROLE=loom-advisor", "LOOM_ROLE_MARKER="+marker)
	if !strings.Contains(out, "You are: loom-advisor") {
		t.Errorf("env must win over marker; got:\n%s", out)
	}
}

// Unset env falls back to the marker (trust-role floor).
func TestRoleInjectMarkerFallback(t *testing.T) {
	home := t.TempDir()
	marker := roleMarkerFixture(t, "loom-author")
	out := roleInject(t, home, "LOOM_ROLE_MARKER="+marker)
	if !strings.Contains(out, "You are: loom-author") {
		t.Errorf("unset env must fall back to the marker; got:\n%s", out)
	}
}

// No env and no marker → fail closed to loom-author (never the privileged seat).
func TestRoleInjectFailsClosed(t *testing.T) {
	home := t.TempDir()
	out := roleInject(t, home, "LOOM_ROLE_MARKER="+filepath.Join(t.TempDir(), "absent"))
	if !strings.Contains(out, "You are: loom-author") {
		t.Errorf("no env + no marker must fail closed to loom-author; got:\n%s", out)
	}
}

// The output must explain marker!=env is by design — the fix for the "drift"
// misread (a fresh advisor must not try to "correct" the marker to advisor).
func TestRoleInjectClarifiesMarkerIsFloorNotDrift(t *testing.T) {
	home := t.TempDir()
	marker := roleMarkerFixture(t, "loom-author")
	out := roleInject(t, home, "LOOM_SESSION_ROLE=loom-advisor", "LOOM_ROLE_MARKER="+marker)
	if !strings.Contains(out, "not drift") || !strings.Contains(out, "FLOOR") {
		t.Errorf("role-inject must clarify marker=fail-closed-floor / not drift; got:\n%s", out)
	}
}

// The shared HALT sentinel silences the SessionStart surface (same kill-switch as
// checkpoint-inject).
func TestRoleInjectHALTSilences(t *testing.T) {
	home := t.TempDir()
	hooks := filepath.Join(home, ".claude", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "HALT-checkpoint"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if out := roleInject(t, home, "LOOM_SESSION_ROLE=loom-advisor"); strings.TrimSpace(out) != "" {
		t.Errorf("HALT sentinel must silence role-inject; got:\n%s", out)
	}
}
