// Package config provides configuration and version resolution logic for istioctl-env.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/istioctl-env/internal/semver"
)

// GetIstioEnvRoot reads the ISTIOENV_ROOT environment variable.
// Returns the path and true if set, or empty string and false if not.
func GetIstioEnvRoot() (string, bool) {
	root := os.Getenv("ISTIOENV_ROOT")
	if root == "" {
		return "", false
	}
	return root, true
}

// IsInitialized checks if ISTIOENV_ROOT is set and the versions directory exists.
func IsInitialized() bool {
	root, ok := GetIstioEnvRoot()
	if !ok {
		return false
	}
	versionsDir := filepath.Join(root, "versions")
	info, err := os.Stat(versionsDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// RequireInit checks that istioctl-env is initialized. If not, it prints an error
// message and returns a non-nil error.
func RequireInit() error {
	root, ok := GetIstioEnvRoot()
	if !ok {
		return fmt.Errorf("istioctl-env is not initialized. ISTIOENV_ROOT is not set.\nRun 'istioctl-env init' to initialize")
	}
	versionsDir := filepath.Join(root, "versions")
	if _, err := os.Stat(versionsDir); os.IsNotExist(err) {
		return fmt.Errorf("istioctl-env is not initialized. Run 'istioctl-env init' to initialize")
	}
	return nil
}

// ResolveVersion determines which istioctl version to use based on priority:
//  1. ISTIOENV_VERSION environment variable (shell version)
//  2. .istioctl-version file in current or parent directories (local version)
//  3. $ISTIOENV_ROOT/version file (global version)
//
// Returns the version string and nil error, or empty string and error if no version is configured.
func ResolveVersion() (string, error) {
	// 1. Check ISTIOENV_VERSION env var (shell version)
	if v := os.Getenv("ISTIOENV_VERSION"); v != "" {
		return strings.TrimSpace(v), nil
	}

	// 2. Walk up directories looking for .istioctl-version (local version)
	if v, err := findLocalVersion(); err == nil && v != "" {
		return v, nil
	}

	// 3. Check global version file
	if v, err := readGlobalVersion(); err == nil && v != "" {
		return v, nil
	}

	return "", fmt.Errorf("no istioctl version configured. Set a version using 'istioctl-env shell', 'istioctl-env local', or 'istioctl-env global'")
}

// findLocalVersion walks up from the current working directory looking for
// a .istioctl-version file.
func findLocalVersion() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return FindLocalVersionFrom(dir)
}

// FindLocalVersionFrom walks up from the given directory looking for
// a .istioctl-version file. Exported for testing.
func FindLocalVersionFrom(dir string) (string, error) {
	for {
		versionFile := filepath.Join(dir, ".istioctl-version")
		data, err := os.ReadFile(versionFile)
		if err == nil {
			v := strings.TrimSpace(string(data))
			if v != "" {
				return v, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no .istioctl-version file found")
}

// readGlobalVersion reads the global version from $ISTIOENV_ROOT/version.
func readGlobalVersion() (string, error) {
	root, ok := GetIstioEnvRoot()
	if !ok {
		return "", fmt.Errorf("ISTIOENV_ROOT not set")
	}
	return ReadGlobalVersion(root)
}

// ReadGlobalVersion reads the global version from the given root directory.
// Exported for testing.
func ReadGlobalVersion(root string) (string, error) {
	versionFile := filepath.Join(root, "version")
	data, err := os.ReadFile(versionFile)
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(data))
	if v == "" {
		return "", fmt.Errorf("global version file is empty")
	}
	return v, nil
}

// GetVersionDir returns the path to the directory for a specific version.
func GetVersionDir(version string) (string, error) {
	root, ok := GetIstioEnvRoot()
	if !ok {
		return "", fmt.Errorf("ISTIOENV_ROOT not set")
	}
	return filepath.Join(root, "versions", version), nil
}

// GetBinaryPath returns the path to the istioctl binary for a specific version.
func GetBinaryPath(version string) (string, error) {
	versionDir, err := GetVersionDir(version)
	if err != nil {
		return "", err
	}
	return filepath.Join(versionDir, "istioctl"), nil
}

// IsVersionInstalled checks if a specific version is installed.
func IsVersionInstalled(version string) (bool, error) {
	binaryPath, err := GetBinaryPath(version)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(binaryPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListInstalledVersions returns all versions that are installed under
// $ISTIOENV_ROOT/versions (i.e. directories whose "istioctl" binary exists).
func ListInstalledVersions() ([]string, error) {
	root, ok := GetIstioEnvRoot()
	if !ok {
		return nil, fmt.Errorf("ISTIOENV_ROOT not set")
	}
	versionsDir := filepath.Join(root, "versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		binaryPath := filepath.Join(versionsDir, e.Name(), "istioctl")
		if _, err := os.Stat(binaryPath); err == nil {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// ResolveInstalledVersion resolves the configured version expression (the
// output of ResolveVersion) to a concrete installed version string.
//
//   - If the expression is a plain exact version it is returned unchanged
//     (and no installed-ness check is performed here — the caller may wish
//     to check IsVersionInstalled for a friendlier error message).
//   - If the expression is a constraint, the highest INSTALLED version
//     matching the constraint is returned. If none matches, an error is
//     returned describing the situation.
//
// The second return value is the raw (configured) expression, so callers
// such as `which` and `status` can display both.
func ResolveInstalledVersion() (resolved string, raw string, err error) {
	raw, err = ResolveVersion()
	if err != nil {
		return "", "", err
	}

	if !semver.IsConstraint(raw) {
		return raw, raw, nil
	}

	constraint, err := semver.ParseConstraint(raw)
	if err != nil {
		return "", raw, fmt.Errorf("invalid version expression %q: %w", raw, err)
	}

	installed, err := ListInstalledVersions()
	if err != nil {
		return "", raw, err
	}
	if len(installed) == 0 {
		return "", raw, fmt.Errorf("no installed versions match %q (none installed). Try 'istioctl-env install %s'", raw, raw)
	}

	match, ok := semver.HighestMatching(constraint, installed)
	if !ok {
		return "", raw, fmt.Errorf("no installed version satisfies %q. Try 'istioctl-env install %s'", raw, raw)
	}
	return match, raw, nil
}
