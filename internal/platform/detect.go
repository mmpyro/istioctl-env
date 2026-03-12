// Package platform provides OS and architecture detection for downloading
// the correct istioctl binary.
package platform

import (
	"fmt"
	"runtime"
)

// Info holds the detected platform information.
type Info struct {
	OS   string
	Arch string
}

// Detect returns the current platform's OS and architecture.
func Detect() (Info, error) {
	osName, err := mapOS(runtime.GOOS)
	if err != nil {
		return Info{}, err
	}
	archName, err := mapArch(runtime.GOARCH)
	if err != nil {
		return Info{}, err
	}
	return Info{OS: osName, Arch: archName}, nil
}

// mapOS maps Go's runtime.GOOS to the istioctl release naming convention.
// Istio uses "osx" instead of "darwin".
func mapOS(goos string) (string, error) {
	switch goos {
	case "linux":
		return "linux", nil
	case "darwin":
		return "osx", nil
	default:
		return "", fmt.Errorf("unsupported operating system: %s", goos)
	}
}

// mapArch maps Go's runtime.GOARCH to the istioctl release naming convention.
func mapArch(goarch string) (string, error) {
	switch goarch {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", goarch)
	}
}

// DownloadPath returns the path part of the GitHub release download URL.
// Istio release tags do NOT use a "v" prefix.
// Format: istio/istio/releases/download/{version}/istioctl-{version}-{os}-{arch}.tar.gz
func DownloadPath(version string, info Info) string {
	return fmt.Sprintf(
		"istio/istio/releases/download/%s/%s",
		version, ArchiveName(version, info),
	)
}

// ArchiveName returns the istioctl archive name for the given version and platform.
func ArchiveName(version string, info Info) string {
	return fmt.Sprintf("istioctl-%s-%s-%s.tar.gz", version, info.OS, info.Arch)
}

// BinaryName returns the name of the istioctl binary inside the archive.
func BinaryName(info Info) string {
	return "istioctl"
}

// ChecksumPath returns the path part of the URL for the per-file .sha256 checksum.
// Istio uses per-file .sha256 files (single hash, no filename).
func ChecksumPath(version string, info Info) string {
	return fmt.Sprintf(
		"istio/istio/releases/download/%s/%s.sha256",
		version, ArchiveName(version, info),
	)
}
