package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/user/istioctl-env/internal/config"
	"github.com/user/istioctl-env/internal/semver"
)

// PruneScanRootsEnv is the name of the environment variable that overrides
// the roots scanned for `.istioctl-version` files when computing the set of
// referenced versions.  Multiple roots are separated by the OS path list
// separator (":" on Unix, ";" on Windows).  When unset, Prune defaults to
// scanning the user's home directory.
const PruneScanRootsEnv = "ISTIOENV_PRUNE_SCAN_ROOTS"

// scanSkipDirs is the set of directory base names skipped during the
// `.istioctl-version` discovery walk.  These are directories we know never
// contain project version files but can be very large (worst-case tens of
// thousands of files), so skipping them keeps the scan fast on typical
// developer home directories.
var scanSkipDirs = map[string]bool{
	".git":         true,
	".hg":          true,
	".svn":         true,
	"node_modules": true,
	"vendor":       true,
	".cache":       true,
	".gradle":      true,
	".m2":          true,
	".venv":        true,
	"venv":         true,
	"__pycache__":  true,
	".terraform":   true,
	".idea":        true,
	".vscode":      true,
	".tox":         true,
	"dist":         true,
	"target":       true,
	"build":        true,
	"Library":      true, // macOS: ~/Library is huge and private
}

// PruneOptions bundles the flags for Prune.  It is a struct (rather than a
// long positional argument list) so callers at the CLI layer can set fields
// explicitly without worrying about argument order and so future flags can
// be added without churning every call site.
type PruneOptions struct {
	// KeepLast keeps the N newest installed versions regardless of whether
	// they are referenced.  Must be ≥ 0.  Zero means "no additional versions
	// are kept by this rule".
	KeepLast int

	// OlderThan narrows the removal set to only versions whose directory
	// mtime is older than now()-OlderThan.  Only applied when
	// OlderThanProvided is true.
	OlderThan         time.Duration
	OlderThanProvided bool

	// ScanRoots is the list of filesystem roots walked for `.istioctl-version`
	// files when computing the set of referenced versions.  When empty, Prune
	// consults the ISTIOENV_PRUNE_SCAN_ROOTS environment variable, and
	// otherwise falls back to the user's home directory.
	ScanRoots []string

	// DryRun forces dry-run mode even if Yes is true.  This matches the
	// documented precedence: when both flags are supplied, DryRun wins
	// (safer default).
	DryRun bool

	// Yes opts into actually removing version directories.  When false (and
	// DryRun is false), Prune still only prints what would be removed —
	// mirroring the CLI contract where `prune` is non-destructive unless the
	// user explicitly passes --yes.
	Yes bool
}

// pruneCandidate pairs a version string with the resolved on-disk size of
// its directory so the summary can report accurate "freed" totals.
type pruneCandidate struct {
	name string
	size int64
}

// Prune removes installed istioctl versions that are not referenced by any
// configured source (global version, .istioctl-version files under
// ISTIOENV_PRUNE_SCAN_ROOTS, or the exported ISTIOENV_VERSION), honouring
// the --keep-last and --older-than guards.
//
// Behaviour summary:
//
//   - A version is a candidate for removal iff it is INSTALLED, NOT
//     referenced, and NOT in the top-KeepLast newest installed versions.
//   - When OlderThanProvided is true the candidate set is further filtered
//     to only versions whose directory mtime is older than now()-OlderThan.
//   - If DryRun is true OR Yes is false, nothing is written to disk — Prune
//     prints "would remove ..." lines and a dry-run summary.
//   - On actual removal, Prune prints "removed ..." lines and a summary of
//     bytes freed.
//
// Prune always prints a one-line scan summary so users can see how much
// filesystem work the reference discovery did.
//
// Returns a non-nil error only on filesystem failures that make the operation
// unsafe to continue (e.g. ISTIOENV_ROOT missing, failing to stat/read the
// versions directory).  Partial scan failures (unreadable directories inside
// a scan root) are swallowed and reported in the scan summary.
func Prune(opts PruneOptions) error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	root, _ := config.GetIstioEnvRoot()
	versionsDir := filepath.Join(root, "versions")

	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return fmt.Errorf("failed to read versions directory: %w", err)
	}

	var installed []string
	for _, entry := range entries {
		if entry.IsDir() {
			installed = append(installed, entry.Name())
		}
	}
	sorted := semver.SortDescending(installed)

	// ── Build the set of referenced versions ──────────────────────────────

	referenced := make(map[string]string) // version → human-readable source

	// 1. Global version (if any).  Errors are tolerated: a missing global
	//    version file simply means nothing is referenced from here.
	if gv, gerr := config.ReadGlobalVersion(root); gerr == nil && gv != "" {
		referenced[gv] = "global"
	}

	// 2. ISTIOENV_VERSION — the currently exported shell version.
	if sv := strings.TrimSpace(os.Getenv("ISTIOENV_VERSION")); sv != "" {
		if _, already := referenced[sv]; !already {
			referenced[sv] = "ISTIOENV_VERSION"
		}
	}

	// 3. `.istioctl-version` files under the configured scan roots.
	scanRoots := resolveScanRoots(opts.ScanRoots)
	scanned, found, scanDur, scanErrs := scanLocalVersions(scanRoots, referenced)

	// ── Scan summary — ALWAYS printed, even on no-op prunes ───────────────

	switch len(scanRoots) {
	case 0:
		fmt.Printf("scanned 0 roots in %s — no .istioctl-version files considered\n", scanDur.Round(time.Millisecond))
	case 1:
		fmt.Printf("scanned %d files in %s under %s — found %d .istioctl-version file(s)\n",
			scanned, scanDur.Round(time.Millisecond), scanRoots[0], found)
	default:
		fmt.Printf("scanned %d files in %s under %d roots — found %d .istioctl-version file(s)\n",
			scanned, scanDur.Round(time.Millisecond), len(scanRoots), found)
	}
	if len(scanErrs) > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d path(s) could not be scanned (first: %v)\n", len(scanErrs), scanErrs[0])
	}

	// ── Build the keep-last-N set ─────────────────────────────────────────

	keepLast := opts.KeepLast
	if keepLast < 0 {
		keepLast = 0
	}
	if keepLast > len(sorted) {
		keepLast = len(sorted)
	}
	keepByLast := make(map[string]bool, keepLast)
	for i := 0; i < keepLast; i++ {
		keepByLast[sorted[i]] = true
	}

	// ── Select candidates ─────────────────────────────────────────────────

	var toRemove []pruneCandidate
	now := time.Now()

	for _, v := range sorted {
		if _, isReferenced := referenced[v]; isReferenced {
			continue
		}
		if keepByLast[v] {
			continue
		}

		versionDir := filepath.Join(versionsDir, v)

		if opts.OlderThanProvided {
			info, statErr := os.Stat(versionDir)
			if statErr != nil {
				if errors.Is(statErr, fs.ErrNotExist) {
					continue
				}
				return fmt.Errorf("failed to stat %s: %w", versionDir, statErr)
			}
			if now.Sub(info.ModTime()) <= opts.OlderThan {
				continue
			}
		}

		size, sizeErr := dirSize(versionDir)
		if sizeErr != nil {
			return fmt.Errorf("failed to measure %s: %w", versionDir, sizeErr)
		}
		toRemove = append(toRemove, pruneCandidate{name: v, size: size})
	}

	// ── Report / act ──────────────────────────────────────────────────────

	destructive := opts.Yes && !opts.DryRun

	if len(toRemove) == 0 {
		fmt.Println("nothing to prune")
		if !destructive {
			printKeepList(referenced, keepByLast)
		}
		return nil
	}

	var freed int64
	for _, c := range toRemove {
		versionDir := filepath.Join(versionsDir, c.name)
		if !destructive {
			fmt.Printf("would remove %s (%s)\n", c.name, humanize(c.size))
			freed += c.size
			continue
		}
		if err := os.RemoveAll(versionDir); err != nil {
			return fmt.Errorf("failed to remove %s: %w", c.name, err)
		}
		fmt.Printf("removed %s (freed %s)\n", c.name, humanize(c.size))
		freed += c.size
	}

	if destructive {
		fmt.Printf("pruned %d version(s), freed %s\n", len(toRemove), humanize(freed))
	} else {
		fmt.Printf("would prune %d version(s), freeing %s — pass --yes to apply\n", len(toRemove), humanize(freed))
		printKeepList(referenced, keepByLast)
	}
	return nil
}

// resolveScanRoots returns the roots to walk for .istioctl-version files,
// consulting (in order): the explicit opts.ScanRoots list, the
// ISTIOENV_PRUNE_SCAN_ROOTS environment variable, and finally the user's
// home directory.  Empty entries are dropped.  Returned paths are cleaned
// but NOT required to exist; non-existent roots are quietly skipped during
// the walk.
//
// Setting ISTIOENV_PRUNE_SCAN_ROOTS to an empty string explicitly disables
// filesystem scanning (useful on servers / CI runners where no human-authored
// .istioctl-version files exist and $HOME can be huge).  Unset falls through
// to the home-directory default.
func resolveScanRoots(explicit []string) []string {
	if len(explicit) > 0 {
		return cleanRoots(explicit)
	}
	if env, isSet := os.LookupEnv(PruneScanRootsEnv); isSet {
		return cleanRoots(strings.Split(env, string(os.PathListSeparator)))
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	return []string{filepath.Clean(home)}
}

func cleanRoots(roots []string) []string {
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		out = append(out, filepath.Clean(r))
	}
	return out
}

// scanLocalVersions walks each root looking for files named
// ".istioctl-version".  For each such file it reads a non-empty version
// string (if present) and adds it to referenced with source=path.  The scan
// skips well-known noisy directories (see scanSkipDirs) and never follows
// symbolic links.  Unreadable paths are collected in errsOut rather than
// aborting the whole prune.
//
// Returns:
//   - filesVisited: total number of files (not directories) encountered;
//     used by the scan summary so users can gauge how much work was done.
//   - matches: number of .istioctl-version files that contained a non-empty
//     version string and were therefore added to referenced.
//   - elapsed: wall-clock duration of the scan.
//   - errsOut: non-fatal errors (permission denied, broken symlinks, …).
func scanLocalVersions(roots []string, referenced map[string]string) (filesVisited int, matches int, elapsed time.Duration, errsOut []error) {
	start := time.Now()
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				errsOut = append(errsOut, fmt.Errorf("%s: %w", root, err))
			}
			continue
		}
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				errsOut = append(errsOut, fmt.Errorf("%s: %w", p, err))
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				// Skip the install root itself and anything inside it: our
				// own versions directory holds no project `.istioctl-version`
				// files and walking it would be pointless (and dangerous if
				// a user ever created one by accident).
				if isInsideIstioEnvRoot(p) {
					return fs.SkipDir
				}
				// Skip well-known noisy dirs by base name — except at the
				// walk root, which the user explicitly asked us to scan.
				if p != root && scanSkipDirs[d.Name()] {
					return fs.SkipDir
				}
				return nil
			}
			filesVisited++
			if d.Name() != ".istioctl-version" {
				return nil
			}
			data, readErr := os.ReadFile(p)
			if readErr != nil {
				errsOut = append(errsOut, fmt.Errorf("%s: %w", p, readErr))
				return nil
			}
			v := strings.TrimSpace(string(data))
			if v == "" {
				return nil
			}
			if _, already := referenced[v]; !already {
				referenced[v] = p
			}
			matches++
			return nil
		})
	}
	return filesVisited, matches, time.Since(start), errsOut
}

// isInsideIstioEnvRoot returns true if p is the ISTIOENV_ROOT directory or
// lives beneath it, so the scanner can avoid walking into our own store.
// It is best-effort: if ISTIOENV_ROOT is unset or the path cannot be made
// absolute, the function returns false and the walker proceeds normally.
func isInsideIstioEnvRoot(p string) bool {
	root, ok := config.GetIstioEnvRoot()
	if !ok || root == "" {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absP, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	if absP == absRoot {
		return true
	}
	return strings.HasPrefix(absP, absRoot+string(os.PathSeparator))
}

// printKeepList prints a short "keeping ..." block so users understand why
// versions survived a prune.  Called from dry-run paths only; destructive
// prunes already report what was removed and omitting the keep list keeps
// their output tight.
func printKeepList(referenced map[string]string, keepByLast map[string]bool) {
	if len(referenced) == 0 && len(keepByLast) == 0 {
		return
	}
	keys := make([]string, 0, len(referenced)+len(keepByLast))
	seen := make(map[string]bool)
	for v := range referenced {
		if !seen[v] {
			keys = append(keys, v)
			seen[v] = true
		}
	}
	for v := range keepByLast {
		if !seen[v] {
			keys = append(keys, v)
			seen[v] = true
		}
	}
	sort.Strings(keys)
	fmt.Println("keeping:")
	for _, v := range keys {
		switch {
		case referenced[v] != "" && keepByLast[v]:
			fmt.Printf("  %s (referenced by %s; also within --keep-last)\n", v, referenced[v])
		case referenced[v] != "":
			fmt.Printf("  %s (referenced by %s)\n", v, referenced[v])
		default:
			fmt.Printf("  %s (within --keep-last)\n", v)
		}
	}
}

// PruneHelp prints help text for the prune command.
func PruneHelp() {
	fmt.Println(`Usage: istioctl-env prune [flags]

Remove installed istioctl versions that are not referenced by any configured
source.  A version is "referenced" when it is:

  - the global version ($ISTIOENV_ROOT/version), OR
  - the currently exported ISTIOENV_VERSION, OR
  - named in a .istioctl-version file found under any of the scan roots.

Scan roots default to $HOME and can be overridden with the environment
variable ISTIOENV_PRUNE_SCAN_ROOTS (OS path list separator, e.g.
":"-separated on Unix).

Flags:
  --keep-last N      Also keep the N newest installed versions, regardless of
                     whether they are referenced (default 0)
  --older-than DUR   Only remove versions whose directory mtime is older than
                     DUR (Go duration syntax, plus "d" days and "w" weeks,
                     e.g. 30m, 24h, 30d, 12w)
  --dry-run          Report what would be removed; do not delete anything
                     (this is the DEFAULT — use --yes to apply)
  --yes              Actually remove the selected versions
  -h, --help         Show this help and exit

Precedence: when both --dry-run and --yes are passed, --dry-run wins.`)
}
