//go:build integration

// Integration tier (RULES §7): real docker-backed e2e. Compiled only under
// `-tags integration` (make gate-integration / CI); skips cleanly when no daemon
// is present. Mirrors the S1 canary (integration_egress_test.go) — same build
// tag, same docker-shelling + defer-cleanup discipline.
//
// T20 slice S2b — the egress ALLOWLIST mechanism PROOF (proof-of-mechanism, NOT
// a feature). It proves on REAL docker that ADR-0028's proxy-sidecar shape (the
// spike .scratch/spikes/t20-s2b-allowlist-mechanism.md promoted S3→S2b) actually
// enforces a per-HOSTNAME allowlist with a mechanism, not trust:
//
//   - A project container sits on an `--internal` docker network (NO route out).
//   - A sidecar "gatekeeper" container sits on BOTH that internal network and a
//     normal bridge (so the sidecar has real internet; the project does not).
//   - The sidecar runs a tiny forward proxy (testdata/egressproxy) that allows
//     only declared hostnames. The project container reaches the internet ONLY
//     through it (http_proxy/https_proxy env).
//
// Three assertions:
//   - ALLOW : curl http://example.com/ THROUGH the proxy → 2xx/3xx.
//   - DENY  : curl http://example.org/ THROUGH the proxy → 403 from the proxy.
//   - BYPASS-DEAD : curl --noproxy '*' http://example.com/ DIRECT → fails (the
//     `--internal` network has no NAT). This is the load-bearing one — it is the
//     `go test`/`/dev/tcp` exfil bypass that T20 exists to close.
//
// This file makes NO engine-API change: it shells docker directly, exactly like
// S1's secondary check. There is no ContainerSpec/playbook/SPEC/FR change — the
// CI integration run IS the proof.
package engine

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// s2bCounter makes per-object names unique within a process WITHOUT relying on
// wall-clock time (getpid + an atomic counter, per the build brief). Two objects
// created in the same millisecond still get distinct names.
var s2bCounter atomic.Int64

func s2bSuffix() string {
	return strconv.Itoa(os.Getpid()) + "-" + strconv.FormatInt(s2bCounter.Add(1), 10)
}

// dockerOK runs a docker command and fails the test with combined output on
// error. Used for setup steps that MUST succeed.
func dockerOK(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// curlCodeInApp runs curl inside the project (app) container and returns the
// HTTP status code string curl printed (via -w '%{http_code}') plus curl's exit
// error (nil on a clean transfer). Used for the ALLOW/DENY assertions, which
// expect curl itself to succeed (the proxy answers).
func curlCodeInApp(t *testing.T, app string, curlArgs ...string) (code string, err error) {
	t.Helper()
	args := append([]string{"exec", app, "curl"}, curlArgs...)
	out, err := exec.Command("docker", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// installCurl apt-installs curl + ca-certificates in a container that currently
// HAS internet (the provision phase of provision-then-restrict).
func installCurl(t *testing.T, name string) {
	t.Helper()
	// Provision with DIRECT egress: clear any proxy env for THIS exec so apt uses
	// the bridge's real internet, never the gatekeeper. The app container carries
	// http_proxy=<gatekeeper> as its runtime env (set at `docker run`, the realistic
	// shape), but the gatekeeper name is not resolvable until the container joins the
	// internal network — so provisioning (apt) MUST bypass the proxy. This is the
	// "provision" half of provision-then-restrict: broad egress during setup, the
	// gatekeeper-only route only after the cut-over below.
	dockerOK(t, "exec",
		"-e", "http_proxy=", "-e", "https_proxy=", "-e", "HTTP_PROXY=", "-e", "HTTPS_PROXY=",
		name, "sh", "-c",
		"apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl ca-certificates >/dev/null")
}

// TestT20S2bProxyEgressAllowlist is the S2b proof. See the file header.
//
// provision-then-restrict: the app container is started on the DEFAULT bridge,
// curl is apt-installed there (it has internet), and only THEN is it
// disconnected from the bridge and connected to the `--internal` network — so
// the running container has the tools it needs but no broad egress. This
// deliberately demonstrates ADR-0028 §2's provision-then-restrict trade with the
// same docker network disconnect/connect primitive the engine will use.
func TestT20S2bProxyEgressAllowlist(t *testing.T) {
	requireDocker(t)
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available to compile the proxy")
	}

	sfx := s2bSuffix()
	extNet := "loom-s2b-ext-" + sfx
	intNet := "loom-s2b-int-" + sfx
	proxy := "loom-s2b-proxy-" + sfx
	app := "loom-s2b-app-" + sfx
	img := baseImage() // debian:bookworm-slim by default (LOOM_BASE_IMAGE in CI).

	// --- Cleanup EVERYTHING first (defer), even on failure/panic. Mirror S1. ---
	// Containers must be removed before their networks; order the defers so the
	// network removals run AFTER the container removals (defer is LIFO, so
	// register networks first, containers last).
	defer func() { _ = exec.Command("docker", "network", "rm", extNet).Run() }()
	defer func() { _ = exec.Command("docker", "network", "rm", intNet).Run() }()
	defer func() { _ = exec.Command("docker", "rm", "-f", proxy).Run() }()
	defer func() { _ = exec.Command("docker", "rm", "-f", app).Run() }()

	// --- 1. Compile the proxy for the container's platform (linux). ---
	tmpbin := t.TempDir() + "/egressproxy"
	build := exec.Command("go", "build", "-o", tmpbin, "./testdata/egressproxy")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build egressproxy (GOOS=linux GOARCH=%s): %v\n%s", runtime.GOARCH, err, out)
	}

	// --- 2. Two networks: a normal bridge and an INTERNAL (no route out) one. ---
	dockerOK(t, "network", "create", extNet)
	dockerOK(t, "network", "create", "--internal", intNet)

	// --- 3. Gatekeeper sidecar on BOTH networks; install curl/ca-certs; run proxy.
	// It is born on the bridge (extNet) so apt works, then also joined to intNet
	// so the project container can reach it.
	dockerOK(t, "run", "-d", "--name", proxy, "--network", extNet, img, "sleep", "infinity")
	dockerOK(t, "network", "connect", intNet, proxy)
	installCurl(t, proxy) // not strictly needed by the proxy, but proves it has internet.
	dockerOK(t, "cp", tmpbin, proxy+":/egressproxy")
	dockerOK(t, "exec", proxy, "chmod", "+x", "/egressproxy")
	// Run the proxy in the background. ALLOW only example.com.
	dockerOK(t, "exec", "-d", "-e", "ALLOW=example.com", "-e", "LISTEN=:8080", proxy, "/egressproxy")

	// dumpProxyLogs surfaces the gatekeeper's allow/deny decisions on any failure.
	dumpProxyLogs := func() {
		out, _ := exec.Command("docker", "logs", proxy).CombinedOutput()
		t.Logf("=== %s logs (proxy stderr) ===\n%s", proxy, out)
	}

	// --- 4. Project container: provision-then-restrict. ---
	// (a) Start on the DEFAULT bridge so apt has internet; install curl there.
	proxyURL := "http://" + proxy + ":8080"
	dockerOK(t, "run", "-d", "--name", app,
		"-e", "http_proxy="+proxyURL,
		"-e", "https_proxy="+proxyURL,
		"-e", "HTTP_PROXY="+proxyURL,
		"-e", "HTTPS_PROXY="+proxyURL,
		img, "sleep", "infinity")
	installCurl(t, app)
	// (b) RESTRICT: drop broad egress, then attach the internal-only network. The
	// running container keeps curl but now has no route out except via the proxy.
	dockerOK(t, "network", "disconnect", "bridge", app)
	dockerOK(t, "network", "connect", intNet, app)

	// Sanity: the app container must have NO non-loopback route that bypasses the
	// proxy except the internal network link (which has no NAT). This is implicit
	// in the BYPASS-DEAD assertion below; we go straight to the three proofs.

	// --- 5. The three assertions. ---

	// ALLOW: example.com is in the allowlist → reachable THROUGH the proxy.
	// example.com may answer 200 or redirect (3xx); accept 200-399.
	code, err := curlCodeInApp(t, app,
		"-sS", "--max-time", "10", "-o", "/dev/null", "-w", "%{http_code}", "http://example.com/")
	if err != nil {
		dumpProxyLogs()
		t.Fatalf("ALLOW: curl http://example.com/ THROUGH proxy failed (exit err=%v, code=%q) — allowlisted host should be reachable", err, code)
	}
	if n, cerr := strconv.Atoi(code); cerr != nil || n < 200 || n > 399 {
		dumpProxyLogs()
		t.Errorf("ALLOW: curl http://example.com/ got HTTP %q, want 200-399 (reachable through the gatekeeper)", code)
	}

	// DENY: example.org is NOT in the allowlist → the proxy answers 403. curl
	// succeeds at the transport level (it got an HTTP response from the proxy),
	// so we assert on the status code.
	code, err = curlCodeInApp(t, app,
		"-sS", "--max-time", "10", "-o", "/dev/null", "-w", "%{http_code}", "http://example.org/")
	if err != nil {
		dumpProxyLogs()
		t.Fatalf("DENY: curl http://example.org/ THROUGH proxy errored (exit err=%v, code=%q) — expected a clean 403 from the proxy", err, code)
	}
	if code != "403" {
		dumpProxyLogs()
		t.Errorf("DENY: curl http://example.org/ got HTTP %q, want 403 (the gatekeeper blocks a non-allowlisted host by name)", code)
	}

	// BYPASS-DEAD (load-bearing): a DIRECT connection that ignores the proxy has
	// nowhere to go — the `--internal` network has no NAT. curl MUST fail.
	code, err = curlCodeInApp(t, app,
		"-sS", "--max-time", "8", "--noproxy", "*", "-o", "/dev/null", "-w", "%{http_code}", "http://example.com/")
	if err == nil {
		dumpProxyLogs()
		t.Errorf("BYPASS-DEAD: DIRECT curl http://example.com/ (--noproxy '*') SUCCEEDED (code=%q) — the internal network has a route out; the gatekeeper is bypassable. This is the exfil path T20 must close.", code)
	}
}
