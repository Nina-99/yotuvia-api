# Backup and restore runbook

The database and `/srv/yotuvia/uploads` are **one restore unit**. A row points
at a file path; restoring the database without the files leaves broken images,
and restoring the files without the database leaves orphans nobody can map back.
`backup.sh` always writes both and `restore.sh` refuses to restore just one
unless you pass `--db-only` on purpose.

| File | Purpose |
|---|---|
| `backup.sh` | dump (custom format) + uploads archive, verifies both, applies retention |
| `restore.sh` | stops the API, replaces uploads and database, restarts, gates on health |
| `yotuvia-backup.service` / `.timer` | nightly run at 03:30 with a 5 minute jitter |

## What a backup set contains

```
/srv/yotuvia/backups/
├── db-YYYYmmdd-HHMMSS.dump      # pg_dump -Fc, compressed
└── uploads-YYYYmmdd-HHMMSS.tar.gz
```

- `pg_dump` produces a **consistent snapshot** of the database on its own.
- The uploads archive is **not** point-in-time: files uploaded during the tar
  can land in the next set. That is acceptable for images/videos; the database
  is the authoritative side and never references a file from an incomplete set
  (the row is written after the file).
- Retention defaults to 14 days, override with `RETENTION_DAYS`.

## Install the nightly job

```bash
sudo cp /srv/yotuvia/api/deploy/yotuvia-backup.{service,timer} /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now yotuvia-backup.timer
systemctl list-timers yotuvia-backup.timer
```

Manual run (same as the timer, prints every step):

```bash
sudo /srv/yotuvia/api/deploy/backup.sh
```

## Offsite copies are not optional

A backup on the disk that holds the database does not survive disk failure,
ransomware or a bad `rm`. After a successful run, copy the set somewhere else:

```bash
# Example: rclone to object storage, keep the local retention shorter than remote
rclone copy /srv/yotuvia/backups remote:yotuvia-backups --include 'db-*' --include 'uploads-*'
```

The dump contains customer PII and payment receipts. **Encrypt before it leaves
the host**, for example:

```bash
age -r age1... -o "$f.age" "$f"
```

Keep the age/gpg private key somewhere that is not the same VPS.

## Before every deploy

`AutoMigrate` runs on boot and has no down-migration. Deploying a model change
without a backup turns a code rollback into a data incident.

```bash
# 1. Backup first
sudo /srv/yotuvia/api/deploy/backup.sh

# 2. Confirm the set is restorable (lists the archive, fails on a bad file)
docker exec -i yotuvia_db pg_restore -l < /srv/yotuvia/backups/db-<stamp>.dump | head

# 3. Deploy
./deploy/deploy-api.sh
```

If the deploy goes bad: `git checkout` the previous tag/commit, redeploy the old
binary, and only restore the database if the new binary already migrated it.

## Restore procedure

```bash
sudo /srv/yotuvia/api/deploy/restore.sh \
  --db /srv/yotuvia/backups/db-20261119-033000.dump \
  --uploads /srv/yotuvia/backups/uploads-20261119-033000.tar.gz
```

`restore.sh` will:
1. Validate the dump header and the tar before touching anything.
2. Ask for `restore` typed back unless `--yes` is passed.
3. Stop `yotuvia-api`.
4. Move the current uploads to `uploads.pre-restore-<stamp>` (nothing is
   deleted; remove it yourself once the restore is verified).
5. `pg_restore --clean --if-exists --no-owner` into the configured database.
6. Start `yotuvia-api` and wait for `/api/healthz`.

`pg_restore` returns non-zero for warnings while having restored the data (for
example a role or extension that does not exist). The script prints a warning
and keeps going; **read the output**.

Do not restore a dump made by a **newer** schema than the running binary: the
binary's `AutoMigrate` only adds what it knows about, and columns it never heard
of stay in a state it cannot reason about.

## Drill it

A backup nobody restored is a hope, not a backup. Once a month:

```bash
# Restore into a throwaway database, never into the live one
docker exec yotuvia_db createdb -U yotuvia yotuvia_restore_check
docker exec -i yotuvia_db pg_restore --no-owner --clean --if-exists \
  -U yotuvia -d yotuvia_restore_check < /srv/yotuvia/backups/db-<stamp>.dump
docker exec yotuvia_db psql -U yotuvia -d yotuvia_restore_check \
  -tAc "select count(*) from information_schema.tables where table_schema='public'"
docker exec yotuvia_db dropdb -U yotuvia yotuvia_restore_check
```

## Disk space

Uploads and the database share one volume, and the API already refuses uploads
below `MIN_FREE_DISK_BYTES` (default 2 GiB). Backups live on that same volume by
default: keep `RETENTION_DAYS` low enough that backups cannot be the thing that
fills it. `df -h /srv/yotuvia` is part of the monthly check.
