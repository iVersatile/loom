package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// autopilotDrainLiveness grades the Writer's AUTOPILOT drain against its only
// host-observable side-effect. The drain (.claude/hooks/drain-inbox.sh) is a
// Stop hook: when AUTOPILOT is on it hands the loom-author inbox's QUEUED items
// to the session. But a session launched outside the repo root never loads the
// repo's hook registration, so eligible work sits undrained and SILENT — the
// 2026-06-12 incident, where autopilot was on, an item was queued, the Stop
// hook was never registered, and no .drain-count was ever written. You could
// not tell "working" from "silently idle".
//
// This mechanizes the hand-check human drafts 032/033 (T27 facet B) asked for:
//
//	AUTOPILOT on + eligible QUEUED + no drain evidence = ANOMALY.
//
// It is point-in-time and side-effect-based BY DESIGN. The drain's `.drain-count`
// file is the evidence the hook ever ran; its absence under live queued work is
// the exact, unambiguous signature of the never-loaded-hook bug. True live-idle
// detection BEYOND the first drain (a hook that loaded once then stopped firing)
// needs a session heartbeat primitive this claim deliberately does not fake —
// it reports what it can prove and names what it cannot (T27 facet B remainder).
//
// The bool return is the "applicable" flag (cf. githooksWired): a tree with no
// loom-author inbox is not graded at all, rather than graded green.
func autopilotDrainLiveness(root string) (Check, bool) {
	inboxDir := filepath.Join(root, ".scratch", "inbox")
	data, err := os.ReadFile(filepath.Join(inboxDir, "loom-author.md"))
	if err != nil {
		return Check{}, false // no Writer inbox on this tree — nothing to grade
	}
	if autopilotState(data) != "on" {
		return Check{Name: "host:autopilot", OK: true, Detail: "drain inactive (AUTOPILOT off)"}, true
	}
	queued := countQueuedItems(data)
	if queued == 0 {
		return Check{Name: "host:autopilot", OK: true, Detail: "AUTOPILOT on, no QUEUED work to drain"}, true
	}
	if _, serr := os.Stat(filepath.Join(inboxDir, ".drain-count")); serr != nil {
		return Check{Name: "host:autopilot", OK: false, Detail: fmt.Sprintf(
			"AUTOPILOT on with %d QUEUED item(s) but the drain has never run (.drain-count absent) — "+
				"the Writer's Stop hook is likely not loaded; its session cwd must be the repo root "+
				"(CLAUDE_PROJECT_DIR)", queued)}, true
	}
	return Check{Name: "host:autopilot", OK: true, Detail: fmt.Sprintf(
		"AUTOPILOT on, %d QUEUED, drain has fired (live-idle detection beyond the first drain "+
			"needs a heartbeat — T27 facet B)", queued)}, true
}

// autopilotState returns the first AUTOPILOT header value (matching the drain
// hook's own `sed -n …; head -1` semantics), or "" if unset.
func autopilotState(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "AUTOPILOT:"); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// countQueuedItems counts inbox items whose status line is QUEUED — the drain's
// own eligibility floor (WAITING items are not yet drainable; QUEUED are). A
// trailing note after QUEUED (e.g. a flip timestamp) still counts.
func countQueuedItems(data []byte) int {
	n := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "status: QUEUED") {
			n++
		}
	}
	return n
}
