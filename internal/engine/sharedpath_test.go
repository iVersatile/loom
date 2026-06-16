package engine

import (
	"strings"
	"testing"
)

// TestHasGoToolchain pins which tool sets need the shared PATH drop-in (adv-065):
// only a Go install (tarball or go-built tool) lands a binary outside the default
// login PATH (/usr/local/go/bin). apt tools (/usr/bin), the relocated harness and
// go-built tools (/usr/local/bin) are already on it.
func TestHasGoToolchain(t *testing.T) {
	cases := []struct {
		name  string
		tools []ToolInstall
		want  bool
	}{
		{"go tarball", []ToolInstall{{Name: "go", Source: "go-tarball"}}, true},
		{"go-built tool", []ToolInstall{{Name: "gopls", Source: "go-install"}}, true},
		{"apt only", []ToolInstall{{Name: "jq", Source: "apt"}}, false},
		{"uv only", []ToolInstall{{Name: "uv", Source: "uv-installer"}}, false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		if got := hasGoToolchain(c.tools); got != c.want {
			t.Errorf("%s: hasGoToolchain=%t want %t", c.name, got, c.want)
		}
	}
}

// TestSharedToolPathScript pins the /etc/profile.d drop-in (adv-065): it puts the
// Go toolchain bin on EVERY login shell's PATH (root probe + non-root runtime user
// alike), root-owned 0644 (read-not-forge), and is empty when no Go is installed.
func TestSharedToolPathScript(t *testing.T) {
	// No Go ⇒ no drop-in: apt/harness/uv tools are already on the default PATH.
	if s := sharedToolPathScript([]ToolInstall{{Name: "jq", Source: "apt"}}); s != "" {
		t.Errorf("no Go toolchain must yield no drop-in, got %q", s)
	}
	if s := sharedToolPathScript(nil); s != "" {
		t.Errorf("empty tool set must yield no drop-in, got %q", s)
	}

	s := sharedToolPathScript([]ToolInstall{{Name: "go", Source: "go-tarball"}})
	for _, want := range []string{
		sharedToolPath,      // writes the /etc/profile.d drop-in
		"/usr/local/go/bin", // the shared toolchain bin it adds to PATH
		`export PATH="$PATH:/usr/local/go/bin"`,
		"chown root:root " + sharedToolPath, // root-owned: a non-root user reads, never forges
		"chmod 0644 " + sharedToolPath,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("sharedToolPathScript missing %q\n---\n%s", want, s)
		}
	}
	// The drop-in lives in /etc (shared, $HOME-independent), never a per-user home
	// dotfile — that home-scoping is exactly what hid the toolchain from the root
	// probe on a non-root container (adv-065).
	for _, never := range []string{"$HOME", "/root", ".bashrc", ".profile\n"} {
		if strings.Contains(s, never) {
			t.Errorf("sharedToolPathScript must not reference %q (it is shared, not home-scoped)\n---\n%s", never, s)
		}
	}
}
