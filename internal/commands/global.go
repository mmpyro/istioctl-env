package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/user/istioctl-env/internal/config"
)

// Global manages the global istioctl version.
// With a version argument: verifies it's installed and writes $ISTIOENV_ROOT/version.
// Without argument: reads and prints the global version or errors.
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

	// Verify version is installed
	installed, err := config.IsVersionInstalled(version)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("version %s not installed", version)
	}

	// Write global version file
	versionFile := filepath.Join(root, "version")
	if err := os.WriteFile(versionFile, []byte(version+"\n"), 0o644); err != nil {
		return fmt.Errorf("failed to write global version: %w", err)
	}

	return nil
}
