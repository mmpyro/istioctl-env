# istioctl-env Documentation

`istioctl-env` is a version manager for the [`istioctl`](https://istio.io/latest/docs/reference/commands/istioctl/) CLI.

It installs multiple `istioctl` versions under a single root directory, generates an `istioctl` shim, and selects the correct `istioctl` binary at runtime based on your configured version.

## Guides

- [Installation and configuration](installation-and-configuration.md)
- [CLI reference](cli-reference.md)
- [Caching strategy](caching.md)

## How version selection works

When you run `istioctl ...`, the shim selects a version using the following priority:

1. **Shell version** via `ISTIOENV_VERSION` (typically set by `istioctl-env shell` after `eval "$(istioctl-env init)"`).
2. **Local version** via a `.istioctl-version` file in the current directory or any parent directory.
3. **Global version** via `$ISTIOENV_ROOT/version`.

If no version is configured, the shim fails with an actionable error message.

## Project layout under `ISTIOENV_ROOT`

By default, `ISTIOENV_ROOT` is set to `~/.istioenv`.

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
