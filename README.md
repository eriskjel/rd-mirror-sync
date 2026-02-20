# rd-mirror-sync

One-way Real-Debrid library mirror daemon:

- Source account (Trondheim) -> destination account (Stavanger)
- Safe rollout with `DRY_RUN=true`
- Optional delete mirror mode (`MIRROR_MODE=mirror-delete`)
- Optional protect rule for destination-only items (`PROTECT_DST_REGEX`)
- Health and metrics endpoints (`/healthz`, `/metrics`)

## Quick start

1. Copy environment file:

```bash
cp .env.example .env
```

2. Edit `.env` and set `SRC_RD_TOKEN` + `DST_RD_TOKEN`.

3. Run:

```bash
go run ./cmd/rd-mirror-sync
```

## Build

```bash
go build -o rd-mirror-sync ./cmd/rd-mirror-sync
```

## Suggested rollout

- Days 1-2: `DRY_RUN=true`, `MIRROR_MODE=add-only`
- Days 3-5: `DRY_RUN=false`, `MIRROR_MODE=add-only`
- Later: `MIRROR_MODE=mirror-delete` (optionally set `PROTECT_DST_REGEX`)

## Testing

```bash
go test ./...
```

## systemd

Service file is at `deploy/rd-mirror-sync.service`.

Install example:

```bash
sudo cp deploy/rd-mirror-sync.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now rd-mirror-sync
sudo systemctl status rd-mirror-sync
```
