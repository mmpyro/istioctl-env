package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/user/istioctl-env/internal/config"
	"github.com/user/istioctl-env/internal/github"
	"github.com/user/istioctl-env/internal/platform"
)

// Install downloads and installs a specific istioctl version or the best
// match for a SemVer constraint.
//
// Behaviour:
//
//   - If version is a plain exact pin (e.g. "1.24.0"), that version is
//     downloaded directly.
//   - If version is empty, the nearest .istioctl-version is consulted;
//     when no such file is present, "latest" is used.
//   - If version is a constraint ("^1.24.0", ">=1.24.0 <1.26.0", "latest",
//     "latest-prerelease", ...), the remote release list is consulted and
//     the newest matching version is installed.
func Install(version string, silent bool) error {
	client := github.NewClient()
	return installWithClient(client, version, silent)
}

func installWithClient(client *github.Client, version string, silent bool) error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	// When no version is specified, resolution falls back to "latest" (or
	// the nearest .istioctl-version). Latest resolution needs the GitHub
	// API, so bail out early with a clear message when running offline
	// rather than letting the fallback silently install the hardcoded
	// baseline version.
	if strings.TrimSpace(version) == "" && github.IsOffline() {
		return fmt.Errorf("cannot determine latest version while ISTIOENV_OFFLINE=1; please specify a version explicitly")
	}

	resolved, raw, err := resolveSpecForInstall(client, version)
	if err != nil {
		return err
	}
	if !silent && raw != resolved {
		fmt.Printf("Resolved %s → %s\n", raw, resolved)
	}
	// From here on, `version` is the concrete resolved version.
	version = resolved

	// Check if already installed
	installed, err := config.IsVersionInstalled(version)
	if err != nil {
		return err
	}
	if installed {
		if !silent {
			fmt.Printf("version %s already installed skipping\n", version)
		}
		return nil
	}

	// Detect platform
	info, err := platform.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect platform: %w", err)
	}

	// Construct download URL
	url := client.DownloadURL(platform.DownloadPath(version, info))
	if !silent {
		fmt.Printf("Downloading istioctl %s for %s/%s...\n", version, info.OS, info.Arch)
	}

	// Download archive with progress
	var data []byte
	if silent {
		data, err = client.DownloadBinary(url)
	} else {
		data, err = client.DownloadWithProgress(url, func(total, current int64) {
			if total <= 0 {
				fmt.Printf("\rDownloaded: %d bytes", current)
				return
			}
			percent := float64(current) / float64(total) * 100
			blocks := int(percent / 2) // 50 blocks
			bar := strings.Repeat("#", blocks) + strings.Repeat(" ", 50-blocks)
			fmt.Printf("\r[%s] %.0f%%", bar, percent)
			if current == total {
				fmt.Println()
			}
		})
	}
	if err != nil {
		return fmt.Errorf("failed to download istioctl %s: %w", version, err)
	}

	// Checksum validation
	// Istio uses per-file .sha256 files (single hash, no filename)
	checksumPath := platform.ChecksumPath(version, info)
	checksumUrl := client.DownloadURL(checksumPath)
	checksumData, err := client.DownloadBinary(checksumUrl)
	if err != nil {
		if !silent {
			fmt.Printf("Warning: could not download checksum for version %s: %v\n", version, err)
		}
	} else {
		expectedChecksum := strings.TrimSpace(string(checksumData))
		// Istio .sha256 files may contain just the hash, or "hash  filename"
		if parts := strings.Fields(expectedChecksum); len(parts) > 0 {
			expectedChecksum = parts[0]
		}
		actualChecksum := sha256.Sum256(data)
		actualChecksumStr := hex.EncodeToString(actualChecksum[:])
		if actualChecksumStr != expectedChecksum {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksumStr)
		}
		if !silent {
			fmt.Println("Checksum verified successfully")
		}
	}

	// Extract istioctl binary from tar.gz archive
	binaryData, err := extractFromTarGz(data, "istioctl")
	if err != nil {
		return fmt.Errorf("failed to extract istioctl from archive: %w", err)
	}

	// Create version directory
	versionDir, err := config.GetVersionDir(version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		return fmt.Errorf("failed to create version directory: %w", err)
	}

	// Write binary
	binaryPath, err := config.GetBinaryPath(version)
	if err != nil {
		return err
	}
	if err := os.WriteFile(binaryPath, binaryData, 0o755); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	if !silent {
		fmt.Printf("Installed istioctl %s\n", version)
	}
	return nil
}

// extractFromTarGz reads a tar.gz archive and extracts the named file.
func extractFromTarGz(data []byte, targetName string) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Match by base name (the archive may have a directory prefix)
		name := header.Name
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}

		if name == targetName && header.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s from archive: %w", targetName, err)
			}
			return content, nil
		}
	}

	return nil, fmt.Errorf("%s not found in archive", targetName)
}
