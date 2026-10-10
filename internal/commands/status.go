package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/user/istioctl-env/internal/config"
	"github.com/user/istioctl-env/internal/semver"
)

// StatusHelp prints help for the status command.
func StatusHelp() {
	fmt.Println(`Usage: istioctl-env status

Show ISTIOENV_ROOT, the active version and where it was set, and the
installed versions.

Flags:
  -h, --help    Show this help message.`)
}

// Status provides an overview of the current istioctl-env environment.
//
// When the active expression is a SemVer constraint, both the raw expression
// and the resolved concrete version are shown, e.g.
//
//	Active version: ^1.24.0 → 1.24.3 (set by .istioctl-version file)
func Status() error {
	root, ok := config.GetIstioEnvRoot()
	if !ok {
		fmt.Println("istioctl-env is not initialized (ISTIOENV_ROOT is not set).")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)

	fmt.Fprintf(w, "ISTIOENV_ROOT:\t%s\n", root)

	rawVersion, source := getActiveVersionWithSource()
	var resolvedVersion string
	if rawVersion == "" {
		fmt.Fprintf(w, "Active version:\tnone\n")
	} else {
		resolvedVersion = resolveForStatus(rawVersion)
		display := rawVersion
		if resolvedVersion != "" && resolvedVersion != rawVersion {
			display = fmt.Sprintf("%s → %s", rawVersion, resolvedVersion)
		} else if resolvedVersion == "" && semver.IsConstraint(rawVersion) {
			display = fmt.Sprintf("%s → (no installed version matches)", rawVersion)
		}
		fmt.Fprintf(w, "Active version:\t%s (set by %s)\n", display, source)

		pathTarget := resolvedVersion
		if pathTarget == "" {
			pathTarget = rawVersion
		}
		binaryPath, _ := config.GetBinaryPath(pathTarget)
		fmt.Fprintf(w, "Binary path:\t%s\n", binaryPath)
	}
	w.Flush()

	versionsDir := filepath.Join(root, "versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		fmt.Println("\nInstalled versions: none")
		return nil
	}

	var installed []string
	for _, entry := range entries {
		if entry.IsDir() {
			installed = append(installed, entry.Name())
		}
	}
	installed = semver.SortDescending(installed)
	fmt.Printf("\nInstalled versions (%d):\n", len(installed))

	activeConcrete := resolvedVersion
	if activeConcrete == "" {
		activeConcrete = rawVersion
	}

	// Compute total disk usage alongside the listing.  A size failure on a
	// single version is best-effort: we skip the broken directory and keep
	// going so `status` is never prevented from reporting the environment.
	var totalBytes int64
	for _, v := range installed {
		marker := "  "
		if v == activeConcrete {
			marker = "* "
		}
		if size, sizeErr := dirSize(filepath.Join(versionsDir, v)); sizeErr == nil {
			totalBytes += size
			fmt.Printf("\t%s%s (%s)\n", marker, v, humanize(size))
		} else {
			fmt.Printf("\t%s%s\n", marker, v)
		}
	}

	fmt.Printf("\nTotal disk usage: %s\n", humanize(totalBytes))
	return nil
}

// resolveForStatus returns the concrete installed version matching raw, or ""
// when raw is a constraint that doesn't match anything installed. Errors are
// swallowed on purpose: Status is best-effort reporting.
func resolveForStatus(raw string) string {
	if !semver.IsConstraint(raw) {
		return raw
	}
	constraint, err := semver.ParseConstraint(raw)
	if err != nil {
		return ""
	}
	installed, err := config.ListInstalledVersions()
	if err != nil || len(installed) == 0 {
		return ""
	}
	match, _ := semver.HighestMatching(constraint, installed)
	return match
}

func getActiveVersionWithSource() (string, string) {
	// 1. Check ISTIOENV_VERSION env var (shell version)
	if v := os.Getenv("ISTIOENV_VERSION"); v != "" {
		return v, "ISTIOENV_VERSION environment variable"
	}

	// 2. Walk up directories looking for .istioctl-version (local version)
	dir, err := os.Getwd()
	if err == nil {
		if v, err := config.FindLocalVersionFrom(dir); err == nil && v != "" {
			return v, ".istioctl-version file"
		}
	}

	// 3. Check global version file
	root, ok := config.GetIstioEnvRoot()
	if ok {
		if v, err := config.ReadGlobalVersion(root); err == nil && v != "" {
			return v, "global version file"
		}
	}

	return "", ""
}
