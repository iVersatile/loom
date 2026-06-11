package engine

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iVersatile/loom/internal/playbook"
)

// materializeResult is one dotfile written into $HOME.
type materializeResult struct {
	Display string // ~/-relative path, for reporting and audit
	Changed bool
}

// materializeDotfiles copies each dotfile reference from the config source into
// homeDir, mapping each reference to its $HOME location. Idempotent: a target
// whose content already matches is left untouched (Changed=false).
//
// This is what makes a custom shell prompt / Claude statusline SURVIVE a docker
// rebuild (ADR-0001, ADR-0006): build reconciles $HOME from the versioned config
// source every run, rather than relying on ad-hoc edits inside a container.
func materializeDotfiles(src fs.FS, refs []string, homeDir string) ([]materializeResult, error) {
	out := make([]materializeResult, 0, len(refs))
	for _, ref := range refs {
		data, err := fs.ReadFile(src, path.Join("dotfiles", ref))
		if err != nil {
			return nil, fmt.Errorf("dotfile %q: %w", ref, err)
		}
		changed, err := writeIfChanged(dotfileTarget(homeDir, ref), data, dotfileMode(ref))
		if err != nil {
			return nil, err
		}
		out = append(out, materializeResult{Display: dotfileDisplay(ref), Changed: changed})
	}
	return out, nil
}

// materializeHarness writes each agent's declared harness-home artifacts
// (SPEC-playbook#harness, ADR-0015 decision 3) into homeDir through the same
// write-if-changed pipeline as dotfiles — so the T7 home-digest sentinel
// covers them for free. Per agent namespace ("claude" -> <home>/.claude):
//
//	settings -> resolved from dotfiles/<ref>, whole-file to <agent home>/<base>
//	hooks    -> resolved from hooks/<name>, to <agent home>/hooks/<name>, 0755
//	skills   -> skills/<name>/ copied recursively to <agent home>/skills/<name>/
//
// Deterministic order (sorted agents, then settings, hooks, skills) so audit
// entries and --json output are stable across runs.
func materializeHarness(src fs.FS, harness map[string]playbook.HarnessAgent, homeDir string) ([]materializeResult, error) {
	agents := make([]string, 0, len(harness))
	for a := range harness {
		agents = append(agents, a)
	}
	sort.Strings(agents)

	var out []materializeResult
	for _, agent := range agents {
		h := harness[agent]
		agentHome := filepath.Join(homeDir, "."+agent)
		display := "~/." + agent + "/"

		if h.Settings != "" {
			data, err := fs.ReadFile(src, path.Join("dotfiles", h.Settings))
			if err != nil {
				return nil, fmt.Errorf("harness.%s.settings %q: %w", agent, h.Settings, err)
			}
			base := path.Base(h.Settings)
			changed, err := writeIfChanged(filepath.Join(agentHome, base), data, 0o644)
			if err != nil {
				return nil, err
			}
			out = append(out, materializeResult{Display: display + base, Changed: changed})
		}

		for _, name := range h.Hooks {
			data, err := fs.ReadFile(src, path.Join("hooks", name))
			if err != nil {
				return nil, fmt.Errorf("harness.%s.hooks %q: %w", agent, name, err)
			}
			changed, err := writeIfChanged(filepath.Join(agentHome, "hooks", name), data, 0o755)
			if err != nil {
				return nil, err
			}
			out = append(out, materializeResult{Display: display + "hooks/" + name, Changed: changed})
		}

		for _, name := range h.Skills {
			root := path.Join("skills", name)
			err := fs.WalkDir(src, root, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				data, err := fs.ReadFile(src, p)
				if err != nil {
					return err
				}
				rel := strings.TrimPrefix(p, root+"/")
				target := filepath.Join(agentHome, "skills", name, filepath.FromSlash(rel))
				changed, err := writeIfChanged(target, data, dotfileMode(p))
				if err != nil {
					return err
				}
				out = append(out, materializeResult{Display: display + "skills/" + name + "/" + rel, Changed: changed})
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("harness.%s.skills %q: %w", agent, name, err)
			}
		}
	}
	return out, nil
}

// dotfileTarget maps a reference to its absolute path under homeDir.
//
//	claude/<x>  -> <home>/.claude/<x>      (env-wide Claude config, e.g. statusline)
//	bash/<x>    -> <home>/.bashrc.d/<base> (per-project shell prompt)
//	<x>         -> <home>/<x>
func dotfileTarget(home, ref string) string {
	switch {
	case strings.HasPrefix(ref, "claude/"):
		return filepath.Join(home, ".claude", strings.TrimPrefix(ref, "claude/"))
	case strings.HasPrefix(ref, "bash/"):
		return filepath.Join(home, ".bashrc.d", filepath.Base(ref))
	default:
		return filepath.Join(home, ref)
	}
}

func dotfileDisplay(ref string) string {
	switch {
	case strings.HasPrefix(ref, "claude/"):
		return "~/.claude/" + strings.TrimPrefix(ref, "claude/")
	case strings.HasPrefix(ref, "bash/"):
		return "~/.bashrc.d/" + filepath.Base(ref)
	default:
		return "~/" + ref
	}
}

// dotfileMode makes shell scripts executable; everything else is a plain file.
func dotfileMode(ref string) os.FileMode {
	if strings.HasSuffix(ref, ".sh") {
		return 0o755
	}
	return 0o644
}

func writeIfChanged(target string, data []byte, mode os.FileMode) (bool, error) {
	if existing, err := os.ReadFile(target); err == nil && bytes.Equal(existing, data) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(target, data, mode); err != nil {
		return false, err
	}
	return true, nil
}
