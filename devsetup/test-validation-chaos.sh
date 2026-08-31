#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

required_tests=(
    TestReconcilerRepairsCrashAfterIntentBeforePublish
    TestXSDWorkerShutdownCancelsValidationAndRedeliversRequest
    TestXSDWorkerHandleControlAndPublicationFailures
    TestReconcilerRepairsCrashAfterResultStateBeforeNextPublish
    TestConcurrentDuplicateResultsConvergeWithoutRedelivery
    TestENWorkerShutdownCancelsValidationAndRedeliversRequest
    TestENWorkerHandleControlAndPublicationFailures
    TestChaosGateDuplicateENResultsProduceOneFinalResult
    TestXSDWorkerRecoversAfterTemporaryObjectStoreOutage
    TestXSDWorkerLeavesRequestUnacknowledgedUntilResultPublicationSucceeds
    TestENWorkerLeavesRequestUnacknowledgedUntilResultPublicationSucceeds
    TestFinalResultPublicationFailureLeavesDurablePendingIntentForReconciliation
    TestChaosGateRestartDrainsThousandsOfCompletedTransitions
    TestENWorkerScalesFromZeroAndFinalizesWithoutXSDWorker
    TestXSDWorkerRejectsStalePreviousGenerationAfterRevalidation
    TestChaosGateConcurrentReplayCommandsConverge
)

listed_tests="$(go test ./orchestrator ./server -list '^Test')"
missing=()
for test_name in "${required_tests[@]}"; do
    if ! grep -Fxq "$test_name" <<<"$listed_tests"; then
        missing+=("$test_name")
    fi
done
if (( ${#missing[@]} > 0 )); then
    echo "ERROR: validation chaos gate is missing required tests:" >&2
    printf '  %s\n' "${missing[@]}" >&2
    exit 1
fi

pattern="$(IFS='|'; printf '%s' "${required_tests[*]}")"
go test ./orchestrator ./server -count=1 -run "^(${pattern})$"
