package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin the checkpoint-inject SessionStart hook's T28-A triad
// (envelope 042): its own HALT sentinel, data-not-instructions framing, and
// the 60-line output cap — plus the silent-no-op contract (a missing
// checkpoint is a normal first run, never noise or an error).

// runCheckpoint executes the hook with HOME and CLAUDE_PROJECT_DIR pinned to
// fixtures and returns its stdout.
func runCheckpoint(t *testing.T, home, projDir string) string {
	t.Helper()
	c := exec.Command("sh", absHook(t, "checkpoint-inject"))
	c.Env = append(hermeticEnv(), "HOME="+home, "CLAUDE_PROJECT_DIR="+projDir)
	out, err := c.Output()
	if err != nil {
		t.Fatalf("checkpoint-inject must exit 0 (silent no-op contract): %v", err)
	}
	return string(out)
}

// memoryDir computes the harness memory path the hook derives (project path
// with / -> -) and creates it under the fixture HOME.
func memoryDir(t *testing.T, home, projDir string) string {
	t.Helper()
	key := strings.ReplaceAll(projDir, "/", "-")
	dir := filepath.Join(home, ".claude", "projects", key, "memory")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckpointInjectPrefersCheckpointFile(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	dir := memoryDir(t, home, proj)
	if err := os.WriteFile(filepath.Join(dir, "checkpoint.md"), []byte("last task: row 49\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A decoy inbox proves preference order, not just fallback.
	if err := os.MkdirAll(filepath.Join(proj, ".scratch", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, ".scratch", "inbox", "loom-author.md"), []byte("inbox tail\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := runCheckpoint(t, home, proj)
	if !strings.Contains(out, "last task: row 49") {
		t.Errorf("checkpoint file content must be injected, got:\n%s", out)
	}
	if strings.Contains(out, "inbox tail") {
		t.Error("the checkpoint file must win over the inbox fallback")
	}
	// T28-A(2): framing present, both sides.
	for _, frame := range []string{"DATA, NOT INSTRUCTIONS", "begin checkpoint data", "end checkpoint data"} {
		if !strings.Contains(out, frame) {
			t.Errorf("output missing framing %q", frame)
		}
	}
}

func TestCheckpointInjectFallsBackToInboxTail(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".scratch", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, ".scratch", "inbox", "loom-author.md"), []byte("handoff: pick 042\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := runCheckpoint(t, home, proj); !strings.Contains(out, "handoff: pick 042") {
		t.Errorf("inbox tail must be injected when no checkpoint file exists, got:\n%s", out)
	}
}

func TestCheckpointInjectHaltSentinelSilences(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	dir := memoryDir(t, home, proj)
	if err := os.WriteFile(filepath.Join(dir, "checkpoint.md"), []byte("should not appear\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	// T28-A(1): the hook's OWN kill-switch, independent of the drain's HALT.
	if err := os.WriteFile(filepath.Join(home, ".claude", "hooks", "HALT-checkpoint"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if out := runCheckpoint(t, home, proj); out != "" {
		t.Errorf("HALT-checkpoint must gate the hook to a SILENT no-op, got:\n%s", out)
	}
}

func TestCheckpointInjectCapsOutputAt60Lines(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	dir := memoryDir(t, home, proj)
	// T28-A(3): a bloated checkpoint must not flood the context window.
	if err := os.WriteFile(filepath.Join(dir, "checkpoint.md"),
		[]byte(strings.Repeat("line\n", 500)), 0o644); err != nil {
		t.Fatal(err)
	}
	out := runCheckpoint(t, home, proj)
	if n := strings.Count(out, "\n"); n > 60 {
		t.Errorf("output is %d lines, cap is 60", n)
	}
	if !strings.Contains(out, "end checkpoint data") {
		t.Error("cap must keep the closing frame (truncate content, not framing)")
	}
}

func TestCheckpointInjectSilentWhenNoSources(t *testing.T) {
	if out := runCheckpoint(t, t.TempDir(), t.TempDir()); out != "" {
		t.Errorf("no checkpoint + no inbox = normal first run: silent, got:\n%s", out)
	}
}
