package engine

import (
	"errors"
	"testing"

	"github.com/iVersatile/loom/internal/playbook"
)

// TestEgressClaimOK pins container:egress (T20 S2a, ADR-0028): for egress: none the
// container must have ONLY loopback (a non-loopback interface means the cut FAILED,
// fail-closed); off/unset is a no-op pass (full egress is the default, nothing to
// verify).
func TestEgressClaimOK(t *testing.T) {
	cases := []struct {
		name   string
		egress string
		ifaces []string
		want   bool
	}{
		{"none only-loopback passes", playbook.EgressNone, []string{"lo"}, true},
		{"none with eth0 fails", playbook.EgressNone, []string{"lo", "eth0"}, false},
		{"none empty interfaces passes", playbook.EgressNone, nil, true}, // no non-loopback = cut holds
		{"off is a no-op pass", playbook.EgressOff, []string{"lo", "eth0"}, true},
		{"unset is a no-op pass", "", []string{"lo", "eth0"}, true},
	}
	for _, c := range cases {
		if got, _ := egressClaimOK(c.egress, c.ifaces); got != c.want {
			t.Errorf("%s: egressClaimOK(%q,%v)=%t want %t", c.name, c.egress, c.ifaces, got, c.want)
		}
	}
}

// TestContainerEgressCheckNone pins the live none path: a container with only
// loopback passes; one with a non-loopback interface FAILS; an unreadable probe
// fails closed (never silently passes a declared cut it could not confirm).
func TestContainerEgressCheckNone(t *testing.T) {
	pb := playbook.Playbook{Name: "demo", Networking: &playbook.Networking{Egress: playbook.EgressNone}}

	// Only loopback ⇒ the cut holds.
	ok := containerEgressCheck(fakeRuntime{netIfaces: []string{"lo"}}, "demo-dev", pb)
	if ok.Name != "container:egress" || !ok.OK {
		t.Errorf("only-loopback should pass: %+v", ok)
	}

	// A non-loopback interface ⇒ the cut failed.
	bad := containerEgressCheck(fakeRuntime{netIfaces: []string{"lo", "eth0"}}, "demo-dev", pb)
	if bad.OK {
		t.Errorf("eth0 present should FAIL the egress: none claim (detail: %s)", bad.Detail)
	}

	// Probe error ⇒ fail closed.
	errd := containerEgressCheck(fakeRuntime{netIfacesErr: errors.New("no such container")}, "demo-dev", pb)
	if errd.OK {
		t.Errorf("an unreadable interface list must fail container:egress, not pass silently (detail: %s)", errd.Detail)
	}
}

// TestContainerEgressCheckOffNoProbe pins the off/unset no-op pass: it passes
// WITHOUT probing the container (the runtime's NetInterfaces is never consulted —
// a runtime that errors on it still yields a pass).
func TestContainerEgressCheckOffNoProbe(t *testing.T) {
	probeWouldFail := fakeRuntime{netIfacesErr: errors.New("must not be called")}
	for _, pb := range []playbook.Playbook{
		{Name: "demo"}, // unset = off
		{Name: "demo", Networking: &playbook.Networking{Egress: playbook.EgressOff}},
	} {
		c := containerEgressCheck(probeWouldFail, "demo-dev", pb)
		if !c.OK {
			t.Errorf("off/unset must pass without a probe: %+v", c)
		}
	}
}
