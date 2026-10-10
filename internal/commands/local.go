package commands

import (
	"fmt"
	"os"

	"github.com/user/istioctl-env/internal/config"
)

// LocalHelp prints help for the local command.
func LocalHelp() {
	fmt.Println(`Usage: istioctl-env local [<version>]

With <version> (exact or constraint), write .istioctl-version in the current
directory.  Without an argument, print the nearest local version (searching
parent directories).

Flags:
  -h, --help    Show this help message.`)
}

// Local manages the local (directory-level) istioctl version expression.
//
// With a version argument: validates that it parses as either a plain version
// or a SemVer constraint and writes .istioctl-version. The expression need
// not resolve to an installed version — that is checked by the shim at exec
// time.
//
// Without argument: reads and prints the stored local expression.
func Local(version string) error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	if version == "" {
		// Read local version
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		v, err := config.FindLocalVersionFrom(cwd)
		if err != nil {
			return fmt.Errorf("no local version configured for this directory")
		}
		fmt.Println(v)
		return nil
	}

	if err := validateVersionExpression(version); err != nil {
		return err
	}

	// Write .istioctl-version in current directory
	if err := os.WriteFile(".istioctl-version", []byte(version+"\n"), 0o644); err != nil {
		return fmt.Errorf("failed to write .istioctl-version: %w", err)
	}

	return nil
}
