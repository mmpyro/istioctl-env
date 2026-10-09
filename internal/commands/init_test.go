package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStderr runs fn with os.Stderr replaced by a pipe and returns what
// was written. It is intentionally local to this file so other command tests
// are not forced to adopt it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestInit(t *testing.T) {
	t.Run("fails when ISTIOENV_ROOT not set", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		err := Init("")
		if err == nil {
			t.Fatal("expected error when ISTIOENV_ROOT not set")
		}
	})

	t.Run("creates versions directory and shim with bash default", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		t.Setenv("SHELL", "/bin/bash")

		output := captureStdout(t, func() {
			err := Init("")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		// Check versions directory was created
		versionsDir := filepath.Join(tmpDir, "versions")
		info, err := os.Stat(versionsDir)
		if err != nil {
			t.Fatalf("versions directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("versions should be a directory")
		}

		// Check shim was created
		shimPath := filepath.Join(tmpDir, "shims", "istioctl")
		if _, err := os.Stat(shimPath); err != nil {
			t.Fatalf("shim not created: %v", err)
		}

		// Check shell init output
		if !strings.Contains(output, "istioctl-env()") {
			t.Fatal("output should contain shell function")
		}
		if !strings.Contains(output, "shims:$PATH") {
			t.Fatal("output should contain PATH update")
		}
	})

	t.Run("explicit --shell zsh emits zsh integration", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		t.Setenv("SHELL", "/bin/bash") // explicit flag must win over detection

		output := captureStdout(t, func() {
			if err := Init("zsh"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "function istioctl-env()") {
			t.Fatalf("expected zsh function block, got:\n%s", output)
		}
	})

	t.Run("explicit --shell fish emits fish integration", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		t.Setenv("SHELL", "/bin/bash")

		output := captureStdout(t, func() {
			if err := Init("fish"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "set -gx PATH") {
			t.Fatalf("expected fish PATH update, got:\n%s", output)
		}
		if !strings.Contains(output, "function istioctl-env") {
			t.Fatalf("expected fish function definition, got:\n%s", output)
		}
	})

	t.Run("unknown shell value is rejected", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		err := Init("ksh")
		if err == nil {
			t.Fatal("expected error for unknown shell")
		}
		if !strings.Contains(err.Error(), "ksh") {
			t.Fatalf("error should mention the bad value, got: %v", err)
		}
	})

	t.Run("auto-detects zsh from $SHELL", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		t.Setenv("SHELL", "/usr/bin/zsh")

		var warn string
		output := captureStdout(t, func() {
			warn = captureStderr(t, func() {
				if err := Init(""); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		})

		if !strings.Contains(output, "function istioctl-env()") {
			t.Fatalf("expected zsh integration from detection, got:\n%s", output)
		}
		if warn != "" {
			t.Fatalf("did not expect a WARN when detection succeeds, got: %q", warn)
		}
	})

	t.Run("falls back to bash with WARN when detection fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		t.Setenv("SHELL", "/bin/dash")

		var warn string
		output := captureStdout(t, func() {
			warn = captureStderr(t, func() {
				if err := Init(""); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		})

		if !strings.Contains(output, "istioctl-env()") {
			t.Fatal("expected bash fallback integration")
		}
		if !strings.Contains(warn, "WARN") {
			t.Fatalf("expected WARN to stderr on detection fallback, got: %q", warn)
		}
		if !strings.Contains(warn, "--shell") {
			t.Fatalf("WARN should mention --shell, got: %q", warn)
		}
	})
}
