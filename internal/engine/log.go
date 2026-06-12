package engine

import (
	"fmt"
	"os"
	"path/filepath"
)

// openDiagLog opens the per-verb diagnostic log at <root>/.loom/logs/<verb>.log
// (ADR-0010) in APPEND mode, each run demarcated by a separator line — a re-run
// must not destroy the previous run's forensics (live-build e2e finding F-c:
// build.log was truncated per run, so run 1's trace was gone by run 2).
// Best-effort: returns (nil, "") if it cannot be opened, so a logging hiccup
// never fails a verb. Callers close the file and pass it as the runtime's LogW.
func openDiagLog(root, verb string) (*os.File, string) {
	dir := filepath.Join(root, ".loom", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, ""
	}
	path := filepath.Join(dir, verb+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, ""
	}
	_, _ = fmt.Fprintf(f, "----- loom %s run -----\n", verb)
	return f, path
}
