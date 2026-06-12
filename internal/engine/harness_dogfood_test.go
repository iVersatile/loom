package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/iVersatile/loom/internal/playbook"
)

// TestDogfoodBasePlaybookHarnessMaterializes pins the T16 PR 2 wire-up against
// the REPO'S OWN config: the real loom.yml resolves, the base layer's harness:
// declaration survives the merge, every reference resolves in the real config
// source, and materializing yields the agent home verify-loom-dev asserts —
// executable ~/.claude/hooks/guard-bash and a settings.json declaring hooks +
// permissions. A dangling ref or a schema drift between config/playbook.yml
// and the engine fails HERE, not on the next human rebuild.
func TestDogfoodBasePlaybookHarnessMaterializes(t *testing.T) {
	resolved, err := playbook.Load(filepath.Join("..", "..", "loom.yml"))
	if err != nil {
		t.Fatalf("load repo loom.yml: %v", err)
	}
	h, ok := resolved.Playbook.Harness["claude"]
	if !ok {
		t.Fatal("merged playbook lacks harness.claude — base wire-up regressed")
	}
	if h.Settings == "" {
		t.Error("harness.claude.settings undeclared")
	}

	home := t.TempDir()
	if _, err := materializeHarness(resolved.Source, resolved.Playbook.Harness, home); err != nil {
		t.Fatalf("materialize real harness: %v", err)
	}

	hook := filepath.Join(home, ".claude", "hooks", "guard-bash")
	if fi, err := os.Stat(hook); err != nil || fi.Mode()&0o111 == 0 {
		t.Errorf("guard-bash should be materialized executable, got %v / %v", fi, err)
	}

	raw, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("settings.json not materialized: %v", err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("materialized settings.json is not valid JSON: %v", err)
	}
	for _, key := range []string{"hooks", "permissions", "statusLine"} {
		if _, ok := settings[key]; !ok {
			t.Errorf("settings.json missing %q — the verify-loom-dev claim would fail", key)
		}
	}
}

// TestDogfoodProjectPlaybookTrustMaterializes pins the 036 fix against the
// repo's own config (FR-BUILD-011, ADR-0018): loom.yml's project-tier
// harness.claude.trust declaration survives the merge, resolves in the real
// config source, and materializes ~/.claude.json carrying the opt-in flag for
// /workspace/loom — the file whose death-on-recreate cost a manual trust
// re-flip per restart (flips.log, T23).
func TestDogfoodProjectPlaybookTrustMaterializes(t *testing.T) {
	resolved, err := playbook.Load(filepath.Join("..", "..", "loom.yml"))
	if err != nil {
		t.Fatalf("load repo loom.yml: %v", err)
	}
	h, ok := resolved.Playbook.Harness["claude"]
	if !ok || h.Trust == "" {
		t.Fatal("merged playbook lacks harness.claude.trust — the 036 wire-up regressed")
	}

	home := t.TempDir()
	if _, err := materializeHarness(resolved.Source, resolved.Playbook.Harness, home); err != nil {
		t.Fatalf("materialize real harness: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		t.Fatalf("~/.claude.json not materialized: %v", err)
	}
	var trust struct {
		Projects map[string]struct {
			HasTrustDialogAccepted bool `json:"hasTrustDialogAccepted"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(raw, &trust); err != nil {
		t.Fatalf("materialized ~/.claude.json is not valid JSON: %v", err)
	}
	if p, ok := trust.Projects["/workspace/loom"]; !ok || !p.HasTrustDialogAccepted {
		t.Errorf("declared trust must opt in /workspace/loom (hasTrustDialogAccepted): got %+v", trust.Projects)
	}
}
