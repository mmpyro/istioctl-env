package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// clearEnv resets the auth/offline/mirror env vars so a test runs with a
// clean slate regardless of what the parent process (or an earlier test)
// configured.
func clearEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ISTIOENV_GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("ISTIOENV_OFFLINE", "")
	t.Setenv("ISTIOENV_API_MIRROR", "")
	t.Setenv("ISTIOENV_DOWNLOAD_MIRROR", "")
	t.Setenv("ISTIOENV_MIRROR_URL", "")
}

func TestNewClient_EnvDefaults(t *testing.T) {
	clearEnv(t)
	c := NewClient()
	if c.Token != "" {
		t.Errorf("expected empty token, got %q", c.Token)
	}
	if c.Offline {
		t.Error("expected Offline=false by default")
	}
	if c.MirrorConfigured {
		t.Error("expected MirrorConfigured=false by default")
	}
	if c.APIMirrorConfigured {
		t.Error("expected APIMirrorConfigured=false by default")
	}
	if c.DownloadBaseURL != "https://github.com" {
		t.Errorf("expected default download base, got %q", c.DownloadBaseURL)
	}
	if c.BaseURL != "https://api.github.com" {
		t.Errorf("expected default API base, got %q", c.BaseURL)
	}
}

func TestNewClient_TokenEnvPrecedence(t *testing.T) {
	clearEnv(t)
	t.Setenv("GITHUB_TOKEN", "github-fallback")
	t.Setenv("ISTIOENV_GITHUB_TOKEN", "istioenv-primary")
	c := NewClient()
	if c.Token != "istioenv-primary" {
		t.Errorf("expected ISTIOENV_GITHUB_TOKEN to take precedence, got %q", c.Token)
	}

	clearEnv(t)
	t.Setenv("GITHUB_TOKEN", "github-fallback")
	c = NewClient()
	if c.Token != "github-fallback" {
		t.Errorf("expected GITHUB_TOKEN fallback, got %q", c.Token)
	}
}

func TestTokenHeaderInjection(t *testing.T) {
	clearEnv(t)

	var (
		gotAPIAuth      string
		gotAPIVersion   string
		gotDownloadAuth string
	)

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIAuth = r.Header.Get("Authorization")
		gotAPIVersion = r.Header.Get("X-GitHub-Api-Version")
		release := Release{TagName: "v1.0.0"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer apiServer.Close()

	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDownloadAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("payload"))
	}))
	defer downloadServer.Close()

	client := &Client{
		BaseURL:         apiServer.URL,
		DownloadBaseURL: downloadServer.URL,
		Token:           "secret-token",
		HTTPClient:      apiServer.Client(),
	}

	if _, err := client.GetLatestRelease(); err != nil {
		t.Fatalf("GetLatestRelease: %v", err)
	}
	if gotAPIAuth != "Bearer secret-token" {
		t.Errorf("expected Authorization 'Bearer secret-token' on API request, got %q", gotAPIAuth)
	}
	if gotAPIVersion != "2022-11-28" {
		t.Errorf("expected X-GitHub-Api-Version '2022-11-28', got %q", gotAPIVersion)
	}

	// Same-origin downloads carry the token (needed for private repos and
	// token-protected mirrors). Cross-origin stripping is covered by a
	// dedicated test below.
	if _, err := client.DownloadBinary(downloadServer.URL + "/foo.tar.gz"); err != nil {
		t.Fatalf("DownloadBinary: %v", err)
	}
	if gotDownloadAuth != "Bearer secret-token" {
		t.Errorf("expected Authorization 'Bearer secret-token' on same-origin download, got %q", gotDownloadAuth)
	}
}

func TestDownloadTokenNotSentCrossOrigin(t *testing.T) {
	clearEnv(t)

	// This server is on a *different* host than DownloadBaseURL and must
	// therefore never see the token.
	var gotForeignAuth string
	foreignServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotForeignAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("foreign-cdn-payload"))
	}))
	defer foreignServer.Close()

	client := &Client{
		// DownloadBaseURL is intentionally a different host than the one
		// we hit, so even the initial request is cross-origin and should
		// not carry the token.
		DownloadBaseURL: "https://downloads.example/istio",
		Token:           "very-secret",
		HTTPClient:      foreignServer.Client(),
	}

	if _, err := client.DownloadBinary(foreignServer.URL + "/foo.tar.gz"); err != nil {
		t.Fatalf("DownloadBinary: %v", err)
	}
	if gotForeignAuth != "" {
		t.Errorf("expected no Authorization on cross-origin download, got %q", gotForeignAuth)
	}
}

func TestDownloadTokenStrippedOnRedirect(t *testing.T) {
	clearEnv(t)

	// Simulate GitHub's behaviour: the release URL 302's to a signed S3
	// URL on a different host. The token must survive the first hop but
	// be stripped before the redirect lands on the "S3" host.
	var (
		gotS3Auth      string
		gotPrimaryAuth string
	)

	s3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotS3Auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("tarball"))
	}))
	defer s3.Close()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPrimaryAuth = r.Header.Get("Authorization")
		http.Redirect(w, r, s3.URL+"/signed-tarball", http.StatusFound)
	}))
	defer primary.Close()

	client := &Client{
		DownloadBaseURL: primary.URL,
		Token:           "redirect-token",
		HTTPClient:      primary.Client(),
	}

	data, err := client.DownloadBinary(primary.URL + "/releases/download/1.0.0/istioctl.tar.gz")
	if err != nil {
		t.Fatalf("DownloadBinary: %v", err)
	}
	if string(data) != "tarball" {
		t.Errorf("expected 'tarball', got %q", data)
	}

	if gotPrimaryAuth != "Bearer redirect-token" {
		t.Errorf("expected token on initial same-origin request, got %q", gotPrimaryAuth)
	}
	if gotS3Auth != "" {
		t.Errorf("expected Authorization to be stripped on cross-origin redirect, got %q", gotS3Auth)
	}
}

func TestTokenHeaderNotInjectedWhenEmpty(t *testing.T) {
	clearEnv(t)

	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		release := Release{TagName: "v1.0.0"}
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		// Token intentionally left empty.
	}
	if _, err := client.GetLatestRelease(); err != nil {
		t.Fatalf("GetLatestRelease: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("expected no Authorization header when token is empty, got %q", gotAuth)
	}
}

func TestDownloadMirrorEnvRewriting(t *testing.T) {
	clearEnv(t)
	t.Setenv("ISTIOENV_DOWNLOAD_MIRROR", "https://mirror.corp.example/istio")
	client := NewClient()

	if !client.MirrorConfigured {
		t.Fatal("expected MirrorConfigured=true when ISTIOENV_DOWNLOAD_MIRROR is set")
	}
	if client.DownloadBaseURL != "https://mirror.corp.example/istio" {
		t.Errorf("expected mirror base URL, got %q", client.DownloadBaseURL)
	}

	got := client.DownloadURL("istio/istio/releases/download/1.24.0/istioctl-1.24.0-osx-arm64.tar.gz")
	want := "https://mirror.corp.example/istio/istio/istio/releases/download/1.24.0/istioctl-1.24.0-osx-arm64.tar.gz"
	if got != want {
		t.Errorf("DownloadURL rewrite: expected %q, got %q", want, got)
	}
}

func TestDownloadMirrorTrimsTrailingSlashes(t *testing.T) {
	clearEnv(t)
	t.Setenv("ISTIOENV_DOWNLOAD_MIRROR", "https://mirror.corp.example/istio///")
	client := NewClient()
	if client.DownloadBaseURL != "https://mirror.corp.example/istio" {
		t.Errorf("expected trailing slashes trimmed, got %q", client.DownloadBaseURL)
	}
}

func TestLegacyMirrorURLStillWorks(t *testing.T) {
	clearEnv(t)
	t.Setenv("ISTIOENV_MIRROR_URL", "https://legacy.example/istio")

	client := NewClient()
	if !client.MirrorConfigured {
		t.Fatal("expected legacy ISTIOENV_MIRROR_URL to configure the download mirror")
	}
	if client.DownloadBaseURL != "https://legacy.example/istio" {
		t.Errorf("expected legacy mirror base, got %q", client.DownloadBaseURL)
	}
}

func TestDownloadMirrorPrefersNewEnvOverLegacy(t *testing.T) {
	clearEnv(t)
	t.Setenv("ISTIOENV_MIRROR_URL", "https://legacy.example/istio")
	t.Setenv("ISTIOENV_DOWNLOAD_MIRROR", "https://new.example/istio")

	client := NewClient()
	if client.DownloadBaseURL != "https://new.example/istio" {
		t.Errorf("ISTIOENV_DOWNLOAD_MIRROR should win over ISTIOENV_MIRROR_URL, got %q", client.DownloadBaseURL)
	}
}

func TestAPIMirrorEnvRewriting(t *testing.T) {
	clearEnv(t)
	t.Setenv("ISTIOENV_API_MIRROR", "https://ghapi.corp.example/")
	client := NewClient()

	if !client.APIMirrorConfigured {
		t.Fatal("expected APIMirrorConfigured=true when ISTIOENV_API_MIRROR is set")
	}
	if client.BaseURL != "https://ghapi.corp.example" {
		t.Errorf("expected API base URL to point at mirror and have trailing slash trimmed, got %q", client.BaseURL)
	}
}

func TestAPIMirrorReceivesRequests(t *testing.T) {
	clearEnv(t)

	var hits int
	apiMirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		// Return a trivial payload so the body parse succeeds.
		release := Release{TagName: "v9.9.9"}
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer apiMirror.Close()

	t.Setenv("ISTIOENV_API_MIRROR", apiMirror.URL)
	client := NewClient()
	client.HTTPClient = apiMirror.Client()

	v, err := client.GetLatestRelease()
	if err != nil {
		t.Fatalf("GetLatestRelease: %v", err)
	}
	if v != "9.9.9" {
		t.Errorf("expected version from mirror, got %q", v)
	}
	if hits == 0 {
		t.Error("expected the API mirror to be hit, but it wasn't")
	}
}

func TestIsOffline(t *testing.T) {
	for _, v := range []string{"1", "true", "yes", "on", "YES", "True"} {
		t.Run("truthy="+v, func(t *testing.T) {
			t.Setenv("ISTIOENV_OFFLINE", v)
			if !IsOffline() {
				t.Fatalf("expected IsOffline()=true for %q", v)
			}
		})
	}
	for _, v := range []string{"", "0", "false", "no", "nope"} {
		t.Run("falsy="+v, func(t *testing.T) {
			t.Setenv("ISTIOENV_OFFLINE", v)
			if IsOffline() {
				t.Fatalf("expected IsOffline()=false for %q", v)
			}
		})
	}
}

func TestOfflineModeReturnsErrOffline(t *testing.T) {
	clearEnv(t)
	t.Setenv("ISTIOENV_OFFLINE", "1")
	client := NewClient()
	if !client.Offline {
		t.Fatal("expected Offline=true when ISTIOENV_OFFLINE=1")
	}

	if _, err := client.ListReleases(false); !errors.Is(err, ErrOffline) {
		t.Errorf("ListReleases: expected ErrOffline, got %v", err)
	}
	if _, err := client.ListReleasesSince("1.0.0", false); !errors.Is(err, ErrOffline) {
		t.Errorf("ListReleasesSince: expected ErrOffline, got %v", err)
	}
	if _, err := client.ListReleasesSince("", false); !errors.Is(err, ErrOffline) {
		t.Errorf("ListReleasesSince(empty): expected ErrOffline, got %v", err)
	}
	if _, err := client.GetLatestRelease(); !errors.Is(err, ErrOffline) {
		t.Errorf("GetLatestRelease: expected ErrOffline, got %v", err)
	}
	if _, err := client.GetLatestReleaseFor("mmpyro/istioctl-env"); !errors.Is(err, ErrOffline) {
		t.Errorf("GetLatestReleaseFor: expected ErrOffline, got %v", err)
	}

	// Without a mirror, downloads must also return ErrOffline.
	if _, err := client.DownloadBinary("https://example.com/foo"); !errors.Is(err, ErrOffline) {
		t.Errorf("DownloadBinary: expected ErrOffline wrapped, got %v", err)
	}
	if _, err := client.DownloadWithProgress("https://example.com/foo", nil); !errors.Is(err, ErrOffline) {
		t.Errorf("DownloadWithProgress: expected ErrOffline wrapped, got %v", err)
	}
}

func TestOfflineDownloadErrMentionsMirror(t *testing.T) {
	clearEnv(t)
	t.Setenv("ISTIOENV_OFFLINE", "1")
	client := NewClient()
	_, err := client.DownloadBinary("https://example.com/foo")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ISTIOENV_DOWNLOAD_MIRROR") {
		t.Errorf("expected error to mention ISTIOENV_DOWNLOAD_MIRROR, got %v", err)
	}
}

func TestOfflineWithMirrorAllowsDownload(t *testing.T) {
	clearEnv(t)

	payload := []byte("mirror-payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	t.Setenv("ISTIOENV_OFFLINE", "1")
	t.Setenv("ISTIOENV_DOWNLOAD_MIRROR", server.URL)
	client := NewClient()
	if !client.Offline || !client.MirrorConfigured {
		t.Fatalf("expected Offline=true, MirrorConfigured=true (got %v / %v)", client.Offline, client.MirrorConfigured)
	}

	got, err := client.DownloadBinary(client.DownloadURL("istio/istio/releases/download/1.24.0/istioctl.tar.gz"))
	if err != nil {
		t.Fatalf("DownloadBinary: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("expected %q, got %q", payload, got)
	}
}
