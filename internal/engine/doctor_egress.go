package engine

import (
	"fmt"

	"github.com/iVersatile/loom/internal/playbook"
)

// container:egress verification (T20 S2a, ADR-0028). The companion to the
// networking: schema joint: a container whose playbook declares `egress: none`
// must ACTUALLY have no external egress interface (only `lo`), so the declared
// posture and the realized container agree — "doctor mechanizes what the reviewer
// hand-checks once" (the same doctrine as container:role-marker-perms). off/unset
// is a no-op pass: full egress is the Phase-1 default, nothing to verify. The probe
// is read-only and hermetic (it reads /sys/class/net inside the container — no
// external host need be reachable); the decision logic is the pure function below
// (gate-tested), the probe is integration-validated. allowlist never reaches here
// (validate fail-closes it as unimplemented — S2b).

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
	return true, fmt.Sprintf("egress: none — container has only loopback %v (no external egress, --network none)", ifaces)
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
