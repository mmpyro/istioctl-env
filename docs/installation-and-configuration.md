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

- A POSIX-like shell (e.g. `bash` or `zsh`).
- Permission to create and write files in your chosen `ISTIOENV_ROOT` directory.
- Network access to GitHub:
  - `istioctl-env install`, `istioctl-env latest`, and `istioctl-env list-remote` fetch information from GitHub.

No existing `istioctl` installation is required; `istioctl-env` manages the `istioctl` binaries it installs.

## Install methods

### Option A: Install a prebuilt binary (recommended)

1. Download the binary for your platform from the project's GitHub releases.
2. Make it executable and move it into a directory on your `PATH`.

Example (Linux x86_64):

```sh
curl -L -o istioctl-env https://github.com/mmpyro/istioctl-env/releases/download/v.0.1.0/istioctl-env-linux-amd64
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

Add this after the `ISTIOENV_ROOT` export.
Add this to your shell profile (e.g. `~/.bashrc` or `~/.zshrc`):
```sh
eval "$(istioctl-env init)"
```

What this does:

- Prepends `$ISTIOENV_ROOT/shims` to your `PATH` so `istioctl` resolves to the shim.
- Defines an `istioctl-env` shell function that enables `istioctl-env shell` to affect the current shell environment.

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

Some commands query GitHub releases. If GitHub returns `403`, `istioctl-env` reports a rate limit error.

Fixes:

- Wait and retry later.
- If running in CI or heavily automated use, consider reducing frequency of `list-remote` / `latest` calls.
