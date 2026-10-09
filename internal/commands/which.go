package commands

import (
	"fmt"
	"os"

	"github.com/user/istioctl-env/internal/config"
)

// Which prints the absolute path to the active istioctl binary.
//
// When the configured version expression is a constraint, the resolution is
// printed to stderr (e.g. "^1.24.0 → 1.24.3") so stdout remains a clean,
// scriptable binary path.
func Which() error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	resolved, raw, err := config.ResolveInstalledVersion()
	if err != nil {
		return err
	}

	if raw != resolved {
		fmt.Fprintf(os.Stderr, "%s → %s\n", raw, resolved)
	}

	binaryPath, err := config.GetBinaryPath(resolved)
	if err != nil {
		return err
	}

	fmt.Println(binaryPath)
	return nil
}
