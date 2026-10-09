# istioctl-env

A version manager for [istioctl](https://istio.io/latest/docs/reference/commands/istioctl/) CLI, similar to [tfenv](https://github.com/tfutils/tfenv) and [pyenv](https://github.com/pyenv/pyenv).

Manage multiple versions of the istioctl CLI and switch between them seamlessly.

## Documentation

Full documentation site: <https://mmpyro.github.io/istioctl-env/>

- [Docs index](docs/index.md)
- [Installation and configuration](docs/installation-and-configuration.md)
- [CLI reference](docs/cli-reference.md)
- [Caching strategy](docs/caching.md)

## Downloads (latest tag v1.0.0)

Latest tag v1.0.0: **v1.0.0** (tag: `v1.0.0`).

| Platform | Asset | Download |
|---|---|---|
| macOS (Intel) | `istioctl-env-darwin-amd64` | [Download](https://github.com/mmpyro/istioctl-env/releases/download/v1.0.0/istioctl-env-darwin-amd64) |
| macOS (Apple Silicon) | `istioctl-env-darwin-arm64` | [Download](https://github.com/mmpyro/istioctl-env/releases/download/v1.0.0/istioctl-env-darwin-arm64) |
| Linux (x86_64) | `istioctl-env-linux-amd64` | [Download](https://github.com/mmpyro/istioctl-env/releases/download/v1.0.0/istioctl-env-linux-amd64) |
| Linux (ARM64) | `istioctl-env-linux-arm64` | [Download](https://github.com/mmpyro/istioctl-env/releases/download/v1.0.0/istioctl-env-linux-arm64) |

## Features

- Install and manage multiple istioctl CLI versions
- Automatic version switching based on project directory (`.istioctl-version`)
- Shell-level, local (directory), and global version configuration
- Version priority: shell > local > global
- SemVer constraints in `.istioctl-version` (`~1.24.0`, `^1.24.0`,
  `>=1.24.0 <1.26.0`, `latest`, `latest-prerelease`) resolved by the shim
- Opt-in auto-install (`ISTIOENV_AUTO_INSTALL=true`) for fresh checkouts
- Auto-detection of OS and architecture for binary downloads
- Shim-based transparent proxying of `istioctl` commands

## Installation

### From Source

```sh
git clone https://github.com/mmpyro/istioctl-env.git
cd istioctl-env
make build
```

The binary will be at `build/istioctl-env`. Move it to a directory in your `PATH`:

```sh
sudo mv build/istioctl-env /usr/local/bin/
```

### Cross-compile for All Platforms

```sh
make build-all
```

This produces binaries for:
- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

## Quick Start

### 1. Set up ISTIOENV_ROOT

Add to your `~/.bashrc` or `~/.zshrc`:

```sh
export ISTIOENV_ROOT="$HOME/.istioenv"
```

Reload your shell:

```sh
source ~/.bashrc  # or source ~/.zshrc
```

### 2. Initialize istioctl-env

Add the following to your `~/.bashrc` or `~/.zshrc` (after the `ISTIOENV_ROOT` export):

```sh
eval "$(istioctl-env init)"
```

This sets up:
- A `istioctl-env` shell function for `istioctl-env shell` support
- PATH prepend for the istioctl shim

### 3. Install an istioctl version

```sh
# Install a specific version
istioctl-env install 1.24.0

# Install the latest stable version
istioctl-env install
```

### 4. Set a version

```sh
# Set global default
istioctl-env global 1.24.0

# Set for current directory (creates .istioctl-version)
istioctl-env local 1.24.0

# Set for current shell session
istioctl-env shell 1.24.0
```

### 5. Use istioctl

```sh
istioctl version
```

The shim automatically resolves and uses the correct version.

## Command Reference

Full reference: [docs/cli-reference.md](docs/cli-reference.md)

| Command | Description |
|---------|-------------|
| `istioctl-env help` | Display help and all available commands |
| `istioctl-env list` | List all installed versions |
| `istioctl-env list --disk-usage` | List installed versions with on-disk size and a TOTAL row |
| `istioctl-env list-remote` | List all available istioctl versions from GitHub |
| `istioctl-env list-remote --prerelease` | Include pre-release istioctl versions |
| `istioctl-env latest` | Print the latest available version of istioctl from GitHub |
| `istioctl-env init` | Initialize istioctl-env setup |
| `istioctl-env status` | Show current environment status |
| `istioctl-env install [VERSION]` | Install a specific version (or latest) |
| `istioctl-env uninstall VERSION` | Uninstall a specific version |
| `istioctl-env prune` | Dry-run removal of unreferenced versions (use `--yes` to apply) |
| `istioctl-env exec VERSION CMD` | Run a command using a specific istioctl version |
| `istioctl-env shell [VERSION]` | Set/show shell version (`ISTIOENV_VERSION`) |
| `istioctl-env local [VERSION]` | Set/show local version (`.istioctl-version`) |
| `istioctl-env global [VERSION]` | Set/show global version (`$ISTIOENV_ROOT/version`) |
| `istioctl-env which` | Print path to active istioctl binary |
| `istioctl-env doctor [--fix] [--deep]` | Diagnose the environment and print OK/WARN/FAIL per check (optionally auto-repair; `--deep` also re-verifies installed binaries against GitHub checksums) |
| `istioctl-env resolve [--install]` | Resolve the active expression to a concrete version |
| `istioctl-env version` | Print istioctl-env version |

### Global flags

Every command accepts the following top-level flags (equivalent env vars shown in parentheses). They may appear before or after the subcommand.

| Flag | Env var | Purpose |
|------|---------|---------|
| `--offline` | `ISTIOENV_OFFLINE=1` | Never contact the network. `list-remote`/`latest` serve from cache/baseline; `install` fails fast with a clear message. |
| `--github-token <token>` | `ISTIOENV_GITHUB_TOKEN` (fallback: `GITHUB_TOKEN`) | GitHub token used for API requests and same-origin asset downloads (5 000/hr instead of 60/hr/IP). |
| `--api-mirror <url>` | `ISTIOENV_API_MIRROR` | Override `api.github.com` (point at an internal mirror). |
| `--download-mirror <url>` | `ISTIOENV_DOWNLOAD_MIRROR` (legacy alias: `ISTIOENV_MIRROR_URL`) | Override `github.com` for release-asset downloads. |

See [docs/installation-and-configuration.md](docs/installation-and-configuration.md#environment-variables) for full details.

## Version Priority

When `istioctl` is invoked, the version expression is resolved in this order:

1. **Shell** — `ISTIOENV_VERSION` environment variable (set via `istioctl-env shell`)
2. **Local** — `.istioctl-version` file in the current or parent directories (set via `istioctl-env local`)
3. **Global** — `$ISTIOENV_ROOT/version` file (set via `istioctl-env global`)

The value may be an exact pin or any SemVer constraint. See
[Version expressions](docs/cli-reference.md#version-expressions) for the full
grammar; a quick summary:

| Expression | Example resolves to |
|---|---|
| `1.24.0` | `1.24.0` (exact) |
| `~1.24.0` | newest installed `1.24.x` |
| `^1.24.0` | newest installed `1.x` matching `>=1.24.0` |
| `>=1.24.0 <1.26.0` | newest installed in that interval |
| `latest` | newest installed stable version |
| `latest-prerelease` | newest installed (incl. pre-releases) |

If no version is configured at any level, the command fails with an informative error.

### Auto-install

Set `ISTIOENV_AUTO_INSTALL=true` in your shell to have the shim install
missing versions on first use:

```sh
export ISTIOENV_AUTO_INSTALL=true
```

This mirrors `nvm`'s auto-install behaviour and makes a fresh clone of a
repository with a `.istioctl-version` file work out of the box.

## Shell Setup

Both `istioctl-env init` and `istioctl-env autocompletion` auto-detect your
shell from `$SHELL` and emit code that is idiomatic for it (`eval`-able POSIX
code for bash/zsh, `source`-able fish code). You can also force a specific
shell with `--shell <bash|zsh|fish>`.

### Bash

Add the following to your `~/.bashrc`:

```sh
# istioctl-env setup
export ISTIOENV_ROOT="$HOME/.istioenv"
eval "$(istioctl-env init --shell bash)"
source <(istioctl-env autocompletion --shell bash)
```

### Zsh

Add the following to your `~/.zshrc`:

```sh
# istioctl-env setup
export ISTIOENV_ROOT="$HOME/.istioenv"
eval "$(istioctl-env init --shell zsh)"
# Make sure compinit has run before sourcing the completion script
autoload -Uz compinit && compinit
source <(istioctl-env autocompletion --shell zsh)
```

The zsh completion uses `_arguments` with per-subcommand descriptions, and
completes **remote versions** (served from the on-disk release cache) for
`istioctl-env install <TAB>`, and **installed versions** for `uninstall`,
`shell`, `local`, `global`, and `exec`.

### Fish

Install the completion into fish's completion directory, and source the init
code from your fish config:

```fish
# ~/.config/fish/config.fish
set -gx ISTIOENV_ROOT "$HOME/.istioenv"
istioctl-env init --shell fish | source

# One-time: install the completion script
istioctl-env autocompletion --shell fish > ~/.config/fish/completions/istioctl-env.fish
```

## Caching

`istioctl-env` uses a three-layer caching strategy to keep `list-remote` and `latest` fast and reliable. For more details on how it works and how to configure it, see [docs/caching.md](docs/caching.md).

## Directory Structure

```
$ISTIOENV_ROOT/
├── versions/           # Installed istioctl versions
│   ├── 1.24.0/
│   │   └── istioctl    # istioctl binary
│   └── 1.25.0/
│       └── istioctl
├── shims/
│   └── istioctl        # Shim script (auto-generated)
└── version             # Global version file
```

## Development

### Run Tests

```sh
make test
```

### Run Docker Integration Tests

```sh
make test-docker
```

### Build

```sh
make build          # Current platform
make build-all      # All platforms
```

## License

See [LICENSE](LICENSE) for details.
