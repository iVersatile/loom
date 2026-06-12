package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests exercise the drain-inbox Stop hook's ROLE GUARD (LL-011): the
// hook registers repo-level, so it fires in every session on the shared tree,
// but it owns loom-author's inbox only. A foreign-role session stopping with
// the Writer's AUTOPILOT on must NOT drain — the 2026-06-11 incident had an
// advisor stop flip the Writer's queued item to TAKEN, which the Writer's own
// drain would then have skipped silently.

// drainFixture builds a minimal project tree the hook can drain: an inbox with
// AUTOPILOT on + one QUEUED item whose serves: matches a tactical-queue row.
func drainFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	inboxDir := filepath.Join(root, ".scratch", "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	inbox := `AUTOPILOT: on

--- id: 001
from: loom-advisor
serves: fixture task row
kind: task
status: QUEUED

Fixture body.
`
	queue := "| task | depends-on | serves | owner | status | PR |\n" +
		"| --- | --- | --- | --- | --- | --- |\n" +
		"| fixture task row | — | testing | loom-author | queued | — |\n"
	if err := os.WriteFile(filepath.Join(inboxDir, "loom-author.md"), []byte(inbox), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "PLAN.md"), []byte(queue), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// runDrain executes the real hook script against the fixture root with an
// explicit session role, feeding a Stop-hook stdin payload. Hermetic env per
// LL-006/LL-010.
func runDrain(t *testing.T, root, role string) (stdout string, err error) {
	t.Helper()
	hook, aerr := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", "drain-inbox.sh"))
	if aerr != nil {
		t.Fatal(aerr)
	}
	c := exec.Command("sh", hook)
	c.Dir = root
	c.Env = append(hermeticEnv(),
		"CLAUDE_PROJECT_DIR="+root,
		"LOOM_SESSION_ROLE="+role,
	)
	c.Stdin = strings.NewReader(`{"stop_hook_active": false}`)
	out, err := c.Output()
	return string(out), err
}

func readInbox(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".scratch", "inbox", "loom-author.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A foreign role stopping with the Writer's AUTOPILOT on: the hook must allow
// a normal stop (exit 0, no output) and leave the Writer's inbox untouched.
func TestDrainRoleGuardForeignRoleNoops(t *testing.T) {
	root := drainFixture(t)
	before := readInbox(t, root)

	out, err := runDrain(t, root, "loom-advisor")
	if err != nil {
		t.Fatalf("foreign-role stop must exit 0, got: %v", err)
	}
	if out != "" {
		t.Fatalf("foreign-role stop must produce no drain decision, got: %q", out)
	}
	if got := readInbox(t, root); got != before {
		t.Fatalf("foreign-role stop mutated the Writer's inbox:\n--- before ---\n%s\n--- after ---\n%s", before, got)
	}
	if _, err := os.Stat(filepath.Join(root, ".scratch", "inbox", ".drain-count")); !os.IsNotExist(err) {
		t.Fatal("foreign-role stop must not start a drain budget counter")
	}
}

// Kill-switch (T23): a HALT file gates ALL drains to no-op regardless of the
// AUTOPILOT header — even the owning role must stop normally and leave the
// inbox untouched.
func TestDrainHaltKillSwitchGatesOwnerRole(t *testing.T) {
	root := drainFixture(t)
	if err := os.WriteFile(filepath.Join(root, ".scratch", "inbox", "HALT"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	before := readInbox(t, root)

	out, err := runDrain(t, root, "loom-author")
	if err != nil {
		t.Fatalf("HALT-gated stop must exit 0, got: %v", err)
	}
	if out != "" {
		t.Fatalf("HALT-gated stop must produce no drain decision, got: %q", out)
	}
	if got := readInbox(t, root); got != before {
		t.Fatalf("HALT-gated stop mutated the inbox:\n--- before ---\n%s\n--- after ---\n%s", before, got)
	}
}

// Non-work kinds (T25): fyi and draft are never ridden and never
// orphan-flagged — the drain skips them BEFORE the orphan check. With a fyi
// (QUEUED, no serves), a draft, and a legal task in the inbox, the drain
// must pick the task and the continuation report must NOT flag the others.
func TestDrainSkipsFyiAndDraftKinds(t *testing.T) {
	root := drainFixture(t)
	inboxPath := filepath.Join(root, ".scratch", "inbox", "loom-author.md")
	extra := `
--- id: 002
from: loom-advisor
kind: fyi
status: QUEUED

Trial flip scheduled tomorrow first thing.

--- id: 003
from: loom-advisor
kind: draft
status: QUEUED

Half-formed proposal awaiting /coordinate triage.
`
	f, err := os.OpenFile(inboxPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(extra); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	out, err := runDrain(t, root, "loom-author")
	if err != nil {
		t.Fatalf("drain with mixed kinds must succeed: %v", err)
	}
	if !strings.Contains(out, "inbox item 001") {
		t.Fatalf("drain must ride the task item, got: %q", out)
	}
	if strings.Contains(out, "skipped") || strings.Contains(out, "002") || strings.Contains(out, "003") {
		t.Fatalf("fyi/draft must be neither ridden nor orphan-flagged, got: %q", out)
	}
	if got := readInbox(t, root); !strings.Contains(got, "kind: fyi\nstatus: QUEUED") {
		t.Fatalf("fyi item must be untouched by the drain, inbox:\n%s", got)
	}
}

// A fyi-only inbox (AUTOPILOT on): normal stop — no decision, no flags.
func TestDrainFyiOnlyInboxStopsNormally(t *testing.T) {
	root := drainFixture(t)
	inboxPath := filepath.Join(root, ".scratch", "inbox", "loom-author.md")
	fyiOnly := `AUTOPILOT: on

--- id: 001
from: loom-advisor
kind: fyi
status: UNREAD

Heads-up only; nothing to ride.
`
	if err := os.WriteFile(inboxPath, []byte(fyiOnly), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runDrain(t, root, "loom-author")
	if err != nil {
		t.Fatalf("fyi-only stop must exit 0, got: %v", err)
	}
	if out != "" {
		t.Fatalf("fyi-only inbox must produce no drain decision, got: %q", out)
	}
}

// The owning role drains: decision block emitted, item flipped to TAKEN.
func TestDrainRoleGuardOwnerRoleDrains(t *testing.T) {
	root := drainFixture(t)

	out, err := runDrain(t, root, "loom-author")
	if err != nil {
		t.Fatalf("owner-role drain failed: %v", err)
	}
	if !strings.Contains(out, `"decision": "block"`) || !strings.Contains(out, "inbox item 001") {
		t.Fatalf("owner-role stop must emit the drain decision for item 001, got: %q", out)
	}
	if got := readInbox(t, root); !strings.Contains(got, "status: TAKEN") {
		t.Fatalf("owner-role drain must flip the item to TAKEN, inbox:\n%s", got)
	}
}
