package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetIstioEnvRoot(t *testing.T) {
	t.Run("returns root when set", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "/tmp/test-istioenv")
		root, ok := GetIstioEnvRoot()
		if !ok {
			t.Fatal("expected ok to be true")
		}
		if root != "/tmp/test-istioenv" {
			t.Fatalf("expected /tmp/test-istioenv, got %s", root)
		}
	})

	t.Run("returns empty when not set", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		root, ok := GetIstioEnvRoot()
		if ok {
			t.Fatal("expected ok to be false")
		}
		if root != "" {
			t.Fatalf("expected empty string, got %s", root)
		}
	})
}

func TestIsInitialized(t *testing.T) {
	t.Run("returns false when ISTIOENV_ROOT not set", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		if IsInitialized() {
			t.Fatal("expected false when ISTIOENV_ROOT not set")
		}
	})

	t.Run("returns false when versions dir does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if IsInitialized() {
			t.Fatal("expected false when versions dir does not exist")
		}
	})

	t.Run("returns true when properly initialized", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		if !IsInitialized() {
			t.Fatal("expected true when properly initialized")
		}
	})
}

func TestRequireInit(t *testing.T) {
	t.Run("returns error when ISTIOENV_ROOT not set", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		err := RequireInit()
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("returns error when versions dir missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		err := RequireInit()
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("returns nil when initialized", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		err := RequireInit()
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})
}

func TestResolveVersion(t *testing.T) {
	t.Run("shell version takes priority", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		t.Setenv("ISTIOENV_VERSION", "1.0.0")

		// Also write a global version to ensure shell takes priority
		if err := os.WriteFile(filepath.Join(tmpDir, "version"), []byte("2.0.0"), 0o644); err != nil {
			t.Fatal(err)
		}

		v, err := ResolveVersion()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "1.0.0" {
			t.Fatalf("expected 1.0.0, got %s", v)
		}
	})

	t.Run("global version used when no shell or local", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		t.Setenv("ISTIOENV_VERSION", "")

		if err := os.WriteFile(filepath.Join(tmpDir, "version"), []byte("2.0.0"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Change to a directory without .istioctl-version
		origDir, _ := os.Getwd()
		noVersionDir := t.TempDir()
		if err := os.Chdir(noVersionDir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(origDir) }()

		v, err := ResolveVersion()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "2.0.0" {
			t.Fatalf("expected 2.0.0, got %s", v)
		}
	})

	t.Run("error when no version configured", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(tmpDir, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		t.Setenv("ISTIOENV_VERSION", "")

		// Change to a directory without .istioctl-version
		origDir, _ := os.Getwd()
		noVersionDir := t.TempDir()
		if err := os.Chdir(noVersionDir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(origDir) }()

		_, err := ResolveVersion()
		if err == nil {
			t.Fatal("expected error when no version configured")
		}
	})
}

func TestFindLocalVersionFrom(t *testing.T) {
	t.Run("finds version in current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, ".istioctl-version"), []byte("1.2.3\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		v, err := FindLocalVersionFrom(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "1.2.3" {
			t.Fatalf("expected 1.2.3, got %s", v)
		}
	})

	t.Run("finds version in parent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		childDir := filepath.Join(tmpDir, "subdir", "nested")
		if err := os.MkdirAll(childDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, ".istioctl-version"), []byte("3.4.5"), 0o644); err != nil {
			t.Fatal(err)
		}

		v, err := FindLocalVersionFrom(childDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "3.4.5" {
			t.Fatalf("expected 3.4.5, got %s", v)
		}
	})

	t.Run("returns error when no version file found", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := FindLocalVersionFrom(tmpDir)
		if err == nil {
			t.Fatal("expected error when no .istioctl-version found")
		}
	})
}

func TestReadGlobalVersion(t *testing.T) {
	t.Run("reads global version", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "version"), []byte("5.6.7\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		v, err := ReadGlobalVersion(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "5.6.7" {
			t.Fatalf("expected 5.6.7, got %s", v)
		}
	})

	t.Run("returns error when file missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := ReadGlobalVersion(tmpDir)
		if err == nil {
			t.Fatal("expected error when version file missing")
		}
	})

	t.Run("returns error when file empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "version"), []byte("  \n"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := ReadGlobalVersion(tmpDir)
		if err == nil {
			t.Fatal("expected error when version file empty")
		}
	})
}

func TestGetVersionDir(t *testing.T) {
	t.Run("returns correct path", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "/tmp/test-istioenv")
		dir, err := GetVersionDir("1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "/tmp/test-istioenv/versions/1.0.0"
		if dir != expected {
			t.Fatalf("expected %s, got %s", expected, dir)
		}
	})

	t.Run("returns error when ISTIOENV_ROOT not set", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		_, err := GetVersionDir("1.0.0")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestGetBinaryPath(t *testing.T) {
	t.Run("returns correct path", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "/tmp/test-istioenv")
		path, err := GetBinaryPath("1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "/tmp/test-istioenv/versions/1.0.0/istioctl"
		if path != expected {
			t.Fatalf("expected %s, got %s", expected, path)
		}
	})
}

func TestIsVersionInstalled(t *testing.T) {
	t.Run("returns true when installed", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)
		versionDir := filepath.Join(tmpDir, "versions", "1.0.0")
		if err := os.MkdirAll(versionDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(versionDir, "istioctl"), []byte("binary"), 0o755); err != nil {
			t.Fatal(err)
		}

		installed, err := IsVersionInstalled("1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !installed {
			t.Fatal("expected version to be installed")
		}
	})

	t.Run("returns false when not installed", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", tmpDir)

		installed, err := IsVersionInstalled("1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if installed {
			t.Fatal("expected version to not be installed")
		}
	})
}
