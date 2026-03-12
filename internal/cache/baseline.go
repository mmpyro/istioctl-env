package cache

// baselineVersions is a hardcoded list of historically known istioctl
// stable releases up to the time this version of istioctl-env was built.
//
// Purpose: provide a zero-network starting point so that:
//  1. The very first invocation (no disk cache yet) can return a useful list
//     without fetching all pages from GitHub.
//  2. If the network is unavailable, the command still returns something
//     meaningful rather than failing entirely.
//
// Maintenance: append new versions here when cutting a new istioctl-env release.
// The delta-fetch logic will automatically pick up anything newer than the
// last entry in this list, so the list does not need to be exhaustively
// up-to-date — it just needs to be a reasonable lower bound.
//
// Versions are stored newest-first (descending semver order) to match the
// output format of list-remote and to make it easy to find the newest entry.
var baselineVersions = []string{
	// 1.25.x
	"1.25.0",
	// 1.24.x
	"1.24.3",
	"1.24.2",
	"1.24.1",
	"1.24.0",
	// 1.23.x
	"1.23.4",
	"1.23.3",
	"1.23.2",
	"1.23.1",
	"1.23.0",
	// 1.22.x
	"1.22.7",
	"1.22.6",
	"1.22.5",
	"1.22.4",
	"1.22.3",
	"1.22.2",
	"1.22.1",
	"1.22.0",
	// 1.21.x
	"1.21.6",
	"1.21.5",
	"1.21.4",
	"1.21.3",
	"1.21.2",
	"1.21.1",
	"1.21.0",
	// 1.20.x
	"1.20.8",
	"1.20.7",
	"1.20.6",
	"1.20.5",
	"1.20.4",
	"1.20.3",
	"1.20.2",
	"1.20.1",
	"1.20.0",
	// 1.19.x
	"1.19.10",
	"1.19.9",
	"1.19.8",
	"1.19.7",
	"1.19.6",
	"1.19.5",
	"1.19.4",
	"1.19.3",
	"1.19.0",
	// 1.18.x
	"1.18.7",
	"1.18.6",
	"1.18.5",
	"1.18.2",
	"1.18.1",
	"1.18.0",
	// 1.17.x
	"1.17.8",
	"1.17.6",
	"1.17.5",
	"1.17.4",
	"1.17.3",
	"1.17.2",
	"1.17.1",
	"1.17.0",
}

// baselinePrereleaseVersions is the same as baselineVersions but also includes
// known pre-release versions.  It is used when --prereleases is requested.
var baselinePrereleaseVersions = []string{
	// 1.25.x
	"1.25.0",
	// 1.24.x
	"1.24.3",
	"1.24.2",
	"1.24.1",
	"1.24.0",
	// 1.23.x
	"1.23.4",
	"1.23.3",
	"1.23.2",
	"1.23.1",
	"1.23.0",
	// 1.22.x
	"1.22.7",
	"1.22.6",
	"1.22.5",
	"1.22.4",
	"1.22.3",
	"1.22.2",
	"1.22.1",
	"1.22.0",
	// 1.21.x
	"1.21.6",
	"1.21.5",
	"1.21.4",
	"1.21.3",
	"1.21.2",
	"1.21.1",
	"1.21.0",
	// 1.20.x
	"1.20.8",
	"1.20.7",
	"1.20.6",
	"1.20.5",
	"1.20.4",
	"1.20.3",
	"1.20.2",
	"1.20.1",
	"1.20.0",
	// 1.19.x
	"1.19.10",
	"1.19.9",
	"1.19.8",
	"1.19.7",
	"1.19.6",
	"1.19.5",
	"1.19.4",
	"1.19.3",
	"1.19.0",
	// 1.18.x
	"1.18.7",
	"1.18.6",
	"1.18.5",
	"1.18.2",
	"1.18.1",
	"1.18.0",
	// 1.17.x
	"1.17.8",
	"1.17.6",
	"1.17.5",
	"1.17.4",
	"1.17.3",
	"1.17.2",
	"1.17.1",
	"1.17.0",
}

// BaselineVersions returns a copy of the hardcoded stable version list.
// The caller receives a fresh slice and may modify it freely.
func BaselineVersions() []string {
	out := make([]string, len(baselineVersions))
	copy(out, baselineVersions)
	return out
}

// BaselinePrereleaseVersions returns a copy of the hardcoded version list
// that includes pre-release entries.
func BaselinePrereleaseVersions() []string {
	out := make([]string, len(baselinePrereleaseVersions))
	copy(out, baselinePrereleaseVersions)
	return out
}

// BaselineNewest returns the newest version present in the baseline list,
// or an empty string if the baseline is empty.  This is used as the anchor
// for delta-fetching: only releases newer than this version are fetched from
// the GitHub API.
func BaselineNewest() string {
	if len(baselineVersions) == 0 {
		return ""
	}
	// The list is stored newest-first, so index 0 is the newest.
	return baselineVersions[0]
}
