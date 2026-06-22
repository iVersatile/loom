package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iVersatile/loom/internal/playbook"
)

// fakeToken is the obvious non-secret stand-in for the M1 volume-token in every
// test — never a realistic key shape (gitleaks must stay clean).
const fakeToken = "FAKE-TOKEN"

// --- M2: apiKeyHelper → targeted settings.json key-set (ADR-0027) ---

// TestSetSettingsKeyTargetedKeySet proves FR-CRED-002: the M2 adapter sets the
// ONE apiKeyHelper key on the materialized settings.json, PRESERVING every other
// key (a targeted key-set, not a whole-file clobber, not a deep merge).
func TestSetSettingsKeyTargetedKeySet(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")
	// A pre-materialized settings.json carrying unrelated config that must survive.
	orig := `{"statusLine":{"type":"command","command":"~/.claude/statusline.sh"},"permissions":{"deny":["Bash(rm -rf /)"]}}`
	if err := os.WriteFile(target, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	helper := "op read op://vault/anthropic/key"
	changed, err := setSettingsKey(target, "apiKeyHelper", helper)
	if err != nil {
		t.Fatalf("setSettingsKey: %v", err)
	}
	if !changed {
		t.Error("adding a new key must report changed")
	}

	var doc map[string]json.RawMessage
	data, _ := os.ReadFile(target)
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("result not valid JSON: %v", err)
	}
	// The carve-out key is set to the operator COMMAND.
	var got string
	if err := json.Unmarshal(doc["apiKeyHelper"], &got); err != nil || got != helper {
		t.Errorf("apiKeyHelper = %q (err %v), want %q", got, err, helper)
	}
	// Pre-existing keys are PRESERVED (targeted, not clobber).
	if _, ok := doc["statusLine"]; !ok {
		t.Error("targeted key-set must preserve statusLine")
	}
	if _, ok := doc["permissions"]; !ok {
		t.Error("targeted key-set must preserve the permissions.deny floor")
	}
}

// TestSetSettingsKeyIdempotent proves the M2 key-set is idempotent — a re-run
// with the same helper leaves the file unchanged (writeIfChanged semantics).
func TestSetSettingsKeyIdempotent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(target, []byte(`{"x":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := setSettingsKey(target, "apiKeyHelper", "cmd"); err != nil {
		t.Fatal(err)
	}
	changed, err := setSettingsKey(target, "apiKeyHelper", "cmd")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("re-setting the same helper must be a no-op (idempotent)")
	}
}

// TestApplyAPIKeyHelperMaterialize proves FR-CRED-002 end-to-end at the
// materialize layer: an apiKeyHelper credential sets the key on the agent's
// staged settings.json; an agent without a credential is untouched (a non-M2
// build is byte-identical).
func TestApplyAPIKeyHelperMaterialize(t *testing.T) {
	home := t.TempDir()
	// Stage the agent's settings.json as materializeHarness would.
	claudeHome := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeHome, "settings.json"), []byte(`{"statusLine":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	harness := map[string]playbook.HarnessAgent{
		"claude": {
			Settings:   "claude/settings.json",
			Credential: &playbook.Credential{Method: playbook.CredAPIKeyHelper, Helper: "op read op://v/k"},
		},
	}
	mats, err := applyAPIKeyHelper(harness, home)
	if err != nil {
		t.Fatalf("applyAPIKeyHelper: %v", err)
	}
	if len(mats) != 1 || !mats[0].Changed {
		t.Fatalf("expected one changed materialize result, got %+v", mats)
	}
	data, _ := os.ReadFile(filepath.Join(claudeHome, "settings.json"))
	if !strings.Contains(string(data), "apiKeyHelper") {
		t.Errorf("apiKeyHelper not set on staged settings: %s", data)
	}
}

// TestApplyAPIKeyHelperNoCredentialNoop proves a non-M2 harness is a clean no-op.
func TestApplyAPIKeyHelperNoCredentialNoop(t *testing.T) {
	mats, err := applyAPIKeyHelper(map[string]playbook.HarnessAgent{
		"claude": {Settings: "claude/settings.json"},
	}, t.TempDir())
	if err != nil {
		t.Fatalf("no-credential harness must not error: %v", err)
	}
	if len(mats) != 0 {
		t.Errorf("no-credential harness must produce no materialize results, got %+v", mats)
	}
}

// --- M1: volume-token → exec-time -e injection (ADR-0027 + ADR-0014 no-leak) ---

// TestCredentialExecEnvInjectsToken proves FR-CRED-003 (the injection): a
// volume-token credential resolves the token through the mockable read-seam and
// yields the exec-time `ENV=TOKEN` pair. The seam means NO real secret is needed.
func TestCredentialExecEnvInjectsToken(t *testing.T) {
	rt := fakeRuntime{credToken: fakeToken}
	harness := map[string]playbook.HarnessAgent{
		"claude": {Credential: &playbook.Credential{Method: playbook.CredVolumeToken, Env: "CLAUDE_CODE_OAUTH_TOKEN"}},
	}
	env, err := credentialExecEnv(rt, "loom-dev", harness)
	if err != nil {
		t.Fatalf("credentialExecEnv: %v", err)
	}
	if len(env) != 1 || env[0] != "CLAUDE_CODE_OAUTH_TOKEN="+fakeToken {
		t.Fatalf("env = %v, want [CLAUDE_CODE_OAUTH_TOKEN=%s]", env, fakeToken)
	}
}

// TestCredentialExecEnvFailClosedAbsent proves FR-CRED-003 (fail-closed): an
// absent volume/token (read error) makes credentialExecEnv error — the verb
// refuses to open an unauthenticated seat (no silent degrade).
func TestCredentialExecEnvFailClosedAbsent(t *testing.T) {
	rt := fakeRuntime{credTokenErr: os.ErrNotExist}
	harness := map[string]playbook.HarnessAgent{
		"claude": {Credential: &playbook.Credential{Method: playbook.CredVolumeToken, Env: "CLAUDE_CODE_OAUTH_TOKEN"}},
	}
	_, err := credentialExecEnv(rt, "loom-dev", harness)
	if err == nil || !strings.Contains(err.Error(), "fail-closed") {
		t.Fatalf("absent token must fail closed, got: %v", err)
	}
}

// TestCredentialExecEnvFailClosedEmpty proves a provisioned-but-EMPTY token is
// also a hard error (empty must never pass as authenticated).
func TestCredentialExecEnvFailClosedEmpty(t *testing.T) {
	rt := fakeRuntime{credToken: "   "}
	harness := map[string]playbook.HarnessAgent{
		"claude": {Credential: &playbook.Credential{Method: playbook.CredVolumeToken, Env: "CLAUDE_CODE_OAUTH_TOKEN"}},
	}
	_, err := credentialExecEnv(rt, "loom-dev", harness)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty token must fail closed, got: %v", err)
	}
}

// TestCredentialExecEnvNoCredentialNoop proves a non-M1 harness yields no env
// (and never reads a token) — a non-credential exec is byte-identical.
func TestCredentialExecEnvNoCredentialNoop(t *testing.T) {
	// credTokenErr would fire IF the token were read; a clean no-op must not read.
	rt := fakeRuntime{credTokenErr: os.ErrNotExist}
	env, err := credentialExecEnv(rt, "loom-dev", map[string]playbook.HarnessAgent{"claude": {}})
	if err != nil || env != nil {
		t.Fatalf("no-credential harness must yield (nil,nil), got env=%v err=%v", env, err)
	}
}

// --- M1 leak-avoidance: exec-`-e`, NEVER run-`-e` ---

// TestCredentialVolumeMountIsVolumeNotRunEnv proves FR-CRED-003 (the ADR-0014
// no-leak invariant, create-time half): the credential reaches the container as a
// `-v` VOLUME mount on `docker run`, NEVER as `docker run -e` (which would persist
// the value into Config.Env / `docker inspect`). No token value appears in the run
// args at all (the value is injected only at exec-time).
func TestCredentialVolumeMountIsVolumeNotRunEnv(t *testing.T) {
	spec := ContainerSpec{
		Name: "demo-dev", Project: "demo", BaseImage: "img",
		Agents:                 []AgentInstall{{Name: "claude-code"}},
		CredentialVolumeAgents: []string{"claude"},
	}
	args := createRunArgs(spec, "", false)
	joined := strings.Join(args, " ")

	// The credential volume is mounted (-v <vol>:<path>:ro).
	wantMount := "-v demo-dev-claude-cred:" + credMountPath + ":ro"
	if !strings.Contains(joined, wantMount) {
		t.Errorf("createRunArgs must mount the per-project credential volume: want %q in %v", wantMount, args)
	}
	// CRITICAL: the token env name must NOT appear as a `docker run -e` — no
	// credential value on run at all (the ADR-0014 leak this design avoids).
	for i, a := range args {
		if a == "-e" && i+1 < len(args) {
			if strings.Contains(args[i+1], "TOKEN") || strings.Contains(args[i+1], fakeToken) {
				t.Errorf("credential must NEVER be on docker run -e (ADR-0014 Config.Env leak): found -e %q", args[i+1])
			}
		}
	}
	// And the raw token value is nowhere in the create args.
	if strings.Contains(joined, fakeToken) {
		t.Errorf("no token value may appear in docker run args: %v", args)
	}
}

// TestExecInjectsCredentialOntoAgentSeat proves FR-CRED-003 through the WHOLE exec
// verb: a volume-token playbook makes execImpl read the token (mock seam) and pass
// it as the exec credEnv — the AGENT seat's `docker exec -e`, never run-`-e`.
func TestExecInjectsCredentialOntoAgentSeat(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	data, err := os.ReadFile(pbPath)
	if err != nil {
		t.Fatal(err)
	}
	// Project-tier credential: declared (any-tier merge) and does not trip the
	// base-only settings gate (no settings: declared here).
	cred := "\nharness:\n  claude:\n    credential:\n      method: volume-token\n      env: CLAUDE_CODE_OAUTH_TOKEN\n"
	if err := os.WriteFile(pbPath, append(data, []byte(cred)...), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := &execCall{}
	rt := fakeRuntime{exists: true, credToken: fakeToken, execRecord: rec}
	if _, err := execImpl(ExecOpts{PlaybookPath: pbPath, Command: []string{"claude", "-p"}}, rt, fixedClock); err != nil {
		t.Fatalf("exec: %v", err)
	}
	if len(rec.credEnv) != 1 || rec.credEnv[0] != "CLAUDE_CODE_OAUTH_TOKEN="+fakeToken {
		t.Fatalf("exec credEnv = %v, want the token injected on the agent seat", rec.credEnv)
	}
}

// TestExecFailClosedOnAbsentCredential proves the exec verb refuses to run when a
// declared credential's token is absent — no unauthenticated seat.
func TestExecFailClosedOnAbsentCredential(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	data, _ := os.ReadFile(pbPath)
	cred := "\nharness:\n  claude:\n    credential:\n      method: volume-token\n      env: CLAUDE_CODE_OAUTH_TOKEN\n"
	if err := os.WriteFile(pbPath, append(data, []byte(cred)...), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := fakeRuntime{exists: true, credTokenErr: os.ErrNotExist}
	_, err := execImpl(ExecOpts{PlaybookPath: pbPath, Command: []string{"claude", "-p"}}, rt, fixedClock)
	if err == nil || !strings.Contains(err.Error(), "fail-closed") {
		t.Fatalf("exec with an absent credential must refuse loudly, got: %v", err)
	}
}

// --- the per-project volume name (ADR-0027 invariant #1) ---

// TestCredentialVolumeName pins the per-project, per-agent credential volume name
// pattern: `<container>-<agent>-cred` (the `<container>-<service>` family).
func TestCredentialVolumeName(t *testing.T) {
	if got := credentialVolume("loom-dev", "claude"); got != "loom-dev-claude-cred" {
		t.Errorf("credentialVolume = %q, want loom-dev-claude-cred", got)
	}
}

// --- doctor slug-uniqueness (ADR-0027 invariant #1, FR-CRED-004) ---

// TestCredentialSlugCheckOK proves FR-CRED-004 (happy path): a well-formed
// project slug + M1 agent yields a passing, unique credential-volume key claim.
func TestCredentialSlugCheckOK(t *testing.T) {
	pb := playbook.Playbook{
		Loom: 1, Tier: playbook.TierProject, Name: "demo",
		Harness: map[string]playbook.HarnessAgent{
			"claude": {Credential: &playbook.Credential{Method: playbook.CredVolumeToken, Env: "CLAUDE_CODE_OAUTH_TOKEN"}},
		},
	}
	c, ok := credentialSlugChecks(pb)
	if !ok || !c.OK {
		t.Fatalf("well-formed slug must pass: ok=%v check=%+v", ok, c)
	}
	if !strings.Contains(c.Detail, "demo-dev-claude-cred") {
		t.Errorf("detail should name the unique key: %q", c.Detail)
	}
}

// TestCredentialSlugCheckBadSlug proves FR-CRED-004 (fail-closed): a project slug
// carrying a volume-name separator fails the check (it would poison the namespace).
func TestCredentialSlugCheckBadSlug(t *testing.T) {
	pb := playbook.Playbook{
		Loom: 1, Tier: playbook.TierProject, Name: "bad/slug",
		Harness: map[string]playbook.HarnessAgent{
			"claude": {Credential: &playbook.Credential{Method: playbook.CredVolumeToken, Env: "X"}},
		},
	}
	c, ok := credentialSlugChecks(pb)
	if !ok || c.OK {
		t.Fatalf("a separator-bearing slug must fail closed: ok=%v check=%+v", ok, c)
	}
}

// TestCredentialSlugCheckNotApplicable proves the check is graded ONLY when an M1
// credential is declared (no volume-token ⇒ nothing to grade).
func TestCredentialSlugCheckNotApplicable(t *testing.T) {
	pb := playbook.Playbook{
		Loom: 1, Tier: playbook.TierProject, Name: "demo",
		Harness: map[string]playbook.HarnessAgent{
			"claude": {Credential: &playbook.Credential{Method: playbook.CredAPIKeyHelper, Helper: "cmd"}},
		},
	}
	if _, ok := credentialSlugChecks(pb); ok {
		t.Error("apiKeyHelper (no per-project volume) must not trigger the slug check")
	}
}
