// Package audit appends an action-log entry for every mutating verb so an
// autonomous run is reviewable after the fact (RULES §5, ADR-0005). Per-project,
// append-only JSONL at <repoRoot>/.loom/actions.log (frozen shape,
// docs/SPEC-verbs.md "Action log"). One JSON object per line is crash-safe and
// trivially greppable by an agent.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	dir  = ".loom"
	file = "actions.log"
)

// Entry is one logged action. Field shape is frozen in SPEC-verbs.md; the entry's
// id is its 1-based ordinal in the log (returned by Append), not a stored field.
type Entry struct {
	TS     string `json:"ts"`
	Verb   string `json:"verb"`
	Action string `json:"action"`
	Target string `json:"target"`
	Before any    `json:"before"`
	After  any    `json:"after"`
	Result string `json:"result"`
	Actor  string `json:"actor"`
}

// Log is an open append handle to a project's action log.
type Log struct {
	path string
	seq  int
}

// Open prepares <repoRoot>/.loom/actions.log, creating the directory. The id
// sequence continues from any existing entries so ids stay unique across runs.
func Open(repoRoot string) (*Log, error) {
	d := filepath.Join(repoRoot, dir)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return nil, err
	}
	p := filepath.Join(d, file)
	seq, err := countLines(p)
	if err != nil {
		return nil, err
	}
	return &Log{path: p, seq: seq}, nil
}

// Append writes one entry as a JSON line and returns its id ("aNNNNNN", the
// 1-based ordinal of the entry).
func (l *Log) Append(e Entry) (string, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	l.seq++
	return fmt.Sprintf("a%06d", l.seq), nil
}

func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	return n, nil
}
