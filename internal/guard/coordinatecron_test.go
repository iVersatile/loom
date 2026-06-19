package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin scripts/coordinate-cron — the daily host-cron actuator glue for
// the /coordinate standup. It is a thin dispatcher: HALT-first, then inject the
// OPERATOR-supplied command (DRY-RUN when unset). The key property under test is
// ISOLATION FROM THE COLD-START FLOOR — coordinate-cron shares no decision logic
// or mutable state with cold-floor-cron; the only common file is the HALT sentinel,
// read-only by both. HALT must freeze the inject (a HALTed seat gets no auto-input).

func coordinateCronScript(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "scripts", "coordinate-cron"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// runCoordinateCron drives the script under a temp repo. injectCmd=="" => DRY-RUN.
// The sentinel "INJECTED" set marker tells the test whether the inject command ran;
// it returns the log contents and whether the inject fired.
func runCoordinateCron(t *testing.T, halt bool, inject bool) (log string, injected bool) {
	t.Helper()
	dir := t.TempDir()
	haltPath := filepath.Join(dir, "HALT")
	logPath := filepath.Join(dir, ".coordinate-log")
	sentinel := filepath.Join(dir, "INJECTED")
	if halt {
		if err := os.WriteFile(haltPath, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := append(hermeticEnv(),
		"LOOM_REPO="+dir,
		"LOOM_HALT="+haltPath,
		"LOOM_COORDINATE_LOG="+logPath,
	)
	if inject {
		// A harmless inject that proves it ran by creating the sentinel.
		env = append(env, "LOOM_COORDINATE_INJECT_CMD=touch "+sentinel)
	} else {
		env = append(env, "LOOM_COORDINATE_INJECT_CMD=")
	}
	c := exec.Command("sh", coordinateCronScript(t))
	c.Env = env
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("coordinate-cron: %v\n%s", err, out)
	}
	b, _ := os.ReadFile(logPath)
	_, statErr := os.Stat(sentinel)
	return string(b), statErr == nil
}

// HALT present => no inject, even with an inject command set (HALT-first).
func TestCoordinateCronHaltBlocks(t *testing.T) {
	log, injected := runCoordinateCron(t, true /*halt*/, true /*inject*/)
	if injected {
		t.Error("HALT must block the standup inject (a HALTed seat gets no auto-input)")
	}
	if !strings.Contains(log, "HALT-blocked") {
		t.Errorf("expected a HALT-blocked log line, got: %q", log)
	}
}

// No inject command => DRY-RUN: log only, inject nothing.
func TestCoordinateCronDryRun(t *testing.T) {
	log, injected := runCoordinateCron(t, false /*halt*/, false /*inject*/)
	if injected {
		t.Error("dry-run (no inject cmd) must inject nothing")
	}
	if !strings.Contains(log, "dry-run") {
		t.Errorf("expected a dry-run log line, got: %q", log)
	}
}

// Inject command set, no HALT => the inject runs exactly once.
func TestCoordinateCronInjects(t *testing.T) {
	log, injected := runCoordinateCron(t, false /*halt*/, true /*inject*/)
	if !injected {
		t.Error("expected the operator inject command to run")
	}
	if !strings.Contains(log, "COORDINATE (inject)") {
		t.Errorf("expected an inject log line, got: %q", log)
	}
}

// Isolation guard: coordinate-cron must not read or write any cold-floor state
// (the drain inbox, .merged-prs, .cold-floor-log). A run with a clean repo dir must
// create ONLY its own log + (when injected) the sentinel — never a cold-floor file.
func TestCoordinateCronTouchesNoColdFloorState(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, ".coordinate-log")
	env := append(hermeticEnv(),
		"LOOM_REPO="+dir,
		"LOOM_HALT="+filepath.Join(dir, "HALT"),
		"LOOM_COORDINATE_LOG="+logPath,
		"LOOM_COORDINATE_INJECT_CMD=",
	)
	c := exec.Command("sh", coordinateCronScript(t))
	c.Env = env
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("coordinate-cron: %v\n%s", err, out)
	}
	for _, forbidden := range []string{".cold-floor-log", ".merged-prs", "loom-author.md"} {
		if _, err := os.Stat(filepath.Join(dir, forbidden)); err == nil {
			t.Errorf("coordinate-cron must not create cold-floor state %q", forbidden)
		}
	}
}
