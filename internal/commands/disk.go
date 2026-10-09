package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
)

// dirSize returns the total size (in bytes) of all regular files reachable
// from path, following the directory tree.  Symbolic links are not traversed.
// A non-existent path returns (0, nil) so callers can tolerate directories
// that have been pruned concurrently; any other filesystem error is returned
// to the caller.
func dirSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return total, err
	}
	return total, nil
}

// humanize formats a byte count as a human-friendly string using binary units
// (KiB, MiB, GiB, TiB) with one decimal of precision.  Values below 1 KiB are
// printed in bytes with no decimal point.  Negative values render with a
// leading "-".
func humanize(bytes int64) string {
	if bytes < 0 {
		return "-" + humanize(-bytes)
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffixes := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	if exp >= len(suffixes) {
		exp = len(suffixes) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), suffixes[exp])
}
