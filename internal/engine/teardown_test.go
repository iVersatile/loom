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
	rt := fakeRuntime{teardownRemoved: Removed{
		Containers: []string{"loom-dev"},
		Volumes:    []string{"loom-dev-data"},
		Images:     []string{},
	}}
	res, err := teardownImpl(TeardownOpts{PlaybookPath: pbPath, Level: "volumes"}, rt, fixedClock, no)
	if err != nil {
		t.Fatalf("teardown: %v", err)
	}
	if res.Level != "volumes" || len(res.Removed.Containers) != 1 || len(res.Removed.Volumes) != 1 {
		t.Errorf("removed = %+v", res.Removed)
	}
	if countLogLines(t, root) == 0 {
		t.Error("teardown should audit removals")
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
