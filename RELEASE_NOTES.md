## rd-mirror-sync v1.0.2

### Fixes and improvements (RD API client)
- **Timer leak fix:** Retry backoff in the API client now uses `time.NewTimer` instead of `time.After`, so timers are stopped on context cancel and no longer leak under load.
- **DRY HTTP handling:** Request execution, auth, status handling, and body decode/discard are centralized in a `doRequest` helper; GET/POST/DELETE helpers now use it.
- **Body discard errors:** When discarding a response body (e.g. selectFiles, delete), read errors are now returned as retryable instead of ignored, so connection reuse and failures are handled correctly.
- **Form encode once:** POST form bodies are encoded once before the retry loop instead of on every attempt.
- **Docs:** `doRequest` comment updated to describe retry behavior.

Same install and config as v1.0.1.

---

## rd-mirror-sync v1.0.1

### Fix
- **selectFiles retry:** After adding a magnet on the destination account, the sync now waits 2 seconds then calls Real-Debrid’s selectFiles (select all). If that fails, it retries up to 3 times with 3 seconds between attempts. This avoids “select files failed” when RD is still preparing the torrent (e.g. fetching metadata). Same install and config as v1.0.0.

---

## rd-mirror-sync v1.0.0

One-way Real-Debrid torrent mirror daemon (source -> destination), built for safe home-server automation.

### Highlights
- Periodic sync between two RD accounts
- `add-only` and `mirror-delete` modes
- `DRY_RUN` support for safe rollout and validation
- Retry/backoff handling for API/transient failures
- Health and metrics endpoints: `/healthz`, `/metrics`
- systemd service template included in `deploy/`

### Recommended mode
- `MIRROR_MODE=add-only`
- `DRY_RUN=false` only after initial dry-run validation

### Install
1. Download the binary for your platform
2. Copy `.env.example` to `.env` and set `SRC_RD_TOKEN` / `DST_RD_TOKEN`
3. Run directly or via systemd unit template in `deploy/`

### Notes
- `deploy/rd-mirror-sync.service` is a template; adjust `User=` and paths for your host
- Never commit `.env` or Real-Debrid tokens
- `dist/` is intentionally ignored in git and used for release assets
