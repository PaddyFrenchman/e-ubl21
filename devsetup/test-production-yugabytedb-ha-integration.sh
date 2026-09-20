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

# Establish a pool before failure. The integration test's reconnect case must
# use the same URL that production applications receive.
EUBL21_TEST_DATABASE_URL="$EUBL21_HA_DATABASE_URL" \
  go test ./database -run '^TestYugabyteDBIntegration$/connect_and_query' -count=1

kubectl delete pod "$EUBL21_HA_TSERVER_POD" -n "$EUBL21_HA_TSERVER_NAMESPACE" --wait=true
kubectl wait --for=condition=Ready pod -A -l "$EUBL21_HA_READY_SELECTOR" --timeout="$timeout"

# A newly opened application pool must reconnect through the HA endpoint after
# the terminated tserver has been replaced and tablet leadership has recovered.
EUBL21_TEST_DATABASE_URL="$EUBL21_HA_DATABASE_URL" \
  go test ./database -run '^TestYugabyteDBIntegration$/connect_and_query' -count=1

echo "Production YugabyteDB HA tserver-failure and application reconnect qualification passed."
