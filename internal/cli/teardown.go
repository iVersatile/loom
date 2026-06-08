package cli

import (
	"fmt"

	"github.com/iVersatile/loom/internal/engine"
	"github.com/spf13/cobra"
)

var teardownLevels = map[string]bool{"stop": true, "volumes": true, "reset": true}

func newTeardownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "teardown [stop|volumes|reset]",
		Short: "Tear down the environment in tiers (container / +volumes / +image)",
		Args:  cobra.MaximumNArgs(1),
	}
	// Mac-side opt-ins; --wipe-project requires typed confirmation that --yes
	// cannot bypass (docs/SPEC-verbs.md "teardown").
	cmd.Flags().Bool("clean-state", false, "also remove Mac-side agent auth, memory, logs")
	cmd.Flags().Bool("wipe-project", false, "remove the whole project folder (typed confirmation)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		level := "stop"
		if len(args) == 1 {
			level = args[0]
		}
		if !teardownLevels[level] {
			return fmt.Errorf("invalid level %q: want stop, volumes, or reset", level)
		}
		cleanState, _ := cmd.Flags().GetBool("clean-state")
		wipe, _ := cmd.Flags().GetBool("wipe-project")
		res, err := engine.Teardown(engine.TeardownOpts{
			PlaybookPath: playbookPath(cmd),
			Level:        level,
			CleanState:   cleanState,
			WipeProject:  wipe,
		})
		return emit(cmd, res, err)
	}
	return cmd
}
