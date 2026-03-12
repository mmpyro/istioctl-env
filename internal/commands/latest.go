package commands

import (
	"fmt"

	"github.com/user/istioctl-env/internal/github"
)

// LatestHelp prints the help message for the latest command.
func LatestHelp() {
	fmt.Println(`Usage: istioctl-env latest [flags]

Print the latest available version of istioctl cli from GitHub releases.

Flags:
  --prerelease    Include pre-release versions (e.g. alpha, beta, rc)
  -h, --help      Show this help message`)
}

// Latest prints the latest available istioctl version from GitHub releases.
// If includePrerelease is false, only the latest stable version is returned.
// If includePrerelease is true, the latest version including prereleases is returned.
// It does NOT require init — only queries GitHub (or the local cache).
func Latest(includePrerelease bool) error {
	return latestWithClient(github.NewClient(), includePrerelease)
}

// latestWithClient is the testable core of Latest.  It accepts an
// injected GitHub client so tests can point it at a mock HTTP server.
func latestWithClient(client *github.Client, includePrerelease bool) error {
	stable, pre, err := getRemoteVersions(client)
	if err != nil {
		return err
	}

	versions := stable
	if includePrerelease {
		versions = pre
	}

	if len(versions) == 0 {
		return fmt.Errorf("no versions found")
	}

	fmt.Println(versions[0])
	return nil
}
