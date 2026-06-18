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

// Known loom-roles a `roles:` declaration may name (Slice A, T34 cutover). These
// are the DECLARATIVE provisioned-role tokens — the trust marker is resolved
// separately by the engine (the scalar `role:`), never derived from this set.
const (
	RoleAuthor  = "loom-author"
	RoleAdvisor = "loom-advisor"
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

	// Role is the container's loom-role identity (T10/ADR-0019 PR4 §5, LL-014).
	// Optional scalar, last-non-empty-wins, authored at ANY tier — the DECLARATIVE
	// IN-TREE source for the root-owned /var/lib/loom/role marker `loom build`
	// writes (replacing the ambient LOOM_SESSION_ROLE env, which is demoted to an
	// explicit override / test-seam). Unset means no marker is written and the
	// build is byte-identical to a pre-PR4 root build — EXCEPT a non-root `user:`
	// with an empty/invalid role is a HARD ERROR (that combination silently breaks
	// the drain role-guard). Marker-safe charset is enforced where it is consumed
	// (engine `validRole`); the schema only requires the same single-token shape as
	// `user:`. The conceptual home is the role model (ADR-0021).
	Role string `json:"role,omitempty"`

	// Roles is the DECLARATIVE set of loom-roles loom-dev is provisioned to act
	// as (Slice A, T34 cutover) — a plural sibling to the scalar Role, forward-
	// looking for the co-residence slice. It is declaration-only: it does NOT
	// derive or affect the trust marker (the scalar `role:` stays the marker
	// default — ADR-0021 §Alternatives, the C2 fail-closed insight). Merges by
	// the generic list concat/dedup like the other []string fields; each entry
	// must be a single role token in {loom-author, loom-advisor}. A single-`role:`
	// config carries no Roles and stays byte-identical to a pre-Slice-A build.
	Roles []string `json:"roles,omitempty"`

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
