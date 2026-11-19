# yotuvia-api — VPS deployment

Files here are the versioned half of `docs/deployment/nginx.md` (the repo root
is not a git repository, so that doc never reaches GitHub).

| File | Purpose |
|---|---|
| `yotuvia-api.service` | systemd unit (`/etc/systemd/system/yotuvia-api.service`) |
| `deploy-api.sh` | builds the binary, installs it, restarts, gates on `/api/healthz` |
| `backup.sh` / `restore.sh` | database + uploads backup set, and its restore |
| `yotuvia-backup.{service,timer}` | nightly backup at 03:30 |
| `BACKUP.md` | backup/restore runbook and the pre-deploy procedure |

## Layout on the server

The API writes uploads to paths **relative to its working directory**
(`uploads/clients/...`, `uploads/videos/...`), while `UPLOAD_DIR` only overrides
the receipts subdirectory. That is why the unit pins `WorkingDirectory`:

```
/srv/yotuvia/
├── api/
│   ├── yotuvia-api      <- binary installed by deploy-api.sh
│   └── .env             <- secrets, chmod 600, NEVER committed
├── web/                 <- static SPA (deploy-web.sh in yotuvia-web)
└── uploads/             <- nginx serves this; must be writable by the API user
    ├── receipts/
    ├── clients/
    ├── images/
    ├── personnel/
    └── videos/
```

## First install

```bash
# 1. Runtime user and directories
sudo useradd --system --home /srv/yotuvia --shell /usr/sbin/nologin yotuvia
sudo mkdir -p /srv/yotuvia/{api,web,uploads/{receipts,clients,images,personnel,videos}}
sudo chown -R yotuvia:yotuvia /srv/yotuvia

# 2. Configuration (never copy the local dev .env)
sudo -u yotuvia cp .env.example /srv/yotuvia/api/.env
sudo chmod 600 /srv/yotuvia/api/.env
sudo -u yotuvia "$EDITOR" /srv/yotuvia/api/.env

# 3. Unit
sudo cp deploy/yotuvia-api.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable yotuvia-api
```

Production `.env` checklist (a fresh `.env.example` boots, but not safely):

- `JWT_SECRET` — new random value, **not** the dev default
- `REGISTRATION_TICKET_SECRET`, `BOOST_IMPRESSION_SECRET` — new random values
- `DB_PASSWORD` — must match `POSTGRES_PASSWORD` in `docker-compose.yml`
- `GIN_MODE=release`, `LOG_FORMAT=json`, `LOG_LEVEL=info`
- `TRUSTED_PROXIES=127.0.0.1/32`
- `CORS_ALLOWED_ORIGINS` and `FRONT_HOST` — the real https origin
- `SEED_DEMO_CENTERS=false`
- `SUPER_ADMIN_EMAIL` / `SUPER_ADMIN_PASSWORD` — rotate after the first boot
  (the seed never overwrites an existing password)
- `SMTP_*` — real provider, sending domain verified (SPF/DKIM/DMARC)

## Deploying a new version

Run from the API checkout on the server:

```bash
./deploy/deploy-api.sh
```

What it does, in order:
1. `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"` (static binary).
2. Refuses to continue if `/srv/yotuvia/api/.env` is missing.
3. Installs the binary beside the old one and `mv`s it into place (atomic: a
   running process keeps the old inode until restart).
4. `systemctl restart yotuvia-api`.
5. Polls `/api/healthz` for up to 30s and exits non-zero with
   `systemctl status` output if the process is not ready.

The schema is migrated by `AutoMigrate` on boot, so **take a database backup
before deploying** anything that touches a model.

## Verify

```bash
curl -fsS http://127.0.0.1:8080/api/healthz      # {"db":"ok","status":"ok"}
curl -sI https://yotuvia.com/api/healthz | head -1
systemctl status yotuvia-api --no-pager
journalctl -u yotuvia-api -n 50 --no-pager
```
