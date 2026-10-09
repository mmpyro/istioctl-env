package commands

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/user/istioctl-env/internal/shim"
)

// isolateEnv clears environment variables that could leak between test
// scenarios and influence the doctor checks.
func isolateEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ISTIOENV_ROOT", "")
	t.Setenv("ISTIOENV_VERSION", "")
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
}

// initTempRoot creates a minimally-initialised root for the doctor to inspect.
func initTempRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "versions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := shim.GenerateShimScript(root); err != nil {
		t.Fatal(err)
	}
	return root
}

// installVersion materialises a fake installed istioctl version in root.
func installVersion(t *testing.T, root, version string, mode os.FileMode) {
	t.Helper()
	dir := filepath.Join(root, "versions", version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "istioctl")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho fake\n"), mode); err != nil {
		t.Fatal(err)
	}
	// os.WriteFile honours the umask, so ensure the mode we asked for.
	if err := os.Chmod(bin, mode); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Individual check tests
// ---------------------------------------------------------------------------

func TestCheckEnvRoot(t *testing.T) {
	t.Run("fails when ISTIOENV_ROOT unset", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "")
		res := checkEnvRoot()
		if res.Status != statusFAIL {
			t.Fatalf("expected FAIL, got %s (%s)", res.Status, res.Detail)
		}
	})

	t.Run("ok when ISTIOENV_ROOT set", func(t *testing.T) {
		t.Setenv("ISTIOENV_ROOT", "/tmp/some-root")
		res := checkEnvRoot()
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s", res.Status)
		}
		if !strings.Contains(res.Detail, "/tmp/some-root") {
			t.Fatalf("expected detail to contain root, got %q", res.Detail)
		}
	})
}

func TestCheckRootWritable(t *testing.T) {
	t.Run("ok when writable", func(t *testing.T) {
		dir := t.TempDir()
		res := checkRootWritable(dir)
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s (%s)", res.Status, res.Detail)
		}
	})

	t.Run("fails when missing", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nope")
		res := checkRootWritable(dir)
		if res.Status != statusFAIL {
			t.Fatalf("expected FAIL, got %s", res.Status)
		}
	})

	t.Run("fails when not a directory", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		res := checkRootWritable(f)
		if res.Status != statusFAIL {
			t.Fatalf("expected FAIL, got %s", res.Status)
		}
	})

	t.Run("fails when readonly", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("readonly directory enforcement unreliable here")
		}
		dir := t.TempDir()
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		res := checkRootWritable(dir)
		if res.Status != statusFAIL {
			t.Fatalf("expected FAIL for readonly dir, got %s (%s)", res.Status, res.Detail)
		}
	})
}

func TestCheckVersionsDir(t *testing.T) {
	t.Run("ok when exists", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		res := checkVersionsDir(root)
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s", res.Status)
		}
	})

	t.Run("fails when missing", func(t *testing.T) {
		res := checkVersionsDir(t.TempDir())
		if res.Status != statusFAIL {
			t.Fatalf("expected FAIL, got %s", res.Status)
		}
	})

	t.Run("fails when a file", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "versions"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		res := checkVersionsDir(root)
		if res.Status != statusFAIL {
			t.Fatalf("expected FAIL, got %s", res.Status)
		}
	})
}

func TestCheckShimDrift(t *testing.T) {
	t.Run("warns when missing", func(t *testing.T) {
		res := checkShimDrift(t.TempDir())
		if res.Status != statusWARN {
			t.Fatalf("expected WARN, got %s", res.Status)
		}
	})

	t.Run("ok when matches generator", func(t *testing.T) {
		root := t.TempDir()
		if err := shim.GenerateShimScript(root); err != nil {
			t.Fatal(err)
		}
		res := checkShimDrift(root)
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s (%s)", res.Status, res.Detail)
		}
	})

	t.Run("warns on drift", func(t *testing.T) {
		root := t.TempDir()
		if err := shim.GenerateShimScript(root); err != nil {
			t.Fatal(err)
		}
		shimPath := filepath.Join(root, "shims", "istioctl")
		if err := os.WriteFile(shimPath, []byte("#!/bin/sh\necho tampered\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		res := checkShimDrift(root)
		if res.Status != statusWARN {
			t.Fatalf("expected WARN, got %s (%s)", res.Status, res.Detail)
		}
	})
}

func TestCheckPATH(t *testing.T) {
	t.Run("fails when shims missing", func(t *testing.T) {
		t.Setenv("PATH", "/usr/local/bin:/usr/bin")
		res := checkPATH(t.TempDir())
		if res.Status != statusFAIL {
			t.Fatalf("expected FAIL, got %s", res.Status)
		}
	})

	t.Run("ok when shims first and no other istioctl", func(t *testing.T) {
		root := t.TempDir()
		shimsDir := filepath.Join(root, "shims")
		if err := os.MkdirAll(shimsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		otherDir := t.TempDir()
		t.Setenv("PATH", shimsDir+string(os.PathListSeparator)+otherDir)
		res := checkPATH(root)
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s (%s)", res.Status, res.Detail)
		}
	})

	t.Run("warns when another istioctl precedes shims", func(t *testing.T) {
		root := t.TempDir()
		shimsDir := filepath.Join(root, "shims")
		if err := os.MkdirAll(shimsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		otherDir := t.TempDir()
		// Place an executable istioctl ahead of the shim.
		if err := os.WriteFile(filepath.Join(otherDir, "istioctl"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", otherDir+string(os.PathListSeparator)+shimsDir)
		res := checkPATH(root)
		if res.Status != statusWARN {
			t.Fatalf("expected WARN, got %s (%s)", res.Status, res.Detail)
		}
	})
}

func TestCheckShellIntegration(t *testing.T) {
	t.Run("info when no rc files", func(t *testing.T) {
		res := checkShellIntegration(t.TempDir())
		if res.Status != statusINFO {
			t.Fatalf("expected INFO, got %s", res.Status)
		}
	})

	t.Run("info when HOME empty", func(t *testing.T) {
		res := checkShellIntegration("")
		if res.Status != statusINFO {
			t.Fatalf("expected INFO when HOME empty, got %s", res.Status)
		}
	})

	t.Run("ok when bashrc references init", func(t *testing.T) {
		home := t.TempDir()
		if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("eval \"$(istioctl-env init)\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		res := checkShellIntegration(home)
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s (%s)", res.Status, res.Detail)
		}
	})

	t.Run("ok when fish config references init", func(t *testing.T) {
		home := t.TempDir()
		fishDir := filepath.Join(home, ".config", "fish")
		if err := os.MkdirAll(fishDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fishDir, "config.fish"), []byte("istioctl-env init | source\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		res := checkShellIntegration(home)
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s", res.Status)
		}
	})
}

func TestCheckResolvedVersion(t *testing.T) {
	t.Run("fails when none configured", func(t *testing.T) {
		isolateEnv(t)
		root := t.TempDir()
		t.Setenv("ISTIOENV_ROOT", root)
		t.Chdir(t.TempDir())
		res, v := checkResolvedVersion()
		if res.Status != statusFAIL {
			t.Fatalf("expected FAIL, got %s", res.Status)
		}
		if v != "" {
			t.Fatalf("expected empty version, got %q", v)
		}
	})

	t.Run("ok when shell version set", func(t *testing.T) {
		isolateEnv(t)
		t.Setenv("ISTIOENV_VERSION", "1.24.0")
		res, v := checkResolvedVersion()
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s", res.Status)
		}
		if v != "1.24.0" {
			t.Fatalf("expected 1.24.0, got %q", v)
		}
	})
}

func TestCheckActiveBinary(t *testing.T) {
	t.Run("ok when executable", func(t *testing.T) {
		root := t.TempDir()
		installVersion(t, root, "1.24.0", 0o755)
		res := checkActiveBinary(root, "1.24.0")
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s (%s)", res.Status, res.Detail)
		}
	})

	t.Run("fails when missing", func(t *testing.T) {
		root := t.TempDir()
		res := checkActiveBinary(root, "1.24.0")
		if res.Status != statusFAIL {
			t.Fatalf("expected FAIL, got %s", res.Status)
		}
	})

	t.Run("fails when non-executable", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("executable bit not meaningful on Windows")
		}
		root := t.TempDir()
		installVersion(t, root, "1.24.0", 0o644)
		res := checkActiveBinary(root, "1.24.0")
		if res.Status != statusFAIL {
			t.Fatalf("expected FAIL, got %s (%s)", res.Status, res.Detail)
		}
	})
}

func TestCheckInstalledBinaries(t *testing.T) {
	t.Run("ok when empty", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		res := checkInstalledBinaries(root)
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s", res.Status)
		}
	})

	t.Run("ok when all healthy", func(t *testing.T) {
		root := t.TempDir()
		installVersion(t, root, "1.24.0", 0o755)
		installVersion(t, root, "1.25.0", 0o755)
		res := checkInstalledBinaries(root)
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s (%s)", res.Status, res.Detail)
		}
	})

	t.Run("warns when a binary is bad", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("executable bit not meaningful on Windows")
		}
		root := t.TempDir()
		installVersion(t, root, "1.24.0", 0o755)
		installVersion(t, root, "1.25.0", 0o644)
		res := checkInstalledBinaries(root)
		if res.Status != statusWARN {
			t.Fatalf("expected WARN, got %s (%s)", res.Status, res.Detail)
		}
		if !strings.Contains(res.Detail, "1.25.0") {
			t.Fatalf("expected detail to mention 1.25.0, got %q", res.Detail)
		}
	})

	t.Run("warns when versions dir missing", func(t *testing.T) {
		res := checkInstalledBinaries(filepath.Join(t.TempDir(), "no-such"))
		if res.Status != statusWARN {
			t.Fatalf("expected WARN, got %s", res.Status)
		}
	})
}

func TestCheckGitHub(t *testing.T) {
	t.Run("ok on success", func(t *testing.T) {
		res := checkGitHub(func() error { return nil })
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s", res.Status)
		}
	})

	t.Run("warns on failure", func(t *testing.T) {
		res := checkGitHub(func() error { return errors.New("timeout") })
		if res.Status != statusWARN {
			t.Fatalf("expected WARN, got %s", res.Status)
		}
		if !strings.Contains(res.Detail, "timeout") {
			t.Fatalf("expected detail to contain cause, got %q", res.Detail)
		}
	})

	t.Run("falls back to default when nil pinger", func(t *testing.T) {
		// Just ensure the nil path does not panic.  We cannot reliably
		// exercise the network here, so accept either outcome.
		res := checkGitHub(nil)
		if res.Status != statusOK && res.Status != statusWARN {
			t.Fatalf("expected OK or WARN, got %s", res.Status)
		}
	})
}

func TestCheckCache(t *testing.T) {
	t.Run("ok when absent", func(t *testing.T) {
		res := checkCache(t.TempDir())
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s", res.Status)
		}
	})

	t.Run("ok when valid json", func(t *testing.T) {
		root := t.TempDir()
		cacheDir := filepath.Join(root, "cache")
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cacheDir, "releases.json"), []byte(`{"versions":["1.24.0"]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		res := checkCache(root)
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s (%s)", res.Status, res.Detail)
		}
	})

	t.Run("warns when corrupt", func(t *testing.T) {
		root := t.TempDir()
		cacheDir := filepath.Join(root, "cache")
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cacheDir, "releases.json"), []byte(`{not json`), 0o644); err != nil {
			t.Fatal(err)
		}
		res := checkCache(root)
		if res.Status != statusWARN {
			t.Fatalf("expected WARN, got %s", res.Status)
		}
	})
}

// ---------------------------------------------------------------------------
// End-to-end runDoctor tests
// ---------------------------------------------------------------------------

func TestRunDoctor_Uninitialized(t *testing.T) {
	isolateEnv(t)
	t.Chdir(t.TempDir())

	var buf bytes.Buffer
	err := runDoctor(false, func() error { return nil }, &buf)
	if err == nil {
		t.Fatal("expected error because ISTIOENV_ROOT is unset")
	}
	out := buf.String()
	for _, want := range []string{"ISTIOENV_ROOT", "Shell integration", "GitHub"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "[FAIL]") {
		t.Errorf("expected at least one FAIL line, got:\n%s", out)
	}
}

func TestRunDoctor_HealthyEnvironment(t *testing.T) {
	isolateEnv(t)
	root := initTempRoot(t)
	t.Setenv("ISTIOENV_ROOT", root)
	installVersion(t, root, "1.24.0", 0o755)
	t.Setenv("ISTIOENV_VERSION", "1.24.0")
	t.Setenv("PATH", filepath.Join(root, "shims"))
	// Seed a bashrc so shell-integration reports OK.
	home := os.Getenv("HOME")
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("eval \"$(istioctl-env init)\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	var buf bytes.Buffer
	err := runDoctor(false, func() error { return nil }, &buf)
	if err != nil {
		t.Fatalf("expected no error in healthy env, got %v\n%s", err, buf.String())
	}
	out := buf.String()
	if strings.Contains(out, "[FAIL]") {
		t.Fatalf("expected no FAIL lines in healthy env, got:\n%s", out)
	}
}

func TestRunDoctor_FixRegeneratesShim(t *testing.T) {
	isolateEnv(t)
	root := initTempRoot(t)
	t.Setenv("ISTIOENV_ROOT", root)
	installVersion(t, root, "1.24.0", 0o644) // non-executable to be fixed
	t.Setenv("ISTIOENV_VERSION", "1.24.0")
	t.Setenv("PATH", filepath.Join(root, "shims"))
	t.Chdir(t.TempDir())

	// Tamper with the shim so drift is detected.
	shimPath := filepath.Join(root, "shims", "istioctl")
	if err := os.WriteFile(shimPath, []byte("#!/bin/sh\necho tampered\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Without --fix, drift is reported.
	var preBuf bytes.Buffer
	_ = runDoctor(false, func() error { return nil }, &preBuf)
	if !strings.Contains(preBuf.String(), "[WARN]") {
		t.Fatalf("expected WARN before fix, got:\n%s", preBuf.String())
	}

	// With --fix, drift is resolved and the binary is chmod'd.
	var buf bytes.Buffer
	if err := runDoctor(true, func() error { return nil }, &buf); err != nil {
		t.Fatalf("unexpected error after --fix: %v\n%s", err, buf.String())
	}
	expected, err := expectedShimContent(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(shimPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("shim not regenerated: diff")
	}
	info, err := os.Stat(filepath.Join(root, "versions", "1.24.0", "istioctl"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("binary should be executable after --fix, got mode %o", info.Mode().Perm())
	}
}

func TestDoctorHelp(t *testing.T) {
	out := captureStdout(t, DoctorHelp)
	if !strings.Contains(out, "doctor") || !strings.Contains(out, "--fix") {
		t.Fatalf("help should mention doctor and --fix, got:\n%s", out)
	}
}

func TestDoctor_PublicWrapperExits(t *testing.T) {
	// Just make sure the Doctor() wrapper does not panic when invoked
	// with ISTIOENV_ROOT unset and the real pinger.  We expect a non-nil
	// error (missing root triggers FAIL).
	isolateEnv(t)
	t.Chdir(t.TempDir())
	out := captureStdout(t, func() {
		if err := Doctor(false); err == nil {
			t.Fatal("expected error from Doctor() with unset env")
		}
	})
	if !strings.Contains(out, "ISTIOENV_ROOT") {
		t.Fatalf("expected ISTIOENV_ROOT check in output, got:\n%s", out)
	}
}

// pingerExample is a simple compile-time sanity check that the pinger type
// matches the expected signature; this doubles as a stand-in test helper.
var _ pinger = func() error { return fmt.Errorf("stub") }
