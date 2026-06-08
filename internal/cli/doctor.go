package cli

import (
	"github.com/iVersatile/loom/internal/engine"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Self-check: tools present, hooks executable, lock consistent, guardrails active",
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		res, err := engine.Doctor(engine.DoctorOpts{})
		if e := emit(cmd, res, err); e != nil {
			return e
		}
		// A failing self-check is a non-zero exit so agents/CI can gate on it.
		if !res.OK() {
			return &exitError{code: 1}
		}
		return nil
	}
	return cmd
}
