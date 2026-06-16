package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/iVersatile/loom/internal/playbook"
)

// Non-root verification checks (T10 PR4 Part 3, adv-064). Part 3 made the
// loom-dev seat run entry verbs as a non-root user; its proof was ad-hoc `loom
// exec` spot-checks (not in the tree — LL-014). These two checks mechanize that
// proof as durable, reproducible `doctor` claims: "doctor mechanizes what the
// reviewer hand-checks once". Both grade the LIVE container (read-only docker
// probes, integration-validated); the decision logic is the pure functions
// below (gate-tested). Graded ONLY for a non-root playbook user — under root the
// marker is root-writable by definition and the workspace invariant is N/A.

// parseOctalMode parses a `stat -c %a` octal mode string ("644", "0755") into
// its permission bits. Errors on non-octal input (fail-closed at the call site).
func parseOctalMode(mode string) (int, error) {
	v, err := strconv.ParseInt(strings.TrimSpace(mode), 8, 32)
	if err != nil {
		return 0, fmt.Errorf("unparseable mode %q: %w", mode, err)
	}
	return int(v), nil
}

// roleMarkerPermsOK decides container:role-marker-perms: the marker is
// NON-FORGEABLE by a non-root runtime user iff it is owned by root AND carries no
// group/other write bit (mode & 022 == 0). Either a foreign owner or any
// group/other write bit means a non-root user could rewrite the role → the
// drain-guard's trust source is forgeable. Fail-closed on unparseable input.
func roleMarkerPermsOK(owner, mode string) (bool, string) {
	bits, err := parseOctalMode(mode)
	if err != nil {
		return false, fmt.Sprintf("cannot read marker mode: %v", err)
	}
	switch {
	case owner != "root":
		return false, fmt.Sprintf("marker owner %q (mode %s) — must be root-owned so a non-root runtime user cannot forge the role", owner, mode)
	case bits&0o022 != 0:
		return false, fmt.Sprintf("marker mode %s is group/other-writable — a non-root runtime user could forge the role (want root-owned, 0644)", mode)
	default:
		return true, fmt.Sprintf("root-owned, mode %s (no group/other write) — non-forgeable by the runtime user", mode)
	}
}

// workspaceWritableOK decides container:workspace-writable: the runtime user can
// write the workspace bind-mount — the loop-survival invariant (a drain that
// cannot flip an inbox status under .scratch/ silently stops delivering). OK iff
// the user's uid owns the workspace, or the dir is group/other-writable. The
// failing case is the uid-collision one: useradd wanted uid 1000 to match the
// bind-mount owner, but 1000 was taken, so the user got another uid and cannot
// write the host-owned tree. Fail-closed on unparseable input.
func workspaceWritableOK(userUID, ownerUID, mode string) (bool, string) {
	if strings.TrimSpace(userUID) == "" {
		return false, "runtime user has no uid in the container (does the user exist?)"
	}
	if strings.TrimSpace(userUID) == strings.TrimSpace(ownerUID) {
		return true, fmt.Sprintf("runtime uid %s owns the workspace — writable", userUID)
	}
	bits, err := parseOctalMode(mode)
	if err != nil {
		return false, fmt.Sprintf("cannot read workspace mode: %v", err)
	}
	if bits&0o022 != 0 {
		return true, fmt.Sprintf("workspace owned by uid %s (mode %s, group/other-writable) — runtime uid %s can write", ownerUID, mode, userUID)
	}
	return false, fmt.Sprintf("runtime uid %s != workspace owner uid %s and mode %s is not group/other-writable — uid collision: the drain cannot flip inbox statuses", userUID, ownerUID, mode)
}

// nonRootContainerChecks mechanizes the Part-3 non-root proof against a LIVE
// container (adv-064). Read-only: it `stat`s the marker and the workspace and
// reads the runtime user's uid — it never writes a probe file or touches the
// marker (doctor never mutates). Returns no checks for a root/unset user (the
// invariants are N/A). The caller gates on a running container (docker exec
// needs one; doctor never Starts a container to ask).
func nonRootContainerChecks(rt ContainerRuntime, cname string, pb playbook.Playbook) []Check {
	if pb.User == "" || pb.User == "root" {
		return nil
	}
	var checks []Check

	// 1. container:role-marker-perms — the marker is non-forgeable by the user.
	if owner, mode, _, err := rt.StatPath(cname, roleMarker); err != nil {
		checks = append(checks, Check{Name: "container:role-marker-perms", OK: false,
			Detail: fmt.Sprintf("%s unreadable (%v) — marker absent on a non-root container means the drain-guard cannot resolve the role", roleMarker, err)})
	} else {
		ok, detail := roleMarkerPermsOK(owner, mode)
		checks = append(checks, Check{Name: "container:role-marker-perms", OK: ok, Detail: detail})
	}

	// 2. container:workspace-writable — the runtime user can write the workspace.
	ws := containerWorkspace(pb.Name)
	userUID, uerr := rt.UserUID(cname, pb.User)
	_, mode, ownerUID, serr := rt.StatPath(cname, ws)
	switch {
	case uerr != nil:
		checks = append(checks, Check{Name: "container:workspace-writable", OK: false,
			Detail: fmt.Sprintf("cannot read uid of user %q: %v", pb.User, uerr)})
	case serr != nil:
		checks = append(checks, Check{Name: "container:workspace-writable", OK: false,
			Detail: fmt.Sprintf("cannot stat workspace %s: %v", ws, serr)})
	default:
		ok, detail := workspaceWritableOK(userUID, ownerUID, mode)
		checks = append(checks, Check{Name: "container:workspace-writable", OK: ok, Detail: detail})
	}

	return checks
}
