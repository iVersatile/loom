package resolver

import (
	"testing"

	"github.com/iVersatile/loom/internal/playbook"
)

type fakeProbe map[string]string

func (f fakeProbe) Version(binary string) (string, bool) {
	v, ok := f[binary]
	return v, ok
}

func TestResolveSourcesAndPins(t *testing.T) {
	pb := &playbook.Playbook{
		Tools:  []string{"go@1.26", "git", "uv", "gopls", "ripgrep"},
		Agents: []string{"claude-code"},
	}
	vp := fakeProbe{
		"go":          "1.26.4",
		"git":         "2.43",
		"uv":          "0.4.1",
		"gopls":       "0.16",
		"rg":          "14.1", // ripgrep -> rg alias
		"claude-code": "1.2.3",
	}
	r := Resolve(pb, vp)

	wantSource := map[string]string{
		"go":      "base-image",
		"git":     "apt",
		"uv":      "uv-installer",
		"gopls":   "go-install",
		"ripgrep": "apt",
	}
	for tool, src := range wantSource {
		got, ok := r.Tools[tool]
		if !ok {
			t.Errorf("missing resolved tool %q", tool)
			continue
		}
		if got.Source != src {
			t.Errorf("%s source = %q, want %q", tool, got.Source, src)
		}
	}
	// Intent rendering: bare name becomes "latest"; name@version keeps the version.
	if r.Tools["go"].Intent != "1.26" || r.Tools["git"].Intent != "latest" {
		t.Errorf("intents: go=%q git=%q", r.Tools["go"].Intent, r.Tools["git"].Intent)
	}
	// ripgrep resolves through the rg alias.
	if r.Tools["ripgrep"].Resolved != "14.1" {
		t.Errorf("ripgrep resolved = %q, want 14.1 (via rg alias)", r.Tools["ripgrep"].Resolved)
	}
	if r.Agents["claude-code"].Resolved != "1.2.3" {
		t.Errorf("agent resolved = %q", r.Agents["claude-code"].Resolved)
	}
}

func TestResolutionToLock(t *testing.T) {
	pb := &playbook.Playbook{Tools: []string{"go@1.26"}}
	l := Resolve(pb, fakeProbe{"go": "1.26.4"}).Lock("debian:bookworm-slim@sha256:x", "2026-06-08T00:00:00Z")
	if l.LoomLock != 1 || l.BaseImage != "debian:bookworm-slim@sha256:x" {
		t.Errorf("lock header wrong: %+v", l)
	}
	if l.Tools["go"].Resolved != "1.26.4" || l.Tools["go"].Source != "base-image" {
		t.Errorf("lock tool wrong: %+v", l.Tools["go"])
	}
}
