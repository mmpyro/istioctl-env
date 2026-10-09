package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/user/istioctl-env/internal/config"
	"github.com/user/istioctl-env/internal/semver"
	"github.com/user/istioctl-env/internal/shim"
)

// Status values used by the doctor command.
const (
	statusOK   = "OK"
	statusWARN = "WARN"
	statusFAIL = "FAIL"
	statusINFO = "INFO"
)

// checkResult is the outcome of a single doctor check.
type checkResult struct {
	Status string
	Name   string
	Detail string
}

// pinger is a stubbable network-reachability probe used by the GitHub check.
type pinger func() error

// Doctor diagnoses the istioctl-env environment and prints a labeled
// OK/WARN/FAIL for each check. When fix is true, it attempts to repair
// the issues it can before re-running the checks.
func Doctor(fix bool) error {
	return runDoctor(fix, defaultPinger, os.Stdout)
}

// DoctorHelp prints help for the doctor command.
func DoctorHelp() {
	fmt.Println(`Usage: istioctl-env doctor [flags]

Diagnoses the istioctl-env environment and prints an OK/WARN/FAIL for
each check with actionable guidance.

Flags:
  --fix          Attempt to repair issues (regenerate shim, chmod binaries)
  -h, --help     Show this help message

Exit codes:
  0  all checks passed (no FAIL)
  1  at least one check reported FAIL`)
}

// runDoctor is the testable core.  All I/O (stdout, network) is injected
// so tests can exercise it deterministically.
func runDoctor(fix bool, ping pinger, out io.Writer) error {
	if ping == nil {
		ping = defaultPinger
	}

	results := gatherChecks(ping)

	if fix {
		applyFixes(results)
		results = gatherChecks(ping)
	}

	printResults(out, results)

	for _, r := range results {
		if r.Status == statusFAIL {
			return errors.New("doctor: one or more checks failed")
		}
	}
	return nil
}

// gatherChecks runs every doctor check in the documented order and
// returns the aggregated results.  Checks that cannot run because an
// earlier prerequisite failed are skipped (not reported as FAIL).
func gatherChecks(ping pinger) []checkResult {
	results := make([]checkResult, 0, 11)

	// 1. ISTIOENV_ROOT set.
	envRes := checkEnvRoot()
	results = append(results, envRes)
	root, haveRoot := config.GetIstioEnvRoot()

	if haveRoot {
		// 2. Root exists + writable.
		results = append(results, checkRootWritable(root))
		// 3. versions/ directory exists.
		results = append(results, checkVersionsDir(root))
		// 4. Shim exists and matches generator output.
		results = append(results, checkShimDrift(root))
		// 5. PATH contains the shims directory first.
		results = append(results, checkPATH(root))
	}

	// 6. Shell integration eval'd (best-effort rc-file scan).
	results = append(results, checkShellIntegration(os.Getenv("HOME")))

	// 7. Active version resolves.
	verRes, resolvedVersion := checkResolvedVersion()
	results = append(results, verRes)

	// 8. Active binary exists + executable.
	if haveRoot && resolvedVersion != "" {
		results = append(results, checkActiveBinary(root, resolvedVersion))
	}

	// 9. All installed version binaries present + executable.
	if haveRoot {
		results = append(results, checkInstalledBinaries(root))
	}

	// 10. GitHub reachable.
	results = append(results, checkGitHub(ping))

	// 11. Cache file parses as valid JSON (if present).
	if haveRoot {
		results = append(results, checkCache(root))
	}

	return results
}

// applyFixes performs the --fix side effects: regenerate shim, chmod any
// non-executable binaries.  Does NOT modify PATH or shell rc files.
func applyFixes(results []checkResult) {
	root, haveRoot := config.GetIstioEnvRoot()
	if !haveRoot {
		return
	}

	// Always regenerate the shim when --fix is used; the user asked for
	// repairs and this is cheap and idempotent.
	_ = shim.GenerateShimScript(root)

	// Chmod active and installed version binaries that are not executable.
	versionsDir := filepath.Join(root, "versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		binPath := filepath.Join(versionsDir, entry.Name(), "istioctl")
		info, err := os.Stat(binPath)
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if info.Mode().Perm()&0o111 == 0 {
			_ = os.Chmod(binPath, 0o755)
		}
	}
}

// printResults writes the results via tabwriter, one row per check.
func printResults(w io.Writer, results []checkResult) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	for _, r := range results {
		fmt.Fprintf(tw, "[%s]\t%s\t%s\n", r.Status, r.Name, r.Detail)
	}
	_ = tw.Flush()
}

// ---------------------------------------------------------------------------
// Individual checks
// ---------------------------------------------------------------------------

// 1. ISTIOENV_ROOT set.
func checkEnvRoot() checkResult {
	root, ok := config.GetIstioEnvRoot()
	if !ok {
		return checkResult{
			Status: statusFAIL,
			Name:   "ISTIOENV_ROOT",
			Detail: "not set; export ISTIOENV_ROOT=\"$HOME/.istioenv\"",
		}
	}
	return checkResult{Status: statusOK, Name: "ISTIOENV_ROOT", Detail: root}
}

// 2. ISTIOENV_ROOT exists and is writable.
func checkRootWritable(root string) checkResult {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return checkResult{
				Status: statusFAIL,
				Name:   "Root directory",
				Detail: fmt.Sprintf("%s does not exist; run 'istioctl-env init'", root),
			}
		}
		return checkResult{
			Status: statusFAIL,
			Name:   "Root directory",
			Detail: fmt.Sprintf("stat %s: %v", root, err),
		}
	}
	if !info.IsDir() {
		return checkResult{
			Status: statusFAIL,
			Name:   "Root directory",
			Detail: fmt.Sprintf("%s is not a directory", root),
		}
	}
	probe, err := os.CreateTemp(root, ".istioenv-doctor-probe-*")
	if err != nil {
		return checkResult{
			Status: statusFAIL,
			Name:   "Root writable",
			Detail: fmt.Sprintf("cannot create files in %s: %v", root, err),
		}
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return checkResult{Status: statusOK, Name: "Root writable", Detail: root}
}

// 3. $ISTIOENV_ROOT/versions directory exists.
func checkVersionsDir(root string) checkResult {
	versionsDir := filepath.Join(root, "versions")
	info, err := os.Stat(versionsDir)
	if err != nil {
		return checkResult{
			Status: statusFAIL,
			Name:   "Versions directory",
			Detail: fmt.Sprintf("%s missing; run 'istioctl-env init'", versionsDir),
		}
	}
	if !info.IsDir() {
		return checkResult{
			Status: statusFAIL,
			Name:   "Versions directory",
			Detail: fmt.Sprintf("%s is not a directory", versionsDir),
		}
	}
	return checkResult{Status: statusOK, Name: "Versions directory", Detail: versionsDir}
}

// 4. Shim exists and byte-matches shim.GenerateShimScript output.
func checkShimDrift(root string) checkResult {
	shimPath := filepath.Join(root, "shims", "istioctl")
	actual, err := os.ReadFile(shimPath)
	if err != nil {
		return checkResult{
			Status: statusWARN,
			Name:   "Shim script",
			Detail: fmt.Sprintf("%s missing; regenerate with 'istioctl-env doctor --fix'", shimPath),
		}
	}
	expected, err := expectedShimContent(root)
	if err != nil {
		return checkResult{
			Status: statusWARN,
			Name:   "Shim script",
			Detail: fmt.Sprintf("cannot determine expected shim content: %v", err),
		}
	}
	if !bytes.Equal(actual, expected) {
		return checkResult{
			Status: statusWARN,
			Name:   "Shim drift",
			Detail: "shim differs from generator output; regenerate with 'istioctl-env doctor --fix'",
		}
	}
	return checkResult{Status: statusOK, Name: "Shim script", Detail: shimPath}
}

// expectedShimContent returns the byte content that shim.GenerateShimScript
// would produce for the given root, without touching the real shims directory.
// It writes the shim into a temp directory, reads it, and substitutes the
// temp path back to the real root (the root is the only input the shim
// template interpolates).
func expectedShimContent(root string) ([]byte, error) {
	tmp, err := os.MkdirTemp("", "istioenv-doctor-shim-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := shim.GenerateShimScript(tmp); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(tmp, "shims", "istioctl"))
	if err != nil {
		return nil, err
	}
	return bytes.ReplaceAll(data, []byte(tmp), []byte(root)), nil
}

// 5. $ISTIOENV_ROOT/shims is on PATH, ahead of other istioctl entries.
func checkPATH(root string) checkResult {
	shimsDir := filepath.Join(root, "shims")
	path := os.Getenv("PATH")
	entries := filepath.SplitList(path)
	shimsIdx := -1
	firstIstioIdx := -1
	for i, dir := range entries {
		if dir == "" {
			continue
		}
		if sameDir(dir, shimsDir) && shimsIdx == -1 {
			shimsIdx = i
		}
		if firstIstioIdx == -1 && hasIstioctl(dir) {
			firstIstioIdx = i
		}
	}
	if shimsIdx == -1 {
		return checkResult{
			Status: statusFAIL,
			Name:   "PATH",
			Detail: fmt.Sprintf("%s not on PATH; add 'eval \"$(istioctl-env init)\"' to your shell rc", shimsDir),
		}
	}
	if firstIstioIdx != -1 && firstIstioIdx < shimsIdx {
		return checkResult{
			Status: statusWARN,
			Name:   "PATH",
			Detail: fmt.Sprintf("%s is on PATH but %s precedes it", shimsDir, entries[firstIstioIdx]),
		}
	}
	return checkResult{Status: statusOK, Name: "PATH", Detail: shimsDir}
}

// sameDir compares two directory strings, tolerating trailing slashes and
// non-canonical paths.
func sameDir(a, b string) bool {
	ca, erra := filepath.Abs(filepath.Clean(a))
	cb, errb := filepath.Abs(filepath.Clean(b))
	if erra != nil || errb != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ca == cb
}

// hasIstioctl reports whether an executable named "istioctl" exists in dir.
func hasIstioctl(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "istioctl"))
	if err != nil {
		return false
	}
	if !info.Mode().IsRegular() {
		return false
	}
	// On Windows the executable bit is not meaningful; everywhere else we
	// require it.
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}

// 6. Shell integration: INFO if no rc file references 'istioctl-env init'.
func checkShellIntegration(home string) checkResult {
	if home == "" {
		return checkResult{
			Status: statusINFO,
			Name:   "Shell integration",
			Detail: "$HOME not set; cannot scan rc files",
		}
	}
	candidates := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".config", "fish", "config.fish"),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if bytes.Contains(data, []byte("istioctl-env init")) {
			return checkResult{
				Status: statusOK,
				Name:   "Shell integration",
				Detail: fmt.Sprintf("found 'istioctl-env init' in %s", path),
			}
		}
	}
	return checkResult{
		Status: statusINFO,
		Name:   "Shell integration",
		Detail: "no 'istioctl-env init' line found in ~/.bashrc, ~/.zshrc or fish config; add: eval \"$(istioctl-env init)\"",
	}
}

// 7. Active version resolves via config.ResolveVersion.
func checkResolvedVersion() (checkResult, string) {
	v, err := config.ResolveVersion()
	if err != nil {
		return checkResult{
			Status: statusFAIL,
			Name:   "Active version",
			Detail: "no version configured; set one with 'istioctl-env global <v>'",
		}, ""
	}
	return checkResult{Status: statusOK, Name: "Active version", Detail: v}, v
}

// 8. Active version binary exists, is a regular file, and is executable.
func checkActiveBinary(root, version string) checkResult {
	binaryPath := filepath.Join(root, "versions", version, "istioctl")
	info, err := os.Stat(binaryPath)
	if err != nil {
		return checkResult{
			Status: statusFAIL,
			Name:   "Active binary",
			Detail: fmt.Sprintf("%s missing; run 'istioctl-env install %s'", binaryPath, version),
		}
	}
	if !info.Mode().IsRegular() {
		return checkResult{
			Status: statusFAIL,
			Name:   "Active binary",
			Detail: fmt.Sprintf("%s is not a regular file", binaryPath),
		}
	}
	if info.Mode().Perm()&0o111 == 0 {
		return checkResult{
			Status: statusFAIL,
			Name:   "Active binary",
			Detail: fmt.Sprintf("%s is not executable; run 'istioctl-env doctor --fix'", binaryPath),
		}
	}
	return checkResult{Status: statusOK, Name: "Active binary", Detail: binaryPath}
}

// 9. For every installed version, binary exists and is executable.
func checkInstalledBinaries(root string) checkResult {
	versionsDir := filepath.Join(root, "versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return checkResult{
			Status: statusWARN,
			Name:   "Installed binaries",
			Detail: fmt.Sprintf("cannot list %s: %v", versionsDir, err),
		}
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	if len(versions) == 0 {
		return checkResult{
			Status: statusOK,
			Name:   "Installed binaries",
			Detail: "no versions installed",
		}
	}
	versions = semver.SortDescending(versions)
	var bad []string
	for _, v := range versions {
		bin := filepath.Join(versionsDir, v, "istioctl")
		info, err := os.Stat(bin)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			bad = append(bad, v)
		}
	}
	if len(bad) > 0 {
		return checkResult{
			Status: statusWARN,
			Name:   "Installed binaries",
			Detail: fmt.Sprintf("missing or non-executable: %s", strings.Join(bad, ", ")),
		}
	}
	return checkResult{
		Status: statusOK,
		Name:   "Installed binaries",
		Detail: fmt.Sprintf("%d installed", len(versions)),
	}
}

// 10. GitHub reachable via the injected pinger.
func checkGitHub(ping pinger) checkResult {
	if ping == nil {
		ping = defaultPinger
	}
	if err := ping(); err != nil {
		return checkResult{
			Status: statusWARN,
			Name:   "GitHub",
			Detail: fmt.Sprintf("unreachable: %v", err),
		}
	}
	return checkResult{Status: statusOK, Name: "GitHub", Detail: "api.github.com reachable"}
}

// 11. Cache file parses as valid JSON if present.
func checkCache(root string) checkResult {
	cachePath := filepath.Join(root, "cache", "releases.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return checkResult{
				Status: statusOK,
				Name:   "Release cache",
				Detail: "no cache file yet",
			}
		}
		return checkResult{
			Status: statusWARN,
			Name:   "Release cache",
			Detail: fmt.Sprintf("cannot read %s: %v", cachePath, err),
		}
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return checkResult{
			Status: statusWARN,
			Name:   "Release cache",
			Detail: fmt.Sprintf("%s is not valid JSON: %v", cachePath, err),
		}
	}
	return checkResult{Status: statusOK, Name: "Release cache", Detail: cachePath}
}

// defaultPinger performs a GET against api.github.com with a 3-second
// timeout.  Any non-5xx response is considered reachable (401, 403 or 404
// still prove the host is up).
func defaultPinger() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("github returned HTTP %d", resp.StatusCode)
	}
	return nil
}
