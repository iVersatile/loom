package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func yes(string) bool { return true }
func no(string) bool  { return false }

func TestTeardownRemovesAndAudits(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	// The fake reports what dockerRuntime would for a clean-state teardown:
	// the container plus the agent-home volume — never a volume the engine
	// didn't ask about (phase-1 review F1: the old fixture fabricated a
	// `loom-dev-data` removal nothing creates).
	rec := &teardownArgs{}
	rt := fakeRuntime{
		teardownRecord: rec,
		teardownRemoved: Removed{
			Containers: []string{"loom-dev"},
			Volumes:    []string{"loom-dev-claude"},
			Images:     []string{},
		},
	}
	res, err := teardownImpl(TeardownOpts{PlaybookPath: pbPath, Level: "volumes", CleanState: true}, rt, fixedClock, no)
	if err != nil {
		t.Fatalf("teardown: %v", err)
	}
	if res.Level != "volumes" || len(res.Removed.Containers) != 1 || len(res.Removed.Volumes) != 1 {
		t.Errorf("removed = %+v", res.Removed)
	}
	// One audit entry per removal — the record matches the report.
	if got, want := countLogLines(t, root), 2; got != want {
		t.Errorf("audit entries = %d, want %d (one per removal)", got, want)
	}
	if rec.name != "loom-dev" || rec.level != "volumes" {
		t.Errorf("runtime asked to teardown %q level %q", rec.name, rec.level)
	}
}

// TestTeardownPassesCleanStateThrough (phase-1 review F1): --clean-state was
// declared, CLI-wired, and never read — a silent no-op over "removes agent
// auth/memory/logs". The engine must hand it to the runtime both ways.
func TestTeardownPassesCleanStateThrough(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	for _, cleanState := range []bool{true, false} {
		rec := &teardownArgs{}
		rt := fakeRuntime{teardownRecord: rec, teardownRemoved: Removed{}}
		if _, err := teardownImpl(TeardownOpts{PlaybookPath: pbPath, Level: "stop", CleanState: cleanState}, rt, fixedClock, no); err != nil {
			t.Fatalf("teardown (cleanState=%t): %v", cleanState, err)
		}
		if rec.cleanState != cleanState {
			t.Errorf("cleanState=%t not passed to the runtime (got %t)", cleanState, rec.cleanState)
		}
	}
}

func TestWipeProjectRequiresConfirmation(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{teardownRemoved: Removed{}}

	// Without a matching confirmation, the project must NOT be wiped.
	_, err := teardownImpl(TeardownOpts{PlaybookPath: pbPath, Level: "reset", WipeProject: true}, rt, fixedClock, no)
	if err == nil {
		t.Fatal("wipe-project without confirmation should error")
	}
	if _, statErr := os.Stat(pbPath); statErr != nil {
		t.Fatalf("project must remain intact when confirmation fails: %v", statErr)
	}
}

func TestWipeProjectWithConfirmation(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{teardownRemoved: Removed{}}

	if _, err := teardownImpl(TeardownOpts{PlaybookPath: pbPath, Level: "reset", WipeProject: true}, rt, fixedClock, yes); err != nil {
		t.Fatalf("teardown: %v", err)
	}
	if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
		t.Errorf("confirmed wipe should remove the project root, stat err = %v", statErr)
	}
}
