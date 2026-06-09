package engine

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// TestDotfilesLaterTierReplacesSameTarget proves FR-SCHEMA-007: dotfiles resolve by
// layer order, and a later reference mapping to the SAME $HOME target path replaces
// the earlier file (whole-file later-wins, SPEC-playbook#dotfiles-resolution). Both
// refs map to ~/.bashrc.d/prompt.sh (dotfileTarget keys bash/* on the basename), so
// the later one (overlay) must win.
func TestDotfilesLaterTierReplacesSameTarget(t *testing.T) {
	src := fstest.MapFS{
		"dotfiles/bash/base/prompt.sh":    {Data: []byte("BASE\n")},
		"dotfiles/bash/overlay/prompt.sh": {Data: []byte("OVERLAY\n")},
	}
	home := t.TempDir()
	refs := []string{"bash/base/prompt.sh", "bash/overlay/prompt.sh"} // base then overlay
	if _, err := materializeDotfiles(src, refs, home); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(home, ".bashrc.d", "prompt.sh"))
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "OVERLAY\n" {
		t.Errorf("later tier must replace the same target: got %q, want %q", got, "OVERLAY\n")
	}
}
