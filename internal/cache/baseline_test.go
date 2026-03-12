package cache

import (
	"testing"

	"github.com/user/istioctl-env/internal/semver"
)

func TestBaselineVersions_NonEmpty(t *testing.T) {
	v := BaselineVersions()
	if len(v) == 0 {
		t.Fatal("BaselineVersions must not be empty")
	}
}

func TestBaselinePrereleaseVersions_NonEmpty(t *testing.T) {
	v := BaselinePrereleaseVersions()
	if len(v) == 0 {
		t.Fatal("BaselinePrereleaseVersions must not be empty")
	}
}

func TestBaselineVersions_AllParseable(t *testing.T) {
	for _, v := range BaselineVersions() {
		parsed := semver.Parse(v)
		if parsed.Major == 0 && parsed.Minor == 0 && parsed.Patch == 0 && parsed.PreRelease == "" {
			// Parse returns a zero Version with Original set when it fails.
			// A real version like "0.4.0" has Major=0, Minor=4, Patch=0 — so
			// we check that at least one of Minor or Patch is non-zero, or
			// that the original string is non-empty and the parse succeeded.
			if parsed.Original == v && (parsed.Minor != 0 || parsed.Patch != 0) {
				continue
			}
			t.Errorf("baseline version %q could not be parsed as semver", v)
		}
	}
}

func TestBaselineVersions_SortedDescending(t *testing.T) {
	versions := BaselineVersions()
	for i := 1; i < len(versions); i++ {
		prev := semver.Parse(versions[i-1])
		curr := semver.Parse(versions[i])
		// prev should be >= curr (descending order).
		if semver.Less(prev, curr) {
			t.Errorf("baseline not sorted descending at index %d: %s < %s",
				i, versions[i-1], versions[i])
		}
	}
}

func TestBaselinePrereleaseVersions_SortedDescending(t *testing.T) {
	versions := BaselinePrereleaseVersions()
	for i := 1; i < len(versions); i++ {
		prev := semver.Parse(versions[i-1])
		curr := semver.Parse(versions[i])
		if semver.Less(prev, curr) {
			t.Errorf("prerelease baseline not sorted descending at index %d: %s < %s",
				i, versions[i-1], versions[i])
		}
	}
}

func TestBaselineVersions_NoPrerelease(t *testing.T) {
	for _, v := range BaselineVersions() {
		parsed := semver.Parse(v)
		if parsed.PreRelease != "" {
			t.Errorf("BaselineVersions should not contain pre-releases, found %q", v)
		}
	}
}

func TestBaselineNewest_MatchesFirstEntry(t *testing.T) {
	versions := BaselineVersions()
	if len(versions) == 0 {
		t.Skip("baseline is empty")
	}
	if got := BaselineNewest(); got != versions[0] {
		t.Errorf("BaselineNewest() = %q, want %q (first entry)", got, versions[0])
	}
}

func TestBaselineVersions_ReturnsCopy(t *testing.T) {
	a := BaselineVersions()
	b := BaselineVersions()
	// Mutating one copy must not affect the other.
	if len(a) > 0 {
		a[0] = "mutated"
		if b[0] == "mutated" {
			t.Fatal("BaselineVersions returned the same underlying slice; expected a copy")
		}
	}
}
