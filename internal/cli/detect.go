package cli

import (
	"fmt"

	"github.com/iVersatile/loom/internal/engine"
	"github.com/spf13/cobra"
)

func newDetectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Read current reality (never mutates)",
	}
	cmd.Flags().Bool("emit-playbook", false, "write a draft base playbook from the detected machine (continuity / carry-forward)")
	cmd.Flags().Bool("migrate", false, "consolidate detected credentials into .env, with confirmation (Phase 2)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		emitPB, _ := cmd.Flags().GetBool("emit-playbook")
		migrate, _ := cmd.Flags().GetBool("migrate")
		res, err := engine.Detect(engine.DetectOpts{
			PlaybookPath: playbookPath(cmd),
			EmitPlaybook: emitPB,
			Migrate:      migrate,
		})
		// The draft path rides stderr (not the frozen --json document), so an
		// agent driving `detect --json --emit-playbook` can still find the file.
		if res.Emitted != "" {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "loom: detect: draft playbook written to %s — review, then commit\n", res.Emitted)
		}
		return emit(cmd, res, err)
	}
	return cmd
}
