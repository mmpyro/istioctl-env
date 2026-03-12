package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/user/istioctl-env/internal/config"
	"github.com/user/istioctl-env/internal/semver"
)

// Status provides an overview of the current istioctl-env environment.
func Status() error {
	root, ok := config.GetIstioEnvRoot()
	if !ok {
		fmt.Println("istioctl-env is not initialized (ISTIOENV_ROOT is not set).")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)

	fmt.Fprintf(w, "ISTIOENV_ROOT:\t%s\n", root)

	version, source := getActiveVersionWithSource()
	if version == "" {
		fmt.Fprintf(w, "Active version:\tnone\n")
	} else {
		fmt.Fprintf(w, "Active version:\t%s (set by %s)\n", version, source)
		binaryPath, _ := config.GetBinaryPath(version)
		fmt.Fprintf(w, "Binary path:\t%s\n", binaryPath)
	}
	w.Flush()

	versionsDir := filepath.Join(root, "versions")
	entries, err := os.ReadDir(versionsDir)
	if err == nil {
		var installed []string
		for _, entry := range entries {
			if entry.IsDir() {
				installed = append(installed, entry.Name())
			}
		}
		installed = semver.SortDescending(installed)
		fmt.Printf("\nInstalled versions (%d):\n", len(installed))
		for _, v := range installed {
			if v == version {
				fmt.Printf("\t* %s\n", v)
			} else {
				fmt.Printf("\t  %s\n", v)
			}
		}
	} else {
		fmt.Println("\nInstalled versions: none")
	}

	return nil
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
