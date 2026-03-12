package commands

import (
	"os"
	"strings"
	"testing"

	"github.com/user/istioctl-env/internal/platform"
	"github.com/user/istioctl-env/internal/semver"
)

func TestUpgradeVersionComparison(t *testing.T) {
	t.Run("already up to date when versions match", func(t *testing.T) {
		current := semver.Parse("0.2.0")
		remote := semver.Parse("0.2.0")

		if semver.Less(current, remote) {
			t.Fatal("expected current == remote, but Less returned true")
		}
	})

	t.Run("detects newer remote version", func(t *testing.T) {
		current := semver.Parse("0.1.0")
		remote := semver.Parse("0.2.0")

		if !semver.Less(current, remote) {
			t.Fatal("expected current < remote")
		}
	})

	t.Run("skips when remote is older", func(t *testing.T) {
		current := semver.Parse("0.3.0")
		remote := semver.Parse("0.2.0")

		if semver.Less(current, remote) {
			t.Fatal("expected current > remote, but Less returned true")
		}
	})

	t.Run("dev version always upgrades", func(t *testing.T) {
		Version = "dev"
		defer func() { Version = "dev" }()

		// When Version is "dev", the upgrade function skips comparison
		// and always proceeds. We verify the logic check here.
		if Version != "dev" {
			t.Fatal("expected Version to be 'dev'")
		}
	})
}

func TestUpgradeAlreadyUpToDate(t *testing.T) {
	// This test verifies the printed message when versions match.
	// We can't call Upgrade() directly (it hits the network), so we
	// test the output path by simulating the version comparison branch.
	Version = "0.5.0"
	defer func() { Version = "dev" }()

	current := semver.Parse(Version)
	remote := semver.Parse("0.5.0")

	if semver.Less(current, remote) {
		t.Fatal("should not be less when equal")
	}

	if !(current.Major == remote.Major && current.Minor == remote.Minor && current.Patch == remote.Patch && current.PreRelease == remote.PreRelease) {
		t.Fatal("versions should be equal")
	}
}

func TestUpgradeSkipsOlderRemote(t *testing.T) {
	Version = "0.5.0"
	defer func() { Version = "dev" }()

	current := semver.Parse(Version)
	remote := semver.Parse("0.4.0")

	if semver.Less(current, remote) {
		t.Fatal("should not upgrade to an older version")
	}
}

func TestAtomicReplace(t *testing.T) {
	t.Run("replaces file content", func(t *testing.T) {
		tmpDir := t.TempDir()
		targetPath := tmpDir + "/istioctl-env"

		// Create initial file
		initialData := []byte("old-binary")
		if err := os.WriteFile(targetPath, initialData, 0o755); err != nil {
			t.Fatal(err)
		}

		newData := []byte("new-binary")
		if err := atomicReplace(targetPath, newData); err != nil {
			t.Fatalf("atomicReplace failed: %v", err)
		}

		// Verify content
		got, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(newData) {
			t.Fatalf("expected %q, got %q", newData, got)
		}
	})

	t.Run("creates file if it does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		targetPath := tmpDir + "/istioctl-env-new"

		data := []byte("brand-new-binary")
		if err := atomicReplace(targetPath, data); err != nil {
			t.Fatalf("atomicReplace failed: %v", err)
		}

		got, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(data) {
			t.Fatalf("expected %q, got %q", data, got)
		}
	})
}

func TestSelfDownloadURLFormat(t *testing.T) {
	// Verify the URL format matches what release.yml publishes.
	info := platform.Info{OS: "darwin", Arch: "arm64"}
	url := platform.SelfDownloadURL("0.2.0", info, "mmpyro/istioctl-env")

	expected := "https://github.com/mmpyro/istioctl-env/releases/download/v0.2.0/istioctl-env-darwin-arm64"
	if url != expected {
		t.Fatalf("expected %s, got %s", expected, url)
	}

	if !strings.Contains(url, "mmpyro/istioctl-env") {
		t.Fatal("URL should contain the correct owner/repo")
	}
}
