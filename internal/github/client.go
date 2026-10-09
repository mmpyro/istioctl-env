// Package github provides a client for interacting with the GitHub API
// to fetch istioctl release information.
package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/user/istioctl-env/internal/semver"
)

// ErrOffline is the sentinel error returned by Client methods when
// ISTIOENV_OFFLINE=1 is set and the operation would otherwise require
// network access.
var ErrOffline = errors.New("istioctl-env: offline mode (ISTIOENV_OFFLINE=1); network access disabled")

// Default upstreams. These are used unless the caller sets a mirror via the
// ISTIOENV_API_MIRROR / ISTIOENV_DOWNLOAD_MIRROR (or legacy
// ISTIOENV_MIRROR_URL) environment variables.
const (
	defaultAPIBaseURL      = "https://api.github.com"
	defaultDownloadBaseURL = "https://github.com"
)

// Env var names. Kept as constants so tests and documentation stay in sync
// with the code.
const (
	envGitHubToken     = "ISTIOENV_GITHUB_TOKEN"
	envGitHubTokenAlt  = "GITHUB_TOKEN"
	envOffline         = "ISTIOENV_OFFLINE"
	envAPIMirror       = "ISTIOENV_API_MIRROR"
	envDownloadMirror  = "ISTIOENV_DOWNLOAD_MIRROR"
	envMirrorLegacyURL = "ISTIOENV_MIRROR_URL"
)

// IsOffline reports whether the ISTIOENV_OFFLINE environment variable is
// set to a truthy value ("1", "true", "yes", "on"; case-insensitive).
func IsOffline() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(envOffline)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// resolveAPIBaseURL returns the effective API base URL and whether an API
// mirror override is in effect. The returned base URL never contains a
// trailing slash.
func resolveAPIBaseURL() (base string, mirror bool) {
	raw := strings.TrimSpace(os.Getenv(envAPIMirror))
	if raw == "" {
		return defaultAPIBaseURL, false
	}
	return strings.TrimRight(raw, "/"), true
}

// resolveDownloadBaseURL returns the effective download base URL and whether
// a mirror override is in effect. It reads ISTIOENV_DOWNLOAD_MIRROR first and
// falls back to the deprecated ISTIOENV_MIRROR_URL for backward compatibility.
// The returned base URL never contains a trailing slash.
func resolveDownloadBaseURL() (base string, mirror bool) {
	raw := strings.TrimSpace(os.Getenv(envDownloadMirror))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv(envMirrorLegacyURL))
	}
	if raw == "" {
		return defaultDownloadBaseURL, false
	}
	return strings.TrimRight(raw, "/"), true
}

// Release represents a GitHub release.
type Release struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}

// Client is a GitHub API client for fetching istioctl releases.
//
// Behavior is influenced by the following environment variables (all read
// at NewClient time):
//
//   - ISTIOENV_GITHUB_TOKEN or GITHUB_TOKEN — populates Token. When set, API
//     requests send Authorization: Bearer. Release-asset downloads also send
//     it, but only on the initial, same-origin request; cross-origin
//     redirects (e.g. to objects.githubusercontent.com or an unrelated CDN)
//     have the header stripped automatically so the token cannot leak to a
//     third party.
//   - ISTIOENV_OFFLINE=1 (true/yes/on) — populates Offline. API methods
//     short-circuit with ErrOffline; download methods also return ErrOffline
//     unless a download mirror is configured.
//   - ISTIOENV_API_MIRROR — populates BaseURL and APIMirrorConfigured.
//     Overrides https://api.github.com. The mirror must speak the GitHub
//     REST API (/repos/.../releases) and be reachable from the host.
//   - ISTIOENV_DOWNLOAD_MIRROR — populates DownloadBaseURL and
//     MirrorConfigured. Overrides https://github.com. The mirror must serve
//     both archives and the per-file .sha256 at the same relative path as
//     GitHub releases.
//   - ISTIOENV_MIRROR_URL — deprecated alias for ISTIOENV_DOWNLOAD_MIRROR.
//     Kept for backward compatibility; a warning may be printed to stderr
//     in a future release.
type Client struct {
	BaseURL             string
	DownloadBaseURL     string
	Token               string
	Offline             bool
	APIMirrorConfigured bool
	MirrorConfigured    bool
	HTTPClient          *http.Client
}

// NewClient creates a new GitHub API client with default settings and
// environment-aware auth, offline, and mirror configuration.
func NewClient() *Client {
	token := os.Getenv(envGitHubToken)
	if token == "" {
		token = os.Getenv(envGitHubTokenAlt)
	}

	apiBase, apiMirror := resolveAPIBaseURL()
	downloadBase, mirror := resolveDownloadBaseURL()

	return &Client{
		BaseURL:             apiBase,
		DownloadBaseURL:     downloadBase,
		Token:               token,
		Offline:             IsOffline(),
		APIMirrorConfigured: apiMirror,
		MirrorConfigured:    mirror,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DownloadURL returns the full download URL for the given relative path.
// The DownloadBaseURL is used (which honors ISTIOENV_DOWNLOAD_MIRROR when set).
func (c *Client) DownloadURL(path string) string {
	base := strings.TrimRight(c.DownloadBaseURL, "/")
	return fmt.Sprintf("%s/%s", base, path)
}

// authorizeAPIRequest attaches the GitHub auth and API-version headers to
// requests whose host matches the configured API BaseURL. Requests to any
// other host are left untouched so a potentially sensitive token is not
// sent through a redirect chain.
func (c *Client) authorizeAPIRequest(req *http.Request) {
	if c.Token == "" {
		return
	}
	apiURL, err := url.Parse(c.BaseURL)
	if err != nil {
		return
	}
	if req.URL.Host != apiURL.Host {
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

// authorizeDownloadRequest attaches the Authorization header to a download
// request whose host matches the configured DownloadBaseURL (i.e. the
// upstream github.com or the explicitly configured mirror). It is a no-op
// for any other host so the token cannot leak to an unrelated CDN.
func (c *Client) authorizeDownloadRequest(req *http.Request) {
	if c.Token == "" {
		return
	}
	dlURL, err := url.Parse(c.DownloadBaseURL)
	if err != nil {
		return
	}
	if req.URL.Host != dlURL.Host {
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
}

// stripAuthOnCrossOriginRedirect is a http.Client.CheckRedirect function
// that removes the Authorization header whenever the redirect target lives
// on a different host than the request that was issued one hop ago. This
// mirrors curl's --location-trusted vs. --location semantics and is the
// recommended pattern for GitHub release downloads, which 302 to a signed
// S3 URL that rejects the Bearer token.
func stripAuthOnCrossOriginRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	last := via[len(via)-1]
	if req.URL.Host != last.URL.Host {
		req.Header.Del("Authorization")
	}
	// Follow the standard library's default of giving up after 10 redirects.
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

// offlineDownloadErr returns an ErrOffline-wrapping error that points the
// user to the download-mirror env var. Called by download methods when
// offline mode is active but no mirror is configured.
func offlineDownloadErr() error {
	return fmt.Errorf("%w: configure %s to enable offline downloads", ErrOffline, envDownloadMirror)
}

// newDownloadClient builds the http.Client used by DownloadBinary and
// DownloadWithProgress. The timeout is intentionally much higher than the
// API client (large archives on slow corporate links).
func (c *Client) newDownloadClient() *http.Client {
	return &http.Client{
		Timeout:       10 * time.Minute,
		CheckRedirect: stripAuthOnCrossOriginRedirect,
	}
}

// ListReleases fetches all istioctl releases from GitHub.
// If includePrerelease is false, pre-releases are filtered out.
func (c *Client) ListReleases(includePrerelease bool) ([]string, error) {
	if c.Offline {
		return nil, ErrOffline
	}

	var allReleases []Release
	page := 1

	for {
		url := fmt.Sprintf("%s/repos/istio/istio/releases?per_page=100&page=%d", c.BaseURL, page)
		releases, nextPage, err := c.fetchReleasesPage(url)
		if err != nil {
			return nil, err
		}

		allReleases = append(allReleases, releases...)

		if nextPage == "" {
			break
		}
		page++
	}

	var versions []string
	for _, r := range allReleases {
		if r.Draft {
			continue
		}
		if !includePrerelease && r.Prerelease {
			continue
		}
		version := strings.TrimPrefix(r.TagName, "v")
		if version != "" {
			versions = append(versions, version)
		}
	}

	return semver.SortDescending(versions), nil
}

// ListReleasesSince fetches only the istioctl releases that are strictly newer
// than sinceVersion (e.g. "0.21.0").  It stops paginating as soon as it
// encounters a version that is ≤ sinceVersion, so it typically needs only one
// API page when few or no new releases exist.
//
// If sinceVersion is empty the behaviour is identical to ListReleases.
// If includePrerelease is false, pre-releases are filtered out of the result
// (but they are still used as stop-markers during pagination).
func (c *Client) ListReleasesSince(sinceVersion string, includePrerelease bool) ([]string, error) {
	if c.Offline {
		return nil, ErrOffline
	}

	// Fast path: no anchor version — fall back to a full fetch.
	if sinceVersion == "" {
		return c.ListReleases(includePrerelease)
	}

	anchor := semver.Parse(sinceVersion)

	var collected []string
	page := 1

	for {
		url := fmt.Sprintf("%s/repos/istio/istio/releases?per_page=100&page=%d", c.BaseURL, page)
		releases, nextPage, err := c.fetchReleasesPage(url)
		if err != nil {
			return nil, err
		}

		done := false
		for _, r := range releases {
			if r.Draft {
				continue
			}
			v := strings.TrimPrefix(r.TagName, "v")
			if v == "" {
				continue
			}
			parsed := semver.Parse(v)
			// GitHub returns releases newest-first.  Stop as soon as we reach
			// a version that is not newer than the anchor.
			if !semver.Less(anchor, parsed) {
				done = true
				break
			}
			if !includePrerelease && r.Prerelease {
				continue
			}
			collected = append(collected, v)
		}

		if done || nextPage == "" {
			break
		}
		page++
	}

	return semver.SortDescending(collected), nil
}

// GetLatestRelease fetches the latest stable release version.
func (c *Client) GetLatestRelease() (string, error) {
	if c.Offline {
		return "", ErrOffline
	}

	url := fmt.Sprintf("%s/repos/istio/istio/releases/latest", c.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "istioctl-env")
	c.authorizeAPIRequest(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("GitHub API rate limit exceeded. Please try again later")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release: %w", err)
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

// GetLatestReleaseFor fetches the latest stable release for the given
// GitHub owner/repo (e.g. "mmpyro/istioctl-env").
func (c *Client) GetLatestReleaseFor(ownerRepo string) (string, error) {
	if c.Offline {
		return "", ErrOffline
	}

	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.BaseURL, ownerRepo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "istioctl-env")
	c.authorizeAPIRequest(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("GitHub API rate limit exceeded. Please try again later")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release: %w", err)
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

// fetchReleasesPage fetches a single page of releases and returns the next page URL.
func (c *Client) fetchReleasesPage(url string) ([]Release, string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "istioctl-env")
	c.authorizeAPIRequest(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, "", fmt.Errorf("GitHub API rate limit exceeded. Please try again later")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, "", fmt.Errorf("failed to parse releases: %w", err)
	}

	nextPage := parseNextPageURL(resp.Header.Get("Link"))
	return releases, nextPage, nil
}

// parseNextPageURL extracts the next page URL from the Link header.
func parseNextPageURL(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	// Link header format: <url>; rel="next", <url>; rel="last"
	re := regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)
	matches := re.FindStringSubmatch(linkHeader)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// DownloadBinary downloads a binary from the given URL and returns its contents.
// It uses a longer timeout than the default API client to accommodate large binaries.
//
// If a token is configured and the URL host matches the configured
// DownloadBaseURL, Authorization: Bearer is attached. The header is stripped
// automatically on any cross-origin redirect (via CheckRedirect) so it cannot
// leak to a CDN.
//
// In offline mode without a configured mirror the method returns an error
// that wraps ErrOffline and directs the user to set ISTIOENV_DOWNLOAD_MIRROR.
func (c *Client) DownloadBinary(url string) ([]byte, error) {
	if c.Offline && !c.MirrorConfigured {
		return nil, offlineDownloadErr()
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "istioctl-env")
	c.authorizeDownloadRequest(req)

	downloadClient := c.newDownloadClient()
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("binary not found at %s. Check that the version exists", url)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read download: %w", err)
	}

	return data, nil
}

// DownloadWithProgress downloads a file from the given URL and reports progress via a callback.
//
// Authentication semantics match DownloadBinary: Authorization is attached
// only when the URL host matches DownloadBaseURL, and is stripped on
// cross-origin redirects.
//
// In offline mode without a configured mirror the method returns an error
// that wraps ErrOffline and directs the user to set ISTIOENV_DOWNLOAD_MIRROR.
func (c *Client) DownloadWithProgress(url string, onProgress func(total, current int64)) ([]byte, error) {
	if c.Offline && !c.MirrorConfigured {
		return nil, offlineDownloadErr()
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "istioctl-env")
	c.authorizeDownloadRequest(req)

	downloadClient := c.newDownloadClient()
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("file not found at %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	total := resp.ContentLength
	var current int64
	var buf bytes.Buffer

	// Create a proxy reader to track progress
	chunk := make([]byte, 32*1024) // 32KB chunks
	for {
		n, err := resp.Body.Read(chunk)
		if n > 0 {
			current += int64(n)
			buf.Write(chunk[:n])
			if onProgress != nil {
				onProgress(total, current)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
	}

	return buf.Bytes(), nil
}
