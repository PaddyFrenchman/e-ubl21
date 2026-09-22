#!/usr/bin/env bash
# Production YBC restore qualification. It deliberately has no logical-dump
# fallback: passing it is evidence for the production backup mechanism.
set -Eeuo pipefail

: "${EUBL21_DR_YBC_BACKUP_COMMAND:?set the platform YBC backup command}"
: "${EUBL21_DR_YBC_RESTORE_COMMAND:?set the platform YBC restore command}"
: "${EUBL21_DR_RECREATE_TARGET:?set command to destroy and recreate isolated target}"
: "${EUBL21_DR_APP_URL:?set restored target application-role YSQL URL}"
: "${EUBL21_DR_DATABASE:=eubl21}"
[[ "$EUBL21_DR_YBC_BACKUP_COMMAND" != *"ysql_dump"* && "$EUBL21_DR_YBC_RESTORE_COMMAND" != *"ysqlsh"* ]] || { echo "DR qualification requires YBC, not logical YSQL dump/restore" >&2; exit 2; }

# The command is supplied by the installed YugabyteDB Anywhere or Kubernetes
# operator integration. yb-admin cannot create object-store YBC backups.
bash -c "$EUBL21_DR_YBC_BACKUP_COMMAND"

# This deliberately explicit command is the required destroy -> recreate
# transition. It must wait until its new, clean target accepts yb-admin.
bash -c "$EUBL21_DR_RECREATE_TARGET"
bash -c "$EUBL21_DR_YBC_RESTORE_COMMAND"

# Proves real application-role access, grants and pgx behavior—not admin access.
EUBL21_TEST_DATABASE_URL="$EUBL21_DR_APP_URL" \
  go test ./database -run '^TestYugabyteDBIntegration$' -count=1
./run-regression-tests.sh

echo "YBC DR drill passed. Retain the platform backup ID and restore evidence."
