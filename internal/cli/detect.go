package cli

import (
	"github.com/iVersatile/loom/internal/engine"
	"github.com/spf13/cobra"
)

func newDetectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Read current reality (never mutates)",
	}
	cmd.Flags().Bool("emit-playbook", false, "write a draft base playbook from the detected machine (Phase 2)")
	cmd.Flags().Bool("migrate", false, "consolidate detected credentials into .env, with confirmation (Phase 2)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		emitPB, _ := cmd.Flags().GetBool("emit-playbook")
		migrate, _ := cmd.Flags().GetBool("migrate")
		res, err := engine.Detect(engine.DetectOpts{EmitPlaybook: emitPB, Migrate: migrate})
		return emit(cmd, res, err)
	}
	return cmd
}
