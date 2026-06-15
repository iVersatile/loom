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

// addLine appends a playbook line to the temp-project loom.yml (test helper for
// the declarative role:/user: cases — same shape as TestDoctorContainerUser).
func addLine(t *testing.T, pbPath, line string) {
	t.Helper()
	data, err := os.ReadFile(pbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pbPath, append(data, []byte("\n"+line+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestBuildPopulatesRole (ADR-0019 PR4 §5, LL-014): build plumbs the DECLARATIVE
// playbook role: onto ContainerSpec.Role; LOOM_SESSION_ROLE is a demoted override
// that wins when set; neither ⇒ empty (no marker, root behavior unchanged). The
// value only LAYS here; the drain-guard reads the marker after human Part 2.
func TestBuildPopulatesRole(t *testing.T) {
	build := func(t *testing.T, pbPath string) ContainerSpec {
		t.Helper()
		var spec ContainerSpec
		rt := fakeRuntime{
			ensureInfo:   ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: "created"},
			ensureRecord: &spec,
		}
		if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err != nil {
			t.Fatalf("build: %v", err)
		}
		return spec
	}

	t.Run("declarative role: flows to spec.Role", func(t *testing.T) {
		t.Setenv("LOOM_SESSION_ROLE", "")
		pbPath := filepath.Join(tempProject(t), "loom.yml")
		addLine(t, pbPath, "role: loom-author")
		if spec := build(t, pbPath); spec.Role != "loom-author" {
			t.Errorf("spec.Role = %q, want loom-author (declarative source)", spec.Role)
		}
	})

	t.Run("LOOM_SESSION_ROLE overrides the playbook role:", func(t *testing.T) {
		t.Setenv("LOOM_SESSION_ROLE", "loom-advisor")
		pbPath := filepath.Join(tempProject(t), "loom.yml")
		addLine(t, pbPath, "role: loom-author")
		if spec := build(t, pbPath); spec.Role != "loom-advisor" {
			t.Errorf("spec.Role = %q, want loom-advisor (env override wins)", spec.Role)
		}
	})

	t.Run("neither ⇒ empty role (no marker)", func(t *testing.T) {
		t.Setenv("LOOM_SESSION_ROLE", "")
		pbPath := filepath.Join(tempProject(t), "loom.yml")
		if spec := build(t, pbPath); spec.Role != "" {
			t.Errorf("spec.Role = %q, want empty", spec.Role)
		}
	})
}

// TestNeedsRoleMarker (LL-014 defect 3) is the gate-testable proxy for the
// convergence fold: a missing marker on an already-converged container reads ""
// and MUST re-trigger a write on the next plain build; an up-to-date marker is a
// no-op; an empty/invalid want never writes (root compatibility). The docker-side
// readRoleMarker + Ensure early-return wiring is integration-validated.
func TestNeedsRoleMarker(t *testing.T) {
	cases := []struct {
		have, want string
		need       bool
	}{
		{"", "loom-author", true},             // defect-3: marker absent on a converged container ⇒ heal
		{"loom-author", "loom-author", false}, // present + current ⇒ no-op
		{"loom-old", "loom-author", true},     // stale ⇒ rewrite
		{"loom-author", "", false},            // want empty (root) ⇒ never write
		{"", "", false},                       // nothing wanted ⇒ no-op
		{"", "bad role", false},               // invalid want ⇒ never write (fail-safe)
	}
	for _, c := range cases {
		if got := needsRoleMarker(c.have, c.want); got != c.need {
			t.Errorf("needsRoleMarker(%q,%q) = %v, want %v", c.have, c.want, got, c.need)
		}
	}
}

// TestRoleMarkerPlan (LL-014 defect 2): the empty-role no-op must fail loud. A
// non-root user with no valid role is a HARD ERROR; root + empty/invalid is a
// visible warning (no marker); a valid role is silent (marker written).
func TestRoleMarkerPlan(t *testing.T) {
	// Hard error: non-root user, no/invalid role.
	for _, role := range []string{"", "bad role"} {
		if w, err := roleMarkerPlan("agent", role); err == nil {
			t.Errorf("roleMarkerPlan(agent,%q): want error, got warning %q", role, w)
		}
	}
	// Warning, no error: root/unset + empty or invalid role.
	for _, user := range []string{"", "root"} {
		w, err := roleMarkerPlan(user, "")
		if err != nil {
			t.Errorf("roleMarkerPlan(%q,\"\"): want warning not error, got %v", user, err)
		}
		if w == "" {
			t.Errorf("roleMarkerPlan(%q,\"\"): want a visible warning, got none", user)
		}
	}
	// Valid role: silent on both axes (the marker is written).
	if w, err := roleMarkerPlan("agent", "loom-author"); w != "" || err != nil {
		t.Errorf("roleMarkerPlan(agent,loom-author) = (%q,%v), want (\"\",nil)", w, err)
	}
}

// TestBuildRoleLoudFail wires the loud-fail through buildImpl: a non-root user:
// with no role: aborts the build with an error; root + no role: succeeds with a
// surfaced warning (and no marker). LL-014 defect 2 at the verb boundary.
func TestBuildRoleLoudFail(t *testing.T) {
	rt := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: "created"}}

	t.Run("non-root user + no role ⇒ hard error", func(t *testing.T) {
		t.Setenv("LOOM_SESSION_ROLE", "")
		pbPath := filepath.Join(tempProject(t), "loom.yml")
		addLine(t, pbPath, "user: agent")
		if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err == nil {
			t.Fatal("want a hard error for non-root user with no role, got nil")
		}
	})

	t.Run("non-root user + role ⇒ ok", func(t *testing.T) {
		t.Setenv("LOOM_SESSION_ROLE", "")
		pbPath := filepath.Join(tempProject(t), "loom.yml")
		addLine(t, pbPath, "user: agent")
		addLine(t, pbPath, "role: agent-role")
		if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err != nil {
			t.Fatalf("non-root user WITH a role must build: %v", err)
		}
	})

	t.Run("root + no role ⇒ warning, not error", func(t *testing.T) {
		t.Setenv("LOOM_SESSION_ROLE", "")
		pbPath := filepath.Join(tempProject(t), "loom.yml")
		res, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock)
		if err != nil {
			t.Fatalf("root + no role must not error: %v", err)
		}
		if len(res.Warnings) == 0 {
			t.Error("root + no role must surface a visible warning")
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
