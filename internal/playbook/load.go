package playbook

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/iVersatile/loom/internal/source"
)

// Layer file names within the config source.
const (
	baseFile      = "playbook.yml"
	stackFragment = "playbook.fragment.yml"
	stacksDir     = "stacks"
	overlaysDir   = "overlays"
)

// Resolved is the effective desired state plus the config-source filesystem the
// engine reads referenced content (rules/hooks/dotfiles) from.
type Resolved struct {
	Playbook *Playbook
	Source   fs.FS
	Root     string // directory containing the project playbook
}

// Load reads a project playbook and produces the merged, validated desired-state
// playbook by layering base → stack/<lang> → overlay/<project> → project
// (ADR-0004). The config source is resolved from the project playbook's
// config_source (ADR-0006).
func Load(projectPlaybookPath string) (*Resolved, error) {
	project, err := ParseFile(projectPlaybookPath)
	if err != nil {
		return nil, err
	}
	if err := project.Validate(); err != nil {
		return nil, err
	}
	if project.ConfigSource == nil {
		return nil, fmt.Errorf("%s: project playbook must declare config_source to resolve layers", projectPlaybookPath)
	}

	root := filepath.Dir(projectPlaybookPath)
	cs := project.ConfigSource
	fsys, err := source.Resolve(root, cs.Type, cs.Path)
	if err != nil {
		return nil, err
	}

	base, err := ParseFS(fsys, baseFile)
	if err != nil {
		return nil, fmt.Errorf("base layer: %w", err)
	}
	layers := []*Playbook{base}

	if project.Stack != "" {
		name := filepath.Join(stacksDir, project.Stack, stackFragment)
		frag, err := ParseFS(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("stack layer %q: %w", project.Stack, err)
		}
		layers = append(layers, frag)
	}

	if project.Overlay != "" {
		name := filepath.Join(overlaysDir, project.Overlay, stackFragment)
		frag, err := ParseFS(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("overlay layer %q: %w", project.Overlay, err)
		}
		layers = append(layers, frag)
	}

	layers = append(layers, project)
	merged := Merge(layers...)
	if err := merged.Validate(); err != nil {
		return nil, fmt.Errorf("merged playbook: %w", err)
	}

	return &Resolved{Playbook: merged, Source: fsys, Root: root}, nil
}
