# Installation and configuration

This guide covers:

- Supported platforms.
- Installation methods.
- The required setup steps.
- How `istioctl-env` finds its configuration.
- Common troubleshooting.

## Supported platforms

The project currently builds prebuilt binaries for:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`

If your platform is not in this list, install from source.

## Prerequisites

- A supported shell (`bash`, `zsh`, or `fish`). The POSIX shim itself works in any `/bin/sh`-compatible shell. The `istioctl-env` shell integration gives full support only to these three shells.
- Permission to create and write files in the `ISTIOENV_ROOT` directory that you choose.
- Network access to GitHub:
  - `istioctl-env install`, `istioctl-env latest`, and `istioctl-env list-remote` get information from GitHub.

You do not need an existing `istioctl` installation. `istioctl-env` manages the `istioctl` binaries that it installs.

## Install methods

### Option A: Install a prebuilt binary (recommended)

1. Download the binary for your platform from the GitHub releases of the project.
2. Make the binary executable and move it into a directory on your `PATH`.

Example (Linux x86_64):

```sh
curl -L -o istioctl-env https://github.com/mmpyro/istioctl-env/releases/download/v1.2.0/istioctl-env-linux-amd64
chmod +x istioctl-env
sudo mv istioctl-env /usr/local/bin/istioctl-env
```

### Option B: Build from source

```sh
git clone https://github.com/mmpyro/istioctl-env.git
cd istioctl-env
make build
```

The build writes the binary to `build/istioctl-env`.

### Option C: Cross-compile for all supported platforms

```sh
make build-all
```

## Initial setup

### 1) Choose and export `ISTIOENV_ROOT`

`ISTIOENV_ROOT` is required. It sets where `istioctl-env` stores installed versions, the shim, and the global version file.

Add this line to your shell profile (e.g. `~/.bashrc` or `~/.zshrc`):

```sh
export ISTIOENV_ROOT="$HOME/.istioenv"
```

Required permissions:

- `istioctl-env init` creates directories under `$ISTIOENV_ROOT`.
- `istioctl-env install` writes `istioctl` binaries under `$ISTIOENV_ROOT/versions/<version>/istioctl`.
- `istioctl-env global` writes `$ISTIOENV_ROOT/version`.

### 2) Initialize shell integration

Add this line after the `ISTIOENV_ROOT` export. `istioctl-env init` detects
your shell from `$SHELL` and prints the matching integration code. To select
a specific shell, use `--shell <bash|zsh|fish>`.

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

The integration code does two things:

- It puts `$ISTIOENV_ROOT/shims` at the front of your `PATH`, so `istioctl` resolves to the shim.
- It defines an `istioctl-env` shell function. This function lets `istioctl-env shell` change the environment of the current shell.

Sometimes `istioctl-env` cannot detect your shell, for example in a
non-interactive environment where `$SHELL` is unset. Then it falls back to the
bash integration. It also prints a `WARN` to stderr that tells you to give
`--shell` explicitly.

Shell completion follows the same pattern. Generate the script with
`istioctl-env completion <bash|zsh|fish|powershell>`. The zsh script uses
`_arguments` with a description for each subcommand. For
`istioctl-env install <TAB>`, it completes **remote** versions from the
release cache on disk. For `uninstall`, `shell`, `local`, `global`, and `exec`,
it completes installed versions. See [CLI reference / completion](cli-reference.md#completion).

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

- Global default. It applies everywhere, unless a local or shell version overrides it:

  ```sh
  istioctl-env global 1.24.0
  ```

- Per directory. The command creates `.istioctl-version` in the current directory:

  ```sh
  istioctl-env local 1.24.0
  ```

- Per shell session. The command sets `ISTIOENV_VERSION`. It needs `eval "$(istioctl-env init)"`:

  ```sh
  istioctl-env shell 1.24.0
  ```

## How configuration is discovered/loaded

When the `istioctl` shim runs, it selects a version in this priority order:

1. `ISTIOENV_VERSION` (shell version)
2. `.istioctl-version` in the current directory or any parent directory (local version)
3. `$ISTIOENV_ROOT/version` (global version)

Notes:

- The shim looks for `.istioctl-version` in each parent directory, up to the filesystem root.
- The shim treats all version values as strings and trims the whitespace.

## Common troubleshooting

### `ISTIOENV_ROOT not set`

Symptoms:

- `istioctl-env init` prints instructions and exits with an error.

Fix:

```sh
export ISTIOENV_ROOT="$HOME/.istioenv"
```

Then make sure that you also have:

```sh
eval "$(istioctl-env init)"
```

### `istioctl-env: no istioctl version configured`

The `istioctl` shim prints this message when no version source has a version.

Fix (choose one):

```sh
istioctl-env global 1.24.0
istioctl-env local 1.24.0
istioctl-env shell 1.24.0
```

### `version <X> not installed`

Before `global`, `local`, or `shell` sets a version, `istioctl-env` makes sure that the version is installed. The shim also makes sure that the installed binary is present.

Fix:

```sh
istioctl-env install <X>
```

### GitHub API rate limit exceeded

Some commands query GitHub releases. If GitHub returns `403`, `istioctl-env` reports a rate limit error.

Fixes:

- Wait and try again later.
- In CI or other heavily automated use, think about fewer `list-remote` / `latest` calls.
- Give a GitHub token to raise the rate limit. See "GitHub authentication" below.

## Environment variables

These environment variables change authentication, networking, caching, and version management. All of them are optional. Some have a matching global CLI flag (see the [CLI reference](cli-reference.md#global-flags)). For those, the env var and the flag do the same thing. If you give both, the flag wins.

| Variable | Flag | Purpose | Default |
|----------|------|---------|---------|
| `ISTIOENV_GITHUB_TOKEN` | `--github-token` | Preferred GitHub token. `istioctl-env` sends it as `Authorization: Bearer …` on API requests and on release-asset downloads from the same origin. It removes the token automatically on cross-origin redirects (for example, to the S3 CDN that GitHub redirects to with a 302). | unset |
| `GITHUB_TOKEN` | (none) | Fallback token when `ISTIOENV_GITHUB_TOKEN` is empty. | unset |
| `ISTIOENV_OFFLINE` | `--offline` | Set to `1` / `true` / `yes` / `on` to disable all outbound HTTP. | unset |
| `ISTIOENV_API_MIRROR` | `--api-mirror` | Base URL that replaces `https://api.github.com`. The mirror must speak the GitHub REST API. | unset |
| `ISTIOENV_DOWNLOAD_MIRROR` | `--download-mirror` | Base URL that replaces `https://github.com` in release-asset URLs. It does not change the API host. | unset |
| `ISTIOENV_MIRROR_URL` | (none) | **Deprecated** alias of `ISTIOENV_DOWNLOAD_MIRROR`. It stays for backward compatibility. Use the new name in new deployments. | unset |
| `ISTIOENV_AUTO_INSTALL` | (none) | Set to `1` / `true` / `yes` / `on` to let the `istioctl` shim, `resolve` and `exec` install missing versions automatically (`exec --no-auto` overrides it). | unset |
| `ISTIOENV_CACHE_TTL` | (none) | How long the release cache on disk stays fresh, as a Go duration (`30m`, `24h`, `0s` to always fetch again). See [caching](caching.md). | `1h` |
| `ISTIOENV_PRUNE_SCAN_ROOTS` | (none) | Path list (`:`-separated on Unix) of directories that `prune` scans for `.istioctl-version` files. Set to `""` to disable scanning. | `$HOME` |

### GitHub authentication

`istioctl-env` reads `ISTIOENV_GITHUB_TOKEN` first. If it is empty, `istioctl-env` uses `GITHUB_TOKEN`. When a token is present, each GitHub API request includes:

```
Authorization: Bearer <token>
X-GitHub-Api-Version: 2022-11-28
```

Release-asset downloads also send the token, but only on the **initial, same-origin** request. That is, only when the host of the download URL matches the configured download base. The download base is `github.com` by default, or your `ISTIOENV_DOWNLOAD_MIRROR` host. On each cross-origin hop, the redirect handler of the HTTP client removes `Authorization`. Thus the token can never leak to the GitHub object CDN (`objects.githubusercontent.com`) or to a third-party host that your mirror redirects to.

Typical uses:

- Avoid the anonymous rate limit of 60 requests/hour (5 000/hr authenticated) when you run `list-remote`, `latest`, or `install` frequently. Examples are CI, or a corporate NAT that shares one IP across users.
- Download assets from a private `istio/istio` fork or from an internal mirror that requires authentication.

```sh
export ISTIOENV_GITHUB_TOKEN="ghp_…"
# or, per-invocation:
istioctl-env --github-token "$CI_GITHUB_TOKEN" install 1.24.0
```

### Offline mode

To stop all outbound HTTP calls from `istioctl-env`, set `ISTIOENV_OFFLINE=1` (or pass `--offline`).

Behavior:

- `list-remote`, `latest`: these commands use the cache on disk. If no cache exists, they use the hardcoded baseline. They print no warning, because the fallback is intentional. See [caching](caching.md#5-offline-mode).
- `install <version>` with an explicit version: the command continues to the download step. If `ISTIOENV_DOWNLOAD_MIRROR` is set, it uses the mirror. If not, the download fails at once with a clear error that points you to `ISTIOENV_DOWNLOAD_MIRROR`. There is no 30-second connect timeout.
- `install` (no version): the command refuses to run with `cannot determine latest version while ISTIOENV_OFFLINE=1`.
- `upgrade`: the command exits cleanly with `istioctl-env: offline mode (ISTIOENV_OFFLINE=1); skipping upgrade` and exit code `0`.

```sh
export ISTIOENV_OFFLINE=1
istioctl-env install 1.24.0           # works if ISTIOENV_DOWNLOAD_MIRROR is set
istioctl-env list-remote              # works, served from cache / baseline
istioctl-env upgrade                  # no-op, exit 0
```

### Custom mirrors

Two independent env vars (and matching flags) let you redirect each host:

- **`ISTIOENV_API_MIRROR`** replaces `https://api.github.com`. Use it when your runners cannot reach the public GitHub API, for example in regulated environments or air-gapped networks with an API proxy. The mirror must serve the GitHub REST API routes that `istioctl-env` uses:
  - `GET /repos/istio/istio/releases`
  - `GET /repos/istio/istio/releases/latest`
  - `GET /repos/<owner>/istioctl-env/releases/latest` (only for `istioctl-env upgrade`)
- **`ISTIOENV_DOWNLOAD_MIRROR`** replaces `https://github.com` in release-asset URLs. The mirror MUST serve the per-file `.sha256` checksum at the same relative path as GitHub releases. `istioctl-env` downloads the checksum from `<archive-url>.sha256`. If the checksum does not match, it refuses the install.

`istioctl-env` removes trailing slashes from both values automatically.

Example (both configured):

```sh
export ISTIOENV_API_MIRROR="https://ghapi.corp.example"
export ISTIOENV_DOWNLOAD_MIRROR="https://mirror.corp.example/istio"
istioctl-env install 1.24.0
```

With these values, the download URL is:

```
https://mirror.corp.example/istio/istio/istio/releases/download/1.24.0/istioctl-1.24.0-<os>-<arch>.tar.gz
```

If you combine both mirrors with `ISTIOENV_OFFLINE=1`, you can do fully air-gapped installs. `istioctl-env` never contacts public GitHub. All archive, checksum, and API traffic goes through your internal mirrors.

#### Legacy `ISTIOENV_MIRROR_URL`

Earlier versions had a single `ISTIOENV_MIRROR_URL` env var. It changed only the download host. `istioctl-env` still accepts it, and it still affects only downloads. In new deployments, use `ISTIOENV_DOWNLOAD_MIRROR` instead (and `ISTIOENV_API_MIRROR` for the API host). If both are set, `ISTIOENV_DOWNLOAD_MIRROR` wins.
