// Package playbook parses, validates, and merges the thin, two-tier playbook
// (ADR-0002, ADR-0004). The JSON struct tags below ARE the authored schema;
// YAML and JSON inputs both decode through them (frozen format decision,
// docs/SPEC-playbook.md). The playbook declares intent; the resolver + lockfile
// pin the concrete versions (separate packages).
package playbook

// SchemaVersion is the only playbook schema version this engine accepts.
const SchemaVersion = 1

// Tier names (docs/SPEC-playbook.md): a machine-wide base and a per-project overlay.
const (
	TierBase    = "base"
	TierProject = "project"
)

// Playbook is the union of both tiers' fields; Tier selects which apply, and
// Validate enforces the tier-appropriate subset. Lists are intent-by-reference
// (rules/dotfiles/hooks resolve against the config source); Tools are name@version
// intent strings the resolver later pins.
type Playbook struct {
	Loom    int    `json:"loom"`              // schema version
	Tier    string `json:"tier"`              // base | project
	Name    string `json:"name,omitempty"`    // project tier
	Extends string `json:"extends,omitempty"` // project tier: inherited base
	Stack   string `json:"stack,omitempty"`   // project tier: selects stacks/<lang>
	Overlay string `json:"overlay,omitempty"` // project tier: most-specific layer

	Agents   []string `json:"agents,omitempty"`   // agent harnesses
	Tools    []string `json:"tools,omitempty"`    // name@version intent
	Rules    []string `json:"rules,omitempty"`    // references, not bodies
	Dotfiles []string `json:"dotfiles,omitempty"` // $HOME config references
	Hooks    []string `json:"hooks,omitempty"`    // guardrail references
	Ports    []int    `json:"ports,omitempty"`    // project tier
	Env      []string `json:"env,omitempty"`      // names only; values from .env/secret store
	CI       []string `json:"ci,omitempty"`       // CI templates to emit

	ConfigSource *ConfigSource `json:"config_source,omitempty"`
}

// ConfigSource locates where rules/skills/hooks/dotfiles resolve from (ADR-0006).
type ConfigSource struct {
	Type string `json:"type"`           // local | git
	Path string `json:"path,omitempty"` // local
	Ref  string `json:"ref,omitempty"`  // git
	URL  string `json:"url,omitempty"`  // git
}
