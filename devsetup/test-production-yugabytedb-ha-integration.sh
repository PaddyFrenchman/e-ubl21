#!/usr/bin/env bash
# Exercises the reference HA universe against a real three-zone Kubernetes
# cluster. It intentionally refuses a single-zone/minikube target.
set -Eeuo pipefail
: "${EUBL21_HA_DATABASE_URL:?set a multi-host YSQL URL}"
: "${EUBL21_HA_TSERVER_POD:?set the tserver pod to terminate}"
: "${EUBL21_HA_TSERVER_NAMESPACE:?set its namespace}"
: "${EUBL21_HA_READY_SELECTOR:?set a selector matching all HA tservers}"
expected_tservers="${EUBL21_HA_TSERVER_COUNT:-3}"
timeout="${EUBL21_HA_TIMEOUT:-10m}"

ready_before="$(kubectl get pods -A -l "$EUBL21_HA_READY_SELECTOR" --field-selector=status.phase=Running --no-headers | wc -l)"
[[ "$ready_before" -ge "$expected_tservers" ]] || {
  echo "expected at least $expected_tservers running tservers, found $ready_before" >&2; exit 1; }

ready_file="$(mktemp)"; done_file="$(mktemp)"; rm -f "$ready_file" "$done_file"; trap 'rm -f "$ready_file" "$done_file"' EXIT
EUBL21_TEST_DATABASE_URL="$EUBL21_HA_DATABASE_URL" EUBL21_TEST_DATABASE_FAILOVER_READY_FILE="$ready_file" EUBL21_TEST_DATABASE_FAILOVER_DONE_FILE="$done_file" \
  go test ./database -run '^TestYugabyteDBIntegration$/same_pool_reconnects_after_failover' -count=1 &
test_pid=$!
until [[ -f "$ready_file" ]]; do sleep 1; done
kubectl delete pod "$EUBL21_HA_TSERVER_POD" -n "$EUBL21_HA_TSERVER_NAMESPACE" --wait=true
touch "$done_file"
kubectl wait --for=condition=Ready pod -A -l "$EUBL21_HA_READY_SELECTOR" --timeout="$timeout"

wait "$test_pid"

echo "Production YugabyteDB HA tserver-failure and application reconnect qualification passed."
