package engine

import (
	"os"
	"path/filepath"
	"strings"
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

// TestPathDotfilesSingleOwner pins T4: the PATH lines that used to be ad-hoc
// .profile appends inside the provision script are engine-generated dotfiles
// in ~/.bashrc.d — the single shell-config owner — and emit $HOME, never a
// hardcoded /root (T10 prep). No Go toolchain / no agent ⇒ nothing generated.
func TestPathDotfilesSingleOwner(t *testing.T) {
	home := t.TempDir()

	files := expectedPathDotfiles(
		[]ToolInstall{{Name: "go", Source: "go-tarball"}},
		[]AgentInstall{{Name: "claude-code", Source: "native-installer"}}, home)
	got := map[string]string{}
	for _, f := range files {
		got[f.Display] = string(f.Data)
	}
	goPath, ok := got["~/.bashrc.d/path.go.sh"]
	if !ok {
		t.Fatalf("missing generated path.go.sh, got %v", got)
	}
	for _, want := range []string{"/usr/local/go/bin", "$HOME/go/bin"} {
		if !strings.Contains(goPath, want) {
			t.Errorf("path.go.sh missing %q: %q", want, goPath)
		}
	}
	if strings.Contains(goPath, "/root/") {
		t.Errorf("path.go.sh must use $HOME, never /root: %q", goPath)
	}
	if local, ok := got["~/.bashrc.d/path.local.sh"]; !ok || !strings.Contains(local, "$HOME/.local/bin") {
		t.Errorf("path.local.sh should put $HOME/.local/bin on PATH, got %q (present=%t)", local, ok)
	}

	// go-install tools alone also need the toolchain PATH (mirrors needGo).
	if f := expectedPathDotfiles([]ToolInstall{{Name: "gopls", Source: "go-install"}}, nil, home); len(f) != 1 {
		t.Errorf("go-install source should generate path.go.sh, got %d files", len(f))
	}
	// Nothing Go- or agent-shaped ⇒ no generated config.
	if f := expectedPathDotfiles([]ToolInstall{{Name: "jq", Source: "apt"}}, nil, home); len(f) != 0 {
		t.Errorf("apt-only tool set should generate nothing, got %v", f)
	}
}
