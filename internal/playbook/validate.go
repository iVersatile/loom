package playbook

import (
	"fmt"
	"regexp"
	"strings"
)

// envVarNameRe is a single valid POSIX-ish env var NAME: a letter or underscore
// followed by letters, digits, or underscores — no `=`, no whitespace, no other
// punctuation. Used to constrain the volume-token credential's env: so a typo like
// `env: "FOO=BAR"` cannot mis-target the exec-time injection (it would otherwise
// yield `docker exec -e FOO=BAR=token`).
var envVarNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// validHomeRelPath reports whether p is a safe HOME-relative mount path for the
// oauth-file adapter: non-empty after trim, no leading '/' (an absolute path would
// mount outside $HOME, escaping the per-project home boundary), and no '..' segment
// (traversal out of $HOME). It also rejects whitespace, which would corrupt the
// `-v vol:<home>/<path>:rw` derivation. Kept here (not the engine) so a malformed
// path fails at validate, before any container is built.
func validHomeRelPath(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" || strings.HasPrefix(p, "/") {
		return false
	}
	if strings.ContainsAny(p, " \t\r\n") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

// Validate checks a fully-formed playbook (a top-level base or project file, or a
// merged result) against the schema. Partial layer fragments are NOT validated
// here — they intentionally omit loom/tier and are only meaningful once merged.
func (pb *Playbook) Validate() error {
	var errs []string

	if pb.Loom != SchemaVersion {
		errs = append(errs, fmt.Sprintf("unsupported schema version loom=%d (want %d)", pb.Loom, SchemaVersion))
	}

	switch pb.Tier {
	case TierBase, TierProject:
	case "":
		errs = append(errs, "missing required field: tier")
	default:
		errs = append(errs, fmt.Sprintf("invalid tier %q (want %q or %q)", pb.Tier, TierBase, TierProject))
	}

	if pb.Tier == TierProject && pb.Name == "" {
		errs = append(errs, "project tier requires field: name")
	}

	if pb.Tier == TierBase {
		for field, set := range map[string]bool{
			"name":    pb.Name != "",
			"stack":   pb.Stack != "",
			"extends": pb.Extends != "",
			"ports":   len(pb.Ports) > 0,
		} {
			if set {
				errs = append(errs, fmt.Sprintf("base tier must not set field: %s", field))
			}
		}
	}

	// user: (T10, ADR-0019) is an optional, any-tier scalar — no tier ban (the
	// expected shape is an env-wide base default with a project override). When
	// set it must be a single shell/username token: no whitespace and no path or
	// docker-mount separators (it lands in `docker run --user` and /home/<user>
	// in PR 3). `user: root` is allowed — it means the default, not a special
	// case (the "later layer re-grants root" edge is gated at the full-auto
	// review, not the scalar merge — ADR-0019).
	if pb.User != "" && (strings.TrimSpace(pb.User) != pb.User || strings.ContainsAny(pb.User, " \t\r\n/:")) {
		errs = append(errs, fmt.Sprintf("invalid user %q (must be a single token: no whitespace, no / or :)", pb.User))
	}

	// role: (T10/ADR-0019 PR4 §5, LL-014) is the declarative source for the
	// /var/lib/loom/role marker — an optional, any-tier scalar, same single-token
	// shape as user: (it lands in a shell-written marker file; the engine
	// additionally enforces a marker-safe charset via validRole). The
	// non-root-user-needs-a-role rule is a BUILD-time check (build.go), not a
	// schema rule: it depends on the user/role COMBINATION, and a base-tier role
	// with a project-tier user is a valid authored split the merged check still
	// catches.
	if pb.Role != "" && (strings.TrimSpace(pb.Role) != pb.Role || strings.ContainsAny(pb.Role, " \t\r\n/:")) {
		errs = append(errs, fmt.Sprintf("invalid role %q (must be a single token: no whitespace, no / or :)", pb.Role))
	}

	// roles: (Slice A, T34 cutover) is the DECLARATIVE provisioned role set — a
	// plural sibling to role:, declaration-only (it does NOT affect the trust
	// marker). Each entry reuses role:'s single-token shape (no whitespace, no /
	// or :) AND must name a known loom-role; an unknown token is a hard error so a
	// typo'd declaration cannot silently widen the provisioned set.
	for _, r := range pb.Roles {
		if strings.TrimSpace(r) != r || strings.ContainsAny(r, " \t\r\n/:") {
			errs = append(errs, fmt.Sprintf("invalid roles entry %q (each must be a single token: no whitespace, no / or :)", r))
			continue
		}
		if !validDeclaredRole(r) {
			errs = append(errs, fmt.Sprintf("unknown roles entry %q (want %s or %s)", r, RoleAuthor, RoleAdvisor))
		}
	}

	// networking: (T20 S2a/S2b, ADR-0028 + Amendment 1) is the optional, any-tier
	// egress posture. egress: must be one of {"" (=off), off, none, allowlist}; ""
	// and "off" both mean full outbound (Phase-1 compatible). allowlist REQUIRES a
	// non-empty allow: list — a deny-by-default posture with nothing authored is a
	// likely mistake (the engine's load-bearing floor is a fail-SAFE the agent can't
	// be bricked by, not a substitute for stating intent). As of S2b allowlist is
	// SUPPORTED (the proxy-sidecar mechanism, ADR-0028 Amendment 1): the S2a
	// fail-close is REMOVED. off/none/unset are accepted; none maps to the T20 S1
	// --network none primitive in the engine.
	if n := pb.Networking; n != nil {
		switch n.Egress {
		case "", EgressOff, EgressNone:
			// off/unset = full egress (Phase-1 default); none = the S1 cut. Both
			// accepted. allow: is meaningful only for allowlist, but an authored
			// allow: list is harmless here (union-merged; ignored for off/none).
		case EgressAllowlist:
			if len(n.Allow) == 0 {
				errs = append(errs, `networking.egress: "allowlist" requires a non-empty allow: hostname list`)
			}
		default:
			errs = append(errs, fmt.Sprintf("invalid networking.egress %q (want %q, %q, or %q)", n.Egress, EgressOff, EgressNone, EgressAllowlist))
		}
	}

	for agent, h := range pb.Harness {
		if agent == "" {
			errs = append(errs, "harness: agent namespace key must be non-empty")
		}
		// NOTE: the Phase-1 "settings is base-authored" rule lives in Load,
		// enforced per non-base LAYER — Validate also runs on the MERGED
		// playbook, which legitimately carries the base's settings under the
		// project's tier (T16 PR 2 dogfood test caught the old tier check
		// rejecting every real wire-up).
		for _, ref := range h.Hooks {
			if ref == "" {
				errs = append(errs, fmt.Sprintf("harness.%s.hooks: empty reference", agent))
			}
		}
		for _, ref := range h.Skills {
			if ref == "" {
				errs = append(errs, fmt.Sprintf("harness.%s.skills: empty reference", agent))
			}
		}
		// credential: (ADR-0027 §1) — the method must be a known enum member, and
		// only the two SLICE-1-WIRED methods are accepted: apiKeyHelper (M2,
		// requires helper:) and volume-token (M1, requires env:). Every other valid
		// enum member (env / volume-store+helper / oauth-file / interactive-login)
		// is a DECLARED-BUT-UNWIRED method: it FAILS CLOSED here with a clear "not
		// yet supported (slice 2+)" error — a declared credential is NEVER silently
		// no-op'd (the guardrail principle: an unhonorable credential must fail
		// LOUD, never leave the seat running unauthenticated). An unknown token is
		// rejected outright.
		if c := h.Credential; c != nil {
			errs = append(errs, validateCredential(agent, c)...)
		}
	}

	if cs := pb.ConfigSource; cs != nil {
		switch cs.Type {
		case "local":
			if cs.Path == "" {
				errs = append(errs, "config_source type local requires field: path")
			}
		case "git":
			if cs.URL == "" {
				errs = append(errs, "config_source type git requires field: url")
			}
		case "":
			errs = append(errs, "config_source requires field: type")
		default:
			errs = append(errs, fmt.Sprintf("invalid config_source.type %q (want local or git)", cs.Type))
		}
	}

	if len(errs) > 0 {
		// Sort so the message is deterministic regardless of map iteration order.
		sortStrings(errs)
		return fmt.Errorf("invalid playbook:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// validateCredential checks one agent's credential declaration (ADR-0027 §1).
// It returns the (possibly empty) list of error strings — fail-closed on any
// method that is not WIRED in slice 1, so a credential is never silently dropped.
//   - apiKeyHelper (M2): requires a non-empty helper: (the operator stdout-helper
//     command); env: is meaningless here.
//   - volume-token (M1): requires a non-empty env: (the env var NAME injected at
//     exec-time, e.g. CLAUDE_CODE_OAUTH_TOKEN); helper: is meaningless here.
//   - oauth-file (slice 2): a per-project :rw volume the agent's CLI refreshes at a
//     HOME-relative path: (default DefaultOAuthFilePath). path: must be HOME-relative
//     (no leading / and no .. traversal) so the mount can never escape $HOME;
//     helper:/env: are meaningless here.
//   - env / volume-store+helper / interactive-login: KNOWN enum members, but UNWIRED
//     in this slice — a hard error ("not yet supported, ADR-0027 slice 2+"), never a
//     silent no-op.
//   - anything else: an unknown method token — a hard error.
//
// NOTE: this validates the WIRING shape only. The credential VALUE is never here
// and is never authored/read/logged by loom (the human trust act, out of scope).
func validateCredential(agent string, c *Credential) []string {
	var errs []string
	switch c.Method {
	case CredAPIKeyHelper:
		if strings.TrimSpace(c.Helper) == "" {
			errs = append(errs, fmt.Sprintf("harness.%s.credential: method %q requires a non-empty helper:", agent, c.Method))
		}
	case CredVolumeToken:
		if strings.TrimSpace(c.Env) == "" {
			errs = append(errs, fmt.Sprintf("harness.%s.credential: method %q requires a non-empty env: (the env var NAME to inject)", agent, c.Method))
		} else if !envVarNameRe.MatchString(c.Env) {
			// The env: is the exec-time `-e <env>=<token>` target. A malformed name
			// (e.g. "FOO=BAR" or "FOO BAR") would mis-target the injection — reject it
			// to a single valid env-var NAME ([A-Za-z_][A-Za-z0-9_]*, no =/whitespace).
			errs = append(errs, fmt.Sprintf("harness.%s.credential: method %q env: %q is not a valid env var NAME (want [A-Za-z_][A-Za-z0-9_]*: a letter/underscore then letters/digits/underscores, no '=' or whitespace)", agent, c.Method, c.Env))
		}
	case CredOAuthFile:
		// oauth-file (slice 2): a per-project :rw volume the agent's CLI refreshes at
		// a HOME-relative path. path: is OPTIONAL (defaults to DefaultOAuthFilePath);
		// when present it MUST be HOME-relative — no leading '/' (would mount at an
		// absolute container path, escaping $HOME) and no '..' segment (traversal out
		// of $HOME). This is the one new validation oauth-file adds over M1/M2.
		if p := strings.TrimSpace(c.Path); p != "" && !validHomeRelPath(p) {
			errs = append(errs, fmt.Sprintf("harness.%s.credential: method %q path: %q must be HOME-relative (no leading '/', no '..' traversal) — the mount must stay inside $HOME", agent, c.Method, c.Path))
		}
	case CredEnv, CredVolumeStoreHelp, CredInteractiveLogin:
		errs = append(errs, fmt.Sprintf("harness.%s.credential: method %q is a known but not-yet-supported adapter (ADR-0027 slice 2+) — declaring it must fail closed, never silently run unauthenticated", agent, c.Method))
	case "":
		errs = append(errs, fmt.Sprintf("harness.%s.credential: missing required field: method", agent))
	default:
		errs = append(errs, fmt.Sprintf("harness.%s.credential: unknown method %q (want one of %s, %s, %s, %s, %s, %s)",
			agent, c.Method, CredEnv, CredAPIKeyHelper, CredVolumeToken, CredVolumeStoreHelp, CredOAuthFile, CredInteractiveLogin))
	}
	return errs
}

// validDeclaredRole reports whether r names a known loom-role permitted in a
// `roles:` declaration. It is an allowlist of declaration intent, distinct from
// the engine's validRole marker-safe charset check (container.go) — the former
// constrains WHAT may be declared, the latter constrains what is safe to write
// into the marker file.
func validDeclaredRole(r string) bool {
	return r == RoleAuthor || r == RoleAdvisor
}
