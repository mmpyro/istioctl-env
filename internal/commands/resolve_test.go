package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/istioctl-env/internal/github"
)

// setupVersions creates a $ISTIOENV_ROOT/versions tree with the given
// versions each containing a stub istioctl binary.
func setupVersions(t *testing.T, versions ...string) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("ISTIOENV_ROOT", tmpDir)
	for _, v := range versions {
		dir := filepath.Join(tmpDir, "versions", v)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "istioctl"), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Make sure versions dir exists even when no versions are given.
	if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
		t.Fatal(err)
	}
	return tmpDir
}

func TestResolve_FailsWhenNotInitialized(t *testing.T) {
	t.Setenv("ISTIOENV_ROOT", "")
	if err := Resolve(false, false); err == nil {
		t.Fatal("expected error when not initialized")
	}
}

func TestResolve_ExactPin_Installed(t *testing.T) {
	setupVersions(t, "1.24.0")
	t.Setenv("ISTIOENV_VERSION", "1.24.0")

	out := captureStdout(t, func() {
		if err := Resolve(false, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if strings.TrimSpace(out) != "1.24.0" {
		t.Fatalf("expected 1.24.0, got %q", strings.TrimSpace(out))
	}
}

func TestResolve_ExactPin_NotInstalled_NoAutoInstall(t *testing.T) {
	setupVersions(t)
	t.Setenv("ISTIOENV_VERSION", "1.24.0")

	err := Resolve(false, true)
	if err == nil {
		t.Fatal("expected error when exact pin not installed and auto-install disabled")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("expected 'not installed' error, got: %v", err)
	}
}

func TestResolve_Caret_PicksHighestInstalled(t *testing.T) {
	setupVersions(t, "1.24.0", "1.24.3", "1.25.1", "2.0.0")
	t.Setenv("ISTIOENV_VERSION", "^1.24.0")

	out := captureStdout(t, func() {
		if err := Resolve(false, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if strings.TrimSpace(out) != "1.25.1" {
		t.Fatalf("expected 1.25.1, got %q", strings.TrimSpace(out))
	}
}

func TestResolve_Constraint_NoInstalledMatch_NoAutoInstall(t *testing.T) {
	setupVersions(t, "1.23.0")
	t.Setenv("ISTIOENV_VERSION", "^1.24.0")

	err := Resolve(false, true)
	if err == nil {
		t.Fatal("expected error when no installed match and auto-install disabled")
	}
	if !strings.Contains(err.Error(), "no installed") {
		t.Fatalf("expected 'no installed' error, got: %v", err)
	}
}

func TestResolve_AutoInstallEnvVar(t *testing.T) {
	tmpDir := setupVersions(t)
	t.Setenv("ISTIOENV_VERSION", "1.24.0")
	t.Setenv("ISTIOENV_AUTO_INSTALL", "true")

	// Serve a tar.gz that extracts a stub istioctl + its sha256.
	archive := createMiniTarGz(t, "istioctl", []byte("fake"))
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

	client := &github.Client{
		BaseURL:         server.URL,
		DownloadBaseURL: server.URL,
		HTTPClient:      server.Client(),
	}

	out := captureStdout(t, func() {
		if err := resolveWithClient(client, false, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if strings.TrimSpace(out) != "1.24.0" {
		t.Fatalf("expected resolved=1.24.0, got %q", strings.TrimSpace(out))
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "versions", "1.24.0", "istioctl")); err != nil {
		t.Fatalf("expected auto-install to have placed the binary: %v", err)
	}
}

func TestResolve_Constraint_AutoInstallsBestRemote(t *testing.T) {
	tmpDir := setupVersions(t) // nothing installed
	t.Setenv("ISTIOENV_VERSION", "~1.24.0")

	// Pre-populate the on-disk release cache so the resolver sees exactly
	// the candidate pool this test cares about (bypasses baseline + delta).
	seedCache(t, tmpDir,
		[]string{"1.25.0", "1.24.5", "1.24.3", "1.23.9"},
		[]string{"1.25.0", "1.24.5", "1.24.3", "1.23.9"},
	)

	archive := createMiniTarGz(t, "istioctl", []byte("fake"))
	sum := sha256.Sum256(archive)
	sumStr := hex.EncodeToString(sum[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".sha256"):
			fmt.Fprint(w, sumStr)
		default:
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(archive)))
			_, _ = w.Write(archive)
		}
	}))
	defer server.Close()

	client := &github.Client{
		BaseURL:         server.URL,
		DownloadBaseURL: server.URL,
		HTTPClient:      server.Client(),
	}

	out := captureStdout(t, func() {
		if err := resolveWithClient(client, true, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	// Highest match for ~1.24.0 is 1.24.5.
	if strings.TrimSpace(out) != "1.24.5" {
		t.Fatalf("expected resolved=1.24.5, got %q", strings.TrimSpace(out))
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "versions", "1.24.5", "istioctl")); err != nil {
		t.Fatalf("expected auto-install to have placed 1.24.5: %v", err)
	}
}

func TestAutoInstallEnabled_TruthyParsing(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "on"} {
		t.Setenv("ISTIOENV_AUTO_INSTALL", v)
		if !AutoInstallEnabled() {
			t.Fatalf("expected %q to be truthy", v)
		}
	}
	for _, v := range []string{"", "0", "false", "no", "off", "nope"} {
		t.Setenv("ISTIOENV_AUTO_INSTALL", v)
		if AutoInstallEnabled() {
			t.Fatalf("expected %q to be falsy", v)
		}
	}
}

// createMiniTarGz is a lightweight local helper that mirrors createTestTarGz
// but is kept here so test files remain independent.
func createMiniTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Mode:     0o755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
