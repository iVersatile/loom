package engine

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
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
