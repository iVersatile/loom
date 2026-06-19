package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin scripts/loom-cron — the host-cron MANAGER for loom's crontab
// block (it installs the cron lines that call the cold-floor + coordinate
// ACTUATORS; it actuates nothing itself). The properties under test are the
// marker-block discipline: set is IDEMPOTENT (twice ⇒ one block), every mutation
// PRESERVES the operator's other cron lines verbatim, verify is a pure read that
// fails when the block is absent, and the default install is a DRY-RUN (no inject
// command baked into the rendered lines). Hermetic via a FAKE crontab shim pointed
// at by LOOM_CRONTAB and backed by a temp file — never the runner's real crontab.

func loomCronScript(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "scripts", "loom-cron"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// fakeCrontab writes a tiny shim that stands in for the crontab binary, backed by a
// temp file: `-l` prints the file (and, like real crontab, exits non-zero with a
// notice when it does not exist yet), `-` writes stdin into it. It returns the shim
// path and the backing-file path; seed pre-existing content by writing the file.
func fakeCrontab(t *testing.T) (shim, backing string) {
	t.Helper()
	dir := t.TempDir()
	backing = filepath.Join(dir, "crontab.txt")
	shim = filepath.Join(dir, "fake-crontab")
	body := "#!/bin/sh\n" +
		"f=\"" + backing + "\"\n" +
		"case \"$1\" in\n" +
		"-l) if [ -f \"$f\" ]; then cat \"$f\"; else echo 'no crontab for test' >&2; exit 1; fi ;;\n" +
		"-)  cat > \"$f\" ;;\n" +
		"*)  exit 2 ;;\n" +
		"esac\n"
	if err := os.WriteFile(shim, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return shim, backing
}

// runLoomCron runs loom-cron <sub> against the fake crontab. extraEnv overrides/adds
// to the base (e.g. the inject env vars). It returns combined output and exit error.
func runLoomCron(t *testing.T, shim, sub string, extraEnv ...string) (out string, err error) {
	t.Helper()
	c := exec.Command("sh", loomCronScript(t), sub)
	c.Env = append(append(hermeticEnv(), "LOOM_CRONTAB="+shim), extraEnv...)
	b, e := c.CombinedOutput()
	return string(b), e
}

// readCrontab returns the current backing-file contents ("" if not yet written).
func readCrontab(t *testing.T, backing string) string {
	t.Helper()
	b, err := os.ReadFile(backing)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatal(err)
	}
	return string(b)
}

const (
	beginMarker = "# >>> loom-cron"
	endMarker   = "# <<< loom-cron <<<"
)

// set adds the marker block with both job lines.
func TestLoomCronSetAddsBlock(t *testing.T) {
	shim, backing := fakeCrontab(t)
	if out, err := runLoomCron(t, shim, "set"); err != nil {
		t.Fatalf("set: %v\n%s", err, out)
	}
	got := readCrontab(t, backing)
	for _, want := range []string{beginMarker, endMarker, "scripts/cold-floor-cron", "scripts/coordinate-cron", "0 * * * *", "0 10 * * *"} {
		if !strings.Contains(got, want) {
			t.Errorf("set crontab missing %q\n--- got ---\n%s", want, got)
		}
	}
}

// set is IDEMPOTENT — run twice ⇒ exactly ONE marker pair.
func TestLoomCronSetIdempotent(t *testing.T) {
	shim, backing := fakeCrontab(t)
	for i := 0; i < 2; i++ {
		if out, err := runLoomCron(t, shim, "set"); err != nil {
			t.Fatalf("set #%d: %v\n%s", i, err, out)
		}
	}
	got := readCrontab(t, backing)
	if n := strings.Count(got, beginMarker); n != 1 {
		t.Errorf("expected exactly one begin marker after two sets, got %d\n%s", n, got)
	}
	if n := strings.Count(got, endMarker); n != 1 {
		t.Errorf("expected exactly one end marker after two sets, got %d\n%s", n, got)
	}
}

// set PRESERVES a pre-existing non-loom line.
func TestLoomCronSetPreservesOtherLines(t *testing.T) {
	shim, backing := fakeCrontab(t)
	const other = "30 3 * * *  /usr/bin/operator-backup.sh"
	if err := os.WriteFile(backing, []byte(other+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runLoomCron(t, shim, "set"); err != nil {
		t.Fatalf("set: %v\n%s", err, out)
	}
	got := readCrontab(t, backing)
	if !strings.Contains(got, other) {
		t.Errorf("set must preserve the operator's pre-existing line %q\n--- got ---\n%s", other, got)
	}
	if !strings.Contains(got, beginMarker) {
		t.Errorf("set must still add the loom block\n--- got ---\n%s", got)
	}
}

// remove strips the block and leaves the non-loom line.
func TestLoomCronRemoveStripsBlockKeepsOther(t *testing.T) {
	shim, backing := fakeCrontab(t)
	const other = "30 3 * * *  /usr/bin/operator-backup.sh"
	if err := os.WriteFile(backing, []byte(other+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runLoomCron(t, shim, "set"); err != nil {
		t.Fatalf("set: %v\n%s", err, out)
	}
	if out, err := runLoomCron(t, shim, "remove"); err != nil {
		t.Fatalf("remove: %v\n%s", err, out)
	}
	got := readCrontab(t, backing)
	if strings.Contains(got, beginMarker) || strings.Contains(got, endMarker) {
		t.Errorf("remove must strip the loom block\n--- got ---\n%s", got)
	}
	if !strings.Contains(got, other) {
		t.Errorf("remove must keep the operator's other line %q\n--- got ---\n%s", other, got)
	}
}

// verify exits non-zero when the block is absent, zero when present.
func TestLoomCronVerify(t *testing.T) {
	shim, _ := fakeCrontab(t)
	if _, err := runLoomCron(t, shim, "verify"); err == nil {
		t.Error("verify must exit non-zero when the loom block is absent")
	}
	if out, err := runLoomCron(t, shim, "set"); err != nil {
		t.Fatalf("set: %v\n%s", err, out)
	}
	if out, err := runLoomCron(t, shim, "verify"); err != nil {
		t.Errorf("verify must exit zero when the block is present: %v\n%s", err, out)
	}
}

// dry-run default: with no inject env set, the rendered lines bake in NO inject
// command (no LOOM_COLD_NUDGE_CMD=… / LOOM_COORDINATE_INJECT_CMD=… assignment).
func TestLoomCronSetDryRunDefault(t *testing.T) {
	shim, backing := fakeCrontab(t)
	// Force the inject env empty so an ambient value can't leak into the render.
	if out, err := runLoomCron(t, shim, "set", "LOOM_COLD_NUDGE_CMD=", "LOOM_COORDINATE_INJECT_CMD="); err != nil {
		t.Fatalf("set: %v\n%s", err, out)
	}
	got := readCrontab(t, backing)
	for _, baked := range []string{"LOOM_COLD_NUDGE_CMD=", "LOOM_COORDINATE_INJECT_CMD="} {
		if strings.Contains(got, baked) {
			t.Errorf("dry-run default must not bake an inject command (%q) into the crontab\n--- got ---\n%s", baked, got)
		}
	}
	// The actuators must still be wired (just inject-less / dry-run).
	if !strings.Contains(got, "scripts/cold-floor-cron") || !strings.Contains(got, "scripts/coordinate-cron") {
		t.Errorf("dry-run block must still install both actuator lines\n--- got ---\n%s", got)
	}
}
