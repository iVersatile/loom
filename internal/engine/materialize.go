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

// homeFile is one file the staged $HOME must carry: the declared content/mode
// at its target path. Computing the expected set separately from writing it
// lets read-only verbs (plan, doctor) grade the staging dir against the config
// source without mutating it — the C1 class: declared guardrails must be
// verifiable as WIRED, not just present in the source.
type homeFile struct {
	Target  string // absolute path under the staging dir
	Display string // ~/-relative, for reporting and audit
	Data    []byte
	Mode    os.FileMode
}

// expectedDotfiles resolves each dotfile reference to the staged file it
// implies, mapping each reference to its $HOME location.
func expectedDotfiles(src fs.FS, refs []string, homeDir string) ([]homeFile, error) {
	out := make([]homeFile, 0, len(refs))
	for _, ref := range refs {
		data, err := fs.ReadFile(src, path.Join("dotfiles", ref))
		if err != nil {
			return nil, fmt.Errorf("dotfile %q: %w", ref, err)
		}
		out = append(out, homeFile{
			Target: dotfileTarget(homeDir, ref), Display: dotfileDisplay(ref),
			Data: data, Mode: dotfileMode(ref),
		})
	}
	return out, nil
}

// expectedHarness resolves each agent's declared harness-home artifacts
// (SPEC-playbook#harness, ADR-0015 decision 3) to the staged files they imply.
// Per agent namespace ("claude" -> <home>/.claude):
//
//	settings -> resolved from dotfiles/<ref>, whole-file to <agent home>/<base>
//	trust    -> resolved from dotfiles/<ref>, whole-file to <home>/.<agent>.json
//	hooks    -> resolved from hooks/<name>, to <agent home>/hooks/<name>, 0755
//	skills   -> skills/<name>/ copied recursively to <agent home>/skills/<name>/
//
// Deterministic order (sorted agents, then settings, trust, hooks, skills) so
// audit entries and --json output are stable across runs.
func expectedHarness(src fs.FS, harness map[string]playbook.HarnessAgent, homeDir string) ([]homeFile, error) {
	agents := make([]string, 0, len(harness))
	for a := range harness {
		agents = append(agents, a)
	}
	sort.Strings(agents)

	var out []homeFile
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
			out = append(out, homeFile{
				Target: filepath.Join(agentHome, base), Display: display + base,
				Data: data, Mode: 0o644,
			})
		}

		// Trust/opt-in flags target <home>/.<agent>.json — a SIBLING of the
		// agent home, on the container overlay rather than the agent-home
		// volume, so it dies on recreate; the home re-sync (T7 digest) is
		// what restores it (036 ruling, ADR-0018).
		if h.Trust != "" {
			data, err := fs.ReadFile(src, path.Join("dotfiles", h.Trust))
			if err != nil {
				return nil, fmt.Errorf("harness.%s.trust %q: %w", agent, h.Trust, err)
			}
			out = append(out, homeFile{
				Target: filepath.Join(homeDir, "."+agent+".json"), Display: "~/." + agent + ".json",
				Data: data, Mode: 0o644,
			})
		}

		for _, name := range h.Hooks {
			data, err := fs.ReadFile(src, path.Join("hooks", name))
			if err != nil {
				return nil, fmt.Errorf("harness.%s.hooks %q: %w", agent, name, err)
			}
			out = append(out, homeFile{
				Target: filepath.Join(agentHome, "hooks", name), Display: display + "hooks/" + name,
				Data: data, Mode: 0o755,
			})
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
				out = append(out, homeFile{
					Target:  filepath.Join(agentHome, "skills", name, filepath.FromSlash(rel)),
					Display: display + "skills/" + name + "/" + rel,
					Data:    data, Mode: dotfileMode(p),
				})
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("harness.%s.skills %q: %w", agent, name, err)
			}
		}
	}
	return out, nil
}

// expectedPathDotfiles is the engine-generated shell-config set (T4): the
// PATH lines that used to be ad-hoc appends to /root/.profile inside the
// provision script. Generated into ~/.bashrc.d so ONE dotfile dir owns shell
// config: they ride the same staging dir as declared dotfiles (the T7 home
// digest covers drift, plan grades them, the audit log names them) and the
// unconditional shell-init loader sources them in login AND interactive
// shells. $HOME, never /root — the same file works under T10's non-root user.
// Keyed on the resolved install sources, mirroring provisionScript's needGo.
func expectedPathDotfiles(tools []ToolInstall, agents []AgentInstall, homeDir string) []homeFile {
	var out []homeFile
	add := func(base, line string) {
		out = append(out, homeFile{
			Target:  filepath.Join(homeDir, ".bashrc.d", base),
			Display: "~/.bashrc.d/" + base,
			Data:    []byte(line + "\n"),
			Mode:    0o755,
		})
	}
	for _, t := range tools {
		if t.Source == "go-tarball" || t.Source == "go-install" {
			add("path.go.sh", `export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"`)
			break
		}
	}
	for _, a := range agents {
		if a.Name == "claude-code" {
			add("path.local.sh", `export PATH="$PATH:$HOME/.local/bin"`)
			break
		}
	}
	return out
}

// materializeFiles writes an expected file set through the write-if-changed
// pipeline. Idempotent: a target whose content already matches is left
// untouched (Changed=false).
func materializeFiles(files []homeFile) ([]materializeResult, error) {
	out := make([]materializeResult, 0, len(files))
	for _, f := range files {
		changed, err := writeIfChanged(f.Target, f.Data, f.Mode)
		if err != nil {
			return nil, err
		}
		out = append(out, materializeResult{Display: f.Display, Changed: changed})
	}
	return out, nil
}

// materializeDotfiles copies each dotfile reference from the config source into
// homeDir. This is what makes a custom shell prompt / Claude statusline SURVIVE
// a docker rebuild (ADR-0001, ADR-0006): build reconciles $HOME from the
// versioned config source every run, rather than relying on ad-hoc edits inside
// a container.
func materializeDotfiles(src fs.FS, refs []string, homeDir string) ([]materializeResult, error) {
	files, err := expectedDotfiles(src, refs, homeDir)
	if err != nil {
		return nil, err
	}
	return materializeFiles(files)
}

// materializeHarness writes each agent's declared harness-home artifacts into
// homeDir through the same write-if-changed pipeline as dotfiles — so the T7
// home-digest sentinel covers them for free.
func materializeHarness(src fs.FS, harness map[string]playbook.HarnessAgent, homeDir string) ([]materializeResult, error) {
	files, err := expectedHarness(src, harness, homeDir)
	if err != nil {
		return nil, err
	}
	return materializeFiles(files)
}

// stagedHomeDrift reports the ~/-relative names of declared $HOME artifacts
// (dotfiles + harness + the engine-generated set, e.g. T4 PATH dotfiles)
// whose staged copy under homeDir is missing, stale, or carries the wrong
// mode — a read-only dry run of materialize, for plan and doctor (F2/C1: a
// verdict about wiring must grade the same surface build converges).
func stagedHomeDrift(src fs.FS, pb *playbook.Playbook, generated []homeFile, homeDir string) ([]string, error) {
	expected, err := expectedDotfiles(src, pb.Dotfiles, homeDir)
	if err != nil {
		return nil, err
	}
	hfiles, err := expectedHarness(src, pb.Harness, homeDir)
	if err != nil {
		return nil, err
	}
	expected = append(expected, hfiles...)
	expected = append(expected, generated...)

	// Content-only comparison, mirroring writeIfChanged exactly: drift here
	// IS "build would write this file" — verdict and action share a domain
	// (F2/LL-012 doctrine), so a difference build would not fix (e.g. a
	// hand-chmod'd staged file) must not loop plan at exit 2 forever.
	var drift []string
	for _, f := range expected {
		data, err := os.ReadFile(f.Target)
		if err != nil || !bytes.Equal(data, f.Data) {
			drift = append(drift, f.Display)
		}
	}
	return drift, nil
}

// dotfileTarget maps a reference to its absolute path under homeDir.
//
//	claude/<x>  -> <home>/.claude/<x>      (env-wide Claude config, e.g. statusline)
//	bash/<x>    -> <home>/.bashrc.d/<base> (per-project shell prompt)
//	gitconfig   -> <home>/.gitconfig       (declared git identity, T16 PR 3 —
//	                                        harness config by weight, plain
//	                                        dotfile by shape per SPEC-playbook;
//	                                        named, not hidden, in the source)
//	<x>         -> <home>/<x>
func dotfileTarget(home, ref string) string {
	switch {
	case strings.HasPrefix(ref, "claude/"):
		return filepath.Join(home, ".claude", strings.TrimPrefix(ref, "claude/"))
	case strings.HasPrefix(ref, "bash/"):
		return filepath.Join(home, ".bashrc.d", filepath.Base(ref))
	case ref == "gitconfig":
		return filepath.Join(home, ".gitconfig")
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
	case ref == "gitconfig":
		return "~/.gitconfig"
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
