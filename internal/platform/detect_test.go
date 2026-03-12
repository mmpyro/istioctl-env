package platform

import (
	"runtime"
	"testing"
)

func TestDetect(t *testing.T) {
	info, err := Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// On macOS or Linux, this should succeed
	switch runtime.GOOS {
	case "darwin":
		if info.OS != "osx" {
			t.Fatalf("expected osx, got %s", info.OS)
		}
	case "linux":
		if info.OS != "linux" {
			t.Fatalf("expected linux, got %s", info.OS)
		}
	}

	switch runtime.GOARCH {
	case "amd64":
		if info.Arch != "amd64" {
			t.Fatalf("expected amd64, got %s", info.Arch)
		}
	case "arm64":
		if info.Arch != "arm64" {
			t.Fatalf("expected arm64, got %s", info.Arch)
		}
	}
}

func TestDownloadPath(t *testing.T) {
	tests := []struct {
		version  string
		info     Info
		expected string
	}{
		{
			version:  "1.24.0",
			info:     Info{OS: "linux", Arch: "amd64"},
			expected: "istio/istio/releases/download/1.24.0/istioctl-1.24.0-linux-amd64.tar.gz",
		},
		{
			version:  "1.25.0",
			info:     Info{OS: "osx", Arch: "arm64"},
			expected: "istio/istio/releases/download/1.25.0/istioctl-1.25.0-osx-arm64.tar.gz",
		},
		{
			version:  "1.23.4",
			info:     Info{OS: "linux", Arch: "arm64"},
			expected: "istio/istio/releases/download/1.23.4/istioctl-1.23.4-linux-arm64.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.version+"_"+tt.info.OS+"_"+tt.info.Arch, func(t *testing.T) {
			path := DownloadPath(tt.version, tt.info)
			if path != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, path)
			}
		})
	}
}

func TestChecksumPath(t *testing.T) {
	info := Info{OS: "linux", Arch: "amd64"}
	got := ChecksumPath("1.24.0", info)
	expected := "istio/istio/releases/download/1.24.0/istioctl-1.24.0-linux-amd64.tar.gz.sha256"
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestArchiveName(t *testing.T) {
	info := Info{OS: "osx", Arch: "arm64"}
	got := ArchiveName("1.25.0", info)
	expected := "istioctl-1.25.0-osx-arm64.tar.gz"
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}
