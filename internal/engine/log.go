package engine

import (
	"os"
	"path/filepath"
)

// openDiagLog creates the per-verb diagnostic log at <root>/.loom/logs/<verb>.log
// (ADR-0010). Best-effort: returns (nil, "") if it cannot be created, so a
// logging hiccup never fails a verb. Callers close the file and pass it as the
// runtime's LogW.
func openDiagLog(root, verb string) (*os.File, string) {
	dir := filepath.Join(root, ".loom", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, ""
	}
	path := filepath.Join(dir, verb+".log")
	f, err := os.Create(path)
	if err != nil {
		return nil, ""
	}
	return f, path
}
