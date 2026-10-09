package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocal(t *testing.T) {
	t.Run("fails when not initialized", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		err := Local("0.31.0")
		if err == nil {
			t.Fatal("expected error when not initialized")
		}
	})

	t.Run("fails when expression is invalid", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		err := Local("not a valid thing @@")
		if err == nil {
			t.Fatal("expected error for invalid expression")
		}
		if !strings.Contains(err.Error(), "invalid version expression") {
			t.Fatalf("expected 'invalid version expression' error, got: %v", err)
		}
	})

	t.Run("writes plain version even if not installed", func(t *testing.T) {
		// Set-time no longer requires the version to be installed; the shim
		// validates installed-ness at exec time.
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		workDir := t.TempDir()
		origDir, _ := os.Getwd()
		if err := os.Chdir(workDir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(origDir) }()

		if err := Local("0.31.0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(workDir, ".istioctl-version"))
		if err != nil {
			t.Fatalf("failed to read .istioctl-version: %v", err)
		}
		if strings.TrimSpace(string(data)) != "0.31.0" {
			t.Fatalf("expected '0.31.0', got %q", strings.TrimSpace(string(data)))
		}
	})

	t.Run("writes constraint expression", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		workDir := t.TempDir()
		origDir, _ := os.Getwd()
		if err := os.Chdir(workDir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(origDir) }()

		if err := Local("~1.24.0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(workDir, ".istioctl-version"))
		if err != nil {
			t.Fatalf("failed to read .istioctl-version: %v", err)
		}
		if strings.TrimSpace(string(data)) != "~1.24.0" {
			t.Fatalf("expected '~1.24.0', got %q", strings.TrimSpace(string(data)))
		}
	})

	t.Run("reads local version", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		// Create .istioctl-version in a temp directory
		workDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(workDir, ".istioctl-version"), []byte("0.31.0\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		origDir, _ := os.Getwd()
		if err := os.Chdir(workDir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(origDir) }()

		output := captureStdout(t, func() {
			err := Local("")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if strings.TrimSpace(output) != "0.31.0" {
			t.Fatalf("expected '0.31.0', got %q", strings.TrimSpace(output))
		}
	})

	t.Run("fails when no local version configured", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		// Change to a directory without .istioctl-version
		workDir := t.TempDir()
		origDir, _ := os.Getwd()
		if err := os.Chdir(workDir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(origDir) }()

		err := Local("")
		if err == nil {
			t.Fatal("expected error when no local version configured")
		}
		if !strings.Contains(err.Error(), "no local version configured") {
			t.Fatalf("expected 'no local version configured' error, got: %v", err)
		}
	})
}
