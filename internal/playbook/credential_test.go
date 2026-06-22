package playbook

import (
	"strings"
	"testing"
)

// TestCredentialMergeAnyTierLastWins proves FR-CRED-001 (merge): credential is
// the pointer analogue of trust — any-tier last-NON-NIL-wins. A later non-nil
// Credential REPLACES the earlier (whole replacement, no field merge); a layer
// that does not set it leaves the earlier value intact; and a later layer may
// introduce a credential the base never declared (NOT base-gated like settings).
func TestCredentialMergeAnyTierLastWins(t *testing.T) {
	base := &Playbook{
		Loom: 1, Tier: TierBase,
		Harness: map[string]HarnessAgent{
			"claude": {Credential: &Credential{Method: CredAPIKeyHelper, Helper: "op read op://base/key"}},
		},
	}
	overlay := &Playbook{
		Harness: map[string]HarnessAgent{
			"claude": {Credential: &Credential{Method: CredVolumeToken, Env: "CLAUDE_CODE_OAUTH_TOKEN"}},
			"gemini": {Credential: &Credential{Method: CredAPIKeyHelper, Helper: "op read op://proj/gemini"}},
		},
	}

	got := Merge(base, overlay)

	claude := got.Harness["claude"].Credential
	if claude == nil || claude.Method != CredVolumeToken || claude.Env != "CLAUDE_CODE_OAUTH_TOKEN" {
		t.Fatalf("a later non-nil credential must REPLACE the earlier (whole replacement): got %+v", claude)
	}
	if claude.Helper != "" {
		t.Errorf("replacement is whole, not field-merge: stale helper leaked through: %q", claude.Helper)
	}
	gem := got.Harness["gemini"].Credential
	if gem == nil || gem.Method != CredAPIKeyHelper {
		t.Errorf("a later layer must be able to introduce a credential the base never declared (NOT base-gated): got %+v", gem)
	}
}

// TestCredentialMergeNilLayerKeepsEarlier proves the nil-layer half of the
// merge rule: a layer whose HarnessAgent omits credential: must NOT clear an
// earlier declaration (the same "settings survives a layer that doesn't set it"
// property, for the pointer field).
func TestCredentialMergeNilLayerKeepsEarlier(t *testing.T) {
	base := &Playbook{
		Loom: 1, Tier: TierBase,
		Harness: map[string]HarnessAgent{
			"claude": {Credential: &Credential{Method: CredVolumeToken, Env: "CLAUDE_CODE_OAUTH_TOKEN"}},
		},
	}
	overlay := &Playbook{
		Harness: map[string]HarnessAgent{
			"claude": {Hooks: []string{"guard-bash"}}, // no credential:
		},
	}
	got := Merge(base, overlay).Harness["claude"].Credential
	if got == nil || got.Method != CredVolumeToken {
		t.Fatalf("a layer that omits credential: must leave the earlier declaration intact: got %+v", got)
	}
}

// TestValidateCredentialWiredMethods proves FR-CRED-001 (validate, happy path):
// the two slice-1-wired methods validate when their required field is present.
func TestValidateCredentialWiredMethods(t *testing.T) {
	for _, tc := range []struct {
		name string
		cred *Credential
	}{
		{"apiKeyHelper requires helper", &Credential{Method: CredAPIKeyHelper, Helper: "op read op://vault/key"}},
		{"volume-token requires env", &Credential{Method: CredVolumeToken, Env: "CLAUDE_CODE_OAUTH_TOKEN"}},
		{"oauth-file defaults path (no path:)", &Credential{Method: CredOAuthFile}},
		{"oauth-file explicit HOME-relative path", &Credential{Method: CredOAuthFile, Path: ".config/gcloud"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pb := &Playbook{Loom: 1, Tier: TierBase, Harness: map[string]HarnessAgent{"claude": {Credential: tc.cred}}}
			if err := pb.Validate(); err != nil {
				t.Fatalf("a wired credential with its required field must validate: %v", err)
			}
		})
	}
}

// TestValidateCredentialRequiredFields proves FR-CRED-001 (validate, per-method
// required field): a wired method missing its required field is a hard error.
func TestValidateCredentialRequiredFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		cred *Credential
		want string
	}{
		{"apiKeyHelper missing helper", &Credential{Method: CredAPIKeyHelper}, "requires a non-empty helper"},
		{"apiKeyHelper blank helper", &Credential{Method: CredAPIKeyHelper, Helper: "   "}, "requires a non-empty helper"},
		{"volume-token missing env", &Credential{Method: CredVolumeToken}, "requires a non-empty env"},
		{"volume-token blank env", &Credential{Method: CredVolumeToken, Env: "  "}, "requires a non-empty env"},
		{"missing method", &Credential{Env: "X"}, "missing required field: method"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pb := &Playbook{Loom: 1, Tier: TierBase, Harness: map[string]HarnessAgent{"claude": {Credential: tc.cred}}}
			err := pb.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got: %v", tc.want, err)
			}
		})
	}
}

// TestValidateCredentialFailClosedUnwired proves FR-CRED-001 (the guardrail
// core): every KNOWN-but-UNWIRED enum member FAILS CLOSED — a declared
// credential is never silently no-op'd. The error must name slice 2+.
func TestValidateCredentialFailClosedUnwired(t *testing.T) {
	for _, m := range []string{CredEnv, CredVolumeStoreHelp, CredInteractiveLogin} {
		t.Run(m, func(t *testing.T) {
			pb := &Playbook{Loom: 1, Tier: TierBase, Harness: map[string]HarnessAgent{
				"claude": {Credential: &Credential{Method: m, Helper: "x", Env: "X"}},
			}}
			err := pb.Validate()
			if err == nil || !strings.Contains(err.Error(), "not-yet-supported") || !strings.Contains(err.Error(), "slice 2+") {
				t.Fatalf("method %q must fail closed as not-yet-supported (slice 2+), got: %v", m, err)
			}
		})
	}
}

// TestValidateCredentialUnknownMethod proves an unknown method token is rejected
// outright (a typo cannot silently widen the credential surface).
func TestValidateCredentialUnknownMethod(t *testing.T) {
	pb := &Playbook{Loom: 1, Tier: TierBase, Harness: map[string]HarnessAgent{
		"claude": {Credential: &Credential{Method: "magic-keychain"}},
	}}
	err := pb.Validate()
	if err == nil || !strings.Contains(err.Error(), "unknown method") {
		t.Fatalf("an unknown credential method must be rejected, got: %v", err)
	}
}

// TestValidateCredentialEnvNameCharset proves the volume-token env: must be a
// single valid env var NAME — a typo like "FOO=BAR" would otherwise mis-target the
// exec-time `-e <env>=<token>` injection (`docker exec -e FOO=BAR=token`). A valid
// NAME passes; a name carrying '=' / whitespace / leading digit / empty is rejected.
func TestValidateCredentialEnvNameCharset(t *testing.T) {
	valid := []string{"CLAUDE_CODE_OAUTH_TOKEN", "FOO", "_X", "A1_B2", "_"}
	for _, env := range valid {
		t.Run("valid/"+env, func(t *testing.T) {
			pb := &Playbook{Loom: 1, Tier: TierBase, Harness: map[string]HarnessAgent{
				"claude": {Credential: &Credential{Method: CredVolumeToken, Env: env}},
			}}
			if err := pb.Validate(); err != nil {
				t.Fatalf("a valid env var NAME %q must validate, got: %v", env, err)
			}
		})
	}

	// Malformed names. Empty/blank are caught by the "non-empty env" branch; the
	// rest by the charset branch. Both must reject (a malformed name never passes).
	bad := []struct {
		env  string
		want string
	}{
		{"FOO=BAR", "not a valid env var NAME"},
		{"FOO BAR", "not a valid env var NAME"},
		{"FOO\tBAR", "not a valid env var NAME"},
		{"1FOO", "not a valid env var NAME"},
		{"FOO-BAR", "not a valid env var NAME"},
		{"", "requires a non-empty env"},
	}
	for _, tc := range bad {
		t.Run("bad/"+tc.env, func(t *testing.T) {
			pb := &Playbook{Loom: 1, Tier: TierBase, Harness: map[string]HarnessAgent{
				"claude": {Credential: &Credential{Method: CredVolumeToken, Env: tc.env}},
			}}
			err := pb.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("env %q must be rejected with %q, got: %v", tc.env, tc.want, err)
			}
		})
	}
}

// TestValidateOAuthFilePath proves FR-CRED-005 (validate, the one new oauth-file
// rule): a HOME-relative path: is accepted (so the mount stays inside $HOME); an
// absolute path or a '..' traversal is REJECTED (it would escape the per-project home
// boundary). An empty path: is valid — it defaults to DefaultOAuthFilePath.
func TestValidateOAuthFilePath(t *testing.T) {
	good := []string{"", ".gemini", ".config/gcloud", "a/b/c"}
	for _, p := range good {
		t.Run("good/"+p, func(t *testing.T) {
			pb := &Playbook{Loom: 1, Tier: TierBase, Harness: map[string]HarnessAgent{
				"gemini": {Credential: &Credential{Method: CredOAuthFile, Path: p}},
			}}
			if err := pb.Validate(); err != nil {
				t.Fatalf("a HOME-relative oauth-file path %q must validate, got: %v", p, err)
			}
		})
	}

	bad := []string{"/etc/passwd", "/root/.gemini", "../escape", "a/../../b", ".."}
	for _, p := range bad {
		t.Run("bad/"+p, func(t *testing.T) {
			pb := &Playbook{Loom: 1, Tier: TierBase, Harness: map[string]HarnessAgent{
				"gemini": {Credential: &Credential{Method: CredOAuthFile, Path: p}},
			}}
			err := pb.Validate()
			if err == nil || !strings.Contains(err.Error(), "HOME-relative") {
				t.Fatalf("an escaping oauth-file path %q must be rejected, got: %v", p, err)
			}
		})
	}
}
