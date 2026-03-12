package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListReleases(t *testing.T) {
	releases := []Release{
		{TagName: "v0.30.0", Prerelease: false, Draft: false},
		{TagName: "v0.31.0", Prerelease: false, Draft: false},
		{TagName: "v0.32.0-alpha.1", Prerelease: true, Draft: false},
		{TagName: "v0.32.0", Prerelease: false, Draft: false},
		{TagName: "v0.33.0-draft", Prerelease: false, Draft: true},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Fatalf("failed to encode releases: %v", err)
		}
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	t.Run("excludes prereleases by default", func(t *testing.T) {
		versions, err := client.ListReleases(false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Expect newest-to-oldest (descending semver order)
		expected := []string{"0.32.0", "0.31.0", "0.30.0"}
		if len(versions) != len(expected) {
			t.Fatalf("expected %d versions, got %d: %v", len(expected), len(versions), versions)
		}
		for i, v := range versions {
			if v != expected[i] {
				t.Fatalf("expected %s at index %d, got %s", expected[i], i, v)
			}
		}
	})

	t.Run("includes prereleases when requested", func(t *testing.T) {
		versions, err := client.ListReleases(true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Expect newest-to-oldest; pre-release 0.32.0-alpha.1 < 0.32.0 so it comes after
		expected := []string{"0.32.0", "0.32.0-alpha.1", "0.31.0", "0.30.0"}
		if len(versions) != len(expected) {
			t.Fatalf("expected %d versions, got %d: %v", len(expected), len(versions), versions)
		}
		for i, v := range versions {
			if v != expected[i] {
				t.Fatalf("expected %s at index %d, got %s", expected[i], i, v)
			}
		}
	})
}

func TestListReleasesPagination(t *testing.T) {
	page1 := []Release{
		{TagName: "v0.30.0", Prerelease: false, Draft: false},
	}
	page2 := []Release{
		{TagName: "v0.31.0", Prerelease: false, Draft: false},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")

		switch page {
		case "", "1":
			// Set Link header pointing to page 2
			nextURL := fmt.Sprintf("<%s/repos/istio/istio/releases?per_page=100&page=2>; rel=\"next\"", r.URL.Scheme+"://"+r.Host)
			w.Header().Set("Link", nextURL)
			if err := json.NewEncoder(w).Encode(page1); err != nil {
				t.Fatalf("failed to encode: %v", err)
			}
		case "2":
			if err := json.NewEncoder(w).Encode(page2); err != nil {
				t.Fatalf("failed to encode: %v", err)
			}
		}
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	versions, err := client.ListReleases(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect newest-to-oldest (descending semver order)
	expected := []string{"0.31.0", "0.30.0"}
	if len(versions) != len(expected) {
		t.Fatalf("expected %d versions, got %d: %v", len(expected), len(versions), versions)
	}
}

func TestGetLatestRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		release := Release{TagName: "v0.32.0", Prerelease: false, Draft: false}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(release); err != nil {
			t.Fatalf("failed to encode: %v", err)
		}
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	version, err := client.GetLatestRelease()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "0.32.0" {
		t.Fatalf("expected 0.32.0, got %s", version)
	}
}

func TestGetLatestReleaseRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	_, err := client.GetLatestRelease()
	if err == nil {
		t.Fatal("expected error on rate limit")
	}
}

func TestDownloadBinary(t *testing.T) {
	expectedData := []byte("fake-binary-data")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(expectedData)
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	data, err := client.DownloadBinary(server.URL + "/download")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != string(expectedData) {
		t.Fatalf("expected %s, got %s", expectedData, data)
	}
}

func TestDownloadBinaryNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	_, err := client.DownloadBinary(server.URL + "/download")
	if err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestParseNextPageURL(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "empty header",
			header:   "",
			expected: "",
		},
		{
			name:     "with next link",
			header:   `<https://api.github.com/repos/istio/istio/releases?page=2>; rel="next", <https://api.github.com/repos/istio/istio/releases?page=5>; rel="last"`,
			expected: "https://api.github.com/repos/istio/istio/releases?page=2",
		},
		{
			name:     "no next link",
			header:   `<https://api.github.com/repos/istio/istio/releases?page=1>; rel="prev"`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNextPageURL(tt.header)
			if result != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
