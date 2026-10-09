# Caching Strategy in `istioctl-env`

`istioctl-env` uses a sophisticated three-layer caching strategy to manage the list of available `istioctl` versions. This ensure that commands like `list-remote` and `latest` remain fast, reliable, and respectful of GitHub API rate limits.

## 1. The Three Layers

### Layer 1: Hardcoded Baseline
A list of historically known stable and pre-release versions is baked directly into the `istioctl-env` binary (see `internal/cache/baseline.go`).

*   **Zero Latency**: Provides a useful starting point even on the very first run.
*   **Offline Fallback**: Acts as the ultimate fallback if both the disk cache and the network are unavailable.
*   **Anchor Point**: The newest version in this list is used as the "anchor" for the first delta fetch.

### Layer 2: Disk Cache
When `ISTIOENV_ROOT` is set, `istioctl-env` persists the merged list of versions to a JSON file at:
`$ISTIOENV_ROOT/cache/releases.json`

*   **Freshness**: If the cache file is younger than the TTL (Time To Live), it is served immediately without any network calls.
*   **Atomicity**: Writes use a "write-to-temp then rename" pattern to ensure that concurrent processes never read a partially written file.

### Layer 3: Delta Fetch
If the disk cache is stale (older than TTL) or missing, `istioctl-env` performs a "delta fetch" from the GitHub API.

*   **Efficiency**: Instead of fetching all history, it only requests tag v1.0.0 newer than the most recent version found in the stale cache (or the baseline).
*   **Auto-Merge**: New tag v1.0.0 are automatically merged with the existing known versions, deduplicated, and sorted.
*   **Graceful Degradation**: If the network is unavailable during a delta fetch, `istioctl-env` will print a warning and fall back to the stale cache or the hardcoded baseline.

---

## 2. Configuration

You can customize the caching behavior using environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `ISTIOENV_ROOT` | Root directory for `istioctl-env`. If not set, caching is memory-only (no disk persistence). | N/A |
| `ISTIOENV_CACHE_TTL` | How long a cache entry is considered fresh. Supports Go duration strings (e.g., `1h`, `30m`, `24h`, `0s`). | `1h` |

### Disabling the Cache
To force a fresh fetch every time, you can set the TTL to zero:
```bash
export ISTIOENV_CACHE_TTL=0s
```

---

## 3. Storage Format

The cache file (`releases.json`) stores:
*   `fetched_at`: UTC timestamp of the last successful fetch.
*   `versions`: List of stable versions (newest-first).
*   `prerelease_versions`: List of all versions including pre-releases (newest-first).

---

## 4. Maintenance

The hardcoded baseline should be updated periodically (e.g., when releasing a new version of `istioctl-env`) to keep the "lower bound" reasonably close to the current state of the world. However, the system is designed to correct itself automatically via delta fetches even if the baseline is significantly out of date.

---

## 5. Offline mode

When `ISTIOENV_OFFLINE=1` is set, the caching strategy changes to keep `istioctl-env` fully functional without any network access:

* **Layer 1 (fresh disk cache)** is still consulted first — if a non-expired `releases.json` exists it is used verbatim.
* **Layer 2 (delta fetch)** is **skipped entirely**. The code does not even attempt the GitHub API call, so no error-path warning is printed to `stderr`.
* **Layer 3 (fallback)** behaves as before: a stale cache (ignoring TTL) is preferred over the hardcoded baseline.

The practical effect is that `list-remote` and `latest` always return results and never warn about network failures while offline. If `ISTIOENV_OFFLINE=1` is combined with `ISTIOENV_DOWNLOAD_MIRROR` (and, optionally, `ISTIOENV_API_MIRROR` for `upgrade`), `install <version>` can also run with no public-internet access — the download mirror serves both the archive and the per-file `.sha256` from the same relative path as GitHub. The legacy `ISTIOENV_MIRROR_URL` is still accepted as an alias for `ISTIOENV_DOWNLOAD_MIRROR`. See [installation and configuration](installation-and-configuration.md) for the full set of environment variables and matching CLI flags.
