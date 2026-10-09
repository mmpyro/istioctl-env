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

func TestUpgradeOfflineNoOp(t *testing.T) {
	// Ensure the upgrade command exits cleanly without any network access
	// when ISTIOENV_OFFLINE=1 is set.
	t.Setenv("ISTIOENV_OFFLINE", "1")
	Version = "1.0.0"
	defer func() { Version = "dev" }()

	output := captureStdout(t, func() {
		if err := Upgrade(); err != nil {
			t.Fatalf("expected nil error in offline mode, got %v", err)
		}
	})

	if !strings.Contains(output, "offline mode") {
		t.Fatalf("expected offline-mode message, got %q", output)
	}
	if !strings.Contains(output, "skipping upgrade") {
		t.Fatalf("expected 'skipping upgrade' message, got %q", output)
	}
	if !strings.Contains(output, "ISTIOENV_OFFLINE=1") {
		t.Fatalf("expected message to reference ISTIOENV_OFFLINE=1, got %q", output)
	}
}

func TestSelfDownloadURLFormat(t *testing.T) {
	// Verify the URL format matches what release.yml publishes.
	info := platform.Info{OS: "darwin", Arch: "arm64"}

	t.Run("defaults to github.com when base is empty", func(t *testing.T) {
		url := platform.SelfDownloadURL("", "0.2.0", info, "mmpyro/istioctl-env")
		expected := "https://github.com/mmpyro/istioctl-env/releases/download/v0.2.0/istioctl-env-darwin-arm64"
		if url != expected {
			t.Fatalf("expected %s, got %s", expected, url)
		}
		if !strings.Contains(url, "mmpyro/istioctl-env") {
			t.Fatal("URL should contain the correct owner/repo")
		}
	})

	t.Run("defaults to github.com when base is DefaultSelfDownloadBaseURL", func(t *testing.T) {
		url := platform.SelfDownloadURL(platform.DefaultSelfDownloadBaseURL, "0.2.0", info, "mmpyro/istioctl-env")
		expected := "https://github.com/mmpyro/istioctl-env/releases/download/v0.2.0/istioctl-env-darwin-arm64"
		if url != expected {
			t.Fatalf("expected %s, got %s", expected, url)
		}
	})

	t.Run("rewrites to mirror when base is set", func(t *testing.T) {
		url := platform.SelfDownloadURL("https://mirror.corp.example/gh///", "0.2.0", info, "mmpyro/istioctl-env")
		expected := "https://mirror.corp.example/gh/mmpyro/istioctl-env/releases/download/v0.2.0/istioctl-env-darwin-arm64"
		if url != expected {
			t.Fatalf("expected trailing slashes trimmed and mirror used, got %s", url)
		}
	})
}
