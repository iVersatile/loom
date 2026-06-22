package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runSelfWakeDoctor runs scripts/self-wake-doctor against a loom-dev-shaped seat
// root (reusing selfWakeRoot from selfwake_test.go) under the hermetic env.
func runSelfWakeDoctor(t *testing.T, root string, env ...string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "scripts", "self-wake-doctor"))
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command("sh", abs)
	c.Env = append(append(hermeticEnv(), "LOOM_ROOT="+root), env...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("self-wake-doctor: %v\n%s", err, out)
	}
	return string(out)
}

// TestSelfWakeDoctor pins the read-only doctor (adv-076 follow-up): it reports
// the self-wake loop's arming/eligibility WITHOUT side effects — it must NOT mint
// an envelope (no promote-next) nor arm a wake (no self-wake-tick), and its
// VERDICT must track HALT > arming > readiness.
func TestSelfWakeDoctor(t *testing.T) {
	t.Run("read-only — does NOT mint an envelope", func(t *testing.T) {
		root := selfWakeRoot(t, "| **demo** [class:exec] | — | agent lifecycle | loom-author | queued | — |")
		inbox := filepath.Join(root, ".scratch", "inbox", "loom-author.md")
		before, err := os.ReadFile(inbox)
		if err != nil {
			t.Fatal(err)
		}
		out := runSelfWakeDoctor(t, root, "LOOM_AUTOPULL_CLASSES=exec")
		after, err := os.ReadFile(inbox)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Errorf("self-wake-doctor must be side-effect-free; inbox changed:\nbefore=%q\nafter=%q", before, after)
		}
		if strings.Contains(string(after), "status: QUEUED") {
			t.Errorf("self-wake-doctor must NOT mint an envelope:\n%s", after)
		}
		if !strings.Contains(out, "VERDICT:") {
			t.Errorf("doctor must print a VERDICT line, got:\n%s", out)
		}
	})

	t.Run("HALT ⇒ VERDICT BLOCKED", func(t *testing.T) {
		root := selfWakeRoot(t, "| **demo** [class:exec] | — | agent lifecycle | loom-author | queued | — |")
		if err := os.WriteFile(filepath.Join(root, ".scratch", "inbox", "HALT"), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		out := runSelfWakeDoctor(t, root, "LOOM_AUTOPULL_CLASSES=exec")
		if !strings.Contains(out, "VERDICT: BLOCKED") {
			t.Fatalf("HALT must yield VERDICT: BLOCKED, got:\n%s", out)
		}
	})

	t.Run("empty floor ⇒ VERDICT NOT ARMED", func(t *testing.T) {
		root := selfWakeRoot(t, "| **demo** [class:exec] | — | agent lifecycle | loom-author | queued | — |")
		out := runSelfWakeDoctor(t, root)
		if !strings.Contains(out, "VERDICT: NOT ARMED") {
			t.Fatalf("empty LOOM_AUTOPULL_CLASSES must yield VERDICT: NOT ARMED, got:\n%s", out)
		}
	})

	t.Run("armed + READY exec row ⇒ VERDICT ARMED & READY", func(t *testing.T) {
		root := selfWakeRoot(t, "| **demo** [class:exec] | — | agent lifecycle | loom-author | queued | — |")
		out := runSelfWakeDoctor(t, root, "LOOM_AUTOPULL_CLASSES=exec")
		if !strings.Contains(out, "VERDICT: ARMED & READY") {
			t.Fatalf("an armed seat with a READY [class:exec] row must yield VERDICT: ARMED & READY, got:\n%s", out)
		}
	})
}
