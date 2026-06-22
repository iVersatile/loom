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

// Egress postures a `networking.egress:` declaration may name (T20 S2a, ADR-0028).
// "" and EgressOff both mean full outbound — the Phase-1 default, so every existing
// playbook keeps meaning what it meant. EgressNone cuts all egress (`--network
// none`, the T20 S1 primitive). EgressAllowlist is the deny-by-default posture
// whose mechanism (custom network / proxy sidecar) is S2b — it is a KNOWN enum
// value but FAIL-CLOSED at validate (not yet implemented), never silently run with
// full egress.
const (
	EgressOff       = "off"
	EgressNone      = "none"
	EgressAllowlist = "allowlist"
)

// Credential adapter methods a `harness.<agent>.credential.method` may name
// (ADR-0027 §1 — the FIXED consumer-adapter enum, no open-ended resolver). Only
// CredVolumeToken (M1) and CredAPIKeyHelper (M2) are WIRED in slice 1; the rest
// are valid-to-DECLARE tokens but FAIL-CLOSED at validate as "not yet supported"
// — a declared credential method is NEVER silently no-op'd (the guardrail
// principle: a credential you can't honor must fail LOUD, not run unauthenticated).
const (
	CredEnv              = "env"                 // exec-time NAME=value (slice 2+)
	CredAPIKeyHelper     = "apiKeyHelper"        // M2: stdout-helper in settings.json (WIRED)
	CredVolumeToken      = "volume-token"        // M1: per-project volume → exec-time -e (WIRED)
	CredVolumeStoreHelp  = "volume-store+helper" // gh's ADR-0026 shape (described, not re-plumbed)
	CredOAuthFile        = "oauth-file"          // Gemini ADC (slice 2+, the resolver gate)
	CredInteractiveLogin = "interactive-login"   // human-seat fallback (slice 2+)
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

	// Networking is the container's declared egress posture (T20 S2a, ADR-0028).
	// Optional section: unset means `egress: off` — full outbound, the Phase-1
	// default (every existing playbook is unchanged). A pointer so "absent" is
	// distinguishable from "present but empty" (the `config_source` precedent),
	// though `off`/unset collapse to the same behavior. The `egress:` scalar is
	// last-non-empty-wins (like `user:`); `allow:` is a union across tiers (like
	// `rules:`/`dotfiles:`). The S2a slice realizes `egress: none` (→ the T20 S1
	// `--network none` primitive); `allowlist` is a KNOWN posture but FAIL-CLOSED at
	// validate (the custom-network mechanism is S2b — never silently full-egress).
	Networking *Networking `json:"networking,omitempty"`

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
	// Credential is the per-agent credential-acquisition declaration (ADR-0027
	// §1 — the cross-class convention). A pointer so "absent" (no credential
	// declared) is distinguishable from "present but empty" (the Networking /
	// config_source precedent). Merged ANY-TIER last-non-empty-wins, exactly like
	// Trust (a later non-nil Credential REPLACES the earlier) — NOT through the
	// base-gated settings path. loom wires the MECHANISM only: it never authors,
	// reads-into-logs, or commits the credential VALUE — that is a human trust act
	// (real provisioning is out of loom's scope). Slice 1 wires two methods —
	// apiKeyHelper (M2) and volume-token (M1); every other enum member is
	// valid-to-declare but FAIL-CLOSED at validate (ADR-0027 slice 2+).
	Credential *Credential `json:"credential,omitempty"`
}

// Credential is one agent's credential-acquisition declaration (ADR-0027 §1).
// Method names a fixed consumer-adapter from the enum; Helper / Env carry the
// per-method config. The VALUE is never here — Helper is an operator COMMAND
// (e.g. `op read op://...`) loom never runs or reads the output of; Env is an
// env var NAME to inject (e.g. CLAUDE_CODE_OAUTH_TOKEN), the value sourced from a
// human-provisioned per-project volume at exec-time. loom holds neither.
type Credential struct {
	Method string `json:"method"`           // one of the Cred* enum
	Helper string `json:"helper,omitempty"` // apiKeyHelper: the operator stdout-helper command
	Env    string `json:"env,omitempty"`    // volume-token: the env var NAME to inject
}

// Networking is the declared egress posture (T20 S2a, ADR-0028). Egress is a
// last-non-empty-wins scalar in {"" (=off), off, none, allowlist}; Allow is the
// union of runtime-reachable hostnames across tiers. The schema is mechanism-
// independent (it names hostnames, not IPs or proxy config), so the S2b→S3
// migration is an engine change behind this frozen field, not a schema change.
type Networking struct {
	// Egress is the posture: "" or "off" = full outbound (Phase-1 default), "none"
	// = no network but loopback (`--network none`, the T20 S1 primitive),
	// "allowlist" = deny-by-default (S2b — fail-closed as unimplemented at validate).
	Egress string `json:"egress,omitempty"`
	// Allow is the runtime-reachable hostname list (union across tiers; the base
	// tier owns the load-bearing model-API/credential-helper/VCS entries a
	// deny-by-default must not drop). Required and meaningful only for `allowlist`.
	Allow []string `json:"allow,omitempty"`
}

// ConfigSource locates where rules/skills/hooks/dotfiles resolve from (ADR-0006).
type ConfigSource struct {
	Type string `json:"type"`           // local | git
	Path string `json:"path,omitempty"` // local
	Ref  string `json:"ref,omitempty"`  // git
	URL  string `json:"url,omitempty"`  // git
}
