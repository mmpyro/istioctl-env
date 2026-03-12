package commands

import (
	"strings"
	"testing"
)

func TestAutocompletion(t *testing.T) {
	t.Run("prints bash completion script", func(t *testing.T) {
		output := captureStdout(t, func() {
			err := Autocompletion()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "complete -F _istioctl_env_completions istioctl-env") {
			t.Errorf("output should contain completion definition, got: %q", output)
		}

		if !strings.Contains(output, "opts=\"help list list-remote init install uninstall shell local global latest which exec status upgrade version\"") {
			t.Errorf("output should contain subcommands list, got: %q", output)
		}
	})
}
