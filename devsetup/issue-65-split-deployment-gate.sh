#!/usr/bin/env bash
# Issue #65 split-deployment end-to-end validation gate
# Reuses the regression-minikube.sh foundation but deploys split architecture
# (API, orchestrator, XSD worker, EN worker) and tests issue #65 scenarios:
# - EN scale-to-zero with backlog accumulation
# - Worker restart and recovery
# - Semantic equivalence from frozen corpus
#
# Usage:
#   ./devsetup/issue-65-split-deployment-gate.sh
#
# Environment: Same as regression-minikube.sh
#   MINIKUBE_PROFILE, MINIKUBE_DRIVER, IMAGE, MANIFEST, ARTIFACT_DIR, etc.

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# Validate prerequisites before starting
for cmd in minikube docker go curl jq kubectl; do
	if ! command -v "$cmd" >/dev/null 2>&1; then
		echo "ERROR: required command not found: $cmd" >&2
		exit 2
	fi
done

# Reuse regression-minikube.sh configuration
MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-e-ubl21-i65-split}"
if [[ -z "${MINIKUBE_DRIVER:-}" ]]; then
	DOCKER_VERSION="$(docker --version 2>&1)"
	if [[ "${DOCKER_VERSION,,}" == *podman* ]]; then
		MINIKUBE_DRIVER=podman
	else
		MINIKUBE_DRIVER=docker
	fi
fi
if [[ "$MINIKUBE_DRIVER" == "podman" ]]; then
	export MINIKUBE_ROOTLESS="${MINIKUBE_ROOTLESS:-true}"
fi

NAMESPACE="e-ubl21-i65-split"
QUEUE_NAMESPACE="eubl21.i65-split"
IMAGE="${IMAGE:-localhost/e-ubl21:i65-split-gate}"
MANIFEST="${MANIFEST:-createsamples/samples-en16931/manifest.json}"
API_LOCAL_PORT="${API_LOCAL_PORT:-18081}"
NATS_LOCAL_PORT="${NATS_LOCAL_PORT:-14223}"
SOURCE_TIMEOUT="${SOURCE_TIMEOUT:-30s}"
ROLLOUT_TIMEOUT="${ROLLOUT_TIMEOUT:-5m}"
ARTIFACT_DIR="${ARTIFACT_DIR:-regression-results/issue-65-split/$(/usr/bin/date -u +%Y%m%dT%H%M%SZ)}"
KUBECTL_BIN="${KUBECTL_BIN:-$ROOT_DIR/devsetup/minikube-kubectl.sh}"
KEEP_DEPLOYMENT="${KEEP_DEPLOYMENT:-0}"

if [[ ! -f "$MANIFEST" ]]; then
	echo "ERROR: manifest not found: $MANIFEST" >&2
	exit 2
fi

mkdir -p "$ARTIFACT_DIR/port-forward"
ARTIFACT_DIR="$(cd "$ARTIFACT_DIR" && pwd)"
MANIFEST="$(cd "$(dirname "$MANIFEST")" && pwd)/$(basename "$MANIFEST")"

# Cleanup
PORT_FORWARD_PIDS=()
DEPLOYMENT_STARTED=0

cleanup() {
	local status=$?
	trap - EXIT INT TERM
	set +e
	for pid in "${PORT_FORWARD_PIDS[@]}"; do
		kill "$pid" >/dev/null 2>&1 || true
		wait "$pid" >/dev/null 2>&1 || true
	done
	if (( status != 0 && DEPLOYMENT_STARTED == 1 )); then
		echo "Collecting failure logs from namespace $NAMESPACE"
		"$KUBECTL_BIN" get all -n "$NAMESPACE" >"$ARTIFACT_DIR/k8s-state.log" 2>&1 || true
		"$KUBECTL_BIN" describe pod -n "$NAMESPACE" >"$ARTIFACT_DIR/k8s-pod-describe.log" 2>&1 || true
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

echo "=== Issue #65 Split-Deployment E2E Gate ==="
echo "Artifact directory: $ARTIFACT_DIR"
echo ""

# Start Minikube
if ! minikube status -p "$MINIKUBE_PROFILE" --format='{{.Host}}' 2>/dev/null | grep -qx Running; then
	echo "Starting Minikube cluster $MINIKUBE_PROFILE"
	minikube start -p "$MINIKUBE_PROFILE" --driver="$MINIKUBE_DRIVER"
fi

echo "Building image $IMAGE"
docker build --tag "$IMAGE" . >"$ARTIFACT_DIR/docker-build.log" 2>&1
docker save "$IMAGE" | minikube -p "$MINIKUBE_PROFILE" image load -

# Generate auth token (matches regression-minikube.sh exactly)
echo "Generating auth token"
HMAC_SECRET="$(go run ./tools/regression-token-secret 2>/dev/null)"
export EUBL21_REGRESSION_HMAC_SECRET="$HMAC_SECRET"
TOKEN="$(go run ./tools/regression-token --issuer https://regression.e-ubl21.local --audience e-ubl21-regression --ttl 2h 2>/dev/null)"
unset EUBL21_REGRESSION_HMAC_SECRET

# Deploy using kubernetes manifests (not minikube simplified ones)
echo "Deploying split architecture to namespace $NAMESPACE using kubernetes manifests"
"$KUBECTL_BIN" apply -f deploy/kubernetes/namespace.yaml >"$ARTIFACT_DIR/apply-namespace.log" 2>&1
DEPLOYMENT_STARTED=1

# Create proper auth secret (matches regression-minikube.sh secret contents)
"$KUBECTL_BIN" --namespace "$NAMESPACE" create secret generic e-ubl21-regression-auth \
	--from-literal="hmac-secret=$HMAC_SECRET" --dry-run=client -o yaml | "$KUBECTL_BIN" apply -f -
unset HMAC_SECRET

# Create runtime config
"$KUBECTL_BIN" --namespace "$NAMESPACE" create configmap e-ubl21-runtime \
	--from-literal="EUBL21_QUEUE_NAMESPACE=$QUEUE_NAMESPACE" \
	--dry-run=client -o yaml | "$KUBECTL_BIN" apply -f -

# Apply split manifests, substituting image and namespace
echo "Applying split component manifests"
for manifest_file in api.yaml orchestrator.yaml orchestrator-reconciler.yaml worker-xsd.yaml worker-en16931.yaml runtime-config.yaml; do
	if [[ -f "deploy/kubernetes/$manifest_file" ]]; then
		sed -e "s|image: e-ubl21:replace-me|image: $IMAGE|g" \
		    -e "s|namespace: default|namespace: $NAMESPACE|g" \
		    "deploy/kubernetes/$manifest_file" | \
			"$KUBECTL_BIN" apply -f - >"$ARTIFACT_DIR/apply-$(basename $manifest_file).log" 2>&1
	fi
done

# Wait for all deployments to be ready (fail immediately if any fails)
echo "Waiting for all deployments to reach ready state"
for deployment in e-ubl21-api e-ubl21-orchestrator e-ubl21-worker-xsd e-ubl21-worker-en16931; do
	echo "  Checking $deployment..."
	"$KUBECTL_BIN" --namespace "$NAMESPACE" rollout status "deployment/$deployment" --timeout="$ROLLOUT_TIMEOUT"
done

# Port forward (matches regression-minikube.sh)
echo "Setting up port forwarding"
"$KUBECTL_BIN" --namespace "$NAMESPACE" port-forward service/e-ubl21-api "$API_LOCAL_PORT:8080" >"$ARTIFACT_DIR/port-forward/api.log" 2>&1 &
PORT_FORWARD_PIDS+=("$!")
"$KUBECTL_BIN" --namespace "$NAMESPACE" port-forward service/nats "$NATS_LOCAL_PORT:4222" >"$ARTIFACT_DIR/port-forward/nats.log" 2>&1 &
PORT_FORWARD_PIDS+=("$!")

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

echo "Waiting for services"
wait_http api "http://127.0.0.1:$API_LOCAL_PORT/healthz" "$TOKEN"
STARTED_AT="$(/usr/bin/date -u +%Y-%m-%dT%H:%M:%SZ)"

echo ""
echo "=== Running Issue #65 Scenarios ==="

# Scenario 1: Valid document through split deployment
echo ""
echo "Scenario 1: Valid EN16931 document through XSD + EN pipeline"
go run . issue-65 \
	--base-url "http://127.0.0.1:$API_LOCAL_PORT" \
	--token "$TOKEN" \
	--nats-url "nats://127.0.0.1:$NATS_LOCAL_PORT" \
	--queue-namespace "$QUEUE_NAMESPACE" \
	--kubectl-bin "$KUBECTL_BIN" \
	--namespace "$NAMESPACE" \
	--manifest "$MANIFEST" \
	--output "$ARTIFACT_DIR" 2>&1 | tee "$ARTIFACT_DIR/i65-runner.log"
RUNNER_STATUS=${PIPESTATUS[0]}

if (( RUNNER_STATUS == 0 )); then
	echo ""
	echo "✓ Issue #65 acceptance gate: PASS"
	exit 0
else
	echo ""
	echo "✗ Issue #65 acceptance gate: FAIL (status $RUNNER_STATUS)"
	exit "$RUNNER_STATUS"
fi
