package playbook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHarnessMergeRules proves FR-SCHEMA-008: harness: merges per agent
// namespace — list fields concatenate + dedup in layer order (the dotfiles
// rule), settings is last-non-empty-wins (SPEC-playbook#harness).
func TestHarnessMergeRules(t *testing.T) {
	base := &Playbook{
		Loom: 1, Tier: TierBase,
		Harness: map[string]HarnessAgent{
			"claude": {
				Settings: "claude/settings.json",
				Hooks:    []string{"guard-bash", "session-snapshot"},
				Skills:   []string{"replan"},
			},
		},
	}
	overlay := &Playbook{
		Harness: map[string]HarnessAgent{
			"claude": {
				Trust:  "claude/trust.json",                    // project-tier trust posture (ADR-0018)
				Hooks:  []string{"guard-bash", "project-hook"}, // dup + new
				Skills: []string{"achievements"},
			},
			"gemini": {Hooks: []string{"guard-bash"}}, // new namespace
		},
	}

	got := Merge(base, overlay)

	claude := got.Harness["claude"]
	if claude.Settings != "claude/settings.json" {
		t.Errorf("settings must survive a layer that doesn't set it: got %q", claude.Settings)
	}
	if claude.Trust != "claude/trust.json" {
		t.Errorf("trust is last-non-empty-wins — a later layer must be able to declare it: got %q", claude.Trust)
	}
	wantHooks := []string{"guard-bash", "session-snapshot", "project-hook"}
	if len(claude.Hooks) != len(wantHooks) {
		t.Fatalf("hooks concat+dedup: got %v, want %v", claude.Hooks, wantHooks)
	}
	for i, h := range wantHooks {
		if claude.Hooks[i] != h {
			t.Errorf("hooks order: got %v, want %v", claude.Hooks, wantHooks)
			break
		}
	}
	if len(claude.Skills) != 2 || claude.Skills[0] != "replan" || claude.Skills[1] != "achievements" {
		t.Errorf("skills concat: got %v", claude.Skills)
	}
	if _, ok := got.Harness["gemini"]; !ok {
		t.Error("a later layer must be able to introduce a new agent namespace")
	}
}

// TestValidateHarnessSettingsBaseOnly proves the Phase-1 constraint in
// FR-SCHEMA-008: harness.<agent>.settings is base-authored — a non-base
// layer declaring one fails Load naming the offending layer (whole-file, no
// key-merge yet). Enforced per layer at Load, NOT on the merged playbook:
// the merge legitimately carries the base's settings under the project's
// tier (the T16 PR 2 dogfood test caught a merged-tier check rejecting
// every real wire-up).
func TestValidateHarnessSettingsBaseOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/proj")); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	pbPath := filepath.Join(dir, "loom.yml")

	declare := func(block string) {
		orig, err := os.ReadFile(filepath.Join("testdata", "proj", "loom.yml"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(pbPath, append(orig, []byte(block)...), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// A project-layer settings declaration must fail, naming the layer.
	declare("\nharness:\n  claude:\n    settings: claude/settings.json\n")
	_, err := Load(pbPath)
	if err == nil || !strings.Contains(err.Error(), "base-tier only") ||
		!strings.Contains(err.Error(), "project playbook") {
		t.Fatalf("project-layer harness settings must fail Load naming the layer, got: %v", err)
	}

	// Project-layer hooks (no settings) are fine.
	declare("\nharness:\n  claude:\n    hooks:\n      - guard-bash\n")
	if _, err := Load(pbPath); err != nil {
		t.Fatalf("project-layer harness hooks (no settings) must load, got: %v", err)
	}
}
