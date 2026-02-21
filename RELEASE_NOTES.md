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
