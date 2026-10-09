# CLI reference

This reference covers every `istioctl-env` command implemented by the project.

Conventions:

- Commands return exit code `0` on success.
- On errors, commands typically print an error message to stderr and exit with code `1`.
- Some commands print help and exit `0`.

## Global usage

```text
istioctl-env [global flags] <command> [arguments]
```

Top-level help is available via `istioctl-env help`, `istioctl-env --help`, or `istioctl-env -h`.

## Global flags

These flags affect authentication, offline behavior, and host resolution. They may appear before or after the subcommand; each is a transient override for the matching environment variable listed in [Environment variables](installation-and-configuration.md#environment-variables). A `--` sentinel ends global-flag parsing (useful for `istioctl-env exec`).

| Flag | Env var | Meaning |
|------|---------|---------|
| `--offline` | `ISTIOENV_OFFLINE=1` | Disable all outbound HTTP. `list-remote` / `latest` serve from cache / baseline; `install` without an explicit version fails fast; `upgrade` is a no-op. |
| `--github-token <token>` | `ISTIOENV_GITHUB_TOKEN` | GitHub token used for both API requests and same-origin release-asset downloads. Stripped on cross-origin redirects. |
| `--api-mirror <url>` | `ISTIOENV_API_MIRROR` | Replace `https://api.github.com`. Mirror must speak the GitHub REST API. |
| `--download-mirror <url>` | `ISTIOENV_DOWNLOAD_MIRROR` | Replace `https://github.com` for release-asset URLs. Mirror must also serve `<asset>.sha256`. |

Both `--flag value` and `--flag=value` forms are accepted. If a flag and its matching env var are both set, the flag wins (because it sets the env var for the current process before the command runs).

```sh
istioctl-env --github-token "$GH_TOKEN" --api-mirror https://ghapi.corp install 1.24.0
istioctl-env --offline list-remote
```

## Environment variables

### `ISTIOENV_ROOT`

Required. Path where `istioctl-env` stores installed versions and shims.

Used by most commands and required for initialization.

### `ISTIOENV_VERSION`

Optional. When set, it forces a particular `istioctl` version to be used (highest priority).

Typically set via `istioctl-env shell` after enabling shell integration with `eval "$(istioctl-env init)"`.

### `ISTIOENV_AUTO_INSTALL`

Optional. When set to a truthy value (`1`, `true`, `yes`, `on`), the `istioctl`
shim will automatically install missing versions on first use rather than
failing. This mirrors `nvm`'s auto-install behaviour.

Combined with constraints in `.istioctl-version`, this makes a fresh clone of
a repository "just work" for new contributors.

## Version expressions

Everywhere a version argument is accepted (`.istioctl-version`,
`$ISTIOENV_ROOT/version`, `ISTIOENV_VERSION`, `istioctl-env local/global/shell
<version>`, `istioctl-env install <version>`), the value may be any of:

| Expression | Meaning |
|---|---|
| `1.24.0` | Exact pin |
| `~1.24.0` | `>=1.24.0, <1.25.0` (patch range) |
| `^1.24.0` | `>=1.24.0, <2.0.0` (minor range) |
| `^0.31.0` | `>=0.31.0, <0.32.0` (zero-major: minor-anchored, SemVer convention) |
| `>=1.24.0 <1.26.0` | Explicit interval (space- or comma-separated) |
| `latest` | Newest stable release |
| `latest-prerelease` | Newest release including pre-releases |

Constraints are resolved at `istioctl` invocation time: the shim picks the
newest installed version satisfying the expression. If none is installed, the
shim prints an actionable error (or auto-installs when `ISTIOENV_AUTO_INSTALL`
is set).

## Commands

### `help`

Purpose: Print usage and the list of available commands.

Syntax:

```text
istioctl-env help
istioctl-env --help
istioctl-env -h
```

Options/flags: none.

Environment variables: none.

Exit codes:

- `0` always.

Example:

```sh
istioctl-env help
```

---

### `version`

Purpose: Print the `istioctl-env` version.

Syntax:

```text
istioctl-env version
istioctl-env --version
istioctl-env -v
```

Options/flags: none.

Environment variables: none.

Exit codes:

- `0` on success.

Example:

```sh
istioctl-env version
```

---

### `init`

Purpose:

- Create required directories under `ISTIOENV_ROOT`.
- Generate the `istioctl` shim under `$ISTIOENV_ROOT/shims/istioctl`.
- Print shell initialization code to stdout for the requested shell
  (intended to be evaluated/sourced by your shell).

Syntax:

```text
istioctl-env init [--shell <bash|zsh|fish>]
```

Options/flags:

- `--shell <name>`: emit integration code for the named shell. When omitted,
  `istioctl-env` auto-detects the shell from `$SHELL`. If detection fails,
  it falls back to `bash` and prints a `WARN` to stderr.
- `-h`, `--help`: show command help and exit.

Environment variables:

- `ISTIOENV_ROOT` (required)
- `SHELL` (read for auto-detection)

Exit codes:

- `0` on success.
- `1` if `ISTIOENV_ROOT` is not set, if `--shell` has an unknown value, or if
  filesystem operations fail.

Example:

```sh
export ISTIOENV_ROOT="$HOME/.istioenv"
eval "$(istioctl-env init)"                 # auto-detect
eval "$(istioctl-env init --shell zsh)"     # explicit zsh
istioctl-env init --shell fish | source     # fish
```

---

### `list`

Purpose: List installed `istioctl` versions (newest to oldest).

Syntax:

```text
istioctl-env list [flags]
```

Options/flags:

- `--disk-usage`: also print the on-disk size of each version and a `TOTAL` row (useful before `prune`).
- `-h`, `--help`: show command help and exit.

Environment variables:

- `ISTIOENV_ROOT` (required)

Exit codes:

- `0` on success.
- `1` if `istioctl-env` is not initialized.

Example:

```sh
istioctl-env list
istioctl-env list --disk-usage
```

---

### `list-remote`

Purpose: List all available `istioctl` versions from GitHub tag v1.0.0 (newest to oldest).

This command does not require `ISTIOENV_ROOT` or initialization. If `ISTIOENV_ROOT` is set, results are persistently cached on disk (see [caching strategy](caching.md)).

Syntax:

```text
istioctl-env list-remote [flags]
```

Options/flags:

- `--prerelease`: include pre-release versions (alpha, beta, rc)
- `--cached`: print the on-disk cached list only, never performing a network
  call. Falls back to the built-in baseline when the cache is missing. This
  flag is intended for shell completion scripts that need a fast, offline
  answer on every TAB.
- `-h`, `--help`: show command help and exit

Environment variables: none.

Exit codes:

- `0` on success, or when printing `--help`.
- `1` on GitHub/network errors (including rate limiting).

Example:

```sh
istioctl-env list-remote --prerelease
```

---

### `latest`

Purpose: Print the latest available `istioctl` version from GitHub tag v1.0.0.

This command does not require `ISTIOENV_ROOT` or initialization. If `ISTIOENV_ROOT` is set, results are persistently cached on disk (see [caching strategy](caching.md)).

Syntax:

```text
istioctl-env latest [flags]
```

Options/flags:

- `--prerelease`: include pre-release versions when selecting the latest
- `-h`, `--help`: show command help and exit

Environment variables: none.

Exit codes:

- `0` on success, or when printing `--help`.
- `1` if no versions are found, or on GitHub/network errors.

Example:

```sh
istioctl-env latest
```

---

### `install`

Purpose: Download and install an `istioctl` version into `$ISTIOENV_ROOT/versions/<version>/istioctl`.

Argument semantics:

- An exact pin (`1.24.0`): downloaded as-is.
- A SemVer constraint (`~1.24.0`, `^1.24.0`, `>=1.24.0 <1.26.0`, `latest`,
  `latest-prerelease`): the remote release list is consulted and the newest
  matching version is installed. The resolution is printed as
  `Resolved <expr> → <concrete>` unless `--silent` is passed.
- Omitted: the nearest `.istioctl-version` file is consulted (walking up from
  the current directory). If no such file exists, `latest` is used.

The command displays a progress bar during the download and automatically verifies the integrity of the downloaded file using SHA256 checksums from the GitHub tag v1.0.0.

Syntax:

```text
istioctl-env install [version-or-constraint] [flags]
```

Options/flags:

- `-s`, `--silent`: do not display the progress bar or checksum verification information
- `-h`, `--help`: show command help and exit

Environment variables:

- `ISTIOENV_ROOT` (required)

Exit codes:

- `0` on success.
- `1` if not initialized, platform detection fails, download fails, checksum mismatch, or filesystem writes fail.

Example:

```sh
istioctl-env install 1.24.0           # exact pin
istioctl-env install ~1.24.0          # latest 1.24.x
istioctl-env install "^1.24.0"        # highest 1.x matching
istioctl-env install latest-prerelease
istioctl-env install                  # resolve from .istioctl-version
istioctl-env install --silent
```

---

### `uninstall`

Purpose: Remove an installed `istioctl` version directory.

Syntax:

```text
istioctl-env uninstall <version>
```

Options/flags: none.

Environment variables:

- `ISTIOENV_ROOT` (required)

Exit codes:

- `0` on success.
- `1` if `<version>` is missing, not initialized, the version is not installed, or filesystem removal fails.

Example:

```sh
istioctl-env uninstall 1.24.0
```

---

### `prune`

Purpose: Remove installed `istioctl` versions that are not referenced by any configured source. A version is considered **referenced** when it matches any of:

- the global version (`$ISTIOENV_ROOT/version`), or
- the currently exported `ISTIOENV_VERSION`, or
- the version named in any `.istioctl-version` file found under a configured scan root.

Scan roots default to `$HOME` and can be overridden with the environment variable `ISTIOENV_PRUNE_SCAN_ROOTS` (OS path list separator, e.g. `:`-separated on Unix). Setting `ISTIOENV_PRUNE_SCAN_ROOTS=""` explicitly disables filesystem scanning — useful on CI runners that only want the env-var and global guards.

The command is **non-destructive by default**: without `--yes`, it reports what *would* be removed and exits successfully without touching the filesystem.

The reference-discovery walk skips well-known noisy directories (`.git`, `node_modules`, `vendor`, `.cache`, `.gradle`, `.m2`, `.venv`, `__pycache__`, `.terraform`, `.idea`, `.vscode`, `.tox`, `dist`, `target`, `build`, `Library`) and never follows symbolic links. It also refuses to descend into `$ISTIOENV_ROOT`.

Syntax:

```text
istioctl-env prune [flags]
```

Options/flags:

- `--keep-last N`: also keep the N newest installed versions, regardless of whether they are referenced (default `0`).
- `--older-than DUR`: only remove versions whose directory mtime is older than `DUR`. Accepts Go duration syntax (`30m`, `24h`) plus the shorthand units `d` (days) and `w` (weeks), e.g. `30d`, `12w`, `1w3d`.
- `--dry-run`: report what would be removed; do not delete anything. This is the **default**.
- `--yes`: actually remove the selected versions. When combined with `--dry-run`, `--dry-run` wins (safer default).
- `-h`, `--help`: show command help and exit.

Environment variables:

- `ISTIOENV_ROOT` (required)
- `ISTIOENV_VERSION` (optional; when set, that version is referenced)
- `ISTIOENV_PRUNE_SCAN_ROOTS` (optional; path-list of filesystem roots to scan for `.istioctl-version` files; default `$HOME`)

Exit codes:

- `0` on success — including when there is nothing to prune, and when running in dry-run mode.
- `1` if `istioctl-env` is not initialized, the versions directory cannot be read, or a destructive removal fails.

Output:

Every invocation prints a one-line scan summary of the form:

```
scanned <N> files in <duration> under <root> — found <M> .istioctl-version file(s)
```

Followed by `would remove ...` lines (dry-run) or `removed ...` lines (`--yes`), and a final summary with the total bytes that would be / were freed. Dry-run mode additionally prints a `keeping:` block explaining why each surviving version was retained.

Example:

```sh
# Preview — safe, prints a plan.
istioctl-env prune

# Keep the two newest versions in addition to anything referenced.
istioctl-env prune --keep-last 2

# Only remove installs older than three months, and actually delete them.
istioctl-env prune --older-than 90d --yes

# Scan multiple source trees instead of all of $HOME.
ISTIOENV_PRUNE_SCAN_ROOTS="$HOME/code:$HOME/work" istioctl-env prune --yes
```

---

### `shell`

Purpose: Set or show the shell-level `istioctl` version expression.

Accepts any [version expression](#version-expressions) — plain pin, tilde,
caret, interval, or `latest` / `latest-prerelease`. The set-time check only
validates that the expression parses; the shim verifies installed-ness at
exec time (and may auto-install when `ISTIOENV_AUTO_INSTALL=true`).

Important: To *set* the version for your current shell session, you must have shell integration enabled via `eval "$(istioctl-env init)"`. Otherwise, you will only see the printed `export ...` line but your current shell will not be updated.

Syntax:

```text
istioctl-env shell            # show
istioctl-env shell <expression>  # set
```

Options/flags: none.

Environment variables:

- `ISTIOENV_ROOT` (required)
- `ISTIOENV_VERSION` (read on show; written when you eval shell integration)

Exit codes:

- `0` on success.
- `1` if not initialized, no shell version is configured (show), or the expression is unparseable.

Example:

```sh
eval "$(istioctl-env init)"
istioctl-env shell 1.24.0
istioctl-env shell "^1.24.0"
istioctl version
```

---

### `local`

Purpose: Set or show the local (directory-level) `istioctl` version expression.

Accepts any [version expression](#version-expressions). Setting writes the
expression verbatim into a `.istioctl-version` file in the current directory.
The set-time check only validates parsing; actual installed-ness is enforced
by the shim at exec time (and optionally auto-installed when
`ISTIOENV_AUTO_INSTALL=true`).

Syntax:

```text
istioctl-env local               # show
istioctl-env local <expression>  # set
```

Options/flags: none.

Environment variables:

- `ISTIOENV_ROOT` (required)

Exit codes:

- `0` on success.
- `1` if not initialized, no local version is configured for this directory (show), the expression is unparseable, or writing `.istioctl-version` fails.

Example:

```sh
istioctl-env install 1.24.0
istioctl-env local 1.24.0
istioctl-env local "~1.24.0"   # pin to the 1.24.x train
istioctl version
```

---

### `global`

Purpose: Set or show the global default `istioctl` version expression.

Accepts any [version expression](#version-expressions). Setting writes the
expression verbatim into `$ISTIOENV_ROOT/version`. The set-time check only
validates parsing; actual installed-ness is enforced by the shim at exec time
(and optionally auto-installed when `ISTIOENV_AUTO_INSTALL=true`).

Syntax:

```text
istioctl-env global               # show
istioctl-env global <expression>  # set
```

Options/flags: none.

Environment variables:

- `ISTIOENV_ROOT` (required)

Exit codes:

- `0` on success.
- `1` if not initialized, no global version is configured (show), the expression is unparseable, or writing fails.

Example:

```sh
istioctl-env install 1.24.0
istioctl-env global 1.24.0
istioctl-env global "^1.24.0"
istioctl version
```

---

### `which`

Purpose: Print the full path to the active `istioctl` binary that would be used based on version resolution.

If the active expression is a SemVer constraint, the resolution step is
printed to stderr (e.g. `^1.24.0 → 1.24.3`) so stdout remains a clean,
scriptable binary path.

Syntax:

```text
istioctl-env which
```

Options/flags: none.

Environment variables:

- `ISTIOENV_ROOT` (required)
- `ISTIOENV_VERSION` (optional; highest priority if set)

Exit codes:

- `0` on success.
- `1` if not initialized, no version is configured, or no installed version satisfies the constraint.

Example:

```sh
istioctl-env which
```

---

### `resolve`

Purpose: Resolve the active version expression (shell > local > global) to a
concrete installed version and print it on stdout. Primarily used by the
`istioctl` shim; also useful for scripts that need the resolved version.

With `--install`, missing versions are installed on-the-fly (equivalent to
setting `ISTIOENV_AUTO_INSTALL=true`).

Syntax:

```text
istioctl-env resolve [flags]
```

Options/flags:

- `--install`: install the best matching remote version when no installed version satisfies the expression
- `-s`, `--silent`: suppress installer progress output (recommended for non-TTY callers)
- `-h`, `--help`: show command help and exit

Environment variables:

- `ISTIOENV_ROOT` (required)
- `ISTIOENV_AUTO_INSTALL` (optional; when truthy, acts as `--install`)

Exit codes:

- `0` on success — the concrete version is printed on stdout.
- `1` if not initialized, no version configured, no installed match (without `--install`), or the install itself fails.

Example:

```sh
# Called by the shim's slow path.
istioctl-env resolve --silent
```

---

### `upgrade`

Purpose: Download the tag v1.0.0 of `istioctl-env` from GitHub and replace the current binary in-place.

Syntax:

```text
istioctl-env upgrade
```

Options/flags: none.

Environment variables: none (the binary path is auto-detected via `os.Executable()`).

Exit codes:

- `0` on success, or when already up to date.
- `1` on network/download errors, permission issues, or filesystem errors.

Notes:

- The command detects the current OS and CPU architecture automatically.
- The new binary is written atomically (temp file + rename) to avoid corruption.
- If the current version is a development build (`dev`), the upgrade always proceeds.

Example:

```sh
istioctl-env upgrade
```

---

### `exec`

Purpose: Run a specific version of `istioctl` for a single command without changing the active version (shell, local, or global).

Syntax:

```text
istioctl-env exec <version> <command> [args...]
```

Options/flags: none.

Environment variables:

- `ISTIOENV_ROOT` (required)
- `ISTIOENV_VERSION` (set for the subprocess to match the requested version)

Exit codes:

- Exit code of the executed command.
- `1` if the version is not installed or initialization fails.

Example:

```sh
istioctl-env exec 1.24.0 version
```

---

### `status`

Purpose: Display a comprehensive overview of the current `istioctl-env` environment.

Output includes:
- `ISTIOENV_ROOT` path.
- Currently active version and the source it was resolved from.
- Full path to the active `istioctl` binary.
- List of all installed versions (active one marked with `*`).

Syntax:

```text
istioctl-env status
```

Options/flags: none.

Environment variables:

- `ISTIOENV_ROOT` (required)
- `ISTIOENV_VERSION` (read if set)

Exit codes:

- `0` on success.
- `1` on error.

Example:

```sh
istioctl-env status
```

---

### `doctor`

Purpose: Diagnose the `istioctl-env` environment and print a labeled
`OK`/`WARN`/`FAIL` for each check with actionable guidance.

Checks performed (in order):

1. `ISTIOENV_ROOT` is set.
2. `ISTIOENV_ROOT` exists and is writable.
3. `$ISTIOENV_ROOT/versions` directory exists.
4. `$ISTIOENV_ROOT/shims/istioctl` exists and byte-matches the current
   generator output (drift detection — catches stale shims left behind
   by an upgrade).
5. `$ISTIOENV_ROOT/shims` is on `PATH`, ahead of any other `istioctl`
   on `PATH` (catches the common case where Homebrew's `istioctl` wins).
6. Shell integration appears active (an `istioctl-env init` line is present
   in `~/.bashrc`, `~/.zshrc` or `~/.config/fish/config.fish`). This is
   best-effort and never reports `FAIL`.
7. An active `istioctl` version resolves. The detail line names the
   source (`ISTIOENV_VERSION environment variable`,
   `.istioctl-version file`, or `global version file`) so you can tell
   at a glance which precedence tier won.
8. The active version's binary exists at
   `$ISTIOENV_ROOT/versions/<v>/istioctl`, is a regular file, and is
   executable.
9. Every installed version has an executable `istioctl` binary.
10. "Dangling versions": version directories under
    `$ISTIOENV_ROOT/versions/` with no `istioctl` binary inside at all
    (typically a half-finished install). Remediation: reinstall the
    version or run `istioctl-env uninstall <version>`.
11. GitHub (`https://api.github.com/rate_limit`) is reachable within
    3 seconds, and `X-RateLimit-Remaining` has sufficient headroom
    (warns when fewer than 10 requests remain in the current window).
    If `GITHUB_TOKEN` is set it is used, so the headroom reflects the
    5,000/hour authenticated bucket instead of the 60/hour anonymous
    one.
12. `$ISTIOENV_ROOT/cache/releases.json`, if it exists, is valid JSON.
13. Only with `--deep`: for every installed version, re-download its
    archive and `.sha256` from GitHub, verify the archive against its
    published checksum, extract the `istioctl` entry, and byte-compare
    it against the on-disk binary. A mismatch is reported as `FAIL`;
    network or extraction errors are reported as `WARN` since they are
    not strong evidence of local corruption.

Syntax:

```text
istioctl-env doctor [flags]
```

Options/flags:

- `--fix`: attempt to repair issues it can resolve safely:
  - Regenerate the shim script by calling the internal shim generator.
  - `chmod 0755` any version binary that is not currently executable.
  - Does NOT modify `PATH` or shell rc files — doctor prints the
    instructions instead.
- `--deep`: additionally run check 13 (per-version checksum
  re-verification). Downloads each installed version's full archive
  from GitHub, so expect tens of MB of traffic per installed version.
- `-h`, `--help`: show command help and exit.

Environment variables:

- `ISTIOENV_ROOT` (optional — doctor runs even when unset, reporting the
  missing root as `FAIL`).
- `HOME` (optional — used to scan shell rc files for the integration check).
- `GITHUB_TOKEN` (optional — forwarded to the rate-limit probe so the
  reported headroom matches your real workflow).

Exit codes:

- `0` if no check reports `FAIL`.
- `1` if at least one check reports `FAIL`.

Example:

```sh
istioctl-env doctor
istioctl-env doctor --fix
istioctl-env doctor --deep         # verify installed binaries against GitHub
istioctl-env doctor --fix --deep   # repair, then verify
```

---

### `autocompletion`

Purpose: Generate an idiomatic shell completion script for `istioctl-env`.

The script completes subcommands and version arguments. For `install`, it
offers **remote** versions read from the on-disk release cache (via
`istioctl-env list-remote --cached`, which never hits the network). For
`uninstall`, `shell`, `local`, `global`, and `exec`, it offers **installed**
versions via `istioctl-env list`. Flags such as `--prerelease`, `--cached`,
`--silent`, and `--shell` are also completed where appropriate.

Syntax:

```text
istioctl-env autocompletion [--shell <bash|zsh|fish>]
```

Options/flags:

- `--shell <name>`: emit the completion script for the named shell. When
  omitted, `istioctl-env` auto-detects the shell from `$SHELL`. If detection
  fails, it falls back to `bash` and prints a `WARN` to stderr.
- `-h`, `--help`: show command help and exit

Environment variables:

- `SHELL` (read for auto-detection)

Exit codes:
- `0` on success.
- `1` if `--shell` has an unknown value.

Example:

```sh
# Bash — enable for the current session or persist in ~/.bashrc
source <(istioctl-env autocompletion --shell bash)

# Zsh — compinit must have run first (usually via your framework)
autoload -Uz compinit && compinit
source <(istioctl-env autocompletion --shell zsh)

# Fish — one-time install
istioctl-env autocompletion --shell fish > ~/.config/fish/completions/istioctl-env.fish
```
