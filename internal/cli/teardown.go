package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/iVersatile/loom/internal/engine"
	"github.com/spf13/cobra"
)

var teardownLevels = map[string]bool{"stop": true, "volumes": true, "reset": true}

func newTeardownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "teardown [stop|volumes|reset]",
		Short: "Tear down the environment in tiers (container / +volumes / +image)",
		Long: `Removes the ENVIRONMENT in tiers: stop (container), volumes (+state
volumes), reset (+image). The repo is not teardown's domain: project files —
including loom.lock, which records resolved intent for the next build to
re-converge from — survive every tier by design; only --wipe-project removes
project files, behind a typed confirmation that --yes cannot bypass.

The most destructive routine verb confirms before acting (guided-run finding
⑨): a [y/N] prompt unless --yes states intent for automation; on silence (EOF,
empty, anything else) it removes nothing. There is no --force.`,
		Args: cobra.MaximumNArgs(1),
	}
	// Mac-side opt-ins; --wipe-project requires typed confirmation that --yes
	// cannot bypass (docs/SPEC-verbs.md "teardown").
	cmd.Flags().Bool("clean-state", false, "also remove Mac-side agent auth, memory, logs")
	cmd.Flags().Bool("wipe-project", false, "remove the whole project folder (typed confirmation)")
	cmd.Flags().Bool("yes", false, "skip the interactive confirmation (automation)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		level := "stop"
		if len(args) == 1 {
			level = args[0]
		}
		if !teardownLevels[level] {
			return fmt.Errorf("invalid level %q: want stop, volumes, or reset", level)
		}
		// Consent gate (FR-TEARDOWN-003): an operator answers the prompt,
		// automation states intent with --yes, and on silence (EOF — e.g.
		// stdin is /dev/null) teardown removes nothing — never-auto applies
		// to the most destructive verb first. No isatty: an unanswerable
		// prompt aborts just the same, so detection buys nothing.
		if yes, _ := cmd.Flags().GetBool("yes"); !yes {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "teardown level %q removes the environment (repo and lock survive). Proceed? [y/N] ", level)
			line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			switch strings.ToLower(strings.TrimSpace(line)) {
			case "y", "yes":
			default:
				return fmt.Errorf("teardown aborted, nothing removed (unattended runs pass --yes)")
			}
		}
		cleanState, _ := cmd.Flags().GetBool("clean-state")
		wipe, _ := cmd.Flags().GetBool("wipe-project")
		res, err := engine.Teardown(engine.TeardownOpts{
			PlaybookPath: playbookPath(cmd),
			Level:        level,
			CleanState:   cleanState,
			WipeProject:  wipe,
		})
		out := emit(cmd, res, err)
		if res.LogPath != "" {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "loom: teardown log: %s\n", res.LogPath)
		}
		return out
	}
	return cmd
}
