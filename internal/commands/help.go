// Package commands implements all istioctl-env CLI commands.
package commands

import "fmt"

// Help prints the help message with all available commands.
func Help() {
	fmt.Println(`Usage: istioctl-env [global flags] <command> [arguments]

Commands:
  help            Display this help message and all available commands
  list            List all installed versions of istioctl cli. Flags: --disk-usage
  list-remote     List all available versions of istioctl cli from GitHub
  init            Initialize istioctl-env setup
  install         Install a specific version (or latest if not specified). Flags: -s, --silent
  uninstall       Uninstall a specific version
  prune           Remove unreferenced installed versions. Flags: --keep-last, --older-than, --dry-run, --yes
  shell           Set or show the shell version of istioctl cli
  local           Set or show the local version of istioctl cli
  global          Set or show the global version of istioctl cli
  latest          Print the latest available version of istioctl cli from GitHub releases.
  which           Print the full path to the active istioctl binary
  exec            Run a command using a specific istioctl version
  status          Show current istioctl-env environment status
  upgrade         Upgrade istioctl-env to the latest version
  autocompletion  Generate bash autocompletion script
  doctor          Diagnose the istioctl-env environment and print OK/WARN/FAIL for each check (flags: --fix, --deep)
  version         Print the version of istioctl-env

Global flags (equivalent env var in parentheses):
  --offline                     Never contact the network   (ISTIOENV_OFFLINE=1)
  --github-token <token>        GitHub token (Bearer auth)  (ISTIOENV_GITHUB_TOKEN)
  --api-mirror <url>            Override api.github.com     (ISTIOENV_API_MIRROR)
  --download-mirror <url>       Override github.com         (ISTIOENV_DOWNLOAD_MIRROR)`)
}

// InstallHelp prints help for the install command.
func InstallHelp() {
	fmt.Println(`Usage: istioctl-env install [version] [flags]

Flags:
  -s, --silent    Do not display progress bar or checksum info`)
}
