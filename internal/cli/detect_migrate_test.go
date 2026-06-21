package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDetectMigrateConfirmAndYes drives `detect --migrate --yes` end to end and
// asserts the secret value lands ONLY in .env — never on stdout (the frozen
// detect doc) nor on stderr (the redacted diff) — and that a re-run is idempotent.
func TestDetectMigrateConfirmAndYes(t *testing.T) {
	const secret = "secret-xyz-987" // gitleaks:allow — fixture, not a real key
	proj := tempCopy(t, "../playbook/testdata/proj")
	t.Setenv("ANTHROPIC_API_KEY", secret)

	pb := filepath.Join(proj, "loom.yml")
	stdout, stderr, err := runCmd(t, "detect", "--migrate", "--yes", "-f", pb)
	if err != nil {
		t.Fatalf("detect --migrate --yes: unexpected error %v", err)
	}

	envPath := filepath.Join(proj, ".env")
	data, readErr := os.ReadFile(envPath)
	if readErr != nil {
		t.Fatalf("read .env: %v", readErr)
	}
	if !strings.Contains(string(data), "ANTHROPIC_API_KEY="+secret) {
		t.Errorf(".env missing ANTHROPIC_API_KEY=%s:\n%s", secret, data)
	}

	if strings.Contains(stdout, secret) {
		t.Errorf("SECURITY: stdout leaked the secret value:\n%s", stdout)
	}
	if strings.Contains(stderr, secret) {
		t.Errorf("SECURITY: stderr leaked the secret value:\n%s", stderr)
	}
	if !strings.Contains(stderr, "ANTHROPIC_API_KEY") {
		t.Errorf("stderr should name the migrated credential:\n%s", stderr)
	}
	if !strings.Contains(stderr, "migrate") {
		t.Errorf("stderr should mention migrate:\n%s", stderr)
	}

	// Idempotent re-run: same command, nothing new written.
	_, stderr2, err := runCmd(t, "detect", "--migrate", "--yes", "-f", pb)
	if err != nil {
		t.Fatalf("second detect --migrate: unexpected error %v", err)
	}
	if strings.Contains(stderr2, secret) {
		t.Errorf("SECURITY: second-run stderr leaked the secret:\n%s", stderr2)
	}
	if !strings.Contains(stderr2, "unchanged") && !strings.Contains(stderr2, "nothing to migrate") {
		t.Errorf("second run should report unchanged / nothing to migrate:\n%s", stderr2)
	}
}
