# Per-stage telemetry contract

Issue #58 instruments the asynchronous validation stages with Prometheus
series whose labels are closed, bounded sets. Upload, validation, document,
rule, and tenant identifiers are never metric labels.

| Metric | Labels | Meaning |
| --- | --- | --- |
| `e_ubl21_validation_stage_requests_total` | `stage`, `outcome` | Completed stage executions; `rate()` is stage throughput in documents/second. |
| `e_ubl21_validation_stage_duration_seconds` | `stage` | Validator service time, excluding queue wait. |
| `e_ubl21_validation_stage_queue_wait_seconds` | `stage` | Durable request creation to worker start. |
| `e_ubl21_validation_stage_request_age_seconds` | `stage` | Request age observed by the worker, including redelivery delay. |
| `e_ubl21_validation_stage_inflight` | `stage` | Requests currently handled by this process. |
| `e_ubl21_validation_stage_retries_total` | `stage`, `reason` | Operational outcomes for which another logical attempt remains. |
| `e_ubl21_validation_stage_redeliveries_total` | `stage` | Requests observed with broker delivery count greater than one. |
| `e_ubl21_validation_stage_queue_depth` | `stage` | Messages retained in each request stream, refreshed on scrape. |
| `e_ubl21_validation_stage_dlq_depth` | `stage` | Permanent stage DLQ entries, refreshed on scrape. |
| `e_ubl21_validation_stage_runtime_errors_total` | `stage`, `reason` | Validator operational failures. |
| `e_ubl21_validation_stage_startup_errors_total` | `stage`, `component` | Process startup failures for queue, stores, assets, or runtime. |
| `e_ubl21_validation_stage_result_publication_latency_seconds` | `stage`, `outcome` | Durable result publication attempt latency. |
| `e_ubl21_orchestrator_transition_duration_seconds` | `transition`, `stage` | State-machine result handling and request dispatch latency. |
| `e_ubl21_orchestrator_result_publication_latency_seconds` | none | Canonical final-result publication latency. |

Stage values are limited to the frozen topology (`ubl21-xsd`, `en16931`, and
reserved `peppol-bis-billing`). Outcomes, operational reasons, startup
components, and transition names are likewise enumerated by code.

Use service and queue-wait histograms together: high service latency indicates
an intrinsically slow validator, while normal service latency combined with
high queue wait, request age, and queue depth indicates insufficient worker
capacity. Alerting should use rates over counters rather than treating a raw
counter as documents per second.

Structured logs retain upload/validation/request identity for correlation but
never invoice content. Completion records include generation, stage, attempt,
outcome, operational reason, duration, and whether DLQ evidence was recovered.
Distributed trace propagation remains owned by #60;
metrics are non-authoritative and never alter durable execution state.
