// Package cache provides a persistent, TTL-aware disk cache for istioctl
// release version lists.
//
// # Strategy
//
// The cache uses a three-layer approach to minimise network traffic while
// always returning an up-to-date version list:
//
//  1. Hardcoded baseline (baseline.go) — all historically known versions
//     baked into the binary.  Provides a useful result with zero network
//     calls and acts as the fallback of last resort when the network is
//     unavailable.
//
//  2. Disk cache ($ISTIOENV_ROOT/cache/releases.json) — persists the merged
//     result of baseline + delta across process invocations.  Served
//     directly when the cache is fresh (within TTL).
//
//  3. Delta fetch — when the cache is stale or missing, only the releases
//     newer than the most recent known version are fetched from GitHub.
//     The delta is merged with the existing cache (or baseline) and the
//     result is written back to disk.
//
// # Cache invalidation
//
// The cache is considered stale when:
//   - The cache file does not exist (first run).
//   - The cache file cannot be parsed (corruption).
//   - The age of the cache exceeds the TTL (default 1 h, overridable via
//     the ISTIOENV_CACHE_TTL environment variable).
//
// # Thread safety
//
// Each istioctl-env invocation is a separate OS process, so no in-process mutex is
// required.  Writes use a write-to-temp-file + os.Rename pattern which is
// atomic on POSIX systems, preventing partial reads by concurrent processes.
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/user/istioctl-env/internal/semver"
)

const (
	// cacheFileName is the name of the JSON file stored under
	// $ISTIOENV_ROOT/cache/.
	cacheFileName = "releases.json"

	// defaultTTL is how long a cache entry is considered fresh before a
	// delta-fetch is triggered.  One hour is a good default: istioctl
	// releases are infrequent (weekly/monthly) so this avoids redundant
	// network calls while still surfacing new releases quickly.
	defaultTTL = time.Hour
)

// entry is the on-disk JSON structure for the cache file.
type entry struct {
	// FetchedAt is the UTC timestamp of the last successful fetch.
	FetchedAt time.Time `json:"fetched_at"`

	// Versions holds the merged stable-only version list, newest-first.
	Versions []string `json:"versions"`

	// PrereleaseVersions holds the merged version list including
	// pre-releases, newest-first.
	PrereleaseVersions []string `json:"prerelease_versions"`
}

// Cache manages the on-disk release version cache.
type Cache struct {
	// dir is the directory that holds the cache file
	// (typically $ISTIOENV_ROOT/cache).
	dir string

	// ttl is the maximum age of a cache entry before it is considered stale.
	ttl time.Duration
}

// New creates a Cache that stores its file in dir.
// If dir is empty the Cache operates in memory-only mode: Load always
// returns a cache-miss and Save is a no-op.
// The TTL is read from the ISTIOENV_CACHE_TTL environment variable (default 1 h).
func New(dir string) *Cache {
	return &Cache{
		dir: dir,
		ttl: parseTTL(),
	}
}

// NewWithTTL creates a Cache with an explicit TTL, bypassing the environment
// variable.  This is primarily useful in tests and for the stale-cache reader
// pattern (pass a very large TTL to read regardless of age).
func NewWithTTL(dir string, ttl time.Duration) *Cache {
	return &Cache{dir: dir, ttl: ttl}
}

// Dir returns the directory in which the cache file is stored.
func (c *Cache) Dir() string {
	return c.dir
}

// parseTTL reads the ISTIOENV_CACHE_TTL environment variable and returns the
// parsed duration, falling back to defaultTTL on any error.
func parseTTL() time.Duration {
	raw := os.Getenv("ISTIOENV_CACHE_TTL")
	if raw == "" {
		return defaultTTL
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return defaultTTL
	}
	return d
}

// path returns the full path to the cache file.
func (c *Cache) path() string {
	return filepath.Join(c.dir, cacheFileName)
}

// Load reads the cache from disk and returns the stored version lists.
// It returns (nil, nil, false) when the cache is missing, corrupt, or stale —
// the caller should perform a fresh fetch in that case.
//
// The returned slices are copies; the caller may modify them freely.
func (c *Cache) Load() (versions []string, prereleaseVersions []string, ok bool) {
	if c.dir == "" {
		return nil, nil, false
	}

	data, err := os.ReadFile(c.path())
	if err != nil {
		// Missing file is a normal cache-miss; other errors are also treated
		// as a miss so the caller falls back to a fresh fetch.
		return nil, nil, false
	}

	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		// Corrupted file — treat as a miss and let the caller overwrite it.
		return nil, nil, false
	}

	if time.Since(e.FetchedAt) > c.ttl {
		// Cache is stale.
		return nil, nil, false
	}

	// Return defensive copies.
	v := make([]string, len(e.Versions))
	copy(v, e.Versions)
	pv := make([]string, len(e.PrereleaseVersions))
	copy(pv, e.PrereleaseVersions)

	return v, pv, true
}

// Save writes the version lists to the disk cache atomically.
// It creates the cache directory if it does not exist.
// Errors are non-fatal: a failed save means the next invocation will simply
// perform another fetch.
func (c *Cache) Save(versions []string, prereleaseVersions []string) error {
	if c.dir == "" {
		return nil
	}

	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return fmt.Errorf("cache: failed to create directory %s: %w", c.dir, err)
	}

	e := entry{
		FetchedAt:          time.Now().UTC(),
		Versions:           versions,
		PrereleaseVersions: prereleaseVersions,
	}

	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("cache: failed to marshal entry: %w", err)
	}

	// Write to a temp file in the same directory, then rename for atomicity.
	tmp, err := os.CreateTemp(c.dir, ".releases-*.json.tmp")
	if err != nil {
		return fmt.Errorf("cache: failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("cache: failed to write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("cache: failed to close temp file: %w", err)
	}

	// Atomic rename — on POSIX this is guaranteed to be atomic.
	if err := os.Rename(tmpName, c.path()); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("cache: failed to rename temp file: %w", err)
	}

	return nil
}

// MergeWithBaseline merges delta (newly fetched versions) with the baseline
// hardcoded list, deduplicates, and returns the result sorted newest-first.
//
// includePrerelease controls which baseline list is used as the starting
// point.  The delta slice may contain pre-releases regardless of this flag;
// they will be included in the merged result only when includePrerelease is
// true.
func MergeWithBaseline(delta []string, includePrerelease bool) []string {
	var base []string
	if includePrerelease {
		base = BaselinePrereleaseVersions()
	} else {
		base = BaselineVersions()
	}
	return mergeAndSort(base, delta)
}

// MergeWithCached merges delta (newly fetched versions) with an existing
// cached list, deduplicates, and returns the result sorted newest-first.
func MergeWithCached(cached []string, delta []string) []string {
	return mergeAndSort(cached, delta)
}

// mergeAndSort combines two version slices, removes duplicates, and returns
// the result sorted in descending semver order.
func mergeAndSort(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	merged := make([]string, 0, len(a)+len(b))

	for _, v := range a {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			merged = append(merged, v)
		}
	}
	for _, v := range b {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			merged = append(merged, v)
		}
	}

	return semver.SortDescending(merged)
}

// NewestVersion returns the newest version in the given slice, or an empty
// string if the slice is empty.  The slice is assumed to be sorted
// newest-first (as returned by Load or MergeWithBaseline).
func NewestVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	return versions[0]
}
