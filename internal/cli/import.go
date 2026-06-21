package cli

import (
	"github.com/iVersatile/loom/internal/engine"
	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <devcontainer.json>",
		Short: "Import a devcontainer.json into a draft Loom playbook (ADR-0003)",
		Args:  cobra.ExactArgs(1),
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		res, err := engine.Import(args[0])
		return emit(cmd, res, err)
	}
	return cmd
}
