package playbook

import (
	"encoding/json"
	"slices"
	"strings"
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

// TestValidateUserFormat covers FR-SCHEMA-009: user: is optional and accepted
// at ANY tier (not base-restricted like name/stack), `user: root` is permitted
// (means the default), and a set value must be a single token — no whitespace,
// no path / docker-mount separators.
func TestValidateUserFormat(t *testing.T) {
	valid := []*Playbook{
		{Loom: 1, Tier: TierBase},              // unset = root
		{Loom: 1, Tier: TierBase, User: "dev"}, // base-authored (NOT banned)
		{Loom: 1, Tier: TierProject, Name: "x", User: "agent"},
		{Loom: 1, Tier: TierProject, Name: "x", User: "root"}, // explicit root allowed
	}
	for i, pb := range valid {
		if err := pb.Validate(); err != nil {
			t.Errorf("valid case %d (user=%q): %v", i, pb.User, err)
		}
	}

	invalid := map[string]*Playbook{
		"user with space":   {Loom: 1, Tier: TierBase, User: "dev user"},
		"user with slash":   {Loom: 1, Tier: TierBase, User: "home/dev"},
		"user with colon":   {Loom: 1, Tier: TierBase, User: "uid:1000"},
		"user padded space": {Loom: 1, Tier: TierProject, Name: "x", User: " dev"},
		"user with tab":     {Loom: 1, Tier: TierBase, User: "de\tv"},
	}
	for name, pb := range invalid {
		if err := pb.Validate(); err == nil {
			t.Errorf("%s: expected validation error, got nil", name)
		}
	}
}

// TestValidateRoleFormat covers the role: schema clause (ADR-0019 PR4 §5,
// LL-014): role: is optional, accepted at any tier, and a set value must be a
// single token (same shape as user:). The marker-safe charset narrowing and the
// non-root-user-needs-a-role rule are engine/build-time concerns, not schema.
func TestValidateRoleFormat(t *testing.T) {
	valid := []*Playbook{
		{Loom: 1, Tier: TierBase},                      // unset = no marker
		{Loom: 1, Tier: TierBase, Role: "loom-author"}, // base-authored
		{Loom: 1, Tier: TierProject, Name: "x", Role: "loom_advisor"},
	}
	for i, pb := range valid {
		if err := pb.Validate(); err != nil {
			t.Errorf("valid case %d (role=%q): %v", i, pb.Role, err)
		}
	}

	invalid := map[string]*Playbook{
		"role with space": {Loom: 1, Tier: TierBase, Role: "loom author"},
		"role with slash": {Loom: 1, Tier: TierBase, Role: "a/b"},
		"role with colon": {Loom: 1, Tier: TierBase, Role: "a:b"},
		"role padded":     {Loom: 1, Tier: TierProject, Name: "x", Role: " role"},
		"role with tab":   {Loom: 1, Tier: TierBase, Role: "ro\tle"},
	}
	for name, pb := range invalid {
		if err := pb.Validate(); err == nil {
			t.Errorf("%s: expected validation error, got nil", name)
		}
	}
}

// TestValidateRolesDeclaration covers FR-SCHEMA-011: each roles: entry must be a
// single token (the role: shape) AND a known loom-role. A bare unknown token, a
// whitespace/slash token, and an otherwise-valid token alongside an unknown one
// all reject; the known set {loom-author, loom-advisor} validates.
func TestValidateRolesDeclaration(t *testing.T) {
	valid := []*Playbook{
		{Loom: 1, Tier: TierBase},                                                 // unset = no declaration
		{Loom: 1, Tier: TierBase, Roles: []string{"loom-author"}},                 // single known role
		{Loom: 1, Tier: TierBase, Roles: []string{"loom-author", "loom-advisor"}}, // the full provisioned set
		{Loom: 1, Tier: TierProject, Name: "x", Roles: []string{"loom-advisor"}},
	}
	for i, pb := range valid {
		if err := pb.Validate(); err != nil {
			t.Errorf("valid case %d (roles=%v): %v", i, pb.Roles, err)
		}
	}

	invalid := map[string]*Playbook{
		"unknown role token":     {Loom: 1, Tier: TierBase, Roles: []string{"loom-reviewer"}},
		"known plus unknown":     {Loom: 1, Tier: TierBase, Roles: []string{"loom-author", "loom-reviewer"}},
		"roles entry with space": {Loom: 1, Tier: TierBase, Roles: []string{"loom author"}},
		"roles entry with slash": {Loom: 1, Tier: TierBase, Roles: []string{"loom/author"}},
		"roles entry with colon": {Loom: 1, Tier: TierBase, Roles: []string{"loom:author"}},
		"roles entry padded":     {Loom: 1, Tier: TierProject, Name: "x", Roles: []string{" loom-author"}},
		"empty roles entry":      {Loom: 1, Tier: TierBase, Roles: []string{""}},
	}
	for name, pb := range invalid {
		if err := pb.Validate(); err == nil {
			t.Errorf("%s: expected validation error, got nil", name)
		}
	}
}

// TestSingleRoleConfigByteIdentical is the C2/ADR-0021 guarantee: adding the
// plural roles: field must NOT change the wire format of a config that authors
// only the scalar role:. With roles unset, the omitempty tag elides the key, so
// the JSON is byte-identical to a pre-Slice-A build — no behavior change, no
// marker derived from a (here absent) roles ceiling.
func TestSingleRoleConfigByteIdentical(t *testing.T) {
	pb := &Playbook{Loom: 1, Tier: TierBase, Role: "loom-author"}
	got, err := json.Marshal(pb)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"loom":1,"tier":"base","role":"loom-author"}`
	if string(got) != want {
		t.Errorf("single-role config JSON =\n  %s\nwant byte-identical (no roles key):\n  %s", got, want)
	}
	if strings.Contains(string(got), "roles") {
		t.Errorf("single-role config leaked a roles key: %s", got)
	}
}

// TestValidateNetworking covers FR-SCHEMA-012 (T20 S2a/S2b, ADR-0028 + Amendment
// 1): the egress enum, the allowlist-requires-allow rule, and — as of S2b —
// allowlist being ACCEPTED with a non-empty allow: (the S2a fail-close is flipped;
// the proxy-sidecar mechanism is now wired). none/off/unset are accepted; an
// allowlist with an EMPTY allow: is still rejected (a deny-by-default posture must
// state intent — the engine's load-bearing floor is a fail-safe, not a stand-in).
func TestValidateNetworking(t *testing.T) {
	valid := []*Playbook{
		{Loom: 1, Tier: TierBase},                                              // unset = off (no networking section)
		{Loom: 1, Tier: TierBase, Networking: &Networking{}},                   // present but empty egress = off
		{Loom: 1, Tier: TierBase, Networking: &Networking{Egress: EgressOff}},  // explicit off
		{Loom: 1, Tier: TierBase, Networking: &Networking{Egress: EgressNone}}, // the S1 cut
		{Loom: 1, Tier: TierProject, Name: "x", Networking: &Networking{Egress: EgressNone}},
		// off carrying an allow: list is harmless (union-merged, ignored for off/none).
		{Loom: 1, Tier: TierBase, Networking: &Networking{Egress: EgressOff, Allow: []string{"api.anthropic.com"}}},
		// S2b: allowlist WITH a non-empty allow: is now accepted (the proxy mechanism).
		{Loom: 1, Tier: TierBase, Networking: &Networking{Egress: EgressAllowlist, Allow: []string{"example.com"}}},
		{Loom: 1, Tier: TierProject, Name: "x",
			Networking: &Networking{Egress: EgressAllowlist, Allow: []string{"api.anthropic.com", "github.com"}}},
	}
	for i, pb := range valid {
		if err := pb.Validate(); err != nil {
			t.Errorf("valid case %d (%+v): %v", i, pb.Networking, err)
		}
	}

	invalid := map[string]*Playbook{
		"unknown egress posture": {Loom: 1, Tier: TierBase, Networking: &Networking{Egress: "deny"}},
		// allowlist still REQUIRES a non-empty allow: list — an empty deny-by-default
		// posture is a likely mistake (the load-bearing floor is a fail-safe, not intent).
		"allowlist without allow": {Loom: 1, Tier: TierBase, Networking: &Networking{Egress: EgressAllowlist}},
	}
	for name, pb := range invalid {
		if err := pb.Validate(); err == nil {
			t.Errorf("%s: expected validation error, got nil", name)
		}
	}

	// The empty-allow allowlist error must name the missing allow: list so the
	// operator knows what to add (not a silent degrade to full egress).
	pb := &Playbook{Loom: 1, Tier: TierBase, Networking: &Networking{Egress: EgressAllowlist}}
	err := pb.Validate()
	if err == nil || !strings.Contains(err.Error(), "non-empty allow") {
		t.Errorf("empty-allow allowlist error must name the required allow: list, got: %v", err)
	}
}

// TestParseNetworkingBothTiers covers FR-SCHEMA-012: the networking: section
// decodes from YAML at both the base and project tiers — egress scalar + allow
// list — through the same struct tags YAML and JSON share.
func TestParseNetworkingBothTiers(t *testing.T) {
	baseYAML := []byte("loom: 1\ntier: base\nnetworking:\n  egress: allowlist\n  allow:\n    - api.anthropic.com\n    - github.com\n")
	base, err := Parse(baseYAML)
	if err != nil {
		t.Fatalf("parse base: %v", err)
	}
	if base.Networking == nil || base.Networking.Egress != EgressAllowlist {
		t.Fatalf("base egress = %+v, want allowlist", base.Networking)
	}
	if !slices.Equal(base.Networking.Allow, []string{"api.anthropic.com", "github.com"}) {
		t.Errorf("base allow = %v, want [api.anthropic.com github.com]", base.Networking.Allow)
	}

	projYAML := []byte("loom: 1\ntier: project\nname: x\nnetworking:\n  egress: none\n")
	proj, err := Parse(projYAML)
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}
	if proj.Networking == nil || proj.Networking.Egress != EgressNone {
		t.Errorf("project egress = %+v, want none", proj.Networking)
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
	wantDotfiles := []string{"claude/settings.json", "claude/statusline.sh", "gitconfig", "bash/prompt.go.sh"}
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

// TestParseRejectsUnknownKey covers FR-SCHEMA-013 (T20/T28 LOW-2): strict
// decoding rejects any key that maps to no declared schema field, and the error
// NAMES the offending key so a typo is loud. The motivating case is a one-char
// typo in a SECURITY field — `egres:` for `egress:`, which under non-strict
// decoding silently parsed to the permissive full-egress default. Each case
// asserts both that parse fails AND that the bad key appears in the error.
func TestParseRejectsUnknownKey(t *testing.T) {
	cases := map[string]struct {
		in     string
		badKey string
	}{
		"typo'd top-level tools key": {
			in:     "loom: 1\ntier: base\ntols:\n  - git\n",
			badKey: "tols",
		},
		"typo'd security egress scalar": {
			in:     "loom: 1\ntier: base\nnetworking:\n  egres: none\n",
			badKey: "egres",
		},
		"typo'd nested credential method": {
			in:     "loom: 1\ntier: base\nharness:\n  claude:\n    credential:\n      methd: volume-token\n",
			badKey: "methd",
		},
		"entirely unknown top-level key": {
			in:     "loom: 1\ntier: base\nbogus: true\n",
			badKey: "bogus",
		},
		"unknown key in JSON input (same decode path)": {
			in:     `{"loom":1,"tier":"base","egres":"none"}`,
			badKey: "egres",
		},
		// Strict YAML decode also rejects a DUPLICATE top-level key (non-strict
		// silently kept last-wins) — e.g. a second `harness:` block that shadows
		// the first. The library names the duplicated key in the error.
		"duplicate top-level key": {
			in:     "loom: 1\ntier: base\nharness:\n  claude:\n    settings: a\nharness:\n  claude:\n    settings: b\n",
			badKey: "harness",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(tc.in))
			if err == nil {
				t.Fatalf("expected parse to REJECT unknown key %q, got nil error (silently dropped — the LOW-2 hole)", tc.badKey)
			}
			if !strings.Contains(err.Error(), tc.badKey) {
				t.Errorf("error must NAME the offending key %q so the typo is loud; got: %v", tc.badKey, err)
			}
		})
	}
}

// TestParseAcceptsKnownGood covers FR-SCHEMA-013's complement: a fully-declared
// playbook — every declared field, with comments and a YAML anchor/alias — still
// parses cleanly under strict decoding. Strictness must reject only keys with no
// struct field, never valid YAML constructs (comments and anchors are resolved by
// the YAML→JSON step before strict decode ever sees them).
func TestParseAcceptsKnownGood(t *testing.T) {
	// A YAML anchor (&tok) + alias (*tok) the YAML layer must resolve away before
	// strict JSON decode; comments interspersed throughout; all-declared fields.
	in := []byte(`loom: 1            # schema version
tier: project
name: x
extends: base
stack: go           # selects stacks/go
overlay: loom
user: dev
role: loom-author
roles:
  - loom-author
networking:
  egress: none      # a SECURITY field — must round-trip, not be rejected
agents:
  - claude-code
tools:
  - &gotool go@1.26 # anchor
  - gopls
  - *gotool         # alias resolves to go@1.26 (a declared list, not a new key)
rules:
  - common/safety
dotfiles:
  - gitconfig
hooks:
  - guard-bash
ports:
  - 8080
env:
  - ANTHROPIC_API_KEY
ci:
  - ci
harness:
  claude:
    settings: claude/settings.json
    trust: claude/trust.json
    hooks:
      - guard-bash
    skills:
      - import-enrich
    credential:
      method: volume-token
      env: ANTHROPIC_API_KEY
config_source:
  type: local
  path: ./config
`)
	pb, err := Parse(in)
	if err != nil {
		t.Fatalf("known-good fully-declared playbook must PARSE under strict decode, got: %v", err)
	}
	if pb.Networking == nil || pb.Networking.Egress != EgressNone {
		t.Errorf("egress should decode to none, got %+v", pb.Networking)
	}
	if !slices.Contains(pb.Tools, "go@1.26") {
		t.Errorf("anchored tool should decode, got tools=%v", pb.Tools)
	}
}
