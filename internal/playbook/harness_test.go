package playbook

import (
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
// FR-SCHEMA-008: harness.<agent>.settings on a project-tier playbook is a
// validation error (whole-file, base-authored — no key-merge yet).
func TestValidateHarnessSettingsBaseOnly(t *testing.T) {
	pb := &Playbook{
		Loom: 1, Tier: TierProject, Name: "demo",
		Harness: map[string]HarnessAgent{
			"claude": {Settings: "claude/settings.json"},
		},
	}
	err := pb.Validate()
	if err == nil || !strings.Contains(err.Error(), "base-tier only") {
		t.Fatalf("project-tier harness settings must fail validation, got: %v", err)
	}

	pb.Harness["claude"] = HarnessAgent{Hooks: []string{"guard-bash"}}
	if err := pb.Validate(); err != nil {
		t.Fatalf("project-tier harness hooks (no settings) must validate, got: %v", err)
	}
}
