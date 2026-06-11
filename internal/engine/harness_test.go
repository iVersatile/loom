package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/iVersatile/loom/internal/playbook"
)

func harnessFixture() (fstest.MapFS, map[string]playbook.HarnessAgent) {
	src := fstest.MapFS{
		"dotfiles/claude/settings.json": {Data: []byte(`{"hooks":{"Stop":[]}}`)},
		"hooks/guard-bash":              {Data: []byte("#!/bin/sh\nexit 0\n")},
		"skills/replan/SKILL.md":        {Data: []byte("# replan\n")},
		"skills/replan/gather.sh":       {Data: []byte("#!/bin/sh\necho hi\n")},
	}
	harness := map[string]playbook.HarnessAgent{
		"claude": {
			Settings: "claude/settings.json",
			Hooks:    []string{"guard-bash"},
			Skills:   []string{"replan"},
		},
	}
	return src, harness
}

// TestMaterializeHarnessArtifacts proves FR-BUILD-009's shape: settings
// whole-file into the agent home, hooks under hooks/ EXECUTABLE, skills
// copied recursively with script bits preserved.
func TestMaterializeHarnessArtifacts(t *testing.T) {
	src, harness := harnessFixture()
	home := t.TempDir()

	mats, err := materializeHarness(src, harness, home)
	if err != nil {
		t.Fatalf("materialize harness: %v", err)
	}
	if len(mats) != 4 { // settings + 1 hook + 2 skill files
		t.Fatalf("expected 4 materialized artifacts, got %d: %+v", len(mats), mats)
	}

	settings, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil || string(settings) != `{"hooks":{"Stop":[]}}` {
		t.Fatalf("settings not materialized whole-file: %q err=%v", settings, err)
	}

	hook := filepath.Join(home, ".claude", "hooks", "guard-bash")
	info, err := os.Stat(hook)
	if err != nil {
		t.Fatalf("hook not materialized: %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("hook must be executable, mode %v", info.Mode())
	}

	for _, rel := range []string{"SKILL.md", "gather.sh"} {
		p := filepath.Join(home, ".claude", "skills", "replan", rel)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("skill file %s not materialized: %v", rel, err)
		}
	}
	gi, _ := os.Stat(filepath.Join(home, ".claude", "skills", "replan", "gather.sh"))
	if gi.Mode().Perm()&0o100 == 0 {
		t.Errorf("skill .sh must keep the executable bit, mode %v", gi.Mode())
	}
}

// TestMaterializeHarnessIdempotent proves FR-BUILD-009's idempotence: a
// second run over an unchanged source reports zero changes (write-if-changed
// — the same property that keeps the T7 home digest stable).
func TestMaterializeHarnessIdempotent(t *testing.T) {
	src, harness := harnessFixture()
	home := t.TempDir()

	if _, err := materializeHarness(src, harness, home); err != nil {
		t.Fatal(err)
	}
	mats, err := materializeHarness(src, harness, home)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range mats {
		if m.Changed {
			t.Errorf("second run must be a no-op, %s reported changed", m.Display)
		}
	}
}

// TestMaterializeHarnessMissingRefErrors: a dangling reference is a loud
// error naming the agent and field, never a silent skip.
func TestMaterializeHarnessMissingRefErrors(t *testing.T) {
	src, harness := harnessFixture()
	h := harness["claude"]
	h.Hooks = append(h.Hooks, "no-such-hook")
	harness["claude"] = h

	_, err := materializeHarness(src, harness, t.TempDir())
	if err == nil {
		t.Fatal("dangling hook reference must error")
	}
	if want := `harness.claude.hooks "no-such-hook"`; !strings.Contains(err.Error(), want) {
		t.Errorf("error must name agent+field+ref: got %q, want substring %q", err, want)
	}
}
