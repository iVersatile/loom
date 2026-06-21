//go:build integration

// Integration tier (RULES §7): real docker-backed e2e. Compiled only under
// `-tags integration` (make gate-integration / CI); skips cleanly when no daemon
// is present.
//
// T20 slice S1 — the egress-control MECHANISM PROOF. These canaries prove that
// `--network none` cuts a container's egress at the NETWORK layer: a no-egress
// container has NO interface but loopback, so an in-container process physically
// cannot reach any host. This refutes the documented weakness that loom's egress
// control is a harness command-deny-list (TRUST) any code with a socket bypasses
// — `--network none` is MECHANISM, not trust. The container-level no-egress mode
// is the proof primitive (ContainerSpec.NoEgress); the user-facing networking
// policy lands with the T20 ADR / slice S2.
package engine

import (
	"os/exec"
	"strings"
	"testing"
)

// minimalEgressSpec builds the smallest ContainerSpec that exercises
// createRunArgs through the engine runtime: a local base image and nothing that
// needs provisioning egress (no tools, no agents, no home, no project). The only
// variable is NoEgress, so the two canaries differ ONLY in the mechanism under
// test.
func minimalEgressSpec(name string, noEgress bool) ContainerSpec {
	return ContainerSpec{
		Name:      name,
		Project:   "t20s1",
		BaseImage: baseImage(),
		NoEgress:  noEgress,
	}
}

// netInterfaces lists the container's network interfaces by reading
// /sys/class/net inside it (hermetic — no external reachability needed). The
// returned slice is the space/newline-split interface names.
func netInterfaces(t *testing.T, name string) []string {
	t.Helper()
	out, err := exec.Command("docker", "exec", name, "ls", "/sys/class/net").CombinedOutput()
	if err != nil {
		t.Fatalf("ls /sys/class/net in %s: %v: %s", name, err, out)
	}
	return strings.Fields(string(out))
}

func hasNonLoopback(ifaces []string) bool {
	for _, i := range ifaces {
		if i != "lo" {
			return true
		}
	}
	return false
}

// TestT20S1NoEgressHasNoNetworkInterface is the decisive, flake-free anchor: a
// container created with NoEgress: true has ONLY the loopback interface — no
// eth0, no path out at all. The interface check is hermetic; it needs no
// external host to be reachable to prove the cut.
func TestT20S1NoEgressHasNoNetworkInterface(t *testing.T) {
	requireDocker(t)
	name := containerName("t20s1-noegress")
	_ = exec.Command("docker", "rm", "-f", name).Run()
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	spec := minimalEgressSpec(name, true)
	if _, err := (dockerRuntime{}).Ensure(spec); err != nil {
		t.Fatalf("Ensure(NoEgress) minimal spec: %v", err)
	}

	ifaces := netInterfaces(t, name)
	if hasNonLoopback(ifaces) {
		t.Errorf("NoEgress container has a non-loopback interface %v — egress cut FAILED; want only [lo]", ifaces)
	}

	// Secondary (tolerant): an outbound connect attempt must fail — there is no
	// route out. This is corroborating, not the anchor: the interface check above
	// is the decisive proof. Any non-zero exit is a pass; a zero exit would mean
	// the connect somehow succeeded (egress not cut).
	out, err := exec.Command("docker", "exec", name, "sh", "-c", "cat </dev/tcp/1.1.1.1/53").CombinedOutput()
	if err == nil {
		t.Errorf("NoEgress container reached 1.1.1.1:53 — egress NOT cut: %s", out)
	}
}

// TestT20S1DefaultHasEgressInterface is the control: the SAME minimal spec with
// NoEgress: false gets Docker's default bridge, so /sys/class/net DOES contain a
// non-loopback interface (eth0). This proves the canary discriminates — the
// interface check distinguishes a no-egress container from a default one, so the
// no-egress assertion above is meaningful rather than vacuously true.
func TestT20S1DefaultHasEgressInterface(t *testing.T) {
	requireDocker(t)
	name := containerName("t20s1-egress")
	_ = exec.Command("docker", "rm", "-f", name).Run()
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	spec := minimalEgressSpec(name, false)
	if _, err := (dockerRuntime{}).Ensure(spec); err != nil {
		t.Fatalf("Ensure(default) minimal spec: %v", err)
	}

	ifaces := netInterfaces(t, name)
	if !hasNonLoopback(ifaces) {
		t.Errorf("default container has no non-loopback interface %v — the canary cannot discriminate; want eth0 present", ifaces)
	}
}
