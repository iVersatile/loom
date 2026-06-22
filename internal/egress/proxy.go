// Package egress holds loom's shipped egress gatekeeper — a tiny, stdlib-only
// HTTP forward proxy that confines a project container's outbound traffic to a
// per-HOSTNAME allowlist (T20 S2b, ADR-0028 Amendment 1).
//
// MECHANISM, not trust (ADR-0028 §3 / Amendment 1 R1). The project container is
// placed on an `--internal` docker network with NO route off-box; this proxy
// sits on BOTH that internal network and a normal bridge, so it is the project
// container's ONLY path to the internet. A raw socket to a non-allowlisted host
// (the `go test` / `python3` / `/dev/tcp` exfil path T20 exists to close) has
// nowhere to go — the route is the fence, the HTTP(S)_PROXY env is only clean
// CONNECT semantics for cooperating clients.
//
// It is an EXPLICIT-CONNECT proxy: it never decrypts TLS (no MITM CA), so the
// allowlist match is by the hostname the client self-describes on the
// `CONNECT host:443` line (or the plain-HTTP proxy request line). This is the
// proven shape from the #249 real-docker proof (TestT20S2bProxyEgressAllowlist).
//
// DELIVERY A (this rework): the proxy is a PACKAGE (not a standalone `main`
// embedded as source and `go run` in the sidecar). The engine runs it inside the
// sidecar via the loom binary's hidden `__egress-proxy` subcommand — so the
// sidecar image needs NO Go toolchain (the previous delivery required Go in the
// base image; debian:bookworm-slim and most project bases lack it). The behavior
// is unchanged: explicit-CONNECT + plain-HTTP forward, allow-by-hostname,
// deny→403, stderr decision logs.
package egress

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

// hostOnly strips any ":port" suffix and returns the bare hostname.
func hostOnly(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return hostport
}

// proxy is the per-run gatekeeper: an allow-set plus the HTTP handler closures
// that consult it. Holding the set on a value (not a package global) keeps
// RunProxy free of hidden state and makes the allow/deny logic unit-testable.
type proxy struct {
	allowed map[string]bool
}

// newProxy builds a gatekeeper from a hostname allowlist (port stripped, blanks
// dropped, deduped via the set).
func newProxy(allow []string) *proxy {
	p := &proxy{allowed: map[string]bool{}}
	for _, h := range allow {
		if h = strings.TrimSpace(h); h != "" {
			p.allowed[hostOnly(h)] = true
		}
	}
	return p
}

// allows reports whether a hostport's bare hostname is on the allowlist. This is
// the single decision point both handlers (and the unit test) consult.
func (p *proxy) allows(hostport string) bool {
	return p.allowed[hostOnly(hostport)]
}

// RunProxy starts the egress gatekeeper, listening on `listen` (e.g. ":8080")
// and permitting outbound only to the hostnames in `allow`. It blocks, returning
// the ListenAndServe error. Decisions are logged to the default logger (stderr,
// surfaced by `docker logs <sidecar>`) — the observe-then-enforce surface
// Amendment 1 R3 calls for.
func RunProxy(allow []string, listen string) error {
	if listen == "" {
		listen = ":8080"
	}
	p := newProxy(allow)
	log.Printf("egressproxy: listen=%s allow=%v", listen, keys(p.allowed))
	srv := &http.Server{Addr: listen, Handler: http.HandlerFunc(p.handle)}
	return srv.ListenAndServe()
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// handle dispatches forward-proxy requests: CONNECT for HTTPS tunnels, anything
// else as a plain-HTTP forward request.
func (p *proxy) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}
	p.handleHTTP(w, r)
}

// handleHTTP forwards a plain-HTTP proxy request (client line is
// `GET http://host/path`): allow → forward to origin and copy the response;
// deny → 403 with a clear body.
func (p *proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	host := hostOnly(r.Host)
	if !p.allows(r.Host) {
		log.Printf("egressproxy: DENY http host=%q", host)
		http.Error(w, fmt.Sprintf("egress denied: %s not in allowlist", host), http.StatusForbidden)
		return
	}
	log.Printf("egressproxy: ALLOW http host=%q url=%q", host, r.RequestURI)

	// Build a fresh outbound request to the origin. RequestURI carries the
	// absolute form in a proxy request, so re-use r.URL directly.
	outReq, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, "bad proxy request: "+err.Error(), http.StatusBadGateway)
		return
	}
	outReq.Header = r.Header.Clone()
	resp, err := http.DefaultTransport.RoundTrip(outReq)
	if err != nil {
		log.Printf("egressproxy: upstream error host=%q: %v", host, err)
		http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// handleConnect tunnels HTTPS: the client sends `CONNECT host:443`; allow →
// dial the origin, reply 200, and bidirectionally copy; deny → 403. No TLS is
// decrypted (the hostname is self-described by the CONNECT line).
func (p *proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	host := hostOnly(r.Host)
	if !p.allows(r.Host) {
		log.Printf("egressproxy: DENY connect host=%q", host)
		http.Error(w, fmt.Sprintf("egress denied: %s not in allowlist", host), http.StatusForbidden)
		return
	}
	log.Printf("egressproxy: ALLOW connect host=%q target=%q", host, r.Host)

	upstream, err := net.Dial("tcp", r.Host)
	if err != nil {
		log.Printf("egressproxy: dial error host=%q: %v", host, err)
		http.Error(w, "dial error: "+err.Error(), http.StatusBadGateway)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack unsupported", http.StatusInternalServerError)
		_ = upstream.Close()
		return
	}
	client, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, "hijack error: "+err.Error(), http.StatusInternalServerError)
		_ = upstream.Close()
		return
	}
	_, _ = client.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))

	// Bidirectional copy; close both ends when either direction ends.
	go func() { _, _ = io.Copy(upstream, client); _ = upstream.Close() }()
	_, _ = io.Copy(client, upstream)
	_ = client.Close()
}
