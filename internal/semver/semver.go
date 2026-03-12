// Package semver provides semantic version parsing and sorting utilities.
package semver

import (
	"sort"
	"strconv"
	"strings"
)

// Version represents a parsed semantic version.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string // e.g. "alpha", "alpha.1", "beta.2"
	Original   string // the original string as passed in
}

// Parse parses a semantic version string (with or without leading "v").
// Returns a zero Version with Original set if parsing fails.
func Parse(s string) Version {
	original := s
	s = strings.TrimPrefix(s, "v")

	// Split on "-" to separate pre-release
	parts := strings.SplitN(s, "-", 2)
	core := parts[0]
	preRelease := ""
	if len(parts) == 2 {
		preRelease = parts[1]
	}

	nums := strings.Split(core, ".")
	if len(nums) != 3 {
		return Version{Original: original}
	}

	major, err1 := strconv.Atoi(nums[0])
	minor, err2 := strconv.Atoi(nums[1])
	patch, err3 := strconv.Atoi(nums[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return Version{Original: original}
	}

	return Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: preRelease,
		Original:   original,
	}
}

// Less reports whether v is less than w in semver precedence.
// Pre-release versions have lower precedence than the release version:
//
//	0.31.1-alpha < 0.31.1
func Less(v, w Version) bool {
	if v.Major != w.Major {
		return v.Major < w.Major
	}
	if v.Minor != w.Minor {
		return v.Minor < w.Minor
	}
	if v.Patch != w.Patch {
		return v.Patch < w.Patch
	}
	// Same major.minor.patch — compare pre-release.
	// No pre-release > any pre-release (e.g. 0.31.1 > 0.31.1-alpha).
	if v.PreRelease == "" && w.PreRelease != "" {
		return false // v is the release, w is pre-release → v > w
	}
	if v.PreRelease != "" && w.PreRelease == "" {
		return true // v is pre-release, w is the release → v < w
	}
	// Both have pre-release: compare lexicographically.
	return v.PreRelease < w.PreRelease
}

// SortDescending sorts a slice of version strings from newest to oldest
// using semantic versioning rules.  Strings that cannot be parsed are placed
// at the end in their original order.
func SortDescending(versions []string) []string {
	parsed := make([]Version, len(versions))
	for i, v := range versions {
		parsed[i] = Parse(v)
	}

	sort.SliceStable(parsed, func(i, j int) bool {
		// Descending: newer (greater) versions come first.
		return Less(parsed[j], parsed[i])
	})

	result := make([]string, len(parsed))
	for i, v := range parsed {
		result[i] = v.Original
	}
	return result
}
