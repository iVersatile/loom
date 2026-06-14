package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iVersatile/loom/internal/playbook"
)

// TestRoleMarkerReminder covers FR-DOCTOR-006: the doctor fires the T10 PR-4
// reminder exactly when a non-root user: is configured but the drain-guard
// still uses the id -un==root fallback (which fail-closes on a non-root
// container and silences the drain). Root/unset user or no drain hook = not
// graded.
func TestRoleMarkerReminder(t *testing.T) {
	wired := "role=$(cat /var/lib/loom/role)\n"
	fallback := "[ \"$(id -un)\" = \"root\" ] && role=\"loom-author\"\n"

	cases := []struct {
		name        string
		user        string
		hook        *string // nil => no drain hook on the tree
		wantApplies bool
		wantOK      bool
	}{
		{name: "root user — not graded", user: "root", hook: &fallback, wantApplies: false},
		{name: "unset user — not graded", user: "", hook: &fallback, wantApplies: false},
		{name: "non-root + no drain hook — not graded", user: "dev", hook: nil, wantApplies: false},
		{name: "non-root + marker-wired — OK", user: "dev", hook: &wired, wantApplies: true, wantOK: true},
		{name: "non-root + id-un fallback — FAIL (reminder fires)", user: "dev", hook: &fallback, wantApplies: true, wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.hook != nil {
				hookDir := filepath.Join(root, ".claude", "hooks")
				if err := os.MkdirAll(hookDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(hookDir, "drain-inbox.sh"), []byte(*tc.hook), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			got, applies := roleMarkerWired(playbook.Playbook{User: tc.user}, root)
			if applies != tc.wantApplies {
				t.Fatalf("applicable = %v, want %v", applies, tc.wantApplies)
			}
			if !applies {
				return
			}
			if got.Name != "host:role-marker" {
				t.Errorf("name = %q, want host:role-marker", got.Name)
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
