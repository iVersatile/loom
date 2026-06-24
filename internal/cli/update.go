package cli

import (
	"fmt"

	"github.com/iVersatile/loom/internal/engine"
	"github.com/spf13/cobra"
)

// newUpdateCmd wires `loom update` — reconcile a running env to the current
// playbook by applying the plan delta (docs/SPEC-verbs.md "update"). Mirrors
// build's wiring: --json is the persistent root flag, emit renders human/json,
// and warnings + the diagnostic log path ride stderr.
func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Reconcile a running environment to the current playbook (apply the delta)",
	}
	cmd.Flags().Bool("force", false, "rebuild from scratch while applying the delta")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		res, err := engine.Update(engine.UpdateOpts{PlaybookPath: playbookPath(cmd), Force: force})
		out := emit(cmd, res, err)
		for _, w := range res.Warnings {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "loom: warning: %s\n", w)
		}
		if res.LogPath != "" {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "loom: update log: %s\n", res.LogPath)
		}
		return out
	}
	return cmd
}
