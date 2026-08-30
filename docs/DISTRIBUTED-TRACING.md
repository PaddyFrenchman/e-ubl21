# Distributed validation tracing

The API, queue publishers, stage workers, and orchestrator use OpenTelemetry
spans with W3C Trace Context propagation. A single trace ID follows an upload
through durable asynchronous boundaries; stable upload, validation, generation,
request, and result IDs remain available as span attributes for exact lookup.

## Propagation contract

HTTP accepts `traceparent` and `tracestate`. Durable NATS messages carry those
same fields as headers. `StageRequest` and `StageResult` V1 JSON are unchanged:
trace metadata is operational context, not immutable validation evidence, and
does not alter deterministic request/result IDs.

Stage dead-letter and canonical result publications also retain the active
trace headers, so broker-side evidence inspection can be correlated with the
request/result path.

Only W3C Trace Context is propagated. OpenTelemetry baggage is intentionally
excluded because it is caller-controlled and could contain sensitive business
data. Invalid remote context is ignored. JetStream retains headers across
redelivery, and every republish injects the active context.

Crash-safe outbox intents also persist a validated, bounded carrier containing
only `traceparent` and optional `tracestate`. Upload enqueue, stage dispatch,
and final-result reconciliation restore that carrier before publication, so a
process restart does not begin an unrelated trace. These fields remain
operational metadata: they are not validation evidence and do not contribute
to deterministic upload, request, result, or execution identity.

The upload response includes the active `traceparent`, allowing a caller or
operator to correlate the accepted upload immediately.

## Span contract

| Span | Boundary measured |
| --- | --- |
| `api.upload.accept` | HTTP upload parsing and durable acceptance |
| `queue.document.publish` / `queue.document.consume` | Legacy document handoff during migration |
| `orchestrator.start` | Creation/recovery of a validation execution |
| `orchestrator.stage_request.queue` | Durable stage outbox publication and state transition |
| `queue.stage_request.publish` | JetStream stage-request publication |
| `queue.stage_request.wait` | Durable request creation to worker handling |
| `stage.request.process` | Worker handling, including authorization and publication |
| `stage.xml.fetch` | Opening, consuming, checking, and closing the immutable XML stream; this overlaps validator execution |
| `stage.validator.execute` | Validator execution while consuming that stream |
| `queue.stage_result.publish` / `queue.stage_result.consume` | Durable result handoff |
| `orchestrator.stage_result.transition` | Evidence persistence and CAS state transition |
| `orchestrator.final_result.publish` | Canonical result publication |

Queue-wait spans use `StageRequest.CreatedAt` as their start timestamp and the
actual worker handler-entry time as their end. Broker delivery count is a
bounded numeric attribute, so a redelivery is visible without inventing a new
logical attempt.

## Safe attributes

Spans may contain only routing/correlation metadata:

- upload and validation IDs;
- validation generation;
- stage name, ordinal, and attempt;
- deterministic request/result IDs;
- bounded stage outcome;
- broker receive count.

Invoice XML, party names, addresses, identifiers from the document, rule text,
diagnostics, and OpenTelemetry baggage are never trace attributes. Operational
errors are recorded as span errors without copying arbitrary error text into
attributes.

## Export configuration

The executable installs an OpenTelemetry SDK provider for every command. Set a
standard OTLP/HTTP endpoint to export spans:

```text
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
OTEL_SERVICE_NAME=e-ubl21-worker-xsd
```

`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` may be used for a traces-only endpoint.
Standard OpenTelemetry resource environment variables remain supported. When
no endpoint is configured, spans are not exported, but valid trace/span IDs are
still produced for structured-log and durable-header correlation.

Worker completion logs include `trace_id` and `span_id` alongside the stable
upload/validation/generation/request identity. Sampling is parent-based: a
valid incoming sampling decision is preserved across every queue boundary.
