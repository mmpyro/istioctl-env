package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/user/istioctl-env/internal/config"
	"github.com/user/istioctl-env/internal/semver"
)

// List prints all installed istioctl versions, newest to oldest.
//
// When showDiskUsage is false the output is a single column of version
// strings — one per line — exactly as it has always been.  The completion
// scripts rely on this exact format (`istioctl-env list | awk '{print $1}'`),
// so do not change it.
//
// When showDiskUsage is true the output becomes two tab-aligned columns:
// `<version>\t<human-readable size>`, followed by a `TOTAL\t<size>` summary
// row that reports the on-disk footprint of all installed versions.
func List(showDiskUsage bool) error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	root, _ := config.GetIstioEnvRoot()
	versionsDir := filepath.Join(root, "versions")

	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return fmt.Errorf("failed to read versions directory: %w", err)
	}

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}

	versions = semver.SortDescending(versions)

	if !showDiskUsage {
		for _, v := range versions {
			fmt.Println(v)
		}
		return nil
	}

	// --disk-usage: tab-aligned <version>\t<size> rows, then a TOTAL row.
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	var total int64
	for _, v := range versions {
		size, err := dirSize(filepath.Join(versionsDir, v))
		if err != nil {
			return fmt.Errorf("failed to measure %s: %w", v, err)
		}
		total += size
		fmt.Fprintf(w, "%s\t%s\n", v, humanize(size))
	}
	// Even with zero installed versions we still want the TOTAL row so that
	// scripts can parse the output deterministically.
	fmt.Fprintf(w, "TOTAL\t%s\n", humanize(total))
	return w.Flush()
}

// ListHelp prints help text for the list command.
func ListHelp() {
	fmt.Println(`Usage: istioctl-env list [flags]

List installed istioctl versions (newest to oldest).

Flags:
  --disk-usage   Also show the on-disk size of each version and a TOTAL row
  -h, --help     Show this help and exit`)
}
