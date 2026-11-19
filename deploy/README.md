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
| `docker-compose.yml` | canonical Postgres definition for `/srv/yotuvia` |

The `docker-compose.yml` in the monorepo root is a **dev-only** copy that binds
`yotuvia-api/postgres_data`; it is not versioned and must not be copied to the
server. The file here is the canonical one: it mounts `./postgres_data`, which
is `/srv/yotuvia/postgres_data` in the layout below.

## Layout on the server

The API writes uploads to paths **relative to its working directory**
(`uploads/clients/...`, `uploads/videos/...`), while `UPLOAD_DIR` only overrides
the receipts subdirectory. That is why the unit pins `WorkingDirectory`:

```
/srv/yotuvia/
├── api/
│   ├── yotuvia-api      <- binary installed by deploy-api.sh
│   └── .env             <- secrets, chmod 600, NEVER committed
├── .env -> api/.env     <- symlink read by docker compose interpolation
├── docker-compose.yml   <- copy of deploy/docker-compose.yml
├── postgres_data/       <- bind mount, owned by uid 999 (the postgres user)
├── backups/             <- backup.sh output (keep retention low)
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
sudo cp .env.example /srv/yotuvia/api/.env
sudo -e /srv/yotuvia/api/.env        # sudoedit, runs your $EDITOR
sudo chown yotuvia:yotuvia /srv/yotuvia/api/.env
sudo chmod 600 /srv/yotuvia/api/.env

# 3. Database
sudo cp deploy/docker-compose.yml /srv/yotuvia/docker-compose.yml
# Compose interpolates from ./.env, while the API reads api/.env. One symlink
# keeps both on the same DB credentials instead of duplicating the password.
sudo ln -s api/.env /srv/yotuvia/.env
cd /srv/yotuvia && sudo docker compose up -d
sudo docker compose ps          # db must report (healthy)

# 4. Unit
sudo cp deploy/yotuvia-api.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable yotuvia-api
```

Production `.env` checklist (a fresh `.env.example` boots, but not safely):

- `JWT_SECRET` — new random value, **not** the dev default
- `REGISTRATION_TICKET_SECRET`, `BOOST_IMPRESSION_SECRET` — new random values
- `DB_PASSWORD` — one value only: the container reads it through the `.env`
  symlink created in step 3, so the API and Postgres cannot diverge
- `GIN_MODE=release`, `LOG_FORMAT=json`, `LOG_LEVEL=info`
- `TRUSTED_PROXIES=127.0.0.1/32`
- `CORS_ALLOWED_ORIGINS` and `FRONT_HOST` — the real https origin
- `SEED_DEMO_CENTERS=false`
- `SUPER_ADMIN_EMAIL` / `SUPER_ADMIN_PASSWORD` — rotate after the first boot
  (the seed never overwrites an existing password)
- `SMTP_*` — real provider, sending domain verified (SPF/DKIM/DMARC)

## Cutting a release (recommended)

`.github/workflows/release.yml` builds `linux/amd64` and `linux/arm64` on every
`v*` tag, runs the test suite, and attaches `yotuvia-api-linux-<arch>.tar.gz`
plus its `.sha256` to the GitHub release:

```bash
git tag -a v1.0.0 -m "v1.0.0"
git push origin v1.0.0
```

The binary is stamped with the tag (`-X main.version=...`) and logs it at boot:

```
[BOOT] yotuvia-api v1.0.0
```

## Deploying a new version

Run from the API checkout on the server:

```bash
# Recommended: install the exact artifact CI built and tested.
ARTIFACT_URL=https://github.com/Nina-99/yotuvia-api/releases/download/v1.0.0/yotuvia-api-linux-amd64.tar.gz \
  ./deploy/deploy-api.sh

# Or build from the checkout (needs the Go toolchain on the server).
./deploy/deploy-api.sh
```

What it does, in order:
1. Downloads the artifact (with `sha256sum -c` against the published
   `.sha256`; a missing or mismatched checksum aborts before touching the
   binary), or builds `CGO_ENABLED=0` from source.
2. Refuses to continue if `/srv/yotuvia/api/.env` is missing.
3. Installs the binary beside the old one and `mv`s it into place (atomic: a
   running process keeps the old inode until restart).
4. `systemctl restart yotuvia-api`.
5. Polls `/api/healthz` for up to 30s and exits non-zero with
   `systemctl status` output if the process is not ready.

`ARCH=arm64` selects the other asset. `ARTIFACT_FILE=/path/file.tar.gz` works
for an air-gapped host.

The schema is migrated by `AutoMigrate` on boot, so **take a database backup
before deploying** anything that touches a model.

## Verify

```bash
curl -fsS http://127.0.0.1:8080/api/healthz      # {"db":"ok","status":"ok"}
curl -sI https://yotuvia.com/api/healthz | head -1
systemctl status yotuvia-api --no-pager
journalctl -u yotuvia-api -n 50 --no-pager
```
