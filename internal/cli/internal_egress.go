package cli

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/iVersatile/loom/internal/egress"
	"github.com/spf13/cobra"
)

// HIDDEN internal subcommands (the `__` prefix marks them not-a-user-verb). They
// are deliberately NOT in SPEC-verbs.md and NOT in the conformance verb list — so
// SpecConformance (which iterates a fixed list keyed by SPEC sections, not the
// cobra tree) never sees them. They are also Hidden:true so they never appear in
// `loom help`/usage. They exist only for loom-to-sidecar plumbing (DELIVERY A,
// T20 S2b): the engine `docker cp`s the loom binary into the egress sidecar and
// project container and invokes these, so neither image needs Go or curl.

// newEgressProxyCmd is `loom __egress-proxy`: run the egress gatekeeper in the
// sidecar. Reads ALLOW (comma-split hostnames) and LISTEN (e.g. ":8080") from the
// environment — set by the engine's `docker exec -e ...` — and blocks in
// egress.RunProxy. It is the in-binary replacement for the old `go run`-the-
// embedded-source delivery, so the sidecar image needs no Go toolchain.
func newEgressProxyCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "__egress-proxy",
		Short:  "internal: run the egress allowlist proxy (sidecar use only)",
		Hidden: true,
		Args:   cobra.NoArgs,
		// Silence usage/errors: this runs detached inside a container; a cobra
		// usage dump on stderr would be noise in `docker logs <sidecar>`.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var allow []string
			for _, h := range strings.Split(os.Getenv("ALLOW"), ",") {
				if h = strings.TrimSpace(h); h != "" {
					allow = append(allow, h)
				}
			}
			return egress.RunProxy(allow, os.Getenv("LISTEN"))
		},
	}
}

// newHTTPGetCmd is `loom __http-get <url>`: a tiny self-contained HTTP probe used
// by the integration canary in place of curl (so the project container's base
// image needs no curl). It performs a GET and prints the HTTP status code on a
// clean transfer (even a 403 is a clean transfer — the proxy answered), or exits
// non-zero on a transport error (the BYPASS-DEAD path: no route out).
//
// By default it honors HTTP(S)_PROXY via http.ProxyFromEnvironment (so the engine-
// set proxy env routes the probe through the sidecar). --no-proxy forces a
// transport with Proxy:nil for the direct-bypass assertion (which must FAIL on the
// internal-only network).
func newHTTPGetCmd() *cobra.Command {
	var noProxy bool
	cmd := &cobra.Command{
		Use:           "__http-get <url>",
		Short:         "internal: HTTP GET probe printing the status code (canary use only)",
		Hidden:        true,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			tr := &http.Transport{Proxy: http.ProxyFromEnvironment}
			if noProxy {
				tr.Proxy = nil // direct: ignore HTTP(S)_PROXY entirely (bypass test)
			}
			client := &http.Client{Transport: tr}
			resp, err := client.Get(args[0])
			if err != nil {
				// Transport error (no route / refused / proxy 403 surfaces as a body,
				// not an error). Non-zero exit; the canary keys on this.
				return fmt.Errorf("http-get: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), resp.StatusCode)
			return nil
		},
	}
	cmd.Flags().BoolVar(&noProxy, "no-proxy", false, "ignore HTTP(S)_PROXY env (direct connection — bypass test)")
	return cmd
}
