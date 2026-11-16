# Caching strategy in `istioctl-env`

`istioctl-env` uses a cache with three layers for the list of available `istioctl` versions. The cache keeps commands like `list-remote` and `latest` fast and reliable. It also keeps them inside the GitHub API rate limits.

## 1. The three layers

### Layer 1: Hardcoded baseline

The `istioctl-env` binary contains a list of known stable and pre-release versions (see `internal/cache/baseline.go`).

- The baseline has zero latency. It gives a useful starting point, even on the first run.
- The baseline is the last fallback when the disk cache and the network are both unavailable.
- The newest version in this list is the "anchor" for the first delta fetch.

### Layer 2: Disk cache

When `ISTIOENV_ROOT` is set, `istioctl-env` writes the merged list of versions to a JSON file:
`$ISTIOENV_ROOT/cache/releases.json`

- If the cache file is younger than the TTL (time to live), `istioctl-env` uses it at once, with no network calls.
- Each write goes to a temporary file first, and then `istioctl-env` renames that file. Thus concurrent processes never read a partly written file.

### Layer 3: Delta fetch

If the disk cache is stale (older than the TTL) or missing, `istioctl-env` does a "delta fetch" from the GitHub API.

- The delta fetch does not get the full history. It only requests releases newer than the most recent version in the stale cache (or in the baseline).
- `istioctl-env` merges the new releases with the known versions. Then it removes duplicates and sorts the list.
- If the network is unavailable during a delta fetch, `istioctl-env` prints a warning. Then it falls back to the stale cache or the hardcoded baseline.

## 2. Configuration

These environment variables change the cache behavior:

| Variable | Description | Default |
|----------|-------------|---------|
| `ISTIOENV_ROOT` | Root directory for `istioctl-env`. If it is not set, the cache is in memory only (nothing is written to disk). | N/A |
| `ISTIOENV_CACHE_TTL` | How long a cache entry stays fresh. Accepts Go duration strings (e.g., `1h`, `30m`, `24h`, `0s`). | `1h` |

### Disabling the cache

To get a fresh list on every call, set the TTL to zero:
```bash
export ISTIOENV_CACHE_TTL=0s
```

## 3. Storage format

The cache file (`releases.json`) has these fields:

- `fetched_at`: UTC timestamp of the last successful fetch.
- `versions`: list of stable versions, newest first.
- `prerelease_versions`: list of all versions, pre-releases included, newest first.

## 4. Maintenance

Update the hardcoded baseline from time to time, for example when you release a new version of `istioctl-env`. This keeps the "lower bound" close to the current releases. But the delta fetch corrects the list automatically, even if the baseline is far out of date.

## 5. Offline mode

When `ISTIOENV_OFFLINE=1` is set, the cache works differently. `istioctl-env` then works fully without network access:

- Layer 1 (fresh disk cache): `istioctl-env` still reads it first. If an unexpired `releases.json` exists, `istioctl-env` uses it unchanged.
- Layer 2 (delta fetch): `istioctl-env` skips it fully. The code does not try the GitHub API call, so it prints no error warning to `stderr`.
- Layer 3 (fallback): no change. `istioctl-env` uses a stale cache (it ignores the TTL) before the hardcoded baseline.

As a result, `list-remote` and `latest` always return results while offline. They never warn about network failures.

You can also combine `ISTIOENV_OFFLINE=1` with `ISTIOENV_DOWNLOAD_MIRROR`, and optionally `ISTIOENV_API_MIRROR` for `upgrade`. Then `install <version>` also works with no access to the public internet. The download mirror serves both the archive and the per-file `.sha256` from the same relative path as GitHub. `istioctl-env` still accepts the legacy `ISTIOENV_MIRROR_URL` as an alias for `ISTIOENV_DOWNLOAD_MIRROR`.

For the full set of environment variables and matching CLI flags, see [installation and configuration](installation-and-configuration.md).
