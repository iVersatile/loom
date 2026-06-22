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

// credFilePresenceProbe is the MOCKABLE SEAM the oauth-file doctor check resolves
// through (the doctor analogue of credential.go's tokenReader) — existence+non-empty
// ONLY, never the body. dockerRuntime.CredFilePresent is the prod impl; tests supply
// a fake. Keeping it narrow keeps the secrets boundary explicit: the doctor seat can
// ask "is the creds file there and non-empty?" and NOTHING else.
type credFilePresenceProbe interface {
	CredFilePresent(name, path string) (bool, error)
}

// oauthFileCredentialChecks mechanizes the ADR-0027 slice-2 fail-closed doctor guard:
// for each declared oauth-file credential, the per-project volume's creds FILE must be
// PRESENT and NON-EMPTY at its HOME-relative mount path (the slice-1 empty-token guard,
// mirrored at verify time). It probes `<home>/<path>/oauth_creds.json` via the
// non-reading CredFilePresent seam — the body is NEVER read or logged (only
// existence/non-empty crosses the boundary). A missing or empty creds file FAILS
// CLOSED (a declared-but-unprovisioned credential must not pass silently); a probe
// failure (no daemon) is surfaced as a non-OK check, never a false pass.
//
// LIVE-only: it reads in-container state, so the caller gates it on a running
// container (doctor never Starts one to ask). Returns (checks, true) when at least
// one oauth-file credential is declared; (nil, false) when there is nothing to grade
// (a non-oauth-file build is byte-identical).
func oauthFileCredentialChecks(rt credFilePresenceProbe, container, home string, pb playbook.Playbook) ([]Check, bool) {
	agents := oauthFileAgents(pb.Harness)
	if len(agents) == 0 {
		return nil, false
	}
	var checks []Check
	for _, a := range agents {
		credsFile := home + "/" + a.Path + "/" + oauthCredsFileName
		name := "container:credential:oauth-file:" + a.Agent
		present, err := rt.CredFilePresent(container, credsFile)
		switch {
		case err != nil:
			checks = append(checks, Check{Name: name, OK: false, Detail: fmt.Sprintf(
				"could not probe the oauth-file credential (Gemini's %s) at %s (%v) — refusing to assume it is present (ADR-0027 fail-closed)", oauthCredsFileName, credsFile, err)})
		case !present:
			checks = append(checks, Check{Name: name, OK: false, Detail: fmt.Sprintf(
				"oauth-file credential for agent %q is missing or empty at %s — this check probes Gemini's %s specifically (gcloud-ADC's application_default_credentials.json is the follow-on c1 slice); provision it (a one-time browser login into the per-project volume %s) or remove the credential: declaration; refusing to pass an unprovisioned credential (ADR-0027 fail-closed)",
				a.Agent, credsFile, oauthCredsFileName, credentialVolume(container, a.Agent))})
		default:
			checks = append(checks, Check{Name: name, OK: true, Detail: fmt.Sprintf(
				"oauth-file credential present + non-empty at %s (Gemini's %s, volume %s, :rw)", credsFile, oauthCredsFileName, credentialVolume(container, a.Agent))})
		}
	}
	return checks, true
}

// oauthCredsFileName is the creds file the oauth-file doctor check probes for inside
// the mounted volume directory — Gemini's OAuth login writes oauth_creds.json. The
// probe is existence+non-empty only; the name is the well-known Gemini default (gcloud
// ADC's application_default_credentials.json variant is the follow-on c1/SA-JSON slice,
// out of scope here).
const oauthCredsFileName = "oauth_creds.json"
