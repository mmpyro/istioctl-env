package commands

import (
	"fmt"

	"github.com/user/istioctl-env/internal/cache"
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
  --cached       Print the on-disk cached list only (no network calls).
                 Falls back to the built-in baseline when the cache is
                 missing.  Intended for shell completion.
  -h, --help     Show this help message`)
}

// ListRemote prints all available istioctl versions from GitHub releases.
// It does NOT require init — only queries GitHub (or the local cache).
func ListRemote(includePrerelease bool) error {
	return listRemoteWithClient(github.NewClient(), includePrerelease)
}

// ListRemoteCached prints the on-disk cached list of istioctl versions
// without ever making a network call. If the cache is missing or corrupt the
// built-in baseline is used so completion always has something to offer.
//
// This is intentionally fast and silent: no WARN, no error on cache miss. It
// is designed to be called from shell completion hooks on every TAB.
func ListRemoteCached(includePrerelease bool) error {
	stable, pre := cachedOrBaselineVersions()
	versions := stable
	if includePrerelease {
		versions = pre
	}
	for _, v := range versions {
		fmt.Println(v)
	}
	return nil
}

// cachedOrBaselineVersions reads $ISTIOENV_ROOT/cache/releases.json ignoring
// the TTL, and returns the baked-in baseline when the cache is unreadable.
func cachedOrBaselineVersions() (stable []string, prerelease []string) {
	c := newCacheForRoot()
	if s, p, ok := loadStaleCache(c); ok {
		return s, p
	}
	return cache.BaselineVersions(), cache.BaselinePrereleaseVersions()
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

