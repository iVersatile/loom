package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/iVersatile/loom/internal/playbook"
)

// credentialSlugChecks mechanizes ADR-0027 invariant #1 — per-project namespacing
// + SLUG-UNIQUENESS (doctor-checked). The per-project credential volume key
// (`<container>-<agent>-cred`, agentVolumeKey) is what isolates one project's
// credential from another's; if two distinct (project, agent) pairs could derive
// the SAME key, two projects would cross-wire onto one credential store — a
// silent multi-tenant secret leak. This check fail-closes that class:
//
//   - the project slug (the container name component) must be a valid, single
//     volume-name token — no whitespace, no '/' or ':' (which would break the
//     `<container>-<agent>-cred` derivation or smuggle a path into a docker-volume
//     name);
//   - the derived credential volume key must be COLLISION-UNAMBIGUOUS across the
//     declared M1 agents: two agents must not derive the same key (they can't, given
//     distinct agent names, but a malformed agent token sharing the separator could),
//     and the key must end in the `-cred` family suffix it is discovered by.
//
// Graded host-side (static over the merged playbook — no container needed) and only
// when at least one agent declares the M1 volume-token method (the only adapter that
// provisions a per-project credential volume in slice 1). Returns (check, true) when
// applicable; (zero, false) when there is nothing to grade.
func credentialSlugChecks(pb playbook.Playbook) (Check, bool) {
	m1 := volumeTokenAgentNames(pb.Harness)
	if len(m1) == 0 {
		return Check{}, false
	}
	cname := containerName(pb.Name)

	// The container name carries the project slug; an invalid slug poisons every
	// derived volume key. validVolumeToken rejects the separators that would make
	// the `<container>-<agent>-cred` derivation ambiguous or the docker-volume name
	// invalid.
	if !validVolumeToken(pb.Name) {
		return Check{Name: "host:credential-slug", OK: false, Detail: fmt.Sprintf(
			"project slug %q is not a valid credential-volume token (no whitespace, no / or :) — it would poison the per-project credential namespace (ADR-0027 invariant #1)", pb.Name)}, true
	}

	seen := map[string]string{} // key -> agent that produced it
	for _, agent := range m1 {
		if !validVolumeToken(agent) {
			return Check{Name: "host:credential-slug", OK: false, Detail: fmt.Sprintf(
				"agent token %q is not a valid credential-volume token (no whitespace, no / or :) (ADR-0027 invariant #1)", agent)}, true
		}
		key := agentVolumeKey(cname, agent)
		if !strings.HasSuffix(key, credentialVolumeSuffix) {
			return Check{Name: "host:credential-slug", OK: false, Detail: fmt.Sprintf(
				"derived credential volume key %q lost its %q family suffix — teardown --clean-state could not discover it (ADR-0027)", key, credentialVolumeSuffix)}, true
		}
		if prev, dup := seen[key]; dup {
			return Check{Name: "host:credential-slug", OK: false, Detail: fmt.Sprintf(
				"credential volume key collision: agents %q and %q both derive %q — two seats would cross-wire onto one credential store (ADR-0027 invariant #1, fail-closed)", prev, agent, key)}, true
		}
		seen[key] = agent
	}

	return Check{Name: "host:credential-slug", OK: true, Detail: fmt.Sprintf(
		"per-project credential volume key(s) unique + well-formed: %v", sortedKeys(seen))}, true
}

// validVolumeToken accepts a single docker-volume-name token: non-empty, no
// whitespace, and none of the separators ('/' ':') that would break the
// `<container>-<agent>-cred` derivation or be illegal in a docker volume name. It
// is the credential-slug analogue of the schema's user:/role: single-token rule.
func validVolumeToken(s string) bool {
	return s != "" && strings.TrimSpace(s) == s && !strings.ContainsAny(s, " \t\r\n/:")
}

// sortedKeys returns the map keys in stable order for a deterministic detail line.
func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
