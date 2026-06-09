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
