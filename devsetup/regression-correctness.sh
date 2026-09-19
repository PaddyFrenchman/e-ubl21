#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

RUN_STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARTIFACT_DIR="${ARTIFACT_DIR:-regression-results/correctness/$RUN_STAMP}"
API_LOCAL_PORT="${API_LOCAL_PORT:-18081}"
WORKER_LOCAL_PORT="${WORKER_LOCAL_PORT:-19092}"
NATS_LOCAL_PORT="${NATS_LOCAL_PORT:-14223}"
NATS_MONITOR_PORT="${NATS_MONITOR_PORT:-18223}"
QUEUE_NAMESPACE="${QUEUE_NAMESPACE:-eubl21.correctness}"
SOURCE_TIMEOUT="${SOURCE_TIMEOUT:-30s}"
ISSUER="https://correctness.e-ubl21.local"
AUDIENCE="e-ubl21-correctness"
NATS_CONTAINER="e-ubl21-correctness-${RUN_STAMP//[^a-zA-Z0-9]/}-$$"
SAXON_DIR="$ROOT_DIR/devsetup/tools/saxon"
SAXON_JAR="$SAXON_DIR/saxon-he-12.7.jar"
RESOLVER_JAR="$SAXON_DIR/xmlresolver-6.0.17.jar"
SAXON_CONFIG="$ROOT_DIR/validation-assets/saxon/saxon-config.xml"
EN16931_XSLT="$ROOT_DIR/validation-assets/en16931/1.3.16/EN16931-UBL-validation.xslt"

for command in go java xmllint docker curl diff; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "ERROR: required command not found: $command" >&2
        exit 2
    fi
done

mkdir -p "$ARTIFACT_DIR/logs"
ARTIFACT_DIR="$(cd "$ARTIFACT_DIR" && pwd)"
TEMP_DIR="$(mktemp -d)"
SERVICE_PIDS=()

cleanup() {
    local status=$?
    trap - EXIT INT TERM
    set +e
    for pid in "${SERVICE_PIDS[@]}"; do
        kill "$pid" >/dev/null 2>&1 || true
    done
    for pid in "${SERVICE_PIDS[@]}"; do
        wait "$pid" >/dev/null 2>&1 || true
    done
    docker stop "$NATS_CONTAINER" >/dev/null 2>&1 || true
    rm -rf -- "$TEMP_DIR"
    unset EUBL21_REGRESSION_HMAC_SECRET EUBL21_OAUTH_HMAC_SECRET
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if [[ ! -f "$SAXON_JAR" || ! -f "$RESOLVER_JAR" ]]; then
    "$ROOT_DIR/devsetup/download_saxon_he.sh" "$SAXON_DIR"
fi
go run ./tools/verify-validation-assets -saxon-dir "$SAXON_DIR"

verify_corpus() {
    local profile=$1
    local tracked=$2
    local generated="$TEMP_DIR/$profile"
    local diff_log="$ARTIFACT_DIR/corpus-$profile.diff"
    local manifest="$tracked/manifest.json"
    local versions=()

    mapfile -t versions < <(sed -n 's/.*"generatorVersion": "\(git:[a-f0-9]*\)".*/\1/p' "$manifest" | sort -u)
    if (( ${#versions[@]} != 1 )); then
        echo "ERROR: expected exactly one generated-corpus provenance version in $manifest" >&2
        return 1
    fi
    go run ./createsamples generate \
        --profile "$profile" \
        --set all \
        --xsd XSD \
        --generator-version "${versions[0]}" \
        --output "$generated"
    if ! diff --recursive --brief "$tracked" "$generated" | tee "$diff_log"; then
        echo "ERROR: regenerated $profile corpus differs from $tracked" >&2
        return 1
    fi
    echo "Verified deterministic $profile corpus and fixture hashes"
}

verify_corpus ubl21 createsamples/samples
verify_corpus en16931 createsamples/samples-en16931

go build -trimpath -o "$TEMP_DIR/e-ubl21" .
docker run --detach --rm \
    --name "$NATS_CONTAINER" \
    --network host \
    docker.io/library/nats:2.14.5-alpine \
    -js -sd /data -p "$NATS_LOCAL_PORT" -m "$NATS_MONITOR_PORT" >/dev/null

wait_http() {
    local name=$1
    local url=$2
    local auth=${3:-}
    local pid=${4:-}
    local deadline=$((SECONDS + 90))
    while (( SECONDS < deadline )); do
        if [[ -n "$pid" ]] && ! kill -0 "$pid" >/dev/null 2>&1; then
            echo "ERROR: $name process exited before $url became healthy" >&2
            return 1
        fi
        if [[ -n "$auth" ]]; then
            if curl --fail --silent --show-error --max-time 2 -H "Authorization: Bearer $auth" "$url" >/dev/null 2>&1; then
                return 0
            fi
        elif curl --fail --silent --show-error --max-time 2 "$url" >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
    done
    echo "ERROR: timed out waiting for $name at $url" >&2
    return 1
}

wait_http nats "http://127.0.0.1:$NATS_MONITOR_PORT/healthz"

HMAC_SECRET="$(go run ./tools/regression-token-secret)"
export EUBL21_REGRESSION_HMAC_SECRET="$HMAC_SECRET"
TOKEN="$(go run ./tools/regression-token --issuer "$ISSUER" --audience "$AUDIENCE" --ttl 2h)"
unset EUBL21_REGRESSION_HMAC_SECRET

export EUBL21_NATS_URL="nats://127.0.0.1:$NATS_LOCAL_PORT"
export EUBL21_QUEUE_NAMESPACE="$QUEUE_NAMESPACE"
export EUBL21_UPLOAD_CONTENT_RETENTION=1h
export EUBL21_UPLOAD_CONTENT_MAX_BYTES=1073741824

EUBL21_HOST=127.0.0.1 \
EUBL21_PORT="$API_LOCAL_PORT" \
EUBL21_OAUTH_ISSUER="$ISSUER" \
EUBL21_OAUTH_AUDIENCE="$AUDIENCE" \
EUBL21_OAUTH_HMAC_SECRET="$HMAC_SECRET" \
EUBL21_RATE_LIMIT_RPS=1000 \
EUBL21_RATE_LIMIT_BURST=2000 \
    "$TEMP_DIR/e-ubl21" serve >"$ARTIFACT_DIR/logs/api.log" 2>&1 &
API_PID=$!
SERVICE_PIDS+=("$API_PID")

EUBL21_WORKER_CONSUMER_NAME=correctness-validator \
EUBL21_WORKER_METRICS_ADDR="127.0.0.1:$WORKER_LOCAL_PORT" \
EUBL21_WORKER_VALIDATION_TIMEOUT=60s \
EUBL21_WORKER_XSD_VALIDATION_TIMEOUT=30s \
EUBL21_WORKER_EN16931_VALIDATION_TIMEOUT=45s \
EUBL21_WORKER_JAVA_EXECUTABLE="$(command -v java)" \
EUBL21_WORKER_SAXON_JAR_PATH="$SAXON_JAR" \
EUBL21_WORKER_XMLRESOLVER_JAR_PATH="$RESOLVER_JAR" \
EUBL21_WORKER_SAXON_CONFIGURATION_PATH="$SAXON_CONFIG" \
EUBL21_WORKER_EN16931_XSLT_PATH="$EN16931_XSLT" \
EUBL21_WORKER_SAXON_STARTUP_TIMEOUT=30s \
EUBL21_WORKER_CONCURRENCY=1 \
EUBL21_WORKER_PREFETCH=1 \
EUBL21_WORKER_ACK_WAIT=90s \
    "$TEMP_DIR/e-ubl21" worker >"$ARTIFACT_DIR/logs/worker.log" 2>&1 &
WORKER_PID=$!
SERVICE_PIDS+=("$WORKER_PID")

unset HMAC_SECRET
wait_http api "http://127.0.0.1:$API_LOCAL_PORT/healthz" "$TOKEN" "$API_PID"
wait_http worker "http://127.0.0.1:$WORKER_LOCAL_PORT/healthz" "" "$WORKER_PID"
STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

RUNNER_STATUS=0
EQUIVALENCE_STATUS=0
EVIDENCE_STATUS=0
run_corpus() {
    local name=$1
    local manifest=$2
    local output="$ARTIFACT_DIR/$name"
    local status

    mkdir -p "$output"
    set +e
    "$TEMP_DIR/e-ubl21" regression \
        --base-url "http://127.0.0.1:$API_LOCAL_PORT" \
        --token "$TOKEN" \
        --manifest "$manifest" \
        --profile all \
        --output "$output" 2>&1 | tee "$output/runner.log"
    status=${PIPESTATUS[0]}
    set -e
    if (( status != 0 )); then
        RUNNER_STATUS=1
    fi

    set +e
    "$TEMP_DIR/e-ubl21" regression equivalence \
        --baseline "$ROOT_DIR/regression/baselines/validation-split/$name.results.jsonl" \
        --candidate "$output/results.jsonl" \
        --output "$output/equivalence.json" 2>&1 | tee "$output/equivalence.log"
    status=${PIPESTATUS[0]}
    set -e
    if (( status != 0 )); then
        EQUIVALENCE_STATUS=1
    fi

    set +e
    "$TEMP_DIR/e-ubl21" \
        --nats-url "nats://127.0.0.1:$NATS_LOCAL_PORT" \
        --queue-namespace "$QUEUE_NAMESPACE" \
        regression collect-evidence \
        --manifest "$manifest" \
        --output "$output" \
        --token "$TOKEN" \
        --log "api=$ARTIFACT_DIR/logs/api.log" \
        --log "worker=$ARTIFACT_DIR/logs/worker.log" \
        --metrics "api=http://127.0.0.1:$API_LOCAL_PORT/metrics" \
        --metrics "worker=http://127.0.0.1:$WORKER_LOCAL_PORT/metrics" \
        --nats-state \
        --started-at "$STARTED_AT" \
        --source-timeout "$SOURCE_TIMEOUT" 2>&1 | tee "$output/evidence.log"
    status=${PIPESTATUS[0]}
    set -e
    if (( status != 0 )); then
        EVIDENCE_STATUS=1
    fi
}

run_corpus ubl21 "$ROOT_DIR/createsamples/samples/manifest.json"
run_corpus en16931 "$ROOT_DIR/createsamples/samples-en16931/manifest.json"

echo "Correctness regression artifacts: $ARTIFACT_DIR"
if (( RUNNER_STATUS != 0 )); then
    echo "ERROR: one or more semantic regression corpora failed" >&2
    exit 1
fi
if (( EQUIVALENCE_STATUS != 0 )); then
    echo "ERROR: one or more corpora differ from the frozen monolithic baseline" >&2
    exit 1
fi
if (( EVIDENCE_STATUS != 0 )); then
    echo "ERROR: one or more evidence collections failed" >&2
    exit 1
fi
