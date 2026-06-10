package cli

import (
	"fmt"

	"github.com/iVersatile/loom/internal/engine"
	"github.com/spf13/cobra"
)

// newExecCmd wires the exec verb (docs/SPEC-verbs.md "exec", ADR-0016):
// transparent passthrough into the project container. No --json — exec is
// exempt from the global convention (the structured record is the audit
// entry), so there is no emit() call here; output belongs to the executed
// command, and the only loom-authored bytes are usage/error text on stderr.
func newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec -- <cmd> [args…]",
		Short: "Run one command inside the project container (transparent passthrough)",
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Command required (SPEC-verbs exec): a bare `loom exec` is a usage
		// error — non-zero exit, never an interactive fallback (an unattended
		// caller must fail loudly, not hang on a TTY).
		if len(args) == 0 {
			return fmt.Errorf("exec requires a command: loom exec -- <cmd> [args…]")
		}
		res, err := engine.Exec(engine.ExecOpts{PlaybookPath: playbookPath(cmd), Command: args})
		if err != nil {
			return err
		}
		// Exit code propagated verbatim. The exitError path returns the code
		// without printing — nothing of loom's pollutes the command's output.
		if res.ExitCode != 0 {
			return &exitError{code: res.ExitCode}
		}
		return nil
	}
	return cmd
}
