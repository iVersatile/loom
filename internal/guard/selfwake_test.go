package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// selfWakeRoot lays out a loom-dev-shaped seat root (docs/PLAN.md + .scratch/inbox)
// so scripts/self-wake-tick can resolve its standard paths from LOOM_ROOT.
func selfWakeRoot(t *testing.T, queueRow string) string {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".scratch", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	plan := "<!-- BEGIN TACTICAL QUEUE -->\n" +
		"| task | depends-on | serves | owner | status | PR |\n" +
		"| --- | --- | --- | --- | --- | --- |\n" +
		queueRow + "\n" +
		"<!-- END TACTICAL QUEUE -->\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "PLAN.md"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "loom-author.md"), []byte("AUTOPILOT: on\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, ".merged-prs"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func runSelfWakeTick(t *testing.T, root string, env ...string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "scripts", "self-wake-tick"))
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command("sh", abs)
	c.Env = append(append(hermeticEnv(), "LOOM_ROOT="+root), env...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("self-wake-tick: %v\n%s", err, out)
	}
	return string(out)
}

// TestSelfWakeTick pins the shape-A wrapper (adv-076): it resolves the seat's
// standard paths from LOOM_ROOT and surfaces spawn-loop's verdict verbatim — a
// READY allow-listed row ⇒ WAKE; a HALT sentinel ⇒ NO-WAKE.
func TestSelfWakeTick(t *testing.T) {
	t.Run("READY + allow-listed ⇒ WAKE", func(t *testing.T) {
		root := selfWakeRoot(t, "| **demo** [class:exec] | — | agent lifecycle | loom-author | queued | — |")
		out := runSelfWakeTick(t, root, "LOOM_AUTOPULL_CLASSES=exec")
		if !strings.HasPrefix(out, "WAKE delay=") {
			t.Fatalf("want WAKE delay= for a READY allow-listed row, got: %s", out)
		}
		// The refill landed in the seat's own inbox (path resolution worked).
		body, _ := os.ReadFile(filepath.Join(root, ".scratch", "inbox", "loom-author.md"))
		if !strings.Contains(string(body), "status: QUEUED") {
			t.Errorf("self-wake-tick must drive promote-next into the seat inbox:\n%s", body)
		}
	})

	t.Run("HALT ⇒ NO-WAKE", func(t *testing.T) {
		root := selfWakeRoot(t, "| **demo** [class:exec] | — | agent lifecycle | loom-author | queued | — |")
		if err := os.WriteFile(filepath.Join(root, ".scratch", "inbox", "HALT"), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		out := runSelfWakeTick(t, root, "LOOM_AUTOPULL_CLASSES=exec")
		if !strings.HasPrefix(out, "NO-WAKE HALT") {
			t.Fatalf("HALT must short-circuit to NO-WAKE HALT, got: %s", out)
		}
	})
}
