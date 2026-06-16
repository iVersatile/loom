package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin adv-066: the statusline glyph resolves the session role from
// the SAME declarative source as the drain role-guard — the /var/lib/loom/role
// marker (loom.yml role:, PR4) — so the glyph and the drain never disagree and a
// seat launches with NO `export LOOM_SESSION_ROLE`. LOOM_SESSION_ROLE remains an
// env-FIRST override/test-seam (not a launch ritual); LOOM_ROLE_MARKER overrides
// the marker path for the test.

// statuslineFixture builds a git repo with both role inboxes, so the statusline's
// loom-session section renders for whichever role resolves. Returns the repo root.
func statuslineFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	inboxDir := filepath.Join(root, ".scratch", "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"loom-author", "loom-advisor"} {
		if err := os.WriteFile(filepath.Join(inboxDir, role+".md"), []byte("AUTOPILOT: on\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return root
}

// runStatusline executes the real statusline script against the fixture repo with
// extra env (marker path / role override) and returns its rendered line.
func runStatusline(t *testing.T, root string, extraEnv ...string) string {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "config", "dotfiles", "claude", "statusline.sh"))
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command("sh", script)
	c.Dir = root
	// Hermetic role source (LL-006, adv-069): the statusline resolves role
	// env-FIRST, so an ambient LOOM_SESSION_ROLE (the advisor shell exports it —
	// the launch ritual adv-066 is retiring) would override the marker/default
	// under test and fail the gate ONLY in that env (CI's clean env hid it). Drop
	// it from the inherited env; a test that wants the override passes it via
	// extraEnv (TestStatuslineEnvOverridesMarker), which is appended last and wins.
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range hermeticEnv() {
		if strings.HasPrefix(kv, "LOOM_SESSION_ROLE=") {
			continue
		}
		env = append(env, kv)
	}
	c.Env = append(env, extraEnv...)
	c.Stdin = strings.NewReader(`{"worktree":{"original_cwd":"` + root + `"},"model":{"display_name":"M"}}`)
	out, err := c.Output() // statusline is best-effort (no set -e); never errors
	if err != nil {
		t.Fatalf("statusline: %v", err)
	}
	return string(out)
}

// TestStatuslineRoleFromMarker: the marker drives the glyph, with NO
// LOOM_SESSION_ROLE in the environment (the no-launch-ritual path).
func TestStatuslineRoleFromMarker(t *testing.T) {
	root := statuslineFixture(t)
	marker := filepath.Join(t.TempDir(), "role")
	if err := os.WriteFile(marker, []byte("loom-advisor\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := runStatusline(t, root, "LOOM_ROLE_MARKER="+marker)
	if !strings.Contains(out, "🧭") {
		t.Errorf("marker=loom-advisor should render the advisor glyph 🧭; got %q", out)
	}
	if strings.Contains(out, "✍️") {
		t.Errorf("marker=loom-advisor must NOT render the author glyph; got %q", out)
	}
}

// TestStatuslineDefaultWhenMarkerAbsent: no marker and no override ⇒ the
// loom-author default (the marker read fails, the `|| echo loom-author` wins).
func TestStatuslineDefaultWhenMarkerAbsent(t *testing.T) {
	root := statuslineFixture(t)
	absent := filepath.Join(t.TempDir(), "no-such-marker")
	out := runStatusline(t, root, "LOOM_ROLE_MARKER="+absent)
	if !strings.Contains(out, "✍️") {
		t.Errorf("absent marker should default to loom-author (✍️); got %q", out)
	}
}

// TestStatuslineEnvOverridesMarker: LOOM_SESSION_ROLE stays env-FIRST (the
// drain-guard's demoted override/test-seam) — so the glyph mirrors what the
// drain-guard would resolve, never diverging from it.
func TestStatuslineEnvOverridesMarker(t *testing.T) {
	root := statuslineFixture(t)
	marker := filepath.Join(t.TempDir(), "role")
	if err := os.WriteFile(marker, []byte("loom-author\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := runStatusline(t, root, "LOOM_ROLE_MARKER="+marker, "LOOM_SESSION_ROLE=loom-advisor")
	if !strings.Contains(out, "🧭") {
		t.Errorf("LOOM_SESSION_ROLE override should win over the marker (🧭); got %q", out)
	}
}
