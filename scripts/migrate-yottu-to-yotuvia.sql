-- ============================================================================
-- Yottu -> Yotuvia backend rename: data-preserving PostgreSQL migration
-- ============================================================================
-- This script renames the database, the login role and one column in place.
-- It NEVER drops, recreates or truncates anything -- existing rows are kept.
--
-- WHY THE ORDER MATTERS
--   1. The login ROLE is renamed so the app can authenticate as `yotuvia`.
--   2. The DATABASE is renamed so `DB_NAME=yotuvia` in .env resolves.
--   3. The market_configs COLUMN is renamed to match the renamed GORM field
--      (json/db tag `yotuvia_to_fiat_rate`).
--   4. The API is restarted with the new .env.
--
-- The DATABASE rename MUST complete BEFORE the API restart: the updated .env
-- points DB_USER/DB_NAME at `yotuvia`, so restarting the API first would fail
-- to connect. The API must be STOPPED before running this script because
-- ALTER DATABASE ... RENAME requires no other session connected to `yottu`.
--
-- HOW TO RUN (connected to the `postgres` maintenance database -- required,
-- ALTER DATABASE cannot run while connected to the DB being renamed):
--
--   # 1. Stop the API so no session is attached to `yottu`.
--   # 2. Apply the migration:
--   docker exec -i yottu_db psql -U yottu -d postgres < scripts/migrate-yottu-to-yotuvia.sql
--   # 3. Start the API again with the new .env (DB_USER/DB_NAME=yotuvia).
--
-- Every step is guarded, so re-running the script is a safe no-op once applied.
-- ============================================================================

\set ON_ERROR_STOP on

-- ---------------------------------------------------------------------------
-- STEP 1 -- Rename the login role.
--   Canonical form (top-level, not inside a transaction):
--       ALTER ROLE yottu RENAME TO yotuvia;
--       ALTER ROLE yotuvia WITH PASSWORD 'yotuvia';
--   Executed through \gexec so an already-migrated database is a no-op.
-- ---------------------------------------------------------------------------
SELECT 'ALTER ROLE yottu RENAME TO yotuvia'
WHERE EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'yottu')
\gexec

SELECT 'ALTER ROLE yotuvia WITH PASSWORD ''yotuvia'''
WHERE EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'yotuvia')
\gexec

-- ---------------------------------------------------------------------------
-- STEP 2 -- Rename the database.
--   Canonical form: ALTER DATABASE yottu RENAME TO yotuvia;
--   Runs at top level (not in a transaction) as PostgreSQL requires.
-- ---------------------------------------------------------------------------
SELECT 'ALTER DATABASE yottu RENAME TO yotuvia'
WHERE EXISTS (SELECT 1 FROM pg_database WHERE datname = 'yottu')
\gexec

-- ---------------------------------------------------------------------------
-- STEP 3 -- Rename the market_configs column.
--   Canonical form:
--       ALTER TABLE market_configs RENAME COLUMN yottu_to_fiat_rate TO yotuvia_to_fiat_rate;
--   Table name comes from GORM default pluralization (`MarketConfig` with no
--   custom TableName() => market_configs). Guarded so re-runs are a no-op.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'market_configs' AND column_name = 'yottu_to_fiat_rate'
  ) THEN
    ALTER TABLE market_configs RENAME COLUMN yottu_to_fiat_rate TO yotuvia_to_fiat_rate;
  END IF;
END $$;

-- ---------------------------------------------------------------------------
-- VERIFICATION (read-only). Expected: database yotuvia, role yotuvia,
-- column yotuvia_to_fiat_rate present and yottu_to_fiat_rate absent.
-- ---------------------------------------------------------------------------
SELECT datname AS database FROM pg_database WHERE datname IN ('yottu', 'yotuvia');
SELECT rolname AS role FROM pg_roles WHERE rolname IN ('yottu', 'yotuvia');
SELECT column_name FROM information_schema.columns
WHERE table_name = 'market_configs' AND column_name IN ('yottu_to_fiat_rate', 'yotuvia_to_fiat_rate');
