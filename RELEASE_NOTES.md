## rd-mirror-sync v1.1.0

### Features

- **Multi-destination sync:** Configure multiple destination RD accounts in `config.json`. Each destination syncs in parallel and is tracked independently.
- **Per-destination overrides:** Each destination can override the global `mode` and `dry_run` settings, or inherit them if not set.
- **Enable/disable destinations:** Set `"enabled": false` on a destination to skip it without removing it from the config.
- **Token isolation via env vars:** Destination tokens are loaded from `RD_TOKEN_<NAME>` environment variables (e.g. `RD_TOKEN_STAVANGER`). The source token uses `SRC_RD_TOKEN`. Tokens can also be set directly in `config.json` but env vars are recommended.
- **Per-destination health endpoint:** `/healthz?dest=<name>` returns status for a single destination (same response shape as before — existing homepage widgets just need `?dest=<name>` appended). `/healthz` with no param returns all destinations.
- **Per-destination metrics:** `/metrics` now includes a `dest` label on all gauges.

### Breaking changes

- **Config file replaces env vars.** All settings previously configured via `.env` are now in `config.json`. Copy `config.example.json` as a starting point. Tokens stay in `.env`.

### Migration from v1.0.x

1. Copy `config.example.json` to `/opt/rd-mirror-sync/config.json` and fill in your settings.
2. Update `.env` to only contain `SRC_RD_TOKEN` and `RD_TOKEN_<DEST_NAME>`.
3. Deploy the new binary and restart the service.
