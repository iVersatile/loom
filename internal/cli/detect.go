package cli

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strings"

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
	cmd.Flags().Bool("yes", false, "skip the migrate confirmation (automation; --migrate only)")
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
		if out := emit(cmd, res, err); out != nil {
			return out
		}

		if migrate {
			return runMigrate(cmd, res)
		}
		return nil
	}
	return cmd
}

// runMigrate consolidates detected credentials into the project .env. The
// REDACTED diff and the result both ride stderr only — detect's frozen --json
// document on stdout is untouched, and no secret VALUE is ever rendered.
func runMigrate(cmd *cobra.Command, res engine.DetectResult) error {
	envPath := filepath.Join(filepath.Dir(playbookPath(cmd)), ".env")
	jMode := jsonMode(cmd)
	autoYes, _ := cmd.Flags().GetBool("yes")

	confirm := func(rep engine.MigrateReport) (bool, error) {
		w := cmd.ErrOrStderr()
		_, _ = fmt.Fprintf(w, "loom: migrate -> %s\n", rep.EnvPath)
		for _, c := range rep.Creds {
			sigil := "="
			switch c.Status {
			case "new":
				sigil = "+"
			case "changed":
				sigil = "~"
			}
			if c.Reference {
				_, _ = fmt.Fprintf(w, "  %s %s [%s] = %s (reference)\n", sigil, c.Name, c.Status, c.Display)
			} else {
				_, _ = fmt.Fprintf(w, "  %s %s [%s]\n", sigil, c.Name, c.Status)
			}
		}
		if autoYes {
			return true, nil
		}
		if jMode {
			// Non-interactive without --yes: do not prompt; the CLI reports
			// that --yes is needed.
			return false, nil
		}
		_, _ = fmt.Fprint(w, "Write these to .env? [y/N] ")
		line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return true, nil
		default:
			return false, nil // never-auto on EOF/empty (mirror teardown)
		}
	}

	report, n, merr := engine.Migrate(res.Credentials, envPath, confirm)
	if merr != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "loom: detect: migrate: %v\n", merr)
		return merr
	}

	hadWritable := false
	for _, c := range report.Creds {
		if c.Status != "unchanged" {
			hadWritable = true
			break
		}
	}

	switch {
	case n > 0:
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "loom: detect: migrate: wrote %d credential(s) to %s\n", n, envPath)
	case hadWritable:
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "loom: detect: migrate: aborted (nothing written; pass --yes for automation)\n")
	default:
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "loom: detect: migrate: nothing to migrate\n")
	}
	return nil
}
