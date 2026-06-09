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

// ToolInstall is one resolved tool the container must provision, with its source.
type ToolInstall struct {
	Name   string
	Source string
}

// ContainerSpec describes the container build wants to converge to.
type ContainerSpec struct {
	Name      string
	BaseImage string
	Tools     []ToolInstall // resolved tools to install, by source
	HomeDir   string        // host staging dir seeding the container $HOME
	Force     bool          // rebuild from scratch even if the container exists
	LogW      io.Writer     // diagnostic log sink for raw docker/provision output
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
	want := toolsetDigest(spec.Tools)
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
		// toolchain. Compare the provision sentinel; if missing or stale, re-seed
		// $HOME and re-run the idempotent provision to converge ("converged").
		if !needsReprovision(readProvisionDigest(spec.Name), want) {
			return ContainerInfo{Name: spec.Name, Image: spec.BaseImage, Status: "exists"}, nil
		}
		if spec.HomeDir != "" {
			if out, err := dockerLogged(spec.LogW, "cp", spec.HomeDir+"/.", spec.Name+":/root/"); err != nil {
				return ContainerInfo{}, fmt.Errorf("docker cp home (reconcile): %v: %s", err, out)
			}
		}
		if len(spec.Tools) > 0 {
			if err := provision(spec.Name, provisionScript(spec.Tools), spec.LogW); err != nil {
				return ContainerInfo{}, err
			}
		}
		return ContainerInfo{Name: spec.Name, Image: spec.BaseImage, Status: "converged"}, nil
	}
	if out, err := dockerLogged(spec.LogW, "run", "-d", "--name", spec.Name,
		spec.BaseImage, "sleep", "infinity"); err != nil {
		return ContainerInfo{}, fmt.Errorf("docker run: %v: %s", err, out)
	}
	if spec.HomeDir != "" {
		if out, err := dockerLogged(spec.LogW, "cp", spec.HomeDir+"/.", spec.Name+":/root/"); err != nil {
			return ContainerInfo{}, fmt.Errorf("docker cp home: %v: %s", err, out)
		}
	}
	if len(spec.Tools) > 0 {
		if err := provision(spec.Name, provisionScript(spec.Tools), spec.LogW); err != nil {
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
	if out, err := dockerLogged(logw, "exec", name, "sh", "/tmp/loom-provision.sh"); err != nil {
		return fmt.Errorf("provision: %v: %s", err, out)
	}
	return nil
}

// toolsetDigest is a stable fingerprint of the declared tool set, written into
// the container as the provision sentinel and compared on re-build. Order-stable
// (sorted) so a reordered playbook does not look like drift. Empty set ⇒ "".
func toolsetDigest(tools []ToolInstall) string {
	if len(tools) == 0 {
		return ""
	}
	lines := make([]string, len(tools))
	for i, t := range tools {
		lines[i] = t.Name + "|" + t.Source
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

func defaultRuntime() ContainerRuntime { return dockerRuntime{} }

// containerName derives the deterministic per-project container name (ADR-0001).
func containerName(project string) string {
	return "loom-" + project + "-dev"
}

// provisionScript builds a POSIX-sh script that installs the resolved tools into
// the container, grouped by source: apt for system packages, the official Go
// tarball for the toolchain, `go install` for Go-distributed tools, the uv
// installer for uv. Run on a minimal debian:bookworm-slim, so it installs its own
// prerequisites (ca-certificates, curl) first.
func provisionScript(tools []ToolInstall) string {
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
	b.WriteString("apt-get update\n")
	pkgs := append([]string{"ca-certificates", "curl", "git", "tar"}, filterOut(apt, "git")...)
	b.WriteString("apt-get install -y --no-install-recommends " + strings.Join(pkgs, " ") + "\n")
	b.WriteString("apt-get clean && rm -rf /var/lib/apt/lists/*\n")

	if needGo {
		b.WriteString(`ARCH="$(dpkg --print-architecture)"
GOVER="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -1)"
curl -fsSL "https://go.dev/dl/${GOVER}.linux-${ARCH}.tar.gz" -o /tmp/go.tgz
rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tgz && rm -f /tmp/go.tgz
export PATH="$PATH:/usr/local/go/bin:/root/go/bin"
echo 'export PATH=$PATH:/usr/local/go/bin:/root/go/bin' >> /root/.profile
# Limit compile parallelism so memory-heavy installs (gopls) fit a small VM.
export GOFLAGS=-p=2
`)
	}
	for _, m := range goInstall {
		b.WriteString("go install " + m + "\n")
	}
	if needUv {
		b.WriteString("curl -fsSL https://astral.sh/uv/install.sh | sh\n")
	}
	// Make bash load the materialized per-project prompt from ~/.bashrc.d.
	b.WriteString("grep -q bashrc.d /root/.bashrc 2>/dev/null || " +
		"echo 'for f in ~/.bashrc.d/*.sh; do [ -r \"$f\" ] && . \"$f\"; done' >> /root/.bashrc\n")
	// Provision sentinel, written LAST: marks the container converged to this exact
	// tool set so a re-build can tell "fully provisioned" from "interrupted"
	// (ADR-0011). set -e above guarantees we never reach here on a failed install.
	fmt.Fprintf(&b, "mkdir -p %s\n", filepath.Dir(provisionSentinel))
	fmt.Fprintf(&b, "printf '%%s' '%s' > %s\n", toolsetDigest(tools), provisionSentinel)
	return b.String()
}

// goModule maps a go-install tool name to its module path.
func goModule(tool string) string {
	switch tool {
	case "gopls":
		return "golang.org/x/tools/gopls@latest"
	case "gitleaks":
		return "github.com/gitleaks/gitleaks/v8@latest"
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
