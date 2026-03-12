package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/user/istioctl-env/internal/config"
	"github.com/user/istioctl-env/internal/shim"
)

// Init initializes the istioctl-env environment.
// If ISTIOENV_ROOT is not set, it prints setup instructions.
// If set, it creates the versions directory, generates the shim, and outputs
// shell initialization code.
func Init() error {
	root, ok := config.GetIstioEnvRoot()
	if !ok {
		fmt.Fprintln(os.Stderr, `ISTIOENV_ROOT not set. Set ISTIOENV_ROOT for your shell using the below commands.
echo 'export ISTIOENV_ROOT="$HOME/.istioenv"' >> ~/.bashrc
or
echo 'export ISTIOENV_ROOT="$HOME/.istioenv"' >> ~/.zshrc`)
		return fmt.Errorf("ISTIOENV_ROOT not set")
	}

	// Create versions directory
	versionsDir := filepath.Join(root, "versions")
	if err := os.MkdirAll(versionsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create versions directory: %w", err)
	}

	// Generate shim script
	if err := shim.GenerateShimScript(root); err != nil {
		return fmt.Errorf("failed to generate shim: %w", err)
	}

	// Output shell initialization code
	fmt.Print(shim.GenerateShellInit(root))

	return nil
}
