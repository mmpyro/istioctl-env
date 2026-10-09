package commands

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/user/istioctl-env/internal/config"
	"github.com/user/istioctl-env/internal/github"
	"github.com/user/istioctl-env/internal/platform"
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

// ghStatus is the richer GitHub probe result used by checkGitHub.  It
// carries both reachability information and (when the server returned
// them) rate-limit headers so we can warn the user before they hit the
// unauthenticated 60-req/hour cap.
type ghStatus struct {
	// Err is non-nil when the request could not complete (DNS, timeout,
	// 5xx).  Any non-5xx response — including 401/403/404 — is treated as
	// reachable with Err == nil.
	Err error

	// RateLimit and RateRemaining reflect X-RateLimit-Limit and
	// X-RateLimit-Remaining.  Zero means the headers were absent or
	// unparseable (which is normal for anonymous ping endpoints that
	// don't count against the limit).
	RateLimit     int
	RateRemaining int

	// RateReset is the UTC time X-RateLimit-Reset points at (zero when
	// absent).  Used by the warning message to tell the user when the
	// quota refreshes.
	RateReset time.Time
}

// pinger is a stubbable network-reachability probe used by the GitHub
// check.  Returning ghStatus (rather than a bare error) lets the probe
// report rate-limit headroom in addition to raw reachability.
type pinger func() ghStatus

// lowRateRemaining is the threshold below which checkGitHub degrades
// from OK → WARN even when GitHub is otherwise reachable.  Ten remaining
// requests is enough to run 'install' once but no more; prompting the
// user to switch to an authenticated client avoids surprise failures.
const lowRateRemaining = 10

// Doctor diagnoses the istioctl-env environment and prints a labeled
// OK/WARN/FAIL for each check. When fix is true, it attempts to repair
// the issues it can before re-running the checks.  When deep is true, it
// additionally re-downloads each installed version's archive from GitHub
// and byte-compares the result against the on-disk binary — slow and
// bandwidth-heavy, but catches silent corruption or tampering.
func Doctor(fix, deep bool) error {
	return runDoctor(fix, deep, defaultPinger, defaultDeepCheck, os.Stdout)
}

// DoctorHelp prints help for the doctor command.
func DoctorHelp() {
	fmt.Println(`Usage: istioctl-env doctor [flags]

Diagnoses the istioctl-env environment and prints an OK/WARN/FAIL for
each check with actionable guidance.

Flags:
  --fix          Attempt to repair issues (regenerate shim, chmod binaries)
  --deep         Re-verify every installed binary against its GitHub
                 checksum (downloads each archive again; slow)
  -h, --help     Show this help message

Exit codes:
  0  all checks passed (no FAIL)
  1  at least one check reported FAIL`)
}

// deepChecker is a stubbable per-version checksum re-verifier.  Separating
// it from the main run makes tests fast (they inject a no-network stub)
// and keeps the expensive GitHub traffic opt-in via --deep.
type deepChecker func(root string) checkResult

// runDoctor is the testable core.  All I/O (stdout, network) is injected
// so tests can exercise it deterministically.
func runDoctor(fix, deep bool, ping pinger, deepFn deepChecker, out io.Writer) error {
	if ping == nil {
		ping = defaultPinger
	}
	if deepFn == nil {
		deepFn = defaultDeepCheck
	}

	results := gatherChecks(ping, deep, deepFn)

	if fix {
		applyFixes(results)
		results = gatherChecks(ping, deep, deepFn)
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
//
// When deep is true, deepFn is invoked at the end to re-verify every
// installed binary against its GitHub checksum.
func gatherChecks(ping pinger, deep bool, deepFn deepChecker) []checkResult {
	results := make([]checkResult, 0, 13)

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
		// 9b. Version directories that contain no istioctl binary at all.
		results = append(results, checkDanglingVersions(root))
	}

	// 10. GitHub reachable.
	results = append(results, checkGitHub(ping))

	// 11. Cache file parses as valid JSON (if present).
	if haveRoot {
		results = append(results, checkCache(root))
	}

	// 12. Optional deep checksum re-verification (--deep).  Only runs when
	// we have a root to walk; otherwise there is nothing to verify.
	if deep && haveRoot {
		if deepFn == nil {
			deepFn = defaultDeepCheck
		}
		results = append(results, deepFn(root))
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

// 7. Active version resolves via getActiveVersionWithSource.  Reusing the
// same helper as 'istioctl-env status' guarantees the two commands always
// agree on which precedence tier won.
func checkResolvedVersion() (checkResult, string) {
	v, src := getActiveVersionWithSource()
	if v == "" {
		return checkResult{
			Status: statusFAIL,
			Name:   "Active version",
			Detail: "no version configured; set one with 'istioctl-env global <v>'",
		}, ""
	}
	return checkResult{
		Status: statusOK,
		Name:   "Active version",
		Detail: fmt.Sprintf("%s (set by %s)", v, src),
	}, v
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

// 9. For every installed version, the istioctl binary exists as a regular
// file and is executable.  Directories missing the binary entirely are
// reported separately by checkDanglingVersions.
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
	var nonExec []string
	healthy := 0
	for _, v := range versions {
		bin := filepath.Join(versionsDir, v, "istioctl")
		info, err := os.Stat(bin)
		if err != nil || !info.Mode().IsRegular() {
			// Reported by checkDanglingVersions.
			continue
		}
		if info.Mode().Perm()&0o111 == 0 {
			nonExec = append(nonExec, v)
			continue
		}
		healthy++
	}
	if len(nonExec) > 0 {
		return checkResult{
			Status: statusWARN,
			Name:   "Installed binaries",
			Detail: fmt.Sprintf("non-executable: %s; run 'istioctl-env doctor --fix'", strings.Join(nonExec, ", ")),
		}
	}
	return checkResult{
		Status: statusOK,
		Name:   "Installed binaries",
		Detail: fmt.Sprintf("%d installed", healthy),
	}
}

// checkDanglingVersions reports version directories under $ISTIOENV_ROOT/versions/
// that have no istioctl binary inside — typically a half-finished install or
// a manually created directory.  The recommended fix is to either reinstall
// the version or remove it with 'istioctl-env uninstall <version>'.
func checkDanglingVersions(root string) checkResult {
	versionsDir := filepath.Join(root, "versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return checkResult{
			Status: statusWARN,
			Name:   "Dangling versions",
			Detail: fmt.Sprintf("cannot list %s: %v", versionsDir, err),
		}
	}
	var dangling []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		bin := filepath.Join(versionsDir, e.Name(), "istioctl")
		info, statErr := os.Stat(bin)
		if statErr != nil || !info.Mode().IsRegular() {
			dangling = append(dangling, e.Name())
		}
	}
	if len(dangling) == 0 {
		return checkResult{
			Status: statusOK,
			Name:   "Dangling versions",
			Detail: "none",
		}
	}
	dangling = semver.SortDescending(dangling)
	return checkResult{
		Status: statusWARN,
		Name:   "Dangling versions",
		Detail: fmt.Sprintf("no istioctl binary in: %s; reinstall or run 'istioctl-env uninstall <version>'", strings.Join(dangling, ", ")),
	}
}

// 10. GitHub reachable via the injected pinger, with rate-limit headroom
// surfaced when the server returned X-RateLimit-* headers.
func checkGitHub(ping pinger) checkResult {
	if ping == nil {
		ping = defaultPinger
	}
	st := ping()
	if st.Err != nil {
		return checkResult{
			Status: statusWARN,
			Name:   "GitHub",
			Detail: fmt.Sprintf("unreachable: %v", st.Err),
		}
	}

	// No headers → treat as plain reachability.  Many /ping-style endpoints
	// (and most proxies / local test servers) don't advertise rate limits.
	if st.RateLimit == 0 && st.RateRemaining == 0 {
		return checkResult{
			Status: statusOK,
			Name:   "GitHub",
			Detail: "api.github.com reachable",
		}
	}

	if st.RateRemaining < lowRateRemaining {
		detail := fmt.Sprintf(
			"low rate-limit headroom: %d/%d remaining",
			st.RateRemaining, st.RateLimit,
		)
		if !st.RateReset.IsZero() {
			detail += fmt.Sprintf("; resets at %s", st.RateReset.Local().Format(time.RFC3339))
		}
		detail += "; set GITHUB_TOKEN or wait before retrying"
		return checkResult{
			Status: statusWARN,
			Name:   "GitHub",
			Detail: detail,
		}
	}
	return checkResult{
		Status: statusOK,
		Name:   "GitHub",
		Detail: fmt.Sprintf("api.github.com reachable (%d/%d requests remaining)", st.RateRemaining, st.RateLimit),
	}
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
// still prove the host is up).  When present, X-RateLimit-* headers are
// parsed so the caller can warn about low headroom.
//
// If GITHUB_TOKEN is set in the environment it is sent as a Bearer token;
// GitHub counts authenticated requests against a much larger (5000/hour)
// bucket, so including the token here means the probe reports the limit
// the user's real workflow will actually see.
func defaultPinger() ghStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/rate_limit", nil)
	if err != nil {
		return ghStatus{Err: err}
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "istioctl-env-doctor")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ghStatus{Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return ghStatus{Err: fmt.Errorf("github returned HTTP %d", resp.StatusCode)}
	}
	return parseRateLimitHeaders(resp.Header)
}

// errChecksumMismatch is returned by verifyInstalledBinary when the
// installed istioctl binary does not match the bytes extracted from the
// freshly-downloaded archive.  It is used as a sentinel so checkDeep
// can report mismatches as FAIL and other errors (network, extract) as
// WARN.
var errChecksumMismatch = errors.New("installed binary does not match GitHub release")

// defaultDeepCheck is the production implementation of deepChecker.  It
// constructs a real GitHub client; tests inject a stub instead.
func defaultDeepCheck(root string) checkResult {
	return runDeepCheck(github.NewClient(), root)
}

// runDeepCheck iterates every installed version, re-downloads its archive
// from GitHub, verifies the archive against its published .sha256, then
// byte-compares the extracted istioctl against the on-disk binary.  Any
// mismatch is a FAIL (the local binary is wrong); any other failure is a
// WARN (network/extract problems are not strong evidence of corruption).
func runDeepCheck(client *github.Client, root string) checkResult {
	info, err := platform.Detect()
	if err != nil {
		return checkResult{
			Status: statusWARN,
			Name:   "Deep checksums",
			Detail: fmt.Sprintf("platform detect: %v", err),
		}
	}

	versionsDir := filepath.Join(root, "versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return checkResult{
			Status: statusWARN,
			Name:   "Deep checksums",
			Detail: fmt.Sprintf("cannot list %s: %v", versionsDir, err),
		}
	}

	var versions []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		bin := filepath.Join(versionsDir, e.Name(), "istioctl")
		if st, statErr := os.Stat(bin); statErr == nil && st.Mode().IsRegular() {
			versions = append(versions, e.Name())
		}
	}
	if len(versions) == 0 {
		return checkResult{
			Status: statusOK,
			Name:   "Deep checksums",
			Detail: "no installed binaries to verify",
		}
	}

	versions = semver.SortDescending(versions)
	var mismatched, failed []string
	for _, v := range versions {
		if err := verifyInstalledBinary(client, info, root, v); err != nil {
			if errors.Is(err, errChecksumMismatch) {
				mismatched = append(mismatched, v)
			} else {
				failed = append(failed, fmt.Sprintf("%s (%v)", v, err))
			}
		}
	}

	if len(mismatched) > 0 {
		detail := fmt.Sprintf(
			"checksum mismatch: %s; reinstall with 'istioctl-env install <version>'",
			strings.Join(mismatched, ", "),
		)
		if len(failed) > 0 {
			detail += "; also failed to verify: " + strings.Join(failed, ", ")
		}
		return checkResult{
			Status: statusFAIL,
			Name:   "Deep checksums",
			Detail: detail,
		}
	}
	if len(failed) > 0 {
		return checkResult{
			Status: statusWARN,
			Name:   "Deep checksums",
			Detail: fmt.Sprintf("could not verify: %s", strings.Join(failed, ", ")),
		}
	}
	return checkResult{
		Status: statusOK,
		Name:   "Deep checksums",
		Detail: fmt.Sprintf("%d binaries verified against GitHub", len(versions)),
	}
}

// verifyInstalledBinary downloads the archive + .sha256 for one version,
// confirms the archive matches its published checksum, extracts the
// istioctl entry, and byte-compares the result against the on-disk
// binary.  Returns errChecksumMismatch when the local binary differs.
func verifyInstalledBinary(client *github.Client, info platform.Info, root, version string) error {
	archiveURL := client.DownloadURL(platform.DownloadPath(version, info))
	checksumURL := client.DownloadURL(platform.ChecksumPath(version, info))

	archiveData, err := client.DownloadBinary(archiveURL)
	if err != nil {
		return fmt.Errorf("download archive: %w", err)
	}
	checksumData, err := client.DownloadBinary(checksumURL)
	if err != nil {
		return fmt.Errorf("download checksum: %w", err)
	}

	expected := strings.TrimSpace(string(checksumData))
	if parts := strings.Fields(expected); len(parts) > 0 {
		expected = parts[0]
	}
	actualSum := sha256.Sum256(archiveData)
	if hex.EncodeToString(actualSum[:]) != expected {
		// GitHub-side mismatch: the published .sha256 doesn't match the
		// archive they're serving.  Not a local problem — report it so the
		// user knows not to trust the mismatch verdict either way.
		return fmt.Errorf("GitHub archive sha256 mismatch")
	}

	freshBinary, err := extractFromTarGz(archiveData, "istioctl")
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	binaryPath := filepath.Join(root, "versions", version, "istioctl")
	installed, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("read installed binary: %w", err)
	}

	if sha256.Sum256(freshBinary) != sha256.Sum256(installed) {
		return errChecksumMismatch
	}
	return nil
}

// parseRateLimitHeaders reads X-RateLimit-Limit, X-RateLimit-Remaining and
// X-RateLimit-Reset from a GitHub API response.  Missing or malformed
// headers result in zero-valued fields rather than an error — the caller
// treats zeros as "no rate-limit information available".
func parseRateLimitHeaders(h http.Header) ghStatus {
	var st ghStatus
	if raw := h.Get("X-RateLimit-Limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			st.RateLimit = n
		}
	}
	if raw := h.Get("X-RateLimit-Remaining"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			st.RateRemaining = n
		}
	}
	if raw := h.Get("X-RateLimit-Reset"); raw != "" {
		if sec, err := strconv.ParseInt(raw, 10, 64); err == nil {
			st.RateReset = time.Unix(sec, 0).UTC()
		}
	}
	return st
}
