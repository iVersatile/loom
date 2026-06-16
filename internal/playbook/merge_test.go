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

// TestUserMergeLastWins covers FR-SCHEMA-009: user: is a last-non-empty-wins
// scalar authored at any tier (a base default, a project override), and an
// empty later layer must not clobber an earlier value.
func TestUserMergeLastWins(t *testing.T) {
	base := &Playbook{Loom: 1, Tier: TierBase, User: "dev"}
	stack := &Playbook{} // no user — must not blank the base default
	proj := &Playbook{Tier: TierProject, Name: "loom", User: "agent"}
	if got := Merge(base, stack, proj).User; got != "agent" {
		t.Errorf("user = %q, want agent (project overrides base, empty layer ignored)", got)
	}

	// Base-only default survives when no later layer authors user.
	baseOnly := &Playbook{Loom: 1, Tier: TierBase, User: "dev"}
	projNoUser := &Playbook{Tier: TierProject, Name: "loom"}
	if got := Merge(baseOnly, projNoUser).User; got != "dev" {
		t.Errorf("user = %q, want dev (base default, no override)", got)
	}

	// Unset everywhere stays unset (= root, the Phase-1 default).
	if got := Merge(&Playbook{Loom: 1, Tier: TierBase}, projNoUser).User; got != "" {
		t.Errorf("user = %q, want \"\" (unset = root)", got)
	}
}

// TestRoleMergeLastWins covers the role: scalar (ADR-0019 PR4 §5): a
// last-non-empty-wins scalar, so an env-wide base default can be overridden by a
// project layer, and an empty later layer must not clobber an earlier value.
func TestRoleMergeLastWins(t *testing.T) {
	base := &Playbook{Loom: 1, Tier: TierBase, Role: "loom-author"}
	stack := &Playbook{} // no role — must not blank the base default
	proj := &Playbook{Tier: TierProject, Name: "loom", Role: "loom-advisor"}
	if got := Merge(base, stack, proj).Role; got != "loom-advisor" {
		t.Errorf("role = %q, want loom-advisor (project overrides base, empty layer ignored)", got)
	}

	// Base-only default survives when no later layer authors role.
	projNoRole := &Playbook{Tier: TierProject, Name: "loom"}
	if got := Merge(base, projNoRole).Role; got != "loom-author" {
		t.Errorf("role = %q, want loom-author (base default, no override)", got)
	}

	// Unset everywhere stays unset (= no marker).
	if got := Merge(&Playbook{Loom: 1, Tier: TierBase}, projNoRole).Role; got != "" {
		t.Errorf("role = %q, want \"\" (unset = no marker)", got)
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
