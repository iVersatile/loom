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
