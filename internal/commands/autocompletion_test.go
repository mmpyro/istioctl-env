package commands

import (
	"strings"
	"testing"
)

func TestAutocompletion(t *testing.T) {
	t.Run("prints bash completion script by default", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/bash")
		output := captureStdout(t, func() {
			err := Autocompletion("")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "complete -F _istioctl_env_completions istioctl-env") {
			t.Errorf("output should contain completion definition, got: %q", output)
		}

		if !strings.Contains(output, "opts=\"help list list-remote init install uninstall prune shell local global latest which exec resolve status upgrade doctor version\"") {
			t.Errorf("output should contain subcommands list, got: %q", output)
		}
	})

	t.Run("--shell zsh prints #compdef header", func(t *testing.T) {
		output := captureStdout(t, func() {
			if err := Autocompletion("zsh"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.HasPrefix(output, "#compdef istioctl-env") {
			t.Fatalf("zsh completion should start with #compdef header, got:\n%s", output)
		}
		if !strings.Contains(output, "_istioctl-env()") {
			t.Fatalf("zsh completion should define _istioctl-env, got:\n%s", output)
		}
		if !strings.Contains(output, "_arguments") {
			t.Fatalf("zsh completion should be _arguments-based, got:\n%s", output)
		}
		// Each version-taking subcommand must appear in the args state dispatch.
		for _, sub := range []string{"install", "uninstall", "shell", "local", "global", "exec"} {
			if !strings.Contains(output, sub) {
				t.Fatalf("zsh completion should handle subcommand %q, got:\n%s", sub, output)
			}
		}
	})

	t.Run("--shell zsh offers remote versions for install from the cache", func(t *testing.T) {
		output := captureStdout(t, func() {
			if err := Autocompletion("zsh"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "__istioctl_env_remote_versions") {
			t.Fatalf("zsh completion should define a remote-version helper, got:\n%s", output)
		}
		if !strings.Contains(output, "istioctl-env list-remote --cached") {
			t.Fatalf("zsh completion should source remote versions from `list-remote --cached`, got:\n%s", output)
		}
		if !strings.Contains(output, "__istioctl_env_installed_versions") {
			t.Fatalf("zsh completion should define an installed-version helper, got:\n%s", output)
		}
	})

	t.Run("--shell zsh completes --shell for init and autocompletion", func(t *testing.T) {
		output := captureStdout(t, func() {
			if err := Autocompletion("zsh"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "(bash zsh fish)") {
			t.Fatalf("zsh completion should offer bash/zsh/fish for --shell, got:\n%s", output)
		}
	})

	t.Run("--shell fish prints complete -c istioctl-env lines", func(t *testing.T) {
		output := captureStdout(t, func() {
			if err := Autocompletion("fish"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "complete -c istioctl-env -f") {
			t.Fatalf("fish completion should disable file completion, got:\n%s", output)
		}
		if !strings.Contains(output, "__fish_use_subcommand") {
			t.Fatalf("fish completion should use __fish_use_subcommand, got:\n%s", output)
		}
		// install should offer remote versions from the cache; the other
		// version-taking subcommands should offer installed versions.
		if !strings.Contains(output, "__fish_seen_subcommand_from install'") &&
			!strings.Contains(output, "__fish_seen_subcommand_from install ") {
			t.Fatalf("fish completion should special-case install, got:\n%s", output)
		}
		if !strings.Contains(output, "istioctl-env list-remote --cached") {
			t.Fatalf("fish completion should pull remote versions from the cache for install, got:\n%s", output)
		}
		if !strings.Contains(output, "__fish_seen_subcommand_from uninstall shell local global exec") {
			t.Fatalf("fish completion should offer installed versions for uninstall/shell/local/global/exec, got:\n%s", output)
		}
	})

	t.Run("auto-detects fish from $SHELL", func(t *testing.T) {
		t.Setenv("SHELL", "/opt/homebrew/bin/fish")
		output := captureStdout(t, func() {
			if err := Autocompletion(""); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if !strings.Contains(output, "complete -c istioctl-env") {
			t.Fatalf("expected fish completion from auto-detection, got:\n%s", output)
		}
	})

	t.Run("falls back to bash with WARN when $SHELL unknown", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/dash")
		var warn string
		output := captureStdout(t, func() {
			warn = captureStderr(t, func() {
				if err := Autocompletion(""); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		})
		if !strings.Contains(output, "complete -F _istioctl_env_completions istioctl-env") {
			t.Fatal("expected bash fallback completion")
		}
		if !strings.Contains(warn, "WARN") {
			t.Fatalf("expected WARN on fallback, got: %q", warn)
		}
	})

	t.Run("unknown --shell value is rejected", func(t *testing.T) {
		err := Autocompletion("ksh")
		if err == nil {
			t.Fatal("expected error for unknown shell")
		}
		if !strings.Contains(err.Error(), "ksh") {
			t.Fatalf("error should mention bad value, got: %v", err)
		}
	})
}
