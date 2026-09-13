#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/devsetup/lib/docker-engine.sh"
cd "$ROOT_DIR"

TARGET_MINIKUBE_VERSION="${TARGET_MINIKUBE_VERSION:-v1.39.0}"
TARGET_MINIKUBE_SHA256="${TARGET_MINIKUBE_SHA256:-b738496da01be06bbaf80c688f57ce25acd3849fbb518155f3a88e03ef555aa4}"
TARGET_KUBERNETES_VERSION="${TARGET_KUBERNETES_VERSION:-v1.35.1}"
MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-e-ubl21-regression}"
MINIKUBE_DRIVER="${MINIKUBE_DRIVER:-docker}"
MINIKUBE_CONTAINER_RUNTIME="${MINIKUBE_CONTAINER_RUNTIME:-docker}"
MINIKUBE_CPUS="${MINIKUBE_CPUS:-2}"
MINIKUBE_MEMORY="${MINIKUBE_MEMORY:-4096mb}"
DOWNLOAD_URL="https://github.com/kubernetes/minikube/releases/download/${TARGET_MINIKUBE_VERSION}/minikube-linux-amd64"
RUN_STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/e-ubl21/minikube-upgrade-${RUN_STAMP}"
TMP_DIR="$(mktemp -d)"

cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

fail() {
    echo "ERROR: $*" >&2
    echo "Upgrade evidence and rollback material, if created: $STATE_DIR" >&2
    exit 1
}

for command in curl sha256sum sudo git docker go minikube; do
    command -v "$command" >/dev/null 2>&1 || fail "required command not found: $command"
done

if (( EUID == 0 )); then
    fail "run this script as the normal development user, not as root"
fi

case "$(uname -m)" in
    x86_64|amd64) ;;
    *) fail "this qualification script currently pins the Linux amd64 Minikube binary; found architecture $(uname -m)" ;;
esac

if [[ -n "$(git status --porcelain --untracked-files=normal)" ]]; then
    fail "working tree must be clean so qualification evidence maps to one exact candidate"
fi

mkdir -p "$STATE_DIR"
chmod 0700 "$STATE_DIR"

BRANCH="$(git branch --show-current)"
COMMIT="$(git rev-parse HEAD)"
OLD_MINIKUBE_BIN="$(readlink -f "$(command -v minikube)")"
OLD_MINIKUBE_VERSION="$(minikube version --short 2>/dev/null || true)"
OLD_MINIKUBE_SHA256="$(sha256sum "$OLD_MINIKUBE_BIN" | awk '{print $1}')"

cat >"$STATE_DIR/before.txt" <<EOF
branch=$BRANCH
commit=$COMMIT
old_minikube_bin=$OLD_MINIKUBE_BIN
old_minikube_version=$OLD_MINIKUBE_VERSION
old_minikube_sha256=$OLD_MINIKUBE_SHA256
target_minikube_version=$TARGET_MINIKUBE_VERSION
target_minikube_sha256=$TARGET_MINIKUBE_SHA256
target_kubernetes_version=$TARGET_KUBERNETES_VERSION
minikube_profile=$MINIKUBE_PROFILE
minikube_driver=$MINIKUBE_DRIVER
minikube_container_runtime=$MINIKUBE_CONTAINER_RUNTIME
EOF

cp "$OLD_MINIKUBE_BIN" "$STATE_DIR/minikube-before"
chmod 0700 "$STATE_DIR/minikube-before"

cat >"$STATE_DIR/rollback.sh" <<EOF
#!/usr/bin/env bash
set -Eeuo pipefail
sudo install -m 0755 "$STATE_DIR/minikube-before" /usr/local/bin/minikube
hash -r
minikube version
printf 'Restored pre-upgrade Minikube binary from %s\n' "$STATE_DIR/minikube-before"
EOF
chmod 0700 "$STATE_DIR/rollback.sh"

printf 'Qualifying Minikube upgrade on branch %s at commit %s\n' "$BRANCH" "$COMMIT"
printf 'Current Minikube: %s (%s)\n' "${OLD_MINIKUBE_VERSION:-unknown}" "$OLD_MINIKUBE_BIN"
printf 'Target: %s, Kubernetes remains %s, driver/runtime remain %s/%s\n' \
    "$TARGET_MINIKUBE_VERSION" "$TARGET_KUBERNETES_VERSION" "$MINIKUBE_DRIVER" "$MINIKUBE_CONTAINER_RUNTIME"
printf 'Rollback material: %s\n' "$STATE_DIR"

require_docker_compose
"$ROOT_DIR/devsetup/verify-docker-engine.sh"

curl --fail --location --silent --show-error --retry 3 \
    --output "$TMP_DIR/minikube" "$DOWNLOAD_URL"
echo "$TARGET_MINIKUBE_SHA256  $TMP_DIR/minikube" | sha256sum --check
chmod 0755 "$TMP_DIR/minikube"

DOWNLOADED_VERSION="$("$TMP_DIR/minikube" version --short)"
[[ "$DOWNLOADED_VERSION" == "$TARGET_MINIKUBE_VERSION" ]] || \
    fail "downloaded Minikube reports $DOWNLOADED_VERSION, expected $TARGET_MINIKUBE_VERSION"

sudo -v
sudo install -m 0755 "$TMP_DIR/minikube" /usr/local/bin/minikube
hash -r

ACTIVE_MINIKUBE_BIN="$(readlink -f "$(command -v minikube)")"
ACTIVE_MINIKUBE_VERSION="$(minikube version --short)"
ACTIVE_MINIKUBE_SHA256="$(sha256sum "$ACTIVE_MINIKUBE_BIN" | awk '{print $1}')"

[[ "$ACTIVE_MINIKUBE_VERSION" == "$TARGET_MINIKUBE_VERSION" ]] || \
    fail "active Minikube reports $ACTIVE_MINIKUBE_VERSION after installation"
[[ "$ACTIVE_MINIKUBE_SHA256" == "$TARGET_MINIKUBE_SHA256" ]] || \
    fail "active Minikube checksum is $ACTIVE_MINIKUBE_SHA256, expected $TARGET_MINIKUBE_SHA256"

unset MINIKUBE_ROOTLESS || true
minikube config unset rootless >/dev/null 2>&1 || true

# The regression profile is disposable. Recreate it so the new Minikube binary
# is qualified against a fresh Docker-backed cluster while holding Kubernetes
# and the container runtime constant.
minikube delete --profile "$MINIKUBE_PROFILE" >/dev/null 2>&1 || true
minikube start \
    --profile "$MINIKUBE_PROFILE" \
    --driver "$MINIKUBE_DRIVER" \
    --container-runtime "$MINIKUBE_CONTAINER_RUNTIME" \
    --kubernetes-version "$TARGET_KUBERNETES_VERSION" \
    --cpus "$MINIKUBE_CPUS" \
    --memory "$MINIKUBE_MEMORY"

STATUS="$(minikube status --profile "$MINIKUBE_PROFILE" --format='host={{.Host}} kubelet={{.Kubelet}} apiserver={{.APIServer}} kubeconfig={{.Kubeconfig}}')"
echo "$STATUS" | tee "$STATE_DIR/minikube-status.txt"
for expected in 'host=Running' 'kubelet=Running' 'apiserver=Running' 'kubeconfig=Configured'; do
    grep -Fq "$expected" <<<"$STATUS" || fail "Minikube status missing expected state: $expected"
done

NODE_VERSION="$(minikube -p "$MINIKUBE_PROFILE" kubectl -- get nodes -o jsonpath='{.items[0].status.nodeInfo.kubeletVersion}')"
CONTAINER_RUNTIME="$(minikube -p "$MINIKUBE_PROFILE" kubectl -- get nodes -o jsonpath='{.items[0].status.nodeInfo.containerRuntimeVersion}')"

[[ "$NODE_VERSION" == "$TARGET_KUBERNETES_VERSION" ]] || \
    fail "cluster kubelet is $NODE_VERSION, expected $TARGET_KUBERNETES_VERSION"
[[ "$CONTAINER_RUNTIME" == docker://* ]] || \
    fail "cluster container runtime is $CONTAINER_RUNTIME, expected Docker"

PROFILE_CONFIG="$HOME/.minikube/profiles/$MINIKUBE_PROFILE/config.json"
if [[ -f "$PROFILE_CONFIG" ]]; then
    grep -Eq '"Driver"[[:space:]]*:[[:space:]]*"docker"' "$PROFILE_CONFIG" || \
        fail "profile config does not record the Docker driver"
fi

{
    echo "active_minikube_bin=$ACTIVE_MINIKUBE_BIN"
    echo "active_minikube_version=$ACTIVE_MINIKUBE_VERSION"
    echo "active_minikube_sha256=$ACTIVE_MINIKUBE_SHA256"
    echo "kubernetes_version=$NODE_VERSION"
    echo "container_runtime=$CONTAINER_RUNTIME"
    echo "docker_version=$(docker --version)"
    echo "docker_compose_version=$(docker compose version)"
} | tee "$STATE_DIR/qualified-runtime.txt"

minikube profile list 2>&1 | tee "$STATE_DIR/minikube-profile-list.txt" || true
minikube -p "$MINIKUBE_PROFILE" kubectl -- version | tee "$STATE_DIR/kubectl-version.txt"

printf '\nRunning Go test suite on exact candidate %s...\n' "$COMMIT"
go test ./... -count=1 2>&1 | tee "$STATE_DIR/go-test.log"

printf '\nRunning complete local regression suite under Minikube %s...\n' "$TARGET_MINIKUBE_VERSION"
set +e
./run-regression-tests.sh 2>&1 | tee "$STATE_DIR/regression.log"
REGRESSION_STATUS=${PIPESTATUS[0]}
set -e
if (( REGRESSION_STATUS != 0 )); then
    fail "regression suite failed with status $REGRESSION_STATUS; inspect $STATE_DIR/regression.log"
fi

cat >"$STATE_DIR/result.txt" <<EOF
result=PASS
branch=$BRANCH
commit=$COMMIT
minikube_version=$ACTIVE_MINIKUBE_VERSION
minikube_sha256=$ACTIVE_MINIKUBE_SHA256
kubernetes_version=$NODE_VERSION
container_runtime=$CONTAINER_RUNTIME
profile=$MINIKUBE_PROFILE
regression_status=$REGRESSION_STATUS
EOF

printf '\nMinikube %s qualification PASSED.\n' "$TARGET_MINIKUBE_VERSION"
printf 'Kubernetes remained pinned at %s and container runtime remained %s.\n' "$NODE_VERSION" "$CONTAINER_RUNTIME"
printf 'Evidence: %s\n' "$STATE_DIR"
printf 'Rollback if needed: %s\n' "$STATE_DIR/rollback.sh"
printf '\nNext repository step after review of this evidence: update the hosted workflow pin to %s and SHA-256 %s.\n' \
    "$TARGET_MINIKUBE_VERSION" "$TARGET_MINIKUBE_SHA256"
