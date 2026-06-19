package guard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These tests gate the offline reaper scripts/reap-workers (ADR-0025 Slice D1) —
// the dead-worker cleanup decision. They mirror the spawn-guard / promote-next
// discipline (hermetic env LL-006/LL-010, fixed-vocab verdicts, injection proof)
// but assert the INVERTED failure mode: the reaper fails SAFE, so every ambiguity
// must resolve to SKIP and NEVER to a (destructive) REAP.

// Deterministic clock. The heartbeat TTL and max-age TTL are pinned so a single
// NOW lets the fixtures place a worker precisely on either side of each bound.
const (
	reapNow     = 100000         // LOOM_NOW
	reapHBTTL   = 300            // LOOM_HEARTBEAT_TTL — heartbeat fresher than this ⇒ LIVE
	reapMaxAge  = 3600           // LOOM_REAP_MAX_AGE — ledger start older than this ⇒ DEAD-eligible
	staleHB     = reapNow - 1000 // heartbeat mtime: age 1000 > 300 ⇒ dead heartbeat
	freshHB     = reapNow - 100  // heartbeat mtime: age 100 <= 300 ⇒ LIVE
	oldStart    = reapNow - 5000 // ledger start: age 5000 > 3600 ⇒ past max-age
	recentStart = reapNow - 1000 // ledger start: age 1000 < 3600 ⇒ NOT past max-age
)

func reapEnv() []string {
	return []string{
		fmt.Sprintf("LOOM_NOW=%d", reapNow),
		fmt.Sprintf("LOOM_HEARTBEAT_TTL=%d", reapHBTTL),
		fmt.Sprintf("LOOM_REAP_MAX_AGE=%d", reapMaxAge),
	}
}

// mkWorker creates a worktree dir under root with the given .worker-id content,
// an optional heartbeat (hbMtime>0 ⇒ a .worker-heartbeat stamped to that epoch;
// hbMtime==0 ⇒ no heartbeat file at all), and an optional .worker-claim.
func mkWorker(t *testing.T, root, name, id string, hbMtime int64, claim string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if id != "" {
		if err := os.WriteFile(filepath.Join(dir, ".worker-id"), []byte(id+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if hbMtime > 0 {
		hb := filepath.Join(dir, ".worker-heartbeat")
		if err := os.WriteFile(hb, []byte("beat\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		tm := time.Unix(hbMtime, 0)
		if err := os.Chtimes(hb, tm, tm); err != nil {
			t.Fatal(err)
		}
	}
	if claim != "" {
		if err := os.WriteFile(filepath.Join(dir, ".worker-claim"), []byte(claim+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// reapPaths lays out the non-worktree inputs (ledger / halt-absent / inbox /
// audit) in dir and returns them.
func reapPaths(t *testing.T, dir, ledger string) (ledgerPath, haltPath, inbox, audit string) {
	t.Helper()
	ledgerPath = writeFixture(t, dir, "ledger", ledger)
	haltPath = filepath.Join(dir, "HALT-absent") // deliberately not created
	inbox = writeFixture(t, dir, "inbox.md", "")
	audit = writeFixture(t, dir, "audit.log", "")
	return
}

// runReap runs scripts/reap-workers over the fixtures with hermetic env + the
// deterministic clock. The script always exits 0 (decision-helper shape); a
// non-zero exit is itself a failure.
func runReap(t *testing.T, env []string, root, ledger, halt, inbox, audit string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "scripts", "reap-workers"))
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command("sh", abs, root, ledger, halt, inbox, audit)
	c.Env = append(append(hermeticEnv(), reapEnv()...), env...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("reap-workers exited non-zero: %v\n%s", err, out)
	}
	return string(out)
}

func tombstoned(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".worker-reaped"))
	return err == nil
}

// TestReapSkipHalt: the HALT sentinel freezes the reaper — even a dead+claimed
// worker is left untouched (no verdict per worktree, no tombstone, no envelope).
func TestReapSkipHalt(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "wt")
	w := mkWorker(t, root, "w-halt", "w-halt", staleHB, "item-x")
	ledger, _, inbox, audit := reapPaths(t, dir, fmt.Sprintf("%d w-halt\n", oldStart))
	halt := filepath.Join(dir, "HALT")
	if err := os.WriteFile(halt, []byte("frozen\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := runReap(t, nil, root, ledger, halt, inbox, audit)
	if !strings.Contains(out, "SKIP-HALT") {
		t.Errorf("HALT present must short-circuit to SKIP-HALT:\n%s", out)
	}
	if strings.Contains(out, "REAP") {
		t.Errorf("HALT must freeze the reaper — no REAP verdict allowed:\n%s", out)
	}
	if tombstoned(w) {
		t.Errorf("HALT must not tombstone any worktree")
	}
	if body := readFile(t, inbox); strings.Contains(body, "--- id: reap-") {
		t.Errorf("HALT must mint no PARK envelope:\n%s", body)
	}
}

// TestReapSkipLive: a fresh heartbeat ⇒ the worker is LIVE ⇒ never reaped, even
// though its ledger start is well past max-age. This is the conservative crux.
func TestReapSkipLive(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "wt")
	w := mkWorker(t, root, "w-live", "w-live", freshHB, "item-x")
	ledger, halt, inbox, audit := reapPaths(t, dir, fmt.Sprintf("%d w-live\n", oldStart))

	out := runReap(t, nil, root, ledger, halt, inbox, audit)
	if !strings.Contains(out, "SKIP-LIVE") {
		t.Errorf("fresh heartbeat must be SKIP-LIVE:\n%s", out)
	}
	if strings.Contains(out, "REAP") {
		t.Errorf("a live worker must NEVER be reaped:\n%s", out)
	}
	if tombstoned(w) {
		t.Errorf("a live worker must not be tombstoned")
	}
}

// TestReapClean: dead (stale heartbeat AND past-max-age ledger start) with NO
// claimed item ⇒ REAP-CLEAN — tombstoned, audited, no PARK envelope minted.
func TestReapClean(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "wt")
	w := mkWorker(t, root, "w-clean", "w-clean", staleHB, "")
	ledger, halt, inbox, audit := reapPaths(t, dir, fmt.Sprintf("%d w-clean\n", oldStart))

	out := runReap(t, nil, root, ledger, halt, inbox, audit)
	if !strings.Contains(out, "REAP-CLEAN") {
		t.Errorf("dead + no claim must be REAP-CLEAN:\n%s", out)
	}
	if !tombstoned(w) {
		t.Errorf("REAP-CLEAN must tombstone the worktree (.worker-reaped)")
	}
	if !strings.Contains(readFile(t, audit), "REAP-CLEAN w-clean") {
		t.Errorf("REAP-CLEAN must write an audit line:\n%s", readFile(t, audit))
	}
	if strings.Contains(readFile(t, inbox), "--- id: reap-") {
		t.Errorf("REAP-CLEAN must mint NO PARK envelope (no claim to requeue):\n%s", readFile(t, inbox))
	}
}

// TestReapRequeue: dead AND holding a claimed item ⇒ REAP-REQUEUE — a parser-valid
// PARK envelope is minted (header fields PRECEDE status:, the parked-on predicate is
// worker-died:<id>), and resurface-decide parses it as a real PARKED item (proving
// the envelope is well-formed and that worker-died: is a non-clearing predicate that
// surfaces to a human rather than silently re-running the work).
func TestReapRequeue(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "wt")
	w := mkWorker(t, root, "w-dead", "w-dead", staleHB, "item-77")
	ledger, halt, inbox, audit := reapPaths(t, dir, fmt.Sprintf("%d w-dead\n", oldStart))

	out := runReap(t, nil, root, ledger, halt, inbox, audit)
	if !strings.Contains(out, "REAP-REQUEUE") {
		t.Errorf("dead + held claim must be REAP-REQUEUE:\n%s", out)
	}
	if !tombstoned(w) {
		t.Errorf("REAP-REQUEUE must tombstone the worktree")
	}
	if !strings.Contains(readFile(t, audit), "REAP-REQUEUE w-dead item-77") {
		t.Errorf("REAP-REQUEUE must write an audit line:\n%s", readFile(t, audit))
	}

	body := readFile(t, inbox)
	for _, want := range []string{
		"--- id: reap-w-dead",
		"claimed-item: item-77",
		"parked-on: worker-died:w-dead",
		fmt.Sprintf("parked-at: %d", reapNow),
		"retry: 1",
		"status: PARKED",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("PARK envelope missing %q:\n%s", want, body)
		}
	}
	// Parser contract: header fields (parked-on) MUST precede status:.
	if pi, si := strings.Index(body, "parked-on:"), strings.Index(body, "status:"); pi < 0 || pi > si {
		t.Errorf("parked-on must precede status: (drain parser contract):\n%s", body)
	}

	// The minted envelope must be recognized by the REAL resurface decision as a
	// valid PARKED item — not "no-op" and not "malformed". worker-died: never
	// clears, and NOW==parked-at is under max-age, so the decision is SKIP-PARKED.
	merged := writeFixture(t, dir, "merged.txt", "")
	rOut := runScript(t, "resurface-decide",
		[]string{fmt.Sprintf("LOOM_NOW=%d", reapNow), "LOOM_MAX_PARK_AGE=604800"}, inbox, merged)
	var line string
	for _, l := range strings.Split(rOut, "\n") {
		if strings.HasPrefix(l, "reap-w-dead:") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("resurface-decide produced no decision for the minted park:\n%s", rOut)
	}
	if !strings.Contains(line, "SKIP-PARKED") || strings.Contains(line, "malformed") {
		t.Errorf("minted park must parse as a valid PARKED item (SKIP-PARKED), got: %q", line)
	}
}

// TestReapIdempotent: a second run over the same worktree is a no-op — the
// reap-marker makes it SKIP-REAPED, so the inbox is not double-minted and the
// audit is not re-appended.
func TestReapIdempotent(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "wt")
	mkWorker(t, root, "w-dead", "w-dead", staleHB, "item-77")
	ledger, halt, inbox, audit := reapPaths(t, dir, fmt.Sprintf("%d w-dead\n", oldStart))

	runReap(t, nil, root, ledger, halt, inbox, audit)
	out2 := runReap(t, nil, root, ledger, halt, inbox, audit)
	if !strings.Contains(out2, "SKIP-REAPED") {
		t.Errorf("second run must be SKIP-REAPED (idempotent):\n%s", out2)
	}
	if n := strings.Count(readFile(t, inbox), "--- id: reap-"); n != 1 {
		t.Errorf("re-run double-minted: got %d envelopes, want 1", n)
	}
	if n := strings.Count(readFile(t, audit), "REAP-REQUEUE"); n != 1 {
		t.Errorf("re-run double-audited: got %d audit lines, want 1", n)
	}
}

// TestReapFailSafe: every ambiguous shape resolves to SKIP and NEVER to a REAP —
// no ledger start, a not-yet-past-max-age start, a garbage ledger, and a malformed
// .worker-id. None may be tombstoned or minted (the destructive-reap floor).
func TestReapFailSafe(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "wt")
	// Each worker has a DEAD (stale) heartbeat + a claim, so ONLY the ledger /
	// id ambiguity can save it from a reap — proving the fail-safe gate, not luck.
	wNoLedger := mkWorker(t, root, "w-noledger", "w-noledger", staleHB, "item-a")
	wRecent := mkWorker(t, root, "w-recent", "w-recent", staleHB, "item-b")
	wGarbage := mkWorker(t, root, "w-garbage", "w-garbage", staleHB, "item-c")
	wBadID := mkWorker(t, root, "w-badid", "bad;id", staleHB, "item-d")

	ledger := fmt.Sprintf("not-an-epoch w-garbage\n%d w-recent\n%d w-someoneelse\n",
		recentStart, oldStart)
	ledgerPath, halt, inbox, audit := reapPaths(t, dir, ledger)

	out := runReap(t, nil, root, ledgerPath, halt, inbox, audit)
	if strings.Contains(out, "REAP") {
		t.Errorf("no ambiguous worker may be REAPed (fail-safe):\n%s", out)
	}
	for _, w := range []string{wNoLedger, wRecent, wGarbage, wBadID} {
		if tombstoned(w) {
			t.Errorf("fail-safe worker %s must not be tombstoned", filepath.Base(w))
		}
	}
	if strings.Contains(readFile(t, inbox), "--- id: reap-") {
		t.Errorf("fail-safe must mint no PARK envelope:\n%s", readFile(t, inbox))
	}
	if strings.TrimSpace(readFile(t, audit)) != "" {
		t.Errorf("fail-safe must write no audit line:\n%s", readFile(t, audit))
	}
}

// TestReapInjection: a worktree DIRECTORY NAME packed with shell metachars
// (command-substitution, ${IFS}, ;) must reach no shell — the loop globs and
// quotes the path, so no payload file is created. The worker is dead+claimed so it
// goes all the way through the REAP-REQUEUE mint/tombstone path; the envelope id
// derives from the VALIDATED .worker-id content, never the dir name.
func TestReapInjection(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "wt")
	// Run with a clean CWD so a stray relative `touch` would be detectable there.
	cwd := t.TempDir()
	marker := "PWNEDMARKER"

	evil := "ev;l `touch " + marker + "` $(touch " + marker + ") ${IFS}x"
	w := mkWorker(t, root, evil, "w-evil", staleHB, "item-evil")
	ledger, halt, inbox, audit := reapPaths(t, dir, fmt.Sprintf("%d w-evil\n", oldStart))

	abs, err := filepath.Abs(filepath.Join("..", "..", "scripts", "reap-workers"))
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command("sh", abs, root, ledger, halt, inbox, audit)
	c.Dir = cwd
	c.Env = append(hermeticEnv(), reapEnv()...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("reap-workers exited non-zero: %v\n%s", err, out)
	}

	if _, err := os.Stat(filepath.Join(cwd, marker)); !os.IsNotExist(err) {
		t.Errorf("INJECTION: %s created — a worktree dir name reached a shell (err=%v)", marker, err)
	}
	if !tombstoned(w) {
		t.Errorf("the metachar-named dead worker should still be reaped (REAP-REQUEUE) via its validated id")
	}
	body := readFile(t, inbox)
	if !strings.Contains(body, "--- id: reap-w-evil") {
		t.Errorf("envelope id must come from the validated .worker-id, not the dir name:\n%s", body)
	}
	for _, bad := range []string{"${IFS}", "$(", "`", ";l"} {
		if strings.Contains(body, bad) {
			t.Errorf("raw metachar payload %q leaked into the minted envelope:\n%s", bad, body)
		}
	}
}
