package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestImportCLIJSONShape pins FR-IMPORT-002's CLI half: `loom import <path>
// --json` emits the documented top-level shape and writes the draft.
func TestImportCLIJSONShape(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "devcontainer.json")
	if err := os.WriteFile(src, []byte(`{"name":"demo","forwardPorts":[3000],"containerEnv":{"NODE_ENV":"x"}}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	out, _, err := runCmd(t, "import", src, "--json")
	if err != nil {
		t.Fatalf("import --json: %v", err)
	}

	var got map[string]json.RawMessage
	if e := json.Unmarshal([]byte(out), &got); e != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", e, out)
	}
	for _, k := range []string{"source", "mapped", "reported", "deferred", "draft"} {
		if _, ok := got[k]; !ok {
			t.Errorf("import --json missing key %q", k)
		}
	}

	var mapped map[string]json.RawMessage
	if e := json.Unmarshal(got["mapped"], &mapped); e != nil {
		t.Fatalf("mapped is not an object: %v", e)
	}
	for _, k := range []string{"ports", "env"} {
		if _, ok := mapped[k]; !ok {
			t.Errorf("import --json mapped missing key %q", k)
		}
	}

	if _, err := os.Stat(filepath.Join(dir, "loom.imported.yml")); err != nil {
		t.Errorf("draft loom.imported.yml not written: %v", err)
	}
}
