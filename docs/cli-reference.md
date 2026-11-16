# CLI reference

This reference describes each `istioctl-env` command in the project.

Conventions:

- Commands return exit code `0` on success.
- On errors, commands usually print an error message to stderr and exit with code `1`.
- Some commands print help and exit `0`.

## Global usage

```text
istioctl-env [global flags] <command> [arguments]
```

For top-level help, run `istioctl-env help`, `istioctl-env --help`, or `istioctl-env -h`.

## Global flags

These flags change authentication, offline behavior, and host resolution. They can come before or after the subcommand. Each flag overrides the matching environment variable for one run. [Environment variables](installation-and-configuration.md#environment-variables) lists these variables. A `--` sentinel stops the parsing of global flags (useful for `istioctl-env exec`).

| Flag | Env var | Meaning |
|------|---------|---------|
| `--offline` | `ISTIOENV_OFFLINE=1` | Disable all outbound HTTP. `list-remote` and `latest` use the cache or the baseline. `install` without an explicit version fails at once. `upgrade` does nothing. |
| `--github-token <token>` | `ISTIOENV_GITHUB_TOKEN` | GitHub token for API requests and for release-asset downloads from the same origin. `istioctl-env` removes it on cross-origin redirects. |
| `--api-mirror <url>` | `ISTIOENV_API_MIRROR` | Replace `https://api.github.com`. The mirror must speak the GitHub REST API. |
| `--download-mirror <url>` | `ISTIOENV_DOWNLOAD_MIRROR` | Replace `https://github.com` in release-asset URLs. The mirror must also serve `<asset>.sha256`. |

`istioctl-env` accepts both the `--flag value` and the `--flag=value` forms. If you set a flag and its matching env var, the flag wins. The reason is that the flag sets the env var for the current process before the command runs.

```sh
istioctl-env --github-token "$GH_TOKEN" --api-mirror https://ghapi.corp install 1.24.0
istioctl-env --offline list-remote
```

## Environment variables

### `ISTIOENV_ROOT`

Required. The path where `istioctl-env` stores installed versions and shims.

Most commands use it. Initialization requires it.

### `ISTIOENV_VERSION`

Optional. When it is set, the shim uses this `istioctl` version (highest priority).

Usually `istioctl-env shell` sets it, after you enable shell integration with `eval "$(istioctl-env init)"`.

### `ISTIOENV_AUTO_INSTALL`

Optional. Set it to a truthy value (`1`, `true`, `yes`, `on`) to change what
the `istioctl` shim does with a missing version. It installs the version on
first use and does not fail. This works like the auto-install of `nvm`. The
variable also enables `--install` for `resolve` and `--auto` for `exec`
(`exec --no-auto` overrides it).

With constraints in `.istioctl-version`, a fresh clone of a repository then
works at once for new contributors.

### `ISTIOENV_CACHE_TTL`

Optional. How long the release cache on disk (`$ISTIOENV_ROOT/cache/releases.json`)
stays fresh, as a Go duration string (`30m`, `24h`, `0s`). Default `1h`.
See [caching strategy](caching.md).

### `ISTIOENV_MIRROR_URL`

Optional, **deprecated**. The legacy name for `ISTIOENV_DOWNLOAD_MIRROR`.
`istioctl-env` reads it only when `ISTIOENV_DOWNLOAD_MIRROR` is empty.

### `ISTIOENV_PRUNE_SCAN_ROOTS`

Optional. The directories that `prune` scans for `.istioctl-version` files. See [`prune`](#prune).

## Version expressions

Each place that accepts a version accepts all of these expressions. These
places are `.istioctl-version`, `$ISTIOENV_ROOT/version`, `ISTIOENV_VERSION`,
`istioctl-env local/global/shell <version>` and `istioctl-env install <version>`.

| Expression | Meaning |
|---|---|
| `1.24.0` | Exact pin |
| `~1.24.0` | `>=1.24.0, <1.25.0` (patch range) |
| `^1.24.0` | `>=1.24.0, <2.0.0` (minor range) |
| `^0.31.0` | `>=0.31.0, <0.32.0` (zero-major: minor-anchored, SemVer convention) |
| `>=1.24.0 <1.26.0` | Explicit interval (space- or comma-separated) |
| `latest` | Newest stable release |
| `latest-prerelease` | Newest release including pre-releases |

The shim resolves constraints each time you run `istioctl`. It picks the
newest installed version that satisfies the expression. If no such version is
installed, the shim prints an error that tells you what to do. If
`ISTIOENV_AUTO_INSTALL` is set, the shim installs the version instead.

## Commands

### `help`

Purpose: Print the usage and the list of available commands.

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

- Create the required directories under `ISTIOENV_ROOT`.
- Generate the `istioctl` shim at `$ISTIOENV_ROOT/shims/istioctl`.
- Print the shell initialization code for the requested shell to stdout.
  Your shell evaluates or sources this code.

Syntax:

```text
istioctl-env init [--shell <bash|zsh|fish>]
```

Options/flags:

- `--shell <name>`: print the integration code for the named shell. Without
  this flag, `istioctl-env` detects the shell from `$SHELL`. If detection fails,
  it falls back to `bash` and prints a `WARN` to stderr.
- `-h`, `--help`: show command help and exit.

Environment variables:

- `ISTIOENV_ROOT` (required)
- `SHELL` (read for auto-detection)

Exit codes:

- `0` on success.
- `1` if `ISTIOENV_ROOT` is not set, if `--shell` has an unknown value, or if
  a filesystem operation fails.

Example:

```sh
export ISTIOENV_ROOT="$HOME/.istioenv"
eval "$(istioctl-env init)"                 # auto-detect
eval "$(istioctl-env init --shell zsh)"     # explicit zsh
istioctl-env init --shell fish | source     # fish
```

---

### `list`

Purpose: List the installed `istioctl` versions (newest to oldest).

Syntax:

```text
istioctl-env list [flags]
```

Options/flags:

- `--disk-usage`: also print the size on disk of each version and a `TOTAL` row (useful before `prune`).
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

Purpose: List all available `istioctl` versions from GitHub releases (newest to oldest).

This command does not need `ISTIOENV_ROOT` or initialization. If `ISTIOENV_ROOT` is set, the command keeps the results in a cache on disk (see [caching strategy](caching.md)).

Syntax:

```text
istioctl-env list-remote [flags]
```

Options/flags:

- `--prerelease`: include pre-release versions (alpha, beta, rc)
- `--cached`: print only the cached list from disk, with no network
  call. If the cache is missing, use the built-in baseline. Shell completion
  scripts use this flag, because they need a fast offline answer on each TAB.
- `-h`, `--help`: show command help and exit

Environment variables: none.

Exit codes:

- `0` on success, or when the command prints `--help`.
- `1` on GitHub or network errors (rate limiting included).

Example:

```sh
istioctl-env list-remote --prerelease
```

---

### `latest`

Purpose: Print the latest available `istioctl` version from GitHub releases.

This command does not need `ISTIOENV_ROOT` or initialization. If `ISTIOENV_ROOT` is set, the command keeps the results in a cache on disk (see [caching strategy](caching.md)).

Syntax:

```text
istioctl-env latest [flags]
```

Options/flags:

- `--prerelease`: include pre-release versions when you select the latest
- `-h`, `--help`: show command help and exit

Environment variables: none.

Exit codes:

- `0` on success, or when the command prints `--help`.
- `1` if the command finds no versions, or on GitHub or network errors.

Example:

```sh
istioctl-env latest
```

---

### `install`

Purpose: Download and install an `istioctl` version into `$ISTIOENV_ROOT/versions/<version>/istioctl`.

What the argument means:

- An exact pin (`1.24.0`): the command downloads this version as it is.
- A SemVer constraint (`~1.24.0`, `^1.24.0`, `>=1.24.0 <1.26.0`, `latest`,
  `latest-prerelease`): the command reads the remote release list and installs
  the newest matching version. It prints the resolution as
  `Resolved <expr> → <concrete>`, unless you pass `--silent`.
- No argument: the command reads the nearest `.istioctl-version` file. It
  looks in the current directory first, then in each parent directory. If no
  such file exists, it uses `latest`.

The command shows a progress bar during the download. It also verifies the downloaded file automatically against the SHA256 checksums that come with the GitHub release.

Syntax:

```text
istioctl-env install [version-or-constraint] [flags]
```

Options/flags:

- `-s`, `--silent`: do not show the progress bar or the checksum verification information
- `-h`, `--help`: show command help and exit

Environment variables:

- `ISTIOENV_ROOT` (required)

Exit codes:

- `0` on success.
- `1` if `istioctl-env` is not initialized, platform detection fails, the download fails, the checksum does not match, or a filesystem write fails.

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

Purpose: Remove the directory of an installed `istioctl` version.

Syntax:

```text
istioctl-env uninstall <version>
```

Options/flags:

- `-h`, `--help`: show command help and exit

Environment variables:

- `ISTIOENV_ROOT` (required)

Exit codes:

- `0` on success.
- `1` if `<version>` is missing, `istioctl-env` is not initialized, the version is not installed, or the removal from the filesystem fails.

Example:

```sh
istioctl-env uninstall 1.24.0
```

---

### `prune`

Purpose: Remove installed `istioctl` versions that no configured source references. A version is **referenced** when it matches one of these:

- the global version (`$ISTIOENV_ROOT/version`)
- the currently exported `ISTIOENV_VERSION`
- the version in a `.istioctl-version` file under a configured scan root

The default scan root is `$HOME`. To override it, set the environment variable `ISTIOENV_PRUNE_SCAN_ROOTS`. It uses the path list separator of the OS, e.g. `:` on Unix. To disable filesystem scanning, set `ISTIOENV_PRUNE_SCAN_ROOTS=""`. This is useful on CI runners that only want the env var and global guards.

The command is **non-destructive by default**. Without `--yes`, it reports what it *would* remove. Then it exits successfully and does not touch the filesystem.

The scan for references skips well-known noisy directories (`.git`, `node_modules`, `vendor`, `.cache`, `.gradle`, `.m2`, `.venv`, `__pycache__`, `.terraform`, `.idea`, `.vscode`, `.tox`, `dist`, `target`, `build`, `Library`). It never follows symbolic links. It also never goes into `$ISTIOENV_ROOT`.

Syntax:

```text
istioctl-env prune [flags]
```

Options/flags:

- `--keep-last N`: also keep the N newest installed versions, even if nothing references them (default `0`).
- `--older-than DUR`: only remove versions whose directory mtime is older than `DUR`. Accepts Go duration syntax (`30m`, `24h`) and the shorthand units `d` (days) and `w` (weeks), e.g. `30d`, `12w`, `1w3d`.
- `--dry-run`: report what the command would remove, but delete nothing. This is the **default**.
- `--yes`: remove the selected versions. If you also give `--dry-run`, `--dry-run` wins (safer default).
- `-h`, `--help`: show command help and exit.

Environment variables:

- `ISTIOENV_ROOT` (required)
- `ISTIOENV_VERSION` (optional. When it is set, that version is referenced.)
- `ISTIOENV_PRUNE_SCAN_ROOTS` (optional. A path list of filesystem roots to scan for `.istioctl-version` files. Default `$HOME`.)

Exit codes:

- `0` on success. This includes runs with nothing to prune and runs in dry-run mode.
- `1` if `istioctl-env` is not initialized, the command cannot read the versions directory, or a destructive removal fails.

Output:

Each run prints a one-line scan summary in this form:

```
scanned <N> files in <duration> under <root> — found <M> .istioctl-version file(s)
```

Then it prints `would remove ...` lines (dry run) or `removed ...` lines (`--yes`). A final summary gives the total bytes that the command would free or freed. In dry-run mode, a `keeping:` block also tells why the command kept each remaining version.

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

Purpose: Set or show the `istioctl` version expression for the shell.

Accepts any [version expression](#version-expressions): a plain pin, tilde,
caret, interval, or `latest` / `latest-prerelease`. When you set the
expression, the command only checks that it parses. The shim checks that the
version is installed when it runs. If `ISTIOENV_AUTO_INSTALL=true`, the shim
can install the version.

Important: To *set* the version for your current shell session, you must enable shell integration with `eval "$(istioctl-env init)"`. Without it, you only see the printed `export ...` line, and your current shell does not change.

Syntax:

```text
istioctl-env shell            # show
istioctl-env shell <expression>  # set
```

Options/flags:

- `-h`, `--help`: show command help and exit

Environment variables:

- `ISTIOENV_ROOT` (required)
- `ISTIOENV_VERSION` (read when you show the version. Written when you eval the shell integration.)

Exit codes:

- `0` on success.
- `1` if `istioctl-env` is not initialized, no shell version is configured (show), or the expression does not parse.

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

Accepts any [version expression](#version-expressions). When you set the
expression, the command writes it unchanged into a `.istioctl-version` file in
the current directory. At that time, the command only checks that the
expression parses. The shim checks that the version is installed when it runs.
If `ISTIOENV_AUTO_INSTALL=true`, the shim can install the version.

Syntax:

```text
istioctl-env local               # show
istioctl-env local <expression>  # set
```

Options/flags:

- `-h`, `--help`: show command help and exit

Environment variables:

- `ISTIOENV_ROOT` (required)

Exit codes:

- `0` on success.
- `1` if `istioctl-env` is not initialized, no local version is configured for this directory (show), the expression does not parse, or the write of `.istioctl-version` fails.

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

Accepts any [version expression](#version-expressions). When you set the
expression, the command writes it unchanged into `$ISTIOENV_ROOT/version`. At
that time, the command only checks that the expression parses. The shim checks
that the version is installed when it runs. If `ISTIOENV_AUTO_INSTALL=true`,
the shim can install the version.

Syntax:

```text
istioctl-env global               # show
istioctl-env global <expression>  # set
```

Options/flags:

- `-h`, `--help`: show command help and exit

Environment variables:

- `ISTIOENV_ROOT` (required)

Exit codes:

- `0` on success.
- `1` if `istioctl-env` is not initialized, no global version is configured (show), the expression does not parse, or the write fails.

Example:

```sh
istioctl-env install 1.24.0
istioctl-env global 1.24.0
istioctl-env global "^1.24.0"
istioctl version
```

---

### `which`

Purpose: Print the full path to the active `istioctl` binary, as version resolution selects it.

If the active expression is a SemVer constraint, the command prints the
resolution step to stderr (e.g. `^1.24.0 → 1.24.3`). Thus stdout contains
only the binary path, and scripts can use it.

Syntax:

```text
istioctl-env which
```

Options/flags:

- `-h`, `--help`: show command help and exit

Environment variables:

- `ISTIOENV_ROOT` (required)
- `ISTIOENV_VERSION` (optional. It has the highest priority if it is set.)

Exit codes:

- `0` on success.
- `1` if `istioctl-env` is not initialized, no version is configured, or no installed version satisfies the constraint.

Example:

```sh
istioctl-env which
```

---

### `resolve`

Purpose: Resolve a [version expression](#version-expressions) to a concrete
installed version and print it on stdout. Without `<spec>`, the command uses
the active expression (shell > local > global). The `istioctl` shim is the
main user of this command. Scripts that need the resolved version can also
use it.

The command matches installed versions first. With `--install`, it installs a
missing version during the call (the same as `ISTIOENV_AUTO_INSTALL=true`).
The install progress goes to stderr, so stdout contains only the bare version.

Syntax:

```text
istioctl-env resolve [<spec>] [flags]
```

Options/flags:

- `--install`: if no installed version satisfies the expression, install the best matching remote version
- `-s`, `--silent`: do not show the installer progress (recommended for callers without a TTY)
- `-h`, `--help`: show command help and exit

Environment variables:

- `ISTIOENV_ROOT` (required)
- `ISTIOENV_AUTO_INSTALL` (optional. When it is truthy, it acts as `--install`.)

Exit codes:

- `0` on success. The command prints the concrete version on stdout.
- `1` if `istioctl-env` is not initialized, no version is configured, no installed version matches (without `--install`), or the install fails.

Example:

```sh
# Called by the shim's slow path.
istioctl-env resolve --silent

# Resolve an explicit constraint against installed versions.
istioctl-env resolve "~1.24.0"
```

---

### `upgrade`

Purpose: Get the latest `istioctl-env` release from GitHub (`mmpyro/istioctl-env`) and replace the current binary in place.

Syntax:

```text
istioctl-env upgrade
```

Options/flags:

- `-h`, `--help`: show command help and exit (never upgrades)

Environment variables (the command finds the path of the binary itself with `os.Executable()`):

- `ISTIOENV_OFFLINE`: skip the upgrade with a message and exit `0`.
- `ISTIOENV_GITHUB_TOKEN` / `GITHUB_TOKEN`: GitHub token for the API request.
- `ISTIOENV_API_MIRROR`: a different GitHub API base, for the lookup of the latest release.
- `ISTIOENV_DOWNLOAD_MIRROR` (legacy `ISTIOENV_MIRROR_URL`): a different download base for the release asset.

The matching global flags (`--offline`, `--github-token`, `--api-mirror`, `--download-mirror`) also work.

Exit codes:

- `0` on success, or when the binary is already up to date.
- `1` on network or download errors, permission problems, or filesystem errors.

Notes:

- The command detects the current OS and CPU architecture automatically.
- The command writes the new binary atomically (temp file + rename), so the binary does not become corrupt.
- If the current version is a development build (`dev`), the upgrade always continues.

Example:

```sh
istioctl-env upgrade
```

---

### `exec`

Purpose: Run a specific version of `istioctl` for a single command. The active version (shell, local, or global) does not change.

`<version>` can be an exact pin or any [version expression](#version-expressions).
The command resolves it like [`resolve`](#resolve) (installed versions first).
It passes `<command>` and `[args...]` to that `istioctl` binary. It removes one
leading `--` after `<version>`. Thus `exec 1.24.0 -- --help` passes `--help` to istioctl.

Syntax:

```text
istioctl-env exec [--auto|--no-auto] <version> [--] <command> [args...]
```

Options/flags (they must come before `<version>`):

- `--auto`: if no installed version matches, install the best matching remote version
- `--no-auto`: never install, even when `ISTIOENV_AUTO_INSTALL` is set
- `-h`, `--help`: show command help and exit

Environment variables:

- `ISTIOENV_ROOT` (required)
- `ISTIOENV_AUTO_INSTALL` (optional. When it is truthy, it acts as `--auto`.)
- `ISTIOENV_VERSION` (the command sets it to the concrete resolved version for the subprocess)

Exit codes:

- The exit code of `istioctl`, unchanged.
- `1` if initialization fails, the command cannot resolve the version, or the install fails.

Example:

```sh
istioctl-env exec 1.24.0 version
istioctl-env exec "~1.24.0" -- analyze -n default
istioctl-env exec --auto 1.25.0 version
```

---

### `status`

Purpose: Show an overview of the current `istioctl-env` environment.

The output includes:

- The `ISTIOENV_ROOT` path.
- The active version and the source that set it.
- The full path to the active `istioctl` binary.
- A list of all installed versions. A `*` marks the active version.

Syntax:

```text
istioctl-env status
```

Options/flags:

- `-h`, `--help`: show command help and exit

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

Purpose: Diagnose the `istioctl-env` environment. For each check, the command
prints an `OK`, `WARN` or `FAIL` label and tells you what to do.

The command does these checks in this order:

1. `ISTIOENV_ROOT` is set.
2. `ISTIOENV_ROOT` exists and is writable.
3. The `$ISTIOENV_ROOT/versions` directory exists.
4. `$ISTIOENV_ROOT/shims/istioctl` exists and is byte-identical to the current
   generator output. This drift check finds stale shims that an upgrade left
   behind.
5. `$ISTIOENV_ROOT/shims` is on `PATH`, before any other `istioctl` on `PATH`.
   This check finds the common case where the Homebrew `istioctl` wins.
6. Shell integration seems active: an `istioctl-env init` line is present in
   `~/.bashrc`, `~/.zshrc` or `~/.config/fish/config.fish`. This check is
   best-effort and never reports `FAIL`.
7. An active `istioctl` version resolves. The detail line names the source
   (`ISTIOENV_VERSION environment variable`, `.istioctl-version file`, or
   `global version file`). Thus you can see at once which precedence tier won.
8. The binary of the active version exists at
   `$ISTIOENV_ROOT/versions/<v>/istioctl`, is a regular file, and is
   executable.
9. Each installed version has an executable `istioctl` binary.
10. "Dangling versions": version directories under `$ISTIOENV_ROOT/versions/`
    with no `istioctl` binary in them. Usually a half-finished install causes
    this. To fix it, install the version again or run
    `istioctl-env uninstall <version>`.
11. GitHub (`https://api.github.com/rate_limit`) answers within 3 seconds, and
    `X-RateLimit-Remaining` has enough headroom. The check warns when fewer
    than 10 requests remain in the current window. If `GITHUB_TOKEN` is set,
    the check uses it. Then the headroom shows the 5,000/hour authenticated
    bucket, not the 60/hour anonymous one.
12. If `$ISTIOENV_ROOT/cache/releases.json` exists, it contains valid JSON.
13. Only with `--deep`: for each installed version, the command downloads the
    archive and the `.sha256` from GitHub again. It verifies the archive
    against its published checksum, extracts the `istioctl` entry, and
    compares it byte for byte with the binary on disk. A mismatch is a `FAIL`.
    Network or extraction errors are a `WARN`, because they are not strong
    evidence of local corruption.

Syntax:

```text
istioctl-env doctor [flags]
```

Options/flags:

- `--fix`: try to repair the problems that the command can fix safely:
  - It generates the shim script again with the internal shim generator.
  - It runs `chmod 0755` on each version binary that is not executable.
  - It does NOT change `PATH` or shell rc files. Instead, `doctor` prints
    the instructions.
- `--deep`: also run check 13 (the checksum check for each version again).
  This downloads the full archive of each installed version from GitHub.
  Expect tens of MB of traffic for each installed version.
- `-h`, `--help`: show command help and exit.

Environment variables:

- `ISTIOENV_ROOT` (optional. `doctor` runs even when it is unset, and reports
  the missing root as `FAIL`.)
- `HOME` (optional. The integration check uses it to find the shell rc files.)
- `GITHUB_TOKEN` (optional. `doctor` sends it to the rate-limit probe, so the
  reported headroom matches your real workflow.)

Exit codes:

- `0` if no check reports `FAIL`.
- `1` if one or more checks report `FAIL`.

Example:

```sh
istioctl-env doctor
istioctl-env doctor --fix
istioctl-env doctor --deep         # verify installed binaries against GitHub
istioctl-env doctor --fix --deep   # repair, then verify
```

---

### `completion`

Purpose: Generate a shell completion script for `istioctl-env` in the normal form for that shell.

The script completes subcommands and version arguments:

- For `install` and `resolve`, it offers **remote** versions from the release
  cache on disk. It gets them with `istioctl-env list-remote --cached`, which
  never uses the network.
- For `uninstall`, `shell`, `local`, `global`, and `exec`, it offers
  **installed** versions from `istioctl-env list`.
- After `completion`, it offers the shell names.

The script also completes flags such as `--prerelease`, `--cached`, `--silent`,
and `--shell` where they apply.

Syntax:

```text
istioctl-env completion [<shell>] [--shell <shell>]
```

`<shell>` is one of `bash`, `zsh`, `fish`, `powershell` (alias `pwsh`).

Options/flags:

- `--shell <name>`: the same as the positional `<shell>`. If you give neither,
  `istioctl-env` detects the shell from `$SHELL`. If detection fails, it
  falls back to `bash` and prints a `WARN` to stderr.
- `-h`, `--help`: show command help and exit

Environment variables:

- `SHELL` (read for auto-detection)

Exit codes:

- `0` on success.
- `1` if the shell name is unknown.

Example:

```sh
# Bash — enable for the current session or persist in ~/.bashrc
source <(istioctl-env completion bash)

# Zsh — compinit must have run first (usually via your framework)
autoload -Uz compinit && compinit
source <(istioctl-env completion zsh)

# Fish — one-time install
istioctl-env completion fish > ~/.config/fish/completions/istioctl-env.fish
```

```powershell
# PowerShell — add to $PROFILE
istioctl-env completion powershell | Out-String | Invoke-Expression
```

Deprecated alias: `istioctl-env autocompletion` accepts the same arguments. It
prints `istioctl-env: 'autocompletion' is deprecated; use 'istioctl-env completion'`
to stderr. Help and completion lists do not show it.
