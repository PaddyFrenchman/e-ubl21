#!/usr/bin/env bash
# Issue #65 split-deployment end-to-end validation gate

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

for cmd in minikube docker go curl jq kubectl; do
	if ! command -v "$cmd" >/dev/null 2>&1; then
		echo "ERROR: required: $cmd" >&2
		exit 2
	fi
done

MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-e-ubl21-i65}"
if [[ -z "${MINIKUBE_DRIVER:-}" ]]; then
	DOCKER_VERSION="$(docker --version 2>&1)"
	if [[ "${DOCKER_VERSION,,}" == *podman* ]]; then
		MINIKUBE_DRIVER=podman
		export MINIKUBE_ROOTLESS=true
	else
		MINIKUBE_DRIVER=docker
	fi
fi

NAMESPACE="e-ubl21"  # Kubernetes manifests hardcode this namespace
QUEUE_NAMESPACE="${QUEUE_NAMESPACE:-eubl21.i65}"
IMAGE="${IMAGE:-localhost/e-ubl21:i65-split}"
MANIFEST="${MANIFEST:-createsamples/samples-en16931/manifest.json}"
API_LOCAL_PORT="${API_LOCAL_PORT:-18081}"
NATS_LOCAL_PORT="${NATS_LOCAL_PORT:-14223}"
ROLLOUT_TIMEOUT="${ROLLOUT_TIMEOUT:-5m}"
ARTIFACT_DIR="${ARTIFACT_DIR:-regression-results/issue-65/$(/usr/bin/date -u +%Y%m%dT%H%M%SZ)}"

[[ -f "$MANIFEST" ]] || { echo "ERROR: manifest not found: $MANIFEST" >&2; exit 2; }

mkdir -p "$ARTIFACT_DIR"
ARTIFACT_DIR="$(cd "$ARTIFACT_DIR" && pwd)"
MANIFEST="$(cd "$(dirname "$MANIFEST")" && pwd)/$(basename "$MANIFEST")"

PORT_FORWARD_PIDS=()
cleanup() {
	local s=$?
	trap - EXIT
	set +e
	for pid in "${PORT_FORWARD_PIDS[@]}"; do kill "$pid" 2>/dev/null || true; done
	exit "$s"
}
trap cleanup EXIT

echo "=== Issue #65 Split-Deployment Gate ==="
echo "Artifact: $ARTIFACT_DIR"

if ! minikube status -p "$MINIKUBE_PROFILE" --format='{{.Host}}' 2>/dev/null | grep -qx Running; then
	echo "Starting Minikube"
	minikube start -p "$MINIKUBE_PROFILE" --driver="$MINIKUBE_DRIVER"
fi

echo "Building image"
docker build --tag "$IMAGE" . >"$ARTIFACT_DIR/build.log" 2>&1
docker save "$IMAGE" | minikube -p "$MINIKUBE_PROFILE" image load -

echo "Generating token"
HMAC_SECRET="$(go run ./tools/regression-token-secret 2>/dev/null)"
export EUBL21_REGRESSION_HMAC_SECRET="$HMAC_SECRET"
TOKEN="$(go run ./tools/regression-token --issuer https://regression.e-ubl21.local --audience e-ubl21-regression --ttl 2h 2>/dev/null)"
unset EUBL21_REGRESSION_HMAC_SECRET

echo "Deploying split architecture"
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

# Auth secret - named what the manifests expect
kubectl -n "$NAMESPACE" create secret generic e-ubl21-auth \
	--from-literal="hmac-secret=$HMAC_SECRET" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
unset HMAC_SECRET

# NATS deployment (not in kubernetes/ manifests)
kubectl apply -f - >"$ARTIFACT_DIR/nats.log" 2>&1 <<EOF
apiVersion: v1
kind: Service
metadata:
  name: nats
  namespace: $NAMESPACE
spec:
  selector:
    app.kubernetes.io/name: nats
  ports:
    - port: 4222
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nats
  namespace: $NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: nats
  template:
    metadata:
      labels:
        app.kubernetes.io/name: nats
    spec:
      containers:
        - name: nats
          image: nats:2.10-alpine
          args: [-js, -sd, /data]
          ports:
            - containerPort: 4222
          volumeMounts:
            - name: data
              mountPath: /data
      volumes:
        - name: data
          emptyDir: {}
EOF

# Deploy split components via kustomize
kubectl kustomize deploy/kubernetes | \
	sed "s|image: e-ubl21:replace-me|image: $IMAGE|g" | \
	sed "s|EUBL21_QUEUE_NAMESPACE: .*|EUBL21_QUEUE_NAMESPACE: $QUEUE_NAMESPACE|" | \
	kubectl apply -f - >"$ARTIFACT_DIR/split.log" 2>&1

echo "Waiting for readiness"
for dep in e-ubl21-api e-ubl21-orchestrator e-ubl21-worker-xsd e-ubl21-worker-en16931 nats; do
	if ! kubectl -n "$NAMESPACE" rollout status "deployment/$dep" --timeout="$ROLLOUT_TIMEOUT" 2>/dev/null; then
		echo "ERROR: $dep failed to become ready"
		exit 1
	fi
done

echo "Port forwarding"
kubectl -n "$NAMESPACE" port-forward service/e-ubl21-api "$API_LOCAL_PORT:8080" >"$ARTIFACT_DIR/pf-api.log" 2>&1 &
PORT_FORWARD_PIDS+=("$!")
kubectl -n "$NAMESPACE" port-forward service/nats "$NATS_LOCAL_PORT:4222" >"$ARTIFACT_DIR/pf-nats.log" 2>&1 &
PORT_FORWARD_PIDS+=("$!")

sleep 2

echo "Waiting for API"
for i in {1..60}; do
	curl -f -s -H "Authorization: Bearer $TOKEN" "http://127.0.0.1:$API_LOCAL_PORT/healthz" >/dev/null 2>&1 && break
	[[ $i -lt 60 ]] || { echo "ERROR: API timeout"; exit 1; }
	sleep 1
done

echo ""
echo "=== Running Issue #65 Scenarios ==="
go run . issue-65 \
	--base-url "http://127.0.0.1:$API_LOCAL_PORT" \
	--token "$TOKEN" \
	--manifest "$MANIFEST" \
	--nats-url "nats://127.0.0.1:$NATS_LOCAL_PORT" \
	--queue-namespace "$QUEUE_NAMESPACE" \
	--kubectl-bin kubectl \
	--namespace "$NAMESPACE" \
	--output "$ARTIFACT_DIR"
