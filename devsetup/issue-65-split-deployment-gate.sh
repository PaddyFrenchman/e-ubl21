#!/usr/bin/env bash
# Issue #65 split-deployment end-to-end validation gate.
#
# Deploys the split topology (API, orchestrator, reconciler, XSD worker,
# EN 16931 worker) from deploy/kubernetes into a disposable Minikube cluster,
# then runs `go run . issue-65`, which proves the acceptance scenarios:
# valid plan, XSD-invalid without EN requests, EN-invalid, EN scale-to-zero
# with durable recovery, and the frozen semantic-equivalence corpus.
#
# Autoscaling manifests (HPA/KEDA) are deliberately NOT applied: the gate
# must own the EN replica count during the scale-to-zero scenario, and the
# KEDA CRDs are not installed in the disposable cluster.

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/devsetup/lib/docker-engine.sh"
cd "$ROOT_DIR"

for cmd in minikube go curl; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "ERROR: required command not found: $cmd" >&2
        exit 2
    fi
done

MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-e-ubl21-i65}"
MINIKUBE_DRIVER="${MINIKUBE_DRIVER:-docker}"
require_docker_engine
if [[ "$MINIKUBE_DRIVER" == "podman" ]]; then
    export MINIKUBE_ROOTLESS="${MINIKUBE_ROOTLESS:-true}"
fi

NAMESPACE="e-ubl21"  # deploy/kubernetes/namespace.yaml creates exactly this namespace
QUEUE_NAMESPACE="${QUEUE_NAMESPACE:-eubl21.i65}"
IMAGE="${IMAGE:-localhost/e-ubl21:i65-gate}"
MANIFEST="${MANIFEST:-createsamples/samples-en16931/manifest.json}"
# The EN 16931 manifest has no XSD-invalid fixture; scenario 2 borrows one
# from the UBL 2.1 corpus and submits it under the ubl21-en16931 profile.
XSD_INVALID_MANIFEST="${XSD_INVALID_MANIFEST:-createsamples/samples/manifest.json}"
API_LOCAL_PORT="${API_LOCAL_PORT:-18081}"
NATS_LOCAL_PORT="${NATS_LOCAL_PORT:-14223}"
GATE_RATE_LIMIT_RPS="${GATE_RATE_LIMIT_RPS:-1000}"
GATE_RATE_LIMIT_BURST="${GATE_RATE_LIMIT_BURST:-2000}"
ROLLOUT_TIMEOUT="${ROLLOUT_TIMEOUT:-5m}"
ARTIFACT_DIR="${ARTIFACT_DIR:-regression-results/issue-65/$(date -u +%Y%m%dT%H%M%SZ)}"
KUBECTL_BIN="${KUBECTL_BIN:-$ROOT_DIR/devsetup/minikube-kubectl.sh}"
KEEP_DEPLOYMENT="${KEEP_DEPLOYMENT:-0}"
OAUTH_ISSUER="https://regression.e-ubl21.local"
OAUTH_AUDIENCE="e-ubl21-regression"
EQUIVALENCE_BASELINE="${EQUIVALENCE_BASELINE:-}"

if [[ ! -f "$MANIFEST" ]]; then
    echo "ERROR: manifest not found: $MANIFEST" >&2
    exit 2
fi
if [[ -z "$EQUIVALENCE_BASELINE" ]]; then
    case "$(cd "$(dirname "$MANIFEST")" && pwd)/$(basename "$MANIFEST")" in
        "$ROOT_DIR/createsamples/samples/manifest.json")
            EQUIVALENCE_BASELINE="$ROOT_DIR/regression/baselines/validation-split/ubl21.results.jsonl"
            ;;
        "$ROOT_DIR/createsamples/samples-en16931/manifest.json")
            EQUIVALENCE_BASELINE="$ROOT_DIR/regression/baselines/validation-split/en16931.results.jsonl"
            ;;
    esac
fi
if [[ -z "$EQUIVALENCE_BASELINE" || ! -f "$EQUIVALENCE_BASELINE" ]]; then
    echo "ERROR: frozen equivalence baseline not found for $MANIFEST (set EQUIVALENCE_BASELINE)" >&2
    exit 2
fi

mkdir -p "$ARTIFACT_DIR/port-forward"
ARTIFACT_DIR="$(cd "$ARTIFACT_DIR" && pwd)"
MANIFEST="$(cd "$(dirname "$MANIFEST")" && pwd)/$(basename "$MANIFEST")"
if [[ -n "$XSD_INVALID_MANIFEST" ]]; then
    XSD_INVALID_MANIFEST="$(cd "$(dirname "$XSD_INVALID_MANIFEST")" && pwd)/$(basename "$XSD_INVALID_MANIFEST")"
fi

export EUBL21_MINIKUBE_PROFILE="$MINIKUBE_PROFILE"

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
        echo "Collecting Kubernetes state for troubleshooting"
        "$KUBECTL_BIN" get all -n "$NAMESPACE" >"$ARTIFACT_DIR/k8s-all.log" 2>&1 || true
        "$KUBECTL_BIN" describe pods -n "$NAMESPACE" >"$ARTIFACT_DIR/k8s-pods.log" 2>&1 || true
        "$KUBECTL_BIN" logs -n "$NAMESPACE" --selector app.kubernetes.io/name=e-ubl21 --all-containers --tail=200 --prefix >"$ARTIFACT_DIR/k8s-logs.log" 2>&1 || true
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

echo "=== Issue #65 Split-Deployment Gate ==="
echo "Artifacts: $ARTIFACT_DIR"

if ! minikube status -p "$MINIKUBE_PROFILE" --format='{{.Host}}' 2>/dev/null | grep -qx Running; then
    minikube start -p "$MINIKUBE_PROFILE" --driver="$MINIKUBE_DRIVER"
fi
MINIKUBE_NODE_IP="$(minikube ip -p "$MINIKUBE_PROFILE")"
if [[ -z "$MINIKUBE_NODE_IP" ]]; then
    echo "ERROR: could not determine the Minikube node IP" >&2
    exit 2
fi

echo "Building candidate image $IMAGE"
docker build --tag "$IMAGE" . >"$ARTIFACT_DIR/docker-build.log" 2>&1
docker save "$IMAGE" | minikube -p "$MINIKUBE_PROFILE" image load -

echo "Generating auth secret and token"
HMAC_SECRET="$(go run ./tools/regression-token-secret)"
export EUBL21_REGRESSION_HMAC_SECRET="$HMAC_SECRET"
TOKEN="$(go run ./tools/regression-token --issuer "$OAUTH_ISSUER" --audience "$OAUTH_AUDIENCE" --ttl 2h)"
unset EUBL21_REGRESSION_HMAC_SECRET

echo "Creating namespace, auth secret, and NATS"
"$KUBECTL_BIN" apply -f deploy/kubernetes/namespace.yaml >"$ARTIFACT_DIR/apply.log" 2>&1
DEPLOYMENT_STARTED=1
# The API deployment resolves all three keys from this secret.
"$KUBECTL_BIN" -n "$NAMESPACE" create secret generic e-ubl21-auth \
    --from-literal="issuer=$OAUTH_ISSUER" \
    --from-literal="audience=$OAUTH_AUDIENCE" \
    --from-literal="hmac-secret=$HMAC_SECRET" \
    --dry-run=client -o yaml | "$KUBECTL_BIN" apply -f - >>"$ARTIFACT_DIR/apply.log" 2>&1
unset HMAC_SECRET

# JetStream-enabled pinned NATS; deploy/kubernetes has no broker manifest.
# A headless Service plus host-networked NATS is intentional here. Rootless
# Podman Minikube does not reliably forward ClusterIP traffic, including the
# DNS Service. The split workloads stay on pod networking and receive the
# reachable node address below; production manifests retain normal Services.
"$KUBECTL_BIN" -n "$NAMESPACE" apply -f - >>"$ARTIFACT_DIR/apply.log" 2>&1 <<'NATS'
apiVersion: v1
kind: ConfigMap
metadata:
  name: nats-config
data:
  nats.conf: |
    jetstream {
      store_dir: /data
    }
    http: 8222
    max_payload: 2097152
---
apiVersion: v1
kind: Service
metadata:
  name: nats
spec:
  clusterIP: None
  selector:
    app.kubernetes.io/name: nats
  ports:
    - name: client
      port: 4222
      targetPort: 4222
    - name: monitor
      port: 8222
      targetPort: 8222
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nats
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
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      containers:
        - name: nats
          image: nats:2.10-alpine
          args: ["-c", "/etc/nats/nats.conf"]
          ports:
            - name: client
              containerPort: 4222
            - name: monitor
              containerPort: 8222
          readinessProbe:
            httpGet:
              path: /healthz
              port: monitor
            periodSeconds: 2
            failureThreshold: 30
          volumeMounts:
            - name: data
              mountPath: /data
            - name: config
              mountPath: /etc/nats
              readOnly: true
      volumes:
        - name: data
          emptyDir: {}
        - name: config
          configMap:
            name: nats-config
NATS

echo "Applying runtime configuration and provisioning job"
# runtime-config.yaml pins production values; override the disposable gate's
# queue namespace and bypass rootless Podman's broken DNS/ClusterIP path.
sed \
    -e "s|EUBL21_QUEUE_NAMESPACE: .*|EUBL21_QUEUE_NAMESPACE: $QUEUE_NAMESPACE|" \
    -e "s|EUBL21_NATS_URL: .*|EUBL21_NATS_URL: nats://$MINIKUBE_NODE_IP:4222|" \
    deploy/kubernetes/runtime-config.yaml | \
    "$KUBECTL_BIN" -n "$NAMESPACE" apply -f - >>"$ARTIFACT_DIR/apply.log" 2>&1
sed "s|image: e-ubl21:replace-me|image: $IMAGE|g" deploy/kubernetes/provision-job.yaml | \
    "$KUBECTL_BIN" -n "$NAMESPACE" apply -f - >>"$ARTIFACT_DIR/apply.log" 2>&1

echo "Waiting for stream provisioning job"
"$KUBECTL_BIN" -n "$NAMESPACE" rollout status deployment/nats --timeout="$ROLLOUT_TIMEOUT"
"$KUBECTL_BIN" -n "$NAMESPACE" wait --for=condition=complete job/e-ubl21-provision --timeout="$ROLLOUT_TIMEOUT"

echo "Activating split deployment barrier"
"$KUBECTL_BIN" -n "$NAMESPACE" run e-ubl21-migration-begin \
    --image="$IMAGE" \
    --restart=Never \
    --env="EUBL21_NATS_URL=nats://$MINIKUBE_NODE_IP:4222" \
    --env="EUBL21_QUEUE_NAMESPACE=$QUEUE_NAMESPACE" \
    -- migration begin-drain >>"$ARTIFACT_DIR/apply.log" 2>&1
"$KUBECTL_BIN" -n "$NAMESPACE" wait \
    --for=jsonpath='{.status.phase}'=Succeeded pod/e-ubl21-migration-begin \
    --timeout="$ROLLOUT_TIMEOUT"
"$KUBECTL_BIN" -n "$NAMESPACE" logs pod/e-ubl21-migration-begin >"$ARTIFACT_DIR/migration-begin.log"

"$KUBECTL_BIN" -n "$NAMESPACE" run e-ubl21-migration-activate \
    --image="$IMAGE" \
    --restart=Never \
    --env="EUBL21_NATS_URL=nats://$MINIKUBE_NODE_IP:4222" \
    --env="EUBL21_QUEUE_NAMESPACE=$QUEUE_NAMESPACE" \
    -- migration activate --allow-no-legacy-consumer >>"$ARTIFACT_DIR/apply.log" 2>&1
"$KUBECTL_BIN" -n "$NAMESPACE" wait \
    --for=jsonpath='{.status.phase}'=Succeeded pod/e-ubl21-migration-activate \
    --timeout="$ROLLOUT_TIMEOUT"
"$KUBECTL_BIN" -n "$NAMESPACE" logs pod/e-ubl21-migration-activate >"$ARTIFACT_DIR/migration-activate.log"

echo "Applying split component manifests (autoscaling deliberately excluded)"
for component in api orchestrator orchestrator-reconciler worker-xsd worker-en16931; do
    sed "s|image: e-ubl21:replace-me|image: $IMAGE|g" "deploy/kubernetes/$component.yaml" | \
        "$KUBECTL_BIN" -n "$NAMESPACE" apply -f - >>"$ARTIFACT_DIR/apply.log" 2>&1
done
# Regression polling and the parallel equivalence corpus intentionally exceed
# production admission defaults. Raise only the disposable gate deployment.
"$KUBECTL_BIN" -n "$NAMESPACE" set env deployment/e-ubl21-api \
    "EUBL21_RATE_LIMIT_RPS=$GATE_RATE_LIMIT_RPS" \
    "EUBL21_RATE_LIMIT_BURST=$GATE_RATE_LIMIT_BURST" \
    >>"$ARTIFACT_DIR/apply.log" 2>&1

echo "Waiting for split deployments"
for deployment in e-ubl21-api e-ubl21-orchestrator e-ubl21-orchestrator-reconciler e-ubl21-worker-xsd e-ubl21-worker-en16931; do
    "$KUBECTL_BIN" -n "$NAMESPACE" rollout status "deployment/$deployment" --timeout="$ROLLOUT_TIMEOUT"
done

echo "Port-forwarding API and NATS"
"$KUBECTL_BIN" -n "$NAMESPACE" port-forward service/e-ubl21-api "$API_LOCAL_PORT:8080" >"$ARTIFACT_DIR/port-forward/api.log" 2>&1 &
PORT_FORWARD_PIDS+=("$!")
"$KUBECTL_BIN" -n "$NAMESPACE" port-forward deployment/nats "$NATS_LOCAL_PORT:4222" >"$ARTIFACT_DIR/port-forward/nats.log" 2>&1 &
PORT_FORWARD_PIDS+=("$!")

wait_http() {
    local name=$1 url=$2 auth=${3:-}
    local deadline=$((SECONDS + 60))
    while (( SECONDS < deadline )); do
        if [[ -n "$auth" ]]; then
            curl --fail --silent --max-time 2 -H "Authorization: Bearer $auth" "$url" >/dev/null 2>&1 && return 0
        else
            curl --fail --silent --max-time 2 "$url" >/dev/null 2>&1 && return 0
        fi
        sleep 1
    done
    echo "ERROR: timed out waiting for $name at $url" >&2
    return 1
}
wait_http api "http://127.0.0.1:$API_LOCAL_PORT/healthz" "$TOKEN"

echo ""
echo "=== Running Issue #65 acceptance scenarios ==="
go run . issue-65 \
    --base-url "http://127.0.0.1:$API_LOCAL_PORT" \
    --token "$TOKEN" \
    --manifest "$MANIFEST" \
    --xsd-invalid-manifest "$XSD_INVALID_MANIFEST" \
    --nats-url "nats://127.0.0.1:$NATS_LOCAL_PORT" \
    --queue-namespace "$QUEUE_NAMESPACE" \
    --kubectl-bin "$KUBECTL_BIN" \
    --namespace "$NAMESPACE" \
    --en-deployment e-ubl21-worker-en16931 \
    --equivalence-baseline "$EQUIVALENCE_BASELINE" \
    --output "$ARTIFACT_DIR" 2>&1 | tee "$ARTIFACT_DIR/gate.log"
exit "${PIPESTATUS[0]}"
