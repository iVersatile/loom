package egress

import "testing"

// TestProxyAllowDeny pins the gatekeeper's core decision (T20 S2b, delivery A):
// allow-by-bare-hostname (port stripped), deny everything else, blanks dropped.
// This is the logic the sidecar enforces; the live tunnel/403 behavior is the
// integration canary's job.
func TestProxyAllowDeny(t *testing.T) {
	p := newProxy([]string{"example.com", " ", "", "github.com:443"})

	cases := []struct {
		hostport string
		want     bool
	}{
		{"example.com", true},      // declared, no port
		{"example.com:443", true},  // declared, port stripped before match
		{"example.com:80", true},   // any port for an allowed host
		{"github.com", true},       // declared WITH a port → matched bare
		{"github.com:443", true},   // and with that port
		{"example.org", false},     // not declared → deny
		{"example.org:443", false}, // not declared, with port → deny
		{"sub.example.com", false}, // no implicit subdomain allow
		{"", false},                // empty → deny
	}
	for _, c := range cases {
		if got := p.allows(c.hostport); got != c.want {
			t.Errorf("allows(%q) = %t, want %t", c.hostport, got, c.want)
		}
	}
}

// TestHostOnly pins the port-strip helper both handlers and allows() rely on.
func TestHostOnly(t *testing.T) {
	for in, want := range map[string]string{
		"example.com:443": "example.com",
		"example.com":     "example.com",
		"10.0.0.1:8080":   "10.0.0.1",
	} {
		if got := hostOnly(in); got != want {
			t.Errorf("hostOnly(%q) = %q, want %q", in, got, want)
		}
	}
}
