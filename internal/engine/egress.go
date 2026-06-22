package engine

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// egressProxySource is loom's shipped egress-gatekeeper proxy, embedded as
// SOURCE (T20 S2b, ADR-0028 Amendment 1). At sidecar startup the engine writes
// this source into the sidecar and runs it with `go run /egressproxy.go` — the
// sidecar uses loom's base image, which BAKES the Go toolchain (ADR-0012), so a
// single stdlib-only standalone file needs no go.mod. Embedding the source (not a
// prebuilt per-arch binary) keeps the build graph a single Go module and makes
// the proxy reproducible from the tree on every arch the base image supports.
// The proxy itself is the proven #249 gatekeeper (testdata→shipped), unchanged in
// behavior: explicit-CONNECT + plain-HTTP forward, allow-by-hostname, deny→403,
// stderr decision logs.
//
//go:embed egressproxy/main.go
var egressProxySource string

// egressProxyPath is where the embedded proxy source is written inside the
// sidecar before `go run`.
const egressProxyPath = "/egressproxy.go"

// egressProxyPort is the port the sidecar proxy listens on (and the port the
// project container's HTTP(S)_PROXY env points at).
const egressProxyPort = "8080"

// loadBearingEgressHosts is the hardcoded base allowlist the engine UNIONs into
// ANY declared `allow:` so a deny-by-default posture can never BRICK the agent by
// a forgotten host (ADR-0028 §4, corrected by Amendment 1 R3 — the biggest risk
// the amendment carries). Amendment 1 R3 is explicit that §4's original three
// entries are INCOMPLETE: claude.ai + console.anthropic.com (OAuth login / token
// refresh for the subscription seat) and statsig.anthropic.com + sentry.io (CLI
// feature-gates + error reporting) are ABSENT from Anthropic's OWN devcontainer
// firewall and would silently kill the agent. github.com + api.github.com cover
// the git-controller seat (ADR-0026). These are owned by the base tier and are
// always present regardless of what a project declares.
//
// NOTE: this is the load-bearing FLOOR, not the full provision allowlist. The
// CDN hosts a broad provision needs (go.dev, apt mirrors, the claude.ai
// installer, npm/PyPI) are reachable during the broad-egress provision phase
// (provision-then-restrict, ADR-0028 §2), not at runtime — so they are
// deliberately NOT in this runtime floor.
var loadBearingEgressHosts = []string{
	"api.anthropic.com",     // the model API — without it the agent cannot think
	"claude.ai",             // OAuth login / the install.sh host (R3: absent from Anthropic's own firewall)
	"console.anthropic.com", // OAuth token refresh for the subscription seat (R3)
	"statsig.anthropic.com", // CLI feature-gates / telemetry (R3)
	"sentry.io",             // CLI error reporting (R3)
	"github.com",            // VCS over HTTPS for the git-controller seat (ADR-0026)
	"api.github.com",        // GitHub API for the git-controller seat (ADR-0026, R3)
}

// resolveEgressAllowlist returns the runtime-reachable hostname set for an
// `egress: allowlist` container: the UNION of the load-bearing floor and the
// playbook-declared `allow:`, deduped and sorted (deterministic for the proxy
// ALLOW env and for tests). The load-bearing floor is unconditional — a project
// that forgets the model endpoint still gets it, never a bricked agent.
func resolveEgressAllowlist(declared []string) []string {
	set := map[string]bool{}
	var out []string
	add := func(h string) {
		h = strings.TrimSpace(h)
		if h == "" || set[h] {
			return
		}
		set[h] = true
		out = append(out, h)
	}
	for _, h := range loadBearingEgressHosts {
		add(h)
	}
	for _, h := range declared {
		add(h)
	}
	sort.Strings(out)
	return out
}

// EgressPolicy is the resolved egress posture threaded onto a ContainerSpec (T20
// S2b, ADR-0028 Amendment 1). Posture is the merged `networking.egress:` value;
// Allow is the playbook-declared `allow:` (the engine unions the load-bearing
// floor in at confinement time, resolveEgressAllowlist). For posture "allowlist"
// the engine builds the proxy-sidecar confinement (an internal network whose sole
// route is a loom-managed proxy enforcing the resolved allowlist). off/none are
// carried by NoEgress / the default path and never reach the confinement code.
type EgressPolicy struct {
	Posture string   // "" | "off" | "none" | "allowlist" (merged networking.egress)
	Allow   []string // playbook-declared allow: hostnames (load-bearing floor unioned in at confinement)
}

// isAllowlist reports whether this policy requests the deny-by-default proxy
// confinement (the only posture egress.go builds; none/off are the NoEgress /
// default-bridge paths).
func (p EgressPolicy) isAllowlist() bool {
	return p.Posture == "allowlist"
}

// egressInternalNetwork is the per-container `--internal` docker network the
// project container joins as its SOLE route (ADR-0028 Amendment 1 R1). An
// `--internal` network has NO route off-box, so the project container can reach
// only the sidecar that bridges it to the internet — the route is the fence.
func egressInternalNetwork(container string) string { return container + "-egress" }

// egressExternalNetwork is the per-container bridge network the sidecar uses for
// its REAL internet (it is on both this and the internal network). A dedicated
// bridge (not the shared default) keeps the confinement self-contained so
// teardown removes exactly what Ensure created.
func egressExternalNetwork(container string) string { return container + "-egress-ext" }

// egressSidecar is the per-container proxy-sidecar container name.
func egressSidecar(container string) string { return container + "-egress-proxy" }

// egressProxyURL is the HTTP(S)_PROXY value the project container points at: the
// sidecar (resolvable by name on the shared internal network) on the proxy port.
// This is CONVENIENCE for cooperating clients (clean CONNECT semantics), NOT the
// control — the internal-network-only route is what makes it mechanism-not-trust.
func egressProxyURL(container string) string {
	return "http://" + egressSidecar(container) + ":" + egressProxyPort
}

// egressProxyEnvNames are the proxy env var NAMES set on the project container so
// cooperating HTTP clients (undici/curl/git-over-HTTPS) route through the sidecar.
// Both cases are set because clients disagree on which they read.
var egressProxyEnvNames = []string{"http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY"}

// egressProxyRunEnvArgs returns the `docker run -e NAME=VALUE` args that point a
// project container's HTTP(S)_PROXY at its egress sidecar (convenience for
// cooperating clients; the internal-network route is the actual fence). Empty for
// every non-allowlist posture — only an allowlist container gets the proxy env.
func egressProxyRunEnvArgs(name string, policy EgressPolicy) []string {
	if !policy.isAllowlist() {
		return nil
	}
	url := egressProxyURL(name)
	args := make([]string, 0, len(egressProxyEnvNames)*2)
	for _, n := range egressProxyEnvNames {
		args = append(args, "-e", n+"="+url)
	}
	return args
}

// provisionEnvArgs returns the `docker exec -e NAME=` args that BLANK the proxy
// env for a provision exec (T20 S2b provision-then-restrict): an allowlist
// container carries proxy env pointing at a not-yet-resolvable sidecar, so
// provisioning must clear it and use the bridge's direct egress. Empty (no
// clearing) for every other case.
func provisionEnvArgs(clearProxyEnv bool) []string {
	if !clearProxyEnv {
		return nil
	}
	args := make([]string, 0, len(egressProxyEnvNames)*2)
	for _, n := range egressProxyEnvNames {
		args = append(args, "-e", n+"=")
	}
	return args
}

// egressProxyRunCmd is the command the sidecar runs to start the proxy: `go run`
// the embedded source with the resolved allowlist in ALLOW and the listen port in
// LISTEN. The base image bakes the Go toolchain (ADR-0012), so a stdlib-only
// single file needs no go.mod. ALLOW is comma-joined (the proxy splits on ",").
func egressProxyRunCmd(allow []string) []string {
	return []string{
		"env",
		"ALLOW=" + strings.Join(allow, ","),
		"LISTEN=:" + egressProxyPort,
		"go", "run", egressProxyPath,
	}
}

// ensureEgressConfinement realizes the `egress: allowlist` proxy-sidecar
// confinement and CUTS the project container over to it (the restrict half of
// provision-then-restrict, ADR-0028 §2 / Amendment 1). It is called from Ensure
// AFTER the project container is created and provisioned WITH direct egress — so
// apt/go/installers ran over the bridge, and the gatekeeper's name (unresolvable
// pre-cutover) was never on the provision path. The exact pre-cutover-name bug
// the #249 proof found and fixed: provisioning must NOT route through the proxy.
//
// Sequence (idempotent-ish; --force recreates from scratch, so this runs on a
// fresh project container):
//  1. create two per-container networks: an `--internal` one (no route off-box)
//     and a normal bridge for the sidecar's real internet;
//  2. create the sidecar on the bridge, join it to the internal network, write
//     the embedded proxy source in, and `go run` it (background) with ALLOW set
//     to the resolved allowlist (declared ∪ load-bearing floor);
//  3. CUT the project container over: disconnect it from the default bridge and
//     connect it to the internal network as its SOLE route. A raw socket now has
//     nowhere to go but the sidecar — the route is the fence (mechanism, not the
//     HTTP(S)_PROXY env, which is convenience for cooperating clients).
//
// NOTE: every docker call here is integration-validated (the #249 canary +
// CI/Mac), not the local gate — like the rest of dockerRuntime.
func ensureEgressConfinement(name string, policy EgressPolicy, image string, logw io.Writer) error {
	if !policy.isAllowlist() {
		return nil
	}
	allow := resolveEgressAllowlist(policy.Allow)
	intNet := egressInternalNetwork(name)
	extNet := egressExternalNetwork(name)
	sidecar := egressSidecar(name)

	// Idempotency / re-run hygiene: a prior attempt may have left these behind.
	// Remove the sidecar + networks first so a re-Ensure starts clean. Ignore
	// errors (absent is the normal case).
	_, _ = dockerLogged(logw, "rm", "-f", sidecar)
	_, _ = dockerLogged(logw, "network", "rm", intNet)
	_, _ = dockerLogged(logw, "network", "rm", extNet)

	// 1. The two networks. The internal one is the fence; the external one gives
	// the sidecar (only) real internet.
	if out, err := dockerLogged(logw, "network", "create", extNet); err != nil {
		return fmt.Errorf("egress: create external network %s: %v: %s", extNet, err, out)
	}
	if out, err := dockerLogged(logw, "network", "create", "--internal", intNet); err != nil {
		return fmt.Errorf("egress: create internal network %s: %v: %s", intNet, err, out)
	}

	// 2. The sidecar on BOTH networks, born on the bridge so it has internet, then
	// joined to the internal network so the project container can reach it.
	if out, err := dockerLogged(logw, "run", "-d",
		"--name", sidecar,
		"--label", "loom.managed=true", "--label", "loom.egress-sidecar="+name,
		"--network", extNet, image, "sleep", "infinity"); err != nil {
		return fmt.Errorf("egress: run sidecar %s: %v: %s", sidecar, err, out)
	}
	if out, err := dockerLogged(logw, "network", "connect", intNet, sidecar); err != nil {
		return fmt.Errorf("egress: join sidecar %s to %s: %v: %s", sidecar, intNet, err, out)
	}
	// Write the embedded proxy source into the sidecar, then `go run` it in the
	// background. The base image bakes the Go toolchain (ADR-0012) — a stdlib-only
	// single file needs no go.mod.
	if err := writeProxySource(sidecar, logw); err != nil {
		return err
	}
	if out, err := dockerLogged(logw, append([]string{"exec", "-d", sidecar}, egressProxyRunCmd(allow)...)...); err != nil {
		return fmt.Errorf("egress: start proxy in sidecar %s: %v: %s", sidecar, err, out)
	}

	// 3. CUT-OVER: drop the project container's broad egress, then attach the
	// internal-only network. The container keeps everything provisioning installed
	// but now has no route out except via the sidecar.
	if out, err := dockerLogged(logw, "network", "disconnect", "bridge", name); err != nil {
		return fmt.Errorf("egress: disconnect %s from bridge: %v: %s", name, err, out)
	}
	if out, err := dockerLogged(logw, "network", "connect", intNet, name); err != nil {
		return fmt.Errorf("egress: connect %s to %s: %v: %s", name, intNet, err, out)
	}
	return nil
}

// writeProxySource writes the embedded proxy source into the sidecar at
// egressProxyPath via `docker cp` of a host tempfile (more robust than piping on
// stdin — the provision() precedent). NOTE: integration-validated.
func writeProxySource(sidecar string, logw io.Writer) error {
	tmp, err := os.CreateTemp("", "loom-egressproxy-*.go")
	if err != nil {
		return fmt.Errorf("egress: proxy tmp: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.WriteString(egressProxySource); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("egress: write proxy source: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("egress: close proxy source: %w", err)
	}
	if out, err := dockerLogged(logw, "cp", tmp.Name(), sidecar+":"+egressProxyPath); err != nil {
		return fmt.Errorf("egress: cp proxy source into %s: %v: %s", sidecar, err, out)
	}
	return nil
}

// teardownEgressConfinement removes the per-container egress sidecar + networks,
// best-effort (each absent object is a no-op). Called from Teardown so the
// confinement rides the existing per-container resource cleanup (ADR-0028
// Consequences: "teardown must clean it"). Removed records what was actually
// removed. NOTE: integration-validated.
func teardownEgressConfinement(name string, logw io.Writer, r *Removed) {
	sidecar := egressSidecar(name)
	if _, err := dockerLogged(logw, "rm", "-f", sidecar); err == nil {
		r.Containers = append(r.Containers, sidecar)
	}
	// The project container must be off the networks before they can be removed;
	// `docker rm` of the project container (Teardown does it before this) handles
	// that, so the network removals here just clean the now-empty networks.
	for _, net := range []string{egressInternalNetwork(name), egressExternalNetwork(name)} {
		if _, err := dockerLogged(logw, "network", "rm", net); err == nil {
			r.Networks = append(r.Networks, net)
		}
	}
}

// egressSidecarReachable reports whether the egress sidecar exists and is running
// — the doctor's allowlist confinement check needs the gatekeeper present (a
// missing sidecar means the project container has NO route out at all: a broken
// confinement, not a safe one). Read-only. NOTE: integration-validated.
func egressSidecarRunning(name string) bool {
	return containerRunning(egressSidecar(name))
}

// containerNetworks lists the docker networks a container is attached to, by
// inspecting it (read-only). Doctor's allowlist claim uses it to confirm the
// project container is on the internal egress network and NOT on a bridge with a
// route out. NOTE: integration-validated, not the local gate.
func containerNetworks(name string) ([]string, error) {
	out, err := exec.Command("docker", "inspect", "-f",
		"{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}", name).Output()
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}
