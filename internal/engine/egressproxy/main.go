// Command egressproxy is loom's shipped egress gatekeeper (T20 S2b, ADR-0028
// Amendment 1). It is a tiny, stdlib-only HTTP forward proxy that confines a
// project container's outbound traffic to a per-HOSTNAME allowlist.
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
// proven shape from the #249 real-docker proof (TestT20S2bProxyEgressAllowlist),
// promoted here to a loom-shipped component the engine embeds and runs in the
// sidecar (`go run` against the base image's baked Go toolchain, ADR-0012).
//
// It is deliberately minimal and well-logged so every allow/deny decision is
// visible on stderr (`docker logs <sidecar>`) — the observe-then-enforce surface
// Amendment 1 R3 calls for.
package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

// allowed holds the set of permitted hostnames (no port), read from ALLOW.
var allowed = map[string]bool{}

// hostOnly strips any ":port" suffix and returns the bare hostname.
func hostOnly(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return hostport
}

func main() {
	listen := os.Getenv("LISTEN")
	if listen == "" {
		listen = ":8080"
	}
	for _, h := range strings.Split(os.Getenv("ALLOW"), ",") {
		if h = strings.TrimSpace(h); h != "" {
			allowed[h] = true
		}
	}
	log.SetOutput(os.Stderr)
	log.Printf("egressproxy: listen=%s allow=%v", listen, keys(allowed))

	srv := &http.Server{Addr: listen, Handler: http.HandlerFunc(handle)}
	log.Fatal(srv.ListenAndServe())
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
func handle(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		handleConnect(w, r)
		return
	}
	handleHTTP(w, r)
}

// handleHTTP forwards a plain-HTTP proxy request (client line is
// `GET http://host/path`): allow → forward to origin and copy the response;
// deny → 403 with a clear body.
func handleHTTP(w http.ResponseWriter, r *http.Request) {
	host := hostOnly(r.Host)
	if !allowed[host] {
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
func handleConnect(w http.ResponseWriter, r *http.Request) {
	host := hostOnly(r.Host)
	if !allowed[host] {
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
