package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/user/istioctl-env/internal/config"
	"github.com/user/istioctl-env/internal/github"
)

const execUsage = "Usage: istioctl-env exec [--auto|--no-auto] <version> [--] <command> [args...]"

// ExecHelp prints help for the exec command.
func ExecHelp() {
	fmt.Println(`Usage: istioctl-env exec [--auto|--no-auto] <version> [--] <command> [args...]

Run istioctl at a specific version without changing the active version.
<command> and [args...] are passed to the selected istioctl binary, e.g.
  istioctl-env exec 1.24.0 version --remote=false
  istioctl-env exec "^1.24.0" -- analyze -n default

<version> may be an exact pin or a constraint (~1.24.0, ^1.24.0,
">=1.24.0 <1.26.0", latest, latest-prerelease); the newest installed match
wins.  A single leading "--" after <version> is stripped.

ISTIOENV_VERSION is set to the concrete version for the child process, and
istioctl's exit code is passed through unchanged.

Flags:
  --auto        Install the best matching remote version if nothing installed
                matches (default when ISTIOENV_AUTO_INSTALL is truthy).
  --no-auto     Never install; fail if nothing installed matches.
  -h, --help    Show this help message.`)
}

// Exec runs a specific istioctl version without changing the active version.
//
// spec may be an exact version or a constraint; it is resolved against the
// installed versions first and, when install is true, against the remote
// release list.  The error returned by the child process is passed through
// unwrapped so callers can recover its exit code via *exec.ExitError.
func Exec(spec string, args []string, install bool) error {
	return execWithClient(github.NewClient(), spec, args, install)
}

func execWithClient(client *github.Client, spec string, args []string, install bool) error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	if spec == "" {
		return fmt.Errorf("version not specified. %s", execUsage)
	}

	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return fmt.Errorf("command not specified. %s", execUsage)
	}

	// Installer progress must not mix with the child's stdout.
	var restore func()
	if install {
		restore = redirectStdoutToStderr()
	}
	version, _, err := resolveSpecForShim(client, spec, install, false)
	if restore != nil {
		restore()
	}
	if err != nil {
		return err
	}

	binaryPath, err := config.GetBinaryPath(version)
	if err != nil {
		return err
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Expose the concrete version to the child so any nested istioctl shim
	// invocation resolves to the same binary.
	cmd.Env = append(os.Environ(), fmt.Sprintf("ISTIOENV_VERSION=%s", version))

	return cmd.Run()
}
