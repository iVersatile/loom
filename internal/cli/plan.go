package cli

import (
	"github.com/iVersatile/loom/internal/engine"
	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Compute the diff between current state and the playbook (never mutates)",
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		res, err := engine.Plan(engine.PlanOpts{PlaybookPath: playbookPath(cmd)})
		if e := emit(cmd, res, err); e != nil {
			return e
		}
		// Check-mode exit code: 2 when changes are needed so CI/agents can gate
		// on drift; 0 when already converged (docs/SPEC-verbs.md "plan").
		if res.Changed() {
			return &exitError{code: 2}
		}
		return nil
	}
	return cmd
}
