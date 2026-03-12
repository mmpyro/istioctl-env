package commands

import (
	"fmt"

	"github.com/user/istioctl-env/internal/config"
)

// Which prints the absolute path to the active istioctl binary.
func Which() error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	version, err := config.ResolveVersion()
	if err != nil {
		return err
	}

	binaryPath, err := config.GetBinaryPath(version)
	if err != nil {
		return err
	}

	fmt.Println(binaryPath)
	return nil
}
