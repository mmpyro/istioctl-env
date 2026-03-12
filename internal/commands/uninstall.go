package commands

import (
	"fmt"
	"os"

	"github.com/user/istioctl-env/internal/config"
)

// Uninstall removes an installed istioctl version.
func Uninstall(version string) error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	if version == "" {
		return fmt.Errorf("version argument is required. Usage: istioctl-env uninstall <version>")
	}

	// Check if version is installed
	installed, err := config.IsVersionInstalled(version)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("version %s is not installed", version)
	}

	// Remove version directory
	versionDir, err := config.GetVersionDir(version)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(versionDir); err != nil {
		return fmt.Errorf("failed to remove version %s: %w", version, err)
	}

	fmt.Printf("version %s uninstalled\n", version)
	return nil
}
