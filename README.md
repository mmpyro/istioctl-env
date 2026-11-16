# istioctl-env

`istioctl-env` is a version manager for the [istioctl](https://istio.io/latest/docs/reference/commands/istioctl/) CLI, similar to [tfenv](https://github.com/tfutils/tfenv) and [pyenv](https://github.com/pyenv/pyenv).

Use it to install several versions of the istioctl CLI and to switch between them.

## Documentation

Full documentation site: <https://mmpyro.github.io/istioctl-env/>

- [Docs index](docs/index.md)
- [Installation and configuration](docs/installation-and-configuration.md)
- [CLI reference](docs/cli-reference.md)
- [Caching strategy](docs/caching.md)

## Downloads (latest release v1.2.0)

The latest release is **v1.2.0** (tag: `v1.2.0`).

| Platform | Asset | Download |
|---|---|---|
| macOS (Intel) | `istioctl-env-darwin-amd64` | [Download](https://github.com/mmpyro/istioctl-env/releases/download/v1.2.0/istioctl-env-darwin-amd64) |
| macOS (Apple Silicon) | `istioctl-env-darwin-arm64` | [Download](https://github.com/mmpyro/istioctl-env/releases/download/v1.2.0/istioctl-env-darwin-arm64) |
| Linux (x86_64) | `istioctl-env-linux-amd64` | [Download](https://github.com/mmpyro/istioctl-env/releases/download/v1.2.0/istioctl-env-linux-amd64) |
| Linux (ARM64) | `istioctl-env-linux-arm64` | [Download](https://github.com/mmpyro/istioctl-env/releases/download/v1.2.0/istioctl-env-linux-arm64) |

## Features

- Install and manage multiple istioctl CLI versions.
- Switch the version automatically from the project directory (`.istioctl-version`).
- Set the version for the shell, for a directory (local) or globally.
- Version priority: shell > local > global.
- Use SemVer constraints in `.istioctl-version` (`~1.24.0`, `^1.24.0`,
  `>=1.24.0 <1.26.0`, `latest`, `latest-prerelease`). The shim resolves them.
- Enable auto-install (`ISTIOENV_AUTO_INSTALL=true`) for fresh checkouts. It is off by default.
- Detect the OS and architecture automatically for binary downloads.
- Run `istioctl` commands through a transparent shim.

## Installation

### From source

```sh
git clone https://github.com/mmpyro/istioctl-env.git
cd istioctl-env
make build
```

The build writes the binary to `build/istioctl-env`. Move it to a directory in your `PATH`:

```sh
sudo mv build/istioctl-env /usr/local/bin/
```

### Cross-compile for all platforms

```sh
make build-all
```

This command builds binaries for:

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

## Quick start

### 1. Set ISTIOENV_ROOT

Add this line to your `~/.bashrc` or `~/.zshrc`:

```sh
export ISTIOENV_ROOT="$HOME/.istioenv"
```

Reload your shell:

```sh
source ~/.bashrc  # or source ~/.zshrc
```

### 2. Initialize istioctl-env

Add this line to your `~/.bashrc` or `~/.zshrc`, after the `ISTIOENV_ROOT` export:

```sh
eval "$(istioctl-env init)"
```

This line sets up:

- An `istioctl-env` shell function, which `istioctl-env shell` needs.
- The istioctl shim at the front of your PATH.

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

The shim finds the correct version and runs it.

## Command reference

Full reference: [docs/cli-reference.md](docs/cli-reference.md)

| Command | Description |
|---------|-------------|
| `istioctl-env help` | Show help and all available commands |
| `istioctl-env list` | List all installed versions |
| `istioctl-env list --disk-usage` | List installed versions with their size on disk and a TOTAL row |
| `istioctl-env list-remote` | List all available istioctl versions from GitHub |
| `istioctl-env list-remote --prerelease` | Include pre-release istioctl versions |
| `istioctl-env list-remote --cached` | List versions from the release cache on disk only (no network) |
| `istioctl-env latest` | Print the latest available version of istioctl from GitHub |
| `istioctl-env init` | Initialize the istioctl-env setup |
| `istioctl-env status` | Show the status of the current environment |
| `istioctl-env install [VERSION] [-s\|--silent]` | Install a specific version, a constraint or the latest version. `--silent` hides the progress |
| `istioctl-env uninstall VERSION` | Uninstall a specific version |
| `istioctl-env prune [--keep-last N] [--older-than DUR] [--dry-run] [--yes]` | Remove versions that nothing references. It is a dry run unless you give `--yes` |
| `istioctl-env exec [--auto\|--no-auto] VERSION [--] CMD [ARGS...]` | Run istioctl at a version or constraint. It returns the exit code of istioctl |
| `istioctl-env shell [VERSION]` | Set or show the shell version (`ISTIOENV_VERSION`) |
| `istioctl-env local [VERSION]` | Set or show the local version (`.istioctl-version`) |
| `istioctl-env global [VERSION]` | Set or show the global version (`$ISTIOENV_ROOT/version`) |
| `istioctl-env which` | Print the path to the active istioctl binary |
| `istioctl-env doctor [--fix] [--deep]` | Diagnose the environment and print OK, WARN or FAIL for each check. `--fix` repairs what it can. `--deep` also checks installed binaries against the GitHub checksums again |
| `istioctl-env resolve [SPEC] [--install] [-s\|--silent]` | Resolve SPEC to a concrete version. The default SPEC is the active expression |
| `istioctl-env upgrade` | Upgrade istioctl-env itself to the latest release |
| `istioctl-env completion [SHELL]` | Print a completion script for bash, zsh, fish or powershell |
| `istioctl-env version` | Print the istioctl-env version |

Every command accepts `-h`/`--help`.

### Global flags

Every command accepts these top-level flags. The table shows the matching environment variable for each flag. The flags can come before or after the subcommand.

| Flag | Env var | Purpose |
|------|---------|---------|
| `--offline` | `ISTIOENV_OFFLINE=1` | Never contact the network. `list-remote` and `latest` use the cache or the baseline. `install` fails at once with a clear message. |
| `--github-token <token>` | `ISTIOENV_GITHUB_TOKEN` (fallback: `GITHUB_TOKEN`) | GitHub token for API requests and for asset downloads from the same origin. It raises the limit from 60/hr per IP to 5 000/hr. |
| `--api-mirror <url>` | `ISTIOENV_API_MIRROR` | Replace `api.github.com`, for example with an internal mirror. |
| `--download-mirror <url>` | `ISTIOENV_DOWNLOAD_MIRROR` (legacy alias: `ISTIOENV_MIRROR_URL`) | Replace `github.com` for release-asset downloads. |

For full details, see [docs/installation-and-configuration.md](docs/installation-and-configuration.md#environment-variables).

## Version priority

When you run `istioctl`, the shim resolves the version expression in this order:

1. Shell: the `ISTIOENV_VERSION` environment variable. `istioctl-env shell` sets it.
2. Local: the `.istioctl-version` file in the current directory or a parent directory. `istioctl-env local` sets it.
3. Global: the `$ISTIOENV_ROOT/version` file. `istioctl-env global` sets it.

The value can be an exact pin or any SemVer constraint. For the full
grammar, see [Version expressions](docs/cli-reference.md#version-expressions).
A short summary:

| Expression | Example resolves to |
|---|---|
| `1.24.0` | `1.24.0` (exact) |
| `~1.24.0` | newest installed `1.24.x` |
| `^1.24.0` | newest installed `1.x` matching `>=1.24.0` |
| `>=1.24.0 <1.26.0` | newest installed in that interval |
| `latest` | newest installed stable version |
| `latest-prerelease` | newest installed (incl. pre-releases) |

If no level has a version, the command fails with an error that explains the problem.

### Auto-install

To make the shim install missing versions on first use, set
`ISTIOENV_AUTO_INSTALL=true` in your shell:

```sh
export ISTIOENV_AUTO_INSTALL=true
```

This works like the auto-install of `nvm`. With it, a fresh clone of a
repository that has a `.istioctl-version` file works at once.

## Shell setup

`istioctl-env init` and `istioctl-env completion` both detect your shell
from `$SHELL`. They print code in the normal form for that shell: POSIX code
for bash and zsh that you can `eval`, and fish code that you can `source`.
You can also select a shell yourself with `init --shell <bash|zsh|fish>` or
`completion <bash|zsh|fish|powershell>`. `completion --shell <name>` also works.

`istioctl-env autocompletion` still works as a deprecated alias of
`completion`. It prints a deprecation warning to stderr.

### Bash

Add these lines to your `~/.bashrc`:

```sh
# istioctl-env setup
export ISTIOENV_ROOT="$HOME/.istioenv"
eval "$(istioctl-env init --shell bash)"
source <(istioctl-env completion bash)
```

### Zsh

Add these lines to your `~/.zshrc`:

```sh
# istioctl-env setup
export ISTIOENV_ROOT="$HOME/.istioenv"
eval "$(istioctl-env init --shell zsh)"
# Make sure compinit has run before sourcing the completion script
autoload -Uz compinit && compinit
source <(istioctl-env completion zsh)
```

The zsh completion uses `_arguments` with a description for each subcommand.
For `istioctl-env install <TAB>` and `resolve <TAB>`, it completes **remote versions**
from the release cache on disk. For `uninstall`, `shell`, `local`, `global`
and `exec`, it completes **installed versions**. The bash, fish and
PowerShell scripts do the same.

### Fish

Install the completion script into the fish completion directory. Then source
the init code from your fish config:

```fish
# ~/.config/fish/config.fish
set -gx ISTIOENV_ROOT "$HOME/.istioenv"
istioctl-env init --shell fish | source

# One-time: install the completion script
istioctl-env completion fish > ~/.config/fish/completions/istioctl-env.fish
```

### PowerShell

Add this line to your `$PROFILE`:

```powershell
istioctl-env completion powershell | Out-String | Invoke-Expression
```

## Caching

`istioctl-env` uses a cache with three layers to keep `list-remote` and `latest` fast and reliable. For details on how the cache works and how to configure it, see [docs/caching.md](docs/caching.md).

## Directory structure

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

### Run tests

```sh
make test
```

### Run Docker integration tests

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
