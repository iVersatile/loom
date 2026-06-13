package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAutopilotDrainLiveness covers FR-DOCTOR-005: doctor flags the
// silent-idle anomaly (AUTOPILOT on + eligible QUEUED + no drain evidence) and
// stays quiet/green otherwise. The .drain-count absence under live queued work
// is the exact signature of the 2026-06-12 never-loaded-Stop-hook incident.
func TestAutopilotDrainLiveness(t *testing.T) {
	const queuedItem = "--- id: x1\nstatus: QUEUED\n\nbody\n"

	cases := []struct {
		name        string
		inbox       *string // nil => no loom-author.md on the tree
		drainCount  bool    // whether .drain-count exists
		wantApplies bool
		wantOK      bool
	}{
		{name: "no inbox — not graded", inbox: nil, wantApplies: false},
		{name: "autopilot off — green", inbox: strptr("AUTOPILOT: off\n" + queuedItem), drainCount: false, wantApplies: true, wantOK: true},
		{name: "autopilot on, no queued — green", inbox: strptr("AUTOPILOT: on\n--- id: w\nstatus: WAITING\n"), drainCount: false, wantApplies: true, wantOK: true},
		{name: "ANOMALY: on + queued + no drain evidence", inbox: strptr("AUTOPILOT: on\n" + queuedItem), drainCount: false, wantApplies: true, wantOK: false},
		{name: "on + queued + drain has fired — green", inbox: strptr("AUTOPILOT: on\n" + queuedItem), drainCount: true, wantApplies: true, wantOK: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			inboxDir := filepath.Join(root, ".scratch", "inbox")
			if err := os.MkdirAll(inboxDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if tc.inbox != nil {
				if err := os.WriteFile(filepath.Join(inboxDir, "loom-author.md"), []byte(*tc.inbox), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.drainCount {
				if err := os.WriteFile(filepath.Join(inboxDir, ".drain-count"), []byte("1\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got, applies := autopilotDrainLiveness(root)
			if applies != tc.wantApplies {
				t.Fatalf("applicable = %v, want %v", applies, tc.wantApplies)
			}
			if !applies {
				return
			}
			if got.Name != "host:autopilot" {
				t.Errorf("check name = %q, want host:autopilot", got.Name)
			}
			if got.OK != tc.wantOK {
				t.Errorf("OK = %v, want %v (detail: %s)", got.OK, tc.wantOK, got.Detail)
			}
			if got.Detail == "" {
				t.Error("check must carry a Detail")
			}
		})
	}
}

func strptr(s string) *string { return &s }
