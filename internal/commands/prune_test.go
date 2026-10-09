package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mkVersion creates a $ISTIOENV_ROOT/versions/<name>/istioctl file of the
// requested size.  Returned path points at the version directory, not the
// binary, matching how prune and list reason about installations.
func mkVersion(t *testing.T, root, name string, size int) string {
	t.Helper()
	dir := filepath.Join(root, "versions", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "istioctl"), make([]byte, size), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// backdate sets both atime and mtime on a path, used to simulate aged
// installations when exercising --older-than.
func backdate(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

// writeLocalVersion creates a `.istioctl-version` file in dir containing v.
// Returns the containing directory for convenience.
func writeLocalVersion(t *testing.T, dir, v string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".istioctl-version"), []byte(v+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// isolatedScan returns PruneOptions with the given scan roots and clears env
// vars that could otherwise expand the reference set unpredictably.
func isolatedScan(t *testing.T, roots ...string) PruneOptions {
	t.Helper()
	t.Setenv("ISTIOENV_VERSION", "")
	t.Setenv(PruneScanRootsEnv, "")
	return PruneOptions{ScanRoots: roots}
}

func TestPrune(t *testing.T) {
	t.Run("fails when not initialized", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		if err := Prune(PruneOptions{}); err == nil {
			t.Fatal("expected error when not initialized")
		}
	})

	t.Run("global version is always referenced and never removed", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmp)

		mkVersion(t, tmp, "1.24.0", 1024)
		mkVersion(t, tmp, "1.23.0", 1024)
		mkVersion(t, tmp, "1.22.0", 1024)
		if err := os.WriteFile(filepath.Join(tmp, "version"), []byte("1.22.0\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		opts := isolatedScan(t) // no scan roots, no shell version → global is only ref
		opts.Yes = true

		output := captureStdout(t, func() {
			if err := Prune(opts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		// Global version must survive.
		if _, err := os.Stat(filepath.Join(tmp, "versions", "1.22.0")); err != nil {
			t.Fatalf("expected global version 1.22.0 to be kept, got %v", err)
		}
		// Others should be removed.
		for _, gone := range []string{"1.24.0", "1.23.0"} {
			if _, err := os.Stat(filepath.Join(tmp, "versions", gone)); !os.IsNotExist(err) {
				t.Fatalf("expected %s to be removed, got err %v", gone, err)
			}
		}
		if !strings.Contains(output, "pruned 2 version") {
			t.Errorf("expected summary mentioning 2 pruned, got %q", output)
		}
	})

	t.Run("ISTIOENV_VERSION is always referenced", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmp)
		t.Setenv(PruneScanRootsEnv, "")
		t.Setenv("ISTIOENV_VERSION", "1.23.0")

		mkVersion(t, tmp, "1.24.0", 1024)
		mkVersion(t, tmp, "1.23.0", 1024)
		mkVersion(t, tmp, "1.22.0", 1024)

		captureStdout(t, func() {
			if err := Prune(PruneOptions{Yes: true}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if _, err := os.Stat(filepath.Join(tmp, "versions", "1.23.0")); err != nil {
			t.Fatalf("expected ISTIOENV_VERSION 1.23.0 to survive, got %v", err)
		}
	})

	t.Run("version referenced by a .istioctl-version in a scan root survives", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmp)

		// Scan root with two nested projects, each pinning a different version.
		scanRoot := t.TempDir()
		writeLocalVersion(t, filepath.Join(scanRoot, "projA"), "1.23.0")
		writeLocalVersion(t, filepath.Join(scanRoot, "deep", "nested", "projB"), "1.22.0")

		mkVersion(t, tmp, "1.24.0", 1024)
		mkVersion(t, tmp, "1.23.0", 1024)
		mkVersion(t, tmp, "1.22.0", 1024)

		opts := isolatedScan(t, scanRoot)
		opts.Yes = true

		output := captureStdout(t, func() {
			if err := Prune(opts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		// 1.23.0 and 1.22.0 are referenced via .istioctl-version files; only
		// 1.24.0 should be removed.
		for _, kept := range []string{"1.23.0", "1.22.0"} {
			if _, err := os.Stat(filepath.Join(tmp, "versions", kept)); err != nil {
				t.Fatalf("expected %s to be kept, got %v", kept, err)
			}
		}
		if _, err := os.Stat(filepath.Join(tmp, "versions", "1.24.0")); !os.IsNotExist(err) {
			t.Fatalf("expected 1.24.0 to be removed, got %v", err)
		}
		if !strings.Contains(output, "found 2 .istioctl-version file") {
			t.Errorf("expected scan summary to mention 2 .istioctl-version files, got %q", output)
		}
	})

	t.Run("scan skips well-known noisy dirs (.git, node_modules)", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmp)

		scanRoot := t.TempDir()
		// Place a .istioctl-version file INSIDE a skipped directory to prove
		// the walker does not descend into it.
		writeLocalVersion(t, filepath.Join(scanRoot, "proj", ".git"), "1.22.0")
		writeLocalVersion(t, filepath.Join(scanRoot, "proj", "node_modules"), "1.23.0")

		mkVersion(t, tmp, "1.24.0", 1024)
		mkVersion(t, tmp, "1.23.0", 1024)
		mkVersion(t, tmp, "1.22.0", 1024)

		opts := isolatedScan(t, scanRoot)
		opts.Yes = true

		captureStdout(t, func() {
			if err := Prune(opts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		// Because the skipped dirs are not scanned, nothing was referenced
		// → every installed version should be gone.
		for _, gone := range []string{"1.24.0", "1.23.0", "1.22.0"} {
			if _, err := os.Stat(filepath.Join(tmp, "versions", gone)); !os.IsNotExist(err) {
				t.Fatalf("expected %s to be removed (skip-dir not scanned), got err %v", gone, err)
			}
		}
	})

	t.Run("scan does not descend into ISTIOENV_ROOT itself", func(t *testing.T) {
		// Simulate the (realistic) case where $HOME contains $ISTIOENV_ROOT.
		// If the scanner mistakenly walked into the install root, it would
		// see no .istioctl-version files there and the test wouldn't catch
		// anything — so we drop a stray one inside and prove it is ignored.
		home := t.TempDir()
		tmp := filepath.Join(home, ".istioenv")
		t.Setenv("ISTIOENV_ROOT", tmp)

		mkVersion(t, tmp, "1.24.0", 1024)
		mkVersion(t, tmp, "1.23.0", 1024)
		// Stray file inside the install root — should be ignored.
		if err := os.WriteFile(filepath.Join(tmp, "versions", "1.23.0", ".istioctl-version"), []byte("1.23.0\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		opts := isolatedScan(t, home)
		opts.Yes = true

		captureStdout(t, func() {
			if err := Prune(opts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		for _, gone := range []string{"1.24.0", "1.23.0"} {
			if _, err := os.Stat(filepath.Join(tmp, "versions", gone)); !os.IsNotExist(err) {
				t.Fatalf("expected %s to be removed, got err %v", gone, err)
			}
		}
	})

	t.Run("--keep-last keeps N newest regardless of references", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmp)

		for _, v := range []string{"1.20.0", "1.21.0", "1.22.0", "1.23.0", "1.24.0"} {
			mkVersion(t, tmp, v, 1024)
		}

		opts := isolatedScan(t)
		opts.KeepLast = 2
		opts.Yes = true

		captureStdout(t, func() {
			if err := Prune(opts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		for _, kept := range []string{"1.24.0", "1.23.0"} {
			if _, err := os.Stat(filepath.Join(tmp, "versions", kept)); err != nil {
				t.Fatalf("expected %s to be kept, got %v", kept, err)
			}
		}
		for _, gone := range []string{"1.22.0", "1.21.0", "1.20.0"} {
			if _, err := os.Stat(filepath.Join(tmp, "versions", gone)); !os.IsNotExist(err) {
				t.Fatalf("expected %s to be removed, got err %v", gone, err)
			}
		}
	})

	t.Run("--older-than filters by directory mtime", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmp)

		mkVersion(t, tmp, "1.24.0", 1024) // fresh
		mkVersion(t, tmp, "1.23.0", 1024) // old
		backdate(t, filepath.Join(tmp, "versions", "1.24.0"), time.Now())
		backdate(t, filepath.Join(tmp, "versions", "1.23.0"), time.Now().Add(-72*time.Hour))

		opts := isolatedScan(t)
		opts.OlderThan = 24 * time.Hour
		opts.OlderThanProvided = true
		opts.Yes = true

		captureStdout(t, func() {
			if err := Prune(opts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if _, err := os.Stat(filepath.Join(tmp, "versions", "1.24.0")); err != nil {
			t.Fatalf("fresh version should survive --older-than, got %v", err)
		}
		if _, err := os.Stat(filepath.Join(tmp, "versions", "1.23.0")); !os.IsNotExist(err) {
			t.Fatalf("old version should be removed, got %v", err)
		}
	})

	t.Run("dry-run is the default — nothing is deleted without --yes", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmp)

		mkVersion(t, tmp, "1.24.0", 2048)
		mkVersion(t, tmp, "1.23.0", 2048)

		opts := isolatedScan(t) // Yes defaults to false → dry-run

		output := captureStdout(t, func() {
			if err := Prune(opts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		for _, v := range []string{"1.24.0", "1.23.0"} {
			if _, err := os.Stat(filepath.Join(tmp, "versions", v)); err != nil {
				t.Fatalf("dry-run removed %s on disk! err: %v", v, err)
			}
		}
		if !strings.Contains(output, "would remove 1.24.0") {
			t.Errorf("expected dry-run output to mention 1.24.0, got %q", output)
		}
		if !strings.Contains(output, "would prune 2 version") {
			t.Errorf("expected 'would prune 2 version(s)' summary, got %q", output)
		}
		if !strings.Contains(output, "pass --yes to apply") {
			t.Errorf("expected prompt to pass --yes, got %q", output)
		}
	})

	t.Run("--dry-run wins when combined with --yes", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmp)

		mkVersion(t, tmp, "1.24.0", 1024)

		opts := isolatedScan(t)
		opts.Yes = true
		opts.DryRun = true

		captureStdout(t, func() {
			if err := Prune(opts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if _, err := os.Stat(filepath.Join(tmp, "versions", "1.24.0")); err != nil {
			t.Fatalf("explicit --dry-run should trump --yes, got %v", err)
		}
	})

	t.Run("--yes actually deletes", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmp)

		mkVersion(t, tmp, "1.24.0", 1024)

		opts := isolatedScan(t)
		opts.Yes = true

		output := captureStdout(t, func() {
			if err := Prune(opts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if _, err := os.Stat(filepath.Join(tmp, "versions", "1.24.0")); !os.IsNotExist(err) {
			t.Fatalf("expected 1.24.0 to be removed, got err %v", err)
		}
		if !strings.Contains(output, "pruned 1 version") {
			t.Errorf("expected summary 'pruned 1 version(s)', got %q", output)
		}
	})

	t.Run("nothing to prune — prints a friendly summary", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmp)

		mkVersion(t, tmp, "1.24.0", 1024)
		if err := os.WriteFile(filepath.Join(tmp, "version"), []byte("1.24.0\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		opts := isolatedScan(t)
		opts.Yes = true

		output := captureStdout(t, func() {
			if err := Prune(opts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "nothing to prune") {
			t.Fatalf("expected 'nothing to prune' message, got %q", output)
		}
	})

	t.Run("ISTIOENV_PRUNE_SCAN_ROOTS env var is honored when ScanRoots is empty", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmp)
		t.Setenv("ISTIOENV_VERSION", "")

		mkVersion(t, tmp, "1.24.0", 1024)
		mkVersion(t, tmp, "1.23.0", 1024)

		scanRoot := t.TempDir()
		writeLocalVersion(t, filepath.Join(scanRoot, "proj"), "1.23.0")

		t.Setenv(PruneScanRootsEnv, scanRoot)

		captureStdout(t, func() {
			if err := Prune(PruneOptions{Yes: true}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if _, err := os.Stat(filepath.Join(tmp, "versions", "1.23.0")); err != nil {
			t.Fatalf("expected env-scanned 1.23.0 to be kept, got %v", err)
		}
		if _, err := os.Stat(filepath.Join(tmp, "versions", "1.24.0")); !os.IsNotExist(err) {
			t.Fatalf("expected 1.24.0 to be removed, got %v", err)
		}
	})

	t.Run("scan summary is always printed and mentions duration", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmp)

		mkVersion(t, tmp, "1.24.0", 1024)

		scanRoot := t.TempDir()
		opts := isolatedScan(t, scanRoot)

		output := captureStdout(t, func() {
			if err := Prune(opts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "scanned") || !strings.Contains(output, "in ") {
			t.Fatalf("expected scan summary with duration, got %q", output)
		}
	})
}

func TestHumanize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1024*1024*1024 + 512*1024*1024, "1.5 GiB"},
		{-2048, "-2.0 KiB"},
	}
	for _, tc := range cases {
		if got := humanize(tc.in); got != tc.want {
			t.Errorf("humanize(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDirSize(t *testing.T) {
	t.Run("sums regular file sizes", func(t *testing.T) {
		tmp := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmp, "a"), make([]byte, 100), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmp, "sub", "b"), make([]byte, 250), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := dirSize(tmp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 350 {
			t.Errorf("expected 350, got %d", got)
		}
	})

	t.Run("missing path yields zero without error", func(t *testing.T) {
		got, err := dirSize(filepath.Join(t.TempDir(), "nope"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})
}
