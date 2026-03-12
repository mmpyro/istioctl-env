package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShell(t *testing.T) {
	t.Run("fails when not initialized", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		err := Shell("0.31.0")
		if err == nil {
			t.Fatal("expected error when not initialized")
		}
	})

	t.Run("prints current shell version", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ISTIOENV_VERSION", "0.31.0")

		output := captureStdout(t, func() {
			err := Shell("")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if strings.TrimSpace(output) != "0.31.0" {
			t.Fatalf("expected '0.31.0', got %q", strings.TrimSpace(output))
		}
	})

	t.Run("fails when no shell version set", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ISTIOENV_VERSION", "")

		err := Shell("")
		if err == nil {
			t.Fatal("expected error when no shell version set")
		}
		if !strings.Contains(err.Error(), "no shell version configured") {
			t.Fatalf("expected 'no shell version configured', got: %v", err)
		}
	})

	t.Run("fails when version not installed", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		err := Shell("0.32.0")
		if err == nil {
			t.Fatal("expected error when version not installed")
		}
		if !strings.Contains(err.Error(), "not installed") {
			t.Fatalf("expected 'not installed' error, got: %v", err)
		}
	})

	t.Run("outputs export when version is installed", func(t *testing.T) {
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
			err := Shell("0.31.0")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "export ISTIOENV_VERSION=0.31.0") {
			t.Fatalf("expected export command, got %q", output)
		}
	})
}
