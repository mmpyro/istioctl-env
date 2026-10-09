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

func TestGenerateShellInitBash(t *testing.T) {
	root := "/home/user/.istioenv"

	t.Run("contains PATH update", func(t *testing.T) {
		output := GenerateShellInitBash(root)
		if !strings.Contains(output, `/home/user/.istioenv/shims:$PATH`) {
			t.Fatal("bash shell init should prepend shims to PATH")
		}
	})

	t.Run("contains shell function", func(t *testing.T) {
		output := GenerateShellInitBash(root)
		if !strings.Contains(output, "istioctl-env()") {
			t.Fatal("bash shell init should define istioctl-env function")
		}
	})

	t.Run("intercepts shell subcommand", func(t *testing.T) {
		output := GenerateShellInitBash(root)
		if !strings.Contains(output, `"$1" = "shell"`) {
			t.Fatal("bash shell init should intercept shell subcommand")
		}
		if !strings.Contains(output, "export ISTIOENV_VERSION") {
			t.Fatal("bash shell init should export ISTIOENV_VERSION")
		}
	})

	t.Run("delegates other commands", func(t *testing.T) {
		output := GenerateShellInitBash(root)
		if !strings.Contains(output, `command istioctl-env "$@"`) {
			t.Fatal("bash shell init should delegate other commands to binary")
		}
	})
}

func TestGenerateShellInitZsh(t *testing.T) {
	root := "/opt/istioenv"
	output := GenerateShellInitZsh(root)

	if !strings.Contains(output, `/opt/istioenv/shims:$PATH`) {
		t.Fatal("zsh shell init should prepend shims to PATH with ISTIOENV_ROOT substituted")
	}
	if !strings.Contains(output, "function istioctl-env()") {
		t.Fatal("zsh shell init should define `function istioctl-env()` block")
	}
	if !strings.Contains(output, "export ISTIOENV_VERSION") {
		t.Fatal("zsh shell init should export ISTIOENV_VERSION on shell <ver>")
	}
	if !strings.Contains(output, `command istioctl-env "$@"`) {
		t.Fatal("zsh shell init should delegate other commands")
	}
}

func TestGenerateShellInitFish(t *testing.T) {
	root := "/opt/istioenv"
	output := GenerateShellInitFish(root)

	if !strings.Contains(output, `set -gx PATH "/opt/istioenv/shims" $PATH`) {
		t.Fatal("fish shell init should prepend shims to PATH using set -gx")
	}
	if !strings.Contains(output, "function istioctl-env") {
		t.Fatal("fish shell init should define `function istioctl-env`")
	}
	if !strings.Contains(output, `set -gx ISTIOENV_VERSION $argv[2]`) {
		t.Fatal("fish shell init should export ISTIOENV_VERSION via set -gx")
	}
	if !strings.Contains(output, "command istioctl-env $argv") {
		t.Fatal("fish shell init should delegate other commands via $argv")
	}
	if !strings.Contains(output, "end") {
		t.Fatal("fish shell init should include fish function `end`")
	}
}

func TestGenerateShellInitDispatch(t *testing.T) {
	root := "/root"
	cases := map[string]string{
		"bash": `/root/shims:$PATH`,
		"zsh":  "function istioctl-env()",
		"fish": `set -gx PATH "/root/shims" $PATH`,
		"":     `/root/shims:$PATH`,       // fallback to bash
		"foo":  `/root/shims:$PATH`,       // unknown falls back to bash
		"ZSH":  "function istioctl-env()", // case-insensitive
	}
	for shell, needle := range cases {
		shell, needle := shell, needle
		t.Run("shell="+shell, func(t *testing.T) {
			out := GenerateShellInit(root, shell)
			if !strings.Contains(out, needle) {
				t.Fatalf("expected output for shell %q to contain %q, got:\n%s", shell, needle, out)
			}
		})
	}
}

func TestDetectShell(t *testing.T) {
	cases := map[string]string{
		"":                "",
		"/bin/bash":       "bash",
		"/usr/bin/zsh":    "zsh",
		"/opt/homebrew/bin/fish": "fish",
		"/bin/dash":       "",
		"zsh":             "zsh",
		"/usr/local/bin/pwsh": "",
	}
	for in, want := range cases {
		in, want := in, want
		t.Run("SHELL="+in, func(t *testing.T) {
			t.Setenv("SHELL", in)
			got := DetectShell()
			if got != want {
				t.Fatalf("DetectShell() for SHELL=%q = %q, want %q", in, got, want)
			}
		})
	}
}

func TestNormalizeShell(t *testing.T) {
	good := map[string]string{
		"bash": "bash",
		"ZSH":  "zsh",
		"Fish": "fish",
		"  bash  ": "bash",
	}
	for in, want := range good {
		in, want := in, want
		t.Run("ok/"+in, func(t *testing.T) {
			got, err := NormalizeShell(in)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", in, err)
			}
			if got != want {
				t.Fatalf("NormalizeShell(%q) = %q, want %q", in, got, want)
			}
		})
	}

	for _, bad := range []string{"", "ksh", "pwsh", "sh", "cmd"} {
		bad := bad
		t.Run("err/"+bad, func(t *testing.T) {
			_, err := NormalizeShell(bad)
			if err == nil {
				t.Fatalf("expected error for %q", bad)
			}
			msg := err.Error()
			for _, s := range SupportedShells {
				if !strings.Contains(msg, s) {
					t.Fatalf("error %q should list supported shell %q", msg, s)
				}
			}
		})
	}
}
