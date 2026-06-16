package playbook

import (
	"fmt"
	"strings"
)

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
