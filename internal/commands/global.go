package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/user/istioctl-env/internal/config"
	"github.com/user/istioctl-env/internal/semver"
)

// GlobalHelp prints help for the global command.
func GlobalHelp() {
	fmt.Println(`Usage: istioctl-env global [<version>]

With <version> (exact or constraint), write $ISTIOENV_ROOT/version.  Without
an argument, print the global version.

Flags:
  -h, --help    Show this help message.`)
}

// Global manages the global istioctl version expression.
//
// With a version argument: validates that it parses as either a plain
// version or a SemVer constraint and writes $ISTIOENV_ROOT/version. The
// expression need not resolve to an installed version at this point —
// the shim validates installed-ness at exec time.
//
// Without argument: reads and prints the stored global expression.
func Global(version string) error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	root, _ := config.GetIstioEnvRoot()

	if version == "" {
		// Read global version
		v, err := config.ReadGlobalVersion(root)
		if err != nil {
			return fmt.Errorf("no global version configured")
		}
		fmt.Println(v)
		return nil
	}

	if err := validateVersionExpression(version); err != nil {
		return err
	}

	// Write global version file
	versionFile := filepath.Join(root, "version")
	if err := os.WriteFile(versionFile, []byte(version+"\n"), 0o644); err != nil {
		return fmt.Errorf("failed to write global version: %w", err)
	}

	return nil
}

// validateVersionExpression accepts any string that is either a plain exact
// version or a parseable SemVer constraint. It does NOT require the
// expression to resolve to an installed version — that is a shim-time
// concern.
func validateVersionExpression(version string) error {
	if !semver.IsConstraint(version) {
		return nil
	}
	if _, err := semver.ParseConstraint(version); err != nil {
		return fmt.Errorf("invalid version expression %q: %w", version, err)
	}
	return nil
}
