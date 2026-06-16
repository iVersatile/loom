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

// TestRoleMarkerPlan (LL-014 defect 2; realigned to the frozen SPEC + ADR-0019 §5,
// adv-067 TASK 2): the rule keys on BOTH (user, role). root + no role ⇒ visible
// warning (no marker, fallback intact); NON-ROOT + no role ⇒ HARD error (an empty
// marker silently breaks the drain role-guard); a malformed role ⇒ hard error
// (any user); a valid role ⇒ silent.
func TestRoleMarkerPlan(t *testing.T) {
	// root (unset or "root") + no role ⇒ warning, no error.
	for _, user := range []string{"", "root"} {
		w, err := roleMarkerPlan(user, "")
		if err != nil {
			t.Errorf("roleMarkerPlan(%q,\"\"): want warning not error, got %v", user, err)
		}
		if w == "" {
			t.Errorf("roleMarkerPlan(%q,\"\"): want a visible warning, got none", user)
		}
	}
	// NON-ROOT user + no role ⇒ HARD error (the spec-realign: empty marker breaks
	// the drain role-guard on a non-root container).
	if w, err := roleMarkerPlan("loom", ""); err == nil {
		t.Errorf("roleMarkerPlan(loom,\"\"): want hard error for a non-root user with no role, got warning %q", w)
	}
	// Declared but not marker-safe ⇒ hard error (independent of the user).
	for _, role := range []string{"bad role", "a;rm -rf /", "a/b"} {
		if w, err := roleMarkerPlan("", role); err == nil {
			t.Errorf("roleMarkerPlan(\"\",%q): want error, got warning %q", role, w)
		}
	}
	// Valid role ⇒ silent (either user).
	for _, user := range []string{"", "loom"} {
		if w, err := roleMarkerPlan(user, "loom-author"); w != "" || err != nil {
			t.Errorf("roleMarkerPlan(%q,loom-author) = (%q,%v), want (\"\",nil)", user, w, err)
		}
	}
}

// TestBuildRoleLoudFail wires the loud-fail through buildImpl, realigned to the
// frozen SPEC + ADR-0019 §5 (adv-067 TASK 2): a DECLARED-but-malformed role aborts
// (any user); a NON-ROOT user with no role aborts (an empty marker silently breaks
// the drain role-guard); root + no role is a surfaced warning, not an error.
// LL-014 defect 2 at the verb boundary.
func TestBuildRoleLoudFail(t *testing.T) {
	rt := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: "created"}}

	t.Run("malformed role ⇒ hard error", func(t *testing.T) {
		t.Setenv("LOOM_SESSION_ROLE", "")
		pbPath := filepath.Join(tempProject(t), "loom.yml")
		addLine(t, pbPath, "role: bad/role")
		if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err == nil {
			t.Fatal("want a hard error for a declared-but-malformed role, got nil")
		}
	})

	t.Run("non-root user + no role ⇒ hard error", func(t *testing.T) {
		t.Setenv("LOOM_SESSION_ROLE", "")
		pbPath := filepath.Join(tempProject(t), "loom.yml")
		addLine(t, pbPath, "user: agent")
		if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err == nil {
			t.Fatal("want a hard error for a non-root user with no role (empty marker breaks the drain role-guard), got nil")
		}
	})

	t.Run("non-root user + valid role ⇒ ok, no warning", func(t *testing.T) {
		t.Setenv("LOOM_SESSION_ROLE", "")
		pbPath := filepath.Join(tempProject(t), "loom.yml")
		addLine(t, pbPath, "user: agent")
		addLine(t, pbPath, "role: agent-role")
		res, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock)
		if err != nil {
			t.Fatalf("non-root user WITH a valid role must build: %v", err)
		}
		if len(res.Warnings) != 0 {
			t.Errorf("valid role must not warn, got %v", res.Warnings)
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
