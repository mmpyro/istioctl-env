package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWhich(t *testing.T) {
	t.Run("fails when not initialized", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		err := Which()
		if err == nil {
			t.Fatal("expected error when not initialized")
		}
	})

	t.Run("prints path using shell version", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		t.Setenv("ISTIOENV_VERSION", "0.31.0")
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		output := captureStdout(t, func() {
			err := Which()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		expected := filepath.Join(tmpDir, "versions", "0.31.0", "istioctl")
		if strings.TrimSpace(output) != expected {
			t.Fatalf("expected %q, got %q", expected, strings.TrimSpace(output))
		}
	})

	t.Run("prints path using global version", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		t.Setenv("ISTIOENV_VERSION", "")
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "version"), []byte("0.32.0"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Change to a directory without .istioctl-version
		origDir, _ := os.Getwd()
		noVersionDir := t.TempDir()
		if err := os.Chdir(noVersionDir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(origDir) }()

		output := captureStdout(t, func() {
			err := Which()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		expected := filepath.Join(tmpDir, "versions", "0.32.0", "istioctl")
		if strings.TrimSpace(output) != expected {
			t.Fatalf("expected %q, got %q", expected, strings.TrimSpace(output))
		}
	})

	t.Run("fails when no version configured", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		t.Setenv("ISTIOENV_VERSION", "")
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		// Change to a directory without .istioctl-version
		origDir, _ := os.Getwd()
		noVersionDir := t.TempDir()
		if err := os.Chdir(noVersionDir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(origDir) }()

		err := Which()
		if err == nil {
			t.Fatal("expected error when no version configured")
		}
	})
}
