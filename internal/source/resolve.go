// Package source resolves a playbook's config_source (ADR-0006) to a read-only
// filesystem the engine reads rules/hooks/dotfiles/fragments from. Keeping the
// engine behind an fs.FS makes it ignorant of local-vs-git layout. This package
// takes primitive inputs (not the playbook type) so playbook can import it
// without an import cycle.
package source

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Resolve returns an fs.FS rooted at the config source.
//   - "local": os.DirFS(projectRoot/path), confined within projectRoot (a path
//     escaping the root is rejected — a guardrail, not a trust assumption).
//   - "git": not yet supported in Phase 1; returns a clear error so the field
//     still parses and validates.
func Resolve(projectRoot, sourceType, sourcePath string) (fs.FS, error) {
	switch sourceType {
	case "local":
		if sourcePath == "" {
			return nil, fmt.Errorf("config_source local requires a path")
		}
		full := filepath.Clean(filepath.Join(projectRoot, sourcePath))
		rel, err := filepath.Rel(projectRoot, full)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("config_source path %q escapes project root", sourcePath)
		}
		info, err := os.Stat(full)
		if err != nil {
			return nil, fmt.Errorf("config_source path %q: %w", sourcePath, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("config_source path %q is not a directory", sourcePath)
		}
		return os.DirFS(full), nil
	case "git":
		return nil, fmt.Errorf("config_source type %q not yet supported (Phase 1)", sourceType)
	case "":
		return nil, fmt.Errorf("config_source requires a type")
	default:
		return nil, fmt.Errorf("unknown config_source type %q", sourceType)
	}
}
