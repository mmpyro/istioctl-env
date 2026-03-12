package platform

import "fmt"

// SelfDownloadURL returns the GitHub release download URL for an istioctl-env binary
// built for the given version and platform.
//
// Asset naming convention: istioctl-env-{os}-{arch}
// Example: https://github.com/mmpyro/istioctl-env/releases/download/v0.2.0/istioctl-env-darwin-arm64
func SelfDownloadURL(version string, info Info, ownerRepo string) string {
	return fmt.Sprintf(
		"https://github.com/%s/releases/download/v%s/istioctl-env-%s-%s",
		ownerRepo, version, info.OS, info.Arch,
	)
}
