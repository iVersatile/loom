package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iVersatile/loom/internal/playbook"
)

// TestDetectEmitPlaybookJSON pins the AI-first surface of detect --emit-playbook:
// the frozen --json document on stdout is UNCHANGED (no 'emitted' key leaks in),
// the written draft path is surfaced on stderr (so a --json consumer can find the
// file the run produced), and the draft actually lands and re-parses as a valid
// base playbook. Hermetic: an empty PATH makes the prober find nothing, so the
// assertion never depends on the host's installed tools (LL-008 trick).
func TestDetectEmitPlaybookJSON(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	pbPath := filepath.Join(dir, "loom.yml")

	stdout, stderr, err := runCmd(t, "detect", "--json", "--emit-playbook", "-f", pbPath)
	if err != nil {
		t.Fatalf("detect --emit-playbook: %v", err)
	}

	// stdout: valid JSON, frozen top-level shape, and NO new key.
	var got map[string]any
	if e := json.Unmarshal([]byte(stdout), &got); e != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", e, stdout)
	}
	for _, k := range []string{"tools", "agents", "credentials", "projects", "drift"} {
		if _, ok := got[k]; !ok {
			t.Errorf("detect --json missing key %q", k)
		}
	}
	if _, leaked := got["emitted"]; leaked {
		t.Errorf("frozen --json shape gained an 'emitted' key: %v", got)
	}

	// stderr names the written draft path (the discovery channel for --json).
	draft := filepath.Join(dir, "loom.base.draft.yml")
	if !strings.Contains(stderr, draft) {
		t.Errorf("stderr should name the draft path %q, got: %q", draft, stderr)
	}

	// the draft actually landed and is a valid base playbook.
	pb, e := playbook.ParseFile(draft)
	if e != nil {
		t.Fatalf("draft not parseable: %v", e)
	}
	if e := pb.Validate(); e != nil {
		t.Fatalf("draft invalid: %v", e)
	}
	if pb.Tier != playbook.TierBase {
		t.Errorf("draft tier = %q, want %q", pb.Tier, playbook.TierBase)
	}
}
