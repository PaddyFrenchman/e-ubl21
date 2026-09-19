#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/devsetup/lib/docker-engine.sh"
cd "$ROOT_DIR"

MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-e-ubl21-regression}"
MINIKUBE_DRIVER="${MINIKUBE_DRIVER:-docker}"
MINIKUBE_MEMORY="${MINIKUBE_MEMORY:-6144mb}"
MINIKUBE_STATUS_TIMEOUT_SECONDS="${MINIKUBE_STATUS_TIMEOUT_SECONDS:-60}"
MINIKUBE_START_TIMEOUT_SECONDS="${MINIKUBE_START_TIMEOUT_SECONDS:-300}"
require_docker_engine
if [[ "$MINIKUBE_DRIVER" == "podman" ]]; then
    export MINIKUBE_ROOTLESS="${MINIKUBE_ROOTLESS:-true}"
fi
NAMESPACE="e-ubl21-regression"
QUEUE_NAMESPACE="eubl21.regression"
IMAGE="${IMAGE:-localhost/e-ubl21:regression}"
MANIFEST="${MANIFEST:-createsamples/samples-en16931/manifest.json}"
PROFILE="${PROFILE:-all}"
API_LOCAL_PORT="${API_LOCAL_PORT:-18080}"
WORKER_LOCAL_PORT="${WORKER_LOCAL_PORT:-19091}"
NATS_LOCAL_PORT="${NATS_LOCAL_PORT:-14222}"
SOURCE_TIMEOUT="${SOURCE_TIMEOUT:-30s}"
ROLLOUT_TIMEOUT="${ROLLOUT_TIMEOUT:-5m}"
RUN_STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARTIFACT_DIR="${ARTIFACT_DIR:-regression-results/minikube/$RUN_STAMP}"
KUBECTL_BIN="${KUBECTL_BIN:-$ROOT_DIR/devsetup/minikube-kubectl.sh}"
KEEP_DEPLOYMENT="${KEEP_DEPLOYMENT:-0}"
EQUIVALENCE_BASELINE="${EQUIVALENCE_BASELINE:-}"

for command in minikube docker go curl helm sha256sum setsid; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "ERROR: required command not found: $command" >&2
        exit 2
    fi
done
if [[ ! -f "$MANIFEST" ]]; then
    echo "ERROR: manifest not found: $MANIFEST" >&2
    exit 2
fi

mkdir -p "$ARTIFACT_DIR/port-forward"
ARTIFACT_DIR="$(cd "$ARTIFACT_DIR" && pwd)"
MANIFEST="$(cd "$(dirname "$MANIFEST")" && pwd)/$(basename "$MANIFEST")"
if [[ -z "$EQUIVALENCE_BASELINE" ]]; then
    case "$MANIFEST" in
        "$ROOT_DIR/createsamples/samples/manifest.json")
            EQUIVALENCE_BASELINE="$ROOT_DIR/regression/baselines/validation-split/ubl21.results.jsonl"
            ;;
        "$ROOT_DIR/createsamples/samples-en16931/manifest.json")
            EQUIVALENCE_BASELINE="$ROOT_DIR/regression/baselines/validation-split/en16931.results.jsonl"
            ;;
    esac
fi
export EUBL21_MINIKUBE_PROFILE="$MINIKUBE_PROFILE"

PORT_FORWARD_PROCESS_GROUPS=()
DEPLOYMENT_STARTED=0
EVIDENCE_ATTEMPTED=0

collect_failure_evidence() {
    local image_digest
    local evidence_status

    echo "Collecting Kubernetes failure evidence before cleanup"
    image_digest="$(docker image inspect --format '{{.Id}}' "$IMAGE" 2>/dev/null)"
    go run . regression collect-evidence \
        --manifest "$MANIFEST" \
        --output "$ARTIFACT_DIR/failure-evidence" \
        --kubernetes \
        --kubernetes-namespace "$NAMESPACE" \
        --kubernetes-selector app.kubernetes.io/part-of=e-ubl21 \
        --kubectl-executable "$KUBECTL_BIN" \
        --minikube-executable minikube \
        --image-digest "$image_digest" \
        --source-timeout "$SOURCE_TIMEOUT" \
        > >(tee "$ARTIFACT_DIR/failure-evidence.log") 2>&1
    evidence_status=$?
    if (( evidence_status != 0 )); then
        echo "WARNING: failure evidence collection was incomplete (status $evidence_status)" >&2
    fi
}

cleanup() {
    local status=$?
    trap - EXIT INT TERM
    set +e
    for process_group in "${PORT_FORWARD_PROCESS_GROUPS[@]}"; do
        # `minikube kubectl` may exit after spawning kubectl, which leaves the
        # port-forward process re-parented. Kill its dedicated process group so
        # sequential regression runs can safely reuse the same local ports.
        kill -- "-$process_group" >/dev/null 2>&1 || true
        wait "$process_group" >/dev/null 2>&1 || true
    done
    if (( status != 0 && DEPLOYMENT_STARTED == 1 && EVIDENCE_ATTEMPTED == 0 )); then
        collect_failure_evidence
    fi
    if [[ "$KEEP_DEPLOYMENT" != "1" ]]; then
        "$KUBECTL_BIN" delete namespace "$NAMESPACE" --ignore-not-found=true --wait=true --timeout=2m >/dev/null 2>&1 || true
    else
        echo "Retaining namespace $NAMESPACE because KEEP_DEPLOYMENT=1"
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

reset_minikube_profile() {
    local container_ids

    minikube delete -p "$MINIKUBE_PROFILE" >/dev/null 2>&1 || true
    # A corrupt Docker-driver profile can have an empty node IP, causing
    # minikube delete to leave its control-plane container behind. The label is
    # owned by this exact disposable regression profile, so this cleanup cannot
    # affect unrelated Docker containers.
    container_ids="$(docker ps --all --quiet --filter "label=mode.minikube.sigs.k8s.io=$MINIKUBE_PROFILE")"
    if [[ -n "$container_ids" ]]; then
        docker rm --force $container_ids >/dev/null
    fi
}

if ! timeout "$MINIKUBE_STATUS_TIMEOUT_SECONDS" minikube status -p "$MINIKUBE_PROFILE" --format='{{.Host}}' 2>/dev/null | grep -qx Running; then
    # This profile is disposable regression infrastructure. Recreate it when
    # Minikube cannot establish a timely control-plane connection instead of
    # allowing an unusable Docker-driver profile to hang the whole suite.
    reset_minikube_profile
    timeout "$MINIKUBE_START_TIMEOUT_SECONDS" minikube start -p "$MINIKUBE_PROFILE" --driver="$MINIKUBE_DRIVER" --memory="$MINIKUBE_MEMORY"
fi

echo "Building candidate image $IMAGE"
docker build --tag "$IMAGE" .
docker save "$IMAGE" | minikube -p "$MINIKUBE_PROFILE" image load -

HMAC_SECRET="$(go run ./tools/regression-token-secret)"
export EUBL21_REGRESSION_HMAC_SECRET="$HMAC_SECRET"
TOKEN="$(go run ./tools/regression-token --issuer https://regression.e-ubl21.local --audience e-ubl21-regression --ttl 2h)"
unset EUBL21_REGRESSION_HMAC_SECRET

"$KUBECTL_BIN" delete namespace "$NAMESPACE" --ignore-not-found=true --wait=true --timeout=2m
"$KUBECTL_BIN" apply -f deploy/minikube/namespace.yaml
DEPLOYMENT_STARTED=1
"$KUBECTL_BIN" --namespace "$NAMESPACE" create secret generic e-ubl21-regression-auth \
    --from-literal="hmac-secret=$HMAC_SECRET" --dry-run=client -o yaml | "$KUBECTL_BIN" apply -f -
unset HMAC_SECRET
EUBL21_DB_NAMESPACE="$NAMESPACE" \
    IMAGE="$IMAGE" \
    KUBECTL_BIN="$KUBECTL_BIN" \
    ARTIFACT_DIR="$ARTIFACT_DIR" \
    ROLLOUT_TIMEOUT="$ROLLOUT_TIMEOUT" \
    "$ROOT_DIR/devsetup/minikube-yugabytedb.sh"
"$KUBECTL_BIN" apply -k deploy/minikube
"$KUBECTL_BIN" --namespace "$NAMESPACE" set image deployment/e-ubl21-api "api=$IMAGE"
"$KUBECTL_BIN" --namespace "$NAMESPACE" set image deployment/e-ubl21-worker "worker=$IMAGE"
"$KUBECTL_BIN" --namespace "$NAMESPACE" rollout restart deployment/e-ubl21-api
"$KUBECTL_BIN" --namespace "$NAMESPACE" rollout restart deployment/e-ubl21-worker

for deployment in nats e-ubl21-api e-ubl21-worker; do
    "$KUBECTL_BIN" --namespace "$NAMESPACE" rollout status "deployment/$deployment" --timeout="$ROLLOUT_TIMEOUT"
done

start_port_forward() {
    local name=$1
    local resource=$2
    local ports=$3
    local log="$ARTIFACT_DIR/port-forward/$name.log"

    setsid "$KUBECTL_BIN" --namespace "$NAMESPACE" port-forward "$resource" "$ports" >"$log" 2>&1 &
    PORT_FORWARD_PROCESS_GROUPS+=("$!")
}

wait_port_forward() {
    local name=$1
    local process_group=$2
    local log="$ARTIFACT_DIR/port-forward/$name.log"
    local deadline=$((SECONDS + 30))

    while (( SECONDS < deadline )); do
        if grep -q '^Forwarding from ' "$log" 2>/dev/null; then
            return 0
        fi
        if ! kill -0 -- "-$process_group" 2>/dev/null; then
            echo "ERROR: $name port-forward exited before becoming ready" >&2
            sed 's/^/  /' "$log" >&2
            return 1
        fi
        sleep 1
    done
    echo "ERROR: timed out waiting for $name port-forward to become ready" >&2
    sed 's/^/  /' "$log" >&2
    return 1
}

start_port_forward api service/e-ubl21-api "$API_LOCAL_PORT:8080"
start_port_forward worker service/e-ubl21-worker "$WORKER_LOCAL_PORT:9091"
start_port_forward nats service/nats "$NATS_LOCAL_PORT:4222"

wait_port_forward api "${PORT_FORWARD_PROCESS_GROUPS[0]}"
wait_port_forward worker "${PORT_FORWARD_PROCESS_GROUPS[1]}"
wait_port_forward nats "${PORT_FORWARD_PROCESS_GROUPS[2]}"

wait_http() {
    local name=$1
    local url=$2
    local auth=${3:-}
    local deadline=$((SECONDS + 60))
    while (( SECONDS < deadline )); do
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

wait_http api "http://127.0.0.1:$API_LOCAL_PORT/healthz" "$TOKEN"
wait_http worker "http://127.0.0.1:$WORKER_LOCAL_PORT/healthz"
STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

set +e
go run . regression \
    --base-url "http://127.0.0.1:$API_LOCAL_PORT" \
    --token "$TOKEN" \
    --manifest "$MANIFEST" \
    --profile "$PROFILE" \
    --output "$ARTIFACT_DIR" 2>&1 | tee "$ARTIFACT_DIR/runner.log"
RUNNER_STATUS=${PIPESTATUS[0]}

EQUIVALENCE_STATUS=0
if [[ -n "$EQUIVALENCE_BASELINE" ]]; then
    go run . regression equivalence \
        --baseline "$EQUIVALENCE_BASELINE" \
        --candidate "$ARTIFACT_DIR/results.jsonl" \
        --output "$ARTIFACT_DIR/equivalence.json" 2>&1 | tee "$ARTIFACT_DIR/equivalence.log"
    EQUIVALENCE_STATUS=${PIPESTATUS[0]}
else
    echo "No frozen equivalence baseline is registered for $MANIFEST; semantic manifest checks still apply" | tee "$ARTIFACT_DIR/equivalence.log"
fi

IMAGE_DIGEST="$(docker image inspect --format '{{.Id}}' "$IMAGE" 2>/dev/null)"
EVIDENCE_ATTEMPTED=1
go run . \
    --nats-url "nats://127.0.0.1:$NATS_LOCAL_PORT" \
    --queue-namespace "$QUEUE_NAMESPACE" \
    regression collect-evidence \
    --manifest "$MANIFEST" \
    --output "$ARTIFACT_DIR" \
    --token "$TOKEN" \
    --metrics "api=http://127.0.0.1:$API_LOCAL_PORT/metrics" \
    --metrics "worker=http://127.0.0.1:$WORKER_LOCAL_PORT/metrics" \
    --nats-state \
    --kubernetes \
    --kubernetes-namespace "$NAMESPACE" \
    --kubernetes-selector app.kubernetes.io/part-of=e-ubl21 \
    --kubectl-executable "$KUBECTL_BIN" \
    --minikube-executable minikube \
    --image-digest "$IMAGE_DIGEST" \
    --started-at "$STARTED_AT" \
    --source-timeout "$SOURCE_TIMEOUT" 2>&1 | tee "$ARTIFACT_DIR/evidence.log"
EVIDENCE_STATUS=${PIPESTATUS[0]}
set -e

echo "Regression artifacts: $ARTIFACT_DIR"
if (( RUNNER_STATUS != 0 )); then
    echo "ERROR: semantic regression runner failed with status $RUNNER_STATUS" >&2
    exit "$RUNNER_STATUS"
fi
if (( EQUIVALENCE_STATUS != 0 )); then
    echo "ERROR: candidate results differ from the frozen monolithic baseline" >&2
    exit "$EQUIVALENCE_STATUS"
fi
if (( EVIDENCE_STATUS != 0 )); then
    echo "ERROR: evidence collection failed with status $EVIDENCE_STATUS" >&2
    exit "$EVIDENCE_STATUS"
fi
