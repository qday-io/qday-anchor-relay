#!/bin/sh
# Creates the read-only portal role at cluster init time.
# Runs as POSTGRES_USER (superuser) via docker-entrypoint-initdb.d.
set -eu

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE ROLE "${PORTAL_DB_USER:-portal_ro}" LOGIN PASSWORD '${PORTAL_DB_PASSWORD:-change-me}';
    GRANT CONNECT ON DATABASE "$POSTGRES_DB" TO "${PORTAL_DB_USER:-portal_ro}";
    GRANT USAGE ON SCHEMA public TO "${PORTAL_DB_USER:-portal_ro}";
EOSQL
