package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/user/istioctl-env/internal/github"
	"github.com/user/istioctl-env/internal/platform"
	"github.com/user/istioctl-env/internal/semver"
)

const istioenvRepo = "mmpyro/istioctl-env"

// Upgrade downloads the latest stable istioctl-env release from GitHub and replaces
// the current binary in-place using an atomic rename.
func Upgrade() error {
	// 0. Short-circuit in offline mode: upgrading istioctl-env itself always
	//    needs the GitHub API and the GitHub release assets, so there is no
	//    sensible fallback. Exit cleanly with a status message.
	if github.IsOffline() {
		fmt.Println("istioctl-env: offline mode (ISTIOENV_OFFLINE=1); skipping upgrade")
		return nil
	}

	// 1. Resolve the running binary path.
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}
	binaryPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks for %s: %w", execPath, err)
	}

	// 2. Fetch latest istioctl-env release from GitHub.
	client := github.NewClient()
	latestVersion, err := client.GetLatestReleaseFor(istioenvRepo)
	if err != nil {
		return fmt.Errorf("failed to fetch latest istioctl-env release: %w", err)
	}

	// 3. Compare versions.
	if Version != "dev" {
		current := semver.Parse(Version)
		remote := semver.Parse(latestVersion)

		if !semver.Less(current, remote) {
			if current.Original == remote.Original ||
				(current.Major == remote.Major && current.Minor == remote.Minor && current.Patch == remote.Patch && current.PreRelease == remote.PreRelease) {
				fmt.Printf("istioctl-env is already up to date (version %s)\n", Version)
				return nil
			}
			fmt.Printf("Current version %s is newer than latest release %s, skipping upgrade\n", Version, latestVersion)
			return nil
		}
	} else {
		fmt.Println("Running a dev build — proceeding with upgrade to latest release")
	}

	// 4. Detect current OS/architecture.
	info, err := platform.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect platform: %w", err)
	}

	// 5. Build the asset download URL. We route through the client's
	//    DownloadBaseURL so an ISTIOENV_DOWNLOAD_MIRROR (or legacy
	//    ISTIOENV_MIRROR_URL) also redirects self-upgrade downloads to
	//    the mirror — in a fully air-gapped environment that is the
	//    only way `istioctl-env upgrade` can succeed.
	url := platform.SelfDownloadURL(client.DownloadBaseURL, latestVersion, info, istioenvRepo)
	fmt.Printf("Downloading istioctl-env %s for %s/%s...\n", latestVersion, info.OS, info.Arch)

	// 6. Download the new binary.
	data, err := client.DownloadBinary(url)
	if err != nil {
		return fmt.Errorf("failed to download istioctl-env %s: %w", latestVersion, err)
	}

	// 7. Atomic replace: write temp file in same directory, then rename.
	if err := atomicReplace(binaryPath, data); err != nil {
		return err
	}

	if Version != "dev" {
		fmt.Printf("Upgraded istioctl-env from %s to %s\n", Version, latestVersion)
	} else {
		fmt.Printf("Upgraded istioctl-env to %s\n", latestVersion)
	}
	return nil
}

// atomicReplace writes data to a temporary file in the same directory as
// targetPath, then atomically renames it over the target.
func atomicReplace(targetPath string, data []byte) error {
	dir := filepath.Dir(targetPath)

	tmpFile, err := os.CreateTemp(dir, ".istioctl-env-upgrade-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w (do you have write permission to %s?)", err, dir)
	}
	tmpPath := tmpFile.Name()

	// Clean up the temp file on any error path.
	defer func() {
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	if _, err = tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temporary file: %w", err)
	}
	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	if err = os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("failed to set permissions on temporary file: %w", err)
	}

	// Attempt atomic rename first.
	if err = os.Rename(tmpPath, targetPath); err != nil {
		// Fall back to copy + remove (e.g. cross-device).
		if fallbackErr := copyFile(tmpPath, targetPath); fallbackErr != nil {
			return fmt.Errorf("failed to replace binary (rename: %w, copy fallback: %v)", err, fallbackErr)
		}
		os.Remove(tmpPath)
	}

	return nil
}

// copyFile copies src to dst, preserving executable permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
