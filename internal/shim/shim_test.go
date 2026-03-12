package shim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateShimScript(t *testing.T) {
	t.Run("creates shim script", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := GenerateShimScript(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		shimPath := filepath.Join(tmpDir, "shims", "istioctl")
		info, err := os.Stat(shimPath)
		if err != nil {
			t.Fatalf("shim script not created: %v", err)
		}

		// Check it's executable
		if info.Mode()&0o111 == 0 {
			t.Fatal("shim script is not executable")
		}

		// Check content
		data, err := os.ReadFile(shimPath)
		if err != nil {
			t.Fatalf("failed to read shim: %v", err)
		}
		content := string(data)

		if !strings.HasPrefix(content, "#!/bin/sh") {
			t.Fatal("shim should start with #!/bin/sh")
		}
		if !strings.Contains(content, "ISTIOENV_ROOT=") {
			t.Fatal("shim should contain ISTIOENV_ROOT")
		}
		if !strings.Contains(content, "ISTIOENV_VERSION") {
			t.Fatal("shim should check ISTIOENV_VERSION")
		}
		if !strings.Contains(content, ".istioctl-version") {
			t.Fatal("shim should check .istioctl-version")
		}
		if !strings.Contains(content, "exec") {
			t.Fatal("shim should exec the binary")
		}
	})

	t.Run("creates shims directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := GenerateShimScript(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		shimsDir := filepath.Join(tmpDir, "shims")
		info, err := os.Stat(shimsDir)
		if err != nil {
			t.Fatalf("shims directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("shims should be a directory")
		}
	})
}

func TestGenerateShellInit(t *testing.T) {
	t.Run("contains PATH update", func(t *testing.T) {
		output := GenerateShellInit("/home/user/.istioenv")
		if !strings.Contains(output, `/home/user/.istioenv/shims:$PATH`) {
			t.Fatal("shell init should prepend shims to PATH")
		}
	})

	t.Run("contains shell function", func(t *testing.T) {
		output := GenerateShellInit("/home/user/.istioenv")
		if !strings.Contains(output, "istioctl-env()") {
			t.Fatal("shell init should define istioctl-env function")
		}
	})

	t.Run("intercepts shell subcommand", func(t *testing.T) {
		output := GenerateShellInit("/home/user/.istioenv")
		if !strings.Contains(output, `"$1" = "shell"`) {
			t.Fatal("shell init should intercept shell subcommand")
		}
		if !strings.Contains(output, "export ISTIOENV_VERSION") {
			t.Fatal("shell init should export ISTIOENV_VERSION")
		}
	})

	t.Run("delegates other commands", func(t *testing.T) {
		output := GenerateShellInit("/home/user/.istioenv")
		if !strings.Contains(output, `command istioctl-env "$@"`) {
			t.Fatal("shell init should delegate other commands to binary")
		}
	})
}
