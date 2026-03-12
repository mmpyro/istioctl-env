package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/user/istioctl-env/internal/cache"
	"github.com/user/istioctl-env/internal/github"
)

func TestLatest(t *testing.T) {
	// Just verify the function signature exists.
	t.Run("function exists", func(t *testing.T) {
		_ = Latest
	})
}

func TestLatestWithMockClient(t *testing.T) {
	releases := []github.Release{
		{TagName: "v1.27.0-alpha.1", Prerelease: true, Draft: false},
		{TagName: "v1.26.0", Prerelease: false, Draft: false},
		{TagName: "v1.25.0", Prerelease: false, Draft: false},
	}

	t.Run("latest stable version uses cache/network", func(t *testing.T) {
		// Isolate disk cache so we don't load a real on-disk cache from $ISTIOENV_ROOT.
		t.Setenv("ISTIOENV_ROOT", t.TempDir())

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

		out := captureStdout(t, func() {
			if err := latestWithClient(client, false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if strings.TrimSpace(out) != "1.26.0" {
			t.Fatalf("expected 1.26.0, got %q", strings.TrimSpace(out))
		}
	})

	t.Run("latest with prerelease returns newest including alpha", func(t *testing.T) {
		// Isolate disk cache so we don't load a real on-disk cache from $ISTIOENV_ROOT.
		t.Setenv("ISTIOENV_ROOT", t.TempDir())

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

		out := captureStdout(t, func() {
			if err := latestWithClient(client, true); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if strings.TrimSpace(out) != "1.27.0-alpha.1" {
			t.Fatalf("expected 1.27.0-alpha.1, got %q", strings.TrimSpace(out))
		}
	})

	t.Run("latest respects fresh cache", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", root)
		c := cache.NewWithTTL(root+"/cache", time.Hour)
		// Cache has 0.99.0 as latest
		if err := c.Save([]string{"0.99.0"}, []string{"0.99.0"}); err != nil {
			t.Fatalf("Save: %v", err)
		}

		// Use a client that fails — it should NOT be called because cache is fresh
		failClient := &github.Client{
			BaseURL:    "http://127.0.0.1:0",
			HTTPClient: &http.Client{Timeout: 100 * time.Millisecond},
		}

		out := captureStdout(t, func() {
			if err := latestWithClient(failClient, false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if strings.TrimSpace(out) != "0.99.0" {
			t.Fatalf("expected version from cache 0.99.0, got %q", strings.TrimSpace(out))
		}
	})
}
