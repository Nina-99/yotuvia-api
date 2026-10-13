# Scripts

Operational scripts for `yotuvia-api`.

## Status

The `yottu` -> `yotuvia` database migration was **already applied**. The running
cluster uses:

- container: `yotuvia_db` (started from the repository root `docker-compose.yml`)
- database: `yotuvia`
- application role: `yotuvia`
- column: `market_configs.yotuvia_to_fiat_rate`

Note: the original bootstrap role `yottu` still exists because it owns
`template0`, which the database system does not allow reassigning. It is inert
(nothing connects with it) and cannot be dropped without re-initializing the
cluster.

### Run the database

```sh
docker compose up -d        # from the repository root
```

## `migrate-yottu-to-yotuvia.sql`

Renames the database, the login role and the `market_configs` column **in
place**. It is data-preserving: no table is dropped, recreated or truncated, and
all existing rows are kept. Every step is guarded, so re-running it after a
successful migration is a safe no-op.

Keep this script for reference; it documents the exact rename applied to the
cluster.

### How it was run (historical)

1. **Back up** (a logical dump is the safety net):

   ```sh
   docker exec yottu_db pg_dump -U yottu -d yottu > /tmp/opencode/yotuvia-backup/pre-rename-$(date +%F).sql
   ```

2. **Stop the API.** `ALTER DATABASE ... RENAME` requires that no other session
   is connected to the old database. Do not recreate the `postgres_data`
   directory — the migration renames objects inside the existing cluster.

3. **Apply** by connecting to the `postgres` maintenance database (you cannot
   rename a database while connected to it):

   ```sh
   docker exec -i yottu_db psql -U yottu -d postgres < scripts/migrate-yottu-to-yotuvia.sql
   ```

   The role rename cannot run from a session authenticated as that same role.
   The applied procedure was: create `yotuvia` as a superuser, `ALTER DATABASE
   ... OWNER TO yotuvia`, reassign table/sequence/schema ownership, then drop
   `yottu` (which failed only because of `template0`; it was left in place).

### Verification

```sh
docker exec yotuvia_db psql -U yotuvia -d yotuvia -c '\d market_configs'
docker exec yotuvia_db psql -U yotuvia -d yotuvia -c 'SELECT count(*) FROM reservations;'
```

### Rollback

The reverse operation is symmetric and equally data-preserving:

```sql
ALTER TABLE market_configs RENAME COLUMN yotuvia_to_fiat_rate TO yottu_to_fiat_rate;
ALTER DATABASE yotuvia RENAME TO yottu;   -- run connected to `postgres`
ALTER ROLE yotuvia RENAME TO yottu;       -- cannot be the current session role
```

Restore `.env` and restart the database and API afterwards.
