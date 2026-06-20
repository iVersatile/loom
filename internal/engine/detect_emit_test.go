package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/iVersatile/loom/internal/playbook"
)

// TestEmitDraftPlaybookRoundTrips proves the draft re-parses and validates as a
// base-tier playbook carrying exactly the present tools (as name@version intent),
// present agents, and credential names — and never the project-tier name/stack.
func TestEmitDraftPlaybookRoundTrips(t *testing.T) {
	res := DetectResult{
		Tools: []Tool{
			{Name: "git", Present: true, Version: "2.43"},
			{Name: "gopls", Present: false},
		},
		Agents: []Agent{
			{Name: "claude-code", Present: true},
			{Name: "codex", Present: false},
		},
		Credentials: []Credential{
			{Name: "ANTHROPIC_API_KEY", FoundIn: []string{"env"}},
		},
	}

	dir := t.TempDir()
	out, err := emitDraftPlaybook(res, filepath.Join(dir, "loom.yml"))
	if err != nil {
		t.Fatalf("emitDraftPlaybook: %v", err)
	}
	want := filepath.Join(dir, "loom.base.draft.yml")
	if out != want {
		t.Fatalf("returned path = %q, want %q", out, want)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("draft file not written: %v", err)
	}

	pb, err := playbook.ParseFile(out)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if err := pb.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if pb.Tier != playbook.TierBase {
		t.Errorf("Tier = %q, want %q", pb.Tier, playbook.TierBase)
	}
	if pb.Loom != playbook.SchemaVersion {
		t.Errorf("Loom = %d, want %d", pb.Loom, playbook.SchemaVersion)
	}
	if !reflect.DeepEqual(pb.Tools, []string{"git@2.43"}) {
		t.Errorf("Tools = %v, want [git@2.43]", pb.Tools)
	}
	if !reflect.DeepEqual(pb.Agents, []string{"claude-code"}) {
		t.Errorf("Agents = %v, want [claude-code]", pb.Agents)
	}
	if !reflect.DeepEqual(pb.Env, []string{"ANTHROPIC_API_KEY"}) {
		t.Errorf("Env = %v, want [ANTHROPIC_API_KEY]", pb.Env)
	}
	if pb.Name != "" || pb.Stack != "" {
		t.Errorf("base tier leaked project fields: name=%q stack=%q", pb.Name, pb.Stack)
	}
}

// TestEmitDraftPlaybookNamesOnlyNoValues pins that only the credential NAME is
// carried into the draft — never the found-in locations (or any value).
func TestEmitDraftPlaybookNamesOnlyNoValues(t *testing.T) {
	res := DetectResult{
		Credentials: []Credential{
			{Name: "ANTHROPIC_API_KEY", FoundIn: []string{"env", "~/.zshrc"}},
		},
	}

	dir := t.TempDir()
	out, err := emitDraftPlaybook(res, filepath.Join(dir, "loom.yml"))
	if err != nil {
		t.Fatalf("emitDraftPlaybook: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Contains(data, []byte("ANTHROPIC_API_KEY")) {
		t.Errorf("draft missing credential name; got:\n%s", data)
	}
	if bytes.Contains(data, []byte("found_in")) {
		t.Errorf("draft leaked found_in; got:\n%s", data)
	}
	if bytes.Contains(data, []byte("~/.zshrc")) {
		t.Errorf("draft leaked found-in location ~/.zshrc; got:\n%s", data)
	}
}

// TestEmitDraftPlaybookVersionBestEffort pins the version handling against
// REALISTIC prober output (not pre-cleaned tokens): "<tool> version <N.N.N>"
// yields name@version, while shapes with no digit-led token (go's, jq's) fall
// back to the bare name rather than a malformed intent. Presence is always
// captured; exact pinning is the resolver/lockfile's job.
func TestEmitDraftPlaybookVersionBestEffort(t *testing.T) {
	res := DetectResult{
		Tools: []Tool{
			{Name: "git", Present: true, Version: "git version 2.39.5"},             // recoverable -> git@2.39.5
			{Name: "go", Present: true, Version: "go version go1.26.4 linux/arm64"}, // no digit-led token -> bare
			{Name: "jq", Present: true, Version: "jq-1.6"},                          // single token, not digit-led -> bare
			{Name: "ripgrep", Present: true, Version: ""},                           // version unknown -> bare
		},
	}

	dir := t.TempDir()
	out, err := emitDraftPlaybook(res, filepath.Join(dir, "loom.yml"))
	if err != nil {
		t.Fatalf("emitDraftPlaybook: %v", err)
	}
	pb, err := playbook.ParseFile(out)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	want := []string{"git@2.39.5", "go", "jq", "ripgrep"}
	if !reflect.DeepEqual(pb.Tools, want) {
		t.Errorf("Tools = %v, want %v", pb.Tools, want)
	}
}

// TestDetectNoEmitUnchanged pins that without --emit-playbook detect writes no
// draft and reports no emitted path — the flag is the only behavior change.
func TestDetectNoEmitUnchanged(t *testing.T) {
	dir := t.TempDir()
	res, err := detectImpl(DetectOpts{PlaybookPath: filepath.Join(dir, "loom.yml")}, fakeProber{"git": "2.43"})
	if err != nil {
		t.Fatalf("detectImpl: %v", err)
	}
	if res.Emitted != "" {
		t.Errorf("Emitted = %q, want empty", res.Emitted)
	}
	if _, err := os.Stat(filepath.Join(dir, "loom.base.draft.yml")); !os.IsNotExist(err) {
		t.Errorf("draft should not exist without --emit-playbook (stat err = %v)", err)
	}
}

// TestDetectEmitPlaybookViaSeam is the FR-RUN-006 end-to-end through the verb
// seam (fast tier, no docker): a bare machine (no playbook) detects via defaults,
// emits a draft, and that draft reloads losslessly as a base playbook.
func TestDetectEmitPlaybookViaSeam(t *testing.T) {
	dir := t.TempDir()
	res, err := detectImpl(
		DetectOpts{PlaybookPath: filepath.Join(dir, "loom.yml"), EmitPlaybook: true},
		fakeProber{"git": "2.43", "claude": "1.2.3"},
	)
	if err != nil {
		t.Fatalf("detectImpl: %v", err)
	}
	want := filepath.Join(dir, "loom.base.draft.yml")
	if res.Emitted != want {
		t.Fatalf("Emitted = %q, want %q", res.Emitted, want)
	}
	if _, err := os.Stat(res.Emitted); err != nil {
		t.Fatalf("draft file not written: %v", err)
	}

	pb, err := playbook.ParseFile(res.Emitted)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if err := pb.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !reflect.DeepEqual(pb.Tools, []string{"git@2.43"}) {
		t.Errorf("Tools = %v, want [git@2.43]", pb.Tools)
	}
	if !reflect.DeepEqual(pb.Agents, []string{"claude-code"}) {
		t.Errorf("Agents = %v, want [claude-code]", pb.Agents)
	}
	if len(pb.Env) != 0 {
		t.Errorf("Env = %v, want empty (no playbook => no declared cred names)", pb.Env)
	}
}
