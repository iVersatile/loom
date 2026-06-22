// credential — the claude credential adapters (ADR-0027 slice 1): M2
// (apiKeyHelper → a targeted key-set on the materialized settings.json) and M1
// (volume-token → exec-time `-e ENV=TOKEN` injection from a per-project volume).
//
// THE SECRET INVARIANT (ADR-0027): loom wires the MECHANISM, never the VALUE. M2
// writes an operator COMMAND (e.g. `op read op://...`) into settings.json — loom
// never runs it or sees its stdout. M1 reads a HUMAN-PROVISIONED token from a
// per-project volume at exec-time and injects it onto the AGENT's `docker exec`
// only — the value is never authored, never logged, and (critically) NEVER on
// `docker run` (the ADR-0014 Config.Env / `docker inspect` leak — see
// credentialExecEnv).
package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iVersatile/loom/internal/playbook"
)

// credMountPath is the FIXED in-container path the per-project credential volume
// mounts at. The M1 token lives at <credMountPath>/token (the consumer reads it;
// loom only reads it at exec-time to inject the env). Distinct from the agent
// home so it never shadows ~/.claude materialized config.
const credMountPath = "/var/lib/loom/cred"

// credTokenFile is the volume-token file the M1 adapter reads (the
// human-provisioned token; loom never writes it).
const credTokenFile = credMountPath + "/token"

// credentialVolume names the per-project, per-agent credential volume (ADR-0027
// invariant #1: per-project namespacing + slug-uniqueness). `<container>-<agent>-cred`,
// the SAME `<container>-<service>` family as agentHomeVolume (`-claude`) and
// ghConfigVolume (`-gh`); slice 1 builds ONLY this one volume — the general
// serviceVolume refactor is deferred (ADR-0027 §Realization). The per-project
// container boundary isolates one project's credential from another's. Like the
// other credential volumes it is human-provisioned and removed only by the opt-in
// --clean-state (never a tier side effect — see Teardown).
func credentialVolume(container, agent string) string {
	return container + "-" + agent + "-cred"
}

// credentialVolumeSuffix is the family suffix the per-project credential volumes
// share (`<container>-<agent>-cred`) — the discriminator Teardown discovers them
// by without playbook context.
const credentialVolumeSuffix = "-cred"

// discoverCredentialVolumes lists the per-project credential volumes for a
// container by name shape (`<name>-<agent>-cred`) so --clean-state wipes any
// declared agent's volume without the playbook. Best-effort: a docker/list error
// yields nothing (Teardown's volume-rm is itself best-effort + idempotent). NOTE:
// integration-validated (docker host), not the local gate.
func discoverCredentialVolumes(container string) []string {
	out, err := dockerVolumeLs()
	if err != nil {
		return nil
	}
	prefix := container + "-"
	var vols []string
	for _, v := range out {
		if strings.HasPrefix(v, prefix) && strings.HasSuffix(v, credentialVolumeSuffix) {
			vols = append(vols, v)
		}
	}
	sort.Strings(vols)
	return vols
}

// agentVolumeKey is the canonical credential-namespace slug for an (container,
// agent) pair — the doctor slug-uniqueness invariant grades collisions on THIS
// key (the volume name minus the family suffix). Identical to credentialVolume
// today; named separately so the doctor check reads as "the key", and so a future
// slug scheme changes one place.
func agentVolumeKey(container, agent string) string {
	return credentialVolume(container, agent)
}

// volumeTokenAgents returns the agents (sorted-stable via the caller) whose
// resolved credential method is volume-token (M1) — the set needing a
// per-project credential volume mounted at create and an exec-time injection.
// apiKeyHelper (M2) needs no volume (the secret is store-owned, reached by the
// helper command), so it is NOT included.
func volumeTokenAgents(harness map[string]playbook.HarnessAgent) map[string]*playbook.Credential {
	out := map[string]*playbook.Credential{}
	for agent, h := range harness {
		if c := h.Credential; c != nil && c.Method == playbook.CredVolumeToken {
			out[agent] = c
		}
	}
	return out
}

// volumeTokenAgentNames returns the sorted-stable names of the M1 (volume-token)
// agents — the create-time view (ContainerSpec.CredentialVolumeAgents) of which
// agents need the per-project credential volume mounted.
func volumeTokenAgentNames(harness map[string]playbook.HarnessAgent) []string {
	m1 := volumeTokenAgents(harness)
	if len(m1) == 0 {
		return nil
	}
	names := make([]string, 0, len(m1))
	for agent := range m1 {
		names = append(names, agent)
	}
	sort.Strings(names)
	return names
}

// credentialVolumeMounts returns the `docker run -v <vol>:<path>:ro` args for the
// M1 (volume-token) agents (ADR-0027). This is a VOLUME mount, not `-e`: a `-v`
// mount does NOT enter Config.Env / `docker inspect` (no value persists), so it is
// leak-safe at create-time — the create-time concern is the run-`-e` LEAK, which
// M1 deliberately avoids (the token is injected at EXEC time instead; see
// credentialExecEnv). The mount only makes the human-provisioned volume REACHABLE;
// nothing is read or logged here. `:ro` so the agent seat can READ the token, not
// rewrite it.
func credentialVolumeMounts(name string, agents []string) []string {
	if len(agents) == 0 {
		return nil
	}
	// Slice 1 wires the single claude adapter, so one shared mount point suffices
	// (the volume name is per-agent; the in-container mount path is fixed). The
	// general per-agent multi-mount is the deferred serviceVolume refactor.
	return []string{"-v", credentialVolume(name, agents[0]) + ":" + credMountPath + ":ro"}
}

// credEnvPair formats one exec-time `-e NAME=VALUE` env injection. Isolated so the
// VALUE never flows through a log/print path: the only caller is credentialExecEnv,
// and the only consumer is the agent's `docker exec` argv (NOT `docker run`).
func credEnvPair(name, value string) string { return name + "=" + value }

// tokenReader reads the M1 volume-token from inside the container — the MOCKABLE
// SEAM (ADR-0027) that lets unit tests assert the exec-time `-e` injection WITHOUT
// a real secret. The production impl is dockerRuntime.ReadCredentialToken
// (`docker exec <name> cat <credTokenFile>`); tests supply a fake returning
// FAKE-TOKEN. fail-closed: a read error (absent volume/token) propagates so the
// caller refuses the seat rather than running unauthenticated.
type tokenReader interface {
	ReadCredentialToken(container, path string) (string, error)
}

// credentialExecEnv resolves the exec-time credential env for an entry verb
// (exec/shell) — the M1 (volume-token) injection (ADR-0027). It returns the
// `NAME=VALUE` pairs to append as `docker exec -e` args on the AGENT's invocation.
//
// LEAK-AVOIDANCE (ADR-0014, the core of M1): the token is injected at EXEC time,
// NEVER on `docker run`. A run-`-e` value persists into the container's Config.Env
// and shows in `docker inspect` (the ADR-0014 leak); an exec-`-e` value is scoped
// to that single `docker exec` process and persists NOWHERE. So the token reaches
// the agent without ever entering create-time state.
//
// FAIL-CLOSED (ADR-0027 invariant #3): if the volume/token is absent the read
// errors and this returns that error — the caller refuses to run (no silent
// unauthenticated seat). An EMPTY token is also a hard error (a provisioned-but-
// empty volume must not pass as "authenticated").
//
// The token VALUE is never logged: it lives only in the returned slice, consumed
// directly into the exec argv. Errors name the env/volume, never the value.
func credentialExecEnv(rt tokenReader, container string, harness map[string]playbook.HarnessAgent) ([]string, error) {
	names := volumeTokenAgentNames(harness) // sorted-stable
	if len(names) == 0 {
		return nil, nil
	}
	// Slice 1 wires the single claude adapter, so the first (sorted-stable) M1 agent
	// is the one injected; the general multi-agent injection is the deferred
	// serviceVolume refactor (ADR-0027 §Realization).
	agent := names[0]
	c := harness[agent].Credential
	token, err := rt.ReadCredentialToken(container, credTokenFile)
	if err != nil {
		return nil, fmt.Errorf("credential (volume-token) for agent %q: token unavailable at %s — provision the per-project credential volume %s or remove the credential: declaration; refusing to run unauthenticated (ADR-0027 fail-closed): %w",
			agent, credTokenFile, credentialVolume(container, agent), err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("credential (volume-token) for agent %q: token at %s is empty — refusing to inject an empty %s (ADR-0027 fail-closed)",
			agent, credTokenFile, c.Env)
	}
	return []string{credEnvPair(c.Env, token)}, nil
}

// applyAPIKeyHelper is the M2 adapter (ADR-0027): a TARGETED key-set of the
// `apiKeyHelper` key on the ALREADY-MATERIALIZED settings.json in the staging home
// (homeDir/.<agent>/settings.json). Phase-1 settings.json is WHOLE-FILE /
// base-only / no-key-merge (SPEC-playbook Open question 1), so the credential is a
// DECLARED CARVE-OUT — exactly like trust was — NOT a general key-merge: it reads
// the materialized file, sets the ONE apiKeyHelper key, and writes it back.
//
// The helper is an operator COMMAND (config, e.g. `op read op://vault/key`); loom
// never RUNS it and never sees its stdout — it only places the wiring. No-op for
// any agent without an apiKeyHelper credential (so a non-M2 build is byte-identical).
//
// It runs AFTER materializeHarness (which wrote the base settings.json), so the
// carve-out lands on top of the declared file. Idempotent: re-running with the
// same helper leaves the file byte-identical (writeIfChanged semantics).
func applyAPIKeyHelper(harness map[string]playbook.HarnessAgent, homeDir string) ([]materializeResult, error) {
	var out []materializeResult
	for _, agent := range sortedAgents(harness) {
		c := harness[agent].Credential
		if c == nil || c.Method != playbook.CredAPIKeyHelper {
			continue
		}
		base := harness[agent].Settings
		if base == "" {
			return nil, fmt.Errorf("harness.%s.credential method apiKeyHelper requires harness.%s.settings (the file the apiKeyHelper key is set on)", agent, agent)
		}
		target := filepath.Join(homeDir, "."+agent, filepath.Base(base))
		changed, err := setSettingsKey(target, "apiKeyHelper", c.Helper)
		if err != nil {
			return nil, fmt.Errorf("harness.%s.credential (apiKeyHelper): %w", agent, err)
		}
		out = append(out, materializeResult{Display: "~/." + agent + "/" + filepath.Base(base) + " [apiKeyHelper]", Changed: changed})
	}
	return out, nil
}

// setSettingsKey reads the materialized settings.json, sets ONE top-level key to
// value (a string), and writes it back preserving every other key — the targeted
// key-set the M2 carve-out needs (NOT a whole-file clobber, NOT a deep merge).
// Returns whether the file changed (writeIfChanged semantics: an identical result
// is left untouched). The value here is an operator command, NOT a secret.
func setSettingsKey(target, key, value string) (bool, error) {
	data, err := os.ReadFile(target)
	if err != nil {
		return false, fmt.Errorf("read materialized settings %s: %w", target, err)
	}
	// Preserve key order is not required (settings.json is a flat object the
	// harness reads by key); a map round-trip with sorted keys is deterministic.
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return false, fmt.Errorf("settings %s is not a JSON object: %w", target, err)
	}
	if doc == nil {
		doc = map[string]json.RawMessage{}
	}
	enc, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	doc[key] = enc
	// Re-encode deterministically (json.Marshal sorts map keys) with a trailing
	// newline, matching the materialize pipeline's file shape.
	outBytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return false, err
	}
	outBytes = append(outBytes, '\n')
	return writeIfChanged(target, outBytes, 0o644)
}
