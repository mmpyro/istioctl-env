// Package github provides a client for interacting with the GitHub API
// to fetch istioctl release information.
package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/user/istioctl-env/internal/semver"
)

// Release represents a GitHub release.
type Release struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}

// Client is a GitHub API client for fetching istioctl releases.
type Client struct {
	BaseURL         string
	DownloadBaseURL string
	HTTPClient      *http.Client
}

// NewClient creates a new GitHub API client with default settings.
func NewClient() *Client {
	return &Client{
		BaseURL:         "https://api.github.com",
		DownloadBaseURL: "https://github.com",
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DownloadURL returns the full download URL given a path.
func (c *Client) DownloadURL(path string) string {
	return fmt.Sprintf("%s/%s", c.DownloadBaseURL, path)
}

// ListReleases fetches all istioctl releases from GitHub.
// If includePrerelease is false, pre-releases are filtered out.
func (c *Client) ListReleases(includePrerelease bool) ([]string, error) {
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
	url := fmt.Sprintf("%s/repos/istio/istio/releases/latest", c.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "istioctl-env")

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
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.BaseURL, ownerRepo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "istioctl-env")

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
func (c *Client) DownloadBinary(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "istioctl-env")

	// Use a dedicated client with a longer timeout for binary downloads.
	downloadClient := &http.Client{Timeout: 10 * time.Minute}
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
func (c *Client) DownloadWithProgress(url string, onProgress func(total, current int64)) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "istioctl-env")

	downloadClient := &http.Client{Timeout: 10 * time.Minute}
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
