package commands

import (
	"fmt"

	"github.com/user/istioctl-env/internal/github"
)

// ListRemoteHelp prints the help message for the list-remote command.
func ListRemoteHelp() {
	fmt.Println(`Usage: istioctl-env list-remote [flags]

List all available versions of istioctl cli from GitHub releases.
Versions are printed from newest to oldest.

Results are cached on disk (at $ISTIOENV_ROOT/cache/releases.json) for one hour
by default to avoid redundant network requests.  Set ISTIOENV_CACHE_TTL to a Go
duration string (e.g. "30m", "24h") to override the TTL.

Flags:
  --prerelease   Include pre-release versions (e.g. alpha, beta, rc)
  -h, --help      Show this help message`)
}

// ListRemote prints all available istioctl versions from GitHub releases.
// It does NOT require init — only queries GitHub (or the local cache).
func ListRemote(includePrerelease bool) error {
	return listRemoteWithClient(github.NewClient(), includePrerelease)
}

// listRemoteWithClient is the testable core of ListRemote.  It accepts an
// injected GitHub client so tests can point it at a mock HTTP server.
func listRemoteWithClient(client *github.Client, includePrerelease bool) error {
	stable, pre, err := getRemoteVersions(client)
	if err != nil {
		return err
	}

	versions := stable
	if includePrerelease {
		versions = pre
	}

	for _, v := range versions {
		fmt.Println(v)
	}
	return nil
}

