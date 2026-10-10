# CLAUDE.md

This file guides Claude Code (claude.ai/code) when working in this repository.

## Overview

`istioctl-env` is a version manager for the `istioctl` CLI, in the style of tfenv and pyenv.
- Versions are installed under `$ISTIOENV_ROOT`, conventionally `~/.istioenv` (**not** `~/.istioctl-env`).
- A generated POSIX shim at `$ISTIOENV_ROOT/shims/istioctl` picks the version on every call. Precedence: **shell** (`ISTIOENV_VERSION`) > **local** (`.istioctl-version`, searched upward from the current directory) > **global** (`$ISTIOENV_ROOT/version`).
- A version can be exact or a SemVer constraint: `~1.24.0`, `^1.24.0`, `>=1.24.0 <1.26.0`, `latest` or `latest-prerelease`.
- Also supports offline mode, GitHub tokens, mirrors, and a three-layer release cache.

Go 1.24.4, **standard library only**: no third-party dependencies and no `go.sum`. Keep it that way unless asked.

## Commands

```sh
make build            # build/istioctl-env  (ldflags: -X main.Version=$(VERSION))
make build-all        # linux/darwin × amd64/arm64 into build/
make test             # go test -v -race ./...
make test-docker      # BATS integration suite in Docker (needs Docker + network)
make install          # go install ./cmd/istioctl-env
make clean
make docs-build       # mkdocs build --strict into site/ (creates .venv-docs from docs/requirements.txt)
make docs-serve       # http://127.0.0.1:8000/istioctl-env/

go test -race -run 'TestName/subtest' -v ./internal/commands/   # single unit test
go vet ./... && golangci-lint run                               # lint (CI uses golangci-lint, default config)
```

## Architecture

**Dispatch is hand-rolled; there is no cobra.** `cmd/istioctl-env/main.go` handles it in two stages:
1. `applyGlobalFlags` removes `--offline`, `--github-token`, `--api-mirror` and `--download-mirror` from **anywhere** in argv and turns them into env vars. A `--` stops this.
2. A big `switch args[0]` parses flags by hand for each command and calls `commands.X(...)`.

Exit codes:
- 2 for a global flag error.
- 1 for a command error.

`main.go` also has `parseDurationShorthand`, which accepts `30d` and `2w` for `prune --older-than`. It has tests in `cmd/istioctl-env/main_test.go`.

| Package | Role |
|---|---|
| `internal/commands` | One file per subcommand, each exporting `X()` and `XHelp()`. Shared helpers live in `versions_shared.go` (`getRemoteVersions`, cache), `resolve_shared.go` (`ReadNearestLocalSpec`, `AutoInstallEnabled`, `resolveSpecFor{Install,Shim}`) and `disk.go`. `doctor.go` is a large set of checks returning OK/WARN/FAIL. `prune.go` scans for `.istioctl-version` files. `upgrade.go` does an atomic self-upgrade from `mmpyro/istioctl-env` |
| `internal/config` | `ISTIOENV_ROOT`, init checks ("initialized" means `versions/` exists), resolution (`ResolveVersion`, `ResolveInstalledVersion`), paths |
| `internal/semver` | `Parse`, `Less`, `SortDescending`. `constraint.go` provides `ParseConstraint`, `Match`, `HighestMatching` and `IsConstraint`. Ranges are space-separated |
| `internal/github` | `Client`. Lists releases (with paging and a delta `ListReleasesSince`) and downloads with progress. Handles the token, `ErrOffline` and mirrors, and drops the auth header on cross-origin redirects |
| `internal/cache` | JSON cache at `$ISTIOENV_ROOT/cache/releases.json`, TTL 1h (`ISTIOENV_CACHE_TTL`). `baseline.go` holds hard-coded known versions. See `docs/caching.md` |
| `internal/platform` | OS/arch detection and archive and checksum paths. `selfurl.go` provides the URL for self-upgrade |
| `internal/shim` | `GenerateShimScript`: its fast path runs the exact pinned binary directly, and its slow path calls `istioctl-env resolve --silent`. Also `DetectShell` and `GenerateShellInit{Bash,Zsh,Fish}` |

Runtime layout:
- `$ISTIOENV_ROOT/versions/<ver>/istioctl`
- `$ISTIOENV_ROOT/shims/istioctl` (generated; do not edit)
- `$ISTIOENV_ROOT/version`
- `$ISTIOENV_ROOT/cache/releases.json`

Only the **first line** of a version file is read.

### Adding or changing a command

1. Add a `case` in `cmd/istioctl-env/main.go`.
2. Add `X()` and `XHelp()` in `internal/commands/<cmd>.go`. If it touches the network, also add an `xWithClient(client *github.Client, ...)` variant and a `<cmd>_test.go`.
3. List it in `internal/commands/help.go`.
4. Add it to completions in `internal/commands/completion.go` (bash, zsh, fish and PowerShell scripts).
5. Update `docs/cli-reference.md` and the README.
6. Add BATS tests in `tests/integration.bats`.

## Testing

- **Unit tests** sit next to the code (`*_test.go`).
  - Use the standard `testing` package with `t.Run` subtests and `net/http/httptest` servers standing in for GitHub. Do not add assertion libraries.
  - Isolate each test with `t.Setenv("ISTIOENV_ROOT", t.TempDir())`.
  - Use the seams: `*WithClient` functions, and the `pinger`/`deepChecker` in doctor.
- **BATS integration tests** live in `tests/integration.bats` (48 tests, bats-support and bats-assert).
  - They run only in Docker through `Dockerfile.test`, with `scripts/run-bats.sh` as the entrypoint.
  - They hit real GitHub, including installing 1.24.0.

### Every user-facing change needs BATS coverage

This covers any new or changed command, flag (global flags included), env var, or resolution or shim behavior. Add or update `@test` cases in `tests/integration.bats`. Go unit tests alone are not enough for behavior the CLI shows to users.
- Follow the existing pattern. `setup()` gives a fresh `ISTIOENV_ROOT=$BATS_TMPDIR/istioenv` with the shims on PATH, and `teardown()` removes it.
- Call `istioctl-env init`, then `run istioctl-env ...`, then `assert_success` and `assert_output --partial "..."` (or `--regexp`).
- Cover the error path too: `assert_failure`, and check exit code 2 for global flag errors.

```bash
@test "istioctl-env <cmd> <does what>" {
	istioctl-env init
	run istioctl-env <cmd> <args>
	assert_success
	assert_output --partial "<expected>"
}
```

## Definition of done: final verification

Run these in order before calling any change complete:
1. `go vet ./...`, plus `golangci-lint run` if it is installed.
2. `make test`: unit tests with `-race`.
3. `make docs-build`, if you touched `docs/` or `mkdocs.yml`. Strict mode fails on broken links and anchors.
4. `make test-docker`: the **full BATS suite**. This is **always the last step**, and it needs Docker running and network access.

Report the real pass/fail counts from the output. If Docker is unavailable, say so plainly; do not claim success.

To iterate on one BATS test:
```sh
docker build -f Dockerfile.test -t istioctl-env-test .
docker run --rm -e TERM=xterm --entrypoint bats istioctl-env-test --filter "<test name regex>" /src/tests/integration.bats
```
Always finish with a full `make test-docker` afterwards.

## CLI contract (shared with helm-env and vc-env)

These three sibling repos expose the same command surface. Keep them in sync when you change one of them.

- `resolve [<spec>] [--install] [-s|--silent]`: with no spec, uses the active spec. Matches installed versions first and goes remote only with `--install` or `ISTIOENV_AUTO_INSTALL`. stdout is only the bare version.
- `exec [--auto|--no-auto] <spec> [--] <cmd> [args...]`: strips one leading `--`, applied after `applyGlobalFlags`, which keeps `--`. **Exits with the child's exit code**: main.go unwraps `*exec.ExitError`.
- `completion [<shell>] [--shell <shell>]`: supports bash, zsh, fish, powershell and `pwsh`, falling back to `$SHELL`. `autocompletion` is a hidden alias that prints a deprecation warning to stderr.
- `-h/--help` short-circuits on **every** command through `exitOnHelp` in main.go, and must never run side effects; `upgrade -h` must not upgrade.
- `prune [--keep-last N] [--older-than DUR] [--dry-run] [--yes]`: a **dry run unless `--yes`**.
- `doctor [--fix] [--deep]`.

## Environment variables

| Variable | Purpose |
|---|---|
| `ISTIOENV_ROOT` | Required. Root directory |
| `ISTIOENV_VERSION` | Shell-level version, set by `istioctl-env shell` |
| `ISTIOENV_AUTO_INSTALL` | `true`: the shim installs missing versions automatically |
| `ISTIOENV_OFFLINE` | No network; uses the cache and baseline only (`--offline`) |
| `ISTIOENV_GITHUB_TOKEN` | GitHub token; falls back to `GITHUB_TOKEN` (`--github-token`) |
| `ISTIOENV_API_MIRROR` | Overrides the API base (`--api-mirror`) |
| `ISTIOENV_DOWNLOAD_MIRROR` | Overrides the download base (`--download-mirror`). The legacy name is `ISTIOENV_MIRROR_URL` |
| `ISTIOENV_CACHE_TTL` | Overrides the 1h release-cache TTL |
| `ISTIOENV_PRUNE_SCAN_ROOTS` | Directories `prune` scans for `.istioctl-version` files |

Shell setup:
- bash/zsh: `eval "$(istioctl-env init [--shell bash|zsh|fish])"`
- fish: `istioctl-env init --shell fish | source`
- Without `--shell`, the shell is taken from `$SHELL`.

## Gotchas

- **The shim has its own fast path** in `internal/shim/shim.go`. If you change the resolution precedence or version-file handling, check both the Go resolver and the shim script.
- The module path is the placeholder `github.com/user/istioctl-env`. The real repo is `github.com/mmpyro/istioctl-env`.
- `.venv-docs/` and `site/` are gitignored local artifacts. Don't commit them.
- Commit style is conventional commits (`feat(scope): ...`, `docs: ...`).

## CI and release

- `.github/workflows/ci.yml` runs on push or PR to `main`:
  1. golangci-lint (latest).
  2. `gotestsum -- -race ./...`, with the JUnit report published by `dorny/test-reporter`.
  3. `make test-docker`.
- `.github/workflows/release.yml` runs on `v*.*.*` tags:
  1. `make build-all VERSION=<tag>`.
  2. BATS against the prebuilt linux amd64 and arm64 binaries using `Dockerfile.ci`. That image has no Go; the binary is mounted at `/src/istioctl-env` and run under QEMU.
  3. Creates a **draft** GitHub release with the 4 binaries. It does not use goreleaser.
- `.github/workflows/docs.yml` runs `mkdocs build --strict`, then deploys to GitHub Pages from `main`. Docs versions are pinned in `docs/requirements.txt`.
- Dependabot updates gomod weekly.
