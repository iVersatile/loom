package playbook

import (
	"slices"
	"testing"
)

func TestMergeScalarLastNonEmptyWins(t *testing.T) {
	base := &Playbook{Loom: 1, Tier: TierBase}
	proj := &Playbook{Tier: TierProject, Name: "loom", Stack: "go"}
	got := Merge(base, proj)

	if got.Tier != TierProject {
		t.Errorf("tier = %q, want project (last non-empty)", got.Tier)
	}
	if got.Name != "loom" || got.Stack != "go" {
		t.Errorf("name/stack = %q/%q", got.Name, got.Stack)
	}
	if got.Loom != 1 {
		t.Errorf("loom = %d, want 1", got.Loom)
	}
}

func TestMergeEmptyScalarDoesNotClobber(t *testing.T) {
	base := &Playbook{Loom: 1, Tier: TierBase, Stack: ""}
	stack := &Playbook{Stack: "go"} // a later fragment with empty name must not blank earlier values
	proj := &Playbook{Tier: TierProject, Name: "loom"}
	got := Merge(base, stack, proj)
	if got.Stack != "go" {
		t.Errorf("stack = %q, want go (set by middle layer, not clobbered by empty)", got.Stack)
	}
}

func TestMergeListsConcatAndDedup(t *testing.T) {
	a := &Playbook{Tools: []string{"git", "jq"}, Ports: []int{8080}}
	b := &Playbook{Tools: []string{"jq", "go@1.26"}, Ports: []int{8080, 9090}}
	got := Merge(a, b)

	wantTools := []string{"git", "jq", "go@1.26"} // jq deduped, order preserved
	if !slices.Equal(got.Tools, wantTools) {
		t.Errorf("tools = %v, want %v", got.Tools, wantTools)
	}
	wantPorts := []int{8080, 9090}
	if !slices.Equal(got.Ports, wantPorts) {
		t.Errorf("ports = %v, want %v", got.Ports, wantPorts)
	}
}
