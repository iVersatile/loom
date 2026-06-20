package engine

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iVersatile/loom/internal/playbook"
	"sigs.k8s.io/yaml"
)

// draftPlaybookName is the distinct file --emit-playbook writes its draft to —
// NEVER the canonical loom.yml, so the carry-forward draft can be reviewed then
// committed without clobbering an authored playbook.
const draftPlaybookName = "loom.base.draft.yml"

// Detect reads current reality and never mutates (docs/SPEC-verbs.md "detect").
func Detect(opts DetectOpts) (DetectResult, error) {
	return detectImpl(opts, execProber{})
}

func detectImpl(opts DetectOpts, p prober) (DetectResult, error) {
	res := DetectResult{
		Tools:       []Tool{},
		Agents:      []Agent{},
		Credentials: []Credential{},
		Projects:    []Project{},
		Drift:       []Drift{},
	}

	path := opts.PlaybookPath
	if path == "" {
		path = defaultPlaybookPath
	}

	// A playbook is optional: detect must also orient on a bare machine. When one
	// is present it narrows what we probe and lets us compute drift.
	var desired *playbook.Playbook
	if resolved, err := playbook.Load(path); err == nil {
		desired = resolved.Playbook
		abs, _ := filepath.Abs(resolved.Root)
		res.Projects = append(res.Projects, Project{Name: desired.Name, Path: abs, Stack: desired.Stack})
	}

	toolIntents := defaultProbeTools
	agentNames := defaultAgents
	var envNames []string
	if desired != nil {
		if len(desired.Tools) > 0 {
			toolIntents = desired.Tools
		}
		if len(desired.Agents) > 0 {
			agentNames = desired.Agents
		}
		envNames = desired.Env
	}

	for _, intent := range toolIntents {
		name, want := playbook.SplitTool(intent)
		present, version := p.probe(playbook.BinaryName(name))
		res.Tools = append(res.Tools, Tool{Name: name, Present: present, Version: version})
		if desired == nil {
			continue
		}
		switch {
		case !present:
			res.Drift = append(res.Drift, Drift{Tool: name, Want: playbook.WantOrLatest(want), Have: nil})
		case !versionSatisfies(want, version):
			have := version
			res.Drift = append(res.Drift, Drift{Tool: name, Want: want, Have: &have})
		}
	}

	for _, a := range agentNames {
		present, _ := p.probe(playbook.BinaryName(a))
		res.Agents = append(res.Agents, Agent{Name: a, Present: present})
	}

	res.Credentials = detectCredentials(envNames)

	if opts.EmitPlaybook {
		out, err := emitDraftPlaybook(res, path)
		if err != nil {
			return res, fmt.Errorf("emit draft playbook: %w", err)
		}
		res.Emitted = out
	}
	return res, nil
}

// emitDraftPlaybook writes the detected machine as a DRAFT base playbook (the
// continuity / carry-forward output spec'd at docs/SPEC-verbs.md#detect). Detect's
// no-mutation contract is about system/container state; writing a review-then-commit
// draft FILE — under a distinct name (loom.base.draft.yml), never the canonical
// loom.yml — is the spec'd continuity output, not a state mutation. It returns the
// path written.
func emitDraftPlaybook(res DetectResult, playbookPath string) (string, error) {
	// Refuse to clobber the canonical playbook with its own draft.
	if filepath.Base(playbookPath) == draftPlaybookName {
		return "", fmt.Errorf("refusing to overwrite %q with its own draft", playbookPath)
	}

	pb := playbook.Playbook{Loom: playbook.SchemaVersion, Tier: playbook.TierBase}

	for _, t := range res.Tools {
		if !t.Present {
			continue
		}
		// The draft carries presence reliably; the version is best-effort intent
		// (the resolver/lockfile pins the exact build version, as everywhere).
		// Prober versions are messy multi-token lines ("git version 2.39.5",
		// "go version go1.26.4 linux/arm64"), so recover a usable token when we
		// can, else emit the bare name rather than a malformed intent.
		if v := cleanVersion(t.Version); v != "" {
			pb.Tools = append(pb.Tools, t.Name+"@"+v)
		} else {
			pb.Tools = append(pb.Tools, t.Name)
		}
	}

	for _, a := range res.Agents {
		if a.Present {
			pb.Agents = append(pb.Agents, a.Name)
		}
	}

	// Credentials carry NAMES ONLY — never values, never their found-in locations.
	for _, c := range res.Credentials {
		pb.Env = append(pb.Env, c.Name)
	}

	if err := pb.Validate(); err != nil {
		return "", err
	}

	data, err := yaml.Marshal(&pb)
	if err != nil {
		return "", err
	}

	out := filepath.Join(filepath.Dir(playbookPath), draftPlaybookName)
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return "", err
	}
	return out, nil
}

// cleanVersion recovers a usable version token from a prober's first --version
// line. Tools format versions inconsistently ("git version 2.39.5",
// "go version go1.26.4 linux/arm64", "jq-1.6"), so we take the first
// whitespace-delimited token that BEGINS with a digit — the common
// "<tool> version <N.N.N>" shape — and otherwise return "" so the caller emits
// the bare name rather than a malformed intent (e.g. never "jq@jq-1.6"). Exact
// pinning is the resolver/lockfile's job; the draft only needs a clean intent.
func cleanVersion(raw string) string {
	for _, tok := range strings.Fields(raw) {
		if tok[0] >= '0' && tok[0] <= '9' {
			return tok
		}
	}
	return ""
}

// detectCredentials reports WHERE each named credential is found, never its
// value (detect-and-report only; --migrate is Phase 2). Phase 1 scans process
// env and common shell rc files; Keychain (macOS) is out of scope here.
func detectCredentials(names []string) []Credential {
	creds := []Credential{}
	home, _ := os.UserHomeDir()
	rcFiles := []string{".bashrc", ".zshrc", ".profile"}
	for _, name := range names {
		var found []string
		if os.Getenv(name) != "" {
			found = append(found, "env")
		}
		for _, rc := range rcFiles {
			if fileContains(filepath.Join(home, rc), name) {
				found = append(found, "~/"+rc)
			}
		}
		if len(found) > 0 {
			creds = append(creds, Credential{Name: name, FoundIn: found})
		}
	}
	return creds
}

func fileContains(path, needle string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte(needle))
}
