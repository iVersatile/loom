//go:build integration

// Integration tier (RULES §7): real docker-backed e2e. Compiled only under
// `-tags integration` (make gate-integration / CI); skips cleanly when no daemon
// (or no host Go toolchain) is present. Mirrors the S1/S2a canaries
// (integration_egress_test.go) — same build tag, same docker-shelling +
// defer-cleanup discipline.
//
// T20 slice S2b — the egress ALLOWLIST mechanism LIVE PROOF, on DELIVERY A (the
// proxy ships as a hidden subcommand of the loom binary, not `go run` source).
// Where the proven #249 canary (.scratch spike) drove the proxy-sidecar shape by
// SHELLING docker directly, THIS canary drives it through loom's OWN engine path:
// it builds a ContainerSpec carrying EgressPolicy{Posture: allowlist} and calls
// dockerRuntime{}.Ensure(spec) — so the production provision-then-restrict +
// ensureEgressConfinement code (two networks + sidecar + `docker cp` loom +
// `loom __egress-proxy` + cut-over) is what's under test, not a hand-rolled dance.
//
// WHY THIS NOW RUNS (NOT SKIPS) ON THE PLAIN debian:bookworm-slim CI BASE:
// Delivery A removed the two reasons the old canary skipped. (1) No Go in the
// sidecar: the engine `docker cp`s the loom binary in and runs its hidden
// `__egress-proxy` subcommand, so the base image needs no Go toolchain. (2) No
// curl in the project container: the probe is `loom __http-get`, the loom binary
// `docker cp`'d into the project container too. The loom CLI builds CGO-free
// (CGO_ENABLED=0, no `import "C"` anywhere in-tree) → a static binary that runs on
// debian-slim's glibc. The ONLY remaining skip is the host preconditions: a docker
// daemon, and Go on the host to `go build` the loom binary the test cp's in (the
// running test binary is NOT a loom binary — it has no `__egress-proxy`).
//
// Three assertions, mirroring the #249 proof, run by `docker exec /loom __http-get`
// in the realized project container:
//   - ALLOW : __http-get http://example.com/ THROUGH the proxy → 2xx/3xx
//     (example.com is the declared allow: host).
//   - DENY  : __http-get http://example.org/ THROUGH the proxy → the proxy 403s,
//     surfaced to the probe (non-2xx/3xx or a transport-visible block).
//   - BYPASS-DEAD (load-bearing): __http-get --no-proxy DIRECT → FAILS. The
//     container is on an `--internal` network with no NAT, so a raw socket has
//     nowhere to go. This is the `go test`/`/dev/tcp` exfil path T20 exists to close.
package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// buildLoomBinary `go build`s the real loom CLI into a tempfile and returns its
// path. The integration test runner HAS Go on the host (the only remaining skip
// gate); the running TEST binary is not a loom binary, so the engine's
// egressProxyBinary seam (default os.Executable()) must be pointed at a freshly
// built loom that carries the `__egress-proxy` / `__http-get` subcommands.
func buildLoomBinary(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "loom")
	cmd := exec.Command("go", "build", "-o", out, "github.com/iVersatile/loom/cmd/loom")
	// Force CGO off so the cp'd binary is fully static and runs on the plain
	// debian:bookworm-slim sidecar/project images regardless of the host's default
	// CGO setting (loom imports no cgo, so this only pins determinism: a static
	// ELF with no dynamic loader / glibc dependency).
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build loom for the integration canary failed: %v\n%s", err, combined)
	}
	return out
}

// httpGetInApp runs `/loom __http-get <args>` inside the project container (the
// loom binary is cp'd in by the test). Returns the status code the probe printed
// (clean transfer — even a proxy 403 surfaces as a non-2xx code or a transport
// error per --no-proxy) plus the exec error (non-nil on a transport failure: the
// BYPASS-DEAD path). NOTE: the engine sets HTTP(S)_PROXY on the container, which
// __http-get honors via http.ProxyFromEnvironment by default.
func httpGetInApp(t *testing.T, app string, getArgs ...string) (code string, err error) {
	t.Helper()
	args := append([]string{"exec", app, "/loom", "__http-get"}, getArgs...)
	out, err := exec.Command("docker", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// TestT20S2bEngineEgressAllowlist drives the allowlist confinement end to end
// through loom's engine and proves the three mechanism assertions on real docker,
// on the plain debian:bookworm-slim base (no Go, no curl in the images).
func TestT20S2bEngineEgressAllowlist(t *testing.T) {
	requireDocker(t)
	if _, err := exec.LookPath("go"); err != nil {
		// The ONLY remaining skip beyond docker: the host needs Go to `go build` the
		// loom binary this test cp's into the sidecar + project container (the running
		// test binary is not a loom binary). CI's integration runner has Go on the host.
		t.Skip("go toolchain not available on the host to build the loom binary the canary cp's into the containers")
	}

	// Build the real loom binary and point the engine's binary-path seam at it: in
	// production ensureEgressConfinement cp's os.Executable() (the running loom CLI),
	// but here os.Executable() is the TEST binary (no __egress-proxy subcommand).
	loomBin := buildLoomBinary(t)
	origBinary := egressProxyBinary
	egressProxyBinary = func() (string, error) { return loomBin, nil }
	t.Cleanup(func() { egressProxyBinary = origBinary })

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
	// the two networks + sidecar + cp's loom in + runs `loom __egress-proxy` + cuts
	// the container over to the internal-only network. example.com is the host we
	// ASSERT reachable, and it is exactly the declared allow: entry.
	spec := minimalEgressSpec(name, false)
	spec.Project = "t20s2b"
	spec.Egress = EgressPolicy{Posture: "allowlist", Allow: []string{"example.com"}}

	if _, err := (dockerRuntime{}).Ensure(spec); err != nil {
		// Delivery A removed the Go-in-base precondition; a failure here is a REAL
		// confinement failure, not a skippable missing-toolchain case. Fail loudly.
		dumpProxyLogs := func() {
			out, _ := exec.Command("docker", "logs", sidecar).CombinedOutput()
			t.Logf("=== %s logs (proxy stderr) ===\n%s", sidecar, out)
		}
		dumpProxyLogs()
		t.Fatalf("Ensure(allowlist) failed (delivery A: no Go/curl needed — this is a real confinement bug): %v", err)
	}

	dumpProxyLogs := func() {
		out, _ := exec.Command("docker", "logs", sidecar).CombinedOutput()
		t.Logf("=== %s logs (proxy stderr) ===\n%s", sidecar, out)
	}

	// Copy the loom binary into the PROJECT container too: the connectivity probe is
	// `loom __http-get`, so neither image needs curl. The container is already on the
	// internal-only network (cut over by Ensure); `docker cp` works regardless of
	// network attachment (it streams via the daemon, not the container's network).
	if out, err := exec.Command("docker", "cp", loomBin, name+":/loom").CombinedOutput(); err != nil {
		t.Fatalf("cp loom binary into project container %s failed: %v\n%s", name, err, out)
	}

	// Give the detached `loom __egress-proxy` a moment to bind its listener. Poll
	// example.com (the declared allow) through the proxy up to a bounded number of
	// tries before the assertions.
	proxyReady := false
	for i := 0; i < 30; i++ {
		if code, err := httpGetInApp(t, name, "http://example.com/"); err == nil {
			if n, cerr := strconv.Atoi(code); cerr == nil && n >= 200 && n <= 399 {
				proxyReady = true
				break
			}
		}
		_ = exec.Command("sh", "-c", "sleep 1").Run()
	}
	if !proxyReady {
		dumpProxyLogs()
		t.Fatalf("the egress proxy sidecar never became reachable through the confinement (loom __egress-proxy may have failed to start)")
	}

	// ALLOW: example.com (the declared allow: host) is reachable THROUGH the proxy.
	// example.com may answer 200 or redirect (3xx); accept 200-399.
	code, err := httpGetInApp(t, name, "http://example.com/")
	if err != nil {
		dumpProxyLogs()
		t.Fatalf("ALLOW: __http-get http://example.com/ THROUGH proxy failed (err=%v, out=%q) — the declared allowlisted host should be reachable", err, code)
	}
	if n, cerr := strconv.Atoi(code); cerr != nil || n < 200 || n > 399 {
		dumpProxyLogs()
		t.Errorf("ALLOW: __http-get http://example.com/ got %q, want a 200-399 status (reachable through the gatekeeper)", code)
	}

	// DENY: example.org is NOT in the resolved allowlist → the proxy answers 403.
	// For a plain-HTTP forward the proxy returns a 403 RESPONSE (clean transfer), so
	// __http-get prints "403". (No --no-proxy here: the probe goes via the proxy.)
	code, err = httpGetInApp(t, name, "http://example.org/")
	if err != nil {
		dumpProxyLogs()
		t.Fatalf("DENY: __http-get http://example.org/ THROUGH proxy errored (err=%v, out=%q) — expected a clean 403 status from the proxy", err, code)
	}
	if code != "403" {
		dumpProxyLogs()
		t.Errorf("DENY: __http-get http://example.org/ got %q, want 403 (the gatekeeper blocks a non-allowlisted host by name)", code)
	}

	// BYPASS-DEAD (load-bearing): a DIRECT connection (--no-proxy ignores HTTP(S)_PROXY)
	// has nowhere to go — the `--internal` network has no NAT. __http-get MUST error.
	code, err = httpGetInApp(t, name, "--no-proxy", "http://example.com/")
	if err == nil {
		dumpProxyLogs()
		t.Errorf("BYPASS-DEAD: DIRECT __http-get http://example.com/ (--no-proxy) SUCCEEDED (out=%q) — the internal network has a route out; the gatekeeper is bypassable. This is the exfil path T20 must close.", code)
	}
}
