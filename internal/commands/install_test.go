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

func TestInstall(t *testing.T) {
	t.Run("fails when not initialized", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		err := Install("1.24.0", true)
		if err == nil {
			t.Fatal("expected error when not initialized")
		}
	})

	t.Run("skips already installed version", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)

		version := "1.24.0"
		// Create version directory with binary
		versionDir := filepath.Join(tmpDir, "versions", version)
		if err := os.MkdirAll(versionDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(versionDir, "istioctl"), []byte("binary"), 0o755); err != nil {
			t.Fatal(err)
		}

		output := captureStdout(t, func() {
			err := Install(version, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "already installed") {
			t.Fatalf("expected 'already installed' message, got %q", output)
		}
	})

	t.Run("silent flag suppresses output", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		version := "1.24.0"
		versionDir := filepath.Join(tmpDir, "versions", version)
		if err := os.MkdirAll(versionDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(versionDir, "istioctl"), []byte("binary"), 0o755); err != nil {
			t.Fatal(err)
		}

		output := captureStdout(t, func() {
			err := Install(version, true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if output != "" {
			t.Fatalf("expected no output with silent flag, got %q", output)
		}
	})

	t.Run("install with progress bar and checksum", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		// Initialize versions directory to satisfy config.RequireInit()
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		version := "1.24.0"

		// Create a tar.gz archive containing an istioctl binary
		archiveData := createTestTarGz(t, "istioctl", []byte("fake binary content"))
		checksum := sha256.Sum256(archiveData)
		checksumStr := hex.EncodeToString(checksum[:])

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, ".sha256") {
				fmt.Fprint(w, checksumStr)
				return
			}
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(archiveData)))
			_, _ = w.Write(archiveData)
		}))
		defer server.Close()

		client := &github.Client{
			BaseURL:         server.URL,
			DownloadBaseURL: server.URL,
			HTTPClient:      server.Client(),
		}

		output := captureStdout(t, func() {
			err := installWithClient(client, version, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if !strings.Contains(output, "[##################################################] 100%") {
			t.Errorf("expected progress bar, got %q", output)
		}
		if !strings.Contains(output, "Checksum verified successfully") {
			t.Errorf("expected checksum verification message, got %q", output)
		}

		// Verify binary was written
		binaryPath := filepath.Join(tmpDir, "versions", version, "istioctl")
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			t.Fatal("binary was not written")
		}
	})
}

// createTestTarGz builds a tar.gz archive containing a single file with the
// given name and content.
func createTestTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name:     name,
		Mode:     0o755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
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
