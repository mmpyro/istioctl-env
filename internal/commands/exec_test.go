package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExec(t *testing.T) {
	t.Run("fails when not initialized", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		err := Exec("0.31.0", []string{"version"})
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

		err := Exec("0.31.0", []string{"version"})
		if err == nil {
			t.Fatal("expected error when version not installed")
		}
	})

	t.Run("fails when version or command missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions", "0.31.0"), 0o755); err != nil {
			t.Fatal(err)
		}

		err := Exec("", []string{"version"})
		if err == nil || err.Error() != "version not specified. Usage: istioctl-env exec <version> <command> [args...]" {
			t.Fatalf("expected version missing error, got %v", err)
		}

		err = Exec("0.31.0", []string{})
		if err == nil || err.Error() != "command not specified. Usage: istioctl-env exec <version> <command> [args...]" {
			t.Fatalf("expected command missing error, got %v", err)
		}
	})
}
