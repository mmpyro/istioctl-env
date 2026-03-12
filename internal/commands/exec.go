package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/user/istioctl-env/internal/config"
)

// Exec runs a specific istioctl version without changing the active version.
func Exec(version string, args []string) error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	if version == "" {
		return fmt.Errorf("version not specified. Usage: istioctl-env exec <version> <command> [args...]")
	}

	if len(args) == 0 {
		return fmt.Errorf("command not specified. Usage: istioctl-env exec <version> <command> [args...]")
	}

	installed, err := config.IsVersionInstalled(version)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("version %s is not installed", version)
	}

	binaryPath, err := config.GetBinaryPath(version)
	if err != nil {
		return err
	}

	// Prepare the command
	cmd := exec.Command(binaryPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Maintain environment but force ISTIOENV_VERSION for the shim if it's ever called
	// but here we are calling the binary directly.
	// However, we should still set it in case the subprocess calls other tools that depend on it.
	cmd.Env = append(os.Environ(), fmt.Sprintf("ISTIOENV_VERSION=%s", version))

	return cmd.Run()
}
