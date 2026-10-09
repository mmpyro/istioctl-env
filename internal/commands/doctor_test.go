package commands

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/user/istioctl-env/internal/github"
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

	t.Run("ok when shell version set and reports source", func(t *testing.T) {
		isolateEnv(t)
		t.Setenv("ISTIOENV_VERSION", "1.24.0")
		res, v := checkResolvedVersion()
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s", res.Status)
		}
		if v != "1.24.0" {
			t.Fatalf("expected 1.24.0, got %q", v)
		}
		if !strings.Contains(res.Detail, "set by ISTIOENV_VERSION") {
			t.Fatalf("expected detail to report env-var source, got %q", res.Detail)
		}
	})

	t.Run("ok when local .istioctl-version file wins and detail says so", func(t *testing.T) {
		isolateEnv(t)
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".istioctl-version"), []byte("1.23.0\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		res, v := checkResolvedVersion()
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s (%s)", res.Status, res.Detail)
		}
		if v != "1.23.0" {
			t.Fatalf("expected 1.23.0, got %q", v)
		}
		if !strings.Contains(res.Detail, ".istioctl-version file") {
			t.Fatalf("expected detail to report local-file source, got %q", res.Detail)
		}
	})

	t.Run("ok when global file wins and detail says so", func(t *testing.T) {
		isolateEnv(t)
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "version"), []byte("1.22.0\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ISTIOENV_ROOT", root)
		// Walk starts in a tempdir that has no .istioctl-version anywhere up the tree.
		t.Chdir(t.TempDir())
		res, v := checkResolvedVersion()
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s (%s)", res.Status, res.Detail)
		}
		if v != "1.22.0" {
			t.Fatalf("expected 1.22.0, got %q", v)
		}
		if !strings.Contains(res.Detail, "global version file") {
			t.Fatalf("expected detail to report global-file source, got %q", res.Detail)
		}
	})
}

func TestCheckDanglingVersions(t *testing.T) {
	t.Run("ok when no versions installed", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		res := checkDanglingVersions(root)
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s (%s)", res.Status, res.Detail)
		}
	})

	t.Run("ok when every version has a binary", func(t *testing.T) {
		root := t.TempDir()
		installVersion(t, root, "1.24.0", 0o755)
		installVersion(t, root, "1.25.0", 0o644)
		res := checkDanglingVersions(root)
		if res.Status != statusOK {
			t.Fatalf("expected OK (non-exec is not dangling), got %s (%s)", res.Status, res.Detail)
		}
	})

	t.Run("warns when a version dir has no binary", func(t *testing.T) {
		root := t.TempDir()
		installVersion(t, root, "1.24.0", 0o755)
		// Create an empty version dir with no istioctl inside.
		if err := os.MkdirAll(filepath.Join(root, "versions", "1.99.0"), 0o755); err != nil {
			t.Fatal(err)
		}
		res := checkDanglingVersions(root)
		if res.Status != statusWARN {
			t.Fatalf("expected WARN, got %s (%s)", res.Status, res.Detail)
		}
		if !strings.Contains(res.Detail, "1.99.0") {
			t.Fatalf("expected detail to mention dangling version, got %q", res.Detail)
		}
		if strings.Contains(res.Detail, "1.24.0") {
			t.Fatalf("healthy version should not appear in dangling list, got %q", res.Detail)
		}
	})
}

func TestCheckInstalledBinaries_SeparatesDangling(t *testing.T) {
	// A dangling version dir (no istioctl at all) must NOT be reported by
	// checkInstalledBinaries — it belongs to checkDanglingVersions.
	root := t.TempDir()
	installVersion(t, root, "1.24.0", 0o755)
	if err := os.MkdirAll(filepath.Join(root, "versions", "1.99.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	res := checkInstalledBinaries(root)
	if res.Status != statusOK {
		t.Fatalf("expected OK (dangling handled elsewhere), got %s (%s)", res.Status, res.Detail)
	}
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
	t.Run("ok on success without rate-limit headers", func(t *testing.T) {
		res := checkGitHub(func() ghStatus { return ghStatus{} })
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s", res.Status)
		}
	})

	t.Run("ok with healthy rate-limit headroom", func(t *testing.T) {
		res := checkGitHub(func() ghStatus {
			return ghStatus{RateLimit: 60, RateRemaining: 42}
		})
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s (%s)", res.Status, res.Detail)
		}
		if !strings.Contains(res.Detail, "42/60") {
			t.Fatalf("expected detail to show headroom, got %q", res.Detail)
		}
	})

	t.Run("warns when rate-limit remaining is low", func(t *testing.T) {
		reset := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
		res := checkGitHub(func() ghStatus {
			return ghStatus{RateLimit: 60, RateRemaining: 3, RateReset: reset}
		})
		if res.Status != statusWARN {
			t.Fatalf("expected WARN, got %s (%s)", res.Status, res.Detail)
		}
		if !strings.Contains(res.Detail, "3/60") {
			t.Fatalf("expected detail to mention headroom, got %q", res.Detail)
		}
		if !strings.Contains(res.Detail, "GITHUB_TOKEN") {
			t.Fatalf("expected remediation hint, got %q", res.Detail)
		}
	})

	t.Run("warns on failure", func(t *testing.T) {
		res := checkGitHub(func() ghStatus { return ghStatus{Err: errors.New("timeout")} })
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

// stubPing is a reusable no-network pinger that reports OK.
func stubPing() ghStatus { return ghStatus{} }

// stubDeepOK is a reusable no-network deep checker that reports OK.
func stubDeepOK(_ string) checkResult {
	return checkResult{Status: statusOK, Name: "Deep checksums", Detail: "stub"}
}

func TestRunDoctor_Uninitialized(t *testing.T) {
	isolateEnv(t)
	t.Chdir(t.TempDir())

	var buf bytes.Buffer
	err := runDoctor(false, false, stubPing, stubDeepOK, &buf)
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
	err := runDoctor(false, false, stubPing, stubDeepOK, &buf)
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
	_ = runDoctor(false, false, stubPing, stubDeepOK, &preBuf)
	if !strings.Contains(preBuf.String(), "[WARN]") {
		t.Fatalf("expected WARN before fix, got:\n%s", preBuf.String())
	}

	// With --fix, drift is resolved and the binary is chmod'd.
	var buf bytes.Buffer
	if err := runDoctor(true, false, stubPing, stubDeepOK, &buf); err != nil {
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
		if err := Doctor(false, false); err == nil {
			t.Fatal("expected error from Doctor() with unset env")
		}
	})
	if !strings.Contains(out, "ISTIOENV_ROOT") {
		t.Fatalf("expected ISTIOENV_ROOT check in output, got:\n%s", out)
	}
}

// pingerExample is a simple compile-time sanity check that the pinger type
// matches the expected signature; this doubles as a stand-in test helper.
var _ pinger = func() ghStatus { return ghStatus{Err: fmt.Errorf("stub")} }

// deepCheckerExample is the equivalent compile-time check for deepChecker.
var _ deepChecker = func(string) checkResult { return checkResult{} }

// ---------------------------------------------------------------------------
// Deep (--deep) check tests
// ---------------------------------------------------------------------------

func TestRunDeepCheck(t *testing.T) {
	t.Run("ok when installed binary matches freshly downloaded archive", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		// Build a tar.gz containing a known istioctl payload, install the
		// same bytes on disk, and have the stub server return that archive.
		payload := []byte("installed istioctl binary content")
		archive := createTestTarGz(t, "istioctl", payload)
		archiveSum := sha256.Sum256(archive)
		archiveSumHex := hex.EncodeToString(archiveSum[:])

		installDir := filepath.Join(root, "versions", "1.24.0")
		if err := os.MkdirAll(installDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(installDir, "istioctl"), payload, 0o755); err != nil {
			t.Fatal(err)
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, ".sha256") {
				fmt.Fprint(w, archiveSumHex)
				return
			}
			_, _ = w.Write(archive)
		}))
		defer server.Close()

		client := &github.Client{
			BaseURL:         server.URL,
			DownloadBaseURL: server.URL,
			HTTPClient:      server.Client(),
		}

		res := runDeepCheck(client, root)
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s (%s)", res.Status, res.Detail)
		}
		if !strings.Contains(res.Detail, "1") {
			t.Fatalf("expected count in detail, got %q", res.Detail)
		}
	})

	t.Run("fails when installed binary differs from GitHub release", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}

		freshPayload := []byte("genuine istioctl content")
		tamperedPayload := []byte("TAMPERED CONTENT")
		archive := createTestTarGz(t, "istioctl", freshPayload)
		archiveSum := sha256.Sum256(archive)
		archiveSumHex := hex.EncodeToString(archiveSum[:])

		installDir := filepath.Join(root, "versions", "1.24.0")
		if err := os.MkdirAll(installDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(installDir, "istioctl"), tamperedPayload, 0o755); err != nil {
			t.Fatal(err)
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, ".sha256") {
				fmt.Fprint(w, archiveSumHex)
				return
			}
			_, _ = w.Write(archive)
		}))
		defer server.Close()

		client := &github.Client{
			BaseURL:         server.URL,
			DownloadBaseURL: server.URL,
			HTTPClient:      server.Client(),
		}

		res := runDeepCheck(client, root)
		if res.Status != statusFAIL {
			t.Fatalf("expected FAIL, got %s (%s)", res.Status, res.Detail)
		}
		if !strings.Contains(res.Detail, "1.24.0") {
			t.Fatalf("expected tampered version in detail, got %q", res.Detail)
		}
		if !strings.Contains(res.Detail, "reinstall") {
			t.Fatalf("expected remediation hint, got %q", res.Detail)
		}
	})

	t.Run("warns when GitHub is unreachable", func(t *testing.T) {
		root := t.TempDir()
		installVersion(t, root, "1.24.0", 0o755)

		// Server that always 404s mimics an unavailable release.
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()
		client := &github.Client{
			BaseURL:         server.URL,
			DownloadBaseURL: server.URL,
			HTTPClient:      server.Client(),
		}

		res := runDeepCheck(client, root)
		if res.Status != statusWARN {
			t.Fatalf("expected WARN, got %s (%s)", res.Status, res.Detail)
		}
	})

	t.Run("ok when no versions installed", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "versions"), 0o755); err != nil {
			t.Fatal(err)
		}
		client := github.NewClient() // never used because the version list is empty
		res := runDeepCheck(client, root)
		if res.Status != statusOK {
			t.Fatalf("expected OK, got %s (%s)", res.Status, res.Detail)
		}
	})
}

func TestRunDoctor_DeepFlagInvokesChecker(t *testing.T) {
	// Prove that --deep plumbs through to the injected deepChecker and that
	// its result appears in the output.
	isolateEnv(t)
	root := initTempRoot(t)
	t.Setenv("ISTIOENV_ROOT", root)
	installVersion(t, root, "1.24.0", 0o755)
	t.Setenv("ISTIOENV_VERSION", "1.24.0")
	t.Setenv("PATH", filepath.Join(root, "shims"))
	t.Chdir(t.TempDir())

	called := false
	deepStub := func(_ string) checkResult {
		called = true
		return checkResult{Status: statusOK, Name: "Deep checksums", Detail: "stub ran"}
	}

	var buf bytes.Buffer
	if err := runDoctor(false, true, stubPing, deepStub, &buf); err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, buf.String())
	}
	if !called {
		t.Fatalf("expected deep checker to be invoked when --deep is set")
	}
	if !strings.Contains(buf.String(), "stub ran") {
		t.Fatalf("expected deep check output, got:\n%s", buf.String())
	}
}

func TestRunDoctor_DeepSkippedWhenNoRoot(t *testing.T) {
	// --deep without ISTIOENV_ROOT must not panic and must not invoke the
	// deep checker (nothing to verify without a root).
	isolateEnv(t)
	t.Chdir(t.TempDir())

	called := false
	deepStub := func(_ string) checkResult {
		called = true
		return checkResult{Status: statusOK, Name: "Deep checksums"}
	}
	var buf bytes.Buffer
	_ = runDoctor(false, true, stubPing, deepStub, &buf)
	if called {
		t.Fatalf("deep check must be skipped when ISTIOENV_ROOT is not set")
	}
}

func TestParseRateLimitHeaders(t *testing.T) {
	t.Run("parses all headers", func(t *testing.T) {
		h := http.Header{}
		h.Set("X-RateLimit-Limit", "60")
		h.Set("X-RateLimit-Remaining", "7")
		h.Set("X-RateLimit-Reset", "1700000000")
		st := parseRateLimitHeaders(h)
		if st.RateLimit != 60 || st.RateRemaining != 7 {
			t.Fatalf("unexpected rate fields: %+v", st)
		}
		if st.RateReset.IsZero() {
			t.Fatalf("expected non-zero reset time")
		}
	})

	t.Run("tolerates missing headers", func(t *testing.T) {
		st := parseRateLimitHeaders(http.Header{})
		if st.RateLimit != 0 || st.RateRemaining != 0 || !st.RateReset.IsZero() {
			t.Fatalf("expected all zero, got %+v", st)
		}
	})

	t.Run("tolerates garbage header values", func(t *testing.T) {
		h := http.Header{}
		h.Set("X-RateLimit-Limit", "not-a-number")
		h.Set("X-RateLimit-Remaining", "")
		h.Set("X-RateLimit-Reset", "also-bad")
		st := parseRateLimitHeaders(h)
		if st.RateLimit != 0 || st.RateRemaining != 0 || !st.RateReset.IsZero() {
			t.Fatalf("expected graceful zeroing, got %+v", st)
		}
	})
}
