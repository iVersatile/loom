package cli

import (
	"github.com/iVersatile/loom/internal/engine"
	"github.com/spf13/cobra"
)

func newBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Materialize the playbook: resolve → lockfile → container + config",
	}
	cmd.Flags().Bool("force", false, "rebuild from scratch")
	// --stack/--overlay apply to the first scaffold only; thereafter the project
	// playbook is the source of truth (docs/SPEC-verbs.md "build").
	cmd.Flags().String("stack", "", "stack for first scaffold only")
	cmd.Flags().String("overlay", "", "overlay for first scaffold only")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		res, err := engine.Build(engine.BuildOpts{Force: force})
		return emit(cmd, res, err)
	}
	return cmd
}
