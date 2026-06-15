package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidRole: marker-safe identities only — non-empty, [A-Za-z0-9_-]. Anything
// that could inject into the marker-write shell (spaces, metachars, slashes) is
// rejected so roleMarkerScript fails safe to "" (no marker).
func TestValidRole(t *testing.T) {
	for _, ok := range []string{"loom-author", "loom_advisor", "agent1", "R"} {
		if !validRole(ok) {
			t.Errorf("validRole(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "loom author", "a;rm -rf /", "a/b", "$(id)", "a`b`", "a\nb"} {
		if validRole(bad) {
			t.Errorf("validRole(%q) = true, want false", bad)
		}
	}
}

// TestRoleMarkerScript (ADR-0019 PR4 Part 1): empty for an invalid/unset role;
// for a valid role it writes a ROOT-OWNED, 0644, single-line /var/lib/loom/role.
// Asserting the SCRIPT (not the live docker exec) is the gate-testable proxy —
// the same pattern as TestProvisionUserScript / TestChownHomeScript.
func TestRoleMarkerScript(t *testing.T) {
	if s := roleMarkerScript(""); s != "" {
		t.Errorf("empty role must emit no marker script, got %q", s)
	}
	if s := roleMarkerScript("bad; touch /pwn"); s != "" {
		t.Errorf("a metachar role must be rejected (no script), got %q", s)
	}
	s := roleMarkerScript("loom-author")
	for _, want := range []string{
		"mkdir -p /var/lib/loom",             // parent dir
		"printf '%s\\n' 'loom-author'",       // single line, exact role
		"> /var/lib/loom/role",               // the marker path
		"chown root:root /var/lib/loom/role", // root-owned: readable, not forgeable
		"chmod 0644 /var/lib/loom/role",      // world-readable, root-writable only
	} {
		if !strings.Contains(s, want) {
			t.Errorf("roleMarkerScript missing %q\n---\n%s", want, s)
		}
	}
}

// TestBuildPopulatesRole (ADR-0019 PR4 Part 1): build plumbs LOOM_SESSION_ROLE
// onto ContainerSpec.Role — the overlay/build source (not a playbook key). Unset
// ⇒ empty (no marker, root behavior unchanged). Part 1 only LAYS the value; the
// drain-guard reads the marker after the human Part-2 swap.
func TestBuildPopulatesRole(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")

	t.Run("env set flows to spec.Role", func(t *testing.T) {
		t.Setenv("LOOM_SESSION_ROLE", "loom-author")
		var spec ContainerSpec
		rt := fakeRuntime{
			ensureInfo:   ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: "created"},
			ensureRecord: &spec,
		}
		if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err != nil {
			t.Fatalf("build: %v", err)
		}
		if spec.Role != "loom-author" {
			t.Errorf("spec.Role = %q, want loom-author", spec.Role)
		}
	})

	t.Run("env unset ⇒ empty role (no marker)", func(t *testing.T) {
		t.Setenv("LOOM_SESSION_ROLE", "")
		var spec ContainerSpec
		rt := fakeRuntime{
			ensureInfo:   ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: "created"},
			ensureRecord: &spec,
		}
		if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err != nil {
			t.Fatalf("build: %v", err)
		}
		if spec.Role != "" {
			t.Errorf("spec.Role = %q, want empty", spec.Role)
		}
	})
}

// TestDoctorContainerUser (ADR-0019 PR4 Part 1): doctor surfaces the container's
// runtime-user model. Under Model A the runtime user is root; a configured
// non-root user is named as the per-verb exec identity.
func TestDoctorContainerUser(t *testing.T) {
	rt := fakeRuntime{exists: true, running: true,
		probeVersions: map[string]string{"git": "2.43", "jq": "1.7", "go": "go1.26.4", "gopls": "0.16"}}

	// Default fixture (no user:) — runtime user is plain root.
	res, err := doctorImpl(DoctorOpts{PlaybookPath: testFixture}, rt)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	c, ok := checksByName(res)["container:user"]
	if !ok || !c.OK {
		t.Fatalf("container:user must be present + OK, got %+v (ok=%v)", c, ok)
	}
	if !strings.Contains(c.Detail, "root") || strings.Contains(c.Detail, "exec -u") {
		t.Errorf("root container detail should be plain root, got %q", c.Detail)
	}

	// Non-root user: the detail names the per-verb exec user.
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	data, err := os.ReadFile(pbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pbPath, append(data, []byte("\nuser: agent\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	res2, err := doctorImpl(DoctorOpts{PlaybookPath: pbPath}, rt)
	if err != nil {
		t.Fatalf("doctor (non-root): %v", err)
	}
	c2 := checksByName(res2)["container:user"]
	if !strings.Contains(c2.Detail, "exec -u agent") {
		t.Errorf("non-root detail must name the entry-verb exec user, got %q", c2.Detail)
	}
}
