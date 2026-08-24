# Server Functionality

This document lists the functionality currently implemented across the two e-ubl21
services - the HTTP API and the validation worker - and the shared `queue` package that
connects them via NATS JetStream. It reflects the code as it exists today, not the
long-term roadmap.

**Architecture at a glance:**

```
        POST /api/v1/ubl/upload           ubl.documents.<Type>              ubl.validation.results
Client ────────────────────────► API ────────────────────────► JetStream ────────────────────────► (external consumers)
                                   │      (per-document-type,                    ▲
                                   │       work-queue stream)                    │
                                   ▼                                             │
                     KV_UBL_UPLOADS (status)  ◄───────────────────────── worker (validates via xmllint,
                     KV_UBL_UPLOAD_CONTENT         updates status KV,           updates status KV, publishes result)
                     (raw XML, write-once)          never touches content
                                   ▲
                                   │
GET /api/v1/ubl/{id} ──────────────┘ (reads both KV buckets)
```

- **API** ([server.go](server.go), [upload.go](upload.go)): performs only cheap, synchronous
  checks on upload (syntactically valid root element, recognized document type by local name and
  namespace), then queues the document and returns immediately.
- **Worker** ([worker.go](worker.go)): consumes queued documents, performs the actual XSD
  validation, records the outcome, and publishes a result.
- **queue package** ([../queue](../queue)): all NATS JetStream wiring (streams, KV bucket,
  subjects) shared by both.

Both are built from the same binary via the CLI subcommands `e-ubl21 serve` (API) and
`e-ubl21 worker` (worker) - see [cmd/serve.go](../cmd/serve.go), [cmd/worker.go](../cmd/worker.go),
[cmd/root.go](../cmd/root.go) - and are run as separate `docker-compose.yml` services (see
section 14).

## 1. HTTP Endpoints (API service)

All routes are registered on a single `http.ServeMux` returned by `NewHandler`. The OpenAPI
document and Swagger UI are intentionally **public** (unauthenticated) so the API is
self-discoverable; every other route is **protected by the OAuth2 bearer-token middleware**
described in section 2. Every route also passes through the rate limiter (section 8) and the
request-observability middleware (section 9).

| Method | Path                                    | Auth      | Description                                                                 |
|--------|------------------------------------------|-----------|------------------------------------------------------------------------------|
| GET    | `/`                                      | Required  | Service info: service name, Swagger UI path, configured OAuth issuer/audience. |
| GET    | `/healthz`                               | Required  | Liveness check; returns `{"status": "ok"}`.                                  |
| GET    | `/metrics`                               | Required  | Prometheus metrics in the text exposition format (section 9).               |
| GET    | configurable (default `/openapi.json`)   | Public    | The OpenAPI 3.0.3 document (section 10).                                    |
| GET    | configurable (default `/swagger/`)       | Public    | Interactive Swagger UI rendering the OpenAPI document above.               |
| POST   | `/api/v1/ubl/upload`                     | Required  | Queues a UBL 2.1 XML document for asynchronous validation (section 4).      |
| GET    | `/api/v1/ubl/{id}`                       | Required  | Retrieves an upload's current status, content, and (once processed) validation outcome (section 6). |

Any other path returns `404 Not Found`; a non-`GET`/`POST` method on a defined route returns
`405 Method Not Allowed`; exceeding the per-client rate limit returns `429 Too Many Requests`.

The worker service exposes its own small, separate, unauthenticated HTTP surface (`/metrics`,
`/healthz`) on a different port - see section 5.

## 2. Authentication (OAuth2 Bearer Token)

Every protected API request is wrapped by `oauthValidator.requireOAuth2` ([auth.go](auth.go)),
which enforces:

- **Presence**: an `Authorization: Bearer <token>` header must be present; missing or
  malformed headers are rejected with `401 Unauthorized`.
- **Signature verification**: the token's HS256 signature is verified against a configured
  shared secret (`--oauth-hmac-secret` / `EUBL21_OAUTH_HMAC_SECRET`) using
  `jwt.ParseWithClaims` with `jwt.WithValidMethods([]string{"HS256"})`.
- **Expiration**: `jwt.WithExpirationRequired()` rejects tokens with no `exp` claim, and the
  library's built-in validator rejects tokens whose `exp` has passed.
- **Issuer check**: if `OAuthIssuer` is configured, the token's `iss` claim must match it
  exactly (`jwt.WithIssuer`).
- **Audience check**: if `OAuthAudience` is configured, the token's `aud` claim (string or
  array form) must contain it (`jwt.WithAudience`).
- **Fail-closed misconfiguration handling**: if no HMAC secret is configured, every protected
  request is rejected with `500 Internal Server Error` rather than silently accepting
  unverified tokens.

> **Note:** only HS256 (shared-secret) signatures are supported. There is no JWKS/issuer key
> discovery or RS256/ES256 support, and no scope/permission checking beyond issuer/audience
> match - see section 16 for what remains out of scope. The worker's own `/metrics`/`/healthz`
> are unauthenticated (see section 5).

## 3. NATS JetStream Queue (shared `queue` package)

Implemented in [../queue](../queue) ([client.go](../queue/client.go),
[subjects.go](../queue/subjects.go), [infra.go](../queue/infra.go),
[documents.go](../queue/documents.go), [results.go](../queue/results.go),
[records.go](../queue/records.go)):

- **`UBL_DOCUMENTS` stream** - one subject per UBL 2.1 document type, `ubl.documents.<Type>`
  (e.g. `ubl.documents.Invoice`, `ubl.documents.Order`), matched by the wildcard subject
  filter `ubl.documents.*`. Uses **work-queue retention**: each message is removed once the
  worker acknowledges it, since every document is meant to be validated exactly once. A
  7-day `MaxAge` is a safety net bounding storage growth if no worker is consuming a
  document type's queue for an extended period.
- **`UBL_VALIDATION_RESULTS` stream** - the single output queue, subject
  `ubl.validation.results`. The worker publishes one `ValidationResult` JSON message here per
  processed document (`{id, documentType, status, error, validatedAt}`), regardless of
  document type; downstream consumers (not part of this repo) can subscribe to this one
  subject to react to every outcome. Uses normal (`Limits`) retention with a 7-day `MaxAge`.
- **`UBL_UPLOADS` KV bucket** - the canonical, durable record of every upload
  (`UploadRecord`: id, document type, status, content, received-at, validated-at, error),
  keyed by upload id. This **replaces local filesystem persistence entirely**: both the API
  (which creates the "pending" record and reads it back for `GET`) and the worker (which
  updates it once validated) talk to this bucket instead of a shared disk, so the two
  services can run as independent processes/replicas with no shared volume.
- Both `NewHandler` (API) and `RunWorker` (worker) call `EnsureInfra` on startup, which
  idempotently creates the above if they don't already exist - either service can start
  first.
- Documents are tagged with `Ubl-Upload-Id`/`Ubl-Document-Type`/`Ubl-Received-At` message
  headers (so a consumer never needs to parse the payload just to route/correlate it) and use
  the upload id as the JetStream de-duplication key (`Nats-Msg-Id`).

## 4. UBL XML Upload Endpoint (`POST /api/v1/ubl/upload`)

Implemented in [upload.go](upload.go):

- Rejects non-`POST` requests with `405 Method Not Allowed`.
- Reads the request body, capped at 10 MiB and enforced exactly via `http.MaxBytesReader` (an
  already-oversized `Content-Length` is rejected before any read); an oversized body is rejected
  outright with `413 Payload Too Large` rather than silently truncated. Rejects empty bodies with
  `400 Bad Request`.
- Requires a `Content-Type` header containing `xml` or `octet-stream`; otherwise `400`.
- Validates that the payload has a **syntactically valid root element**, and extracts that root
  element's local name and namespace URI (e.g. `Invoice` in
  `urn:oasis:names:specification:ubl:schema:xsd:Invoice-2`) via a single `xml.Decoder.Token()` pass
  that stops at the first `xml.StartElement` - it does not parse or otherwise establish that the
  rest of the document is well-formed XML.
- Performs a cheap catalog lookup (no `xmllint` invocation) to confirm the root element's local
  name **and** namespace match a recognized UBL 2.1 main document type; an unrecognized type (or a
  recognized local name in the wrong namespace) is rejected immediately with `400 Bad Request`
  rather than being queued.
- **No XSD validation happens synchronously here anymore** - see section 5. A document with a
  recognized root element is always queued, even if it will later turn out not to be fully
  well-formed XML, or to be well-formed but schema-invalid.
- On success: generates a random id, writes a `"pending"` `UploadRecord` to the JetStream KV
  bucket (section 3), publishes the raw document to its per-document-type subject, and
  responds `202 Accepted`:
  ```json
  {
    "status": "pending",
    "id": "3f9c2b1a7e4d4f0a9b1c2d3e4f5a6b7c",
    "document": "Invoice",
    "receivedAt": "2026-08-20T12:00:00Z"
  }
  ```
- Every outcome (`queued`, `invalid_xml`, `unsupported_document_type`, `bad_request`,
  `internal_error`) increments a labeled Prometheus counter (section 9).

**Still out of scope:** no Schematron/PEPPOL/EN 16931 business-rule validation is performed
(see section 16) - only XSD structural correctness is checked, and only once the worker has
processed the document.

## 5. The Validation Worker (`e-ubl21 worker`)

Implemented in [worker.go](worker.go) and [worker_http.go](worker_http.go):

- `RunWorker` connects to NATS, calls `EnsureInfra`, loads the embedded UBL 2.1 schema
  catalog (section 7), and creates a **durable JetStream pull consumer** on `UBL_DOCUMENTS`
  (`--worker-consumer-name`, default `ubl-validator`) filtered to `--worker-subject-filter`
  (default: every document type, `ubl.documents.>`; set to e.g. `ubl.documents.Invoice` to
  dedicate a worker to a single document type's queue).
- For each queued document, it: resolves the document type to its XSD, runs the same
  `xmllint`-based validation the API used to run synchronously, updates the upload's KV
  record with the outcome (`valid`/`invalid` + error + `validatedAt`), and publishes a
  `ValidationResult` to the output stream (section 3).
- **Acknowledgement semantics**: a message is acked once the KV update and result publish both
  succeed, regardless of whether the document was valid or invalid (both are "successfully
  processed" outcomes). If `xmllint` is unavailable, or the KV/results-stream writes fail
  transiently, the message is `Nak`'d with a 10s delay for redelivery (up to 5 attempts,
  30s ack-wait) instead of being acknowledged. A document whose type has no matching schema
  (which the API should already have rejected) is `Term`'d - never redelivered - since
  retrying could not possibly succeed.
- **Horizontal scaling**: running multiple worker replicas with the same
  `--worker-consumer-name` load-balances documents across them (JetStream's standard
  work-queue consumer semantics); `docker-compose.yml` documents this via
  `docker compose up --scale worker=3`.
- **Graceful shutdown**: unlike `serve`, `cmd/worker.go` installs a SIGINT/SIGTERM handler
  (`signal.NotifyContext`) so a `docker stop` or Ctrl-C stops pulling new messages and lets
  any in-flight message finish being acknowledged, rather than abandoning it mid-processing.
- **Operational HTTP surface** (`--worker-metrics-addr`, default `:9091`), separate from the
  API's routes and **not behind OAuth2** (meant for an internal monitoring network only):
  - `GET /metrics` - the same Prometheus registry/exposition format as the API, but with the
    worker's own metrics (see section 9).
  - `GET /healthz` - `200` if the NATS connection is connected, `503` otherwise.

## 6. Upload Status Retrieval (`GET /api/v1/ubl/{id}`)

Implemented in [upload.go](upload.go), reading directly from the JetStream KV upload store
(section 3):

- Returns `404 Not Found` if `id` doesn't exist or isn't a well-formed upload id (32
  lowercase hex characters) - malformed ids are rejected before ever being used as a KV key.
- Otherwise returns the full `UploadRecord`: `id`, `document` (root element/type), `status`
  (`pending`, `valid`, or `invalid`), `receivedAt`, the original `content`, and - once the
  worker has processed it - `validatedAt` and, if invalid, `error` (the XSD validation
  failure detail from `xmllint`).

## 7. Real UBL 2.1 XSD Schema Validation

Implemented in [schema_catalog.go](schema_catalog.go) and the sibling
[XSD/embed.go](../XSD/embed.go) package, used by both the API (catalog lookup only) and the
worker (catalog lookup + actual validation):

- The authoritative OASIS UBL 2.1 schema tree (`XSD/common/*.xsd`, `XSD/maindoc/*.xsd`) is
  **embedded into the compiled binary** via `//go:embed`, so it does not need to exist on
  disk at the deployment location.
- At startup (`loadSchemaCatalog`, memoized once per process), the embedded tree is
  extracted to a temporary directory and every `maindoc/*.xsd` file is parsed for its
  declared root element, building a root-element-name → schema-file catalog (mirroring the
  approach used by [createsamples](../createsamples)).
- Validation itself shells out to the **`xmllint`** CLI (part of libxml2) -
  `xmllint --noout --schema <resolved-schema> <uploaded-document>` - the same independent
  validation oracle used by `createsamples` (see
  [.github/create-samples.md](../.github/create-samples.md) section 7). This is a **required
  runtime dependency of the worker**: if `xmllint` is not on `PATH`, the worker retries the
  affected message (see section 5) rather than silently skipping validation. The Docker image
  (section 14) installs `libxml2-utils` for this reason.

## 8. Rate Limiting

Implemented in [ratelimit.go](ratelimit.go) using `golang.org/x/time/rate`, the standard Go
rate-limiting primitive, applied only to the API service:

- A separate token-bucket limiter is created **per client IP** (from `RemoteAddr`), lazily on
  first request, guarded by a mutex.
- Configurable via `--rate-limit-rps` (default 5 requests/second) and `--rate-limit-burst`
  (default burst of 10).
- Applied as the outermost middleware, so it covers every route, including 404s and the
  public docs/Swagger routes.
- A background sweep removes any client's limiter after 10 minutes of inactivity so the
  limiter map does not grow unbounded across many distinct clients.
- Exceeding the limit returns `429 Too Many Requests`.

## 9. Request Logging and Metrics

Implemented in [logging.go](logging.go) and [metrics.go](metrics.go):

- A single `observabilityMiddleware` wraps every API route, recording each request's status
  and duration once and feeding both:
  - **Structured logs** via the standard library's `log/slog`: method, path, status,
    duration (ms), and remote address. The worker logs similarly (id, document type, status,
    duration) for each processed document.
  - **Prometheus metrics** via `github.com/prometheus/client_golang` (the de facto standard
    Go metrics client):
    - `e_ubl21_http_requests_total{method,route,status}` - API request counter (route is the
      low-cardinality mux pattern, not the raw path, to bound cardinality; status is bucketed
      as `2xx`/`3xx`/`4xx`/`5xx`).
    - `e_ubl21_http_request_duration_seconds{method,route}` - API request duration histogram.
    - `e_ubl21_uploads_total{outcome}` - upload attempts handled by the **API**, labeled by
      outcome (`queued`, `invalid_xml`, `unsupported_document_type`, `bad_request`,
      `internal_error`).
    - `e_ubl21_validations_total{outcome}` - documents processed by the **worker**, labeled by
      outcome (`valid`, `invalid`, `validator_unavailable`, `unsupported_document_type`,
      `internal_error`).
    - `e_ubl21_validation_duration_seconds{document_type}` - worker validation duration
      histogram.
  - Standard Go runtime/process metrics (`go_*`, `process_*`) are included automatically by
    the Prometheus client's default registry, on both services.
- The API's `/metrics` is protected by the same bearer-token requirement as other operational
  routes (consistent with `/healthz`); the worker's `/metrics` is unauthenticated (section 5).

## 10. OpenAPI Documentation and Swagger UI

- The OpenAPI 3.0.3 document is generated in-code by `openAPIDocument()`
  ([openapi.go](openapi.go)) and served as JSON at the configurable `OpenAPIPath` (default
  `/openapi.json`), documenting both `/api/v1/ubl/upload` and `/api/v1/ubl/{id}` (including
  the `pending`/`valid`/`invalid` status lifecycle) plus the shared `ErrorResponse` schema and
  the `oauth2` security scheme.
- A hand-maintained static copy is kept at [openapi.yaml](openapi.yaml) for external tooling,
  linted by [lint-openapi.sh](lint-openapi.sh) using
  [vacuum](https://github.com/daveshanley/vacuum).
- An interactive **Swagger UI** is served at the configurable `SwaggerPath` (default
  `/swagger/`) using [`swaggo/http-swagger`](https://github.com/swaggo/http-swagger)
  (`httpSwagger.Handler(httpSwagger.URL(openAPIPath))`); visiting `/swagger/` redirects to
  `/swagger/index.html`.

## 11. Configuration

Configuration is resolved by Cobra + Viper in [cmd/root.go](../cmd/root.go), supporting (in
order of precedence) CLI flags, environment variables (prefixed `EUBL21_`, e.g.
`EUBL21_PORT`), and an optional config file (`--config`). Flags marked "worker" are read by
`e-ubl21 worker`; all others apply to `e-ubl21 serve` (`--nats-url` is shared by both).

| Flag                       | Default                 | Purpose                                            |
|-----------------------------|--------------------------|-----------------------------------------------------|
| `--host`                    | `0.0.0.0`                | (serve) HTTP bind host.                             |
| `--port`                    | `8080`                   | (serve) HTTP bind port.                             |
| `--openapi-path`            | `/openapi.json`          | (serve) Path the OpenAPI document is served from.   |
| `--swagger-path`            | `/swagger/`              | (serve) Path the Swagger UI is served from.         |
| `--oauth-issuer`            | *(empty)*                | (serve) Expected JWT `iss` claim (skipped if empty). |
| `--oauth-audience`          | *(empty)*                | (serve) Expected JWT `aud` claim (skipped if empty). |
| `--oauth-hmac-secret`       | *(empty)*                | (serve) Shared secret verifying HS256 bearer tokens; protected routes fail closed (500) if unset. |
| `--nats-url`                | `nats://localhost:4222`  | (serve, worker) NATS server to connect to.          |
| `--rate-limit-rps`          | `5`                      | (serve) Per-client requests/second before rate limiting. |
| `--rate-limit-burst`        | `10`                     | (serve) Per-client burst size for the rate limiter. |
| `--tls-cert-file`           | *(empty)*                | (serve) TLS certificate file (enables TLS with `--tls-key-file`). |
| `--tls-key-file`            | *(empty)*                | (serve) TLS private key file (enables TLS with `--tls-cert-file`). |
| `--worker-consumer-name`    | `ubl-validator`          | (worker) Durable JetStream consumer name; shared across replicas to load-balance. |
| `--worker-subject-filter`   | *(empty = every type)*   | (worker) Restrict this worker to one document type's queue, e.g. `ubl.documents.Invoice`. |
| `--worker-metrics-addr`     | `:9091`                  | (worker) Address the worker's `/metrics`/`/healthz` are served on. |
| `--config`                  | *(empty)*                | Optional configuration file path.                   |

## 12. TLS Termination (API only)

[cmd/serve.go](../cmd/serve.go) starts the API with `http.Server.ListenAndServeTLS` when both
`--tls-cert-file` and `--tls-key-file` are set (rejecting startup if only one is set), and
falls back to plain `ListenAndServe` otherwise. The worker has no TLS support of its own (it
only makes outbound connections, to NATS).

## 13. CLI Integration

- `e-ubl21 serve` starts the HTTP API, binding to `host:port`, logging
  `starting server on <addr>` (or `... (TLS)`) to stdout, and running until the process is
  terminated or the command context is cancelled. Startup failures (e.g. schema catalog or
  NATS connection errors, invalid TLS flag combination) are printed to stderr before exit.
- `e-ubl21 worker` starts the validation worker (section 5), running until SIGINT/SIGTERM.

## 14. Docker / docker-compose Deployment

- [Dockerfile](../Dockerfile) is a multi-stage build: a `golang:1.25-alpine` build stage
  compiles a static (`CGO_ENABLED=0`) binary, copied into an `alpine` runtime stage with
  `libxml2-utils` (providing `xmllint`, a required worker dependency - see section 7) and
  `ca-certificates` installed, running as a non-root user. The same image serves both the
  `api` and `worker` docker-compose services - only the command differs (`serve` vs
  `worker`).
- [.dockerignore](../.dockerignore) keeps the build context small (excludes `.git`,
  generated sample output, docs, etc.) while explicitly **not** excluding `XSD/`, since it is
  embedded into the binary at build time.
- [docker-compose.yml](../docker-compose.yml) defines three services:
  - `nats`: the official `nats:2.14-alpine` image with JetStream enabled (`-js`), a health
    check against its monitoring port's `/healthz`, and a named volume for its JetStream
    store.
  - `api`: builds from the Dockerfile, runs `serve`, depends on `nats` being healthy, and is
    configured entirely via `EUBL21_*` environment variables (NATS URL pointed at the `nats`
    service, OAuth settings, rate limits).
  - `worker`: builds from the same Dockerfile, runs `worker`, also depends on `nats`, and
    exposes its metrics/health port. Can be scaled with `docker compose up --scale worker=3`
    (see section 5).
- Verified end-to-end in this repository: a full `docker compose up` was exercised with real
  HTTP uploads through the `api` container, confirming the `worker` container consumes the
  queued document, validates it via `xmllint`, updates the KV record, and publishes to the
  results stream - all observable via the NATS monitoring API and both services' `/metrics`.

## 15. Testing

- [queue/queue_test.go](../queue/queue_test.go): exercises the `queue` package directly
  against a real, embedded, JetStream-enabled NATS server (see
  [queue/queuetest](../queue/queuetest), built on
  `github.com/nats-io/nats-server/v2/test`) - publishing/consuming documents, per-type
  consumer filtering, the KV upload-record lifecycle, and result publishing.
- [server_test.go](server_test.go): the API's HTTP-level behavior (upload queues a `pending`
  record, unsupported document types are rejected synchronously, unknown ids 404, OpenAPI/
  Swagger are public, OAuth2 signature/expiry/fail-closed behavior, metrics, rate limiting) -
  also against the same embedded NATS server.
- [worker_test.go](worker_test.go): the full asynchronous pipeline - upload → worker consumes
  → `xmllint` validates → KV record updated → `GET` reflects `valid`/`invalid` - plus a test
  that a worker configured with a narrow `SubjectFilter` never consumes a different document
  type's queue.
- [cmd/serve_test.go](../cmd/serve_test.go): CLI config loading and handler wiring.
- All of the above run in well under a second in total and require no Docker/external
  services - the embedded NATS test server is created and torn down per test.

## 16. Known Gaps / Not Yet Implemented

- **No Schematron/PEPPOL/EN 16931 business-rule validation.** The official rule sets are
  separate, large, license-governed artifacts from CEN/PEPPOL that are not included in this
  repository, and implementing a full Schematron processor is a significant separate
  undertaking; documents are validated for XSD structural correctness only.
- **No JWKS/OIDC discovery or asymmetric (RS256/ES256) token signatures** - only a static
  HS256 shared secret is supported; there is no key rotation.
- **No scope/permission-based authorization** beyond issuer/audience matching.
- **No graceful HTTP shutdown for the API** - `serve` runs until killed; only the worker
  installs signal handling to stop cleanly (see section 5), since abandoning an in-flight
  JetStream message mid-processing is the more consequential failure mode this addresses.
- **Rate limiting is per-process, in-memory** - it does not coordinate across multiple API
  replicas behind a load balancer.
- **No consumer of the `UBL_VALIDATION_RESULTS` output stream is included in this repo** - it
  is intentionally left for downstream systems (e.g. notification, indexing) to subscribe to;
  the KV-backed `GET /api/v1/ubl/{id}` is this repo's own way of surfacing outcomes.
