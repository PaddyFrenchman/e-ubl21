#!/usr/bin/env bash
# Performs a non-destructive logical YSQL restore drill against a separately
# addressed clean target. It deliberately never drops or recreates production.
set -Eeuo pipefail

: "${EUBL21_DR_SOURCE_URL:?set the source YSQL URL}"
: "${EUBL21_DR_RESTORE_URL:?set the clean restore-target YSQL URL}"
: "${EUBL21_DR_BACKUP_DIR:?set an off-machine/off-cluster mounted backup directory}"
ysql_dump_bin="${EUBL21_DR_YSQL_DUMP:-ysql_dump}"
ysqlsh_bin="${EUBL21_DR_YSQLSH:-ysqlsh}"

[[ "$EUBL21_DR_SOURCE_URL" != "$EUBL21_DR_RESTORE_URL" ]] || {
  echo "source and restore target URLs must be different" >&2; exit 2;
}
[[ -d "$EUBL21_DR_BACKUP_DIR" && -w "$EUBL21_DR_BACKUP_DIR" ]] || {
  echo "EUBL21_DR_BACKUP_DIR must be a writable externally mounted directory" >&2; exit 2;
}
command -v "$ysql_dump_bin" >/dev/null
command -v "$ysqlsh_bin" >/dev/null

umask 077
run_id="$(date -u +%Y%m%dT%H%M%SZ)-$$"
backup="$EUBL21_DR_BACKUP_DIR/eubl21-ysql-$run_id.sql"
metadata="$backup.sha256"
trap 'rm -f "$backup.partial" "$metadata.partial"' EXIT

# The probe is deliberately ordinary relational data: its schema and value
# prove that the restored target accepts application-style YSQL access.
"$ysqlsh_bin" "$EUBL21_DR_SOURCE_URL" -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS eubl21_dr_qualification (
  id integer PRIMARY KEY,
  payload text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO eubl21_dr_qualification (id, payload)
VALUES (128, 'eubl21-yugabytedb-dr-qualification')
ON CONFLICT (id) DO UPDATE SET payload = EXCLUDED.payload;
SQL

"$ysql_dump_bin" --dbname="$EUBL21_DR_SOURCE_URL" --file="$backup.partial"
sha256sum "$backup.partial" >"$metadata.partial"
mv -- "$backup.partial" "$backup"
mv -- "$metadata.partial" "$metadata"
sha256sum --check --status "$metadata"

# The target must have been freshly created from template0 (or be an equally
# clean disposable database). Refuse to merge a drill into an existing probe.
if "$ysqlsh_bin" "$EUBL21_DR_RESTORE_URL" -Atqc \
  "SELECT 1 FROM pg_catalog.pg_tables WHERE schemaname = 'public' AND tablename = 'eubl21_dr_qualification'" | grep -qx 1; then
  echo "restore target is not clean: eubl21_dr_qualification already exists" >&2; exit 2
fi
"$ysqlsh_bin" "$EUBL21_DR_RESTORE_URL" -v ON_ERROR_STOP=1 -f "$backup"
"$ysqlsh_bin" "$EUBL21_DR_RESTORE_URL" -Atqc \
  "SELECT payload FROM eubl21_dr_qualification WHERE id = 128" | \
  grep -Fxq 'eubl21-yugabytedb-dr-qualification'

echo "YugabyteDB DR drill passed; retained backup: $backup"
