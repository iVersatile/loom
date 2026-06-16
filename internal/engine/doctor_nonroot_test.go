package engine

import (
	"errors"
	"testing"

	"github.com/iVersatile/loom/internal/playbook"
)

// TestRoleMarkerPermsOK pins container:role-marker-perms (adv-064): the marker is
// non-forgeable iff root-owned with no group/other write bit.
func TestRoleMarkerPermsOK(t *testing.T) {
	cases := []struct {
		name        string
		owner, mode string
		want        bool
	}{
		{"root 0644 ok", "root", "644", true},
		{"root 0600 ok", "root", "600", true},
		{"group-writable fails", "root", "664", false},
		{"other-writable fails", "root", "646", false},
		{"non-root owner fails", "loom", "644", false},
		{"unparseable mode fails", "root", "rwx", false},
	}
	for _, c := range cases {
		if got, _ := roleMarkerPermsOK(c.owner, c.mode); got != c.want {
			t.Errorf("%s: roleMarkerPermsOK(%q,%q)=%t want %t", c.name, c.owner, c.mode, got, c.want)
		}
	}
}

// TestWorkspaceWritableOK pins container:workspace-writable (adv-064): the
// runtime uid owns the workspace, or the dir is group/other-writable; the
// uid-collision case (mismatch + non-writable mode) is the loud failure.
func TestWorkspaceWritableOK(t *testing.T) {
	cases := []struct {
		name                    string
		userUID, ownerUID, mode string
		want                    bool
	}{
		{"uid match ok", "1000", "1000", "755", true},
		{"uid collision fails", "1001", "1000", "755", false},
		{"collision but other-writable ok", "1001", "1000", "757", true},
		{"collision but group-writable ok", "1001", "1000", "775", true},
		{"empty uid fails", "", "1000", "755", false},
		{"mismatch + unparseable mode fails", "1001", "1000", "zzz", false},
	}
	for _, c := range cases {
		if got, _ := workspaceWritableOK(c.userUID, c.ownerUID, c.mode); got != c.want {
			t.Errorf("%s: workspaceWritableOK(%q,%q,%q)=%t want %t", c.name, c.userUID, c.ownerUID, c.mode, got, c.want)
		}
	}
}

// TestNonRootContainerChecksGatedOnUser pins the grading gate: root/unset user ⇒
// no non-root checks (the invariants are N/A under Model A's root container).
func TestNonRootContainerChecksGatedOnUser(t *testing.T) {
	rt := fakeRuntime{}
	for _, user := range []string{"", "root"} {
		if c := nonRootContainerChecks(rt, "demo-dev", playbook.Playbook{Name: "demo", User: user}); c != nil {
			t.Errorf("user %q: expected no checks (not graded), got %+v", user, c)
		}
	}
}

// TestNonRootContainerChecksProbesLive pins the happy non-root path: a root-owned
// 0644 marker and a uid-matched workspace ⇒ both checks OK, read from the live
// probes (read-only stat/id — doctor never mutates).
func TestNonRootContainerChecksProbesLive(t *testing.T) {
	rt := fakeRuntime{
		statOut: map[string][3]string{
			"/var/lib/loom/role": {"root", "644", "0"},
			"/workspace/demo":    {"loom", "755", "1000"},
		},
		userUIDs: map[string]string{"loom": "1000"},
	}
	checks := nonRootContainerChecks(rt, "demo-dev", playbook.Playbook{Name: "demo", User: "loom"})
	got := map[string]bool{}
	for _, c := range checks {
		got[c.Name] = c.OK
	}
	for _, name := range []string{"container:role-marker-perms", "container:workspace-writable"} {
		ok, present := got[name]
		if !present {
			t.Errorf("missing check %q (got %+v)", name, checks)
		}
		if !ok {
			t.Errorf("check %q should be OK on the happy path", name)
		}
	}
}

// TestNonRootContainerChecksFailLoud pins the failure surface: a forgeable marker
// (group-writable) and a uid-collision workspace ⇒ both checks FAIL with detail,
// and an unreadable marker fails closed rather than passing silently.
func TestNonRootContainerChecksFailLoud(t *testing.T) {
	rt := fakeRuntime{
		statOut: map[string][3]string{
			"/var/lib/loom/role": {"root", "664", "0"}, // group-writable: forgeable
			"/workspace/demo":    {"root", "755", "0"}, // owned by root, not loom
		},
		userUIDs: map[string]string{"loom": "1001"}, // uid collision
	}
	checks := nonRootContainerChecks(rt, "demo-dev", playbook.Playbook{Name: "demo", User: "loom"})
	for _, c := range checks {
		if c.OK {
			t.Errorf("check %q should FAIL (detail: %s)", c.Name, c.Detail)
		}
	}

	// Marker unreadable (absent on a non-root container) ⇒ fail closed.
	rt2 := fakeRuntime{
		statErr:  map[string]error{"/var/lib/loom/role": errors.New("No such file")},
		statOut:  map[string][3]string{"/workspace/demo": {"loom", "755", "1000"}},
		userUIDs: map[string]string{"loom": "1000"},
	}
	for _, c := range nonRootContainerChecks(rt2, "demo-dev", playbook.Playbook{Name: "demo", User: "loom"}) {
		if c.Name == "container:role-marker-perms" && c.OK {
			t.Error("an unreadable marker must fail container:role-marker-perms, not pass silently")
		}
	}
}
