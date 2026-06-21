package engine

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSource is a deterministic credentialSource for tests.
type fakeSource map[string]string

func (f fakeSource) value(name string) (string, bool) {
	v, ok := f[name]
	return v, ok
}

// TestMigratePlanRedactsSecretValues proves a secret VALUE never appears in the
// redacted report (FR-RUN-007): the report marshals to JSON with no secret, the
// secret cred's Display is empty, and an aborted run writes nothing.
func TestMigratePlanRedactsSecretValues(t *testing.T) {
	const secret = "super-secret-value-123"
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	var captured MigrateReport
	confirm := func(rep MigrateReport) (bool, error) {
		captured = rep
		return false, nil // abort: nothing written
	}

	report, n, err := migrateImpl(
		[]Credential{{Name: "API_KEY"}},
		fakeSource{"API_KEY": secret},
		envPath,
		confirm,
	)
	if err != nil {
		t.Fatalf("migrateImpl: unexpected error %v", err)
	}
	if n != 0 {
		t.Errorf("aborted run wrote %d, want 0", n)
	}

	raw, err := json.Marshal(captured)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatalf("redacted report leaked the secret value:\n%s", raw)
	}
	// Belt and suspenders: the returned report must also be value-free.
	if raw2, _ := json.Marshal(report); strings.Contains(string(raw2), secret) {
		t.Fatalf("returned report leaked the secret value:\n%s", raw2)
	}

	if len(captured.Creds) != 1 {
		t.Fatalf("got %d creds, want 1", len(captured.Creds))
	}
	if captured.Creds[0].Display != "" {
		t.Errorf("secret cred Display = %q, want empty", captured.Creds[0].Display)
	}
	if captured.Creds[0].Status != "new" {
		t.Errorf("status = %q, want new", captured.Creds[0].Status)
	}
	if _, statErr := os.Stat(envPath); !os.IsNotExist(statErr) {
		t.Errorf(".env should not exist after an aborted run (stat err = %v)", statErr)
	}
}

// TestMigrateMergeIsIdempotent proves a no-change re-run writes nothing and does
// not even call confirm (FR-RUN-007: idempotent re-run).
func TestMigrateMergeIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	src := fakeSource{"API_KEY": "tok-abc"}

	report, n, err := migrateImpl([]Credential{{Name: "API_KEY"}}, src, envPath, func(MigrateReport) (bool, error) {
		return true, nil
	})
	if err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if n != 1 {
		t.Fatalf("first migrate wrote %d, want 1", n)
	}
	if report.Creds[0].Status != "new" {
		t.Errorf("first status = %q, want new", report.Creds[0].Status)
	}
	data, _ := os.ReadFile(envPath)
	if !strings.Contains(string(data), "API_KEY=tok-abc") {
		t.Fatalf(".env missing API_KEY=tok-abc:\n%s", data)
	}

	// Second identical run: must be an idempotent no-op that never calls confirm.
	report2, n2, err := migrateImpl([]Credential{{Name: "API_KEY"}}, src, envPath, func(MigrateReport) (bool, error) {
		t.Fatal("confirm must NOT be called on an idempotent no-op")
		return true, nil
	})
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if n2 != 0 {
		t.Errorf("second migrate wrote %d, want 0", n2)
	}
	if report2.Creds[0].Status != "unchanged" {
		t.Errorf("second status = %q, want unchanged", report2.Creds[0].Status)
	}
}

// TestMigrateRefusesUnignoredEnv proves the gitignore guard refuses to write a
// literal secret to a .env a repo does not ignore, then succeeds once ignored
// (FR-RUN-007).
func TestMigrateRefusesUnignoredEnv(t *testing.T) {
	if err := exec.Command("git", "--version").Run(); err != nil {
		t.Skip("git unavailable")
	}
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	envPath := filepath.Join(dir, ".env")
	src := fakeSource{"API_KEY": "tok-secret"}
	confirm := func(MigrateReport) (bool, error) { return true, nil }

	_, n, err := migrateImpl([]Credential{{Name: "API_KEY"}}, src, envPath, confirm)
	if err == nil {
		t.Fatal("expected a refusal error for an unignored .env, got nil")
	}
	if !strings.Contains(err.Error(), "gitignore") && !strings.Contains(err.Error(), "refusing") {
		t.Errorf("error %q should mention gitignore/refusing", err)
	}
	if n != 0 {
		t.Errorf("refused run wrote %d, want 0", n)
	}
	if _, statErr := os.Stat(envPath); !os.IsNotExist(statErr) {
		t.Errorf(".env should not exist after a refusal (stat err = %v)", statErr)
	}

	// Now ignore .env and the same call must succeed.
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".env\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	_, n2, err := migrateImpl([]Credential{{Name: "API_KEY"}}, src, envPath, confirm)
	if err != nil {
		t.Fatalf("migrate after gitignore: unexpected error %v", err)
	}
	if n2 != 1 {
		t.Errorf("migrate after gitignore wrote %d, want 1", n2)
	}
	data, _ := os.ReadFile(envPath)
	if !strings.Contains(string(data), "API_KEY=tok-secret") {
		t.Errorf(".env missing API_KEY=tok-secret:\n%s", data)
	}
}

// TestMigratePassesThroughReferencesUnredacted proves op:// references round-trip
// unmangled and unredacted (FR-RUN-008).
func TestMigratePassesThroughReferencesUnredacted(t *testing.T) {
	const ref = "op://vault/item/field"
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	var captured MigrateReport
	confirm := func(rep MigrateReport) (bool, error) {
		captured = rep
		return true, nil
	}
	_, n, err := migrateImpl([]Credential{{Name: "OP_KEY"}}, fakeSource{"OP_KEY": ref}, envPath, confirm)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if n != 1 {
		t.Fatalf("wrote %d, want 1", n)
	}
	if !captured.Creds[0].Reference {
		t.Error("Reference = false, want true for an op:// value")
	}
	if captured.Creds[0].Display != ref {
		t.Errorf("Display = %q, want %q (references are shown in the clear)", captured.Creds[0].Display, ref)
	}
	data, _ := os.ReadFile(envPath)
	if !strings.Contains(string(data), "OP_KEY="+ref) {
		t.Errorf(".env missing verbatim reference OP_KEY=%s:\n%s", ref, data)
	}
}

// TestMigrateEnvFileMode0600 proves a migrate tightens a PRE-EXISTING
// world-readable .env to 0600 (os.WriteFile does not chmod an existing file) and
// preserves unrelated lines (FR-RUN-007: a credentials file must not stay loose).
func TestMigrateEnvFileMode0600(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("EXISTING=keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, n, err := migrateImpl([]Credential{{Name: "API_KEY"}}, fakeSource{"API_KEY": "tok-new"}, envPath,
		func(MigrateReport) (bool, error) { return true, nil })
	if err != nil || n != 1 {
		t.Fatalf("migrate: n=%d err=%v", n, err)
	}

	info, err := os.Stat(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf(".env mode = %o, want 600 (a credentials file must be owner-only)", perm)
	}
	data, _ := os.ReadFile(envPath)
	if !strings.Contains(string(data), "EXISTING=keep") {
		t.Errorf("merge dropped the pre-existing unrelated line:\n%s", data)
	}
	if !strings.Contains(string(data), "API_KEY=tok-new") {
		t.Errorf(".env missing the migrated key:\n%s", data)
	}
}
