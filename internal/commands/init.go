package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/istioctl-env/internal/config"
	"github.com/user/istioctl-env/internal/shim"
)

// Init initializes the istioctl-env environment.
// If ISTIOENV_ROOT is not set, it prints setup instructions.
// If set, it creates the versions directory, generates the shim, and outputs
// shell initialization code for the requested shell.
//
// shell may be one of: "bash", "zsh", "fish", or "" to auto-detect from $SHELL.
// When auto-detection fails, a warning is printed to stderr and bash is used.
func Init(shell string) error {
	root, ok := config.GetIstioEnvRoot()
	if !ok {
		fmt.Fprintln(os.Stderr, `ISTIOENV_ROOT not set. Set ISTIOENV_ROOT for your shell using the below commands.
echo 'export ISTIOENV_ROOT="$HOME/.istioenv"' >> ~/.bashrc
or
echo 'export ISTIOENV_ROOT="$HOME/.istioenv"' >> ~/.zshrc
or
echo 'set -gx ISTIOENV_ROOT "$HOME/.istioenv"' >> ~/.config/fish/config.fish`)
		return fmt.Errorf("ISTIOENV_ROOT not set")
	}

	resolvedShell, err := resolveShell(shell)
	if err != nil {
		return err
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

	// Output shell initialization code for the resolved shell
	fmt.Print(shim.GenerateShellInit(root, resolvedShell))

	return nil
}

// InitHelp prints help for the init command.
func InitHelp() {
	fmt.Println(`Usage: istioctl-env init [--shell <bash|zsh|fish>]

Generate the istioctl shim and shell integration code.

Flags:
  --shell <name>   Target shell for the integration snippet. Defaults to
                   auto-detection via $SHELL and falls back to bash.
  -h, --help       Show this help and exit.

Examples:
  eval "$(istioctl-env init)"                 # auto-detect
  eval "$(istioctl-env init --shell bash)"    # bash
  eval "$(istioctl-env init --shell zsh)"     # zsh
  istioctl-env init --shell fish | source     # fish`)
}

// resolveShell picks a supported shell identifier based on an explicit flag
// value or, when the flag is empty, by inspecting $SHELL. It returns the
// canonical identifier and emits a WARN to stderr when it has to fall back
// to bash because detection was inconclusive.
func resolveShell(flagValue string) (string, error) {
	if strings.TrimSpace(flagValue) != "" {
		return shim.NormalizeShell(flagValue)
	}
	if detected := shim.DetectShell(); detected != "" {
		return detected, nil
	}
	fmt.Fprintln(os.Stderr, "WARN: could not detect shell from $SHELL; defaulting to bash. Pass --shell <bash|zsh|fish> to silence this warning.")
	return shim.ShellBash, nil
}
