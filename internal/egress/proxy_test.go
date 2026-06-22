package egress

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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

// closedLocalAddr returns a 127.0.0.1:<port> that is guaranteed NOT listening: it
// binds an ephemeral port, captures the address, then closes the listener. Hermetic
// (loopback only, no real network) — a dial to it fails promptly with connection
// refused, the proxy's dial-timeout footgun stand-in.
func closedLocalAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("bind ephemeral port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// TestProxyConnectDialReturnsPromptly proves the CONNECT path's bounded dial: an
// ALLOWLISTED-but-unreachable upstream returns an error promptly (502) rather than
// hanging the handler goroutine + fd indefinitely. Hermetic: the "upstream" is a
// just-closed loopback port (connection refused, no real network). It also guards
// against a regression to an unbounded net.Dial by asserting the whole exchange
// finishes well under the configured dial timeout.
func TestProxyConnectDialReturnsPromptly(t *testing.T) {
	dead := closedLocalAddr(t) // allowlisted host, but nothing is listening
	p := newProxy([]string{dead})
	srv := httptest.NewServer(http.HandlerFunc(p.handle)) // httptest conns support Hijack
	defer srv.Close()

	// Raw CONNECT to the proxy targeting the dead upstream.
	conn, err := net.DialTimeout("tcp", srv.Listener.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer func() { _ = conn.Close() }()

	start := time.Now()
	_ = conn.SetDeadline(time.Now().Add(egressDialTimeout + 5*time.Second))
	if _, err := fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", dead, dead); err != nil {
		t.Fatalf("write CONNECT: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("read CONNECT response: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if elapsed := time.Since(start); elapsed >= egressDialTimeout {
		t.Errorf("CONNECT to a dead upstream took %v (>= dial timeout %v) — dial is not bounded / hangs", elapsed, egressDialTimeout)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("CONNECT to a dead allowlisted upstream: status = %d, want %d (dial error → 502)", resp.StatusCode, http.StatusBadGateway)
	}
}
