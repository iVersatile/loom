package playbook

import (
	"slices"
	"testing"
)

func TestParseYAMLAndJSONAgree(t *testing.T) {
	yamlIn := []byte("loom: 1\ntier: project\nname: x\nstack: go\ntools:\n  - go@1.26\n")
	jsonIn := []byte(`{"loom":1,"tier":"project","name":"x","stack":"go","tools":["go@1.26"]}`)

	y, err := Parse(yamlIn)
	if err != nil {
		t.Fatalf("yaml: %v", err)
	}
	j, err := Parse(jsonIn)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	if y.Name != j.Name || y.Stack != j.Stack || !slices.Equal(y.Tools, j.Tools) {
		t.Errorf("YAML and JSON parses differ:\n yaml=%+v\n json=%+v", y, j)
	}
}

func TestValidateGood(t *testing.T) {
	good := []*Playbook{
		{Loom: 1, Tier: TierBase, Tools: []string{"git"}},
		{Loom: 1, Tier: TierProject, Name: "x", Stack: "go"},
		{Loom: 1, Tier: TierProject, Name: "x", ConfigSource: &ConfigSource{Type: "local", Path: "./config"}},
	}
	for i, pb := range good {
		if err := pb.Validate(); err != nil {
			t.Errorf("case %d should be valid: %v", i, err)
		}
	}
}

func TestValidateErrors(t *testing.T) {
	cases := map[string]*Playbook{
		"wrong schema version": {Loom: 2, Tier: TierBase},
		"missing tier":         {Loom: 1},
		"bad tier":             {Loom: 1, Tier: "machine"},
		"project without name": {Loom: 1, Tier: TierProject},
		"base with name":       {Loom: 1, Tier: TierBase, Name: "x"},
		"base with stack":      {Loom: 1, Tier: TierBase, Stack: "go"},
		"config_source no type": {Loom: 1, Tier: TierProject, Name: "x",
			ConfigSource: &ConfigSource{Path: "./config"}},
		"local source no path": {Loom: 1, Tier: TierProject, Name: "x",
			ConfigSource: &ConfigSource{Type: "local"}},
		"git source no url": {Loom: 1, Tier: TierProject, Name: "x",
			ConfigSource: &ConfigSource{Type: "git"}},
	}
	for name, pb := range cases {
		if err := pb.Validate(); err == nil {
			t.Errorf("%s: expected validation error, got nil", name)
		}
	}
}

func TestLoadMergesLayers(t *testing.T) {
	res, err := Load("testdata/proj/loom.yml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	pb := res.Playbook

	if pb.Tier != TierProject || pb.Name != "loom" || pb.Stack != "go" || pb.Overlay != "loom" {
		t.Errorf("scalars wrong: tier=%s name=%s stack=%s overlay=%s", pb.Tier, pb.Name, pb.Stack, pb.Overlay)
	}
	// base + stack + project, dedup keeps first occurrence.
	wantTools := []string{"git", "jq", "go@1.26", "gopls"}
	if !slices.Equal(pb.Tools, wantTools) {
		t.Errorf("tools = %v, want %v", pb.Tools, wantTools)
	}
	// rules: union across base/stack/overlay/project (frozen explicit-by-reference).
	wantRules := []string{"common/safety", "go/strict", "loom/project"}
	if !slices.Equal(pb.Rules, wantRules) {
		t.Errorf("rules = %v, want %v", pb.Rules, wantRules)
	}
	wantDotfiles := []string{"claude/settings.json", "claude/statusline.sh", "bash/prompt.go.sh"}
	if !slices.Equal(pb.Dotfiles, wantDotfiles) {
		t.Errorf("dotfiles = %v, want %v", pb.Dotfiles, wantDotfiles)
	}
	if !slices.Equal(pb.Hooks, []string{"guard-bash", "branch-guard", "protect-paths"}) {
		t.Errorf("hooks = %v", pb.Hooks)
	}
	if !slices.Equal(pb.Ports, []int{8080}) {
		t.Errorf("ports = %v", pb.Ports)
	}
	if res.Source == nil {
		t.Error("resolved source FS is nil")
	}
}
