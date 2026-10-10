package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/user/istioctl-env/internal/github"
)

func TestExec(t *testing.T) {
	t.Run("fails when not initialized", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		err := Exec("0.31.0", []string{"version"}, false)
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

		err := Exec("0.31.0", []string{"version"}, false)
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

		err := Exec("", []string{"version"}, false)
		if err == nil || err.Error() != "version not specified. "+execUsage {
			t.Fatalf("expected version missing error, got %v", err)
		}

		err = Exec("0.31.0", []string{}, false)
		if err == nil || err.Error() != "command not specified. "+execUsage {
			t.Fatalf("expected command missing error, got %v", err)
		}

		// A lone "--" is stripped, leaving no command.
		err = Exec("0.31.0", []string{"--"}, false)
		if err == nil || err.Error() != "command not specified. "+execUsage {
			t.Fatalf("expected command missing error after --, got %v", err)
		}
	})
}

// installStubIstioctl writes a shell-script istioctl into
// $ISTIOENV_ROOT/versions/<v>/ for each version.
func installStubIstioctl(t *testing.T, script string, versions ...string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub istioctl is a POSIX shell script")
	}
	root := setupVersions(t)
	for _, v := range versions {
		dir := filepath.Join(root, "versions", v)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "istioctl"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestExec_ExitCodePassthrough(t *testing.T) {
	installStubIstioctl(t, `exit 3`, "1.24.0")

	err := Exec("1.24.0", []string{"version"}, false)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 3 {
		t.Fatalf("expected exit code 3, got %d", exitErr.ExitCode())
	}
}

func TestExec_StripsLeadingDoubleDashAndSetsVersion(t *testing.T) {
	root := installStubIstioctl(t, `printf '%s|%s\n' "$ISTIOENV_VERSION" "$*" > "$(dirname "$0")/out"`, "1.24.0")

	if err := Exec("1.24.0", []string{"--", "analyze", "--", "x"}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "versions", "1.24.0", "out"))
	if err != nil {
		t.Fatal(err)
	}
	// Only the first "--" is stripped.
	if want := "1.24.0|analyze -- x"; strings.TrimSpace(string(got)) != want {
		t.Fatalf("got %q, want %q", strings.TrimSpace(string(got)), want)
	}
}

func TestExec_ConstraintPicksHighestInstalled(t *testing.T) {
	root := installStubIstioctl(t, `echo "$ISTIOENV_VERSION" > "$(dirname "$0")/out"`, "1.24.0", "1.24.3", "1.25.0")

	if err := Exec("~1.24.0", []string{"version"}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "versions", "1.24.3", "out"))
	if err != nil {
		t.Fatalf("expected 1.24.3 to be executed: %v", err)
	}
	if strings.TrimSpace(string(got)) != "1.24.3" {
		t.Fatalf("expected ISTIOENV_VERSION=1.24.3, got %q", got)
	}
}

func TestExec_ConstraintNoMatchWithoutAuto(t *testing.T) {
	installStubIstioctl(t, `exit 0`, "1.23.0")

	err := Exec("^1.24.0", []string{"version"}, false)
	if err == nil || !strings.Contains(err.Error(), "no installed") {
		t.Fatalf("expected 'no installed' error, got %v", err)
	}
}

func TestExec_AutoInstallsMissingVersion(t *testing.T) {
	tmpDir := setupVersions(t)

	archive := createMiniTarGz(t, "istioctl", []byte("#!/bin/sh\nexit 0\n"))
	sum := sha256.Sum256(archive)
	sumStr := hex.EncodeToString(sum[:])
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".sha256") {
			fmt.Fprint(w, sumStr)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(archive)))
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	client := &github.Client{BaseURL: server.URL, DownloadBaseURL: server.URL, HTTPClient: server.Client()}

	stdout := captureStdout(t, func() {
		if err := execWithClient(client, "1.24.0", []string{"version"}, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if _, err := os.Stat(filepath.Join(tmpDir, "versions", "1.24.0", "istioctl")); err != nil {
		t.Fatalf("expected auto-install to place the binary: %v", err)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("installer output should go to stderr, got stdout %q", stdout)
	}
}
