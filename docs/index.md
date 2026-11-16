# istioctl-env documentation

`istioctl-env` is a version manager for the [`istioctl`](https://istio.io/latest/docs/reference/commands/istioctl/) CLI.

It installs multiple `istioctl` versions under one root directory and generates an `istioctl` shim. At runtime, the shim selects the correct `istioctl` binary for the version that you configured.

## Guides

- [Installation and configuration](installation-and-configuration.md)
- [CLI reference](cli-reference.md)
- [Caching strategy](caching.md)

## How version selection works

When you run `istioctl ...`, the shim selects a version in this priority order:

1. Shell version: `ISTIOENV_VERSION`. Usually `istioctl-env shell` sets it, after `eval "$(istioctl-env init)"`.
2. Local version: a `.istioctl-version` file in the current directory or in any parent directory.
3. Global version: `$ISTIOENV_ROOT/version`.

If no version is configured, the shim fails. Its error message tells you what to do.

## Project layout under `ISTIOENV_ROOT`

The default value of `ISTIOENV_ROOT` is `~/.istioenv`.

```text
$ISTIOENV_ROOT/
├── versions/
│   ├── <istioctl-version>/
│   │   └── istioctl
│   └── ...
├── shims/
│   └── istioctl
└── version
```
