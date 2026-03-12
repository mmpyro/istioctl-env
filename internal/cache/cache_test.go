package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func writeCacheFile(t *testing.T, dir string, e entry) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, cacheFileName), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// ── Load ─────────────────────────────────────────────────────────────────────

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	_, _, ok := c.Load()
	if ok {
		t.Fatal("expected cache miss for missing file, got hit")
	}
}

func TestLoad_FreshCache(t *testing.T) {
	dir := t.TempDir()
	e := entry{
		FetchedAt:          time.Now().UTC(),
		Versions:           []string{"0.22.0", "0.21.0"},
		PrereleaseVersions: []string{"0.22.0", "0.22.0-alpha.1", "0.21.0"},
	}
	writeCacheFile(t, dir, e)

	c := New(dir)
	versions, prereleaseVersions, ok := c.Load()
	if !ok {
		t.Fatal("expected cache hit for fresh file, got miss")
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 stable versions, got %d", len(versions))
	}
	if len(prereleaseVersions) != 3 {
		t.Fatalf("expected 3 prerelease versions, got %d", len(prereleaseVersions))
	}
	if versions[0] != "0.22.0" {
		t.Fatalf("expected first version 0.22.0, got %s", versions[0])
	}
}

func TestLoad_StaleCache(t *testing.T) {
	dir := t.TempDir()
	e := entry{
		// Fetched 2 hours ago — older than the default 1 h TTL.
		FetchedAt:          time.Now().UTC().Add(-2 * time.Hour),
		Versions:           []string{"0.22.0"},
		PrereleaseVersions: []string{"0.22.0"},
	}
	writeCacheFile(t, dir, e)

	c := New(dir)
	_, _, ok := c.Load()
	if ok {
		t.Fatal("expected cache miss for stale file, got hit")
	}
}

func TestLoad_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, cacheFileName), []byte("not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	c := New(dir)
	_, _, ok := c.Load()
	if ok {
		t.Fatal("expected cache miss for corrupt file, got hit")
	}
}

func TestLoad_EmptyDir(t *testing.T) {
	// dir == "" means memory-only mode.
	c := New("")
	_, _, ok := c.Load()
	if ok {
		t.Fatal("expected cache miss for empty dir, got hit")
	}
}

func TestLoad_CustomTTL(t *testing.T) {
	dir := t.TempDir()
	e := entry{
		// Fetched 90 seconds ago.
		FetchedAt:          time.Now().UTC().Add(-90 * time.Second),
		Versions:           []string{"0.22.0"},
		PrereleaseVersions: []string{"0.22.0"},
	}
	writeCacheFile(t, dir, e)

	// With a 2-minute TTL the cache should still be fresh.
	c := NewWithTTL(dir, 2*time.Minute)
	_, _, ok := c.Load()
	if !ok {
		t.Fatal("expected cache hit with 2-minute TTL, got miss")
	}

	// With a 1-minute TTL the cache should be stale.
	c2 := NewWithTTL(dir, 1*time.Minute)
	_, _, ok2 := c2.Load()
	if ok2 {
		t.Fatal("expected cache miss with 1-minute TTL, got hit")
	}
}

// ── Save ─────────────────────────────────────────────────────────────────────

func TestSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	stable := []string{"0.22.0", "0.21.0"}
	pre := []string{"0.22.0", "0.22.0-alpha.1", "0.21.0"}

	if err := c.Save(stable, pre); err != nil {
		t.Fatalf("Save: %v", err)
	}

	versions, prereleaseVersions, ok := c.Load()
	if !ok {
		t.Fatal("expected cache hit after Save, got miss")
	}
	if len(versions) != len(stable) {
		t.Fatalf("expected %d stable versions, got %d", len(stable), len(versions))
	}
	if len(prereleaseVersions) != len(pre) {
		t.Fatalf("expected %d prerelease versions, got %d", len(pre), len(prereleaseVersions))
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	base := t.TempDir()
	// Use a nested directory that does not yet exist.
	dir := filepath.Join(base, "nested", "cache")
	c := New(dir)

	if err := c.Save([]string{"0.22.0"}, []string{"0.22.0"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, cacheFileName)); err != nil {
		t.Fatalf("cache file not created: %v", err)
	}
}

func TestSave_EmptyDir_NoOp(t *testing.T) {
	c := New("")
	// Should not error even though there is nowhere to write.
	if err := c.Save([]string{"0.22.0"}, []string{"0.22.0"}); err != nil {
		t.Fatalf("Save with empty dir should be a no-op, got: %v", err)
	}
}

// ── MergeWithBaseline ─────────────────────────────────────────────────────────

func TestMergeWithBaseline_DeduplicatesAndSorts(t *testing.T) {
	// Simulate a delta that overlaps with the baseline.
	delta := []string{"1.26.0", "1.25.0"} // 1.25.0 is already in baseline
	merged := MergeWithBaseline(delta, false)

	// 1.25.0 should appear exactly once.
	count := 0
	for _, v := range merged {
		if v == "1.25.0" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1.25.0 exactly once, got %d occurrences", count)
	}

	// 1.26.0 should be first (newest).
	if len(merged) == 0 || merged[0] != "1.26.0" {
		t.Fatalf("expected 1.26.0 as newest, got %v", merged)
	}
}

func TestMergeWithBaseline_EmptyDelta(t *testing.T) {
	merged := MergeWithBaseline(nil, false)
	if len(merged) == 0 {
		t.Fatal("expected non-empty result from baseline alone")
	}
}

// ── MergeWithCached ───────────────────────────────────────────────────────────

func TestMergeWithCached_DeduplicatesAndSorts(t *testing.T) {
	cached := []string{"0.22.0", "0.21.0"}
	delta := []string{"0.23.0", "0.22.0"} // 0.22.0 is a duplicate
	merged := MergeWithCached(cached, delta)

	if len(merged) != 3 {
		t.Fatalf("expected 3 unique versions, got %d: %v", len(merged), merged)
	}
	if merged[0] != "0.23.0" {
		t.Fatalf("expected 0.23.0 first, got %s", merged[0])
	}
}

// ── NewestVersion ─────────────────────────────────────────────────────────────

func TestNewestVersion(t *testing.T) {
	if v := NewestVersion([]string{"0.22.0", "0.21.0"}); v != "0.22.0" {
		t.Fatalf("expected 0.22.0, got %s", v)
	}
	if v := NewestVersion(nil); v != "" {
		t.Fatalf("expected empty string for nil slice, got %s", v)
	}
}

// ── Dir ───────────────────────────────────────────────────────────────────────

func TestDir(t *testing.T) {
	c := New("/some/path")
	if c.Dir() != "/some/path" {
		t.Fatalf("expected /some/path, got %s", c.Dir())
	}
}
