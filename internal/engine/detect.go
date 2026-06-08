package engine

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/iVersatile/loom/internal/playbook"
)

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
		present, version := p.probe(binaryName(name))
		res.Tools = append(res.Tools, Tool{Name: name, Present: present, Version: version})
		if desired == nil {
			continue
		}
		switch {
		case !present:
			res.Drift = append(res.Drift, Drift{Tool: name, Want: wantOrLatest(want), Have: nil})
		case !versionSatisfies(want, version):
			have := version
			res.Drift = append(res.Drift, Drift{Tool: name, Want: want, Have: &have})
		}
	}

	for _, a := range agentNames {
		present, _ := p.probe(binaryName(a))
		res.Agents = append(res.Agents, Agent{Name: a, Present: present})
	}

	res.Credentials = detectCredentials(envNames)
	return res, nil
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
