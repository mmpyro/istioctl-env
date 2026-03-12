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
		err := List()
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
			err := List()
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
			err := List()
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
			err := List()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if strings.TrimSpace(output) != "" {
			t.Fatalf("expected empty output, got %q", output)
		}
	})
}
