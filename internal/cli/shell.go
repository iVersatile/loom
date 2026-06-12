package cli

import (
	"github.com/iVersatile/loom/internal/engine"
	"github.com/spf13/cobra"
)

// newShellCmd wires the shell verb (docs/SPEC-verbs.md "shell"): an
// interactive login shell in the project container — sugar over exec's
// engine path with a TTY. No --json (interactive; spec exemption) and no
// emit(): the session owns the terminal, loom authors only error text.
func newShellCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Interactive login shell in the project container (audited session-open; commands not captured)",
		Args:  cobra.NoArgs,
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		res, err := engine.Shell(engine.ShellOpts{PlaybookPath: playbookPath(cmd)})
		if err != nil {
			return err
		}
		// Exits with the shell's exit code (SPEC-verbs shell), silently —
		// same verbatim propagation as exec.
		if res.ExitCode != 0 {
			return &exitError{code: res.ExitCode}
		}
		return nil
	}
	return cmd
}
