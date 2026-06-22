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

// TestRolesMergeConcatDedup covers FR-SCHEMA-011: the plural roles: declaration
// merges by the generic list concat/dedup path (like tools/rules/hooks), NOT the
// last-wins scalar rule that role: uses. A later layer's roles union with the
// earlier layer's, dedup keeping first occurrence.
func TestRolesMergeConcatDedup(t *testing.T) {
	base := &Playbook{Loom: 1, Tier: TierBase, Roles: []string{"loom-author"}}
	proj := &Playbook{Tier: TierProject, Name: "loom", Roles: []string{"loom-advisor", "loom-author"}}
	got := Merge(base, proj).Roles

	want := []string{"loom-author", "loom-advisor"} // union, dedup keeps first
	if !slices.Equal(got, want) {
		t.Errorf("roles = %v, want %v (concat + dedup)", got, want)
	}

	// Plural roles: and scalar role: are independent — merging roles must not
	// touch the marker-default scalar.
	if r := Merge(base, proj).Role; r != "" {
		t.Errorf("role scalar = %q, want \"\" (roles: must not derive the scalar marker)", r)
	}
}

// TestNetworkingMerge covers FR-SCHEMA-012 (T20 S2a, ADR-0028): egress: is a
// last-non-empty-wins scalar (like user:) and allow: is a union across tiers,
// deduped in layer order (like rules:/dotfiles:), so the base load-bearing entries
// cannot be dropped by an overlay.
func TestNetworkingMerge(t *testing.T) {
	// egress: last-non-empty-wins; an empty later layer must not blank the value.
	base := &Playbook{Loom: 1, Tier: TierBase, Networking: &Networking{Egress: EgressNone}}
	stack := &Playbook{}                               // no networking — must not blank the base posture
	proj := &Playbook{Tier: TierProject, Name: "loom"} // no egress override
	if got := Merge(base, stack, proj).Networking; got == nil || got.Egress != EgressNone {
		t.Errorf("egress = %+v, want none (base posture survives empty later layers)", got)
	}

	// A later non-empty egress wins (the posture is a scalar).
	over := &Playbook{Tier: TierProject, Name: "loom", Networking: &Networking{Egress: EgressOff}}
	if got := Merge(base, over).Networking; got == nil || got.Egress != EgressOff {
		t.Errorf("egress = %+v, want off (later tier wins the scalar)", got)
	}

	// allow: union across tiers, deduped keeping first occurrence (the base
	// load-bearing entry survives, overlay adds, the shared host is not duplicated).
	baseAllow := &Playbook{Loom: 1, Tier: TierBase,
		Networking: &Networking{Egress: EgressAllowlist, Allow: []string{"api.anthropic.com", "github.com"}}}
	projAllow := &Playbook{Tier: TierProject, Name: "loom",
		Networking: &Networking{Allow: []string{"github.com", "svc.internal"}}}
	got := Merge(baseAllow, projAllow).Networking
	want := []string{"api.anthropic.com", "github.com", "svc.internal"}
	if got == nil || !slices.Equal(got.Allow, want) {
		t.Errorf("allow = %+v, want %v (union + dedup, layer order)", got, want)
	}
	// The scalar still carries the base posture (egress did not get unioned).
	if got.Egress != EgressAllowlist {
		t.Errorf("egress = %q, want allowlist (base scalar survives an overlay with no egress)", got.Egress)
	}

	// Unset everywhere stays nil (= off, the Phase-1 default).
	if got := Merge(&Playbook{Loom: 1, Tier: TierBase}, proj).Networking; got != nil {
		t.Errorf("networking = %+v, want nil (unset = off)", got)
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

// TestMergeDropsReported pins the FR-IMPORT-005 security boundary at the merge seam:
// reported is DRAFT-ONLY review data (the imported lifecycle commands), never part of
// the effective desired-state. Merge must DROP it — so a captured (untrusted) command
// can never propagate into a merged playbook that build/provision consumes. There is
// no execution path because there is no propagation path (ADR-0005 worst-thing test).
func TestMergeDropsReported(t *testing.T) {
	proj := &Playbook{
		Loom: 1, Tier: TierProject, Name: "imported",
		Reported: &Reported{Commands: []ReportedCommand{
			{Hook: "postCreateCommand", Run: []string{"make"}},
		}},
	}
	got := Merge(&Playbook{Loom: 1, Tier: TierBase}, proj)
	if got.Reported != nil {
		t.Errorf("merged Reported = %+v, want nil (reported is draft-only review data, never merged into effective state)", got.Reported)
	}
}
