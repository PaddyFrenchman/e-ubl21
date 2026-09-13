#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/deploy/local/yugabytedb-compose.yml"
PROJECT_NAME="eubl21-yugabytedb-local"
SERVICE_NAME="yugabytedb"
YSQL_HOST="127.0.0.1"
YSQL_PORT="${YB_LOCAL_YSQL_PORT:-5433}"
YSQL_USER="eubl21"
YSQL_PASSWORD="eubl21-local-only"
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
  test        Start the runtime and run the database package integration test.
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
    if docker compose version >/dev/null 2>&1; then
        COMPOSE=(docker compose)
    elif command -v docker-compose >/dev/null 2>&1; then
        COMPOSE=(docker-compose)
    else
        die "Docker Compose is required (docker compose or docker-compose)"
    fi
    if ! YB_LOCAL_YSQL_PORT="$YSQL_PORT" \
        "${COMPOSE[@]}" --project-name "$PROJECT_NAME" --file "$COMPOSE_FILE" \
        ps >/dev/null 2>&1; then
        die "the Docker-compatible container engine is not reachable by Compose"
    fi
    COMPOSE_READY=1
}

require_free_space() {
    local available_kib
    available_kib="$(df -Pk "$ROOT_DIR" | awk 'NR == 2 {print $4}')"
    [[ "$available_kib" =~ ^[0-9]+$ ]] || die "could not determine available local disk space"
    (( available_kib >= MIN_FREE_KIB )) ||
        die "at least 2 GiB of free local disk space is required to start YugabyteDB"
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
    smoke
}

stop_runtime() {
    detect_compose
    # Ask yugabyted to stop its database processes cleanly before Compose
    # removes the container. This keeps the persisted universe restartable.
    local stop_status=0
    timeout --foreground 90s \
        env YB_LOCAL_YSQL_PORT="$YSQL_PORT" \
        "${COMPOSE[@]}" --project-name "$PROJECT_NAME" --file "$COMPOSE_FILE" \
        exec -T "$SERVICE_NAME" /home/yugabyte/bin/yugabyted stop \
        --base_dir=/home/yugabyte/yb_data || stop_status=$?
    # Docker reports 137 when yugabyted's successful stop terminates the
    # foreground container and consequently its exec session.
    if (( stop_status != 0 && stop_status != 137 )); then
        die "yugabyted graceful stop failed with status $stop_status"
    fi
    # Keep the stopped container and its named volume. Starting that same
    # container preserves both its stable network identity and database state.
}

reset_runtime() {
    compose down --volumes --remove-orphans
}

run_integration_test() {
    up
    EUBL21_TEST_DATABASE_URL="$(database_url)" \
        go test ./database -run '^TestYugabyteDBIntegration$' -count=1
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
        go test ./database -run '^TestYugabyteDBIntegration$' -count=1
    echo "Local YugabyteDB lifecycle verification passed."
}

command="${1:-}"
case "$command" in
    up) up ;;
    wait) wait_ready ;;
    smoke) smoke ;;
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
