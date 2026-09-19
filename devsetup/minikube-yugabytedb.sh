#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NAMESPACE="${EUBL21_DB_NAMESPACE:-e-ubl21-regression}"
IMAGE="${IMAGE:-localhost/e-ubl21:regression}"
KUBECTL_BIN="${KUBECTL_BIN:-$ROOT_DIR/devsetup/minikube-kubectl.sh}"
ARTIFACT_DIR="${ARTIFACT_DIR:-$ROOT_DIR/regression-results/minikube}"
ROLLOUT_TIMEOUT="${ROLLOUT_TIMEOUT:-5m}"
YUGABYTEDB_CHART_URL="https://charts.yugabyte.com/yugabyte-2025.2.6.tgz"
YUGABYTEDB_CHART_SHA256="26518b8dfb7389dd6e97bc034df59838236c1a0c15d6f7482a1cf9a7dd4b80a0"
YUGABYTEDB_IMAGE="yugabytedb/yugabyte:2025.2.6.0-b111"
YUGABYTEDB_VALUES="$ROOT_DIR/deploy/minikube/yugabytedb-values.yaml"
DATABASE_SECRET="eubl21-db-auth"
DATABASE_USER="eubl21"
DATABASE_PASSWORD="eubl21-minikube-regression-only"
DATABASE_URL="postgresql://${DATABASE_USER}:${DATABASE_PASSWORD}@yb-tservers.${NAMESPACE}.svc.cluster.local:5433/eubl21?sslmode=disable"
TEMPORARY_DIR="$(mktemp -d)"
DATABASE_ARTIFACT_DIR="$ARTIFACT_DIR/database"
CURRENT_PHASE="preflight"
READINESS_RESULT="not-run"
BOOTSTRAP_RESULT="not-run"
MIGRATION_RESULT="not-run"
SMOKE_RESULT="not-run"
RESTART_RESULT="not-run"
YUGABYTEDB_VERSION=""
SCHEMA_VERSION=""

cleanup() {
    rm -rf "$TEMPORARY_DIR"
}
trap cleanup EXIT

for command in curl helm sha256sum; do
    command -v "$command" >/dev/null 2>&1 || {
        echo "ERROR: required YugabyteDB regression command not found: $command" >&2
        exit 2
    }
done
[[ -x "$KUBECTL_BIN" ]] || {
    echo "ERROR: Minikube kubectl wrapper is not executable: $KUBECTL_BIN" >&2
    exit 2
}
[[ -r "$YUGABYTEDB_VALUES" ]] || {
    echo "ERROR: YugabyteDB regression values are missing: $YUGABYTEDB_VALUES" >&2
    exit 2
}

mkdir -p "$DATABASE_ARTIFACT_DIR"

write_summary() {
    local integration_result=$1 failed_phase=${2:-}
    local version_json=null schema_version_json=null failed_phase_json=null
    if [[ -n "$YUGABYTEDB_VERSION" ]]; then
        version_json="\"$YUGABYTEDB_VERSION\""
    fi
    if [[ "$SCHEMA_VERSION" =~ ^[0-9]+$ ]]; then
        schema_version_json="$SCHEMA_VERSION"
    fi
    if [[ -n "$failed_phase" ]]; then
        failed_phase_json="\"$failed_phase\""
    fi
    cat >"$DATABASE_ARTIFACT_DIR/summary.json" <<EOF
{
  "schemaVersion": 1,
  "yugabyteDBVersion": ${version_json},
  "databaseIntegration": "${integration_result}",
  "readiness": "${READINESS_RESULT}",
  "bootstrap": "${BOOTSTRAP_RESULT}",
  "migration": "${MIGRATION_RESULT}",
  "smoke": "${SMOKE_RESULT}",
  "applicationSchemaVersion": ${schema_version_json},
  "tserverRestart": "${RESTART_RESULT}",
  "failedPhase": ${failed_phase_json}
}
EOF
}

record_failure() {
    local status=$?
    trap - ERR
    write_summary fail "$CURRENT_PHASE"
    echo "ERROR: YugabyteDB Minikube integration failed during ${CURRENT_PHASE}" >&2
    exit "$status"
}
trap record_failure ERR

wait_for_job() {
    local job_name=$1
    if ! "$KUBECTL_BIN" --namespace "$NAMESPACE" wait \
        --for=condition=Complete "job/$job_name" --timeout="$ROLLOUT_TIMEOUT"; then
        "$KUBECTL_BIN" --namespace "$NAMESPACE" describe "job/$job_name" || true
        "$KUBECTL_BIN" --namespace "$NAMESPACE" logs "job/$job_name" --all-containers=true || true
        return 1
    fi
    "$KUBECTL_BIN" --namespace "$NAMESPACE" logs "job/$job_name" --all-containers=true
}

apply_bootstrap_job() {
    "$KUBECTL_BIN" --namespace "$NAMESPACE" create configmap eubl21-yugabytedb-bootstrap \
        --from-file="yugabytedb-bootstrap.sh=$ROOT_DIR/devsetup/yugabytedb-bootstrap.sh" \
        --dry-run=client -o yaml | "$KUBECTL_BIN" apply -f -
    "$KUBECTL_BIN" --namespace "$NAMESPACE" delete job eubl21-yugabytedb-bootstrap \
        --ignore-not-found=true --wait=true
    cat <<EOF | "$KUBECTL_BIN" apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: eubl21-yugabytedb-bootstrap
  namespace: ${NAMESPACE}
  labels:
    app.kubernetes.io/name: eubl21-yugabytedb-bootstrap
    app.kubernetes.io/part-of: e-ubl21
spec:
  backoffLimit: 3
  template:
    metadata:
      labels:
        app.kubernetes.io/name: eubl21-yugabytedb-bootstrap
        app.kubernetes.io/part-of: e-ubl21
    spec:
      restartPolicy: Never
      containers:
        - name: bootstrap
          image: ${YUGABYTEDB_IMAGE}
          imagePullPolicy: IfNotPresent
          command: [/bin/bash, /opt/e-ubl21/yugabytedb-bootstrap.sh]
          env:
            - name: YSQL_HOST
              value: yb-tservers.${NAMESPACE}.svc.cluster.local
            - name: YSQL_USER
              valueFrom:
                secretKeyRef:
                  name: ${DATABASE_SECRET}
                  key: username
            - name: YSQL_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: ${DATABASE_SECRET}
                  key: password
            - name: YSQL_DB
              value: eubl21
            - name: YSQL_EXPECTED_VERSION
              value: 2025.2.6.0-b111
          volumeMounts:
            - name: bootstrap-script
              mountPath: /opt/e-ubl21
              readOnly: true
      volumes:
        - name: bootstrap-script
          configMap:
            name: eubl21-yugabytedb-bootstrap
            defaultMode: 0555
EOF
    wait_for_job eubl21-yugabytedb-bootstrap | tee "$DATABASE_ARTIFACT_DIR/bootstrap.log"
}

apply_migration_job() {
    local suffix=$1 job_name="eubl21-database-migrate-$1"
    "$KUBECTL_BIN" --namespace "$NAMESPACE" delete job "$job_name" --ignore-not-found=true --wait=true
    cat <<EOF | "$KUBECTL_BIN" apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job_name}
  namespace: ${NAMESPACE}
  labels:
    app.kubernetes.io/name: eubl21-database-migrate
    app.kubernetes.io/part-of: e-ubl21
spec:
  backoffLimit: 1
  template:
    metadata:
      labels:
        app.kubernetes.io/name: eubl21-database-migrate
        app.kubernetes.io/part-of: e-ubl21
    spec:
      restartPolicy: Never
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: migrate
          image: ${IMAGE}
          imagePullPolicy: Never
          args: [database, migrate, --timeout, 5m]
          env:
            - name: EUBL21_DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: ${DATABASE_SECRET}
                  key: database-url
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: [ALL]
            readOnlyRootFilesystem: true
EOF
    wait_for_job "$job_name" | tee "$DATABASE_ARTIFACT_DIR/migration-${suffix}.log"
}

apply_smoke_job() {
    local suffix=$1 job_name="eubl21-database-smoke-$1"
    "$KUBECTL_BIN" --namespace "$NAMESPACE" delete job "$job_name" --ignore-not-found=true --wait=true
    cat <<EOF | "$KUBECTL_BIN" apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job_name}
  namespace: ${NAMESPACE}
  labels:
    app.kubernetes.io/name: eubl21-database-smoke
    app.kubernetes.io/part-of: e-ubl21
spec:
  backoffLimit: 1
  template:
    metadata:
      labels:
        app.kubernetes.io/name: eubl21-database-smoke
        app.kubernetes.io/part-of: e-ubl21
    spec:
      restartPolicy: Never
      containers:
        - name: smoke
          image: ${YUGABYTEDB_IMAGE}
          imagePullPolicy: IfNotPresent
          command: [/bin/bash, -ceu, --]
          args:
            - |
              export PGPASSWORD="\$YSQL_PASSWORD"
              ysql=(/home/yugabyte/bin/ysqlsh --host "\$YSQL_HOST" --port 5433 --username "\$YSQL_USER" --dbname eubl21 --no-psqlrc --no-align --tuples-only --set ON_ERROR_STOP=1)
              one="\$("\${ysql[@]}" --command 'SELECT 1')"
              version="\$("\${ysql[@]}" --command "SELECT current_setting('server_version')")"
              schema_version="\$("\${ysql[@]}" --command "SELECT COALESCE(MAX(version), 0) FROM public.eubl21_schema_migrations WHERE state = 'applied'")"
              invalid_history="\$("\${ysql[@]}" --command "SELECT COUNT(*) FROM public.eubl21_schema_migrations WHERE state <> 'applied'")"
              metadata="\$("\${ysql[@]}" --command "SELECT COUNT(*) FROM eubl21.schema_metadata WHERE id = 1 AND product = 'e-ubl21'")"
              [[ "\$one" == 1 && "\$schema_version" =~ ^[0-9]+\$ && "\$schema_version" -gt 0 && "\$invalid_history" == 0 && "\$metadata" == 1 ]]
              printf 'YUGABYTEDB_VERSION=%s\\nSCHEMA_VERSION=%s\\n' "\$version" "\$schema_version"
          env:
            - name: YSQL_HOST
              value: yb-tservers.${NAMESPACE}.svc.cluster.local
            - name: YSQL_USER
              valueFrom:
                secretKeyRef:
                  name: ${DATABASE_SECRET}
                  key: username
            - name: YSQL_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: ${DATABASE_SECRET}
                  key: password
EOF
    wait_for_job "$job_name" | tee "$DATABASE_ARTIFACT_DIR/smoke-${suffix}.log"
}

CURRENT_PHASE="chart-download"
echo "Installing pinned YugabyteDB ${YUGABYTEDB_IMAGE} into ${NAMESPACE}"
curl -fsSL "$YUGABYTEDB_CHART_URL" -o "$TEMPORARY_DIR/yugabyte.tgz"
printf '%s  %s\n' "$YUGABYTEDB_CHART_SHA256" "$TEMPORARY_DIR/yugabyte.tgz" |
    sha256sum --check --status
helm template yugabyte "$TEMPORARY_DIR/yugabyte.tgz" \
    --namespace "$NAMESPACE" \
    --kube-version 1.35.1 \
    --values "$YUGABYTEDB_VALUES" \
    --set-string Image.repository=yugabytedb/yugabyte \
    --set-string Image.tag=2025.2.6.0-b111 \
    --set-string storage.master.size=1Gi \
    --set-string storage.tserver.size=1Gi \
    --set-string storage.master.storageClass=standard \
    --set-string storage.tserver.storageClass=standard \
    >"$TEMPORARY_DIR/yugabytedb.yaml"
# The pinned chart emits PodDisruptionBudget policy/v1beta1 resources. The
# Kubernetes v1.35 API served by the regression profile only supports the
# schema-compatible policy/v1 form.
sed -i 's#apiVersion: policy/v1beta1#apiVersion: policy/v1#g' "$TEMPORARY_DIR/yugabytedb.yaml"
"$KUBECTL_BIN" --namespace "$NAMESPACE" apply -f "$TEMPORARY_DIR/yugabytedb.yaml"
CURRENT_PHASE="database-readiness"
READINESS_RESULT="fail"
"$KUBECTL_BIN" --namespace "$NAMESPACE" rollout status statefulset/yb-master --timeout="$ROLLOUT_TIMEOUT"
"$KUBECTL_BIN" --namespace "$NAMESPACE" rollout status statefulset/yb-tserver --timeout="$ROLLOUT_TIMEOUT"
READINESS_RESULT="pass"

CURRENT_PHASE="database-secret"
"$KUBECTL_BIN" --namespace "$NAMESPACE" create secret generic "$DATABASE_SECRET" \
    --type=kubernetes.io/basic-auth \
    --from-literal="username=$DATABASE_USER" \
    --from-literal="password=$DATABASE_PASSWORD" \
    --from-literal="database-url=$DATABASE_URL" \
    --dry-run=client -o yaml | "$KUBECTL_BIN" apply -f -

CURRENT_PHASE="database-bootstrap"
BOOTSTRAP_RESULT="fail"
apply_bootstrap_job
BOOTSTRAP_RESULT="pass"
CURRENT_PHASE="initial-migration"
MIGRATION_RESULT="fail"
apply_migration_job initial
MIGRATION_RESULT="pass"
CURRENT_PHASE="initial-smoke"
SMOKE_RESULT="fail"
apply_smoke_job initial
SMOKE_RESULT="pass"

echo "Restarting YugabyteDB tserver and revalidating the durable schema"
CURRENT_PHASE="tserver-restart"
RESTART_RESULT="fail"
"$KUBECTL_BIN" --namespace "$NAMESPACE" delete pod yb-tserver-0 --wait=true
"$KUBECTL_BIN" --namespace "$NAMESPACE" rollout status statefulset/yb-tserver --timeout="$ROLLOUT_TIMEOUT"
RESTART_RESULT="pass"
CURRENT_PHASE="restart-migration"
MIGRATION_RESULT="fail"
apply_migration_job restart
MIGRATION_RESULT="pass"
CURRENT_PHASE="restart-smoke"
SMOKE_RESULT="fail"
apply_smoke_job restart
SMOKE_RESULT="pass"

CURRENT_PHASE="evidence-validation"
YUGABYTEDB_VERSION="$(sed -n 's/^YUGABYTEDB_VERSION=//p' "$DATABASE_ARTIFACT_DIR/smoke-restart.log" | tail -n 1)"
SCHEMA_VERSION="$(sed -n 's/^SCHEMA_VERSION=//p' "$DATABASE_ARTIFACT_DIR/smoke-restart.log" | tail -n 1)"
[[ -n "$YUGABYTEDB_VERSION" && "$SCHEMA_VERSION" =~ ^[0-9]+$ && "$SCHEMA_VERSION" -gt 0 ]] || {
    echo "ERROR: YugabyteDB smoke evidence was incomplete" >&2
    exit 1
}
write_summary pass
echo "YugabyteDB integration passed at ${YUGABYTEDB_VERSION}, schema version ${SCHEMA_VERSION}"
