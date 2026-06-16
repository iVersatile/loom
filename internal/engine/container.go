package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// provisionSentinel is the in-container marker that records the digest of the
// tool set the last *completed* provision installed. Written only at the end of
// the provision script (after `set -eux`), so an interrupted build leaves it
// absent — letting the next build detect "exists but not converged" (ADR-0011).
const provisionSentinel = "/var/lib/loom/provisioned"

// provisionAttempts bounds how many times the whole provision exec is retried on
// a transient failure (a constrained-VM kill). The script is idempotent, so a
// retry is safe; bounded so a deterministic failure can't spin.
const provisionAttempts = 2

// ToolInstall is one resolved tool the container must provision, with its source.
type ToolInstall struct {
	Name   string
	Source string
}

// AgentInstall is one resolved agent harness the container must install, with its
// install mechanism. Phase 1: claude-code via its native installer (no Node).
type AgentInstall struct {
	Name   string
	Source string
}

// ContainerSpec describes the container build wants to converge to.
type ContainerSpec struct {
	Name       string
	Project    string // playbook project name; drives the loom.project label and the workspace mount target
	BaseImage  string
	Tools      []ToolInstall  // resolved tools to install, by source
	Agents     []AgentInstall // resolved agent harnesses to install (T8)
	Env        []string       // env var NAMES passed through at run; values from host (RULES: no secret values in code/lock/logs)
	HomeDir    string         // host staging dir seeding the container $HOME
	ProjectDir string         // absolute host path of the project root (dir holding loom.yml), bind-mounted RW (T13); empty = no mount
	Force      bool           // rebuild from scratch even if the container exists
	LogW       io.Writer      // diagnostic log sink for raw docker/provision output

	// User is the configured container runtime user (T10/ADR-0019, from playbook
	// user:); "" or "root" means root. Home is the resolved in-container $HOME
	// for User (homeForUser); "" defaults to containerHome. PR 2 populates both
	// from the merged playbook; the engine CONSUMES them (docker run --user,
	// home-sync/creds retarget, ownership chown) in T10 PR 3.
	User string
	Home string

	// Role is the container's loom-role identity, written to the ROOT-OWNED
	// /var/lib/loom/role marker (ADR-0019 PR4 §5, LL-014). It is the NON-FORGEABLE
	// role source the drain-guard will read once the human Part-2 trust swap lands
	// — replacing today's forgeable `id -un==root ⇒ loom-author` guess (ADR-0022
	// S3 caveat). Sourced from the DECLARATIVE playbook `role:` field (build.go
	// sessionRole; LOOM_SESSION_ROLE is a demoted override/test-seam) so the marker
	// reproduces from the tree on every host. The write is a CONVERGENCE DIMENSION
	// (needsRoleMarker, mirroring needsHomeSync), so a missing/stale marker
	// self-heals on the next plain build. "" ⇒ no marker and root behavior is
	// UNCHANGED; nothing reads it until human Part 2.
	Role string
}

// dockerLogged runs a docker command, tees its combined output to logw (when
// set), and returns it. This is the diagnostic trail for troubleshooting.
func dockerLogged(logw io.Writer, args ...string) ([]byte, error) {
	out, err := exec.Command("docker", args...).CombinedOutput()
	if logw != nil {
		_, _ = fmt.Fprintf(logw, "$ docker %s\n%s\n", strings.Join(args, " "), out)
	}
	return out, err
}

// ContainerRuntime is the engine's view of a container engine. Docker hides
// behind it (ADR-0008 favors Go-native clients, but Phase 1 shells the CLI),
// so the verbs stay testable without a daemon and podman can slot in later.
type ContainerRuntime interface {
	// Exists reports whether a named container is present. A nil error with
	// false means "confirmed absent"; a non-nil error means "could not tell"
	// (e.g. no daemon) — callers treat both as "needs creating", since build
	// is idempotent.
	Exists(name string) (bool, error)
	// ResolveBaseDigest returns the manifest-list (index) digest of a base
	// image (e.g. "sha256:..."), identical across architectures so the lock is
	// reproducible on arm64 and amd64.
	ResolveBaseDigest(image string) (string, error)
	// Ensure creates or converges the container, returning its info. Status is
	// "created" when newly made, "exists" when already present.
	Ensure(spec ContainerSpec) (ContainerInfo, error)
	// Teardown removes the environment to the given tier (stop|volumes|reset)
	// and reports what was removed; raw output is tee'd to logw. cleanState
	// additionally removes the agent-home volume (auth/memory/logs — the
	// opt-in SPEC-verbs --clean-state tier, orthogonal to the level).
	Teardown(name, level string, cleanState bool, logw io.Writer) (Removed, error)
	// Probe reports whether a tool binary exists INSIDE the named container and,
	// best-effort, its version. The lock's `resolved` source of truth (T5): the
	// lock pins the container, never the build host.
	Probe(container, binary string) (present bool, version string)
	// HomeDigest reads the container's home-sync sentinel digest (T7); "" when
	// absent or unreadable (including a stopped container — callers never start
	// one to ask; read-only verbs grade the staging tier instead).
	HomeDigest(name string) string
	// Running reports whether the named container's main process is up —
	// read-only verbs (plan) need it to choose between a live in-container
	// probe and the lock fallback, because Probe requires a running container
	// and plan must never Start one (LL-012).
	Running(name string) (bool, error)
	// Start brings a stopped container up; idempotent (starting a running
	// container is a no-op). It never creates (SPEC-verbs exec lifecycle).
	Start(name string) error
	// Exec runs argv inside the container with login-shell env, cwd workdir,
	// stdio attached to the calling process (transparent passthrough,
	// SPEC-verbs exec). tty allocates a terminal (SPEC-verbs shell — the one
	// engine path with a TTY option; no second code path). Returns the
	// command's exit code verbatim; a non-nil error means the transport
	// failed and no exit code exists.
	Exec(name string, argv []string, workdir, user string, tty bool) (int, error)
}

type dockerRuntime struct{}

func (dockerRuntime) Exists(name string) (bool, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return false, fmt.Errorf("docker not available: %w", err)
	}
	if err := exec.Command("docker", "container", "inspect", name).Run(); err != nil {
		return false, nil // inspect fails when the container is absent
	}
	return true, nil
}

// ResolveBaseDigest reads the multi-arch manifest-list digest without pulling
// layers. NOTE: integration-validated (docker host), not the local gate.
func (dockerRuntime) ResolveBaseDigest(image string) (string, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return "", fmt.Errorf("docker not available: %w", err)
	}
	out, err := exec.Command("docker", "buildx", "imagetools", "inspect", image, "--format", "{{.Manifest.Digest}}").Output()
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", image, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Ensure converges the per-project container: create it, seed $HOME, and install
// the resolved tool set. NOTE: the docker path here is exercised only under the
// integration tier (Work 7 / CI / Mac one-off), not the local gate.
func (dockerRuntime) Ensure(spec ContainerSpec) (ContainerInfo, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return ContainerInfo{}, fmt.Errorf("docker not available: %w", err)
	}
	want := provisionDigest(spec.Tools, spec.Agents)
	exists := exec.Command("docker", "container", "inspect", spec.Name).Run() == nil
	if exists && spec.Force {
		// --force: rebuild from scratch — remove the existing container first.
		if out, err := exec.Command("docker", "rm", "-f", spec.Name).CombinedOutput(); err != nil {
			return ContainerInfo{}, fmt.Errorf("docker rm (force): %v: %s", err, out)
		}
		exists = false
	}
	if exists {
		// Presence is not convergence (SPEC-verbs build, ADR-0011): a prior build
		// interrupted mid-provision leaves a container that exists but lacks the
		// toolchain, and a dotfile-only change leaves one whose $HOME drifted from
		// the staged config (T7). Compare both sentinels; converge whichever is
		// missing or stale ("converged").
		homeWant := homeDigest(spec.HomeDir)
		reprovision := needsReprovision(readProvisionDigest(spec.Name), want)
		homeSync := needsHomeSync(readHomeDigest(spec.Name), homeWant)
		// The role marker is a convergence dimension too (ADR-0019 PR4 §5, LL-014
		// defect 3): a container converged before the marker existed — or one whose
		// marker was lost — reads "" and must self-heal on a plain build, not only
		// on --force/create. writeRoleMarker below (idempotent) does the write.
		roleMark := needsRoleMarker(readRoleMarker(spec.Name), spec.Role)
		if !reprovision && !homeSync && !roleMark {
			return ContainerInfo{Name: spec.Name, Image: spec.BaseImage, Status: "exists"}, nil
		}
		// The non-root user must exist before shell-init targets its home and
		// before the post-sync chown (T10 PR 3); idempotent, no-op for root.
		if err := ensureUser(spec.Name, spec.User, spec.LogW); err != nil {
			return ContainerInfo{}, err
		}
		// Role marker (ADR-0019 PR4 Part 1): root-owned /var/lib/loom/role.
		// No-op for an empty role; writes nothing else's behavior changes.
		if err := writeRoleMarker(spec.Name, spec.Role, spec.LogW); err != nil {
			return ContainerInfo{}, err
		}
		// Shell-init is wired on every converge, UNCONDITIONALLY — not gated
		// on the tool set (T4): a toolless playbook's ~/.bashrc.d dotfiles
		// must be sourced too, in login and interactive shells alike.
		if err := ensureShellInit(spec.Name, spec.home(), spec.LogW); err != nil {
			return ContainerInfo{}, err
		}
		// Shared toolchain PATH (/etc/profile.d, adv-065): unconditional like
		// shell-init, so the root probe and the non-root runtime user both find
		// /usr/local/go/bin regardless of whose home the path dotfile landed in.
		if err := ensureSharedToolPath(spec.Name, spec.Tools, spec.LogW); err != nil {
			return ContainerInfo{}, err
		}
		synced := false
		if spec.HomeDir != "" {
			if out, err := dockerLogged(spec.LogW, "cp", spec.HomeDir+"/.", homeCpTarget(spec.Name, spec.home())); err != nil {
				return ContainerInfo{}, fmt.Errorf("docker cp home (reconcile): %v: %s", err, out)
			}
			writeHomeSentinel(spec.Name, homeWant, spec.LogW)
			// docker cp writes root-owned files — restore ownership to the user
			// (T10 PR 3, red-team finding 3). No-op for root.
			if err := chownHome(spec.Name, spec.User, spec.home(), spec.LogW); err != nil {
				return ContainerInfo{}, err
			}
			synced = true
		}
		// Provision (tool/agent install) stays gated on its own digest: a
		// dotfile-only change must not re-run apt/go-install (T7/T4 interplay).
		if reprovision && (len(spec.Tools) > 0 || len(spec.Agents) > 0) {
			if err := provision(spec.Name, provisionScript(spec.Tools, spec.Agents), spec.LogW); err != nil {
				return ContainerInfo{}, err
			}
		}
		return ContainerInfo{Name: spec.Name, Image: spec.BaseImage, Status: "converged", HomeSynced: synced}, nil
	}
	hostHome, _ := os.UserHomeDir()
	credsPath := filepath.Join(hostHome, ".claude", ".credentials.json")
	_, credsErr := os.Stat(credsPath)
	runArgs := createRunArgs(spec, credsPath, credsErr == nil)
	if out, err := dockerLogged(spec.LogW, runArgs...); err != nil {
		return ContainerInfo{}, fmt.Errorf("docker run: %v: %s", err, out)
	}
	// Create the non-root user (T10 PR 3, Model A) before shell-init targets its
	// home and before the home sync's chown; the container itself runs as root,
	// so this and every provision step need no `-u`. No-op for root/unset.
	if err := ensureUser(spec.Name, spec.User, spec.LogW); err != nil {
		return ContainerInfo{}, err
	}
	// Role marker (ADR-0019 PR4 Part 1): root-owned /var/lib/loom/role, written
	// at create. No-op for an empty role; nothing reads it until human Part 2.
	if err := writeRoleMarker(spec.Name, spec.Role, spec.LogW); err != nil {
		return ContainerInfo{}, err
	}
	// Unconditional shell-init (T4): wired on create regardless of the tool
	// set, so the loader exists before any home sync or provision step.
	if err := ensureShellInit(spec.Name, spec.home(), spec.LogW); err != nil {
		return ContainerInfo{}, err
	}
	// Shared toolchain PATH (/etc/profile.d, adv-065): see the reconcile branch.
	if err := ensureSharedToolPath(spec.Name, spec.Tools, spec.LogW); err != nil {
		return ContainerInfo{}, err
	}
	synced := false
	if spec.HomeDir != "" {
		if out, err := dockerLogged(spec.LogW, "cp", spec.HomeDir+"/.", homeCpTarget(spec.Name, spec.home())); err != nil {
			return ContainerInfo{}, fmt.Errorf("docker cp home: %v: %s", err, out)
		}
		writeHomeSentinel(spec.Name, homeDigest(spec.HomeDir), spec.LogW)
		// Restore ownership of the just-synced root-owned files to the user
		// (T10 PR 3, red-team finding 3). No-op for root.
		if err := chownHome(spec.Name, spec.User, spec.home(), spec.LogW); err != nil {
			return ContainerInfo{}, err
		}
		synced = true
	}
	if len(spec.Tools) > 0 || len(spec.Agents) > 0 {
		if err := provision(spec.Name, provisionScript(spec.Tools, spec.Agents), spec.LogW); err != nil {
			return ContainerInfo{}, err
		}
	}
	return ContainerInfo{Name: spec.Name, Image: spec.BaseImage, Status: "created", HomeSynced: synced}, nil
}

// Teardown removes the per-project container, and (by tier) its volumes and
// image. Phase 1 creates no per-project data volume, so the volumes tier has
// nothing extra to remove today (phase-1 review F1: it used to claim a
// `<name>-data` volume nothing creates — fiction); it stays a distinct level
// as the Phase-2 data-volume surface. The agent-home volume (auth/memory/
// logs) is removed ONLY by cleanState — wiping agent state must be an
// explicit choice, never a side effect of a tier.
// NOTE: the docker path is integration-validated (Work 7 / CI), not the local gate.
func (dockerRuntime) Teardown(name, level string, cleanState bool, logw io.Writer) (Removed, error) {
	r := Removed{Containers: []string{}, Volumes: []string{}, Images: []string{}}
	if _, err := exec.LookPath("docker"); err != nil {
		return r, fmt.Errorf("docker not available: %w", err)
	}
	_, _ = dockerLogged(logw, "stop", name)
	if _, err := dockerLogged(logw, "rm", name); err == nil {
		r.Containers = append(r.Containers, name)
	}
	if cleanState {
		vol := agentHomeVolume(name)
		if _, err := dockerLogged(logw, "volume", "rm", vol); err == nil {
			r.Volumes = append(r.Volumes, vol)
		}
	}
	if level == "reset" {
		if _, err := dockerLogged(logw, "image", "rm", "loom-"+name); err == nil {
			r.Images = append(r.Images, "loom-"+name)
		}
	}
	return r, nil
}

// shellInitScript wires the ONE shell-config loader (T4): both login
// (.profile) and interactive (.bashrc) shells source every ~/.bashrc.d/*.sh,
// so the single dotfile dir owns shell config for every shell type.
// Idempotent (grep guard — also matches the loader older builds appended to
// .bashrc, so upgrades don't duplicate it). $HOME, never /root: the same
// wiring survives T10's non-root user.
const shellInitScript = `loader='for f in "$HOME"/.bashrc.d/*.sh; do [ -r "$f" ] && . "$f"; done'
mkdir -p "$HOME/.bashrc.d"
for rc in "$HOME/.profile" "$HOME/.bashrc"; do
  grep -qs bashrc.d "$rc" || printf '%s\n' "$loader" >> "$rc"
done`

// ensureShellInit runs the shell-init wiring inside the container. Called on
// EVERY Ensure path (create and converge), never gated on tools — the T4 fix
// for "a toolless playbook copies bash/* dotfiles but never sources them".
// NOTE: integration-validated (docker host), not the local gate.
func ensureShellInit(name, home string, logw io.Writer) error {
	// HOME is pinned explicitly so the loader lands in the RESOLVED home — for a
	// non-root user the shell-init runs as root (provisioning stays root, Model
	// A) but must target /home/<user>, not /root. The post-sync chown gives the
	// user ownership of what this writes.
	if out, err := dockerLogged(logw, "exec", "-e", "HOME="+home, name, "sh", "-c", shellInitScript); err != nil {
		return fmt.Errorf("shell-init: %v: %s", err, out)
	}
	return nil
}

// ensureUser creates the configured non-root user inside the container (T10 PR
// 3). No-op for root/unset. Runs as root (the container default under Model A),
// so it must precede any step that needs the user to exist (shell-init targeting
// the user's home, the ownership chown, and every later `exec -u <user>`).
func ensureUser(name, user string, logw io.Writer) error {
	script := provisionUserScript(user)
	if script == "" {
		return nil
	}
	if out, err := dockerLogged(logw, "exec", name, "sh", "-c", script); err != nil {
		return fmt.Errorf("useradd %s: %v: %s", user, err, out)
	}
	return nil
}

// chownHome restores ownership of the synced home tree to the configured user
// (T10 PR 3, red-team finding 3). No-op for root/unset. Runs as root.
func chownHome(name, user, home string, logw io.Writer) error {
	script := chownHomeScript(user, home)
	if script == "" {
		return nil
	}
	if out, err := dockerLogged(logw, "exec", name, "sh", "-c", script); err != nil {
		return fmt.Errorf("chown home %s: %v: %s", home, err, out)
	}
	return nil
}

// validRole accepts a marker-safe role identity: non-empty and [A-Za-z0-9_-]
// only, so the value can NEVER inject into the marker-write shell. Anything else
// (empty, whitespace, shell metachars) ⇒ no marker (fail-safe, behavior unchanged).
func validRole(role string) bool {
	if role == "" {
		return false
	}
	for _, r := range role {
		switch {
		case r == '-' || r == '_':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// roleMarker is the in-container marker recording the container's loom-role
// identity (ADR-0019 PR4 §5). Root-owned single line; the non-forgeable role
// source the drain-guard reads after the human Part-2 swap. It joins the
// convergence digests (needsRoleMarker), the same presence-is-not-convergence
// pattern as homeSentinel/provisionSentinel.
const roleMarker = "/var/lib/loom/role"

// roleMarkerScript writes the role to the ROOT-OWNED /var/lib/loom/role marker
// (one line) — ADR-0019 PR4 §5. Root-owned + 0644 so a future non-root agent
// can READ but never FORGE it (the non-forgeable role source the drain-guard
// reads after the human Part-2 swap, replacing the `id -un==root` guess). Empty
// for an invalid/unset role (no marker written; root behavior unchanged). The
// role is charset-validated, so embedding it in the script cannot inject.
func roleMarkerScript(role string) string {
	if !validRole(role) {
		return ""
	}
	return fmt.Sprintf("set -e\nmkdir -p /var/lib/loom\nprintf '%%s\\n' '%s' > %s\nchown root:root %s\nchmod 0644 %s\n", role, roleMarker, roleMarker, roleMarker)
}

// readRoleMarker reads the in-container role marker; "" when absent/unreadable
// (a container provisioned before PR4, or one whose marker was lost). Mirrors
// readHomeDigest. NOTE: integration-validated (docker host), not the local gate.
func readRoleMarker(name string) string {
	out, err := exec.Command("docker", "exec", name, "cat", roleMarker).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// needsRoleMarker mirrors needsHomeSync for the role marker: (re)write when a
// valid role is declared and the container's marker is missing or stale (LL-014
// defect 3 — a missing marker reads "" and self-heals on the next plain build).
// An empty/invalid want ⇒ false: no marker, no rewrite (root compatibility).
func needsRoleMarker(have, want string) bool {
	return validRole(want) && have != want
}

// roleMarkerPlan reports how the marker behaves for a declared role — LL-014
// defect 2: an empty role must no longer be a silent no-op. The rule keys ONLY on
// the role, NOT the user (adv-063): a non-root user: is general container
// hardening; a non-root container that is not a loom drain-seat has no role and
// must not be forced to invent one (a missing marker is a safe no-op — the
// drain-guard fails closed, and the doctor host:role-marker check #144 already
// fires at the non-root moment). So:
//   - no role declared ⇒ visible WARNING, no marker (fallback intact);
//   - a role declared but not marker-safe ⇒ HARD ERROR (a typo to fix, not a
//     silent skip — same charset the write path enforces);
//   - a valid role ⇒ silent (the marker is written).
func roleMarkerPlan(role string) (warning string, err error) {
	if role == "" {
		return fmt.Sprintf("no role: declared — %s not written; drain-guard keeps the id -un==root fallback", roleMarker), nil
	}
	if !validRole(role) {
		return "", fmt.Errorf("role %q is not marker-safe ([A-Za-z0-9_-] only) — fix the typo or clear role: (ADR-0019 PR4 §5)", role)
	}
	return "", nil
}

// writeRoleMarker writes the /var/lib/loom/role marker inside the container (ADR-
// 0019 PR4 Part 1). No-op for an empty/invalid role. Runs as root (the container
// default under Model A), like ensureUser — the marker must be root-owned.
func writeRoleMarker(name, role string, logw io.Writer) error {
	script := roleMarkerScript(role)
	if script == "" {
		return nil
	}
	if out, err := dockerLogged(logw, "exec", name, "sh", "-c", script); err != nil {
		return fmt.Errorf("role marker %s: %v: %s", role, err, out)
	}
	return nil
}

// sharedToolPath is the SHARED, container-wide PATH drop-in (adv-065). The Go
// toolchain installs to /usr/local/go (shared), but its bin dir reaches a user's
// PATH only via the home-synced ~/.bashrc.d/path.go.sh — which lands in the
// CONFIGURED user's home (T10). The version probe and provision run as ROOT,
// whose /root/.profile has no such loader on a non-root container, so `go` (and
// every go-built tool) fell off root's probe PATH → an all-empty loom.lock.
// /etc/profile.d is sourced by EVERY login shell (`sh -lc`) regardless of $HOME
// or user, so it puts the toolchain on PATH for the root probe AND the non-root
// runtime user uniformly — the shared-tool analogue of the home-scoped dotfile.
const sharedToolPath = "/etc/profile.d/loom-path.sh"

// hasGoToolchain reports whether the resolved tool set installs Go (tarball or a
// go-built tool) — the only tool whose bin dir (/usr/local/go/bin) is not already
// on the default login PATH and so needs the shared drop-in.
func hasGoToolchain(tools []ToolInstall) bool {
	for _, t := range tools {
		if t.Source == "go-tarball" || t.Source == "go-install" {
			return true
		}
	}
	return false
}

// sharedToolPathScript writes the /etc/profile.d drop-in adding /usr/local/go/bin
// to every login shell's PATH. Root-owned + 0644 (a non-root user reads but never
// forges it). Empty when no Go toolchain is installed — apt tools live in /usr/bin
// and the relocated harness/go-built tools in /usr/local/bin, both already on the
// default login PATH, so no drop-in is needed.
func sharedToolPathScript(tools []ToolInstall) string {
	if !hasGoToolchain(tools) {
		return ""
	}
	return fmt.Sprintf("set -e\nmkdir -p %s\nprintf '%%s\\n' 'export PATH=\"$PATH:/usr/local/go/bin\"' > %s\nchown root:root %s\nchmod 0644 %s\n",
		filepath.Dir(sharedToolPath), sharedToolPath, sharedToolPath, sharedToolPath)
}

// ensureSharedToolPath writes the shared PATH drop-in inside the container, run
// unconditionally on every converge (like ensureShellInit) so a container missing
// it self-heals on a plain build. Runs as root (the container default, Model A) —
// /etc/profile.d must be root-owned. No-op when no Go toolchain is declared.
func ensureSharedToolPath(name string, tools []ToolInstall, logw io.Writer) error {
	script := sharedToolPathScript(tools)
	if script == "" {
		return nil
	}
	if out, err := dockerLogged(logw, "exec", name, "sh", "-c", script); err != nil {
		return fmt.Errorf("shared tool PATH: %v: %s", err, out)
	}
	return nil
}

// provision copies the script into the container as a file and execs it (more
// robust than piping via `sh -s` on stdin). With `set -x` in the script, the
// combined output ends at the exact command that failed.
func provision(name, script string, logw io.Writer) error {
	tmp, err := os.CreateTemp("", "loom-provision-*.sh")
	if err != nil {
		return fmt.Errorf("provision tmp: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.WriteString(script); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("provision write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("provision close: %w", err)
	}
	if out, err := dockerLogged(logw, "cp", tmp.Name(), name+":/tmp/loom-provision.sh"); err != nil {
		return fmt.Errorf("cp provision: %v: %s", err, out)
	}
	// The script is idempotent and retries its own flaky steps internally; this
	// outer loop covers a transient kill of the whole exec (e.g. a SIGKILL/137 on
	// a memory-pressured VM) by re-running the script fresh before giving up. A
	// deterministic failure still fails — bounded so it can't spin (ADR-0011).
	var lastErr error
	for attempt := 1; attempt <= provisionAttempts; attempt++ {
		out, err := dockerLogged(logw, "exec", name, "sh", "/tmp/loom-provision.sh")
		if err == nil {
			return nil
		}
		lastErr = fmt.Errorf("provision (attempt %d/%d): %v: %s", attempt, provisionAttempts, err, out)
		// If the container itself exited (e.g. an OOM-kill of the whole cgroup, not
		// just the exec'd process), retrying only yields a misleading "is not
		// running" error — stop and surface THIS attempt's real trace.
		if !containerRunning(name) {
			return fmt.Errorf("%w [container exited during provisioning]", lastErr)
		}
		if logw != nil {
			_, _ = fmt.Fprintf(logw, "provision attempt %d/%d failed (%v) — retrying\n", attempt, provisionAttempts, err)
		}
	}
	return lastErr
}

// Running implements the interface query via containerRunning. NOTE:
// integration-validated (docker host), not the local gate.
func (dockerRuntime) Running(name string) (bool, error) {
	return containerRunning(name), nil
}

// containerRunning reports whether the named container's main process is still up.
func containerRunning(name string) bool {
	out, err := exec.Command("docker", "container", "inspect", "-f", "{{.State.Running}}", name).Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// toolsetDigest is a stable fingerprint of the declared tool set, written into
// the container as the provision sentinel and compared on re-build. Order-stable
// (sorted) so a reordered playbook does not look like drift. Empty set ⇒ "".
// provisionDigest is the sentinel value: a stable hash of the exact tool AND agent
// set the last completed provision installed (T8 folds agents in, so adding or
// removing an agent re-provisions, not just a tool change). Order-stable; empty
// when there is nothing to provision.
func provisionDigest(tools []ToolInstall, agents []AgentInstall) string {
	if len(tools) == 0 && len(agents) == 0 {
		return ""
	}
	lines := make([]string, 0, len(tools)+len(agents))
	for _, t := range tools {
		lines = append(lines, "tool|"+t.Name+"|"+t.Source)
	}
	for _, a := range agents {
		lines = append(lines, "agent|"+a.Name+"|"+a.Source)
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// needsReprovision decides whether an existing container must be re-provisioned:
// when the wanted tool set is non-empty and its digest differs from the sentinel
// the container carries (have=="" means no completed provision — interrupted).
func needsReprovision(have, want string) bool {
	return want != "" && have != want
}

// readProvisionDigest reads the in-container provision sentinel; "" if the
// container has no fully-completed provision (file absent / unreadable).
func readProvisionDigest(name string) string {
	out, err := exec.Command("docker", "exec", name, "cat", provisionSentinel).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// homeSentinel is the in-container marker recording the digest of the staged
// $HOME content the last completed home sync copied in (T7) — the same
// presence-is-not-convergence pattern as the provision sentinel (ADR-0011),
// for the config surface ADR-0015 materializes. Written only after a
// successful `docker cp`, so an interrupted sync re-syncs on the next build.
const homeSentinel = "/var/lib/loom/home"

// homeDigest fingerprints the host staging dir that seeds the container $HOME:
// a stable hash over each regular file's relative path, permission bits, and
// content (mode included so a chmod-only change re-syncs). "" when the dir is
// absent or holds no files — nothing to sync.
func homeDigest(dir string) string {
	var lines []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil // unreadable entries are skipped, not fatal: worst case is a re-sync
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		mode := ""
		if info, ierr := d.Info(); ierr == nil {
			mode = info.Mode().Perm().String()
		}
		rel, _ := filepath.Rel(dir, p)
		sum := sha256.Sum256(data)
		lines = append(lines, filepath.ToSlash(rel)+"|"+mode+"|"+hex.EncodeToString(sum[:]))
		return nil
	})
	if len(lines) == 0 {
		return ""
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// needsHomeSync mirrors needsReprovision for the $HOME surface: sync when
// there is staged content and its digest differs from the container's home
// sentinel (a missing sentinel — pre-T7 container or interrupted sync — reads
// as "" and re-syncs).
func needsHomeSync(have, want string) bool {
	return want != "" && have != want
}

// HomeDigest exposes the home sentinel to read-only verbs (doctor/plan grade
// home wiring against it — C1/F2, phase-1 review).
func (dockerRuntime) HomeDigest(name string) string { return readHomeDigest(name) }

// readHomeDigest reads the in-container home sentinel; "" when absent.
func readHomeDigest(name string) string {
	out, err := exec.Command("docker", "exec", name, "cat", homeSentinel).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// writeHomeSentinel records the synced home digest in the container.
// Best-effort: a failed write leaves the sentinel absent/stale, which re-syncs
// on the next build — the safe direction.
func writeHomeSentinel(name, digest string, logw io.Writer) {
	_, _ = dockerLogged(logw, "exec", name, "sh", "-c",
		"mkdir -p /var/lib/loom && printf %s "+digest+" > "+homeSentinel)
}

// Probe asks the container for a binary's version via a LOGIN shell, so the
// provision PATH (.profile: /usr/local/go/bin, ~/.local/bin) applies — the same
// PATH an interactive user gets. Tools disagree on the flag (git --version vs
// go version); try both. NOTE: integration-validated, not the local gate.
func (dockerRuntime) Probe(container, binary string) (bool, string) {
	for _, arg := range []string{"--version", "version"} {
		out, err := exec.Command("docker", "exec", container, "sh", "-lc", binary+" "+arg).Output()
		if err == nil {
			return true, firstLine(string(out))
		}
	}
	return false, ""
}

// Start brings a stopped container up. `docker start` on a running container
// is a no-op, so this is idempotent. NOTE: integration-validated, not the
// local gate.
func (dockerRuntime) Start(name string) error {
	if out, err := exec.Command("docker", "start", name).CombinedOutput(); err != nil {
		return fmt.Errorf("docker start: %v: %s", err, out)
	}
	return nil
}

// Exec runs argv inside the container per the SPEC-verbs exec contract:
// login-shell env (`sh -lc`, the Probe lesson — provisioned PATH applies),
// cwd = workdir, AS the configured user (Model A, ADR-0019 amended: the
// container runs as root but entry verbs run as `user` via `docker exec -u
// <user>` BY NAME — collision-proof; root/unset = the container default, no
// flag), stdio passed through untouched. tty adds `-t` (SPEC-verbs shell: a TTY
// plus a login shell — same path, one option). The argv is shell-quoted and
// exec'd so the command — not a wrapper shell — receives signals and owns the
// exit code. NOTE: integration-validated, not the local gate.
func (dockerRuntime) Exec(name string, argv []string, workdir, user string, tty bool) (int, error) {
	quoted := make([]string, len(argv))
	for i, a := range argv {
		quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	dockerArgs := []string{"exec", "-i"}
	if tty {
		dockerArgs = append(dockerArgs, "-t")
	}
	dockerArgs = append(dockerArgs, execUserArgs(user)...)
	dockerArgs = append(dockerArgs, "-w", workdir, name,
		"sh", "-lc", "exec "+strings.Join(quoted, " "))
	c := exec.Command("docker", dockerArgs...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), nil // the command's own exit, propagated verbatim
		}
		return -1, err // transport failure: docker never ran the command
	}
	return 0, nil
}

func defaultRuntime() ContainerRuntime { return dockerRuntime{} }

// containerName derives the deterministic per-project container name (ADR-0001).
// `<project>-dev` (T11): the loom-managed marker is the loom.managed/loom.project
// labels, not a name prefix — a `loom-` prefix doubled for the loom project itself
// (`loom-loom-dev`) and tied identity to display. Discover loom's containers with
// `docker ps --filter label=loom.managed=true`.
func containerName(project string) string {
	return project + "-dev"
}

// agentHomeVolume names the durable agent-home volume (T14): mounted at
// ~/.claude so in-container credentials (and the agent home) survive
// `build --force`/`teardown` (`docker rm` keeps named volumes). Removed ONLY
// by the opt-in `--clean-state` flag (SPEC-verbs teardown), never the
// volumes/reset tiers — wiping agent auth must be an explicit choice.
func agentHomeVolume(container string) string {
	return container + "-claude"
}

// containerWorkspace is the fixed in-container mount point for the project repo
// (T13): /workspace/<project>, matching the devcontainer convention (ADR-0003).
func containerWorkspace(project string) string {
	return "/workspace/" + project
}

// createRunArgs assembles the `docker run` arg list for first creation. All of
// this is create-time-only state — docker cannot add labels/-e/-v to a live
// container, so changing any of it requires `--force` (a new container):
//   - labels: the loom-managed marker + project identity (T11).
//   - env passthrough (-e NAME, no value): docker forwards the value from loom's
//     own environment; values never enter code/lock/image/logs (RULES).
//   - agent-home volume at ~/.claude (T14): persists in-container credentials
//     across rebuilds; only when an agent needing auth is declared.
//   - project bind-mount (T13): the repo, RW, host↔container live edits.
//   - creds mount (-v ...:ro): reuse the host's EXISTING Claude credentials file
//     when one exists (Linux hosts; macOS keeps creds in the Keychain so this is
//     a no-op there). Single-file + read-only, nested inside the agent-home
//     volume, so it never shadows the materialised settings.json/statusline.sh.
func createRunArgs(spec ContainerSpec, hostCredsPath string, credsPresent bool) []string {
	args := []string{"run", "-d", "--name", spec.Name,
		"--label", "loom.managed=true", "--label", "loom.project=" + spec.Project}
	args = append(args, envArgs(spec.Env)...)
	if hasAgent(spec.Agents, "claude-code") {
		args = append(args, "-v", agentHomeVolume(spec.Name)+":"+spec.home()+"/.claude")
	}
	if spec.ProjectDir != "" {
		args = append(args, "-v", spec.ProjectDir+":"+containerWorkspace(spec.Project))
	}
	// Model A (ADR-0019 amended): the container runs as root — NO `--user` on
	// `docker run`. The configured user is created at provision and entry verbs
	// run as it (execUserArgs). This keeps provisioning root with no `-u` juggle.
	args = append(args, credsMount(hostCredsPath, credsPresent, spec.Agents, spec.home())...)
	return append(args, spec.BaseImage, "sleep", "infinity")
}

// provisionScript builds a POSIX-sh script that installs the resolved tools into
// the container, grouped by source: apt for system packages, the official Go
// tarball for the toolchain, `go install` for Go-distributed tools, the uv
// installer for uv. Run on a minimal debian:bookworm-slim, so it installs its own
// prerequisites (ca-certificates, curl) first.
func provisionScript(tools []ToolInstall, agents []AgentInstall) string {
	var apt, goInstall []string
	var needGo, needUv bool
	for _, t := range tools {
		switch t.Source {
		case "apt":
			apt = append(apt, t.Name)
		case "go-install":
			if m := goModule(t.Name); m != "" {
				goInstall = append(goInstall, m)
			}
			needGo = true
		case "go-tarball":
			needGo = true
		case "uv-installer":
			needUv = true
		}
	}
	sort.Strings(apt)
	sort.Strings(goInstall)

	var b strings.Builder
	// set -x traces each command so a failing provision pinpoints the exact line.
	b.WriteString("#!/bin/sh\nset -eux\nexport DEBIAN_FRONTEND=noninteractive\n")
	// retry wraps flaky/network/memory-heavy steps: re-attempt with linear backoff
	// before aborting, so a transient blip (network, a kill on a constrained VM)
	// doesn't fail the whole provision. The full trace stays in the diagnostic log.
	b.WriteString("retry() { n=0; until \"$@\"; do n=$((n+1)); [ \"$n\" -ge 3 ] && return 1; echo \"retry $n: $*\"; sleep $((n*3)); done; }\n")
	// Acquire::Languages=none trims the apt package-list cache build (skips
	// translation indices) — the step that OOMs/kills on small Docker VMs; Retries
	// rides out network blips.
	b.WriteString("retry apt-get -o Acquire::Languages=none -o Acquire::Retries=3 update\n")
	pkgs := append([]string{"ca-certificates", "curl", "git", "tar"}, filterOut(apt, "git")...)
	b.WriteString("retry apt-get install -y --no-install-recommends " + strings.Join(pkgs, " ") + "\n")
	b.WriteString("apt-get clean && rm -rf /var/lib/apt/lists/*\n")

	if needGo {
		b.WriteString(`ARCH="$(dpkg --print-architecture)"
GOVER="$(retry curl -fsSL 'https://go.dev/VERSION?m=text' | head -1)"
retry curl -fsSL "https://go.dev/dl/${GOVER}.linux-${ARCH}.tar.gz" -o /tmp/go.tgz
rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tgz && rm -f /tmp/go.tgz
export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"
# go install lands binaries in a SHARED, world-exec dir (/usr/local/bin, like the
# Go toolchain's own /usr/local/go) so the NON-ROOT runtime user can run them too
# — NOT root's $HOME/go/bin, which a non-root user cannot reach (/root is 0700;
# T10/adv-065). Only the install target moves; the GOPATH build cache stays in
# root's home (root-only, not needed at runtime).
export GOBIN=/usr/local/bin
# Keep memory-heavy installs (gopls) alive on a small VM / ~7GB CI box: serialize
# the build (-p=1, GOMAXPROCS=1) and cap the Go heap (GOMEMLIMIT) so the GC stays
# under the ceiling instead of OOMing. Trades build speed for survival.
export GOFLAGS=-p=1
export GOMAXPROCS=1
export GOMEMLIMIT=1GiB
`)
	}
	for _, m := range goInstall {
		b.WriteString("retry go install " + m + "\n")
	}
	if needUv {
		// UV_INSTALL_DIR puts uv in the SHARED /usr/local/bin (reachable by the
		// non-root runtime user), not the provisioning root's ~/.local/bin (adv-065).
		b.WriteString("retry sh -c 'curl -fsSL https://astral.sh/uv/install.sh | UV_INSTALL_DIR=/usr/local/bin sh'\n")
	}
	// Install declared agent harnesses (T8). claude-code's native installer needs
	// no Node; it lands at root's ~/.local/bin (provision runs as root, Model A),
	// then agentInstallCmd relocates the BINARY to the SHARED /usr/local/bin so the
	// non-root runtime user can run it (adv-065). Persistent PATH for /usr/local/bin
	// is the default login PATH; trust/hooks materialization is unchanged (dotfiles).
	for _, a := range agents {
		b.WriteString(agentInstallCmd(a))
	}

	// Shell-init wiring (the ~/.bashrc.d loader) is NOT here: it is owned by
	// ensureShellInit, which runs unconditionally on every Ensure — a toolless
	// playbook gets its dotfiles sourced too (T4).
	// Provision sentinel, written LAST: marks the container converged to this exact
	// tool set so a re-build can tell "fully provisioned" from "interrupted"
	// (ADR-0011). set -e above guarantees we never reach here on a failed install.
	fmt.Fprintf(&b, "mkdir -p %s\n", filepath.Dir(provisionSentinel))
	fmt.Fprintf(&b, "printf '%%s' '%s' > %s\n", provisionDigest(tools, agents), provisionSentinel)
	return b.String()
}

// agentInstallCmd emits the provision step that installs one agent harness, then
// relocates its BINARY to the SHARED /usr/local/bin. The native installer lands
// the binary in root's ~/.local/bin (provision runs as root, Model A) — a path a
// non-root runtime user cannot reach (/root is 0700), which left a non-root seat
// with `claude: command not found` and an all-empty loom.lock (adv-065). The
// claude binary is a self-contained ELF, so `cp -L` of the ~/.local/bin/claude
// symlink target yields a working, world-exec binary on the default login PATH
// for every user; only the binary moves — trust/hooks stay home-synced dotfiles.
// Unknown agents (no installer yet) emit nothing — still recorded in the provision
// digest so the intent is tracked.
func agentInstallCmd(a AgentInstall) string {
	switch a.Name {
	case "claude-code":
		return "retry sh -c 'curl -fsSL https://claude.ai/install.sh | bash'\n" +
			"cp -L \"$HOME/.local/bin/claude\" /usr/local/bin/claude && chmod 0755 /usr/local/bin/claude\n"
	default:
		return ""
	}
}

// envArgs turns declared env var NAMES into `docker run -e NAME` passthrough args.
// Only the name is passed, so docker forwards the value from loom's own
// environment at run time — the value never enters loom's code, lock, image, or
// logs (RULES: no secrets in code/logs). Docker drops names that are unset, so
// the arg list stays deterministic regardless of which secrets are present.
func envArgs(env []string) []string {
	args := make([]string, 0, len(env)*2)
	for _, name := range env {
		if name == "" {
			continue
		}
		args = append(args, "-e", name)
	}
	return args
}

// containerHome is the in-container $HOME loom materialises into. Hardcoded to
// root's home for Phase 1; T10 will parameterise it for a non-root user.
// SINGLE OWNER (T10 PR 1): every in-container home path — cp targets, volume
// mounts, creds mounts — derives from this constant; a literal "/root"
// anywhere else is a bug (two were found bypassing it, the cp targets).
const containerHome = "/root"

// homeForUser resolves the in-container $HOME for a configured user: value
// (T10, ADR-0019). Unset or "root" → containerHome ("/root"), so every existing
// playbook (no user:) keeps its exact Phase-1 home; any other user → /home/<user>.
// This is the single point ContainerSpec.Home is populated from (build.go). The
// engine consumes Home — retargeting the home-sync cp and creds mount, plus the
// post-sync ownership chown — in T10 PR 3; PR 2 only lays the resolved value.
func homeForUser(user string) string {
	if user == "" || user == "root" {
		return containerHome
	}
	return "/home/" + user
}

// home is the resolved in-container $HOME for this spec; the single owner every
// home-path helper routes through (T10). Empty Home (a spec built before T10, or
// the default-root case) falls back to containerHome, so existing callers are
// byte-identical.
func (s ContainerSpec) home() string {
	if s.Home != "" {
		return s.Home
	}
	return containerHome
}

// execUserArgs returns the `docker exec` user flag for entry verbs under Model A
// (ADR-0019, amended): the container runs as root (PID 1) but exec/shell run AS
// the configured user, BY NAME — collision-proof (the name resolves to whatever
// uid useradd assigned). Root/unset => no flag (unchanged).
func execUserArgs(user string) []string {
	if user == "" || user == "root" {
		return nil
	}
	return []string{"-u", user}
}

// provisionUserScript emits the provision step that creates the non-root user
// (T10 PR 3, ADR-0019 decision 3). Empty for root/unset. IDEMPOTENT and
// COLLISION-TOLERANT (red-team finding 4): the name already existing is a reuse
// (re-provision is a no-op); uid 1000 held by a DIFFERENT account means we take
// the next free uid and LOG it — never hard-fail, never adopt a foreign account.
// uid 1000 is preferred-when-free, never a hard pin; doctor verifies by NAME.
// Runs as root (the container default under Model A), before any entry verb
// needs the name.
func provisionUserScript(user string) string {
	if user == "" || user == "root" {
		return ""
	}
	// id -u <name> succeeds iff the named account exists (reuse path).
	// id -u 1000 succeeds iff SOME account already holds uid 1000 (collision).
	return fmt.Sprintf(`if id -u %[1]s >/dev/null 2>&1; then :
elif id -u 1000 >/dev/null 2>&1; then useradd -m %[1]s && echo "loom: uid 1000 taken; created %[1]s with an auto-assigned uid" >&2
else useradd -m -u 1000 %[1]s
fi
`, user)
}

// chownHomeScript emits the post-home-sync ownership fix (T10 PR 3, red-team
// finding 3): `docker cp` writes root-owned files, unreadable/unwritable by the
// non-root user. Empty for root/unset. It chowns the resolved home tree to the
// user but PRUNES the read-only `.credentials.json` bind — a blanket `chown -R`
// walks into that ro mount and ERRORS (the finding's core safety requirement).
// Broader than the materialized FILE set on purpose: the agent-home volume dir
// and `.claude/` must be user-owned for the agent to write session state, and a
// fresh `useradd -m` skeleton is already user-owned, so the recursive chown
// (minus the ro bind) is the correct, idempotent scope. `docker cp -a` is NOT a
// substitute — it preserves the host numeric uid, the wrong owner.
func chownHomeScript(user, home string) string {
	if user == "" || user == "root" {
		return ""
	}
	creds := home + "/.claude/.credentials.json"
	return fmt.Sprintf("find %s -path %s -prune -o -exec chown %s:%s {} +\n", home, creds, user, user)
}

// homeCpTarget is the docker-cp destination for the staged $HOME tree. Routed
// through containerHome so T10's parameterisation changes exactly one value
// (ADR-0016 consequence: "T10 retargets entry by changing one configured-user
// value" — that only holds if nothing bypasses the constant).
func homeCpTarget(name, home string) string {
	return name + ":" + home + "/"
}

// credsMount returns a read-only single-file bind of the host's EXISTING Claude
// credentials into the container, so the in-container agent reuses the same
// subscription auth the host already has (no token generation, no browser flow).
// Gated on claude-code being installed and the host creds file being present —
// mounting a missing path would make docker create a directory. Single-file so it
// does not shadow the materialised settings.json/statusline.sh.
func credsMount(hostCredsPath string, present bool, agents []AgentInstall, home string) []string {
	if !present || !hasAgent(agents, "claude-code") {
		return nil
	}
	return []string{"-v", hostCredsPath + ":" + home + "/.claude/.credentials.json:ro"}
}

func hasAgent(agents []AgentInstall, name string) bool {
	for _, a := range agents {
		if a.Name == name {
			return true
		}
	}
	return false
}

// goModule maps a go-install tool name to its module path.
func goModule(tool string) string {
	switch tool {
	case "gopls":
		return "golang.org/x/tools/gopls@latest"
	case "gitleaks":
		// The v8 module's go.mod still declares the legacy zricethezav path; the
		// github.com/gitleaks/gitleaks/v8 path fails `go install` (path conflict).
		return "github.com/zricethezav/gitleaks/v8@latest"
	case "golangci-lint":
		return "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
	default:
		return ""
	}
}

func filterOut(items []string, drop string) []string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		if s != drop {
			out = append(out, s)
		}
	}
	return out
}
