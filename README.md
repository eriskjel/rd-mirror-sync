# rd-mirror-sync

One-way Real-Debrid library mirror daemon. Syncs torrents from a source account to one or more destination accounts, running each destination concurrently.

- `add-only` and `mirror-delete` modes
- Per-destination mode and dry-run overrides
- Safe rollout with `dry_run: true`
- Health and metrics endpoints (`/healthz`, `/metrics`)
- systemd service template included in `deploy/`

## Setup

### 1. Create config file

Copy the example and edit it:

```bash
cp config.example.json /opt/rd-mirror-sync/config.json
```

`config.json` holds all settings **except tokens**. Tokens go in `.env`.

### 2. Create .env with tokens

```bash
# Source account
SRC_RD_TOKEN=your_source_token

# One entry per destination — name must match config.json
RD_TOKEN_LOCATION_1=your_destination_token
RD_TOKEN_LOCATION_2=another_destination_token
```

The env var name is derived from the destination `name` in config.json:
`location-1` → `RD_TOKEN_LOCATION_1`, `my place` → `RD_TOKEN_MY_PLACE`

### 3. Run

```bash
go run ./cmd/rd-mirror-sync
```

Or via systemd — see [systemd](#systemd) below.

## config.json reference

```json
{
  "health_addr": ":8099",
  "mode": "add-only",
  "dry_run": true,
  "interval": "1m",

  "destinations": [
    { "name": "location-1" },
    { "name": "location-2", "mode": "add-only", "dry_run": true, "enabled": false }
  ]
}
```

| Field | Default | Description |
|---|---|---|
| `mode` | `add-only` | `add-only` or `mirror-delete` |
| `dry_run` | `false` | Log actions without making changes |
| `interval` | `45s` | How often to sync (min 10s) |
| `run_timeout` | `10m` | Max time per sync run (0 = no limit) |
| `http_timeout` | `20s` | RD API request timeout |
| `write_delay` | `250ms` | Delay between add/delete operations |
| `max_retries` | `4` | API retry attempts |
| `page_limit` | `250` | Torrents per API page |
| `health_addr` | _(disabled)_ | Address for `/healthz` and `/metrics` |
| `base_url` | RD API | Override RD API base URL |

Each destination inherits all global settings and can override `mode`, `dry_run`, and `protect_dst_regex`. Set `enabled: false` to skip a destination without removing it.

## Health endpoint

```
GET /healthz              # all destinations
GET /healthz?dest=name    # single destination
GET /metrics              # Prometheus-style metrics per destination
```

Example homepage widget URL: `http://host:8099/healthz?dest=location-1`

## Suggested rollout

1. Start with `dry_run: true` and `mode: add-only` — verify logs look correct
2. Set `dry_run: false` — live adds begin
3. Optionally switch to `mode: mirror-delete` once confident (set `protect_dst_regex` if needed)

## Build

```bash
go build -o rd-mirror-sync ./cmd/rd-mirror-sync
```

## Testing

```bash
go test ./...
```

## systemd

Copy the service template and install:

```bash
sudo cp deploy/rd-mirror-sync.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now rd-mirror-sync
sudo systemctl status rd-mirror-sync
```

The service expects the binary and config at `/opt/rd-mirror-sync/`. Adjust `User=` and `WorkingDirectory` in the service file if needed.
