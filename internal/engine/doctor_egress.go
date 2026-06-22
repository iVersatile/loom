package engine

import (
	"fmt"

	"github.com/iVersatile/loom/internal/playbook"
)

// container:egress verification (T20 S2a/S2b, ADR-0028 + Amendment 1). The
// companion to the networking: schema joint: a container's declared `egress:`
// posture must ACTUALLY hold on the realized container — "doctor mechanizes what
// the reviewer hand-checks once" (the same doctrine as container:role-marker-perms).
//   - none: the container must have ONLY loopback (the --network none cut, S2a).
//   - allowlist (S2b): the container must be on its internal egress network and
//     NOT on a bridge with a route out, and its gatekeeper sidecar must be running —
//     so the proxy confinement actually holds (a missing sidecar means NO route out
//     at all: a broken confinement, fail-closed).
//   - off/unset: a no-op pass (full egress is the Phase-1 default, nothing to verify).
// The probes are read-only (interfaces, the container's networks, sidecar state);
// the decision logic is the pure functions below (gate-tested), the probe is
// integration-validated. Fail-closed on unreadable probes (a security claim must
// never green on a garbage answer).

// hasNonLoopbackIface reports whether any interface but loopback is present — the
// discriminator between a no-egress container (only `lo`) and one on a network.
func hasNonLoopbackIface(ifaces []string) bool {
	for _, i := range ifaces {
		if i != "lo" {
			return true
		}
	}
	return false
}

// hasLoopbackIface reports whether loopback is present. Every real container has
// `lo`, so its ABSENCE means a degenerate probe result (empty/garbage) — which must
// fail the `none` claim closed rather than green on nothing.
func hasLoopbackIface(ifaces []string) bool {
	for _, i := range ifaces {
		if i == "lo" {
			return true
		}
	}
	return false
}

// egressClaimOK decides container:egress: the declared posture must match the
// container's actual interfaces. For `none` the container must have ONLY loopback
// (a non-loopback interface means the egress cut FAILED — fail-closed). For
// off/unset it is a no-op pass (full egress is the default; nothing to verify).
func egressClaimOK(egress string, ifaces []string) (bool, string) {
	if egress != playbook.EgressNone {
		// off/unset (and any other non-none posture validate would have rejected):
		// no egress restriction declared, so there is nothing to verify.
		return true, fmt.Sprintf("egress: %s — full outbound (Phase-1 default); no egress restriction to verify", egressLabel(egress))
	}
	if hasNonLoopbackIface(ifaces) {
		return false, fmt.Sprintf("egress: none declared but container has a non-loopback interface %v — the egress cut FAILED (want only [lo])", ifaces)
	}
	// Fail-closed: a real --network none container always has `lo`. An empty/degenerate
	// interface list means the probe returned nothing usable — we cannot CONFIRM the cut,
	// so we must not green it (a doctor that passes on a garbage probe is worse than
	// useless for a security claim).
	if !hasLoopbackIface(ifaces) {
		return false, fmt.Sprintf("egress: none declared but no loopback interface in %v — the probe returned no usable interfaces; cannot confirm the egress cut (fail-closed)", ifaces)
	}
	return true, fmt.Sprintf("egress: none — container has only loopback %v (no external egress, --network none)", ifaces)
}

// onNetwork reports whether net is among the container's attached networks.
func onNetwork(networks []string, net string) bool {
	for _, n := range networks {
		if n == net {
			return true
		}
	}
	return false
}

// egressAllowlistClaimOK decides container:egress for the allowlist posture (T20
// S2b, ADR-0028 Amendment 1). The confinement holds iff the project container's
// network set is EXACTLY {intNet} — its sole internal egress network (whose
// --internal flag means NO route off-box) and NOTHING ELSE routable — AND its
// gatekeeper sidecar is running (a missing sidecar means the internal-network-only
// container has NO route out at all — a BROKEN confinement, not a safe one;
// fail-closed).
//
// Defense-in-depth: asserting the set is EXACTLY {intNet} (not merely "on intNet
// and not on bridge") rejects ANY unexpected additional network — a future bug
// that attached a second routable net (not named `bridge`) would otherwise pass
// the looser check while quietly providing a route around the proxy.
//
// Pure and probe-free so it is gate-testable (mirrors egressClaimOK); the caller
// gathers networks + sidecar state via read-only docker probes. networksErr being
// non-nil means the network probe was unreadable — fail-closed (a security claim
// must never green on a garbage answer).
func egressAllowlistClaimOK(intNet string, networks []string, networksErr error, sidecarRunning bool) (bool, string) {
	if networksErr != nil {
		return false, fmt.Sprintf("egress: allowlist declared but the container's networks are unreadable (%v) — cannot confirm the proxy confinement (fail-closed)", networksErr)
	}
	if len(networks) == 0 {
		return false, "egress: allowlist declared but the container reports no networks — the probe returned nothing usable; cannot confirm the proxy confinement (fail-closed)"
	}
	if !onNetwork(networks, intNet) {
		return false, fmt.Sprintf("egress: allowlist declared but the container is NOT on its internal egress network %q (networks %v) — the cut-over to the confined route FAILED", intNet, networks)
	}
	// Defense-in-depth: the set must be EXACTLY {intNet}. Any other attached
	// network is a route the proxy cannot see — reject it (this subsumes the old
	// "not on bridge" check and also catches a second routable net of any name).
	for _, n := range networks {
		if n != intNet {
			return false, fmt.Sprintf("egress: allowlist declared but the container is on an unexpected additional network %q (networks %v, want exactly [%s]) — that is a route out that bypasses the proxy", n, networks, intNet)
		}
	}
	if !sidecarRunning {
		return false, fmt.Sprintf("egress: allowlist declared and the container is on its internal egress network %q, but the proxy sidecar is NOT running — the container has NO route out at all (broken confinement, fail-closed)", intNet)
	}
	return true, fmt.Sprintf("egress: allowlist — container is on its internal egress network %q (no off-box route) with the proxy sidecar running (networks %v); outbound is confined to the resolved allowlist", intNet, networks)
}

// egressLabel renders the declared posture for a check detail; "" reads as "off".
func egressLabel(egress string) string {
	if egress == "" {
		return playbook.EgressOff + " (unset)"
	}
	return egress
}

// containerEgressCheck grades container:egress against a LIVE container (T20 S2a).
// Read-only: it lists the container's interfaces (/sys/class/net) and compares them
// to the declared posture — it never mutates. The caller gates on a running
// container (the probe needs one; doctor never Starts a container to ask). For
// off/unset it is a no-op pass produced without a probe.
func containerEgressCheck(rt ContainerRuntime, cname string, pb playbook.Playbook) Check {
	egress := ""
	if pb.Networking != nil {
		egress = pb.Networking.Egress
	}
	// allowlist (T20 S2b): verify the proxy-sidecar confinement on the LIVE
	// container — it must be on its internal egress network (no off-box route) and
	// NOT on a bridge, with the gatekeeper sidecar running. Probes are read-only
	// (the container's networks via inspect + the sidecar's run state); the decision
	// is the pure egressAllowlistClaimOK below. Fail-closed on an unreadable probe.
	if egress == playbook.EgressAllowlist {
		networks, nerr := containerNetworks(cname)
		ok, detail := egressAllowlistClaimOK(egressInternalNetwork(cname), networks, nerr, egressSidecarRunning(cname))
		return Check{Name: "container:egress", OK: ok, Detail: detail}
	}
	if egress != playbook.EgressNone {
		// No egress restriction declared — pass without probing the container.
		ok, detail := egressClaimOK(egress, nil)
		return Check{Name: "container:egress", OK: ok, Detail: detail}
	}
	ifaces, err := rt.NetInterfaces(cname)
	if err != nil {
		return Check{Name: "container:egress", OK: false,
			Detail: fmt.Sprintf("egress: none declared but interfaces unreadable (%v) — cannot confirm the egress cut", err)}
	}
	ok, detail := egressClaimOK(egress, ifaces)
	return Check{Name: "container:egress", OK: ok, Detail: detail}
}
