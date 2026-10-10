package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/user/istioctl-env/internal/config"
	"github.com/user/istioctl-env/internal/github"
	"github.com/user/istioctl-env/internal/semver"
)

// ReadNearestLocalSpec walks up from the current working directory looking
// for a .istioctl-version file and returns its trimmed contents.  It returns
// ("", nil) when no file is found.
func ReadNearestLocalSpec() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	v, err := config.FindLocalVersionFrom(cwd)
	if err != nil {
		return "", nil // not finding a file is not an error here
	}
	return strings.TrimSpace(v), nil
}

// isTruthy returns true for the usual shell "truthy" strings.
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// AutoInstallEnabled reports whether ISTIOENV_AUTO_INSTALL is set to a truthy
// value.  Used by the shim's slow path.
func AutoInstallEnabled() bool {
	return isTruthy(os.Getenv("ISTIOENV_AUTO_INSTALL"))
}

// resolveSpecForInstall turns a user-supplied version spec into the
// concrete istioctl version that should be installed.  It accepts:
//
//   - an empty spec: falls back to the nearest .istioctl-version, then to
//     "latest" as a last resort;
//   - a plain exact pin ("1.24.0"): returned verbatim;
//   - "latest" / "latest-prerelease": resolved against the remote release
//     list (newest wins);
//   - any SemVer constraint ("^1.24.0", ">=1.24.0 <1.26.0", ...): resolved
//     against the remote release list (newest matching wins).
//
// The second return value is the raw spec that was ultimately parsed, so
// callers can produce helpful log messages.
func resolveSpecForInstall(client *github.Client, spec string) (resolved string, raw string, err error) {
	spec = strings.TrimSpace(spec)

	if spec == "" {
		local, lerr := ReadNearestLocalSpec()
		if lerr == nil && local != "" {
			spec = local
		} else {
			spec = "latest"
		}
	}
	raw = spec

	// Fast path: plain exact pin → install it directly.
	if !semver.IsConstraint(spec) {
		return spec, raw, nil
	}

	constraint, err := semver.ParseConstraint(spec)
	if err != nil {
		return "", raw, fmt.Errorf("invalid version expression %q: %w", spec, err)
	}

	stable, pre, err := getRemoteVersions(client)
	if err != nil {
		return "", raw, fmt.Errorf("failed to list remote versions: %w", err)
	}

	// Include pre-releases only when the constraint asks for them.
	candidates := stable
	if constraintIncludesPrerelease(constraint, spec) {
		candidates = pre
	}

	match, ok := semver.HighestMatching(constraint, candidates)
	if !ok {
		return "", raw, fmt.Errorf("no remote istioctl version satisfies %q", spec)
	}
	return match, raw, nil
}

// constraintIncludesPrerelease returns true when the given constraint should
// be matched against the pre-release candidate pool.  Today this is true for
// the "latest-prerelease" selector and for constraints whose text contains
// a pre-release tag (e.g. ">=1.24.0-rc.1").
func constraintIncludesPrerelease(_ semver.Constraint, raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if lower == "latest-prerelease" {
		return true
	}
	// Pre-releases are introduced by a '-' between the version core and
	// the identifier (e.g. 1.24.0-rc.1).  A single '-' anywhere in a
	// constraint is a strong indicator.
	return strings.Contains(raw, "-")
}

// resolveSpecForShim returns the concrete installed version that satisfies
// spec, optionally auto-installing the best matching remote version when
// nothing installed matches.  An empty spec means the active expression
// (shell > local > global).
//
// It is intentionally a different flow from resolveSpecForInstall:
//
//   - the shim prefers an already-installed version matching the constraint
//     over hitting the network;
//   - auto-install is gated on ISTIOENV_AUTO_INSTALL (or the explicit
//     install=true argument), so non-TTY callers that haven't opted in get
//     a clear error instead of a silent background download.
func resolveSpecForShim(client *github.Client, spec string, install bool, silent bool) (resolved string, raw string, err error) {
	raw = strings.TrimSpace(spec)
	if raw == "" {
		raw, err = config.ResolveVersion()
		if err != nil {
			return "", "", err
		}
	}

	// Not a constraint → trivial resolution.  Honour auto-install for exact
	// pins too so a cloned-repo workflow works out of the box.
	if !semver.IsConstraint(raw) {
		installed, _ := config.IsVersionInstalled(raw)
		if installed {
			return raw, raw, nil
		}
		if !install {
			return "", raw, fmt.Errorf("version %s is not installed. Set ISTIOENV_AUTO_INSTALL=true or run 'istioctl-env install %s'", raw, raw)
		}
		if err := installWithClient(client, raw, silent); err != nil {
			return "", raw, err
		}
		return raw, raw, nil
	}

	constraint, err := semver.ParseConstraint(raw)
	if err != nil {
		return "", raw, fmt.Errorf("invalid version expression %q: %w", raw, err)
	}

	// Prefer the highest already-installed version satisfying the constraint.
	installed, err := config.ListInstalledVersions()
	if err != nil {
		return "", raw, err
	}
	if match, ok := semver.HighestMatching(constraint, installed); ok {
		return match, raw, nil
	}

	if !install {
		return "", raw, fmt.Errorf("no installed istioctl version satisfies %q. Set ISTIOENV_AUTO_INSTALL=true or run 'istioctl-env install'", raw)
	}

	stable, pre, err := getRemoteVersions(client)
	if err != nil {
		return "", raw, fmt.Errorf("failed to list remote versions: %w", err)
	}
	candidates := stable
	if constraintIncludesPrerelease(constraint, raw) {
		candidates = pre
	}
	match, ok := semver.HighestMatching(constraint, candidates)
	if !ok {
		return "", raw, fmt.Errorf("no remote istioctl version satisfies %q", raw)
	}
	if err := installWithClient(client, match, silent); err != nil {
		return "", raw, err
	}
	return match, raw, nil
}
