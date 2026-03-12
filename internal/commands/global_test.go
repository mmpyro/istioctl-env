package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobal(t *testing.T) {
	t.Run("fails when not initialized", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		err := Global("0.31.0")
		if err == nil {
			t.Fatal("expected error when not initialized")
		}
	})

	t.Run("fails when version not installed", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		err := Global("0.32.0")
		if err == nil {
			t.Fatal("expected error when version not installed")
		}
		if !strings.Contains(err.Error(), "not installed") {
			t.Fatalf("expected 'not installed' error, got: %v", err)
		}
	})

	t.Run("writes global version file", func(t *testing.T) {
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

		err := Global("0.31.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify file was written
		data, err := os.ReadFile(filepath.Join(tmpDir, "version"))
		if err != nil {
			t.Fatalf("failed to read version file: %v", err)
		}
		if strings.TrimSpace(string(data)) != "0.31.0" {
			t.Fatalf("expected '0.31.0', got %q", strings.TrimSpace(string(data)))
		}
	})

	t.Run("reads global version", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "version"), []byte("0.31.0\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		output := captureStdout(t, func() {
			err := Global("")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if strings.TrimSpace(output) != "0.31.0" {
			t.Fatalf("expected '0.31.0', got %q", strings.TrimSpace(output))
		}
	})

	t.Run("fails when no global version configured", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		err := Global("")
		if err == nil {
			t.Fatal("expected error when no global version configured")
		}
		if !strings.Contains(err.Error(), "no global version configured") {
			t.Fatalf("expected 'no global version configured' error, got: %v", err)
		}
	})
}
