package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendWritesJSONLAndIDs(t *testing.T) {
	root := t.TempDir()
	log, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	id1, err := log.Append(Entry{TS: "t1", Verb: "build", Action: "lock.write", Target: "loom.lock", Result: "written", Actor: "cli"})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	id2, _ := log.Append(Entry{TS: "t1", Verb: "build", Action: "container.create", Target: "loom-x-dev", Result: "created", Actor: "cli"})
	if id1 != "a000001" || id2 != "a000002" {
		t.Errorf("ids = %q,%q want a000001,a000002", id1, id2)
	}

	data, err := os.ReadFile(filepath.Join(root, ".loom", "actions.log"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 JSONL lines, got %d", len(lines))
	}
	var e Entry
	if err := json.Unmarshal([]byte(lines[0]), &e); err != nil {
		t.Fatalf("line 1 not JSON: %v", err)
	}
	if e.Action != "lock.write" || e.Actor != "cli" {
		t.Errorf("entry round-trip wrong: %+v", e)
	}
}

func TestOpenContinuesSequence(t *testing.T) {
	root := t.TempDir()
	l1, _ := Open(root)
	_, _ = l1.Append(Entry{TS: "t", Action: "a"})
	// Re-open: a fresh handle must keep numbering after existing entries.
	l2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := l2.Append(Entry{TS: "t", Action: "b"})
	if id != "a000002" {
		t.Errorf("reopened log id = %q, want a000002", id)
	}
}
