package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestGuardFixtureParity pins the integration fixture's role-deny guards to the
// REAL guards in config/hooks/. The docker-backed e2e (TestE2EGuardsBlockByRole,
// FR-GUARD-E2E) builds the fixture container and drives THESE copies; if they
// drift from the shipped guards the e2e would silently certify a stale guard.
// This unit-tier check (no docker, runs in `make gate`) keeps the copies honest:
// edit config/hooks/<guard> and you must re-copy it into the fixture or fail here.
func TestGuardFixtureParity(t *testing.T) {
	for _, g := range []string{"role-push-guard", "spawn-guard", "segment-split"} {
		real, err := os.ReadFile(filepath.Join("..", "..", "config", "hooks", g))
		if err != nil {
			t.Fatalf("read real guard %s: %v", g, err)
		}
		fixture, err := os.ReadFile(filepath.Join("..", "playbook", "testdata", "proj", "config", "hooks", g))
		if err != nil {
			t.Fatalf("read fixture guard %s: %v", g, err)
		}
		if !bytes.Equal(real, fixture) {
			t.Errorf("fixture guard %s has DRIFTED from config/hooks/%s — re-copy it so the e2e tests the shipped guard", g, g)
		}
	}
}
