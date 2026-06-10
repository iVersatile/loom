package engine

import (
	"crypto/sha256"
	"encoding/hex"
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
	// and reports what was removed; raw output is tee'd to logw.
	Teardown(name, level string, logw io.Writer) (Removed, error)
	// Probe reports whether a tool binary exists INSIDE the named container and,
	// best-effort, its version. The lock's `resolved` source of truth (T5): the
	// lock pins the container, never the build host.
	Probe(container, binary string) (present bool, version string)
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
		if !reprovision && !homeSync {
			return ContainerInfo{Name: spec.Name, Image: spec.BaseImage, Status: "exists"}, nil
		}
		if spec.HomeDir != "" {
			if out, err := dockerLogged(spec.LogW, "cp", spec.HomeDir+"/.", spec.Name+":/root/"); err != nil {
				return ContainerInfo{}, fmt.Errorf("docker cp home (reconcile): %v: %s", err, out)
			}
			writeHomeSentinel(spec.Name, homeWant, spec.LogW)
		}
		// Provision (tool/agent install) stays gated on its own digest: a
		// dotfile-only change must not re-run apt/go-install (T7/T4 interplay).
		if reprovision && (len(spec.Tools) > 0 || len(spec.Agents) > 0) {
			if err := provision(spec.Name, provisionScript(spec.Tools, spec.Agents), spec.LogW); err != nil {
				return ContainerInfo{}, err
			}
		}
		return ContainerInfo{Name: spec.Name, Image: spec.BaseImage, Status: "converged"}, nil
	}
	hostHome, _ := os.UserHomeDir()
	credsPath := filepath.Join(hostHome, ".claude", ".credentials.json")
	_, credsErr := os.Stat(credsPath)
	runArgs := createRunArgs(spec, credsPath, credsErr == nil)
	if out, err := dockerLogged(spec.LogW, runArgs...); err != nil {
		return ContainerInfo{}, fmt.Errorf("docker run: %v: %s", err, out)
	}
	if spec.HomeDir != "" {
		if out, err := dockerLogged(spec.LogW, "cp", spec.HomeDir+"/.", spec.Name+":/root/"); err != nil {
			return ContainerInfo{}, fmt.Errorf("docker cp home: %v: %s", err, out)
		}
		writeHomeSentinel(spec.Name, homeDigest(spec.HomeDir), spec.LogW)
	}
	if len(spec.Tools) > 0 || len(spec.Agents) > 0 {
		if err := provision(spec.Name, provisionScript(spec.Tools, spec.Agents), spec.LogW); err != nil {
			return ContainerInfo{}, err
		}
	}
	return ContainerInfo{Name: spec.Name, Image: spec.BaseImage, Status: "created"}, nil
}

// Teardown removes the per-project container, and (by tier) its volume and image.
// NOTE: the docker path is integration-validated (Work 7 / CI), not the local gate.
func (dockerRuntime) Teardown(name, level string, logw io.Writer) (Removed, error) {
	r := Removed{Containers: []string{}, Volumes: []string{}, Images: []string{}}
	if _, err := exec.LookPath("docker"); err != nil {
		return r, fmt.Errorf("docker not available: %w", err)
	}
	_, _ = dockerLogged(logw, "stop", name)
	if _, err := dockerLogged(logw, "rm", name); err == nil {
		r.Containers = append(r.Containers, name)
	}
	if level == "volumes" || level == "reset" {
		vol := name + "-data"
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
// `build --force`/`teardown` (`docker rm` keeps named volumes). Removal is the
// opt-in `--clean-state` Mac-side tier (SPEC-verbs teardown), NOT the volumes/
// reset tiers — wiping agent auth must be an explicit choice.
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
		args = append(args, "-v", agentHomeVolume(spec.Name)+":"+containerHome+"/.claude")
	}
	if spec.ProjectDir != "" {
		args = append(args, "-v", spec.ProjectDir+":"+containerWorkspace(spec.Project))
	}
	args = append(args, credsMount(hostCredsPath, credsPresent, spec.Agents)...)
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
export PATH="$PATH:/usr/local/go/bin:/root/go/bin"
echo 'export PATH=$PATH:/usr/local/go/bin:/root/go/bin' >> /root/.profile
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
		b.WriteString("retry sh -c 'curl -fsSL https://astral.sh/uv/install.sh | sh'\n")
	}
	// Install declared agent harnesses (T8). claude-code's native installer needs
	// no Node; it lands at ~/.local/bin, so put that on PATH for both login
	// (.profile) and interactive (.bashrc) shells (ties to T4's PATH split).
	for _, a := range agents {
		b.WriteString(agentInstallCmd(a))
	}

	// Make bash load the materialized per-project prompt from ~/.bashrc.d.
	b.WriteString("grep -q bashrc.d /root/.bashrc 2>/dev/null || " +
		"echo 'for f in ~/.bashrc.d/*.sh; do [ -r \"$f\" ] && . \"$f\"; done' >> /root/.bashrc\n")
	// Provision sentinel, written LAST: marks the container converged to this exact
	// tool set so a re-build can tell "fully provisioned" from "interrupted"
	// (ADR-0011). set -e above guarantees we never reach here on a failed install.
	fmt.Fprintf(&b, "mkdir -p %s\n", filepath.Dir(provisionSentinel))
	fmt.Fprintf(&b, "printf '%%s' '%s' > %s\n", provisionDigest(tools, agents), provisionSentinel)
	return b.String()
}

// agentInstallCmd emits the provision step that installs one agent harness and
// puts it on PATH. Unknown agents (no installer yet) emit nothing — they are
// still recorded in the provision digest so the intent is tracked.
func agentInstallCmd(a AgentInstall) string {
	switch a.Name {
	case "claude-code":
		const localBin = "$PATH:$HOME/.local/bin"
		return "retry sh -c 'curl -fsSL https://claude.ai/install.sh | bash'\n" +
			"grep -q '.local/bin' /root/.profile 2>/dev/null || echo 'export PATH=" + localBin + "' >> /root/.profile\n" +
			"grep -q '.local/bin' /root/.bashrc 2>/dev/null || echo 'export PATH=" + localBin + "' >> /root/.bashrc\n"
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
const containerHome = "/root"

// credsMount returns a read-only single-file bind of the host's EXISTING Claude
// credentials into the container, so the in-container agent reuses the same
// subscription auth the host already has (no token generation, no browser flow).
// Gated on claude-code being installed and the host creds file being present —
// mounting a missing path would make docker create a directory. Single-file so it
// does not shadow the materialised settings.json/statusline.sh.
func credsMount(hostCredsPath string, present bool, agents []AgentInstall) []string {
	if !present || !hasAgent(agents, "claude-code") {
		return nil
	}
	return []string{"-v", hostCredsPath + ":" + containerHome + "/.claude/.credentials.json:ro"}
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
