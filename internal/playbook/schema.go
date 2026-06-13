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

	// User is the container's runtime user (T10, ADR-0019). Optional scalar,
	// last-non-empty-wins, authored at ANY tier (env-wide base default with a
	// project override is the expected shape). Unset means root (Phase-1
	// compatibility — every existing playbook keeps meaning what it meant). A
	// non-root value resolves $HOME to /home/<user> (homeForUser) and is created
	// at provision. PR 2 (this) carries schema/merge/validate + the resolved
	// ContainerSpec.User/Home plumbing; the engine BEHAVIOR that consumes it
	// (docker run --user, useradd, ownership chown) is T10 PR 3.
	User string `json:"user,omitempty"`

	Agents   []string `json:"agents,omitempty"`   // agent harnesses
	Tools    []string `json:"tools,omitempty"`    // name@version intent
	Rules    []string `json:"rules,omitempty"`    // references, not bodies
	Dotfiles []string `json:"dotfiles,omitempty"` // $HOME config references
	Hooks    []string `json:"hooks,omitempty"`    // guardrail references
	Ports    []int    `json:"ports,omitempty"`    // project tier
	Env      []string `json:"env,omitempty"`      // names only; values from .env/secret store
	CI       []string `json:"ci,omitempty"`       // CI templates to emit

	// Harness is the per-agent harness-home declaration (SPEC-playbook
	// #harness, ADR-0015 decision 3): artifacts with semantics plain dotfiles
	// lack — hook registration inside settings.json, executable bits,
	// guardrail policy weight. Keyed by agent namespace ("claude" today).
	Harness map[string]HarnessAgent `json:"harness,omitempty"`

	ConfigSource *ConfigSource `json:"config_source,omitempty"`
}

// HarnessAgent is one agent's harness-home config. All fields are references
// resolved against the config source (explicit-by-reference, like rules:).
type HarnessAgent struct {
	// Settings is a dotfiles/ reference materialized WHOLE-FILE into the
	// agent home; it carries its own hook registrations (Phase 1: base-tier
	// only, no key-merge — SPEC-playbook Open question 1).
	Settings string `json:"settings,omitempty"`
	// Trust is a dotfiles/ reference materialized WHOLE-FILE to the agent's
	// top-level state file (<home>/.<agent>.json — a SIBLING of the agent
	// home): the trust/opt-in flags the harness reads at session start
	// (hasTrustDialogAccepted et al.). Declared trust posture is config,
	// not state — the playbook owns the declared file; anything else the
	// live file accumulates is rederivable cache (036 ruling, ADR-0018).
	Trust string `json:"trust,omitempty"`
	// Hooks are hooks/ references materialized to <agent home>/hooks/<name>,
	// executable.
	Hooks []string `json:"hooks,omitempty"`
	// Skills are skills/<name>/ directories materialized recursively to
	// <agent home>/skills/<name>/.
	Skills []string `json:"skills,omitempty"`
}

// ConfigSource locates where rules/skills/hooks/dotfiles resolve from (ADR-0006).
type ConfigSource struct {
	Type string `json:"type"`           // local | git
	Path string `json:"path,omitempty"` // local
	Ref  string `json:"ref,omitempty"`  // git
	URL  string `json:"url,omitempty"`  // git
}
