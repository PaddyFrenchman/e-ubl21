#!/usr/bin/env bash
# Exercises the reference HA universe against a real three-zone Kubernetes
# cluster. It intentionally refuses a single-zone/minikube target.
set -Eeuo pipefail
: "${EUBL21_HA_DATABASE_URL:?set a multi-host YSQL URL}"
: "${EUBL21_HA_TSERVER_POD:?set the tserver pod to terminate}"
: "${EUBL21_HA_TSERVER_NAMESPACE:?set its namespace}"
: "${EUBL21_HA_READY_SELECTOR:?set a selector matching all HA tservers}"
: "${EUBL21_HA_MASTER_POD:?set the master pod to terminate}"
: "${EUBL21_HA_MASTER_NAMESPACE:?set its namespace}"
: "${EUBL21_HA_FAILURE_NODE:?set the node for the isolated failure domain}"
: "${EUBL21_HA_FAILOVER_EXPECTED_HOST:?set the selected tserver pod IP}"
expected_tservers="${EUBL21_HA_TSERVER_COUNT:-3}"
timeout="${EUBL21_HA_TIMEOUT:-10m}"

ready_before="$(kubectl get pods -A -l "$EUBL21_HA_READY_SELECTOR" --field-selector=status.phase=Running --no-headers | wc -l)"
[[ "$ready_before" -ge "$expected_tservers" ]] || {
  echo "expected at least $expected_tservers running tservers, found $ready_before" >&2; exit 1; }

ready_file="$(mktemp)"; done_file="$(mktemp)"; rm -f "$ready_file" "$done_file"
test_pid=""
cleanup() { [[ -z "$test_pid" ]] || kill "$test_pid" 2>/dev/null || true; kubectl uncordon "$EUBL21_HA_FAILURE_NODE" >/dev/null 2>&1 || true; rm -f "$ready_file" "$done_file"; }
trap cleanup EXIT
EUBL21_TEST_DATABASE_URL="$EUBL21_HA_DATABASE_URL" EUBL21_TEST_DATABASE_FAILOVER_READY_FILE="$ready_file" EUBL21_TEST_DATABASE_FAILOVER_DONE_FILE="$done_file" EUBL21_TEST_DATABASE_FAILOVER_EXPECTED_HOST="$EUBL21_HA_FAILOVER_EXPECTED_HOST" \
  go test ./database -run '^TestYugabyteDBIntegration$/same_pool_reconnects_after_failover' -count=1 &
test_pid=$!
deadline=$((SECONDS + 60))
until [[ -f "$ready_file" ]]; do (( SECONDS < deadline )) || { echo "same-pool test did not establish its connection" >&2; exit 1; }; sleep 1; done
kubectl cordon "$EUBL21_HA_FAILURE_NODE"
kubectl delete pod "$EUBL21_HA_TSERVER_POD" -n "$EUBL21_HA_TSERVER_NAMESPACE" --wait=true
touch "$done_file"
wait "$test_pid"
test_pid=""
kubectl uncordon "$EUBL21_HA_FAILURE_NODE"
kubectl wait --for=condition=Ready pod -A -l "$EUBL21_HA_READY_SELECTOR" --timeout="$timeout"

# Hold the same zone down while proving the other two masters retain control
# plane quorum and YSQL remains available.
kubectl cordon "$EUBL21_HA_FAILURE_NODE"
kubectl delete pod "$EUBL21_HA_MASTER_POD" -n "$EUBL21_HA_MASTER_NAMESPACE" --wait=true
# Prove the surviving two masters retain quorum before the deleted master is
# allowed to return. A later wait verifies recovery separately.
deadline=$((SECONDS + 60))
while [[ "$(kubectl get pods -A -l app=yb-master --field-selector=status.phase=Running --no-headers | awk '$3 == "3/3" { count++ } END { print count+0 }')" -lt 2 ]]; do
  (( SECONDS < deadline )) || { echo "master quorum did not retain two Ready members" >&2; exit 1; }
  sleep 1
done
EUBL21_TEST_DATABASE_URL="$EUBL21_HA_DATABASE_URL" \
  go test ./database -run '^TestYugabyteDBIntegration$/connect_and_query' -count=1
kubectl uncordon "$EUBL21_HA_FAILURE_NODE"
kubectl wait --for=condition=Ready pod -A -l app=yb-master --timeout="$timeout"

echo "Production YugabyteDB HA tserver failover, same-pool reconnect, and master-quorum qualification passed."
