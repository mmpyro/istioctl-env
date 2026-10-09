package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestList(t *testing.T) {
	t.Run("fails when not initialized", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		err := List(false)
		if err == nil {
			t.Fatal("expected error when not initialized")
		}
	})

	t.Run("lists installed versions sorted newest to oldest", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)

		// Create versions directory with some versions (in arbitrary order)
		for _, v := range []string{"0.32.0", "0.30.0", "0.31.0"} {
			if err := os.MkdirAll(filepath.Join(tmpDir, "versions", v), 0o755); err != nil {
				t.Fatal(err)
			}
		}

		output := captureStdout(t, func() {
			err := List(false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		lines := strings.Split(strings.TrimSpace(output), "\n")
		// Expect newest first (descending semver order)
		expected := []string{"0.32.0", "0.31.0", "0.30.0"}
		if len(lines) != len(expected) {
			t.Fatalf("expected %d versions, got %d: %v", len(expected), len(lines), lines)
		}
		for i, line := range lines {
			if line != expected[i] {
				t.Fatalf("expected %s at index %d, got %s", expected[i], i, line)
			}
		}
	})

	t.Run("lists installed versions with prereleases sorted newest to oldest", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)

		// Create versions including a pre-release
		for _, v := range []string{"0.31.1-alpha", "0.31.1", "0.31.0", "0.1.0"} {
			if err := os.MkdirAll(filepath.Join(tmpDir, "versions", v), 0o755); err != nil {
				t.Fatal(err)
			}
		}

		output := captureStdout(t, func() {
			err := List(false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		lines := strings.Split(strings.TrimSpace(output), "\n")
		// Expect: 0.31.1, 0.31.1-alpha, 0.31.0, 0.1.0
		expected := []string{"0.31.1", "0.31.1-alpha", "0.31.0", "0.1.0"}
		if len(lines) != len(expected) {
			t.Fatalf("expected %d versions, got %d: %v", len(expected), len(lines), lines)
		}
		for i, line := range lines {
			if line != expected[i] {
				t.Fatalf("expected %s at index %d, got %s", expected[i], i, line)
			}
		}
	})

	t.Run("empty when no versions installed", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		output := captureStdout(t, func() {
			err := List(false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if strings.TrimSpace(output) != "" {
			t.Fatalf("expected empty output, got %q", output)
		}
	})

	// --- --disk-usage ---

	t.Run("default output is one version per line (awk-compatible)", func(t *testing.T) {
		// Pins the format relied on by the autocompletion script:
		// `istioctl-env list | awk '{print $1}'`.  Each line is exactly the
		// version string: no size column, no header, no trailing whitespace.
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		for _, v := range []string{"1.24.0", "1.23.0"} {
			if err := os.MkdirAll(filepath.Join(tmpDir, "versions", v), 0o755); err != nil {
				t.Fatal(err)
			}
		}

		output := captureStdout(t, func() {
			if err := List(false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		want := "1.24.0\n1.23.0\n"
		if output != want {
			t.Fatalf("default list format changed!\nwant %q\ngot  %q", want, output)
		}
	})

	t.Run("--disk-usage shows size column and TOTAL row", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)

		mkVer := func(name string, size int) {
			t.Helper()
			dir := filepath.Join(tmpDir, "versions", name)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "istioctl"), make([]byte, size), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		mkVer("1.24.0", 2048) // 2 KiB
		mkVer("1.23.0", 1024) // 1 KiB

		output := captureStdout(t, func() {
			if err := List(true); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "1.24.0") || !strings.Contains(output, "2.0 KiB") {
			t.Fatalf("expected 1.24.0 with 2.0 KiB, got %q", output)
		}
		if !strings.Contains(output, "1.23.0") || !strings.Contains(output, "1.0 KiB") {
			t.Fatalf("expected 1.23.0 with 1.0 KiB, got %q", output)
		}
		if !strings.Contains(output, "TOTAL") || !strings.Contains(output, "3.0 KiB") {
			t.Fatalf("expected TOTAL row with 3.0 KiB, got %q", output)
		}

		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines (2 versions + TOTAL), got %d: %v", len(lines), lines)
		}
		if !strings.HasPrefix(lines[0], "1.24.0") {
			t.Fatalf("expected 1.24.0 first, got %q", lines[0])
		}
		if !strings.HasPrefix(lines[1], "1.23.0") {
			t.Fatalf("expected 1.23.0 second, got %q", lines[1])
		}
		if !strings.HasPrefix(lines[2], "TOTAL") {
			t.Fatalf("expected TOTAL last, got %q", lines[2])
		}
	})

	t.Run("--disk-usage with zero versions still prints a TOTAL row", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		output := captureStdout(t, func() {
			if err := List(true); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "TOTAL") || !strings.Contains(output, "0 B") {
			t.Fatalf("expected TOTAL 0 B row, got %q", output)
		}
	})
}
