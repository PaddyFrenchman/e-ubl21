#!/usr/bin/env bash
set -Eeuo pipefail

# e-ubl21 single-server on-premises bootstrap.
#
# Supported baseline:
#   - Ubuntu/Debian-family Linux
#   - RHEL-family Linux
#   - K3s
#   - Helm
#   - MetalLB
#   - cert-manager
#   - KEDA
#   - CloudNativePG + one PostgreSQL instance
#   - NATS JetStream
#   - single-node MinIO-compatible object storage
#   - kube-prometheus-stack (Prometheus + Grafana)
#   - OpenTelemetry Collector
#
# Review this script before running it as root.

EUBL21_NAMESPACE="${EUBL21_NAMESPACE:-e-ubl21}"
METALLB_ADDRESS_POOL="${METALLB_ADDRESS_POOL:-}"
NATS_STORAGE_SIZE="${NATS_STORAGE_SIZE:-20Gi}"
POSTGRES_STORAGE_SIZE="${POSTGRES_STORAGE_SIZE:-50Gi}"
MINIO_STORAGE_SIZE="${MINIO_STORAGE_SIZE:-100Gi}"
PROMETHEUS_STORAGE_SIZE="${PROMETHEUS_STORAGE_SIZE:-30Gi}"
GRAFANA_STORAGE_SIZE="${GRAFANA_STORAGE_SIZE:-10Gi}"
MINIO_IMAGE="${MINIO_IMAGE:-quay.io/minio/minio:latest}"
INSTALL_PLATFORM="${INSTALL_PLATFORM:-1}"
DISABLE_SWAP="${DISABLE_SWAP:-1}"
K3S_VERSION="${K3S_VERSION:-}"
STATE_DIR="${STATE_DIR:-/var/lib/e-ubl21-onprem}"

log() {
  printf '\n[e-ubl21] %s\n' "$*"
}

warn() {
  printf '\n[e-ubl21] WARNING: %s\n' "$*" >&2
}

fail() {
  printf '\n[e-ubl21] ERROR: %s\n' "$*" >&2
  exit 1
}

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    fail "Run this installer as root, normally with: sudo -E bash $0"
  fi
}

load_os() {
  [[ -r /etc/os-release ]] || fail "/etc/os-release is missing; unsupported Linux distribution"
  # shellcheck disable=SC1091
  . /etc/os-release

  OS_ID="${ID:-unknown}"
  OS_LIKE="${ID_LIKE:-}"
  OS_VERSION_ID="${VERSION_ID:-}"

  case "${OS_ID}:${OS_LIKE}" in
    ubuntu:*|debian:*|*:debian*)
      PKG_FAMILY="apt"
      ;;
    rhel:*|centos:*|rocky:*|almalinux:*|fedora:*|*:rhel*|*:fedora*)
      PKG_FAMILY="dnf"
      ;;
    *)
      fail "Unsupported distribution: ID=${OS_ID}, ID_LIKE=${OS_LIKE}"
      ;;
  esac

  log "Detected ${PRETTY_NAME:-${OS_ID}}"
}

install_base_packages() {
  log "Installing base operating-system packages"

  if [[ "${PKG_FAMILY}" == "apt" ]]; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y \
      ca-certificates \
      curl \
      git \
      gzip \
      jq \
      openssl \
      tar
  else
    dnf install -y \
      ca-certificates \
      curl \
      git \
      gzip \
      jq \
      openssl \
      tar

    if [[ "${OS_ID}" == "rhel" && "${OS_VERSION_ID%%.*}" == "10" ]]; then
      dnf install -y kernel-modules-extra
    fi
  fi
}

warn_about_active_firewall() {
  if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet firewalld 2>/dev/null; then
    warn "firewalld is active. Before relying on pod networking, configure the K3s-required rules documented in deploy/on-prem/deployment-on-prem.md."
  fi

  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q '^Status: active'; then
    warn "UFW is active. Before relying on pod networking, configure the K3s-required rules documented in deploy/on-prem/deployment-on-prem.md."
  fi
}

disable_swap_if_requested() {
  if [[ "${DISABLE_SWAP}" != "1" ]]; then
    log "DISABLE_SWAP=${DISABLE_SWAP}; leaving swap configuration unchanged"
    return
  fi

  log "Disabling swap"
  swapoff -a || true

  if [[ -f /etc/fstab ]]; then
    if [[ ! -f /etc/fstab.eubl21-before-swap-disable ]]; then
      cp -a /etc/fstab /etc/fstab.eubl21-before-swap-disable
    fi

    sed -ri '/^[[:space:]]*#/! { /[[:space:]]swap[[:space:]]/ s/^/# e-ubl21 disabled swap: /; }' /etc/fstab
  fi
}

prepare_k3s_config() {
  install -d -m 0755 /etc/rancher/k3s

  if [[ -f /etc/rancher/k3s/config.yaml ]]; then
    warn "/etc/rancher/k3s/config.yaml already exists; it has not been overwritten. Verify that servicelb is disabled if MetalLB will be used."
    return
  fi

  cat >/etc/rancher/k3s/config.yaml <<'EOF'
write-kubeconfig-mode: "0640"
disable:
  - servicelb
EOF
}

install_k3s() {
  if command -v k3s >/dev/null 2>&1; then
    log "K3s is already installed; skipping binary installation"
  else
    log "Downloading the official K3s installer"
    curl -sfL https://get.k3s.io -o /tmp/eubl21-install-k3s.sh
    chmod 0700 /tmp/eubl21-install-k3s.sh

    log "Installing K3s"
    if [[ -n "${K3S_VERSION}" ]]; then
      INSTALL_K3S_VERSION="${K3S_VERSION}" /tmp/eubl21-install-k3s.sh
    else
      /tmp/eubl21-install-k3s.sh
    fi
  fi

  systemctl enable --now k3s

  export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

  log "Waiting for Kubernetes node readiness"
  kubectl wait --for=condition=Ready node --all --timeout=300s
}

install_user_kubeconfig() {
  if [[ -z "${SUDO_USER:-}" || "${SUDO_USER}" == "root" ]]; then
    return
  fi

  local target_home
  target_home="$(getent passwd "${SUDO_USER}" | cut -d: -f6)"
  [[ -n "${target_home}" ]] || return

  install -d -m 0700 -o "${SUDO_USER}" -g "${SUDO_USER}" "${target_home}/.kube"
  cp /etc/rancher/k3s/k3s.yaml "${target_home}/.kube/config"
  chown "${SUDO_USER}:${SUDO_USER}" "${target_home}/.kube/config"
  chmod 0600 "${target_home}/.kube/config"

  log "Installed kubeconfig for ${SUDO_USER} at ${target_home}/.kube/config"
}

install_helm() {
  if command -v helm >/dev/null 2>&1; then
    log "Helm is already installed; skipping binary installation"
    return
  fi

  log "Downloading the official Helm installer"
  curl -fsSL \
    -o /tmp/eubl21-get-helm-3.sh \
    https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
  chmod 0700 /tmp/eubl21-get-helm-3.sh

  log "Installing Helm"
  /tmp/eubl21-get-helm-3.sh
}

ensure_namespace() {
  local namespace="$1"
  kubectl create namespace "${namespace}" --dry-run=client -o yaml | kubectl apply -f -
}

add_helm_repositories() {
  log "Adding Helm repositories"

  helm repo add metallb https://metallb.github.io/metallb --force-update
  helm repo add kedacore https://kedacore.github.io/charts --force-update
  helm repo add nats https://nats-io.github.io/k8s/helm/charts/ --force-update
  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
  helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts --force-update
  helm repo add cnpg https://cloudnative-pg.github.io/charts --force-update
  helm repo update
}

install_metallb() {
  log "Installing MetalLB"
  ensure_namespace metallb-system

  kubectl label namespace metallb-system \
    pod-security.kubernetes.io/enforce=privileged \
    pod-security.kubernetes.io/audit=privileged \
    pod-security.kubernetes.io/warn=privileged \
    --overwrite

  # The single-server target uses L2 advertisement only. Disable FRR/FRR-K8s
  # to keep the MetalLB footprint small and avoid unnecessary BGP components.
  helm upgrade --install metallb metallb/metallb \
    --namespace metallb-system \
    --set speaker.frr.enabled=false \
    --set frrk8s.enabled=false \
    --wait \
    --timeout 10m

  if [[ -z "${METALLB_ADDRESS_POOL}" ]]; then
    warn "METALLB_ADDRESS_POOL is empty. MetalLB is installed but no LAN address pool has been configured."
    return
  fi

  log "Configuring MetalLB address pool: ${METALLB_ADDRESS_POOL}"
  cat <<EOF | kubectl apply -f -
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: eubl21-lan
  namespace: metallb-system
spec:
  addresses:
    - ${METALLB_ADDRESS_POOL}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: eubl21-lan
  namespace: metallb-system
spec:
  ipAddressPools:
    - eubl21-lan
EOF
}

install_cert_manager() {
  log "Installing cert-manager"
  helm upgrade --install cert-manager \
    oci://quay.io/jetstack/charts/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --set crds.enabled=true \
    --wait \
    --timeout 10m
}

install_keda() {
  log "Installing KEDA"
  helm upgrade --install keda kedacore/keda \
    --namespace keda \
    --create-namespace \
    --wait \
    --timeout 10m
}

install_cloudnativepg() {
  log "Installing CloudNativePG operator"
  helm upgrade --install cnpg cnpg/cloudnative-pg \
    --namespace cnpg-system \
    --create-namespace \
    --wait \
    --timeout 10m
}

ensure_database_secret() {
  if kubectl get secret eubl21-db-auth -n "${EUBL21_NAMESPACE}" >/dev/null 2>&1; then
    log "Database bootstrap secret already exists; preserving it"
    return
  fi

  local password
  password="$(openssl rand -base64 32 | tr -d '\n')"

  kubectl create secret generic eubl21-db-auth \
    --namespace "${EUBL21_NAMESPACE}" \
    --type kubernetes.io/basic-auth \
    --from-literal=username=eubl21 \
    --from-literal=password="${password}"
}

install_postgresql_cluster() {
  log "Creating/updating the PostgreSQL cluster"
  ensure_database_secret

  cat <<EOF | kubectl apply -f -
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: e-ubl21-postgres
  namespace: ${EUBL21_NAMESPACE}
spec:
  instances: 1
  bootstrap:
    initdb:
      database: eubl21
      owner: eubl21
      secret:
        name: eubl21-db-auth
  storage:
    size: ${POSTGRES_STORAGE_SIZE}
EOF
}

install_nats() {
  log "Installing NATS JetStream"
  helm upgrade --install nats nats/nats \
    --namespace "${EUBL21_NAMESPACE}" \
    --create-namespace \
    --set config.jetstream.enabled=true \
    --set config.jetstream.fileStore.pvc.size="${NATS_STORAGE_SIZE}" \
    --wait \
    --timeout 10m
}

ensure_minio_secret() {
  if kubectl get secret eubl21-minio-auth -n "${EUBL21_NAMESPACE}" >/dev/null 2>&1; then
    log "MinIO bootstrap secret already exists; preserving it"
    return
  fi

  local password
  password="$(openssl rand -base64 36 | tr -d '\n')"

  kubectl create secret generic eubl21-minio-auth \
    --namespace "${EUBL21_NAMESPACE}" \
    --from-literal=MINIO_ROOT_USER=eubl21 \
    --from-literal=MINIO_ROOT_PASSWORD="${password}"
}

install_minio() {
  log "Installing single-node MinIO-compatible object storage"
  ensure_minio_secret

  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: minio
  namespace: ${EUBL21_NAMESPACE}
spec:
  selector:
    app.kubernetes.io/name: minio
  ports:
    - name: s3
      port: 9000
      targetPort: s3
    - name: console
      port: 9001
      targetPort: console
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: minio
  namespace: ${EUBL21_NAMESPACE}
spec:
  serviceName: minio
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: minio
  template:
    metadata:
      labels:
        app.kubernetes.io/name: minio
    spec:
      containers:
        - name: minio
          image: ${MINIO_IMAGE}
          imagePullPolicy: IfNotPresent
          args:
            - server
            - /data
            - --console-address
            - :9001
          envFrom:
            - secretRef:
                name: eubl21-minio-auth
          ports:
            - name: s3
              containerPort: 9000
            - name: console
              containerPort: 9001
          readinessProbe:
            httpGet:
              path: /minio/health/ready
              port: s3
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /minio/health/live
              port: s3
            initialDelaySeconds: 15
            periodSeconds: 20
          volumeMounts:
            - name: data
              mountPath: /data
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: ${MINIO_STORAGE_SIZE}
EOF
}

install_monitoring() {
  log "Installing Prometheus and Grafana"
  helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
    --namespace monitoring \
    --create-namespace \
    --set 'prometheus.prometheusSpec.storageSpec.volumeClaimTemplate.spec.accessModes[0]=ReadWriteOnce' \
    --set "prometheus.prometheusSpec.storageSpec.volumeClaimTemplate.spec.resources.requests.storage=${PROMETHEUS_STORAGE_SIZE}" \
    --set grafana.persistence.enabled=true \
    --set "grafana.persistence.size=${GRAFANA_STORAGE_SIZE}" \
    --wait \
    --timeout 15m
}

install_opentelemetry() {
  log "Installing OpenTelemetry Collector"
  helm upgrade --install otel-collector open-telemetry/opentelemetry-collector \
    --namespace observability \
    --create-namespace \
    --set mode=deployment \
    --set image.repository=otel/opentelemetry-collector-k8s \
    --wait \
    --timeout 10m
}

record_state() {
  install -d -m 0700 "${STATE_DIR}"

  {
    printf 'Installed: %s\n' "$(date --iso-8601=seconds)"
    printf '\nOS:\n'
    cat /etc/os-release
    printf '\nK3s:\n'
    k3s --version || true
    printf '\nHelm:\n'
    helm version --short || true
    printf '\nKubernetes:\n'
    kubectl version --client || true
    printf '\nNodes:\n'
    kubectl get nodes -o wide || true
    printf '\nHelm releases:\n'
    helm list -A || true
  } >"${STATE_DIR}/install-state.txt"

  chmod 0600 "${STATE_DIR}/install-state.txt"
}

print_summary() {
  log "Installation summary"
  kubectl get nodes -o wide
  kubectl get pods -A
  kubectl get pvc -A

  cat <<EOF

The e-ubl21 on-premises platform bootstrap is complete.

Namespace: ${EUBL21_NAMESPACE}
State:     ${STATE_DIR}/install-state.txt

Useful checks:
  kubectl get pods -A
  kubectl get pvc -A
  kubectl get clusters.postgresql.cnpg.io -n ${EUBL21_NAMESPACE}
  helm list -A

Retrieve PostgreSQL credentials only when needed:
  kubectl get secret eubl21-db-auth -n ${EUBL21_NAMESPACE} -o jsonpath='{.data.username}' | base64 -d; echo
  kubectl get secret eubl21-db-auth -n ${EUBL21_NAMESPACE} -o jsonpath='{.data.password}' | base64 -d; echo

Retrieve MinIO credentials only when needed:
  kubectl get secret eubl21-minio-auth -n ${EUBL21_NAMESPACE} -o jsonpath='{.data.MINIO_ROOT_USER}' | base64 -d; echo
  kubectl get secret eubl21-minio-auth -n ${EUBL21_NAMESPACE} -o jsonpath='{.data.MINIO_ROOT_PASSWORD}' | base64 -d; echo
EOF

  if [[ -z "${METALLB_ADDRESS_POOL}" ]]; then
    cat <<'EOF'

MetalLB has no address pool yet. Re-run with a reserved LAN range, for example:
  METALLB_ADDRESS_POOL=192.168.1.200-192.168.1.220 sudo -E bash deploy/on-prem/scripts/install-on-prem.sh
EOF
  fi
}

main() {
  require_root
  load_os
  install_base_packages
  warn_about_active_firewall
  disable_swap_if_requested
  prepare_k3s_config
  install_k3s
  install_user_kubeconfig
  install_helm

  if [[ "${INSTALL_PLATFORM}" != "1" ]]; then
    warn "INSTALL_PLATFORM=${INSTALL_PLATFORM}; stopping after K3s + Helm installation"
    record_state
    exit 0
  fi

  export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

  ensure_namespace "${EUBL21_NAMESPACE}"
  add_helm_repositories
  install_metallb
  install_cert_manager
  install_keda
  install_cloudnativepg
  install_postgresql_cluster
  install_nats
  install_minio
  install_monitoring
  install_opentelemetry
  record_state
  print_summary
}

main "$@"
