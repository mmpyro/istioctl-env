package commands

import (
	"fmt"
	"os"

	"github.com/user/istioctl-env/internal/config"
)

// ShellHelp prints help for the shell command.
func ShellHelp() {
	fmt.Println(`Usage: istioctl-env shell [<version>]

With <version> (exact or constraint), print an export command that sets
ISTIOENV_VERSION for the current shell; the shell function installed by
'istioctl-env init' evaluates it.  Without an argument, print the current
shell-level version.

Flags:
  -h, --help    Show this help message.`)
}

// Shell manages the shell-level istioctl version expression.
//
// With a version argument: validates that it parses as either a plain version
// or a SemVer constraint and prints the export command for the shell wrapper
// to eval. The expression need not resolve to an installed version — the
// shim validates installed-ness at exec time.
//
// Without argument: prints the current shell expression.
func Shell(version string) error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	if version == "" {
		// Print current shell version
		v := os.Getenv("ISTIOENV_VERSION")
		if v == "" {
			return fmt.Errorf("no shell version configured")
		}
		fmt.Println(v)
		return nil
	}

	if err := validateVersionExpression(version); err != nil {
		return err
	}

	// Output export command for the shell function wrapper to eval
	fmt.Printf("export ISTIOENV_VERSION=%s\n", version)
	return nil
}
