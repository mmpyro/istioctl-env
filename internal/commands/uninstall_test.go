package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstall(t *testing.T) {
	t.Run("fails when not initialized", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		err := Uninstall("0.31.0")
		if err == nil {
			t.Fatal("expected error when not initialized")
		}
	})

	t.Run("fails when no version argument", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		err := Uninstall("")
		if err == nil {
			t.Fatal("expected error when no version argument")
		}
	})

	t.Run("fails when version not installed", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		err := Uninstall("0.31.0")
		if err == nil {
			t.Fatal("expected error when version not installed")
		}
		if !strings.Contains(err.Error(), "not installed") {
			t.Fatalf("expected 'not installed' error, got: %v", err)
		}
	})

	t.Run("successfully uninstalls version", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)

		// Create installed version
		versionDir := filepath.Join(tmpDir, "versions", "0.31.0")
		if err := os.MkdirAll(versionDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(versionDir, "istioctl"), []byte("binary"), 0o755); err != nil {
			t.Fatal(err)
		}

		output := captureStdout(t, func() {
			err := Uninstall("0.31.0")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "uninstalled") {
			t.Fatalf("expected 'uninstalled' message, got %q", output)
		}

		// Verify directory was removed
		if _, err := os.Stat(versionDir); !os.IsNotExist(err) {
			t.Fatal("version directory should have been removed")
		}
	})
}
