package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMutatingVerbsAreObservable is the spec-conformance gate for ADR-0010 /
// SPEC-verbs Cross-cutting "Observable": every mutating verb must produce BOTH an
// audit entry (.loom/actions.log — what changed) AND a diagnostic log
// (.loom/logs/<verb>.log — how it ran). This makes the requirement mechanism, not
// trust: drop either log from build or teardown and this fails.
func TestMutatingVerbsAreObservable(t *testing.T) {
	rootB := tempProject(t)
	if _, err := buildImpl(
		BuildOpts{PlaybookPath: filepath.Join(rootB, "loom.yml")},
		buildProber(), fakeRuntime{ensureInfo: ContainerInfo{Status: "created"}}, fixedClock,
	); err != nil {
		t.Fatalf("build: %v", err)
	}
	requireNonEmpty(t, filepath.Join(rootB, ".loom", "actions.log"))
	requireExists(t, filepath.Join(rootB, ".loom", "logs", "build.log"))

	rootT := tempProject(t)
	if _, err := teardownImpl(
		TeardownOpts{PlaybookPath: filepath.Join(rootT, "loom.yml"), Level: "stop"},
		fakeRuntime{teardownRemoved: Removed{Containers: []string{"loom-loom-dev"}}}, fixedClock, no,
	); err != nil {
		t.Fatalf("teardown: %v", err)
	}
	requireNonEmpty(t, filepath.Join(rootT, ".loom", "actions.log"))
	requireExists(t, filepath.Join(rootT, ".loom", "logs", "teardown.log"))
}

func requireExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file %s: %v", path, err)
	}
}

func requireNonEmpty(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Errorf("expected file %s: %v", path, err)
		return
	}
	if fi.Size() == 0 {
		t.Errorf("expected non-empty file %s", path)
	}
}
