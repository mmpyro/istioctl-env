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

	t.Run("fails when expression is invalid", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		err := Global("not a valid thing @@")
		if err == nil {
			t.Fatal("expected error for invalid expression")
		}
		if !strings.Contains(err.Error(), "invalid version expression") {
			t.Fatalf("expected 'invalid version expression' error, got: %v", err)
		}
	})

	t.Run("writes plain version even if not installed", func(t *testing.T) {
		// Set-time no longer requires the version to be installed. The shim
		// enforces that at exec time, which is Feature 1's new contract.
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := Global("0.31.0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(tmpDir, "version"))
		if err != nil {
			t.Fatalf("failed to read version file: %v", err)
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

		if err := Global("^1.24.0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(tmpDir, "version"))
		if err != nil {
			t.Fatalf("failed to read version file: %v", err)
		}
		if strings.TrimSpace(string(data)) != "^1.24.0" {
			t.Fatalf("expected '^1.24.0', got %q", strings.TrimSpace(string(data)))
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
