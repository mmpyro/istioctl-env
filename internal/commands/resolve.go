package commands

import (
	"fmt"
	"os"

	"github.com/user/istioctl-env/internal/config"
	"github.com/user/istioctl-env/internal/github"
)

// ResolveHelp prints the help text for the `resolve` command.
func ResolveHelp() {
	fmt.Println(`Usage: istioctl-env resolve [flags]

Resolve the active istioctl version expression (shell > local > global) to a
concrete installed version and print it on stdout.  Intended for internal use
by the istioctl shim and advanced tooling.

If the active expression is a SemVer constraint (e.g. ^1.24.0,
>=1.24.0 <1.26.0, latest, latest-prerelease), the newest already-installed
version that satisfies the constraint is printed.

Flags:
  --install      When no installed version satisfies the expression, install
                 the best matching remote version instead of erroring.
                 Equivalent to setting ISTIOENV_AUTO_INSTALL=true.
  --silent       Suppress installer progress output (recommended for the shim
                 and other non-TTY callers).
  -h, --help     Show this help message.

Environment:
  ISTIOENV_AUTO_INSTALL  When set to a truthy value (1, true, yes, on),
                         enables --install implicitly.`)
}

// Resolve implements the `resolve` subcommand.  It consults the active
// version expression, resolves any constraints, optionally auto-installs
// the best match, and prints the concrete version on stdout.
//
// Any download / progress output goes to stderr (so the printed resolved
// version remains a clean one-line value suitable for capture by the shim).
func Resolve(install bool, silent bool) error {
	return resolveWithClient(github.NewClient(), install, silent)
}

func resolveWithClient(client *github.Client, install bool, silent bool) error {
	if err := config.RequireInit(); err != nil {
		return err
	}
	if !install && AutoInstallEnabled() {
		install = true
	}

	// Route installer progress output to stderr so stdout stays clean.
	var restore func()
	if install {
		restore = redirectStdoutToStderr()
		defer restore()
	}

	resolved, _, err := resolveSpecForShim(client, install, silent)
	if restore != nil {
		restore()
		restore = nil
	}
	if err != nil {
		return err
	}
	fmt.Println(resolved)
	return nil
}

// redirectStdoutToStderr temporarily swaps os.Stdout for os.Stderr so that
// progress output from installer helpers (which print to stdout) is routed
// to stderr during auto-install.  Returns a restorer that puts the original
// stdout back.
func redirectStdoutToStderr() func() {
	orig := os.Stdout
	os.Stdout = os.Stderr
	return func() {
		os.Stdout = orig
	}
}
