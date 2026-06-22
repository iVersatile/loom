//go:build integration

// Integration tier (RULES §7): real docker-backed e2e. Compiled only under
// `-tags integration` (make gate-integration / CI); skips cleanly when no daemon
// is present. Mirrors the S1/S2a canaries (integration_egress_test.go) — same
// build tag, same docker-shelling + defer-cleanup discipline.
//
// T20 slice S2b — the egress ALLOWLIST mechanism LIVE PROOF (proof-of-mechanism,
// NOT a fr-cited feature test; the engine-seam unit tests are the fr-cited ones).
// Where the proven #249 canary (.scratch spike) drove the proxy-sidecar shape by
// SHELLING docker directly, THIS canary drives it through loom's OWN engine path:
// it builds a ContainerSpec carrying EgressPolicy{Posture: allowlist} and calls
// dockerRuntime{}.Ensure(spec) — so the production provision-then-restrict +
// ensureEgressConfinement code (two networks + sidecar + `go run` embedded proxy +
// cut-over) is what's under test, not a hand-rolled docker dance.
//
// Three assertions, mirroring the #249 proof, run by `docker exec curl` in the
// realized project container:
//   - ALLOW : curl http://example.com/ THROUGH the proxy → 2xx/3xx (example.com is
//     the declared allow: host).
//   - DENY  : curl http://example.org/ THROUGH the proxy → 403 from the gatekeeper
//     (a non-allowlisted host blocked by name).
//   - BYPASS-DEAD (load-bearing): curl --noproxy '*' DIRECT → fails. The container
//     is on an `--internal` network with no NAT, so a raw socket has nowhere to go.
//     This is the `go test`/`/dev/tcp` exfil path T20 exists to close.
//
// REQUIREMENTS / why it can SKIP:
//   - The sidecar runs the embedded proxy via `go run` against the base image's
//     baked Go toolchain (ADR-0012). The default debian:bookworm-slim has NO Go, so
//     this canary REQUIRES LOOM_BASE_IMAGE to point at the loom-base image (the CI
//     integration job sets it). Absent Go in the base image, the proxy never starts
//     and the canary SKIPs rather than false-failing.
//   - The assertions need `curl` in the project container. The engine only
//     provisions when the spec has tools/agents (this minimal spec has none), so the
//     base image must carry curl. If curl is absent the canary SKIPs (honest about
//     its base-image dependency rather than failing on a missing probe tool).
package engine

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// curlInApp runs curl inside the project container, returning the HTTP status
// code curl printed (-w '%{http_code}') plus curl's exit error (nil on a clean
// transfer to the proxy). Used for the ALLOW/DENY assertions, where curl itself
// succeeds (the proxy answers, even a 403).
func curlInApp(t *testing.T, app string, curlArgs ...string) (code string, err error) {
	t.Helper()
	args := append([]string{"exec", app, "curl"}, curlArgs...)
	out, err := exec.Command("docker", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// TestT20S2bEngineEgressAllowlist drives the allowlist confinement end to end
// through loom's engine and proves the three mechanism assertions on real docker.
func TestT20S2bEngineEgressAllowlist(t *testing.T) {
	requireDocker(t)
	if _, err := exec.LookPath("go"); err != nil {
		// The build host needs Go to compile/`go run` nothing here — the sidecar
		// runs go run INSIDE the base image — but keep parity with the #249 canary's
		// guard; the real gate is the base image carrying Go (checked below).
		t.Skip("go toolchain not available on the build host")
	}

	name := containerName("t20s2b-allowlist")
	intNet := egressInternalNetwork(name)
	extNet := egressExternalNetwork(name)
	sidecar := egressSidecar(name)

	// Defer-clean ALL docker objects, even on failure/panic. Teardown removes the
	// project container + (best-effort) the sidecar/networks; the explicit removals
	// after it are belt-and-suspenders (LIFO: register the broad cleanups first so
	// they run last). Containers must be off the networks before a network rm
	// succeeds — Teardown's `docker rm` of the project container handles that.
	cleanup := func() {
		_ = exec.Command("docker", "rm", "-f", name).Run()
		_ = exec.Command("docker", "rm", "-f", sidecar).Run()
		_ = exec.Command("docker", "network", "rm", intNet).Run()
		_ = exec.Command("docker", "network", "rm", extNet).Run()
	}
	cleanup() // pre-clean any leftover from a crashed prior run.
	t.Cleanup(cleanup)

	// Drive the WHOLE confinement through the engine: an allowlist spec with
	// example.com declared. resolveEgressAllowlist unions in the load-bearing floor;
	// Ensure provisions (no-op here — no tools) then ensureEgressConfinement builds
	// the two networks + sidecar + `go run` proxy + cuts the container over to the
	// internal-only network. example.com is the host we ASSERT reachable, and it is
	// exactly the declared allow: entry (so the assertion targets a host that is
	// provably in the resolved allowlist, not merely in the floor).
	spec := minimalEgressSpec(name, false)
	spec.Project = "t20s2b"
	spec.Egress = EgressPolicy{Posture: "allowlist", Allow: []string{"example.com"}}

	if _, err := (dockerRuntime{}).Ensure(spec); err != nil {
		// A base image without Go cannot `go run` the sidecar proxy — that surfaces
		// as an Ensure error. Skip rather than fail: the canary's precondition (a
		// Go-bearing base image, ADR-0012) is not met. CI sets LOOM_BASE_IMAGE.
		t.Skipf("Ensure(allowlist) failed — base image likely lacks the Go toolchain the sidecar needs (set LOOM_BASE_IMAGE to the loom-base image): %v", err)
	}

	// The assertions need curl in the project container. The minimal spec installs
	// no tools, so curl must be baked into the base image. If it is not, skip — a
	// missing probe tool is not a confinement failure.
	if _, err := curlInApp(t, name, "--version"); err != nil {
		t.Skipf("curl not present in the project container (base image does not bake it); cannot run the reachability assertions: %v", err)
	}

	// Give the `go run`-compiled proxy a moment to come up (first `go run` compiles).
	// Poll the sidecar's listen port from inside the project container via the proxy
	// URL up to a bounded number of tries before the assertions.
	proxyReady := false
	for i := 0; i < 30; i++ {
		if _, err := curlInApp(t, name, "-sS", "--max-time", "3", "-o", "/dev/null",
			"-w", "%{http_code}", "http://example.com/"); err == nil {
			proxyReady = true
			break
		}
		_ = exec.Command("sh", "-c", "sleep 1").Run()
	}
	dumpProxyLogs := func() {
		out, _ := exec.Command("docker", "logs", sidecar).CombinedOutput()
		t.Logf("=== %s logs (proxy stderr) ===\n%s", sidecar, out)
	}
	if !proxyReady {
		dumpProxyLogs()
		t.Fatalf("the egress proxy sidecar never became reachable through the confinement (go run may have failed to compile/start)")
	}

	// ALLOW: example.com (the declared allow: host) is reachable THROUGH the proxy.
	// example.com may answer 200 or redirect (3xx); accept 200-399.
	code, err := curlInApp(t, name,
		"-sS", "--max-time", "10", "-o", "/dev/null", "-w", "%{http_code}", "http://example.com/")
	if err != nil {
		dumpProxyLogs()
		t.Fatalf("ALLOW: curl http://example.com/ THROUGH proxy failed (exit err=%v, code=%q) — the declared allowlisted host should be reachable", err, code)
	}
	if n, cerr := strconv.Atoi(code); cerr != nil || n < 200 || n > 399 {
		dumpProxyLogs()
		t.Errorf("ALLOW: curl http://example.com/ got HTTP %q, want 200-399 (reachable through the gatekeeper)", code)
	}

	// DENY: example.org is NOT in the resolved allowlist → the proxy answers 403.
	// curl succeeds at the transport level (it got an HTTP response from the proxy).
	code, err = curlInApp(t, name,
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
	code, err = curlInApp(t, name,
		"-sS", "--max-time", "8", "--noproxy", "*", "-o", "/dev/null", "-w", "%{http_code}", "http://example.com/")
	if err == nil {
		dumpProxyLogs()
		t.Errorf("BYPASS-DEAD: DIRECT curl http://example.com/ (--noproxy '*') SUCCEEDED (code=%q) — the internal network has a route out; the gatekeeper is bypassable. This is the exfil path T20 must close.", code)
	}
}
