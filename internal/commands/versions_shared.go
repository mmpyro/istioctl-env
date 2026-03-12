package commands

import (
	"fmt"
	"os"

	"github.com/user/istioctl-env/internal/cache"
	"github.com/user/istioctl-env/internal/github"
)

// getRemoteVersions returns the list of stable and prerelease versions.
// It uses a three-layer caching strategy:
//
//  1. Disk cache ($ISTIOENV_ROOT/cache/releases.json) — if the cache file exists
//     and is younger than the TTL (default 1 h, overridable via
//     ISTIOENV_CACHE_TTL), the cached list is returned immediately with no
//     network call.
//
//  2. Delta fetch — if the cache is stale or missing, only the releases newer
//     than the most recent known version are fetched from GitHub.  The delta
//     is merged with the existing cache (or the hardcoded baseline) and the
//     result is written back to disk.
//
//  3. Hardcoded baseline (cache.BaselineVersions) — if the network is
//     unavailable and no disk cache exists, the hardcoded list is returned
//     with a warning printed to stderr.  This ensures the command never
//     returns an empty list due to a transient network failure.
func getRemoteVersions(client *github.Client) (stable []string, prerelease []string, err error) {
	c := newCacheForRoot()

	// ── Layer 1: serve from disk cache if it is still fresh ──────────────
	if freshStable, freshPre, ok := c.Load(); ok {
		return freshStable, freshPre, nil
	}

	// ── Layer 2: delta fetch ──────────────────────────────────────────────
	// Read the stale cache (ignoring TTL) so we can use it as the merge base
	// and as the anchor for the delta fetch.  If no stale cache exists we fall
	// back to the hardcoded baseline.
	staleStable, stalePre, hasStale := loadStaleCache(c)

	stableAnchor := cache.BaselineNewest()
	prereleaseAnchor := cache.BaselineNewest()
	if hasStale {
		stableAnchor = cache.NewestVersion(staleStable)
		prereleaseAnchor = cache.NewestVersion(stalePre)
	}

	// Fetch only the releases newer than our anchor.
	deltaStable, errStable := client.ListReleasesSince(stableAnchor, false)
	deltaPre, errPre := client.ListReleasesSince(prereleaseAnchor, true)

	if errStable != nil || errPre != nil {
		// ── Layer 3: network unavailable — fall back to baseline / stale cache ──
		fmt.Fprintln(os.Stderr, "warning: failed to fetch remote versions; showing cached/baseline data")

		// Try to return a stale cache first (better than nothing).
		if hasStale {
			return staleStable, stalePre, nil
		}

		// Last resort: hardcoded baseline.
		return cache.BaselineVersions(), cache.BaselinePrereleaseVersions(), nil
	}

	// Merge delta with the stale cache (preferred) or the hardcoded baseline.
	var mergedStable, mergedPre []string
	if hasStale {
		mergedStable = cache.MergeWithCached(staleStable, deltaStable)
		mergedPre = cache.MergeWithCached(stalePre, deltaPre)
	} else {
		mergedStable = cache.MergeWithBaseline(deltaStable, false)
		mergedPre = cache.MergeWithBaseline(deltaPre, true)
	}

	// Persist the merged result so the next invocation is served from cache.
	// A save failure is non-fatal.
	if err := c.Save(mergedStable, mergedPre); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write version cache: %v\n", err)
	}

	return mergedStable, mergedPre, nil
}

// newCacheForRoot creates a Cache rooted at $ISTIOENV_ROOT/cache.
// If ISTIOENV_ROOT is not set the cache operates in memory-only mode (no disk
// persistence), which means every invocation performs a delta fetch.
func newCacheForRoot() *cache.Cache {
	root := os.Getenv("ISTIOENV_ROOT")
	if root == "" {
		return cache.New("")
	}
	return cache.New(root + "/cache")
}

// loadStaleCache reads the cache file ignoring the TTL.  It returns the
// version lists and true if the file exists and is parseable, regardless of
// age.
func loadStaleCache(c *cache.Cache) (versions []string, prereleaseVersions []string, ok bool) {
	// We exploit the fact that Cache.Load respects the TTL, so we create a
	// temporary cache instance with a very large TTL to bypass expiry.
	staleReader := cache.NewWithTTL(c.Dir(), 1<<62) // effectively infinite TTL
	return staleReader.Load()
}
