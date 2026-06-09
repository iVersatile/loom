// Command loom is the Loom build engine: it reconciles a real dev environment
// to a declarative playbook. See docs/SPEC-verbs.md for the verb contracts.
package main

import (
	"os"

	"github.com/iVersatile/loom/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
