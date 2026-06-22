package engine

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/iVersatile/loom/internal/playbook"
)

// TestGuardsOnlyFixtureDropsToolchain is the unit-tier proof for E1 (#75
// mitigation #2): the lighter guards e2e fixture (tempGuardsProject) resolves with
// the Go toolchain DROPPED yet still materializes the role-deny guards the e2e
// drives. The docker e2e that consumes the fixture (TestE2EGuardsBlockByRole) is
// behind //go:build integration and only runs on the CI integration tier — so
// without this test `make gate` would never exercise the change. It guards both
// directions: the fixture must NOT regress to provisioning Go (the OOM weight the
// row removes), AND it must KEEP every guard the e2e asserts.
func TestGuardsOnlyFixtureDropsToolchain(t *testing.T) {
	root := tempGuardsProject(t)
	resolved, err := playbook.Load(filepath.Join(root, "loom.yml"))
	if err != nil {
		t.Fatalf("guards-only fixture must resolve: %v", err)
	}

	// (1) Toolchain dropped — no stack, no Go tool. The Go toolchain (go@1.26 +
	// gopls) comes ONLY from the go stack fragment; the base layer keeps just the
	// lightweight apt tools (git, jq). A future edit that re-adds the toolchain to
	// the guards fixture (re-inflating the OOM surface) fails here.
	if s := resolved.Playbook.Stack; s != "" {
		t.Errorf("guards-only fixture must declare no stack, got %q", s)
	}
	for _, tool := range resolved.Playbook.Tools {
		if strings.HasPrefix(tool, "go") { // go@1.26, gopls, go-anything
			t.Errorf("guards-only fixture must not provision a Go tool, got %q in %v", tool, resolved.Playbook.Tools)
		}
	}

	// (2) Guards survive — declared in the base config/playbook.yml harness: block,
	// which loads independent of the stack, so the e2e still has guards to drive.
	h, ok := resolved.Playbook.Harness["claude"]
	if !ok {
		t.Fatal("guards-only fixture lost harness.claude — guards would not materialize")
	}
	for _, g := range []string{"role-push-guard", "spawn-guard", "segment-split"} {
		if !slices.Contains(h.Hooks, g) {
			t.Errorf("harness.claude.hooks must keep %q, got %v", g, h.Hooks)
		}
	}

	// (3) ...and actually materialize executable — the exact files the e2e asserts
	// at ~/.claude/hooks/<guard> (mirrors TestDogfoodBasePlaybookHarnessMaterializes).
	home := t.TempDir()
	if _, err := materializeHarness(resolved.Source, resolved.Playbook.Harness, home); err != nil {
		t.Fatalf("materialize guards-only harness: %v", err)
	}
	for _, g := range []string{"role-push-guard", "spawn-guard", "segment-split"} {
		hp := filepath.Join(home, ".claude", "hooks", g)
		if fi, err := os.Stat(hp); err != nil || fi.Mode()&0o111 == 0 {
			t.Errorf("guard %q must materialize executable in ~/.claude/hooks: stat=%v err=%v", g, fi, err)
		}
	}
}
