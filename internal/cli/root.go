// Package cli wires the verb contract surface: the cobra command tree, global
// flags (--json), and exit-code mapping. It owns no engine logic —
// each command translates flags to an engine call and renders the result
// (docs/SPEC-verbs.md). This separation keeps the engine reusable by the future
// cloud sibling (ADR-0007).
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/iVersatile/loom/internal/engine"
	"github.com/iVersatile/loom/internal/render"
	"github.com/spf13/cobra"
)

// exitError carries a specific process exit code up to Execute. Used for the
// load-bearing plan check-mode code (2 = changes needed) and doctor failures.
type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("exit code %d", e.code) }

// Execute runs the command tree and maps the outcome to a process exit code:
// 0 success/no-op, 1 error, 2 "changes needed" (docs/SPEC-verbs.md).
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			return ee.code
		}
		fmt.Fprintln(os.Stderr, "loom: "+err.Error())
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "loom",
		Short:         "Loom reconciles a dev environment to a declarative playbook",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// Global flags inherited by every verb (RULES §5: --json on every verb).
	root.PersistentFlags().Bool("json", false, "machine-readable JSON on stdout; logs on stderr")
	root.PersistentFlags().StringP("playbook", "f", "loom.yml", "path to the project playbook")
	root.AddCommand(
		newDetectCmd(),
		newPlanCmd(),
		newBuildCmd(),
		newStartCmd(),
		newTeardownCmd(),
		newDoctorCmd(),
		newExecCmd(),
		newShellCmd(),
		newImportCmd(),
		// HIDDEN internal subcommands (`__` prefix): loom-to-sidecar/probe plumbing
		// for the egress allowlist (DELIVERY A, T20 S2b). Not user verbs — Hidden
		// from help and deliberately absent from SPEC-verbs.md / SpecConformance.
		newEgressProxyCmd(),
		newHTTPGetCmd(),
	)
	return root
}

func jsonMode(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("json")
	return v
}

func playbookPath(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("playbook")
	return v
}

// emit renders a verb result, then handles a stub sentinel by noting it on
// stderr (honest about unimplemented verbs without faking a failure). Any other
// error propagates to Execute as a code-1 failure.
func emit(cmd *cobra.Command, v render.Renderable, err error) error {
	if e := render.Emit(cmd.OutOrStdout(), jsonMode(cmd), v); e != nil {
		return e
	}
	if errors.Is(err, engine.ErrNotImplemented) {
		// Best-effort stderr note; a failed stderr write is not actionable.
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "loom: %s: %v\n", cmd.Name(), err)
		return nil
	}
	return err
}
