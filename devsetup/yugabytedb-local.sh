#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/devsetup/lib/docker-engine.sh"
COMPOSE_FILE="$ROOT_DIR/deploy/local/yugabytedb-compose.yml"
PROJECT_NAME="eubl21-yugabytedb-local"
SERVICE_NAME="yugabytedb"
STORAGE_CHECK_SERVICE="storage-check"
YSQL_HOST="127.0.0.1"
YSQL_PORT="${YB_LOCAL_YSQL_PORT:-5433}"
YSQL_USER="eubl21"
YSQL_PASSWORD="eubl21-local-only"
YSQL_MIGRATION_USER="eubl21_migrator"
YSQL_MIGRATION_PASSWORD="eubl21-migrator-local-only"
YSQL_DATABASE="eubl21"
READY_TIMEOUT="${YB_LOCAL_READY_TIMEOUT:-120}"
EXPECTED_VERSION="2025.2.6.0-b111"
SCRIPT_PATH="$ROOT_DIR/devsetup/yugabytedb-local.sh"
COMPOSE_READY=0
MIN_FREE_KIB=2097152

usage() {
    cat <<'USAGE'
Usage: devsetup/yugabytedb-local.sh COMMAND

Commands:
  up          Start the pinned local YugabyteDB and wait for a smoke query.
  wait        Wait for authenticated YSQL readiness.
  smoke       Check the exact YugabyteDB build and execute SELECT 1.
  migrate     Apply pending embedded schema migrations.
  test        Start the runtime and run database, guarded-write, and upload-metadata integration tests.
  stop        Stop the container while retaining its persistent volume.
  restart     Restart the container and verify readiness.
  reset       Remove the local container, network, and persistent volume.
  verify      Destructively verify clean start, timeout handling, persistence,
              reset semantics, and application-package connectivity.
  url         Print the local application/test database URL.
USAGE
}

die() {
    echo "ERROR: $*" >&2
    exit 1
}

require_timeout() {
    [[ "$READY_TIMEOUT" =~ ^[1-9][0-9]*$ ]] ||
        die "YB_LOCAL_READY_TIMEOUT must be a positive integer number of seconds"
}

detect_compose() {
    (( COMPOSE_READY == 0 )) || return 0
    require_docker_compose || exit 1
    COMPOSE=(docker compose)
    COMPOSE_READY=1
}

require_free_kib() {
    local available_kib="$1"
    local filesystem_name="$2"
    [[ "$available_kib" =~ ^[0-9]+$ ]] ||
        die "could not determine free space on the $filesystem_name filesystem"
    (( available_kib >= MIN_FREE_KIB )) ||
        die "at least 2 GiB must be free on the $filesystem_name filesystem to start YugabyteDB"
    printf 'Disk preflight: %s has %s MiB free.\n' \
        "$filesystem_name" "$((available_kib / 1024))"
}

require_free_space() {
    local repository_available_kib volume_available_kib
    repository_available_kib="$(df -Pk "$ROOT_DIR" | awk 'NR == 2 {print $4}')"
    require_free_kib "$repository_available_kib" "repository"

    # Query the filesystem through the actual service volume. This remains
    # accurate when Docker storage is on another disk or a remote host.
    volume_available_kib="$(
        compose run --rm --no-deps "$STORAGE_CHECK_SERVICE" \
            -Pk /home/yugabyte/yb_data |
            awk '$4 ~ /^[0-9]+$/ {available = $4} END {print available}'
    )"
    require_free_kib "$volume_available_kib" "container-volume"
}

compose() {
    detect_compose
    YB_LOCAL_YSQL_PORT="$YSQL_PORT" \
        "${COMPOSE[@]}" --project-name "$PROJECT_NAME" --file "$COMPOSE_FILE" "$@"
}

database_url() {
    printf 'postgresql://%s:%s@%s:%s/%s?sslmode=disable\n' \
        "$YSQL_USER" "$YSQL_PASSWORD" "$YSQL_HOST" "$YSQL_PORT" "$YSQL_DATABASE"
}

migration_database_url() {
    printf 'postgresql://%s:%s@%s:%s/%s?sslmode=disable\n' \
        "$YSQL_MIGRATION_USER" "$YSQL_MIGRATION_PASSWORD" "$YSQL_HOST" "$YSQL_PORT" "$YSQL_DATABASE"
}

admin_database_url() {
    printf 'postgresql://yugabyte:yugabyte@%s:%s/yugabyte?sslmode=disable\n' \
        "$YSQL_HOST" "$YSQL_PORT"
}

run_migrations() {
    EUBL21_DATABASE_URL="$(migration_database_url)" \
        go run . database migrate
}

ysql_as() {
    local user="$1"
    local password="$2"
    local database="$3"
    local statement="$4"
    require_timeout
    detect_compose
    local attempt_timeout=15
    if (( READY_TIMEOUT < attempt_timeout )); then
        attempt_timeout="$READY_TIMEOUT"
    fi
    timeout --foreground "${attempt_timeout}s" \
        env YB_LOCAL_YSQL_PORT="$YSQL_PORT" \
        "${COMPOSE[@]}" --project-name "$PROJECT_NAME" --file "$COMPOSE_FILE" \
        exec -T \
        -e "PGPASSWORD=$password" \
        "$SERVICE_NAME" \
        /home/yugabyte/bin/ysqlsh \
        --host eubl21-yugabytedb \
        --port 5433 \
        --username "$user" \
        --dbname "$database" \
        --no-psqlrc \
        --no-align \
        --tuples-only \
        --command "$statement"
}

ysql() {
    ysql_as "$YSQL_USER" "$YSQL_PASSWORD" "$YSQL_DATABASE" "$1"
}

admin_ysql() {
    ysql_as yugabyte yugabyte yugabyte "$1"
}

wait_admin_ready() {
    require_timeout
    local result
    local started_at=$SECONDS
    while (( SECONDS - started_at < READY_TIMEOUT )); do
        if result="$(admin_ysql 'SELECT 1' 2>/dev/null)" && grep -Fxq '1' <<<"$result"; then
            return 0
        fi
        sleep 2
    done
    compose logs --tail=100 "$SERVICE_NAME" >&2 || true
    die "YugabyteDB admin YSQL did not become ready within ${READY_TIMEOUT}s"
}

wait_ready() {
    require_timeout
    local result
    local started_at=$SECONDS
    while (( SECONDS - started_at < READY_TIMEOUT )); do
        if result="$(ysql 'SELECT 1' 2>/dev/null)" && grep -Fxq '1' <<<"$result"; then
            echo "YugabyteDB YSQL is ready on $YSQL_HOST:$YSQL_PORT."
            return 0
        fi
        sleep 2
    done
    compose logs --tail=100 "$SERVICE_NAME" >&2 || true
    die "YugabyteDB did not become ready within ${READY_TIMEOUT}s"
}

smoke() {
    local sql_version runtime_version one
    sql_version="$(ysql 'SELECT version()')"
    [[ "$sql_version" == *"YugabyteDB"* || "$sql_version" == *"-YB-"* ]] ||
        die "the local endpoint did not identify itself as YugabyteDB"
    detect_compose
    runtime_version="$(timeout --foreground 15s \
        env YB_LOCAL_YSQL_PORT="$YSQL_PORT" \
        "${COMPOSE[@]}" --project-name "$PROJECT_NAME" --file "$COMPOSE_FILE" \
        exec -T "$SERVICE_NAME" /home/yugabyte/bin/yugabyted version)"
    [[ "$runtime_version" == *"$EXPECTED_VERSION"* ]] ||
        die "the container runtime is not YugabyteDB $EXPECTED_VERSION"
    one="$(ysql 'SELECT 1')"
    grep -Fxq '1' <<<"$one" || die "YSQL smoke query returned an unexpected result"
    echo "YugabyteDB $EXPECTED_VERSION smoke query passed."
}

up() {
    require_free_space
    compose up --detach "$SERVICE_NAME"
    wait_admin_ready
    compose run --rm bootstrap
    wait_ready
    run_migrations
    smoke
}

stop_runtime() {
    # Compose records an explicit stop and gives YugabyteDB its full configured
    # grace period, keeping it unavailable until the next up/restart command.
    compose stop --timeout 120 "$SERVICE_NAME"
}

reset_runtime() {
    compose down --volumes --remove-orphans
}

run_integration_test() {
    up
    EUBL21_TEST_DATABASE_URL="$(database_url)" \
    EUBL21_TEST_DATABASE_ADMIN_URL="$(admin_database_url)" \
        go test ./database -run '^TestYugabyteDB(Integration|Migrations)$' -count=1
    EUBL21_TEST_DATABASE_URL="$(database_url)" \
        go test ./documentstore -run '^TestGuardedWriteClaims$' -count=1
	EUBL21_TEST_DATABASE_URL="$(database_url)" \
		go test ./queue -run '^TestYSQLUploadMetadataStore$' -count=1
}

verify_lifecycle() {
    echo "WARNING: verify removes all state in the $PROJECT_NAME Compose project." >&2
    reset_runtime
    trap reset_runtime EXIT

    up
    ysql "CREATE TABLE issue_121_persistence_probe (id integer PRIMARY KEY, value text NOT NULL); INSERT INTO issue_121_persistence_probe VALUES (1, 'preserved');" >/dev/null

    stop_runtime
    if YB_LOCAL_READY_TIMEOUT=2 "$SCRIPT_PATH" wait >/dev/null 2>&1; then
        die "readiness unexpectedly succeeded while YugabyteDB was stopped"
    fi

    up
    local preserved
    preserved="$(ysql 'SELECT value FROM issue_121_persistence_probe WHERE id = 1')"
    grep -Fxq 'preserved' <<<"$preserved" || die "persistent data was not preserved across restart"

    reset_runtime
    up
    local reset_relation
    reset_relation="$(ysql "SELECT COALESCE(to_regclass('public.issue_121_persistence_probe')::text, '')")"
    [[ -z "$reset_relation" ]] || die "clean reset retained test state"

    EUBL21_TEST_DATABASE_URL="$(database_url)" \
    EUBL21_TEST_DATABASE_ADMIN_URL="$(admin_database_url)" \
        go test ./database -run '^TestYugabyteDB(Integration|Migrations)$' -count=1
    echo "Local YugabyteDB lifecycle verification passed."
}

command="${1:-}"
case "$command" in
    up) up ;;
    wait) wait_ready ;;
    smoke) smoke ;;
    migrate) run_migrations ;;
    test) run_integration_test ;;
    stop) stop_runtime ;;
    restart)
        stop_runtime
        up
        ;;
    reset) reset_runtime ;;
    verify) verify_lifecycle ;;
    url) database_url ;;
    -h|--help|help) usage ;;
    *)
        usage >&2
        [[ -n "$command" ]] && echo "ERROR: unknown command: $command" >&2
        exit 2
        ;;
esac
