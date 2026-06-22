package engine

import (
	"errors"
	"slices"
	"testing"

	"github.com/iVersatile/loom/internal/playbook"
)

// TestResolveEgressAllowlist pins the runtime allowlist resolution (T20 S2b,
// ADR-0028 Amendment 1): the load-bearing floor is ALWAYS unioned in (a project
// can never BRICK its agent by forgetting the model endpoint), the union is
// deduped and sorted (deterministic for the proxy ALLOW env and for tests), and
// an empty declared list still yields exactly the floor.
func TestResolveEgressAllowlist(t *testing.T) {
	// The load-bearing floor is always present, regardless of what is declared.
	got := resolveEgressAllowlist([]string{"example.com"})
	for _, must := range loadBearingEgressHosts {
		if !slices.Contains(got, must) {
			t.Errorf("resolved allowlist %v missing load-bearing host %q", got, must)
		}
	}
	if !slices.Contains(got, "example.com") {
		t.Errorf("resolved allowlist %v missing declared host example.com", got)
	}

	// Sorted + deterministic.
	if !slices.IsSorted(got) {
		t.Errorf("resolved allowlist must be sorted, got %v", got)
	}

	// Dedup: a declared host already in the floor is not duplicated.
	withDup := resolveEgressAllowlist([]string{"github.com", "github.com", "example.com"})
	count := 0
	for _, h := range withDup {
		if h == "github.com" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("github.com (in floor + declared twice) must appear once, got %d in %v", count, withDup)
	}

	// Empty declared still yields the floor (the fail-safe).
	floor := resolveEgressAllowlist(nil)
	if len(floor) != len(loadBearingEgressHosts) {
		t.Errorf("empty declared should yield exactly the floor (%d hosts), got %v", len(loadBearingEgressHosts), floor)
	}
	for _, must := range loadBearingEgressHosts {
		if !slices.Contains(floor, must) {
			t.Errorf("empty-declared floor %v missing %q", floor, must)
		}
	}

	// Determinism: same input ⇒ identical output across calls.
	if !slices.Equal(resolveEgressAllowlist([]string{"b.com", "a.com"}), resolveEgressAllowlist([]string{"a.com", "b.com"})) {
		t.Error("resolveEgressAllowlist must be order-independent and deterministic")
	}
}

// TestEgressPolicyMapping pins egressPolicy (build.go): networking.egress:
// allowlist + allow ⇒ EgressPolicy{allowlist, allow}; nil networking ⇒ zero
// policy; isAllowlist() is true only for allowlist.
func TestEgressPolicyMapping(t *testing.T) {
	allowPB := &playbook.Playbook{Networking: &playbook.Networking{
		Egress: playbook.EgressAllowlist, Allow: []string{"example.com"}}}
	p := egressPolicy(allowPB)
	if p.Posture != playbook.EgressAllowlist {
		t.Errorf("allowlist posture = %q, want %q", p.Posture, playbook.EgressAllowlist)
	}
	if !slices.Equal(p.Allow, []string{"example.com"}) {
		t.Errorf("allow = %v, want [example.com]", p.Allow)
	}
	if !p.isAllowlist() {
		t.Error("allowlist policy must report isAllowlist()=true")
	}

	// nil networking ⇒ zero policy, not an allowlist.
	zero := egressPolicy(&playbook.Playbook{})
	if zero.Posture != "" || zero.Allow != nil {
		t.Errorf("nil networking should yield zero EgressPolicy, got %+v", zero)
	}
	if zero.isAllowlist() {
		t.Error("zero EgressPolicy must not report isAllowlist()")
	}

	// none/off are not allowlist.
	for _, posture := range []string{playbook.EgressNone, playbook.EgressOff} {
		pp := egressPolicy(&playbook.Playbook{Networking: &playbook.Networking{Egress: posture}})
		if pp.isAllowlist() {
			t.Errorf("posture %q must not report isAllowlist()", posture)
		}
	}
}

// TestEgressAllowlistClaimOK pins the doctor allowlist decision (T20 S2b): the
// confinement holds iff the container's network set is EXACTLY {intNet} with the
// sidecar running. A missing sidecar, a bridge attachment, ANY unexpected
// additional network (defense-in-depth — a second routable net of any name), an
// off-internal-net container, or an unreadable probe all FAIL closed.
func TestEgressAllowlistClaimOK(t *testing.T) {
	const intNet = "demo-dev-egress"
	cases := []struct {
		name      string
		networks  []string
		netErr    error
		sidecarUp bool
		want      bool
	}{
		{"exactly {intNet} + sidecar running passes", []string{intNet}, nil, true, true},
		{"missing sidecar fails (broken confinement)", []string{intNet}, nil, false, false},
		{"on bridge fails (route out bypasses proxy)", []string{intNet, "bridge"}, nil, true, false},
		{"intNet + some other net fails (defense-in-depth)", []string{intNet, "some-other-net"}, nil, true, false},
		{"not on internal net fails", []string{"bridge"}, nil, true, false},
		{"some other network only fails", []string{"demo-dev-egress-ext"}, nil, true, false},
		{"unreadable probe fails closed", nil, errors.New("no such container"), true, false},
		{"empty networks fails closed", nil, nil, true, false},
	}
	for _, c := range cases {
		got, detail := egressAllowlistClaimOK(intNet, c.networks, c.netErr, c.sidecarUp)
		if got != c.want {
			t.Errorf("%s: egressAllowlistClaimOK(%v, err=%v, sidecar=%t)=%t want %t (detail: %s)",
				c.name, c.networks, c.netErr, c.sidecarUp, got, c.want, detail)
		}
	}
}

// TestEgressProxyRunEnvArgs pins the proxy-env wiring: only an allowlist container
// gets HTTP(S)_PROXY pointed at its sidecar; provision clears it
// (provision-then-restrict). none/off carry no proxy env.
func TestEgressProxyRunEnvArgs(t *testing.T) {
	allow := EgressPolicy{Posture: playbook.EgressAllowlist, Allow: []string{"example.com"}}
	args := egressProxyRunEnvArgs("demo-dev", allow)
	if len(args) != len(egressProxyEnvNames)*2 {
		t.Fatalf("allowlist should set %d proxy env args, got %v", len(egressProxyEnvNames)*2, args)
	}
	wantURL := egressProxyURL("demo-dev")
	for i := 0; i < len(args); i += 2 {
		if args[i] != "-e" {
			t.Errorf("arg %d = %q, want -e", i, args[i])
		}
		// every value must point at the sidecar proxy URL
		val := args[i+1]
		if !slices.Contains(egressProxyEnvNames, val[:len(val)-len("="+wantURL)]) {
			t.Errorf("unexpected env var in %q", val)
		}
		if val[len(val)-len(wantURL):] != wantURL {
			t.Errorf("env %q does not point at proxy URL %q", val, wantURL)
		}
	}

	// none/off ⇒ no proxy env.
	for _, posture := range []string{"", playbook.EgressOff, playbook.EgressNone} {
		if got := egressProxyRunEnvArgs("demo-dev", EgressPolicy{Posture: posture}); got != nil {
			t.Errorf("posture %q must yield no proxy env, got %v", posture, got)
		}
	}

	// provision-then-restrict: clearing blanks every proxy env var name; not
	// clearing yields nothing.
	cleared := provisionEnvArgs(true)
	if len(cleared) != len(egressProxyEnvNames)*2 {
		t.Fatalf("clearProxyEnv should blank %d env args, got %v", len(egressProxyEnvNames)*2, cleared)
	}
	for i := 0; i < len(cleared); i += 2 {
		if cleared[i] != "-e" {
			t.Errorf("clear arg %d = %q, want -e", i, cleared[i])
		}
		if cleared[i+1][len(cleared[i+1])-1] != '=' {
			t.Errorf("clear arg %q must end in = (blank value)", cleared[i+1])
		}
	}
	if got := provisionEnvArgs(false); got != nil {
		t.Errorf("no-clear should yield nothing, got %v", got)
	}
}
