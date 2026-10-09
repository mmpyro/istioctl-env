package platform

import (
	"fmt"
	"strings"
)

// DefaultSelfDownloadBaseURL is the upstream GitHub host used to download
// istioctl-env release artifacts when no mirror is configured.
const DefaultSelfDownloadBaseURL = "https://github.com"

// SelfDownloadURL returns the release download URL for an istioctl-env
// binary built for the given version and platform, rooted at baseURL.
//
// Pass DefaultSelfDownloadBaseURL (or an empty string, which is treated as
// the default) to use upstream GitHub. Pass a mirror base to retarget the
// download — for example an internal corporate mirror configured via
// ISTIOENV_DOWNLOAD_MIRROR.
//
// Asset naming convention: istioctl-env-{os}-{arch}
// Example: https://github.com/mmpyro/istioctl-env/releases/download/v0.2.0/istioctl-env-darwin-arm64
func SelfDownloadURL(baseURL, version string, info Info, ownerRepo string) string {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = DefaultSelfDownloadBaseURL
	}
	return fmt.Sprintf(
		"%s/%s/releases/download/v%s/istioctl-env-%s-%s",
		base, ownerRepo, version, info.OS, info.Arch,
	)
}
