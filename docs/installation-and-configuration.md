# Installation and configuration

This guide covers supported platforms, installation methods, required setup steps, how `istioctl-env` discovers configuration, and common troubleshooting.

## Supported platforms

Prebuilt binaries are currently produced for:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`

If your platform is not listed, install from source.

## Prerequisites

- A supported shell (`bash`, `zsh`, or `fish`). The POSIX shim itself works in any `/bin/sh`-compatible shell; the `istioctl-env` shell integration is first-class for the three listed shells.
- Permission to create and write files in your chosen `ISTIOENV_ROOT` directory.
- Network access to GitHub:
  - `istioctl-env install`, `istioctl-env latest`, and `istioctl-env list-remote` fetch information from GitHub.

No existing `istioctl` installation is required; `istioctl-env` manages the `istioctl` binaries it installs.

## Install methods

### Option A: Install a prebuilt binary (recommended)

1. Download the binary for your platform from the project's GitHub tag v1.0.0.
2. Make it executable and move it into a directory on your `PATH`.

Example (Linux x86_64):

```sh
curl -L -o istioctl-env https://github.com/mmpyro/istioctl-env/releases/download/v1.0.0/istioctl-env-linux-amd64
chmod +x istioctl-env
sudo mv istioctl-env /usr/local/bin/istioctl-env
```

### Option B: Build from source

```sh
git clone https://github.com/mmpyro/istioctl-env.git
cd istioctl-env
make build
```

The binary will be available at `build/istioctl-env`.

### Option C: Cross-compile for all supported platforms

```sh
make build-all
```

## Initial setup

### 1) Choose and export `ISTIOENV_ROOT`

`ISTIOENV_ROOT` is required. It defines where `istioctl-env` stores installed versions, the shim, and the global version file.

Add this to your shell profile (e.g. `~/.bashrc` or `~/.zshrc`):

```sh
export ISTIOENV_ROOT="$HOME/.istioenv"
```

Required permissions:

- `istioctl-env init` creates directories under `$ISTIOENV_ROOT`.
- `istioctl-env install` writes `istioctl` binaries under `$ISTIOENV_ROOT/versions/<version>/istioctl`.
- `istioctl-env global` writes `$ISTIOENV_ROOT/version`.

### 2) Initialize shell integration

Add this after the `ISTIOENV_ROOT` export. `istioctl-env init` auto-detects
your shell from `$SHELL` and prints the matching integration code; you can
also force a specific shell with `--shell <bash|zsh|fish>`.

#### Bash (`~/.bashrc`)

```sh
eval "$(istioctl-env init)"
# or, to be explicit:
eval "$(istioctl-env init --shell bash)"
```

#### Zsh (`~/.zshrc`)

```sh
eval "$(istioctl-env init --shell zsh)"
```

#### Fish (`~/.config/fish/config.fish`)

```fish
istioctl-env init --shell fish | source
```

What this does:

- Prepends `$ISTIOENV_ROOT/shims` to your `PATH` so `istioctl` resolves to the shim.
- Defines an `istioctl-env` shell function that enables `istioctl-env shell` to affect the current shell environment.

If `istioctl-env` cannot detect your shell (for example in a non-interactive
environment where `$SHELL` is unset), it falls back to the bash integration
and prints a `WARN` to stderr suggesting that you pass `--shell` explicitly.

Shell completion follows the same pattern. The zsh script uses `_arguments`
with per-subcommand descriptions and completes **remote** versions (served
from the on-disk release cache) for `istioctl-env install <TAB>`, while
`uninstall`, `shell`, `local`, `global`, and `exec` complete against
installed versions. Generate it with `istioctl-env completion <bash|zsh|fish|powershell>`;
see [CLI reference / completion](cli-reference.md#completion).

### 3) Install an `istioctl` version

```sh
istioctl-env install 1.24.0
```

Or install the latest stable version:

```sh
istioctl-env install
```

### 4) Configure which version to use

Pick one of:

- Global default (applies everywhere unless overridden):

  ```sh
  istioctl-env global 1.24.0
  ```

- Per-directory (creates `.istioctl-version` in the current directory):

  ```sh
  istioctl-env local 1.24.0
  ```

- Per-shell session (sets `ISTIOENV_VERSION`; requires `eval "$(istioctl-env init)"`):

  ```sh
  istioctl-env shell 1.24.0
  ```

## How configuration is discovered/loaded

When the `istioctl` shim runs, it selects a version using this priority order:

1. `ISTIOENV_VERSION` (shell version)
2. `.istioctl-version` in the current directory or any parent directory (local version)
3. `$ISTIOENV_ROOT/version` (global version)

Notes:

- The `.istioctl-version` lookup walks upward until the filesystem root.
- All version values are treated as strings and trimmed for whitespace.

## Common troubleshooting

### `ISTIOENV_ROOT not set`

Symptoms:

- `istioctl-env init` prints instructions and exits with an error.

Fix:

```sh
export ISTIOENV_ROOT="$HOME/.istioenv"
```

Then ensure you also have:

```sh
eval "$(istioctl-env init)"
```

### `istioctl-env: no istioctl version configured`

This is emitted by the `istioctl` shim when none of the version sources are configured.

Fix (choose one):

```sh
istioctl-env global 1.24.0
istioctl-env local 1.24.0
istioctl-env shell 1.24.0
```

### `version <X> not installed`

`istioctl-env` validates that a version is installed before setting it via `global`, `local`, or `shell`, and the shim also verifies the installed binary is present.

Fix:

```sh
istioctl-env install <X>
```

### GitHub API rate limit exceeded

Some commands query GitHub tag v1.0.0. If GitHub returns `403`, `istioctl-env` reports a rate limit error.

Fixes:

- Wait and retry later.
- If running in CI or heavily automated use, consider reducing frequency of `list-remote` / `latest` calls.
- Provide a GitHub token to raise the rate limit — see "GitHub authentication" below.

## Environment variables

The following environment variables influence authentication, networking, caching, and version management. All are optional. Where a matching global CLI flag exists (see the [CLI reference](cli-reference.md#global-flags)), env vars and flags are interchangeable; if both are given, the flag wins.

| Variable | Flag | Purpose | Default |
|----------|------|---------|---------|
| `ISTIOENV_GITHUB_TOKEN` | `--github-token` | Preferred GitHub token. Sent as `Authorization: Bearer …` on API requests and on same-origin release-asset downloads. Stripped automatically on cross-origin redirects (e.g. the S3 CDN GitHub 302s to). | unset |
| `GITHUB_TOKEN` | — | Fallback token used when `ISTIOENV_GITHUB_TOKEN` is empty. | unset |
| `ISTIOENV_OFFLINE` | `--offline` | Set to `1` / `true` / `yes` / `on` to disable all outbound HTTP. | unset |
| `ISTIOENV_API_MIRROR` | `--api-mirror` | Base URL that replaces `https://api.github.com`. The mirror must speak the GitHub REST API. | unset |
| `ISTIOENV_DOWNLOAD_MIRROR` | `--download-mirror` | Base URL that replaces `https://github.com` when building release-asset URLs. The API host is unaffected. | unset |
| `ISTIOENV_MIRROR_URL` | — | **Deprecated** alias of `ISTIOENV_DOWNLOAD_MIRROR`. Kept for backward compatibility; use the new name in new deployments. | unset |
| `ISTIOENV_AUTO_INSTALL` | — | Set to `1` / `true` / `yes` / `on` to let the `istioctl` shim, `resolve` and `exec` install missing versions automatically (`exec --no-auto` overrides it). | unset |
| `ISTIOENV_CACHE_TTL` | — | How long the on-disk release cache is fresh, as a Go duration (`30m`, `24h`, `0s` to always refetch). See [caching](caching.md). | `1h` |
| `ISTIOENV_PRUNE_SCAN_ROOTS` | — | Path list (`:`-separated on Unix) of directories `prune` scans for `.istioctl-version` files. Set to `""` to disable scanning. | `$HOME` |

### GitHub authentication

`istioctl-env` reads `ISTIOENV_GITHUB_TOKEN` first, then falls back to `GITHUB_TOKEN`. When a token is present, every outgoing GitHub API request includes:

```
Authorization: Bearer <token>
X-GitHub-Api-Version: 2022-11-28
```

Release-asset downloads also carry the token, but only on the **initial, same-origin** request — i.e. only when the download URL host matches the configured download base (`github.com` by default, or your `ISTIOENV_DOWNLOAD_MIRROR` host). The HTTP client's redirect handler strips `Authorization` on any cross-origin hop, so the token can never leak to GitHub's object CDN (`objects.githubusercontent.com`) or to a third-party host your mirror redirects to.

Typical uses:

- Avoid the 60-request/hour anonymous rate limit (5 000/hr authenticated) when running `list-remote`, `latest`, or `install` frequently (e.g. in CI, or behind a corporate NAT that shares an IP across users).
- Download assets from a private `istio/istio` fork or an authenticated internal mirror.

```sh
export ISTIOENV_GITHUB_TOKEN="ghp_…"
# or, per-invocation:
istioctl-env --github-token "$CI_GITHUB_TOKEN" install 1.24.0
```

### Offline mode

Set `ISTIOENV_OFFLINE=1` (or pass `--offline`) to prevent `istioctl-env` from making any outbound HTTP calls.

Behavior:

- `list-remote`, `latest` — served from the on-disk cache; if no cache exists, the hardcoded baseline is used. No warning is printed because the fallback is intentional. See [caching](caching.md#5-offline-mode).
- `install <version>` with an explicit version — proceeds to the download step. If `ISTIOENV_DOWNLOAD_MIRROR` is set the mirror is used; otherwise the download fails fast with a clear error pointing you at `ISTIOENV_DOWNLOAD_MIRROR` (no 30-second connect timeout).
- `install` (no version) — refuses to run with `cannot determine latest version while ISTIOENV_OFFLINE=1`.
- `upgrade` — exits cleanly with `istioctl-env: offline mode (ISTIOENV_OFFLINE=1); skipping upgrade` and exit code `0`.

```sh
export ISTIOENV_OFFLINE=1
istioctl-env install 1.24.0           # works if ISTIOENV_DOWNLOAD_MIRROR is set
istioctl-env list-remote              # works, served from cache / baseline
istioctl-env upgrade                  # no-op, exit 0
```

### Custom mirrors

Two independent env vars (and matching flags) let you redirect either host:

- **`ISTIOENV_API_MIRROR`** — replaces `https://api.github.com`. Use it when your runners can't reach the public GitHub API (regulated environments, air-gapped networks with an API proxy). The mirror must serve the GitHub REST API routes `istioctl-env` uses:
  - `GET /repos/istio/istio/releases`
  - `GET /repos/istio/istio/releases/latest`
  - `GET /repos/<owner>/istioctl-env/releases/latest` (only for `istioctl-env upgrade`)
- **`ISTIOENV_DOWNLOAD_MIRROR`** — replaces `https://github.com` when building release-asset URLs. The mirror MUST serve the per-file `.sha256` checksum at the same relative path as GitHub releases — `istioctl-env` downloads it using `<archive-url>.sha256` and refuses the install on mismatch.

Trailing slashes on both values are stripped automatically.

Example (both configured):

```sh
export ISTIOENV_API_MIRROR="https://ghapi.corp.example"
export ISTIOENV_DOWNLOAD_MIRROR="https://mirror.corp.example/istio"
istioctl-env install 1.24.0
```

The download URL above becomes:

```
https://mirror.corp.example/istio/istio/istio/releases/download/1.24.0/istioctl-1.24.0-<os>-<arch>.tar.gz
```

Combining both mirrors with `ISTIOENV_OFFLINE=1` enables fully air-gapped installs: `istioctl-env` will never contact public GitHub, and all archive, checksum, and API traffic flows through your internal mirrors.

#### Legacy `ISTIOENV_MIRROR_URL`

Earlier versions exposed a single `ISTIOENV_MIRROR_URL` env var that only overrode the download host. It is still accepted and still only affects downloads, but new deployments should prefer `ISTIOENV_DOWNLOAD_MIRROR` (and `ISTIOENV_API_MIRROR` for the API host). If both are set, `ISTIOENV_DOWNLOAD_MIRROR` wins.
