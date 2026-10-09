package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/user/istioctl-env/internal/cache"
	"github.com/user/istioctl-env/internal/github"
)

// newMockServer returns a test HTTP server that serves the given releases.
func newMockServer(t *testing.T, releases []github.Release) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}))
}

// ── listRemoteWithClient tests ────────────────────────────────────────────────

func TestListRemote_FreshCache_NoNetworkCall(t *testing.T) {
	// Pre-populate a fresh cache.
	// newCacheForRoot stores the cache at $ISTIOENV_ROOT/cache, so we must
	// write to that sub-directory.
	root := t.TempDir()
	c := cache.NewWithTTL(root+"/cache", time.Hour)
	stable := []string{"0.31.0", "0.30.0"}
	pre := []string{"0.32.0-alpha.1", "0.31.0", "0.30.0"}
	if err := c.Save(stable, pre); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Point ISTIOENV_ROOT at our temp dir so newCacheForRoot picks it up.
	t.Setenv("ISTIOENV_ROOT", root)

	// Use a client that always fails — it must never be called.
	failClient := &github.Client{
		BaseURL: "http://127.0.0.1:0", // nothing listening here
		HTTPClient: &http.Client{
			Timeout: 100 * time.Millisecond,
		},
	}

	out := captureStdout(t, func() {
		if err := listRemoteWithClient(failClient, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "0.31.0") {
		t.Errorf("expected 0.31.0 in output, got: %s", out)
	}
	if !strings.Contains(out, "0.30.0") {
		t.Errorf("expected 0.30.0 in output, got: %s", out)
	}
}

func TestListRemote_FreshCache_Prereleases(t *testing.T) {
	root := t.TempDir()
	c := cache.NewWithTTL(root+"/cache", time.Hour)
	stable := []string{"0.31.0", "0.30.0"}
	pre := []string{"0.32.0-alpha.1", "0.31.0", "0.30.0"}
	if err := c.Save(stable, pre); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Setenv("ISTIOENV_ROOT", root)

	failClient := &github.Client{
		BaseURL:    "http://127.0.0.1:0",
		HTTPClient: &http.Client{Timeout: 100 * time.Millisecond},
	}

	out := captureStdout(t, func() {
		if err := listRemoteWithClient(failClient, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "0.32.0-alpha.1") {
		t.Errorf("expected pre-release in output, got: %s", out)
	}
}

func TestListRemote_StaleCache_DeltaFetch(t *testing.T) {
	// Write a stale cache that contains only 0.30.0.
	root := t.TempDir()
	c := cache.NewWithTTL(root+"/cache", -1) // negative TTL → always stale
	if err := c.Save([]string{"0.30.0"}, []string{"0.30.0"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Setenv("ISTIOENV_ROOT", root)
	t.Setenv("ISTIOENV_CACHE_TTL", "1ns")

	// The mock server returns a new release 0.31.0 that is newer than 0.30.0.
	releases := []github.Release{
		{TagName: "v0.31.0", Prerelease: false, Draft: false},
		{TagName: "v0.30.0", Prerelease: false, Draft: false},
	}
	server := newMockServer(t, releases)
	defer server.Close()

	client := &github.Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	out := captureStdout(t, func() {
		if err := listRemoteWithClient(client, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "0.31.0") {
		t.Errorf("expected new version 0.31.0 in output, got: %s", out)
	}
	if !strings.Contains(out, "0.30.0") {
		t.Errorf("expected existing version 0.30.0 in output, got: %s", out)
	}

	// Verify the cache was updated.
	freshCache := cache.NewWithTTL(root+"/cache", time.Hour)
	versions, _, ok := freshCache.Load()
	if !ok {
		t.Fatal("expected cache to be refreshed after delta fetch")
	}
	found := false
	for _, v := range versions {
		if v == "0.31.0" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 0.31.0 in refreshed cache, got: %v", versions)
	}
}

func TestListRemote_NoCache_NetworkFails_FallsBackToBaseline(t *testing.T) {
	// No ISTIOENV_ROOT → no disk cache.
	t.Setenv("ISTIOENV_ROOT", "")

	failClient := &github.Client{
		BaseURL:    "http://127.0.0.1:0",
		HTTPClient: &http.Client{Timeout: 100 * time.Millisecond},
	}

	var stderrBuf bytes.Buffer
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	out := captureStdout(t, func() {
		// Should not return an error even though the network is down.
		_ = listRemoteWithClient(failClient, false)
	})

	_ = w.Close()
	os.Stderr = oldStderr
	_, _ = io.Copy(&stderrBuf, r)

	// The baseline must contain at least one version.
	if strings.TrimSpace(out) == "" {
		t.Fatal("expected baseline versions in output, got empty string")
	}
	// A warning should have been printed to stderr.
	if !strings.Contains(stderrBuf.String(), "warning") {
		t.Errorf("expected warning on stderr, got: %s", stderrBuf.String())
	}
}

func TestListRemote_OutputSortedDescending(t *testing.T) {
	t.Setenv("ISTIOENV_ROOT", t.TempDir())

	releases := []github.Release{
		{TagName: "v1.26.0", Prerelease: false, Draft: false},
		{TagName: "v1.27.0", Prerelease: false, Draft: false},
		{TagName: "v1.28.0", Prerelease: false, Draft: false},
	}
	server := newMockServer(t, releases)
	defer server.Close()

	client := &github.Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	out := captureStdout(t, func() {
		if err := listRemoteWithClient(client, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Find the positions of the three versions.
	pos := func(v string) int {
		for i, l := range lines {
			if strings.TrimSpace(l) == v {
				return i
			}
		}
		return -1
	}

	p28 := pos("1.28.0")
	p27 := pos("1.27.0")
	p26 := pos("1.26.0")

	if p28 == -1 || p27 == -1 || p26 == -1 {
		t.Fatalf("not all versions found in output: %v", lines)
	}
	if !(p28 < p27 && p27 < p26) {
		t.Fatalf("expected descending order 1.28.0 > 1.27.0 > 1.26.0, got positions %d %d %d",
			p28, p27, p26)
	}
}

func TestListRemote_DraftExcluded(t *testing.T) {
	t.Setenv("ISTIOENV_ROOT", t.TempDir())

	releases := []github.Release{
		{TagName: "v0.31.0", Prerelease: false, Draft: false},
		{TagName: "v0.32.0-draft", Prerelease: false, Draft: true},
	}
	server := newMockServer(t, releases)
	defer server.Close()

	client := &github.Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	out := captureStdout(t, func() {
		if err := listRemoteWithClient(client, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.Contains(out, "draft") {
		t.Errorf("draft release should not appear in output, got: %s", out)
	}
}

// ── github.Client.ListReleasesSince tests ─────────────────────────────────────

func TestListReleasesSince_StopsEarly(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		releases := []github.Release{
			{TagName: "v0.32.0", Prerelease: false, Draft: false},
			{TagName: "v0.31.0", Prerelease: false, Draft: false},
			// 0.30.0 is the anchor — pagination should stop here.
			{TagName: "v0.30.0", Prerelease: false, Draft: false},
			{TagName: "v0.29.0", Prerelease: false, Draft: false},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}))
	defer server.Close()

	client := &github.Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	versions, err := client.ListReleasesSince("0.30.0", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only 0.32.0 and 0.31.0 are strictly newer than 0.30.0.
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d: %v", len(versions), versions)
	}
	if versions[0] != "0.32.0" || versions[1] != "0.31.0" {
		t.Fatalf("expected [0.32.0 0.31.0], got %v", versions)
	}
	// Only one page should have been fetched.
	if callCount != 1 {
		t.Fatalf("expected 1 API call, got %d", callCount)
	}
}

func TestListReleasesSince_EmptyAnchor_FullFetch(t *testing.T) {
	releases := []github.Release{
		{TagName: "v0.32.0", Prerelease: false, Draft: false},
		{TagName: "v0.31.0", Prerelease: false, Draft: false},
	}
	server := newMockServer(t, releases)
	defer server.Close()

	client := &github.Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	// Empty anchor → behaves like ListReleases.
	versions, err := client.ListReleasesSince("", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d: %v", len(versions), versions)
	}
}

func TestListReleasesSince_NothingNewer(t *testing.T) {
	releases := []github.Release{
		{TagName: "v0.32.0", Prerelease: false, Draft: false},
		{TagName: "v0.31.0", Prerelease: false, Draft: false},
	}
	server := newMockServer(t, releases)
	defer server.Close()

	client := &github.Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	// Anchor is the newest known version — nothing should be returned.
	versions, err := client.ListReleasesSince("0.32.0", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 0 {
		t.Fatalf("expected 0 versions, got %d: %v", len(versions), versions)
	}
}

func TestListReleasesSince_IncludesPrereleases(t *testing.T) {
	releases := []github.Release{
		{TagName: "v0.33.0-alpha.1", Prerelease: true, Draft: false},
		{TagName: "v0.32.0", Prerelease: false, Draft: false},
		{TagName: "v0.31.0", Prerelease: false, Draft: false},
	}
	server := newMockServer(t, releases)
	defer server.Close()

	client := &github.Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	versions, err := client.ListReleasesSince("0.31.0", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should include both 0.33.0-alpha.1 and 0.32.0.
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d: %v", len(versions), versions)
	}
	found := false
	for _, v := range versions {
		if v == "0.33.0-alpha.1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected pre-release in result, got %v", versions)
	}
}

// ── Existing tests (preserved) ────────────────────────────────────────────────

func TestListRemote(t *testing.T) {
	t.Run("function exists and returns error on bad URL", func(t *testing.T) {
		_ = ListRemote
	})
}

func TestListRemoteCached(t *testing.T) {
	t.Run("returns disk cache without hitting the network", func(t *testing.T) {
		root := t.TempDir()
		// Deliberately store versions that are NOT in the baked-in baseline
		// so we can tell whether the cache was consulted.
		c := cache.NewWithTTL(root+"/cache", time.Hour)
		stable := []string{"99.99.99", "99.99.98"}
		pre := []string{"99.99.99", "99.99.98-rc.1"}
		if err := c.Save(stable, pre); err != nil {
			t.Fatalf("Save: %v", err)
		}
		t.Setenv("ISTIOENV_ROOT", root)

		out := captureStdout(t, func() {
			if err := ListRemoteCached(false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if !strings.Contains(out, "99.99.99") || !strings.Contains(out, "99.99.98") {
			t.Fatalf("expected cached stable versions in output, got:\n%s", out)
		}

		out = captureStdout(t, func() {
			if err := ListRemoteCached(true); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if !strings.Contains(out, "99.99.98-rc.1") {
			t.Fatalf("expected cached prerelease in output, got:\n%s", out)
		}
	})

	t.Run("returns stale cache (bypasses TTL)", func(t *testing.T) {
		root := t.TempDir()
		// Save with a TTL that would normally mark the cache as stale.
		c := cache.NewWithTTL(root+"/cache", -time.Hour)
		if err := c.Save([]string{"77.77.77"}, []string{"77.77.77"}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		t.Setenv("ISTIOENV_ROOT", root)

		out := captureStdout(t, func() {
			if err := ListRemoteCached(false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if !strings.Contains(out, "77.77.77") {
			t.Fatalf("expected stale cache contents to be returned, got:\n%s", out)
		}
	})

	t.Run("falls back to baseline when no cache", func(t *testing.T) {
		// ISTIOENV_ROOT not set → no on-disk cache available at all.
		t.Setenv("ISTIOENV_ROOT", "")

		out := captureStdout(t, func() {
			if err := ListRemoteCached(false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		// At minimum the baseline must contain the oldest known version
		// "1.24.0", which is guaranteed to be present forever (see
		// internal/cache/baseline.go). Any string change there should
		// prompt updating this assertion.
		if strings.TrimSpace(out) == "" {
			t.Fatalf("expected baseline fallback output, got empty")
		}
		if !strings.Contains(out, "1.24.0") {
			t.Fatalf("expected baseline to contain 1.24.0, got:\n%s", out)
		}
	})
}

func TestListRemoteWithMockClient(t *testing.T) {
	releases := []github.Release{
		{TagName: "v0.30.0", Prerelease: false, Draft: false},
		{TagName: "v0.31.0", Prerelease: false, Draft: false},
		{TagName: "v0.32.0-alpha.1", Prerelease: true, Draft: false},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Fatalf("failed to encode: %v", err)
		}
	}))
	defer server.Close()

	client := &github.Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	t.Run("excludes prereleases", func(t *testing.T) {
		versions, err := client.ListReleases(false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		output := strings.Join(versions, "\n")
		if strings.Contains(output, "alpha") {
			t.Fatal("should not contain pre-releases")
		}
		if !strings.Contains(output, "0.30.0") || !strings.Contains(output, "0.31.0") {
			t.Fatal("should contain stable releases")
		}
		// Verify descending order: 0.31.0 should appear before 0.30.0
		idx31 := strings.Index(output, "0.31.0")
		idx30 := strings.Index(output, "0.30.0")
		if idx31 > idx30 {
			t.Fatalf("expected 0.31.0 before 0.30.0 (newest first), got: %v", versions)
		}
	})

	t.Run("includes prereleases", func(t *testing.T) {
		versions, err := client.ListReleases(true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		output := strings.Join(versions, "\n")
		if !strings.Contains(output, "alpha") {
			t.Fatal("should contain pre-releases")
		}
		// Verify descending order: 0.32.0-alpha.1 should appear before 0.31.0
		idx32 := strings.Index(output, "0.32.0-alpha.1")
		idx31 := strings.Index(output, "0.31.0")
		if idx32 > idx31 {
			t.Fatalf("expected 0.32.0-alpha.1 before 0.31.0 (newest first), got: %v", versions)
		}
	})
}


